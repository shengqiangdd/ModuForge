package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// llmHTTPTransport is a shared transport with connection pooling.
// Reusing TCP connections avoids the TLS handshake overhead on every LLM call,
// which is critical for free-tier providers that rate-limit per-connection.
var llmHTTPTransport = &http.Transport{
	MaxIdleConns:        30, // Optimization 41: increased from 10 to support concurrent agent sessions
	MaxIdleConnsPerHost: 10, // Optimization 41: increased from 5 to reuse more connections per LLM endpoint
	IdleConnTimeout:     90 * time.Second,
	TLSHandshakeTimeout: 10 * time.Second,
	DisableKeepAlives:   false,
}

// llmHTTPClient is a shared HTTP client with timeouts for LLM API calls.
// Using http.DefaultClient (no timeout) caused requests to hang forever when
// the LLM endpoint was unresponsive, leading to connection drops and "network error" on the frontend.
var llmHTTPClient = &http.Client{
	Transport: llmHTTPTransport,
	Timeout:   180 * time.Second, // hard ceiling per request (increased for slow/free models with large context)
}

// LLMHTTPClient returns the shared HTTP client used for all LLM API calls.
// Skills should reuse this client to share connection pools and timeouts.
func LLMHTTPClient() *http.Client {
	return llmHTTPClient
}

// Optimization 35: HTTP connection pre-warm
// Proactively establishes a TCP+TLS connection to the LLM endpoint so the first
// real request doesn't pay the cold-start latency (200-500ms for TLS handshake).
// Called once per endpoint; subsequent calls are no-ops via sync.Once.
var prewarmOnce sync.Map // endpoint -> *sync.Once

// PrewarmConnection establishes an idle connection to the given endpoint.
// Safe to call concurrently; only performs the handshake once per endpoint.
func PrewarmConnection(endpoint string) {
	if endpoint == "" {
		return
	}
	val, _ := prewarmOnce.LoadOrStore(endpoint, &sync.Once{})
	once := val.(*sync.Once)
	once.Do(func() {
		go func() {
			// Strip /chat/completions suffix for the HEAD probe
			base := endpoint
			if idx := strings.Index(base, "/chat/completions"); idx > 0 {
				base = base[:idx]
			}
			req, err := http.NewRequest("HEAD", base, nil)
			if err != nil {
				return
			}
			resp, err := llmHTTPClient.Do(req)
			if err != nil {
				debugLog("prewarm failed for %s: %v", base, err)
				return
			}
			resp.Body.Close()
			debugLog("prewarmed connection to %s (status=%d)", base, resp.StatusCode)
		}()
	})
}

// llmMaxRetries is the maximum number of retries for transient LLM errors (429, 5xx, network).
const llmMaxRetries = 3

// isLLMRetryableError returns true for errors worth retrying (rate limit, server errors, network).
func isLLMRetryableError(statusCode int) bool {
	return statusCode == 429 || statusCode == 502 || statusCode == 503 || statusCode == 504
}

// llmRetryBackoff returns the sleep duration for attempt number (1-based).
// Uses exponential backoff: 2s, 4s, 8s. For 429 with Retry-After header, uses that instead.
func llmRetryBackoff(attempt int, retryAfter string) time.Duration {
	if retryAfter != "" {
		if d, err := time.ParseDuration(retryAfter + "s"); err == nil && d > 0 && d <= 30*time.Second {
			return d
		}
	}
	return time.Duration(1<<uint(attempt)) * time.Second // 2s, 4s, 8s
}

// ═══════════════════════════════════════════════════════════════════
// Circuit Breaker — for free model providers only
//
// Tracks consecutive failures per provider. After threshold failures,
// temporarily skips the provider for cooldown period.
// Optimization 29: Adaptive cooldown — longer cooldown for more failures
// ═══════════════════════════════════════════════════════════════════

// Circuit breaker: consecutive failures before skipping a free model provider
const circuitBreakerThreshold = 3

