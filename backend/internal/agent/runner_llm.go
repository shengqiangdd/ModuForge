package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/service"
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
	select {
	case <-time.After(backoff):
		// backoff elapsed, proceed with retry
	case <-ctx.Done():
		close(sleepDone)
		log.Printf("[Agent] retry backoff interrupted by context cancellation")
		return conversation, consecutiveErrors, ctx.Err()
	}
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

// findFallbackProvider queries the DB for another free-tier provider that isn't
// currently circuit-broken. Returns empty strings if no fallback is available.
func (r *AgentRunner) findFallbackProvider(userID, excludeProviderID, currentModel string) (endpoint, apiKey, model, providerID string) {
	if r.db == nil {
		return
	}
	rows, err := r.db.Query(
		`SELECT id, endpoint, api_key, model_id FROM llm_providers
		 WHERE user_id=? AND id != ? AND model_id != ''
		 ORDER BY created_at DESC LIMIT 10`,
		userID, excludeProviderID,
	)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id, ep, key, mdl string
		if err := rows.Scan(&id, &ep, &key, &mdl); err != nil {
			continue
		}
		// Only consider free-tier models as fallbacks
		tier := resolveModelTier(mdl)
		if tier != TierFree {
			continue
		}
		// Skip providers that are also circuit-broken
		if globalCircuitBreaker.IsOpen(id) {
			continue
		}
		return ep, key, mdl, id
	}
	return
}

