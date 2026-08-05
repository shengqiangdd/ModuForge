package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"strings"
)

// toolTask is a validated tool call ready for execution.
type toolTask struct {
	tc         LLMToolCall
	skillName  string
	skillInput map[string]interface{}
	parallel   bool
}

// runMetrics tracks tool-call accounting that accumulates across Run iterations.
type runMetrics struct {
	toolCallHistory          map[string]int
	uniqueOps                map[string]bool
	totalToolCalls           int
	toolConsecutiveErrors    map[string]int    // skill name -> consecutive error count
	toolLastResults          map[string]string // skill name -> last result (for pattern detection)
	toolConsecutiveIdentical map[string]int    // skill name -> consecutive identical calls
}

// preparedToolCalls is the result of planning a batch of tool calls.
type preparedToolCalls struct {
	conversation     []map[string]interface{}
	deduped          []toolTask
	seen             map[uint64]int // dedupHash -> index in deduped slice
	skippedToolCalls []LLMToolCall
	parallelTasks    []toolTask
	sequentialTasks  []toolTask
}

// prepareToolTasks validates, deduplicates, analyzes dependencies, and groups a
// batch of tool calls into parallel-safe and sequential execution sets.
func (r *AgentRunner) prepareToolTasks(llmResp *LLMResponse, conversation []map[string]interface{}, sessionID string, w SSEWriter, cfg RunConfig, readOnlySkills map[string]bool, maxReadFilePerTurn, maxWriteFilePerTurn int) preparedToolCalls {
	readFileCount := 0
	writeFileCount := 0
	// Optimization 1: Parallel tool execution for read-only tools
	// Separate tools into parallel-safe (read-only) and sequential (write/side-effect)
	var tasks []toolTask
	for _, tc := range llmResp.ToolCalls {
		skillName := tc.Function.Name
		var skillInput map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &skillInput); err != nil {
			debugLog("tool args unmarshal failed for %s: %v", skillName, err)
		}

		// Plan mode: block write operations
		if cfg.Mode == ModePlan && !readOnlySkills[skillName] {
			blocked := fmt.Sprintf("⚠️ Plan 模式下无法执行 %s。请切换到 Act 模式后再执行写入操作。", skillName)
			w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "skill_result",
				"skill":   skillName,
				"content": blocked,
				"blocked": true,
			})
			conversation = r.appendToolResult(conversation, sessionID, tc.ID, blocked)
			continue
		}

		// read_file safety limit — prevents SSE timeout from too many parallel reads
		if skillName == "read_file" {
			readFileCount++
			log.Printf("[Agent] read_file limit check: count=%d max=%d", readFileCount, maxReadFilePerTurn)
			if readFileCount > maxReadFilePerTurn {
				blocked := fmt.Sprintf("⚠️ 安全限制：单轮最多允许 %d 次 read_file 调用，已达到上限。请先分析已有文件内容，下一轮再继续读取。", maxReadFilePerTurn)
				log.Printf("[Agent] read_file limit reached (%d), blocking further reads", maxReadFilePerTurn)
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "skill_result",
					"skill":   skillName,
					"content": blocked,
					"blocked": true,
				})
				conversation = r.appendToolResult(conversation, sessionID, tc.ID, blocked)
				continue
			}
		}

		// write_file safety limit
		if skillName == "write_file" {
			writeFileCount++
			if writeFileCount > maxWriteFilePerTurn {
				blocked := fmt.Sprintf("⚠️ 安全限制：单轮最多允许 %d 次 write_file 调用，已达到上限。请在下一轮继续修改。", maxWriteFilePerTurn)
				log.Printf("[Agent] write_file limit reached (%d), blocking further writes", maxWriteFilePerTurn)
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "skill_result",
					"skill":   skillName,
					"content": blocked,
					"blocked": true,
				})
				conversation = r.appendToolResult(conversation, sessionID, tc.ID, blocked)
				continue
			}
		}

		// Auto-inject project_id and user_id
		if skillInput == nil {
			skillInput = make(map[string]interface{})
		}
		if cfg.ProjectID != "" {
			if _, exists := skillInput["project_id"]; !exists {
				skillInput["project_id"] = cfg.ProjectID
			}
		}
		if cfg.UserID != "" {
			skillInput["user_id"] = cfg.UserID
		}

		// Normalize skill input
		skillInput = normalizeSkillInput(skillInput)

		// Validate required parameters
		if missing := validateRequiredParams(skillName, skillInput); missing != "" {
			log.Printf("[Agent] missing required param for %s: %s", skillName, missing)
			paramErr := fmt.Sprintf("❌ Missing required parameter(s): %s. Check the tool schema and provide all required fields.", missing)
			conversation = r.appendToolResult(conversation, sessionID, tc.ID, paramErr)
			w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "skill_result",
				"skill":   skillName,
				"content": paramErr,
			})
			continue
		}

		// Determine if this tool can run in parallel (read-only, no side effects)
		parallelSafe := readOnlySkills[skillName] && skillName != "build_module"
		tasks = append(tasks, toolTask{tc: tc, skillName: skillName, skillInput: skillInput, parallel: parallelSafe})
	}

	// Optimization 6: Deduplicate identical tool calls (same name + arguments).
	// Weak models sometimes issue the same tool call multiple times in one iteration.
	// Uses FNV hash for fast dedup key generation instead of full string comparison.
	originalTasks := tasks
	seen := make(map[uint64]int) // dedupHash -> index in deduped slice
	var deduped []toolTask
	var skippedToolCalls []LLMToolCall
	for _, t := range tasks {
		// Fast hash: FNV-1a on skill name + arguments
		h := fnv.New64a()
		h.Write([]byte(t.skillName))
		h.Write([]byte{':'})
		h.Write([]byte(t.tc.Function.Arguments))
		dedupHash := h.Sum64()
		if idx, exists := seen[dedupHash]; exists {
			debugLog("dedup: skipping duplicate tool call %s (same as task %d)", t.skillName, idx)
			skippedToolCalls = append(skippedToolCalls, t.tc)
			continue
		}
		seen[dedupHash] = len(deduped)
		deduped = append(deduped, t)
	}
	if len(deduped) < len(originalTasks) {
		log.Printf("[Agent] dedup: removed %d/%d duplicate tool calls", len(originalTasks)-len(deduped), len(originalTasks))
	}
	tasks = deduped

	// NEW: Analyze dependencies for better parallelism
	// Skip for single tool call (no dependencies possible)
	if len(tasks) > 1 {
		r.depGraph.Reset()
		for _, t := range tasks {
			filePath := ""
			if p, ok := t.skillInput["path"].(string); ok {
				filePath = p
			}
			r.depGraph.AddToolCall(t.tc.ID, t.skillName, filePath, !readOnlySkills[t.skillName])
		}
		r.depGraph.AnalyzeAndLink()
		depLayers := r.depGraph.GetExecutionLayers()
		if len(depLayers) > 1 {
			log.Printf("[Agent] dependency analysis: %d layers, grouping: %s", len(depLayers), r.depGraph.GetParallelGroup())
			w.WriteSSE(map[string]interface{}{
				"type":          "step",
				"step":          "dependency_analysis",
				"layers":        len(depLayers),
				"parallel_info": r.depGraph.GetParallelGroup(),
			})
		}
	}

	// Group tasks: read-only tools run in parallel; write/edit tools run
	// sequentially to avoid same-file conflicts. Non-write tools are also
	// promoted to the parallel set so each tool executes exactly once.
	var parallelTasks []toolTask
	var sequentialTasks []toolTask
	for _, t := range tasks {
		if t.parallel || (t.skillName != "write_file" && t.skillName != "edit_file") {
			parallelTasks = append(parallelTasks, t)
		} else {
			sequentialTasks = append(sequentialTasks, t)
		}
	}

	return preparedToolCalls{
		conversation:     conversation,
		deduped:          deduped,
		seen:             seen,
		skippedToolCalls: skippedToolCalls,
		parallelTasks:    parallelTasks,
		sequentialTasks:  sequentialTasks,
	}
}