// Circuit breaker: base cooldown period after breaker opens
const circuitBreakerBaseCooldown = 60 * time.Second

type circuitBreaker struct {
	mu            sync.Mutex
	failures      map[string]int       // providerID -> consecutive failures
	lastFailure   map[string]time.Time // providerID -> last failure time
	breakerActive map[string]bool      // providerID -> is breaker open?
}

var globalCircuitBreaker = &circuitBreaker{
	failures:      make(map[string]int),
	lastFailure:   make(map[string]time.Time),
	breakerActive: make(map[string]bool),
}

// adaptiveCooldown returns cooldown proportional to failure count:
// 3 failures → 60s, 5 → 120s, 10 → 300s, 15+ → 600s
func adaptiveCooldown(failures int) time.Duration {
	switch {
	case failures >= 15:
		return 600 * time.Second
	case failures >= 10:
		return 300 * time.Second
	case failures >= 5:
		return 120 * time.Second
	default:
		return circuitBreakerBaseCooldown
	}
}

// RecordSuccess resets the failure counter for a provider.
func (cb *circuitBreaker) RecordSuccess(providerID string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures[providerID] = 0
	cb.breakerActive[providerID] = false
}

// RecordFailure increments the failure counter and opens the breaker if threshold reached.
func (cb *circuitBreaker) RecordFailure(providerID string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures[providerID]++
	cb.lastFailure[providerID] = time.Now()
	if cb.failures[providerID] >= circuitBreakerThreshold {
		cb.breakerActive[providerID] = true
		cooldown := adaptiveCooldown(cb.failures[providerID])
		log.Printf("[CircuitBreaker] OPEN for provider %s after %d failures (cooldown=%v)",
			providerID, cb.failures[providerID], cooldown)
	}
}

// IsOpen returns true if the circuit breaker is open (provider should be skipped).
func (cb *circuitBreaker) IsOpen(providerID string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if !cb.breakerActive[providerID] {
		return false
	}
	// Adaptive cooldown based on failure count
	cooldown := adaptiveCooldown(cb.failures[providerID])
	if time.Since(cb.lastFailure[providerID]) > cooldown {
		log.Printf("[CircuitBreaker] cooldown elapsed for provider %s (was %v), half-opening", providerID, cooldown)
		cb.breakerActive[providerID] = false
		cb.failures[providerID] = 0
		return false
	}
	return true
}

// ═══════════════════════════════════════════════════════════════════
// Rate Limit Tracker — client-side tracking to avoid 429s
// ═══════════════════════════════════════════════════════════════════

type rateLimitTracker struct {
	mu           sync.Mutex
	requests     []time.Time   // timestamps of recent requests
	maxPerMinute int           // max requests per minute (0 = no limit)
	minInterval  time.Duration // minimum interval between requests
}

var globalRateLimiter = &rateLimitTracker{
	maxPerMinute: 0, // disabled by default, can be configured per model
	minInterval:  0,
}

// ConfigureForModel sets rate limits based on model tier.
func (r *rateLimitTracker) ConfigureForModel(modelName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	lower := strings.ToLower(modelName)
	// Free models: conservative limits to avoid 429s
	if strings.Contains(lower, "free") || strings.Contains(lower, "mini") || strings.Contains(lower, "lite") {
		r.maxPerMinute = 10             // 10 requests per minute
		r.minInterval = 6 * time.Second // at least 6s between requests
		log.Printf("[RateLimit] configured for free model: %d req/min, %v interval", r.maxPerMinute, r.minInterval)
	} else {
		// Paid models: no client-side limits
		r.maxPerMinute = 0
		r.minInterval = 0
	}
}

