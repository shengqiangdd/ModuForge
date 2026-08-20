package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// LLMResponse represents the parsed response from an LLM API call.
type LLMResponse struct {
	Role         string        `json:"role"`
	Content      string        `json:"content"`
	ToolCalls    []LLMToolCall `json:"tool_calls"`
	FinishReason string        `json:"finish_reason,omitempty"` // "stop" or "length"
	TokenUsage   *TokenUsage   `json:"token_usage,omitempty"`
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
	llmStart := time.Now()
	// P0-Optimization: Use cached resolved config from RunConfig instead of re-querying DB.
	// The config was resolved once at Run() entry and stored in cfg.resolved* fields.
	endpoint := cfg.resolvedEndpoint
	apiKey := cfg.resolvedAPIKey
	model := cfg.resolvedModel
	modelTier := cfg.modelTier

	// Fallback: if resolved config is empty (shouldn't happen after Run() init), resolve now
	if endpoint == "" {
		endpoint, apiKey, model = r.resolveLLMConfig(userID, reqProviderID, reqModel, cfg)
		modelTier = resolveModelTier(model)
	}

	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint = endpoint + "/chat/completions"
	}

	// Optimization 35: Pre-warm HTTP connection (async, no-op after first call)
	PrewarmConnection(endpoint)

	// Circuit breaker: skip free model providers with consecutive failures
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

	bodyBytes, err := buildLLMRequestBody(messages, tools, cfg, model, modelTier)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Retry loop with exponential backoff for transient errors (429, 5xx, network)
	var lastErr error
	var retryable bool
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
			lastErr, retryable = handleLLMHTTPError(resp, modelTier, reqProviderID, model)
			if !retryable {
				r.perfMetrics.RecordError()
				return nil, lastErr // permanent error, no retry
			}
			continue // retry on 429/5xx
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
		if result != nil && result.TokenUsage != nil && result.TokenUsage.TotalTokens > 0 {
			// Record aggregated token usage and notify the frontend (per-message display).
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

// buildLLMRequestBody constructs the chat/completions request body, applying
// adaptive max_tokens based on the approximate context size.
func buildLLMRequestBody(messages []map[string]interface{}, tools []ToolDef, cfg RunConfig, model string, modelTier ModelTier) ([]byte, error) {
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

	return json.Marshal(body)
}
