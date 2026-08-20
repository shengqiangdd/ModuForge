package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════
// Sequential tool execution — write/side-effect tools
// ═══════════════════════════════════════════════════════════════════

// seqToolExecState bundles the mutable state needed by executeSequentialToolBlock
// so it can be passed as a single parameter instead of 20+ individual args.
type seqToolExecState struct {
	r                  *AgentRunner
	ctx                context.Context
	w                  SSEWriter
	cfg                RunConfig
	sessionID          string
	reqProviderID      string
	reqModel           string
	m                  *runMetrics
	modelTier          ModelTier
	toolCache          *toolResultCache
	callBudget         *CallBudget
	fileLock           *FileLock
	stagnationDetector *StagnationDetector
	reflectionLog      *ReflectionLog
	enhancedPlanner    *EnhancedPlanner
	enhancedPlan       *EnhancedPlan
	qualityVerifier    *QualityVerifier
	qualityReports     *[]QualityReport
	qualitySem         chan struct{}
	mu                 *sync.Mutex
	results            *[]toolResult
	startTime          time.Time
	toolRetryFallback  *ToolRetryFallback
	// Mutable tracking state (pointers so mutations are visible to caller)
	writeFileCalled                *bool
	anyWriteCalled                 *bool
	editFileConsecutiveFailures    *int
	answerSent                     *bool
	conversation                   *[]map[string]interface{}
}

