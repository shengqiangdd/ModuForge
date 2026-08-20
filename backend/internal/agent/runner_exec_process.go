package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
)

// toolResultProcessor runs post-execution analysis on executed tool results:
// stagnation detection, self-reflection, loop detection, and progress tracking.
type toolResultProcessor struct {
	r                  *AgentRunner
	ctx                context.Context
	w                  SSEWriter
	cfg                RunConfig
	sessionID          string
	reqProviderID      string
	reqModel           string
	stagnationDetector *StagnationDetector
	m                  *runMetrics
	progressTracker    *ProgressTracker
}

// process analyzes executed tool results and returns the updated conversation,
// whether an answer was force-sent, and any termination error.
func (p *toolResultProcessor) process(iter int, conversation []map[string]interface{}, results []toolResult, anyWriteCalled, answerSent bool) ([]map[string]interface{}, bool, error) {
	// P0-1: Track stagnation for each tool call
	// P2: Record progress for each tool call
	for _, res := range results {
		// Record progress
		if p.progressTracker != nil {
			var toolInput map[string]interface{}
			if err := json.Unmarshal([]byte(res.tc.Function.Arguments), &toolInput); err == nil {
				p.progressTracker.RecordToolCall(res.tc.Function.Name, toolInput, res.result, !strings.HasPrefix(res.result, "Error:"))
			}
		}

		// Check stagnation
		var toolInput map[string]interface{}
		if err := json.Unmarshal([]byte(res.tc.Function.Arguments), &toolInput); err != nil {
			debugLog("tool args unmarshal failed for stagnation check (%s): %v", res.tc.Function.Name, err)
		}
		if stagnant, reason := p.stagnationDetector.RecordToolCall(res.tc.Function.Name, toolInput, res.result); stagnant {
			log.Printf("[Agent] stagnation detected: %s", reason)
			p.w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "think",
				"content": fmt.Sprintf("⚠️ %s", reason),
			})
			// Force answer by appending a user message asking for summary
			conversation = appendRoleMessage(conversation, "user",
				fmt.Sprintf("[System: %s. Please provide a summary of what you've done so far and any remaining work.]", reason))
			answerSent = true
			break
		}

		// Track write_file calls
		if res.tc.Function.Name == "write_file" {
			anyWriteCalled = true
			p.stagnationDetector.ResetNoWrite()
		}

		// Detect build_module completion
		if res.tc.Function.Name == "build_module" {
			isError := strings.HasPrefix(res.result, "Error:") || strings.HasPrefix(res.result, "❌")
			buildReady := strings.Contains(res.result, `"build_ready":true`)

			// Parse and emit build progress events to frontend
			emitBuildProgress(p.w, res.result)

			if buildReady || !isError {
				// Build succeeded - inject completion prompt with next steps
				log.Printf("[Agent] build_module completed successfully (build_ready=%v), injecting completion prompt", buildReady)
				p.w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "think",
					"content": "✅ Build completed successfully.",
				})
				// Reset read_file counter to give Agent a chance to verify if needed
				p.m.toolCallHistory["read_file"] = 0
				p.m.toolConsecutiveIdentical = make(map[string]int)

				// Inject prompt for next steps
				nextStepsPrompt := buildNextStepsPrompt(res.result)
				conversation = appendRoleMessage(conversation, "system", nextStepsPrompt)
			} else {
				// Build failed - inject auto-fix prompt with specific error guidance
				log.Printf("[Agent] build_module failed, injecting auto-fix prompt")
				fixPrompt := buildAutoFixPrompt(res.result)
				conversation = appendRoleMessage(conversation, "system", fixPrompt)
			}
		}
	}

	// Check no-write stagnation AND hard read cap
	if !anyWriteCalled && !answerSent {
		// Hard cap: total reads across ALL iterations
		totalReads := p.m.toolCallHistory["read_file"] + p.m.toolCallHistory["grep_search"] + p.m.toolCallHistory["glob_search"] + p.m.toolCallHistory["list_dir"]
		if totalReads >= 30 {
			log.Printf("[Agent] hard read cap reached: %d total reads, 0 writes", totalReads)
			return conversation, true, p.r.forceAnswer(p.ctx, conversation, p.w, p.sessionID, p.cfg, p.reqProviderID, p.reqModel,
				fmt.Sprintf("CRITICAL: You have made %d read/list/search calls without a single write/edit. "+
					"You are an ENGINEER, not a REVIEWER. You MUST now: "+
					"(1) Use edit_file to make targeted changes to build.sh (add -Os -flto -s flags), or "+
					"(2) Use write_file to rewrite build.sh with the optimized flags, or "+
					"(3) Stop all tools and provide your final answer. "+
					"DO NOT read any more files. Start writing code NOW.", totalReads))
		}
		if p.stagnationDetector.RecordNoWrite() {
			log.Printf("[Agent] no-write stagnation: %d iterations without write_file", p.stagnationDetector.consecutiveNoWrite)
			p.w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "think",
				"content": "⚠️ 已连续5轮未执行写入操作，强制要求：你必须立即使用 edit_file 或 write_file 修改代码，或者停止使用工具并提供最终答案。",
			})
			conversation = appendRoleMessage(conversation, "user",
				"[System: CRITICAL - You have not written any files for 5 consecutive iterations. You MUST either: (1) Use edit_file or write_file to make code changes NOW, or (2) Stop using all tools and provide your final answer immediately. Do NOT read any more files.]")
			answerSent = true
		}
	}

	if answerSent {
		return conversation, true, nil
	}

	// Add all results to conversation in order
	// Optimization 18: Merge consecutive write_file results to save tokens
	conversation = p.r.appendResultsToConversation(conversation, p.sessionID, results, p.m.uniqueOps, p.m.totalToolCalls)

	// Optimization 24: Self-reflection — track failures and inject diagnostic prompts
	// Update error tracking for each tool result
	for _, res := range results {
		skillName := res.tc.Function.Name
		isError := strings.HasPrefix(res.result, "Error:") || strings.HasPrefix(res.result, "❌") ||
			strings.HasPrefix(res.result, "⚠️") || strings.Contains(res.result, "failed")

		if isError {
			p.m.toolConsecutiveErrors[skillName]++
			if p.m.toolConsecutiveErrors[skillName] >= 3 {
				// Same tool failed 3 times in a row — inject reflection prompt
				// Extract error category for structured logging
				errCat := ClassifyError(res.result)
				errReason := extractErrorReason(res.result)
				diagnostic := fmt.Sprintf(
					"⚠️ [Self-Reflection] The tool '%s' has failed %d times consecutively. "+
						"Error category: %s (%s). Recent errors: %s. "+
						"STOP using this tool with the same approach. Instead: "+
						"(1) Analyze WHY it's failing, (2) Try a completely different approach, "+
						"(3) If stuck, use write_file to create the file directly.",
					skillName, p.m.toolConsecutiveErrors[skillName],
					errorCategoryName(errCat), errReason, res.result)
				log.Printf("[Agent] self-reflection triggered: %s failed %d times, category=%s, reason=%s",
					skillName, p.m.toolConsecutiveErrors[skillName], errorCategoryName(errCat), errReason)
				conversation = appendRoleMessage(conversation, "system", diagnostic)
				p.w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "think",
					"content": fmt.Sprintf("🔄 检测到 %s 连续失败 %d 次 [%s/%s]，已注入反思提示", skillName, p.m.toolConsecutiveErrors[skillName], errorCategoryName(errCat), errReason),
				})
				p.m.toolConsecutiveErrors[skillName] = 0 // reset after injection
			}
		} else {
			p.m.toolConsecutiveErrors[skillName] = 0 // success resets counter
		}

		// Track consecutive identical tool calls (same skill + same input)
		inputKey := skillName
		if len(res.tc.Function.Arguments) > 0 {
			arg := res.tc.Function.Arguments
			if len(arg) > 100 {
				arg = arg[:100]
			}
			inputKey = skillName + ":" + arg
		}
		if prev, ok := p.m.toolLastResults[inputKey]; ok && prev == res.result {
			p.m.toolConsecutiveIdentical[inputKey]++
			if p.m.toolConsecutiveIdentical[inputKey] >= 3 {
				// Same exact tool call with same result 3 times — force answer
				log.Printf("[Agent] early termination: '%s' called %d times with identical result", skillName, p.m.toolConsecutiveIdentical[inputKey])
				return conversation, true, p.r.forceAnswer(p.ctx, conversation, p.w, p.sessionID, p.cfg, p.reqProviderID, p.reqModel,
					fmt.Sprintf("You've called '%s' with identical parameters %d times and got the same result each time. This is a dead end. You must stop using tools and provide your final answer.", skillName, p.m.toolConsecutiveIdentical[inputKey]))
			}
		} else {
			p.m.toolConsecutiveIdentical[inputKey] = 0
		}
		p.m.toolLastResults[inputKey] = res.result
	}

	// Read-only loop detection: if Agent only calls read_file/grep/glob without any write/edit, inject reminder
	readOnlyCount := p.m.toolCallHistory["read_file"] + p.m.toolCallHistory["grep_search"] + p.m.toolCallHistory["glob_search"] + p.m.toolCallHistory["list_dir"]
	writeCount := p.m.toolCallHistory["write_file"] + p.m.toolCallHistory["edit_file"] + p.m.toolCallHistory["write_file_batch"]
	skipLoopDetection := false

	// Phase 1 (iter >= 2, readOnlyCount >= 6): Inject warning, give one chance
	// Phase 2 (iter >= 3, readOnlyCount >= 10): Force answer immediately
	if readOnlyCount >= 10 && writeCount == 0 && iter >= 3 {
		// Second warning: Agent ignored the first warning and still only reads. Force answer now.
		log.Printf("[Agent] read-only loop forced termination: %d reads, %d writes, %d iterations", readOnlyCount, writeCount, iter)
		return conversation, true, p.r.forceAnswer(p.ctx, conversation, p.w, p.sessionID, p.cfg, p.reqProviderID, p.reqModel,
			fmt.Sprintf("CRITICAL: You have called read tools %d times across %d iterations without a single write/edit. "+
				"You MUST stop reading and start writing code NOW. Use edit_file or write_file to make changes, "+
				"then call build_module to verify. Do NOT read any more files.", readOnlyCount, iter))
	}
	if readOnlyCount >= 6 && writeCount == 0 && iter >= 2 {
		diagnostic := fmt.Sprintf(
			"⚠️ [Read-Only Loop] You have called read tools %d times without any write/edit operations. "+
				"You have already read enough code. NOW you MUST: "+
				"(1) Use edit_file for targeted fixes, or write_file to rewrite files completely. "+
				"(2) Then call build_module to verify. "+
				"DO NOT read any more files. Start writing code immediately. "+
				"If you do not write/edit in the next iteration, I will force you to provide a final answer.",
			readOnlyCount)
		log.Printf("[Agent] read-only loop detected: %d reads, %d writes", readOnlyCount, writeCount)
		conversation = appendRoleMessage(conversation, "system", diagnostic)
		p.w.WriteSSE(map[string]interface{}{
			"type":    "step",
			"step":    "think",
			"content": fmt.Sprintf("🔄 检测到只读循环（%d 次读取，0 次写入），已注入编辑提醒，下一轮仍无写入将强制回答", readOnlyCount),
		})
		skipLoopDetection = true // Give Agent one last chance before forcing answer
	}

	// Global error cap: if total consecutive errors across all skills >= 5, force answer
	totalConsecutiveErrors := 0
	for _, count := range p.m.toolConsecutiveErrors {
		totalConsecutiveErrors += count
	}
	if totalConsecutiveErrors >= 5 {
		log.Printf("[Agent] early termination: total consecutive errors across all skills = %d", totalConsecutiveErrors)
		return conversation, true, p.r.forceAnswer(p.ctx, conversation, p.w, p.sessionID, p.cfg, p.reqProviderID, p.reqModel,
			"Multiple tools have failed consecutively. Stop using tools and provide your final answer based on what you've learned so far.")
	}

	// Smart loop detection (skip if read-only reminder was just injected)
	// O(1): Use pre-computed uniqueTargetsPerSkill for loop detection
	if !skipLoopDetection {
		if reason := detectLoop(p.m.toolCallHistory, p.m.uniqueOps, p.m.totalToolCalls,
			p.m.uniqueTargetsPerSkill); reason != "" {
			debugLog("loop detected: %s", reason)
			return conversation, true, p.r.forceAnswer(p.ctx, conversation, p.w, p.sessionID, p.cfg, p.reqProviderID, p.reqModel, reason)
		}
	}

	return conversation, false, nil
}

