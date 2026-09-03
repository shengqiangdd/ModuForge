package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/moduforge/backend/internal/saferead"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════
// HTTP Transport & Client
// ═══════════════════════════════════════════════════════════════════

// llmHTTPTransport is a shared transport with connection pooling.
var llmHTTPTransport = &http.Transport{
	MaxIdleConns:        30,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
	TLSHandshakeTimeout: 10 * time.Second,
	DisableKeepAlives:   false,
}

// llmHTTPClient is a shared HTTP client with timeouts for LLM API calls.
var llmHTTPClient = &http.Client{
	Transport: llmHTTPTransport,
	Timeout:   180 * time.Second,
}

// LLMHTTPClient returns the shared HTTP client used for all LLM API calls.
func LLMHTTPClient() *http.Client {
	return llmHTTPClient
}

// Optimization 35: HTTP connection pre-warm
var prewarmOnce sync.Map

// PrewarmConnection establishes an idle connection to the given endpoint.
func PrewarmConnection(endpoint string) {
	if endpoint == "" {
		return
	}
	val, _ := prewarmOnce.LoadOrStore(endpoint, &sync.Once{})
	once := val.(*sync.Once)
	once.Do(func() {
		go func() {
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

// ═══════════════════════════════════════════════════════════════════
// Retry Logic
// ═══════════════════════════════════════════════════════════════════

const llmMaxRetries = 5

func isLLMRetryableError(statusCode int) bool {
	return statusCode == 429 || statusCode == 502 || statusCode == 503 || statusCode == 504
}

func llmRetryBackoff(attempt int, retryAfter string) time.Duration {
	if retryAfter != "" {
		if d, err := time.ParseDuration(retryAfter + "s"); err == nil && d > 0 && d <= 120*time.Second {
			return d
		}
	}
	return time.Duration(3<<uint(attempt)) * time.Second
}

// ═══════════════════════════════════════════════════════════════════
// Circuit Breaker
// ═══════════════════════════════════════════════════════════════════

const circuitBreakerThreshold = 5
const circuitBreakerBaseCooldown = 60 * time.Second

type circuitBreaker struct {
	mu            sync.Mutex
	failures      map[string]int
	lastFailure   map[string]time.Time
	breakerActive map[string]bool
}

var globalCircuitBreaker = &circuitBreaker{
	failures:      make(map[string]int),
	lastFailure:   make(map[string]time.Time),
	breakerActive: make(map[string]bool),
}

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

func (cb *circuitBreaker) RecordSuccess(providerID string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures[providerID] = 0
	cb.breakerActive[providerID] = false
}

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

func (cb *circuitBreaker) IsOpen(providerID string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if !cb.breakerActive[providerID] {
		return false
	}
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
// Rate Limit Tracker
// ═══════════════════════════════════════════════════════════════════

type rateLimitTracker struct {
	mu           sync.Mutex
	requests     []time.Time
	maxPerMinute int
	minInterval  time.Duration
}

var globalRateLimiter = &rateLimitTracker{
	maxPerMinute: 0,
	minInterval:  0,
}

func (r *rateLimitTracker) ConfigureForModel(modelName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	lower := strings.ToLower(modelName)
	if strings.Contains(lower, "free") || strings.Contains(lower, "mini") || strings.Contains(lower, "lite") {
		r.maxPerMinute = 0
		r.minInterval = 2 * time.Second
		log.Printf("[RateLimit] configured for free model: no cap, %v min interval", r.minInterval)
	} else {
		r.maxPerMinute = 0
		r.minInterval = 0
	}
}

func (r *rateLimitTracker) WaitForSlot(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.maxPerMinute == 0 && r.minInterval == 0 {
		return nil
	}
	now := time.Now()
	cutoff := now.Add(-1 * time.Minute)
	newRequests := make([]time.Time, 0, len(r.requests))
	for _, t := range r.requests {
		if t.After(cutoff) {
			newRequests = append(newRequests, t)
		}
	}
	r.requests = newRequests
	if r.maxPerMinute > 0 && len(r.requests) >= r.maxPerMinute {
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
	r.requests = append(r.requests, time.Now())
	return nil
}

func (r *rateLimitTracker) Record429() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.maxPerMinute > 4 {
		r.maxPerMinute = r.maxPerMinute * 3 / 4
		log.Printf("[RateLimit] 429 received, reduced limit to %d req/min", r.maxPerMinute)
	}
	if r.minInterval < 10*time.Second {
		r.minInterval += 2 * time.Second
		log.Printf("[RateLimit] increased interval to %v", r.minInterval)
	}
}

// ═══════════════════════════════════════════════════════════════════
// LLM Request
// ═══════════════════════════════════════════════════════════════════

func (r *AgentRunner) callLLMWithTools(ctx context.Context, messages []map[string]interface{}, tools []ToolDef, w SSEWriter, userID, reqProviderID, reqModel string, cfg RunConfig) (*LLMResponse, error) {
	llmStart := time.Now()
	endpoint := cfg.resolvedEndpoint
	apiKey := cfg.resolvedAPIKey
	model := cfg.resolvedModel
	modelTier := cfg.modelTier
	if endpoint == "" {
		endpoint, apiKey, model = r.resolveLLMConfig(userID, reqProviderID, reqModel, cfg)
		modelTier = resolveModelTierWithMaxTokens(model, cfg.MaxOutputTokens)
	}
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint = endpoint + "/chat/completions"
	}
	PrewarmConnection(endpoint)
	if modelTier == TierFree && reqProviderID != "" && globalCircuitBreaker.IsOpen(reqProviderID) {
		log.Printf("[Agent] circuit breaker OPEN for provider %s, attempting fallback", reqProviderID)
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
	globalRateLimiter.ConfigureForModel(model)
	if err := globalRateLimiter.WaitForSlot(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait cancelled: %w", err)
	}
	bodyBytes, err := buildLLMRequestBody(messages, tools, cfg, model, modelTier)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	var lastErr error
	var retryable bool
	for attempt := 0; attempt <= llmMaxRetries; attempt++ {
		if attempt > 0 {
			retryAfter := ""
			if lastErr != nil {
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
			continue
		}
		if resp.StatusCode >= 400 {
			lastErr, retryable = handleLLMHTTPError(resp, modelTier, reqProviderID, model)
			if !retryable {
				r.perfMetrics.RecordError()
				return nil, lastErr
			}
			continue
		}
		if modelTier == TierFree && reqProviderID != "" {
			globalCircuitBreaker.RecordSuccess(reqProviderID)
		}
		result, parseErr := r.parseStreamingResponse(ctx, resp, w)
		resp.Body.Close()
		if parseErr != nil {
			lastErr = parseErr
			if result == nil || (result.Content == "" && len(result.ToolCalls) == 0) {
				continue
			}
		}
		if result != nil && result.TokenUsage != nil && result.TokenUsage.TotalTokens > 0 {
			r.perfMetrics.RecordTokenUsage(result.TokenUsage.TotalTokens)
			w.WriteSSE(map[string]interface{}{
				"type":  "usage",
				"usage": result.TokenUsage,
			})
		}
		r.perfMetrics.RecordLLMCall(time.Since(llmStart))
		return result, nil
	}
	r.perfMetrics.RecordError()
	return nil, fmt.Errorf("LLM failed after %d attempts: %w", llmMaxRetries+1, lastErr)
}

func buildLLMRequestBody(messages []map[string]interface{}, tools []ToolDef, cfg RunConfig, model string, modelTier ModelTier) ([]byte, error) {
	body := map[string]interface{}{
		"model":    model,
		"messages": messages,
		"stream":   true,
	}
	approxContextTokens := 0
	for _, msg := range messages {
		if c, ok := msg["content"].(string); ok {
			approxContextTokens += len(c) / 4
		}
	}
	maxTokens := cfg.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = resolveModelMaxTokens(model)
	}
	// Context window limit: use tier-based limits for context window,
	// NOT maxTokens (which is max OUTPUT tokens, not context window).
	// This ensures models with small output limits but large context windows
	// (like mimo-v2.5 with 8K output but 32K+ context) can use their full context.
	contextLimit := 16000
	if modelTier == TierMid {
		contextLimit = 32000
	} else if modelTier == TierStrong {
		contextLimit = 128000
	}
	// Reserve space for system prompt + tools + output
	reservedTokens := 8000
	if modelTier == TierStrong {
		reservedTokens = 4000
	}
	if maxTokens > 0 && approxContextTokens > 0 {
		remaining := contextLimit - approxContextTokens - reservedTokens
		if remaining < maxTokens {
			maxTokens = remaining
			if maxTokens < 1024 {
				maxTokens = 1024
			}
			log.Printf("[Agent] adaptive max_tokens: context=%d/%d, reduced max_tokens to %d (reserved=%d)", approxContextTokens, contextLimit, maxTokens, reservedTokens)
		}
	}
	if maxTokens > 0 {
		body["max_tokens"] = maxTokens
	}
	if len(tools) > 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}
	return json.Marshal(body)
}

func handleLLMHTTPError(resp *http.Response, modelTier ModelTier, reqProviderID, model string) (error, bool) {
	respBody, _ := saferead.SafeReadAll(resp.Body)
	resp.Body.Close()
	errMsg := fmt.Sprintf("LLM error (HTTP %d) provider=%s model=%s: %s", resp.StatusCode, reqProviderID, model, string(respBody))
	err := errors.New(errMsg)
	if !isLLMRetryableError(resp.StatusCode) {
		return err, false
	}
	if resp.StatusCode == 429 {
		globalRateLimiter.Record429()
	}
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		err = fmt.Errorf("%s\nRetry-After: %s", errMsg, retryAfter)
	}
	return err, true
}