// executeSequentialToolBlock runs all sequential (write/side-effect) tool tasks
// one at a time, with permission checks, security checks, error recovery,
// quality verification, and audit logging.
func (state *seqToolExecState) executeSequentialToolBlock(sequentialTasks []toolTask) {
	for _, st := range sequentialTasks {
		// P1-3: Check call budget
		if !state.callBudget.CanCall(st.skillName) {
			budgetMsg := fmt.Sprintf("⚠️ 工具调用预算已用尽 (读取: %d/%d, 写入: %d/%d, 总计: %d/%d)",
				state.callBudget.ReadCalls, state.callBudget.MaxRead,
				state.callBudget.WriteCalls, state.callBudget.MaxWrite,
				state.callBudget.TotalCalls, state.callBudget.MaxTotal)
			log.Printf("[Agent] call budget exceeded: %s", budgetMsg)
			state.w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "skill_result",
				"skill":   st.skillName,
				"content": budgetMsg,
				"blocked": true,
			})
			*state.conversation = state.r.appendToolResult(*state.conversation, state.sessionID, st.tc.ID, budgetMsg)
			continue
		}

		// NEW: Permission check
		allowed, needsConfirm, reason := state.r.permChecker.CheckPermission(st.skillName, state.sessionID)
		if !allowed {
			denyMsg := fmt.Sprintf("❌ 权限拒绝: %s", reason)
			state.r.permChecker.LogDenial(state.sessionID, st.skillName, reason)
			log.Printf("[Agent] permission denied: %s - %s", st.skillName, reason)
			state.w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "skill_result",
				"skill":   st.skillName,
				"content": denyMsg,
				"blocked": true,
			})
			*state.conversation = state.r.appendToolResult(*state.conversation, state.sessionID, st.tc.ID, denyMsg)
			continue
		}

		// NEW: Security check for bash commands
		if st.skillName == "bash" {
			if command, ok := st.skillInput["command"].(string); ok {
				allowed, needsConfirm, riskScore, secMsg := state.r.securityEngine.AuditAndCheck(command, state.sessionID)
				if !allowed {
					log.Printf("[Security] DENIED session=%s risk=%d", state.sessionID, riskScore)
					state.w.WriteSSE(map[string]interface{}{
						"type":       "step",
						"step":       "skill_result",
						"skill":      st.skillName,
						"content":    secMsg,
						"blocked":    true,
						"risk_score": riskScore,
					})
					*state.conversation = state.r.appendToolResult(*state.conversation, state.sessionID, st.tc.ID, secMsg)
					continue
				}
				if needsConfirm {
					notifyData := map[string]interface{}{
						"type":       "step",
						"step":       "security_confirm",
						"skill":      st.skillName,
						"command":    command,
						"risk_score": riskScore,
						"message":    secMsg,
					}
					state.w.WriteSSE(notifyData)
				}
			}
		}

		// Notify frontend with permission info
		notifyData := map[string]interface{}{
			"type":  "step",
			"step":  "skill_call",
			"skill": st.skillName,
			"input": st.skillInput,
		}
		if needsConfirm {
			notifyData["needs_confirm"] = true
			notifyData["confirm_msg"] = state.r.permChecker.GetConfirmationMessage(st.skillName, st.skillInput)
		}
		state.w.WriteSSE(notifyData)

		// Confidence check: auto-pause after 5 consecutive edit_file failures
		if st.skillName == "edit_file" && *state.editFileConsecutiveFailures >= 5 {
			pauseMsg := fmt.Sprintf("⚠️ [Confidence Check] edit_file 已连续失败 %d 次。Agent 可能陷入了无效的编辑循环。请确认是否继续执行，或建议 Agent 换一种方法（例如使用 write_file 重写整个文件）。", *state.editFileConsecutiveFailures)
			log.Printf("[Agent] confidence check triggered: edit_file failed %d times consecutively", *state.editFileConsecutiveFailures)
			state.w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "think",
				"content": pauseMsg,
			})
			*state.conversation = appendRoleMessage(*state.conversation, "system",
				fmt.Sprintf("[System: Confidence check triggered. edit_file has failed %d times consecutively. "+
					"STOP using edit_file. Instead: (1) Use write_file to rewrite the entire file, "+
					"(2) Or analyze the root cause before trying again. Do NOT continue the same approach.]", *state.editFileConsecutiveFailures))
			*state.editFileConsecutiveFailures = 0
		}

		// Keepalive during execution
		skillDone := make(chan struct{})
		startKeepalive(state.ctx, state.w, skillDone, 10*time.Second)

		// Execute with timeout
		toolTimeout := toolTimeoutForName(st.skillName)
		toolCtx, toolCancel := context.WithTimeout(state.ctx, toolTimeout)

		// P0-Fix: Acquire file lock for write operations.
		var lockedPath string
		if st.skillName == "write_file" || st.skillName == "write_file_batch" {
			if path, ok := st.skillInput["path"].(string); ok {
				state.fileLock.Lock(path)
				lockedPath = path
			}
		}

		result, err := state.r.executeSkill(toolCtx, st.skillName, st.skillInput, state.w)
		toolCancel()
		close(skillDone)

		// Release file lock immediately after execution
		if lockedPath != "" {
			state.fileLock.Unlock(lockedPath)
		}

		if toolCtx.Err() == context.DeadlineExceeded {
			result = fmt.Sprintf("⚠️ Tool execution timed out after %v", toolTimeout)
		} else if err != nil && st.skillName == "write_file" {
			log.Printf("[Agent] write_file failed: %v, providing error context to LLM", err)
			result = fmt.Sprintf("Write failed: %v. Please check the path and content, then try again.", err)
		} else if err != nil {
			// P0-2: Classify error and determine recovery strategy
			errCategory := ClassifyError(err.Error())
			state.m.toolConsecutiveErrors[st.skillName]++
			recovery := GetRecoveryStrategy(errCategory, state.m.toolConsecutiveErrors[st.skillName])
			recoveryMsg := GetRecoveryMessageDetailed(recovery, st.skillName, err, errCategory)
			errReason := extractErrorReason(err.Error())
			log.Printf("[Agent] tool %s failed (attempt %d): %v, category=%d(%s), reason=%s, recovery=%d",
				st.skillName, state.m.toolConsecutiveErrors[st.skillName], err, errCategory, errorCategoryName(errCategory), errReason, recovery)

			state.w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "think",
				"content": recoveryMsg,
			})

			switch recovery {
			case RecoveryRetrySame:
				result = fmt.Sprintf("Error [%s]: %v. Please try again.", errReason, err)
			case RecoverySimplifyInput:
				simplified := state.toolRetryFallback.SimplifyTaskInput(st.skillName, st.skillInput)
				retryResult, retryErr := state.r.executeSkill(toolCtx, st.skillName, simplified, state.w)
				if retryErr == nil {
					result = retryResult
					state.m.toolConsecutiveErrors[st.skillName] = 0
				} else {
					result = fmt.Sprintf("Error [%s]: %v (simplified retry also failed: %v)", errReason, err, retryErr)
				}
			case RecoverySwitchModel:
				result = fmt.Sprintf("Error [%s]: %v. Model rate limited, please try a different approach.", errReason, err)
			case RecoveryForceAnswer:
				result = fmt.Sprintf("Error [%s]: %v. Please provide your best answer based on available information.", errReason, err)
				*state.conversation = appendRoleMessage(*state.conversation, "user",
					"[System: Multiple tool failures. Please provide your final answer based on available information.]")
				*state.answerSent = true
			case RecoverySkipTool:
				result = fmt.Sprintf("Skipped tool '%s' [%s]: %v", st.skillName, errReason, err)
			case RecoveryCompactContext:
				result = fmt.Sprintf("Error [%s]: %v. Context will be compacted on next iteration.", errReason, err)
			case RecoveryAbort:
				state.w.WriteSSE(map[string]interface{}{
					"type":  "error",
					"error": fmt.Sprintf("多次执行失败 [%s]，已终止: %v", errReason, err),
				})
				state.w.WriteSSEPlain("[DONE]")
				return
			default:
				result = fmt.Sprintf("Error [%s]: %v", errReason, err)
			}
		} else {
			state.m.toolConsecutiveErrors[st.skillName] = 0 // reset on success
		}

		// P2-1 V2: Track step progress based on tool execution
		if state.enhancedPlan != nil && err != nil {
			currentStep := state.enhancedPlanner.GetCurrentStep(state.enhancedPlan)
			if currentStep != nil {
				state.enhancedPlanner.MarkStepFailed(state.enhancedPlan, err.Error())
				state.w.WriteSSE(map[string]interface{}{
					"type":       "step",
					"step":       "task_progress",
					"step_id":    currentStep.ID,
					"status":     currentStep.Status,
					"content":    currentStep.Description,
					"error":      err.Error(),
					"progress":   state.enhancedPlanner.GetProgress(state.enhancedPlan),
					"files":      currentStep.Files,
				})

				if nextStep := state.enhancedPlanner.GetCurrentStep(state.enhancedPlan); nextStep != nil && nextStep.ID != currentStep.ID {
					stepContext := state.enhancedPlanner.BuildContextMessage(state.enhancedPlan)
					*state.conversation = appendRoleMessage(*state.conversation, "system", stepContext)
					log.Printf("[Agent] after tool failure, advancing to step: %s", nextStep.Description)
				}
			}
		}

		// Track edit_file failures for confidence check
		if st.skillName == "edit_file" {
			*state.anyWriteCalled = true
			isEditError := err != nil || strings.HasPrefix(result, "Error:") || strings.HasPrefix(result, "❌")
			if isEditError {
				*state.editFileConsecutiveFailures++
			} else {
				*state.editFileConsecutiveFailures = 0
			}
		}

		if st.skillName == "write_file" || st.skillName == "write_file_batch" {
			*state.writeFileCalled = true
			*state.anyWriteCalled = true
			state.stagnationDetector.ResetNoWrite()
			if path, ok := st.skillInput["path"].(string); ok {
				state.toolCache.invalidate(path)
				if state.r.fileHashCache != nil {
					state.r.fileHashCache.Invalidate(path)
				}
				if state.r.repoMap != nil {
					if content, ok := st.skillInput["content"].(string); ok {
						state.r.repoMap.UpdateFile(path, content)
					}
				}
				// Cache written content for immediate read-back
				if content, ok := st.skillInput["content"].(string); ok && err == nil {
					state.r.cacheWriteContent(state.sessionID, path, content)
					// Quality verification AFTER write (deferred to background)
					go func(p, c string) {
						state.qualitySem <- struct{}{}
						defer func() { <-state.qualitySem }()
						report := state.qualityVerifier.VerifyFile(p, c)
						state.mu.Lock()
						*state.qualityReports = append(*state.qualityReports, report)
						state.mu.Unlock()
						if len(report.Issues) > 0 {
							log.Printf("[Agent] quality issues in %s: %v", p, report.Issues)
						}
						if report.Score < 40 {
							log.Printf("[Agent] quality score %d < 40 for %s (rollback deferred)", report.Score, p)
						}
					}(path, content)
				}
			}
			// Invalidate build_module cache when source files change
			if state.cfg.ProjectID != "" {
				state.toolCache.InvalidateBuild(state.cfg.ProjectID)
			}
			if state.cfg.ProjectID == "" && strings.HasPrefix(result, "[project_id:") {
				if endIdx := strings.Index(result, "]"); endIdx > 12 {
					autoPID := result[12:endIdx]
					state.cfg.ProjectID = autoPID
					log.Printf("[Agent] write_file auto-created project: %s", autoPID)
					state.w.WriteSSE(map[string]interface{}{
						"type":       "project_created",
						"project_id": autoPID,
					})
				}
			}
			if path, ok := st.skillInput["path"].(string); ok {
				state.w.WriteSSE(map[string]interface{}{
					"type":       "checkpoint",
					"path":       path,
					"can_undo":   true,
				})
			}
		}

		// Track operations
		state.m.toolCallHistory[st.skillName]++
		state.m.totalToolCalls++
		opKey := st.skillName
		if st.skillName == "read_file" || st.skillName == "write_file" {
			if path, ok := st.skillInput["path"].(string); ok {
				opKey = st.skillName + ":" + path
			}
		}
		state.m.uniqueOps[opKey] = true

		// O(1): Update pre-computed unique targets counter for loop detection
		if !strings.Contains(opKey, ":") {
			state.m.uniqueTargetsPerSkill[st.skillName]++
		} else if st.skillName == "read_file" || st.skillName == "write_file" {
			if path, ok := st.skillInput["path"].(string); ok {
				uniqueKey := st.skillName + ":" + path
				if _, exists := state.m.uniqueOps[uniqueKey]; !exists {
					state.m.uniqueTargetsPerSkill[st.skillName]++
				}
			}
		}

		// P2-1: Record reflection
		if err != nil {
			state.reflectionLog.Record(st.skillName, "failure", err.Error(), 0)
		} else {
			state.reflectionLog.Record(st.skillName, "success", "", 0)
		}

		// Audit logging
		state.r.auditLog.RecordToolCall(
			state.sessionID,
			st.skillName,
			st.tc.ID,
			st.skillInput,
			result,
			err == nil,
			time.Since(state.startTime),
			0,
			state.cfg.UserID,
			state.cfg.ProjectID,
		)

		// Truncate large results
		result = truncateResultForModel(result, st.skillName, state.modelTier, state.cfg.MaxResultLen)

		state.w.WriteSSE(map[string]interface{}{
			"type":    "step",
			"step":    "skill_result",
			"skill":   st.skillName,
			"content": result,
		})

		state.mu.Lock()
		appendToolResultToList(state.results, st.tc, result)
		state.mu.Unlock()
	}
}