// emitBuildProgress parses [BUILD_PROGRESS] markers from build output and emits SSE events.
func emitBuildProgress(w SSEWriter, buildOutput string) {
	lines := strings.Split(buildOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[BUILD_PROGRESS]") {
			continue
		}

		// Parse key=value pairs from [BUILD_PROGRESS] phase=xxx status=xxx ...
		attrs := make(map[string]string)
		parts := strings.Fields(line)
		for _, part := range parts {
			if kv := strings.SplitN(part, "=", 2); len(kv) == 2 {
				attrs[kv[0]] = kv[1]
			}
		}

		phase := attrs["phase"]
		status := attrs["status"]

		// Map phase names to human-readable labels
		phaseLabel := phase
		switch phase {
		case "init":
			phaseLabel = "初始化"
		case "incremental":
			phaseLabel = "增量检查"
		case "validate":
			phaseLabel = "结构验证"
		case "compile":
			phaseLabel = "源码编译"
		case "shellcheck":
			phaseLabel = "脚本检查"
		case "package":
			phaseLabel = "打包压缩"
		}

		content := fmt.Sprintf("🔨 %s: %s", phaseLabel, status)
		if pct, ok := attrs["pct"]; ok {
			content += fmt.Sprintf(" (%s%%)", pct)
		}
		if reason, ok := attrs["reason"]; ok {
			content += fmt.Sprintf(" [%s]", reason)
		}

		w.WriteSSE(map[string]interface{}{
			"type":    "step",
			"step":    "build_progress",
			"phase":   phase,
			"status":  status,
			"content": content,
		})
	}
}