// WaitForSlot blocks until it's safe to make a request.
func (r *rateLimitTracker) WaitForSlot(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.maxPerMinute == 0 && r.minInterval == 0 {
		return nil // no limits
	}

	now := time.Now()

	// Clean up old requests (older than 1 minute)
	cutoff := now.Add(-1 * time.Minute)
	newRequests := make([]time.Time, 0, len(r.requests))
	for _, t := range r.requests {
		if t.After(cutoff) {
			newRequests = append(newRequests, t)
		}
	}
	r.requests = newRequests

	// Check rate limit
	if r.maxPerMinute > 0 && len(r.requests) >= r.maxPerMinute {
		// Wait until oldest request falls out of window
		oldest := r.requests[0]
		waitTime := oldest.Add(1*time.Minute).Sub(now) + 100*time.Millisecond
		if waitTime > 0 {
			log.Printf("[RateLimit] rate limit reached, waiting %v", waitTime)
			select {
			case <-time.After(waitTime):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	// Check minimum interval
	if r.minInterval > 0 && len(r.requests) > 0 {
		lastRequest := r.requests[len(r.requests)-1]
		elapsed := now.Sub(lastRequest)
		if elapsed < r.minInterval {
			waitTime := r.minInterval - elapsed
			log.Printf("[RateLimit] min interval not met, waiting %v", waitTime)
			select {
			case <-time.After(waitTime):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	// Record this request
	r.requests = append(r.requests, time.Now())
	return nil
}

// Record429 records a 429 error and adjusts limits.
func (r *rateLimitTracker) Record429() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// On 429, be more conservative: reduce limit by 25%
	if r.maxPerMinute > 4 {
		r.maxPerMinute = r.maxPerMinute * 3 / 4
		log.Printf("[RateLimit] 429 received, reduced limit to %d req/min", r.maxPerMinute)
	}
	// Increase minimum interval
	if r.minInterval < 10*time.Second {
		r.minInterval += 2 * time.Second
		log.Printf("[RateLimit] increased interval to %v", r.minInterval)
	}
}

// ═══════════════════════════════════════════════════════════════════
// Model Tier & Max Tokens
// ═══════════════════════════════════════════════════════════════════

// defaultModelMaxTokens maps known model name substrings to their default max output tokens.
// Used when the user hasn't explicitly configured max_tokens for a model.
var defaultModelMaxTokens = map[string]int{
	// OpenAI
	"o1":          32768,
	"o3":          32768,
	"o4-mini":     32768,
	"gpt-4o":      16384,
	"gpt-4-turbo": 4096,
	"gpt-4":       8192,
	"gpt-3.5":     4096,
	// Anthropic
	"claude-3.5-sonnet": 8192,
	"claude-3-opus":     4096,
	"claude-3-haiku":    8192,
	"claude-4":          16384,
	"claude":            8192,
	// Google
	"gemini-2.5-pro":   65536,
	"gemini-2.5-flash": 65536,
	"gemini-2.0":       8192,
	"gemini-1.5-pro":   8192,
	"gemini":           8192,
	// DeepSeek
	"deepseek-v3":    8192,
	"deepseek-v2.5":  8192,
	"deepseek-coder": 8192,
	"deepseek":       8192,
	// Qwen
	"qwen-max":   8192,
	"qwen-plus":  8192,
	"qwen-turbo": 8192,
	"qwen":       8192,
	// Meta
	"llama-3.1-405b": 4096,
	"llama-3.1-70b":  4096,
	"llama-3.1-8b":   4096,
	"llama":          4096,
	// Mistral
	"mistral-large":  8192,
	"mistral-medium": 8192,
	"mistral":        8192,
	// Default for unknown models
	"_default": 8192,
}

// resolveModelMaxTokens returns the max output tokens for a model name.
// It checks the model name against known patterns (case-insensitive substring match).
func resolveModelMaxTokens(modelName string) int {
	lower := strings.ToLower(modelName)
	// Longest match first (more specific patterns)
	bestLen := 0
	bestVal := defaultModelMaxTokens["_default"]
	for pattern, val := range defaultModelMaxTokens {
		if pattern == "_default" {
			continue
		}
		if strings.Contains(lower, pattern) && len(pattern) > bestLen {
			bestLen = len(pattern)
			bestVal = val
		}
	}
	return bestVal
}

// Model tier determines context handling aggressiveness.
// Tier 0 (free/weak): small context, aggressive compaction, smart truncation
// Tier 1 (mid): moderate context, standard compaction
// Tier 2 (strong/paid): large context, lazy compaction, no truncation
type ModelTier int

const (
	TierFree   ModelTier = 0 // free models, small context (deepseek-v4-flash-free, etc.)
	TierMid    ModelTier = 1 // mid-tier models (deepseek-v3, qwen-turbo, etc.)
	TierStrong ModelTier = 2 // strong paid models (gpt-4o, claude, gemini-pro, etc.)
)

// modelTierCache caches tier resolution results (model names don't change at runtime).
var modelTierCache sync.Map

func resolveModelTier(modelName string) ModelTier {
	// Fast path: cached
	if cached, ok := modelTierCache.Load(modelName); ok {
		return cached.(ModelTier)
	}
	// Slow path: compute and cache
	lower := strings.ToLower(modelName)
	// Free/weak models — aggressive limits
	freePatterns := []string{"free", "mini", "flash-free", "lite", "nano"}
	for _, p := range freePatterns {
		if strings.Contains(lower, p) {
			modelTierCache.Store(modelName, TierFree)
			return TierFree
		}
	}
	// Strong models — generous limits
	strongPatterns := []string{"gpt-4o", "gpt-4-turbo", "claude-3.5", "claude-4", "claude-3-opus",
		"gemini-2.5-pro", "gemini-1.5-pro", "o1", "o3", "deepseek-r1", "qwen-max"}
	for _, p := range strongPatterns {
		if strings.Contains(lower, p) {
			modelTierCache.Store(modelName, TierStrong)
			return TierStrong
		}
	}
	// Everything else is mid-tier
	modelTierCache.Store(modelName, TierMid)
	return TierMid
}

// compactionThresholdForTier returns the context compaction threshold for a model tier.
// For free models with 16K context, we must be much more aggressive to leave room for
// system prompt (~800) + tool definitions (~1840) + output (~4096) = ~6700 tokens overhead.
func compactionThresholdForTier(tier ModelTier) int {
	switch tier {
	case TierFree:
		return 8000 // very aggressive: 16K context - 6700 overhead = ~9K for conversation
	case TierMid:
		return 30000 // moderate
	case TierStrong:
		return 100000 // generous: let strong models use their full context
	default:
		return 30000
	}
}

// maxResultLenForTier returns the tool result size limit for a model tier.
func maxResultLenForTier(tier ModelTier) int {
	switch tier {
	case TierFree:
		return 12000 // small: minimize context bloat
	case TierMid:
		return 24000 // moderate
	case TierStrong:
		return 48000 // generous: let strong models read large files
	default:
		return 24000
	}
}

// ═══════════════════════════════════════════════════════════════════
// LLM API — native function calling
// ═══════════════════════════════════════════════════════════════════

// LLMResponse represents the parsed response from an LLM API call.
type LLMResponse struct {
	Role         string        `json:"role"`
	Content      string        `json:"content"`
	ToolCalls    []LLMToolCall `json:"tool_calls"`
	FinishReason string        `json:"finish_reason,omitempty"` // "stop" or "length"
}

// ToolCallFunction holds the function details of a tool call.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// LLMToolCall represents a single tool call returned by the LLM.
type LLMToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

func (r *AgentRunner) callLLMWithTools(ctx context.Context, messages []map[string]interface{}, tools []ToolDef, w SSEWriter, userID, reqProviderID, reqModel string, cfg RunConfig) (*LLMResponse, error) {
	endpoint, apiKey, model := r.resolveLLMConfig(userID, reqProviderID, reqModel, cfg)
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint = endpoint + "/chat/completions"
	}

	// Optimization 35: Pre-warm HTTP connection (async, no-op after first call)
	PrewarmConnection(endpoint)

	// Circuit breaker: skip free model providers with consecutive failures
	modelTier := resolveModelTier(model)
	if modelTier == TierFree && reqProviderID != "" && globalCircuitBreaker.IsOpen(reqProviderID) {
		log.Printf("[Agent] circuit breaker OPEN for provider %s, attempting fallback", reqProviderID)
		// Optimization 21: Try fallback providers before giving up
		if fallbackEndpoint, fallbackKey, fallbackModel, fallbackID := r.findFallbackProvider(userID, reqProviderID, model); fallbackID != "" {
			log.Printf("[Agent] fallback provider found: %s (endpoint=%s model=%s)", fallbackID, fallbackEndpoint, fallbackModel)
			endpoint = fallbackEndpoint
			apiKey = fallbackKey
			model = fallbackModel
			reqProviderID = fallbackID
		} else {
			return nil, fmt.Errorf("provider %s temporarily unavailable (circuit breaker open), and no fallback providers found. Please try another model or wait %v", reqProviderID, circuitBreakerBaseCooldown)
		}
	}

	// Configure and wait for rate limit slot
	globalRateLimiter.ConfigureForModel(model)
	if err := globalRateLimiter.WaitForSlot(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait cancelled: %w", err)
	}

	body := map[string]interface{}{
		"model":    model,
		"messages": messages,
		"stream":   true,
	}

	// Optimization 7: Adaptive max_tokens based on context size
	// Calculate approximate context size to adjust output tokens
	approxContextTokens := 0
	for _, msg := range messages {
		if c, ok := msg["content"].(string); ok {
			approxContextTokens += len(c) / 4 // rough estimate: 4 chars per token
		}
	}
	// Resolve max_output_tokens: RunConfig > model default
	maxTokens := cfg.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = resolveModelMaxTokens(model)
	}
	// Adaptive: if context is large, reduce max_tokens to leave room
	if maxTokens > 0 && approxContextTokens > 0 {
		// For free models with 16K context, be more aggressive
		contextLimit := 16000
		if modelTier == TierMid {
			contextLimit = 32000
		} else if modelTier == TierStrong {
			contextLimit = 128000
		}
		remaining := contextLimit - approxContextTokens - 1000 // 1000 for system prompt overhead
		if remaining < maxTokens {
			maxTokens = remaining
			if maxTokens < 1024 {
				maxTokens = 1024 // minimum output
			}
			log.Printf("[Agent] adaptive max_tokens: context=%d, reduced max_tokens to %d", approxContextTokens, maxTokens)
		}
	}
	if maxTokens > 0 {
		body["max_tokens"] = maxTokens
	}
	if len(tools) > 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Retry loop with exponential backoff for transient errors (429, 5xx, network)
	var lastErr error
	for attempt := 0; attempt <= llmMaxRetries; attempt++ {
		if attempt > 0 {
			retryAfter := ""
			if lastErr != nil {
				// Try to extract Retry-After from error message
				errStr := lastErr.Error()
				if idx := strings.Index(errStr, "Retry-After:"); idx >= 0 {
					rest := errStr[idx+len("Retry-After:"):]
					if endIdx := strings.IndexAny(rest, "\n\r"); endIdx > 0 {
						retryAfter = strings.TrimSpace(rest[:endIdx])
					}
				}
			}
			backoff := llmRetryBackoff(attempt, retryAfter)
			log.Printf("[Agent] LLM retry %d/%d after %v: %v", attempt, llmMaxRetries, backoff, lastErr)
			w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "think",
				"content": fmt.Sprintf("⚠️ LLM 请求失败 (%v)，%v 后重试 (%d/%d)...", lastErr, backoff, attempt, llmMaxRetries),
			})
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		debugLog("LLM request (attempt %d/%d): endpoint=%s model=%s apiKey_len=%d", attempt+1, llmMaxRetries+1, endpoint, model, len(apiKey))
		resp, err := llmHTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("LLM request failed: %w", err)
			continue // retry on network error
		}

		if resp.StatusCode >= 400 {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			errMsg := fmt.Sprintf("LLM error (HTTP %d): %s", resp.StatusCode, string(respBody))
			lastErr = errors.New(errMsg)
			if isLLMRetryableError(resp.StatusCode) {
				// Record 429 for rate limit tracking
				if resp.StatusCode == 429 {
					globalRateLimiter.Record429()
					// Circuit breaker: record failure for free models
					if modelTier == TierFree && reqProviderID != "" {
						globalCircuitBreaker.RecordFailure(reqProviderID)
					}
				}
				// Extract Retry-After header for 429
				if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
					lastErr = fmt.Errorf("%s\nRetry-After: %s", errMsg, retryAfter)
				}
				continue // retry on 429/5xx
			}
			return nil, lastErr // permanent error, no retry
		}

		// Success — record success for circuit breaker
		if modelTier == TierFree && reqProviderID != "" {
			globalCircuitBreaker.RecordSuccess(reqProviderID)
		}

		// Success — parse the streaming response
		result, parseErr := r.parseStreamingResponse(ctx, resp, w)
		resp.Body.Close()
		if parseErr != nil {
			lastErr = parseErr
			// Only retry if we got no data at all (stream interrupted before any content)
			if result == nil || (result.Content == "" && len(result.ToolCalls) == 0) {
				continue
			}
		}
		return result, nil
	}

	return nil, fmt.Errorf("LLM failed after %d attempts: %w", llmMaxRetries+1, lastErr)
}

