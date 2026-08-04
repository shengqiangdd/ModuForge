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
				"content": "📋 上下文过长，已自动压缩，正在重试...",
			})
		}
	}
	// Backoff: 1s, 2s, 4s (exponential)
	backoff := time.Duration(1<<(consecutiveErrors-1)) * time.Second
	log.Printf("[Agent] LLM error (attempt %d): %v, retrying in %v...", consecutiveErrors, err, backoff)
	// Notify frontend before sleeping so safety timer resets
	w.WriteSSE(map[string]interface{}{
		"type":    "step",
		"step":    "think",
		"content": fmt.Sprintf("⚠️ LLM 调用失败 (attempt %d/%d)，%v 后重试...", consecutiveErrors, 3, backoff),
	})
	// Keepalive during sleep — send real data events, not just comments
	// Some proxies/CDNs drop connections with only SSE comments
	sleepDone := make(chan struct{})
	startKeepalive(ctx, w, sleepDone, 3*time.Second)
	time.Sleep(backoff)
	close(sleepDone)
	conversation = appendRoleMessage(conversation, "user",
		fmt.Sprintf("[System: LLM call failed with error: %v. Please try again.]", err))
	return conversation, consecutiveErrors, nil
}