// handleFinalAnswer processes the final answer from the LLM, including quality
// reports, reflection summaries, plan step advancement, and truncation retries.
// Returns true if the answer was sent (caller should return), or false to continue.
func (r *AgentRunner) handleFinalAnswer(
	ctx context.Context,
	llmResp *LLMResponse,
	conversation []map[string]interface{},
	sessionID string,
	w SSEWriter,
	cfg RunConfig,
	iter int,
	qualityReports []QualityReport,
	qualityVerifier *QualityVerifier,
	reflectionLog *ReflectionLog,
	enhancedPlanner *EnhancedPlanner,
	enhancedPlan *EnhancedPlan,
	writeFileCalled bool,
	anyWriteCalled bool,
	answerSent bool,
) ([]map[string]interface{}, bool, bool, error) {
	answer := cleanAnswer(llmResp.Content)

	// Append quality report if we have quality data
	if len(qualityReports) > 0 {
		qualitySummary := qualityVerifier.GetQualitySummary(qualityReports)
		answer += "\n\n" + qualitySummary
	}

	// Append reflection summary
	reflectionSummary := reflectionLog.GetSummary()
	if reflectionSummary != "无反思记录" {
		answer += "\n\n📊 " + reflectionSummary
	}

	// Task plan step completion
	if enhancedPlan != nil {
		currentStep := enhancedPlanner.GetCurrentStep(enhancedPlan)
		if currentStep != nil {
			if enhancedPlanner.IsStepDone(answer) || iter > 2 {
				currentStep.Status = "completed"
				currentStep.CompletedAt = time.Now().Unix()
				currentStep.Result = truncateString(answer, 200)

				w.WriteSSE(map[string]interface{}{
					"type":       "step",
					"step":       "task_progress",
					"step_id":    currentStep.ID,
					"status":     "completed",
					"content":    currentStep.Description,
					"progress":   enhancedPlanner.GetProgress(enhancedPlan),
					"files":      currentStep.Files,
				})

				nextStep := enhancedPlanner.AdvanceToNextStep(enhancedPlan)
				if nextStep != nil {
					nextStep.Status = "in_progress"
					nextStep.StartedAt = time.Now().Unix()
					w.WriteSSE(map[string]interface{}{
						"type":       "step",
						"step":       "task_progress",
						"step_id":    nextStep.ID,
						"status":     "in_progress",
						"content":    nextStep.Description,
						"progress":   enhancedPlanner.GetProgress(enhancedPlan),
						"files":      nextStep.Files,
					})

					stepContext := enhancedPlanner.BuildContextMessage(enhancedPlan)
					conversation = appendRoleMessage(conversation, "system", stepContext)
					log.Printf("[Agent] advancing to step: %s", nextStep.Description)
				} else {
					log.Printf("[Agent] all enhanced plan steps completed")
				}
			}
		}
	}

	// Auto-retry if answer was truncated by max_tokens
	if llmResp.FinishReason == "length" && iter < cfg.MaxIterations-1 {
		log.Printf("[Agent] answer truncated (finish_reason=length, len=%d), requesting continuation...", len(answer))
		w.WriteSSE(map[string]interface{}{
			"type":    "step",
			"step":    "think",
			"content": "⚠️ 答案被截断，正在请求续写...",
		})
		conversation = appendRoleMessage(conversation, "assistant", answer)
		conversation = appendRoleMessage(conversation, "user",
			"你的回答被截断了。请继续完成上面的回答，从上次中断的地方接着写。不要重复已有内容。")
		return conversation, false, writeFileCalled, nil
	}

	// If answer is garbled, retry once
	if isGarbageOutput(answer) && iter < cfg.MaxIterations-1 {
		debugLog("garbage answer detected in main loop (len=%d), retrying...", len(answer))
		conversation = appendRoleMessage(conversation, "assistant", answer)
		conversation = appendRoleMessage(conversation, "user",
			"你的上一轮回答出现了乱码。请重新开始，使用工具完成任务。从头读取文件并继续实现功能。")
		return conversation, false, writeFileCalled, nil
	}
	if answer == "" {
		answer = "（Agent 未返回内容）"
	}

	// In Plan mode: check if answer includes a plan that needs approval
	if cfg.Mode == ModePlan {
		w.WriteSSE(map[string]interface{}{
			"type":    "step",
			"step":    "answer",
			"content": answer,
			"mode":    "plan",
		})
		return conversation, true, writeFileCalled, nil
	}

	// P0-2: Enhanced declaration-execution consistency check
	if claimsFileModification(answer) && !writeFileCalled && !anyWriteCalled && iter < cfg.MaxIterations-1 {
		log.Printf("[Agent] answer claims modification but no write tool called (writeFileCalled=%v, anyWriteCalled=%v)", writeFileCalled, anyWriteCalled)
		conversation = appendRoleMessage(conversation, "assistant", answer)
		conversation = appendRoleMessage(conversation, "user",
			"你提到修改了文件但没有调用 write_file/edit_file。请立即调用 write_file 保存所有更改，或者直接回答。这是最后的机会。")
		return conversation, false, false, nil
	}
	// P0-2: Additional check — if answer lists files but no writes happened
	if iter >= 2 && !anyWriteCalled && !answerSent {
		containsFilePath := strings.Contains(answer, "src/") || strings.Contains(answer, "lib/") ||
			strings.Contains(answer, ".rs") || strings.Contains(answer, ".go") ||
			strings.Contains(answer, ".js") || strings.Contains(answer, ".ts")
		if containsFilePath {
			log.Printf("[Agent] answer mentions file paths but no writes happened")
			conversation = appendRoleMessage(conversation, "user",
				"你的回答提到了文件路径，但没有实际调用 write_file。请调用 write_file 创建或修改文件，然后给出最终答案。")
			return conversation, false, writeFileCalled, nil
		}
	}

	w.WriteSSE(map[string]interface{}{
		"type":    "step",
		"step":    "answer",
		"content": answer,
	})

	if sessionID != "" {
		r.convStore.Append(sessionID, service.Message{Role: "assistant", Content: answer})
	}
	w.WriteSSEPlain("[DONE]")
	// Auto-store the direct answer as episodic memory
	if llmResp != nil && len(llmResp.Content) > 0 {
		r.autoStoreMemory(cfg.UserID, sessionID, llmResp.Content)
	}
	return conversation, true, writeFileCalled, nil
}