// buildNextStepsPrompt generates a prompt for what to do after a successful build.
func buildNextStepsPrompt(buildOutput string) string {
	hasZip := strings.Contains(buildOutput, ".zip")

	parts := []string{
		"✅ Build succeeded! Your module is ready.",
	}

	if hasZip {
		parts = append(parts, "📦 The module ZIP has been created and is ready for distribution.")
	}

	parts = append(parts, "")
	parts = append(parts, "Suggested next steps:")
	parts = append(parts, "1. Run tests if test scripts exist (e.g., test.sh)")
	parts = append(parts, "2. Verify the module installs correctly on a device")
	parts = append(parts, "3. If everything looks good, provide your final answer summarizing what was built")

	parts = append(parts, "")
	parts = append(parts, "💡 If you need to make changes, use edit_file, then call build_module again to rebuild.")

	return strings.Join(parts, "\n")
}

// buildAutoFixPrompt generates an auto-fix prompt that instructs the Agent to fix errors.
func buildAutoFixPrompt(buildOutput string) string {
	errorTypes := classifyBuildErrors(buildOutput)
	parsedErrors := extractErrorLocations(buildOutput)

	parts := []string{
		"❌ Build failed. You MUST fix the errors automatically.",
		"",
		"📋 AUTO-FIX WORKFLOW:",
		"  1. Analyze the errors below",
		"  2. For each error with a file:line location:",
		"     a. Use read_file to read the specific file around that line",
		"     b. Use edit_file to fix the issue",
		"  3. After fixing ALL errors, call build_module to verify",
		"  4. If new errors appear, repeat steps 2-3",
		"",
	}

	if len(parsedErrors) > 0 {
		parts = append(parts, fmt.Sprintf("Found %d specific error(s) to fix:", len(parsedErrors)))
		parts = append(parts, "")
		for i, pe := range parsedErrors {
			loc := ""
			if pe.file != "" {
				loc = fmt.Sprintf(" in %s:%d", pe.file, pe.line)
			}
			parts = append(parts, fmt.Sprintf("  %d. [%s]%s: %s", i+1, pe.errorType, loc, pe.message))
		}
		parts = append(parts, "")
	}

	if len(errorTypes) > 0 {
		parts = append(parts, fmt.Sprintf("Error categories: %s", strings.Join(errorTypes, ", ")))
		parts = append(parts, "")

		if containsErrorType(errorTypes, "missing_import") {
			parts = append(parts, "🔧 Fix missing imports:")
			parts = append(parts, "  - Add the missing import statement to the file")
			parts = append(parts, "  - For Go: also run 'go mod tidy' if it's a new dependency")
			parts = append(parts, "  - For Rust: add the dependency to Cargo.toml")
			parts = append(parts, "")
		}
		if containsErrorType(errorTypes, "undefined") {
			parts = append(parts, "🔧 Fix undefined references:")
			parts = append(parts, "  - Check if the variable/function is declared in the same file or package")
			parts = append(parts, "  - Check if it's imported from the correct package")
			parts = append(parts, "  - Check for typos")
			parts = append(parts, "")
		}
		if containsErrorType(errorTypes, "type_mismatch") {
			parts = append(parts, "🔧 Fix type mismatches:")
			parts = append(parts, "  - Check the expected type from the function signature")
			parts = append(parts, "  - Add type conversion if needed")
			parts = append(parts, "")
		}
		if containsErrorType(errorTypes, "syntax") {
			parts = append(parts, "🔧 Fix syntax errors:")
			parts = append(parts, "  - Check for missing brackets, semicolons, or keywords")
			parts = append(parts, "  - Verify language-specific syntax rules")
			parts = append(parts, "")
		}
	}

	parts = append(parts, "⚠️ CRITICAL: Do NOT just read files. You MUST use edit_file to fix each error.")
	parts = append(parts, "⚠️ After fixing, ALWAYS call build_module to verify your fixes worked.")
	parts = append(parts, "⚠️ Do NOT give up. Keep fixing until the build succeeds.")

	return strings.Join(parts, "\n")
}