// parseStreamingResponse reads an SSE stream and extracts content + tool calls.
// Uses a 256KB scanner buffer to handle large tool call JSON without truncation.
func (r *AgentRunner) parseStreamingResponse(ctx context.Context, resp *http.Response, w SSEWriter) (*LLMResponse, error) {
	var fullContent strings.Builder
	var toolCalls []LLMToolCall
	toolCallMap := make(map[int]*LLMToolCall, 4) // Optimization 36: pre-allocate for typical 1-3 tool calls
	var finishReason string

	keepAliveDone := make(chan struct{})
	startKeepalive(ctx, w, keepAliveDone, 10*time.Second)

	// Use a large buffer to avoid "bufio.Scanner: token too long" on long tool call JSONs.
	// Default 64KB is too small; 256KB handles most LLM responses comfortably.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		if w.IsDisconnected() {
			break
		}

		var parsed struct {
			Choices []struct {
				Delta struct {
					Role             string `json:"role"`
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &parsed); err != nil {
			// Log failed parse at debug level (some LLMs send non-standard chunks)
			debugLog("stream parse failed (len=%d): %v", len(data), err)
			continue
		}
		if len(parsed.Choices) == 0 {
			continue
		}

		delta := parsed.Choices[0].Delta
		if parsed.Choices[0].FinishReason != "" {
			finishReason = parsed.Choices[0].FinishReason
		}

		if delta.ReasoningContent != "" {
			cleaned := sanitizeReasoning(delta.ReasoningContent)
			if cleaned != "" {
				w.WriteSSE(map[string]interface{}{"type": "reasoning", "content": cleaned})
			}
		}
		if delta.Content != "" {
			fullContent.WriteString(delta.Content)
			w.WriteSSE(map[string]interface{}{"type": "stream_delta", "content": delta.Content})
		}
		for _, tc := range delta.ToolCalls {
			idx := tc.Index
			if idx < 0 {
				idx = 0
			}
			existing, ok := toolCallMap[idx]
			if !ok {
				toolCallMap[idx] = &LLMToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: ToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			} else {
				if tc.ID != "" {
					existing.ID = tc.ID
				}
				if tc.Function.Name != "" {
					existing.Function.Name = tc.Function.Name
				}
				existing.Function.Arguments += tc.Function.Arguments
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("[Agent] scanner error: %v", err)
		close(keepAliveDone)
		// If scanner failed and we have no data at all, propagate the error
		// so callers know the LLM stream was interrupted (network/proxy issue).
		if fullContent.Len() == 0 && len(toolCallMap) == 0 {
			return nil, fmt.Errorf("LLM stream interrupted: %w", err)
		}
	}
	close(keepAliveDone)

	toolCalls = mergeToolCalls(toolCallMap)

	// Validate and repair tool calls from weak models
	toolCalls = repairToolCalls(toolCalls)

	return &LLMResponse{
		Role:         "assistant",
		Content:      fullContent.String(),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
	}, nil
}

// mergeToolCalls flattens the per-index tool call map into an ordered slice.
func mergeToolCalls(toolCallMap map[int]*LLMToolCall) []LLMToolCall {
	if len(toolCallMap) == 0 {
		return nil
	}
	toolCalls := make([]LLMToolCall, 0, len(toolCallMap))
	for i := 0; i < len(toolCallMap); i++ {
		if tc, ok := toolCallMap[i]; ok {
			toolCalls = append(toolCalls, *tc)
		}
	}
	return toolCalls
}

// repairToolCalls attempts to fix malformed tool call JSON from weak/free models.
// Common issues: truncated JSON, missing quotes, unescaped characters.
func repairToolCalls(toolCalls []LLMToolCall) []LLMToolCall {
	if len(toolCalls) == 0 {
		return toolCalls
	}

	repaired := make([]LLMToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		if tc.Function.Name == "" {
			log.Printf("[Agent] skipping tool call with empty name")
			continue
		}

		args := tc.Function.Arguments
		if args == "" {
			// Some models send empty arguments for tools that don't need them
			repaired = append(repaired, tc)
			continue
		}

		// Try to parse as JSON
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(args), &parsed); err != nil {
			fixed, repairErr := repairJSONArguments(args)
			if repairErr != nil {
				log.Printf("[Agent] cannot repair tool call JSON for %s: %v (original: %s)", tc.Function.Name, repairErr, args[:min(len(args), 100)])
				// Skip this tool call - it's unrecoverable
				continue
			}
			tc.Function.Arguments = fixed
		}

		repaired = append(repaired, tc)
	}

	return repaired
}

// repairJSONArguments applies a chain of progressively more aggressive fixes to
// malformed tool-call argument JSON. Returns the fixed JSON or an error if all
// repair attempts fail.
func repairJSONArguments(args string) (string, error) {
	fixed := args

	// Fix 1: Unescaped newlines in strings
	fixed = strings.ReplaceAll(fixed, "\n", "\\n")
	fixed = strings.ReplaceAll(fixed, "\r", "\\r")
	fixed = strings.ReplaceAll(fixed, "\t", "\\t")

	// Fix 2: Try to find JSON object boundaries
	start := strings.Index(fixed, "{")
	end := strings.LastIndex(fixed, "}")
	if start >= 0 && end > start {
		fixed = fixed[start : end+1]
	}

	// Fix 3: Try to fix trailing commas
	fixed = strings.ReplaceAll(fixed, ",}", "}")
	fixed = strings.ReplaceAll(fixed, ",]", "]")

	// Fix 4: Fix unescaped quotes inside strings (common in code content)
	// This is tricky - we need to be careful not to break valid JSON
	// Only apply if the JSON still fails after other fixes

	// Fix 5: Fix missing colons between key and value
	fixed = strings.ReplaceAll(fixed, `"path" "`, `"path": "`)
	fixed = strings.ReplaceAll(fixed, `"content" "`, `"content": "`)
	fixed = strings.ReplaceAll(fixed, `"query" "`, `"query": "`)
	fixed = strings.ReplaceAll(fixed, `"thought" "`, `"thought": "`)
	fixed = strings.ReplaceAll(fixed, `"action" "`, `"action": "`)
	fixed = strings.ReplaceAll(fixed, `"key" "`, `"key": "`)
	fixed = strings.ReplaceAll(fixed, `"value" "`, `"value": "`)
	fixed = strings.ReplaceAll(fixed, `"description" "`, `"description": "`)

	// Fix 6: Fix single quotes instead of double quotes for keys
	fixed = strings.ReplaceAll(fixed, "'path'", `"path"`)
	fixed = strings.ReplaceAll(fixed, "'content'", `"content"`)
	fixed = strings.ReplaceAll(fixed, "'query'", `"query"`)
	fixed = strings.ReplaceAll(fixed, "'thought'", `"thought"`)
	fixed = strings.ReplaceAll(fixed, "'action'", `"action"`)
	fixed = strings.ReplaceAll(fixed, "'key'", `"key"`)
	fixed = strings.ReplaceAll(fixed, "'value'", `"value"`)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(fixed), &parsed); err != nil {
		// Fix 7: Try to extract the first valid JSON object from the string
		// Some models prefix with explanatory text
		idx := strings.Index(fixed, "{")
		if idx > 0 {
			candidate := fixed[idx:]
			// Find matching closing brace
			depth := 0
			endIdx := -1
			for i, ch := range candidate {
				if ch == '{' {
					depth++
				} else if ch == '}' {
					depth--
					if depth == 0 {
						endIdx = i + 1
						break
					}
				}
			}
			if endIdx > 0 {
				candidate = candidate[:endIdx]
				if err3 := json.Unmarshal([]byte(candidate), &parsed); err3 == nil {
					return candidate, nil
				}
			}
		}
		return "", err
	}
	return fixed, nil
}

