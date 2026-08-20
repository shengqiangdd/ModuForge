package agent

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
)

// toolTask is a validated tool call ready for execution.
type toolTask struct {
	tc         LLMToolCall
	skillName  string
	skillInput map[string]interface{}
	parallel   bool
}

// runMetrics tracks tool-call accounting that accumulates across Run iterations.
// Includes O(1) pre-computed counters for loop detection.
type runMetrics struct {
	toolCallHistory          map[string]int
	uniqueOps                map[string]bool
	totalToolCalls           int
	toolConsecutiveErrors    map[string]int    // skill name -> consecutive error count
	toolLastResults          map[string]string // skill name -> last result (for pattern detection)
	toolConsecutiveIdentical map[string]int    // skill name -> consecutive identical calls

	// O(1) pre-computed counters for detectLoop (avoids iterating uniqueOps per skill)
	uniqueTargetsPerSkill map[string]int // skill name -> count of unique targets (e.g., "read_file" -> 5)
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