// parsedError holds an extracted error location from build output.
type parsedError struct {
	file      string
	line      int
	message   string
	errorType string
}

// extractErrorLocations parses build output to find specific error locations.
func extractErrorLocations(output string) []parsedError {
	var errors []parsedError
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Go error: file:line:col: message
		if m := regexp.MustCompile(`(.+\.go):(\d+):\d+:\s*(.+)`).FindStringSubmatch(line); len(m) > 3 {
			errType := "unknown"
			msg := strings.ToLower(m[3])
			switch {
			case strings.Contains(msg, "undefined") || strings.Contains(msg, "undeclared"):
				errType = "undefined"
			case strings.Contains(msg, "cannot use") || strings.Contains(msg, "type"):
				errType = "type_mismatch"
			case strings.Contains(msg, "syntax error"):
				errType = "syntax"
			case strings.Contains(msg, "cannot find package"):
				errType = "missing_import"
			}
			errors = append(errors, parsedError{
				file:      m[1],
				line:      atoi(m[2]),
				message:   m[3],
				errorType: errType,
			})
			continue
		}

		// Rust error: error[E0XXX]: message --> file:line:col
		if m := regexp.MustCompile(`error\[(E\d+)\]:\s*(.+)`).FindStringSubmatch(line); len(m) > 2 {
			errType := "unknown"
			code := m[1]
			switch {
			case strings.Contains(code, "0412") || strings.Contains(code, "0432") || strings.Contains(code, "0433"):
				errType = "missing_import"
			case strings.Contains(code, "0596") || strings.Contains(code, "0599") || strings.Contains(code, "0609"):
				errType = "undefined"
			case strings.Contains(code, "0308") || strings.Contains(code, "0305"):
				errType = "type_mismatch"
			case strings.Contains(code, "0001") || strings.Contains(code, "0002") || strings.Contains(code, "0003"):
				errType = "syntax"
			}
			errors = append(errors, parsedError{
				message:   m[2],
				errorType: errType,
			})
			continue
		}

		// C++ error: file:line:col: error: message
		if m := regexp.MustCompile(`(.+\.(cpp|c|cc|cxx|h)):(\d+):\d+:\s*(?:error|warning):\s*(.+)`).FindStringSubmatch(line); len(m) > 4 {
			errType := "unknown"
			msg := strings.ToLower(m[4])
			switch {
			case strings.Contains(msg, "undeclared") || strings.Contains(msg, "was not declared"):
				errType = "undefined"
			case strings.Contains(msg, "no matching function") || strings.Contains(msg, "cannot convert"):
				errType = "type_mismatch"
			case strings.Contains(msg, "expected") || strings.Contains(msg, "unexpected"):
				errType = "syntax"
			case strings.Contains(msg, "fatal error: ") || strings.Contains(msg, "no such file"):
				errType = "missing_import"
			}
			errors = append(errors, parsedError{
				file:      m[1],
				line:      atoi(m[3]),
				message:   m[4],
				errorType: errType,
			})
		}
	}

	return errors
}