// ═══════════════════════════════════════════════════════════════════
// Auto-trigger build_module after writes
// ═══════════════════════════════════════════════════════════════════

// autoTriggerBuildIfNeeded triggers build_module if files were written but
// build_module was never called during the run.
func (r *AgentRunner) autoTriggerBuildIfNeeded(
	ctx context.Context,
	w SSEWriter,
	sessionID string,
	cfg RunConfig,
	anyWriteCalled bool,
	buildModuleCalled bool,
) {
	if !anyWriteCalled || buildModuleCalled || cfg.ProjectID == "" {
		return
	}
	log.Printf("[Agent] Auto-trigger: files written but build_module not called, triggering build for project %s", cfg.ProjectID)
	w.WriteSSE(map[string]interface{}{
		"type":  "step",
		"step":  "skill_call",
		"skill": "build_module",
		"input": map[string]interface{}{"project_id": cfg.ProjectID},
	})
	buildTimeout := toolTimeoutForName("build_module")
	buildCtx, buildCancel := context.WithTimeout(ctx, buildTimeout)
	buildResult, buildErr := r.executeSkill(buildCtx, "build_module", map[string]interface{}{
		"project_id": cfg.ProjectID,
	}, w)
	buildCancel()
	if buildErr != nil {
		log.Printf("[Agent] Auto-trigger build_module failed: %v", buildErr)
	} else {
		log.Printf("[Agent] Auto-trigger build_module result: %s", truncateString(buildResult, 200))
		w.WriteSSE(map[string]interface{}{
			"type":    "step",
			"step":    "skill_result",
			"skill":   "build_module",
			"content": buildResult,
		})
	}
}

// handleBuildModuleError processes build_module failures and optionally
// triggers the build healer to inject error context for the LLM.
func (r *AgentRunner) handleBuildModuleError(
	ctx context.Context,
	w SSEWriter,
	sessionID string,
	cfg RunConfig,
	result string,
	err error,
	conversation []map[string]interface{},
) []map[string]interface{} {
	projectPath := ""
	if cfg.ProjectID != "" && r.db != nil {
		var storagePath string
		r.db.QueryRow(`SELECT COALESCE(storage_path,'') FROM projects WHERE id=?`, cfg.ProjectID).Scan(&storagePath)
		projectPath = storagePath
	}
	if projectPath != "" {
		healResult, shouldHeal := r.buildHealer.HandleBuildFailure(ctx, sessionID, result, projectPath, w)
		if shouldHeal && healResult.ContextForLLM != "" {
			conversation = appendRoleMessage(conversation, "system", healResult.ContextForLLM)
			log.Printf("[Agent] build_healer: injected error context for %d diagnostics", len(healResult.Diagnostics))
		} else if healResult.Strategy == HealForceAnswer || healResult.Strategy == HealAbort {
			result = fmt.Sprintf("%s\n\n%s", result, healResult.UserMessage)
		}
	}
	return conversation
}