// resolveLLMConfig picks the right endpoint/apiKey/model for this request.
// Priority: RunConfig (handler-resolved) > llm_providers DB > llm_config DB > runner defaults
func (r *AgentRunner) resolveLLMConfig(userID, reqProviderID, reqModel string, cfg ...RunConfig) (string, string, string) {
	// If RunConfig has pre-resolved values, use them directly
	if len(cfg) > 0 && cfg[0].LLMEndpoint != "" {
		endpoint := cfg[0].LLMEndpoint
		apiKey := cfg[0].LLMApiKey
		model := cfg[0].LLMModel
		if reqModel != "" {
			model = reqModel
		}
		log.Printf("[Agent] resolveLLMConfig: using RunConfig endpoint=%s model=%s", endpoint, model)
		return endpoint, apiKey, model
	}

	endpoint := r.endpoint
	apiKey := r.apiKey
	model := r.model

	if reqProviderID != "" && r.db != nil {
		var ep, key, mdl string
		err := r.db.QueryRow(
			"SELECT endpoint, api_key, model_id FROM llm_providers WHERE id=? AND user_id=?",
			reqProviderID, userID,
		).Scan(&ep, &key, &mdl)
		if err == nil && ep != "" {
			endpoint = ep
			if key != "" {
				apiKey = key
			}
			if reqModel != "" {
				model = reqModel
			} else if mdl != "" {
				model = mdl
			}
			log.Printf("[Agent] resolveLLMConfig: provider=%s endpoint=%s model=%s", reqProviderID, endpoint, model)
			return endpoint, apiKey, model
		}
		log.Printf("[Agent] resolveLLMConfig: provider=%s not found in db, fallback to default", reqProviderID)
	}

	if r.db != nil {
		var cfgModel, cfgEndpoint, cfgKey string
		err := r.db.QueryRow("SELECT model_id, endpoint, api_key FROM llm_config WHERE id='default'").Scan(&cfgModel, &cfgEndpoint, &cfgKey)
		if err == nil {
			if cfgModel != "" {
				model = cfgModel
			}
			if cfgEndpoint != "" {
				endpoint = cfgEndpoint
			}
			if cfgKey != "" {
				apiKey = cfgKey
			}
		}
	}

	if reqModel != "" {
		model = reqModel
	}
	log.Printf("[Agent] resolveLLMConfig: fallback endpoint=%s model=%s", endpoint, model)
	return endpoint, apiKey, model
}