// atoi is a simple string-to-int converter.
func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}

// classifyBuildErrors extracts error types from build output.
func classifyBuildErrors(output string) []string {
	seen := make(map[string]bool)
	var types []string

	checkAndAdd := func(t string) {
		if !seen[t] {
			seen[t] = true
			types = append(types, t)
		}
	}

	outputLower := strings.ToLower(output)

	if strings.Contains(outputLower, "cannot find package") || strings.Contains(outputLower, "no required module") {
		checkAndAdd("missing_import")
	}
	if strings.Contains(outputLower, "undefined") || strings.Contains(outputLower, "undeclared") || strings.Contains(outputLower, "was not declared") {
		checkAndAdd("undefined")
	}
	if strings.Contains(outputLower, "cannot use") || strings.Contains(outputLower, "type mismatch") || (strings.Contains(outputLower, "type ") && strings.Contains(outputLower, "expected")) {
		checkAndAdd("type_mismatch")
	}
	if strings.Contains(outputLower, "syntax error") || (strings.Contains(outputLower, "expected") && strings.Contains(outputLower, "unexpected")) {
		checkAndAdd("syntax")
	}
	if strings.Contains(outputLower, "linker") || strings.Contains(outputLower, "undefined reference") || strings.Contains(outputLower, "cannot find -l") {
		checkAndAdd("linker")
	}
	if strings.Contains(outputLower, "timed out") || strings.Contains(outputLower, "timeout") {
		checkAndAdd("timeout")
	}

	return types
}

// containsErrorType checks if a string slice contains a specific error type.
func containsErrorType(types []string, target string) bool {
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}