// toolResultProcessor runs post-execution analysis on executed tool results:
// stagnation detection, self-reflection, and loop detection.
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
}

// process analyzes executed tool results and returns the updated conversation,
// whether an answer was force-sent, and any termination error.
func (p *toolResultProcessor) process(iter int, conversation []map[string]interface{}, results []toolResult, anyWriteCalled, answerSent bool) ([]map[string]interface{}, bool, error) {
	// P0-1: Track stagnation for each tool call
	for _, res := range results {
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
				// Build failed - inject intelligent fix prompt
				log.Printf("[Agent] build_module failed, injecting intelligent fix prompt")
				fixPrompt := buildErrorFixPrompt(res.result)
				conversation = appendRoleMessage(conversation, "system", fixPrompt)
			}
		}
	}

	// Check no-write stagnation
	if !anyWriteCalled && !answerSent {
		if p.stagnationDetector.RecordNoWrite() {
			log.Printf("[Agent] no-write stagnation: %d iterations without write_file", p.stagnationDetector.consecutiveNoWrite)
			p.w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "think",
				"content": "⚠️ 已连续多轮未执行写入操作，请直接给出当前进度的答案或执行必要的文件修改",
			})
			conversation = appendRoleMessage(conversation, "user",
				"[System: You have not written any files for multiple iterations. Please either write the necessary files or provide your final answer based on what you've read so far.]")
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
				diagnostic := fmt.Sprintf(
					"⚠️ [Self-Reflection] The tool '%s' has failed %d times consecutively. "+
						"Recent errors: %s. "+
						"STOP using this tool with the same approach. Instead: "+
						"(1) Analyze WHY it's failing, (2) Try a completely different approach, "+
						"(3) If stuck, use write_file to create the file directly.",
					skillName, p.m.toolConsecutiveErrors[skillName], res.result)
				log.Printf("[Agent] self-reflection triggered: %s failed %d times", skillName, p.m.toolConsecutiveErrors[skillName])
				conversation = appendRoleMessage(conversation, "system", diagnostic)
				p.w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "think",
					"content": fmt.Sprintf("🔄 检测到 %s 连续失败 %d 次，已注入反思提示", skillName, p.m.toolConsecutiveErrors[skillName]),
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
	if readOnlyCount >= 6 && writeCount == 0 && iter >= 2 {
		diagnostic := fmt.Sprintf(
			"⚠️ [Read-Only Loop] You have called read tools %d times without any write/edit operations. "+
				"You have already read enough code. NOW you MUST: "+
				"(1) Use edit_file for targeted fixes, or write_file to rewrite files completely. "+
				"(2) Then call build_module to verify. "+
				"DO NOT read any more files. Start writing code immediately.",
			readOnlyCount)
		log.Printf("[Agent] read-only loop detected: %d reads, %d writes", readOnlyCount, writeCount)
		conversation = appendRoleMessage(conversation, "system", diagnostic)
		p.w.WriteSSE(map[string]interface{}{
			"type":    "step",
			"step":    "think",
			"content": fmt.Sprintf("🔄 检测到只读循环（%d 次读取，0 次写入），已注入编辑提醒", readOnlyCount),
		})
		skipLoopDetection = true // Give Agent a chance to respond before forcing answer
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
	if !skipLoopDetection {
		if reason := detectLoop(p.m.toolCallHistory, p.m.uniqueOps, p.m.totalToolCalls); reason != "" {
			debugLog("loop detected: %s", reason)
			return conversation, true, p.r.forceAnswer(p.ctx, conversation, p.w, p.sessionID, p.cfg, p.reqProviderID, p.reqModel, reason)
		}
	}

	return conversation, false, nil
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

// buildErrorFixPrompt generates an intelligent fix prompt based on build errors.
func buildErrorFixPrompt(buildOutput string) string {
	parts := []string{
		"❌ Build failed. Here's how to fix it:",
		"",
	}

	// Parse error types from build output
	errorTypes := classifyBuildErrors(buildOutput)

	if len(errorTypes) == 0 {
		parts = append(parts, "Could not classify the specific error type.")
		parts = append(parts, "📋 General approach:")
		parts = append(parts, "  1. Read the error message above carefully")
		parts = append(parts, "  2. Identify the file and line number mentioned")
		parts = append(parts, "  3. Use read_file to read that specific file")
		parts = append(parts, "  4. Use edit_file to fix the issue")
		parts = append(parts, "  5. Call build_module again to verify")
	} else {
		parts = append(parts, fmt.Sprintf("Detected %d error type(s): %s", len(errorTypes), strings.Join(errorTypes, ", ")))
		parts = append(parts, "")

		if containsErrorType(errorTypes, "missing_import") {
			parts = append(parts, "🔧 Missing import/package:")
			parts = append(parts, "  - Check the imports section of the file")
			parts = append(parts, "  - For Go: add the import and run 'go mod tidy'")
			parts = append(parts, "  - For Rust: check Cargo.toml dependencies")
			parts = append(parts, "")
		}
		if containsErrorType(errorTypes, "undefined") {
			parts = append(parts, "🔧 Undefined variable/function:")
			parts = append(parts, "  - Check if the variable is declared")
			parts = append(parts, "  - Check if the function exists in the imported package")
			parts = append(parts, "  - Check for typos in variable/function names")
			parts = append(parts, "")
		}
		if containsErrorType(errorTypes, "type_mismatch") {
			parts = append(parts, "🔧 Type mismatch:")
			parts = append(parts, "  - Check function signatures and expected types")
			parts = append(parts, "  - Ensure variables match the expected types")
			parts = append(parts, "  - Check for type conversions needed")
			parts = append(parts, "")
		}
		if containsErrorType(errorTypes, "syntax") {
			parts = append(parts, "🔧 Syntax error:")
			parts = append(parts, "  - Check brackets, parentheses, and semicolons")
			parts = append(parts, "  - Verify Go/C++/Rust syntax rules")
			parts = append(parts, "")
		}
		if containsErrorType(errorTypes, "linker") {
			parts = append(parts, "🔧 Linker error:")
			parts = append(parts, "  - This is usually a cross-compilation issue, not a code issue")
			parts = append(parts, "  - Host validation should still work")
			parts = append(parts, "  - Focus on fixing code errors first")
			parts = append(parts, "")
		}
	}

	parts = append(parts, "⚠️ Important: After fixing, call build_module again to verify the fix.")
	parts = append(parts, "Do NOT repeatedly read files without making changes.")

	return strings.Join(parts, "\n")
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
	if strings.Contains(outputLower, "cannot use") || strings.Contains(outputLower, "type mismatch") || strings.Contains(outputLower, "type ") && strings.Contains(outputLower, "expected") {
		checkAndAdd("type_mismatch")
	}
	if strings.Contains(outputLower, "syntax error") || strings.Contains(outputLower, "expected") && strings.Contains(outputLower, "unexpected") {
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
