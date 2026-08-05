package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// handleLLMCallError processes a failed LLM call. It returns the (possibly
// compacted or annotated) conversation, the updated consecutive error count,
// and a non-nil error only when the run must abort immediately (non-retryable
// error or retry limit reached). When it returns nil the caller should
// `continue` the iteration loop so the next attempt re-enters the loop body.
func (r *AgentRunner) handleLLMCallError(ctx context.Context, w SSEWriter, cfg RunConfig, conversation []map[string]interface{}, consecutiveErrors int, err error) ([]map[string]interface{}, int, error) {
	// Classify error: permanent errors stop immediately, transient ones retry
	if !isRetryableError(err) {
		log.Printf("[Agent] non-retryable error: %v", err)
		w.WriteSSE(map[string]interface{}{"type": "error", "error": userFriendlyError(err)})
		w.WriteSSEPlain("[DONE]")
		return conversation, consecutiveErrors, err
	}
	consecutiveErrors++
	if consecutiveErrors >= 3 {
		log.Printf("[Agent] retry limit reached after %d attempts: %v", consecutiveErrors, err)
		w.WriteSSE(map[string]interface{}{"type": "error", "error": userFriendlyError(err)})
		w.WriteSSEPlain("[DONE]")
		return conversation, consecutiveErrors, err
	}
	// Auto-compact on context-too-long errors before retrying
	errStr := err.Error()
	if strings.Contains(errStr, "context_length_exceeded") || strings.Contains(errStr, "maximum context length") || strings.Contains(errStr, "max_tokens") {
		log.Printf("[Agent] context too long, compacting conversation before retry...")
		compacted, cErr := r.compactConversation(ctx, conversation, w, cfg)
		if cErr == nil {
			conversation = compacted
			w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "think",
				"content": "上下文过长，已自动压缩，正在重试...",
			})
		}
	}
	// Optimization 49: Exponential backoff with max cap (2s, 4s, 8s max)
	backoff := time.Duration(1<<(consecutiveErrors-1)) * time.Second
	const maxBackoff = 8 * time.Second
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	log.Printf("[Agent] LLM error (attempt %d): %v, retrying in %v...", consecutiveErrors, err, backoff)
	// Notify frontend before sleeping so safety timer resets
	w.WriteSSE(map[string]interface{}{
		"type":    "step",
		"step":    "think",
		"content": fmt.Sprintf("LLM 调用失败 (attempt %d/%d)，%v 后重试...", consecutiveErrors, 3, backoff),
	})
	// Keepalive during sleep — send real data events, not just comments
	// Some proxies/CDNs drop connections with only SSE comments
	sleepDone := make(chan struct{})
	startKeepalive(ctx, w, sleepDone, 3*time.Second)
	time.Sleep(backoff)
	close(sleepDone)

	// Smart retry: Add context about what was being attempted to help LLM recover
	retryContext := buildRetryContext(conversation)
	conversation = appendRoleMessage(conversation, "user",
		fmt.Sprintf("[System: LLM call failed with error: %v. %sPlease try again.]", err, retryContext))
	return conversation, consecutiveErrors, nil
}

// buildRetryContext analyzes the conversation to provide context for retry.
// This helps the LLM understand what it was doing and recover from errors.
func buildRetryContext(conversation []map[string]interface{}) string {
	// Find the last assistant message with tool calls
	for i := len(conversation) - 1; i >= 0; i-- {
		msg := conversation[i]
		role, _ := msg["role"].(string)
		if role != "assistant" {
			continue
		}
		toolCalls, ok := msg["tool_calls"].([]LLMToolCall)
		if !ok || len(toolCalls) == 0 {
			continue
		}
		// Build context from last tool calls
		var tools []string
		for _, tc := range toolCalls {
			tools = append(tools, tc.Function.Name)
		}
		return fmt.Sprintf("You were about to call: %s. ", strings.Join(tools, ", "))
	}
	return ""
}
