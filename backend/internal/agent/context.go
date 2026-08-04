package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/moduforge/backend/internal/service"
)

func (r *AgentRunner) estimateConversationSize(conversation []map[string]interface{}) int {
	total := 0
	for _, msg := range conversation {
		if c, ok := msg["content"].(string); ok {
			total += len(c)
		}
		// Optimization 8: Include tool_calls in size estimation
		if toolCalls, ok := msg["tool_calls"].([]LLMToolCall); ok {
			for _, tc := range toolCalls {
				total += len(tc.Function.Name) + len(tc.Function.Arguments)
			}
		}
	}
	return total
}

func (r *AgentRunner) smartCompressHistory(ctx context.Context, history []service.Message, w SSEWriter, cfg RunConfig) []service.Message {
	total := 0
	for _, m := range history {
		total += len(m.Content)
	}
	if total <= maxHistoryChars {
		return history
	}

	// Phase 1: Incremental compaction — summarize oldest messages progressively
	compacted := r.incrementalCompactHistory(ctx, history, w, cfg, total)
	if compacted != nil && len(compacted) > 0 {
		// Recalculate total after incremental compaction
		newTotal := 0
		for _, m := range compacted {
			newTotal += len(m.Content)
		}
		if newTotal <= maxHistoryChars {
			log.Printf("[Agent] incremental compaction: %d msgs → %d msgs (was %d chars, now %d)", len(history), len(compacted), total, newTotal)
			// Fix: ensure tool_calls/tool_call_id consistency after compression
			compacted = fixToolCallsInHistory(compacted)
			return compacted
		}
		// Still too large, use the compacted version as input for LLM compaction
		history = compacted
		total = newTotal
	}

	// Phase 2: Full LLM compaction (only if incremental wasn't enough)
	compacted2 := r.compactHistoryViaLLM(ctx, history, w, cfg)
	if compacted2 != nil && len(compacted2) > 0 {
		log.Printf("[Agent] LLM compaction: %d msgs → %d msgs (was %d chars)", len(history), len(compacted2), total)
		// Fix: ensure tool_calls/tool_call_id consistency after compression
		compacted2 = fixToolCallsInHistory(compacted2)
		return compacted2
	}

	// Fallback: keep most recent messages
	log.Printf("[Agent] fallback truncation: %d chars → %d chars", total, maxHistoryChars)
	var result []service.Message
	for i := len(history) - 1; i >= 0; i-- {
		total -= len(history[i].Content)
		if total < 0 {
			break
		}
		result = append([]service.Message{history[i]}, result...)
	}
	return result
}

// incrementalCompactHistory progressively summarizes older messages as the
// conversation grows. Unlike the full LLM compaction, this:
// 1. Only summarizes the oldest N messages (not the entire history)
// 2. Replaces them with a compact summary system message
// 3. Preserves the most recent messages in full
// 4. Optimization 23: Prioritizes preserving write_file/build_module results
//    (they contain critical file modification records) over read_file results.
//
// This is cheaper and faster than full LLM compaction, and triggers earlier
// (at 60% of maxHistoryChars) to avoid sudden expensive calls.
func (r *AgentRunner) incrementalCompactHistory(ctx context.Context, history []service.Message, w SSEWriter, cfg RunConfig, currentTotal int) []service.Message {
	if len(history) < 6 {
		return nil // too few messages to compact
	}

	// Don't compact if we already have a summary marker
	for _, m := range history {
		if m.Role == "system" && strings.HasPrefix(m.Content, "[上下文增量压缩]") {
			return nil // already compacted incrementally
		}
	}

	// Target: reduce to 60% of maxHistoryChars
	target := int(float64(maxHistoryChars) * 0.6)
	if currentTotal <= target {
		return nil
	}

	// Find the split point: keep the last 6 messages (3 user-assistant rounds) intact
	keepCount := 6
	if len(history) <= keepCount {
		return nil
	}

	splitIdx := len(history) - keepCount
	toCompact := history[:splitIdx]

	// Build a compact summary of the old messages
	// Optimization 23: Classify messages by importance for smarter summarization
	var summary strings.Builder
	summary.WriteString("[上下文增量压缩]\n\n")
	summary.WriteString(fmt.Sprintf("以下是 %d 条早期对话的摘要（共 %d 字符）：\n\n", len(toCompact), currentTotal))

	// Track key facts for the summary
	var fileChanges []string // files that were written/modified
	var buildResults []string // build outcomes

	for _, msg := range toCompact {
		role := "User"
		switch msg.Role {
		case "assistant":
			role = "Agent"
		case "system":
			role = "System"
		case "tool":
			role = "Tool"
		}
		content := msg.Content

		// Optimization 23: Extract key facts from important messages
		if strings.Contains(content, "write_file") || strings.Contains(content, "Successfully wrote") {
			// Extract file paths from write results
			if idx := strings.Index(content, "Successfully wrote"); idx >= 0 {
				end := strings.Index(content[idx:], "\n")
				if end < 0 {
					end = len(content[idx:])
				}
				fileChanges = append(fileChanges, content[idx:idx+end])
			} else if len(content) < 200 {
				fileChanges = append(fileChanges, content)
			}
			// Don't truncate write results — keep them at full length
		} else if strings.Contains(content, "build_module") || strings.Contains(content, "Build") {
			// Keep build results at full length too
			if len(content) > 300 {
				content = content[:300] + "...[截断]"
			}
			buildResults = append(buildResults, content)
		} else {
			// Aggressively truncate read_file and other low-value messages
			if len(content) > 200 {
				content = content[:200] + "...[截断]"
			}
		}

		summary.WriteString(fmt.Sprintf("[%s]: %s\n\n", role, content))
	}

	// Append a key-facts section at the end for quick reference
	if len(fileChanges) > 0 || len(buildResults) > 0 {
		summary.WriteString("\n## Key Facts (preserved from compacted messages):\n")
		if len(fileChanges) > 0 {
			summary.WriteString(fmt.Sprintf("Files modified: %d\n", len(fileChanges)))
			// Show at most 10 file changes
			limit := len(fileChanges)
			if limit > 10 {
				limit = 10
			}
			for _, fc := range fileChanges[:limit] {
				summary.WriteString(fmt.Sprintf("  - %s\n", fc))
			}
		}
		if len(buildResults) > 0 {
			summary.WriteString(fmt.Sprintf("Build results: %d\n", len(buildResults)))
		}
	}

	// Build new history: summary + last keepCount messages
	result := make([]service.Message, 0, keepCount+1)
	result = append(result, service.Message{
		Role:    "system",
		Content: summary.String(),
	})
	result = append(result, history[splitIdx:]...)

	return result
}

func (r *AgentRunner) compactHistoryViaLLM(ctx context.Context, history []service.Message, w SSEWriter, cfg RunConfig) []service.Message {
	if len(history) < 4 {
		return nil
	}

	// Optimization 28: Free models use heuristic summarization (zero LLM cost)
	modelTier := resolveModelTier(cfg.LLMModel)
	if modelTier == TierFree {
		return r.heuristicCompactHistory(history)
	}

	// Build a summary request
	var historyText strings.Builder
	for _, msg := range history {
		role := "User"
		if msg.Role == "assistant" {
			role = "Assistant"
		}
		historyText.WriteString(fmt.Sprintf("%s: %s\n\n", role, msg.Content))
	}

	summaryPrompt := []map[string]string{
		{"role": "system", "content": `You are a conversation summarizer. Summarize the following conversation between a user and an AI coding agent. 

CRITICAL: Preserve ALL of the following:
- File paths mentioned or modified
- Key decisions and their reasons
- Errors encountered and how they were resolved
- Current work in progress
- User constraints and requirements

Be concise but complete. Output ONLY the summary text, no labels.`},
		{"role": "user", "content": historyText.String()},
	}

	endpoint, apiKey, model := r.resolveLLMConfig(cfg.UserID, "", "", cfg)
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint = endpoint + "/chat/completions"
	}

	body := map[string]interface{}{
		"model":    model,
		"messages": summaryPrompt,
		"stream":   true, // Optimization 28: streaming reduces latency and connection hold time
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		log.Printf("[Agent] failed to marshal LLM request body: %v", err)
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	// Streaming: parse SSE chunks incrementally
	var summary strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			summary.WriteString(chunk.Choices[0].Delta.Content)
		}
	}

	summaryStr := summary.String()
	if summaryStr == "" {
		return nil
	}

	// Return summary as a single system message + the last 2 user messages
	compacted := []service.Message{
		{Role: "system", Content: "[对话已压缩] " + summaryStr},
	}
	// Keep the last 2 messages for immediate context
	if len(history) >= 2 {
		compacted = append(compacted, history[len(history)-2])
		compacted = append(compacted, history[len(history)-1])
	}

	return compacted
}

// heuristicCompactHistory provides zero-LLM-cost summarization for free models.
// Optimization 28: Extracts key facts (file paths, decisions, errors) and keeps
// the most recent messages, discarding the rest. Much cheaper than calling an LLM
// which itself consumes precious tokens from the free model's small context window.
func (r *AgentRunner) heuristicCompactHistory(history []service.Message) []service.Message {
	var fileChanges []string
	var decisions []string
	var errors []string

	for _, msg := range history {
		content := msg.Content
		// Extract file paths from write results
		if strings.Contains(content, "Successfully wrote") {
			if idx := strings.Index(content, "Successfully wrote"); idx >= 0 {
				end := strings.Index(content[idx:], "\n")
				if end < 0 {
					end = len(content[idx:])
				}
				fileChanges = append(fileChanges, content[idx:idx+end])
			}
		}
		// Extract decisions (assistant messages with key phrases)
		if msg.Role == "assistant" {
			lower := strings.ToLower(content)
			if strings.Contains(lower, "decided") || strings.Contains(lower, "decision") ||
				strings.Contains(lower, "chose") || strings.Contains(lower, "approach") {
				if len(content) > 200 {
					content = content[:200] + "..."
				}
				decisions = append(decisions, content)
			}
		}
		// Extract errors
		if strings.HasPrefix(content, "Error:") || strings.HasPrefix(content, "❌") ||
			strings.HasPrefix(content, "⚠️") {
			if len(content) > 150 {
				content = content[:150] + "..."
			}
			errors = append(errors, content)
		}
	}

	// Build compact summary
	var sb strings.Builder
	sb.WriteString("[上下文压缩 - 节省模式]\n\n")
	if len(fileChanges) > 0 {
		sb.WriteString(fmt.Sprintf("已修改文件 (%d):\n", len(fileChanges)))
		limit := len(fileChanges)
		if limit > 10 {
			limit = 10
		}
		for _, fc := range fileChanges[:limit] {
			sb.WriteString(fmt.Sprintf("  - %s\n", fc))
		}
	}
	if len(decisions) > 0 {
		sb.WriteString(fmt.Sprintf("\n关键决策 (%d):\n", len(decisions)))
		limit := len(decisions)
		if limit > 5 {
			limit = 5
		}
		for _, d := range decisions[:limit] {
			sb.WriteString(fmt.Sprintf("  - %s\n", d))
		}
	}
	if len(errors) > 0 {
		sb.WriteString(fmt.Sprintf("\n遇到的错误 (%d):\n", len(errors)))
		limit := len(errors)
		if limit > 5 {
			limit = 5
		}
		for _, e := range errors[:limit] {
			sb.WriteString(fmt.Sprintf("  - %s\n", e))
		}
	}

	// Keep the last 4 messages for immediate context
	keepCount := 4
	if len(history) < keepCount {
		keepCount = len(history)
	}
	result := []service.Message{
		{Role: "system", Content: sb.String()},
	}
	result = append(result, history[len(history)-keepCount:]...)

	return result
}

func (r *AgentRunner) compactConversation(ctx context.Context, conversation []map[string]interface{}, w SSEWriter, cfg RunConfig) ([]map[string]interface{}, error) {
	// Optimization 28: Free models use heuristic summarization (zero LLM cost)
	modelTier := resolveModelTier(cfg.LLMModel)
	if modelTier == TierFree {
		return r.heuristicCompactConversation(conversation), nil
	}

	// Build summary from conversation
	var convText strings.Builder
	for _, msg := range conversation {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		if content == "" {
			continue
		}
		label := "User"
		if role == "assistant" {
			label = "Agent"
		} else if role == "system" {
			label = "System"
		} else if role == "tool" {
			label = "Tool Result"
		}
		convText.WriteString(fmt.Sprintf("%s: %s\n\n", label, content))
	}

	summaryPrompt := []map[string]string{
		{"role": "system", "content": `Summarize this agent conversation. Preserve: file paths, decisions, errors, work in progress, user requirements. Be concise. Output ONLY the summary.`},
		{"role": "user", "content": convText.String()},
	}

	endpoint, apiKey, model := r.resolveLLMConfig(cfg.UserID, "", "", cfg)
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint = endpoint + "/chat/completions"
	}

	body := map[string]interface{}{
		"model":    model,
		"messages": summaryPrompt,
		"stream":   true, // Optimization 28: streaming reduces latency and connection hold time
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return conversation, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return conversation, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		return conversation, err
	}
	defer resp.Body.Close()

	// Streaming: parse SSE chunks incrementally
	var summary strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			summary.WriteString(chunk.Choices[0].Delta.Content)
		}
	}

	summaryStr := summary.String()
	if summaryStr == "" {
		return conversation, fmt.Errorf("compaction LLM returned empty summary")
	}

	// Rebuild conversation: system prompt + summary + last user message
	newConv := make([]map[string]interface{}, 0)
	// Keep the first system message
	for _, msg := range conversation {
		if msg["role"] == "system" {
			newConv = append(newConv, msg)
			break
		}
	}

	// Add summary
	newConv = append(newConv, map[string]interface{}{
		"role":    "system",
		"content": fmt.Sprintf("[上下文已压缩]\n\n之前的对话摘要：\n%s", summaryStr),
	})

	// Add last user message
	for i := len(conversation) - 1; i >= 0; i-- {
		if conversation[i]["role"] == "user" {
			newConv = append(newConv, conversation[i])
			break
		}
	}

	return newConv, nil
}

// heuristicCompactConversation provides zero-LLM-cost summarization for free models.
// Optimization 28: Extracts key facts and keeps recent messages, no LLM call needed.
// P1-1: Enhanced with information density scoring to preserve critical context.
func (r *AgentRunner) heuristicCompactConversation(conversation []map[string]interface{}) []map[string]interface{} {
	// P1-1: Score messages by importance
	type scoredMsg struct {
		msg      map[string]interface{}
		score    float64
		position int
	}

	scored := make([]scoredMsg, 0, len(conversation))
	for i, msg := range conversation {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		score := 0.0

		// System messages are always important
		if role == "system" {
			score = 100.0
		}

		// User messages are important
		if role == "user" {
			score = 80.0
		}

		// Tool results with file changes are critical
		if role == "tool" && strings.Contains(content, "Successfully wrote") {
			score = 90.0
		}

		// Errors are important for debugging
		if role == "tool" && (strings.HasPrefix(content, "Error:") || strings.HasPrefix(content, "❌")) {
			score = 70.0
		}

		// Recent messages get position bonus
		positionBonus := float64(i) / float64(len(conversation)) * 20.0
		score += positionBonus

		// Penalize very long messages (they waste context window)
		if len(content) > 5000 {
			score -= 10.0
		}

		scored = append(scored, scoredMsg{msg: msg, score: score, position: i})
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Keep top messages by score, but maintain order
	keepCount := len(conversation) / 3 // Keep 1/3 of messages
	if keepCount < 5 {
		keepCount = 5
	}
	if keepCount > len(scored) {
		keepCount = len(scored)
	}

	// Select top scored messages
	keepSet := make(map[int]bool)
	for i := 0; i < keepCount; i++ {
		keepSet[scored[i].position] = true
	}

	// Rebuild conversation in original order
	var fileChanges []string
	var errors []string
	newConv := make([]map[string]interface{}, 0)

	// Always keep system prompt
	for _, msg := range conversation {
		if msg["role"] == "system" {
			newConv = append(newConv, msg)
			break
		}
	}

	// Add summary of what happened
	for _, msg := range conversation {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		if role == "tool" && strings.Contains(content, "Successfully wrote") {
			if idx := strings.Index(content, "Successfully wrote"); idx >= 0 {
				end := strings.Index(content[idx:], "\n")
				if end < 0 {
					end = len(content[idx:])
				}
				fileChanges = append(fileChanges, content[idx:idx+end])
			}
		}
		if role == "tool" && (strings.HasPrefix(content, "Error:") || strings.HasPrefix(content, "❌") || strings.HasPrefix(content, "⚠️")) {
			if len(content) > 150 {
				content = content[:150] + "..."
			}
			errors = append(errors, content)
		}
	}

	// Build summary
	var summary strings.Builder
	summary.WriteString("[上下文压缩 - 智能保留重要信息]\n\n")
	if len(fileChanges) > 0 {
		summary.WriteString(fmt.Sprintf("已修改文件 (%d):\n", len(fileChanges)))
		for _, fc := range fileChanges {
			if len(fileChanges) > 10 && summary.Len() > 500 {
				summary.WriteString(fmt.Sprintf("  ... 还有 %d 个文件\n", len(fileChanges)-10))
				break
			}
			summary.WriteString(fmt.Sprintf("  - %s\n", fc))
		}
	}
	if len(errors) > 0 {
		summary.WriteString(fmt.Sprintf("\n错误 (%d):\n", len(errors)))
		limit := len(errors)
		if limit > 5 {
			limit = 5
		}
		for _, e := range errors[:limit] {
			summary.WriteString(fmt.Sprintf("  - %s\n", e))
		}
	}

	newConv = append(newConv, map[string]interface{}{
		"role":    "system",
		"content": summary.String(),
	})

	// Add kept messages in order
	for i, msg := range conversation {
		if keepSet[i] && msg["role"] != "system" {
			newConv = append(newConv, msg)
		}
	}

	// Always keep last user message
	for i := len(conversation) - 1; i >= 0; i-- {
		if conversation[i]["role"] == "user" {
			if !keepSet[i] {
				newConv = append(newConv, conversation[i])
			}
			break
		}
	}

	return newConv
}

// ═══════════════════════════════════════════════════════════════════
// System Prompt — mode-aware
// ═══════════════════════════════════════════════════════════════════

// systemPromptCache caches system prompts per mode (they never change at runtime).
var systemPromptCache sync.Map

func (r *AgentRunner) buildSystemPromptForMode(mode AgentMode) string {
	// Fast path: cached
	if cached, ok := systemPromptCache.Load(mode); ok {
		return cached.(string)
	}
	// Slow path: build and cache
	prompt := r.buildSystemPromptForModeUncached(mode)
	systemPromptCache.Store(mode, prompt)
	return prompt
}

func (r *AgentRunner) buildSystemPromptForModeUncached(mode AgentMode) string {
	var sb strings.Builder

	if mode == ModePlan {
		sb.WriteString(`You are an AI coding agent in PLAN MODE (read-only). Analyze code and create implementation plans.

## RULES
- CANNOT modify files or execute write commands
- Read files to understand current state before planning
- Break tasks into clear, actionable steps with file lists
- Identify risks and edge cases

## OUTPUT FORMAT
Your FINAL answer (when done using tools) MUST be clean Markdown inside <answer> tags:
<answer>
## Implementation Plan
### Step 1: [description]
- Files: [list]
- Changes: [what to do]
### Risks & Considerations
- [issues]
### Estimated Complexity: [Low/Medium/High]
</answer>

Do NOT output raw tool call syntax. Summarize tool results instead of repeating logs.
`)
	} else {
		sb.WriteString(`You are an expert AI coding agent in ACT MODE. You have FULL access to read, write, and build files.

## 3 NON-NEGOTIABLE RULES
1. If a task requires file changes, you MUST call write_file for EACH file. No exceptions.
2. After writing code, you MUST call build_module to verify it compiles. No exceptions.
3. Your FINAL answer lists files you ACTUALLY wrote, not files you plan to write.

## WORKFLOW (follow this order, never skip steps)
1. read_file → understand current state
2. write_file → create/modify each file (COMPLETE content, not snippets)
3. build_module → verify compilation
4. If build fails: read error, fix with write_file, rebuild (max 3 retries)
5. Answer: "I modified X files: [list]. Build status: [pass/fail]."

## ANTI-PATTERNS (NEVER do these)
- Outputting a "plan" without calling write_file
- Listing "missing files" without creating them
- Saying "需要创建 X" without write_file
- Reading files and only outputting analysis
- Skipping build_module after writing code

## OUTPUT FORMAT
Final answer must be clean Markdown. NO raw tool syntax, NO JSON, NO "Executing tool..." lines.
Example: "I updated ipc.rs to fix libc::open type mismatch. Build passes."
`)
	}

	// NOTE: Tool names are already provided via the `tools` parameter in the API call.
	// Listing them again in the system prompt wastes ~2000 tokens and confuses small models.
	// Only include a compact reference for the most commonly misused tools.
	if mode == ModeAct {
		sb.WriteString(`## KEY TOOLS (full schema in tools parameter)
- read_file(path) → read file content
- write_file(path, content) → write COMPLETE file (auto-creates dirs)
- edit_file(path, old_text, new_text) → find-and-replace (preferred for small changes)
- grep_search(pattern) → search code across all files (like grep -rn)
- glob_search(pattern) → find files by name (e.g., "**/*.go")
- bash(command) → run shell commands (build, test, git)
- build_module(project_id) → compile + package ZIP

## TOOL BEST PRACTICES
### write_file
- ALWAYS read_file first to understand current content
- Write COMPLETE file content, not snippets
- For large files (>100 lines), consider edit_file for changes <30%
- After write, verify with read_file to confirm

### edit_file
- Use for changes <30% of file content
- old_text must match EXACTLY (including whitespace)
- Test with small changes first
- If edit fails, fall back to write_file

### grep_search
- Use before read_file to locate relevant code
- Supports regex patterns
- Use is_regex=true for complex patterns
- Check include_pattern to filter file types

### bash
- Use for build, test, git operations
- Check exit code for success/failure
- Use cd && command for directory-specific ops
- Timeout is 30s for read-only, 60s for write operations

### build_module
- ALWAYS call after write_file
- Reads error messages carefully
- Fix issues before retrying (max 3 retries)

## FILE RULES
- edit_file for changes <30% of file (MOST common)
- write_file for new files or complete rewrites ONLY
- grep_search BEFORE read_file (locate code first)
- After writing, ALWAYS call build_module

## WORKFLOW
1. grep_search → find relevant code
2. read_file → understand context
3. edit_file → make changes (or write_file for new files)
4. build_module → verify compilation
5. Report: files changed + build status

CRITICAL: You are evaluated on whether you ACTUALLY WROTE FILES and VERIFIED THE BUILD.
A plan without execution is a failure. Analysis without modification is a failure.
`)
	}

	return sb.String()
}

// ═══════════════════════════════════════════════════════════════════
// Project Context

func (r *AgentRunner) buildProjectContext(cfg RunConfig) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n## CURRENT PROJECT\nProject ID: %s\n", cfg.ProjectID))

	if cfg.ProjectContext != "" {
		sb.WriteString(fmt.Sprintf("Description:\n%s\n", cfg.ProjectContext))
	}

	if r.db != nil {
		rows, err := r.db.Query(
			`SELECT path, length(content) as size FROM project_files WHERE project_id=? ORDER BY path`,
			cfg.ProjectID,
		)
		if err == nil {
			defer rows.Close()
			var files []string
			for rows.Next() {
				var p string
				var sz int
				if rows.Scan(&p, &sz) == nil {
					files = append(files, fmt.Sprintf("- %s (%d bytes)", p, sz))
				}
			}
			if len(files) > 0 {
				sb.WriteString(fmt.Sprintf("\n## PROJECT FILES (%d files)\n\n", len(files)))
				for _, f := range files {
					sb.WriteString(f + "\n")
				}
			}
		}
	}

	sb.WriteString(`
## BUILD ENVIRONMENT (Alpine 3.21 container)
Full cross-compilation toolchain available:
- Go: /usr/local/go/bin/go (v1.25, CGO_ENABLED=1)
- Rust + Cargo: /usr/local/cargo/bin/ (musl-based, with aarch64-linux-android target pre-installed)
- Android NDK r27c: /opt/android-ndk (trimmed, aarch64-linux-android clang/clang++)
- GCC/musl: Alpine system compiler
- Node.js 22: for frontend builds

You CAN cross-compile for Android ARM64:
- C/C++: use NDK clang at /opt/android-ndk/bin/aarch64-linux-android*-clang
- Rust: cargo build --target aarch64-linux-android (target already installed)
- Go: CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build

For Magisk/KernelSU/APatch modules:
- Shell scripts (customize.sh, service.sh) run on the PHONE — use #!/system/bin/sh
- Native binaries (.so, executables) should be cross-compiled here and included in the module ZIP
- Use build_module skill to package the final ZIP

## WORKFLOW
1. Understand the user's request
2. Use read_file to read files you need to modify (DO NOT guess content)
3. Use write_file to save changes (project_id auto-injected)
4. ONLY modify files that need changes — do NOT rewrite unchanged files
5. Explain what you changed

## CRITICAL RULES
- NEVER call write_file without first reading the file with read_file
- NEVER write empty or whitespace-only content
- NEVER rewrite files that don't need changes`)

	// Load project knowledge and session history from memory
	if r.memV2Store != nil && cfg.ProjectID != "" {
		sb.WriteString(r.memV2Store.LoadProjectContextForAgent(cfg.UserID, cfg.ProjectID))
	}

	return sb.String()
}

// ═══════════════════════════════════════════════════════════════════
// Optimization 2: Conversation Pre-filtering
//
// Cleans the conversation before sending to LLM:
// - Removes empty tool results
// - Removes consecutive duplicate assistant messages
// - Collapses long tool result sequences
// ═══════════════════════════════════════════════════════════════════

// ═══════════════════════════════════════════════════════════════════
// Optimization 32: Incremental prefilterConversation
//
// Cleans the conversation to reduce token waste.
// Reuses result slice across calls via sync.Pool.
// Optimization 43: Smart message pruning — keeps most recent N messages in full,
// compresses older messages to just role + key facts.
// ═══════════════════════════════════════════════════════════════════

var prefilterPool = sync.Pool{
	New: func() interface{} {
		s := make([]map[string]interface{}, 0, 128)
		return &s
	},
}

func prefilterConversation(conversation []map[string]interface{}) []map[string]interface{} {
	if len(conversation) <= 2 {
		return conversation
	}

	// Optimization 43: If conversation is very long (>30 messages), apply aggressive pruning
	// Keep last 10 messages in full, compress older ones to essential info only
	if len(conversation) > 30 {
		return smartPruneConversation(conversation)
	}

	// Reuse pooled slice
	sp := prefilterPool.Get().(*[]map[string]interface{})
	result := (*sp)[:0]
	defer func() {
		*sp = result
		prefilterPool.Put(sp)
	}()

	prevRole := ""
	prevContent := ""
	skipped := 0
	seenToolResults := make(map[string]int)

	// First pass: collect all tool_call_ids that have responses
	toolCallIDsToKeep := make(map[string]bool)
	for _, msg := range conversation {
		role, _ := msg["role"].(string)
		if role == "assistant" {
			if toolCalls, ok := msg["tool_calls"].([]LLMToolCall); ok && len(toolCalls) > 0 {
				for _, tc := range toolCalls {
					toolCallIDsToKeep[tc.ID] = false // mark as needing response
				}
			}
		}
		if role == "tool" {
			if toolCallID, ok := msg["tool_call_id"].(string); ok && toolCallID != "" {
				if _, needsResponse := toolCallIDsToKeep[toolCallID]; needsResponse {
					toolCallIDsToKeep[toolCallID] = true // has response
				}
			}
		}
	}

	for _, msg := range conversation {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)

		// Skip empty tool results (waste tokens) - BUT only if not required by tool_calls
		if role == "tool" && strings.TrimSpace(content) == "" {
			if toolCallID, ok := msg["tool_call_id"].(string); ok && toolCallID != "" {
				if keep, exists := toolCallIDsToKeep[toolCallID]; exists && keep {
					// This tool response is needed - keep it with placeholder
					msg = copyMap(msg)
					msg["content"] = "(empty result)"
				} else {
					skipped++
					continue
				}
			} else {
				skipped++
				continue
			}
		}

		// Skip empty system messages
		if role == "system" && strings.TrimSpace(content) == "" {
			skipped++
			continue
		}

		// Skip consecutive duplicate assistant messages (LLM repeating itself)
		if role == "assistant" && role == prevRole && content == prevContent {
			skipped++
			continue
		}

		// Deduplicate identical tool results across rounds
		if role == "tool" {
			key := content
			if len(key) > 200 {
				key = key[:200]
			}
			if count, exists := seenToolResults[key]; exists {
				if count >= 2 {
					skipped++
					continue
				}
				seenToolResults[key] = count + 1
			} else {
				seenToolResults[key] = 1
			}
		}

		// For tool results: if content is very long (>4K), keep only first 2K + last 1K
		if role == "tool" && len(content) > 4000 {
			newContent := content[:2000] + "\n... [truncated to save context] ...\n" + content[len(content)-1000:]
			msg = copyMap(msg)
			msg["content"] = newContent
		}

		result = append(result, msg)
		prevRole = role
		prevContent = content
	}

	// Final pass: fix tool_calls/tool response consistency
	// 1. Remove tool_calls from assistant messages if their tool responses are missing
	// 2. Remove orphaned tool messages (tool responses without preceding tool_calls)
	// This prevents LLM API errors about missing/mismatched tool messages
	toolResponseIDsInResult := make(map[string]bool)
	for _, msg := range result {
		if role, _ := msg["role"].(string); role == "tool" {
			if toolCallID, ok := msg["tool_call_id"].(string); ok && toolCallID != "" {
				toolResponseIDsInResult[toolCallID] = true
			}
		}
	}
	// Collect all tool_call_ids from assistant messages
	assistantToolCallIDs := make(map[string]bool)
	for _, msg := range result {
		if role, _ := msg["role"].(string); role == "assistant" {
			if toolCalls, ok := msg["tool_calls"].([]LLMToolCall); ok && len(toolCalls) > 0 {
				for _, tc := range toolCalls {
					if tc.ID != "" {
						assistantToolCallIDs[tc.ID] = true
					}
				}
			}
		}
	}
	// Remove tool_calls from assistant messages if their tool responses are missing
	for i, msg := range result {
		if role, _ := msg["role"].(string); role == "assistant" {
			if toolCalls, ok := msg["tool_calls"].([]LLMToolCall); ok && len(toolCalls) > 0 {
				var validCalls []LLMToolCall
				for _, tc := range toolCalls {
					if tc.ID == "" || toolResponseIDsInResult[tc.ID] {
						validCalls = append(validCalls, tc)
					}
				}
				if len(validCalls) != len(toolCalls) {
					result[i] = copyMap(msg)
					result[i]["tool_calls"] = validCalls
				}
			}
		}
	}
	// Remove orphaned tool messages (tool responses without preceding tool_calls in assistant)
	var filtered []map[string]interface{}
	for _, msg := range result {
		if role, _ := msg["role"].(string); role == "tool" {
			if toolCallID, ok := msg["tool_call_id"].(string); ok && toolCallID != "" {
				// Keep only if there's a matching assistant tool_call
				if assistantToolCallIDs[toolCallID] {
					filtered = append(filtered, msg)
				} else {
					skipped++
				}
				continue
			}
		}
		filtered = append(filtered, msg)
	}
	result = filtered

	// Copy result to avoid returning pooled slice
	out := make([]map[string]interface{}, len(result))
	copy(out, result)
	if skipped > 0 {
		log.Printf("[Prefilter] removed %d messages (was %d, now %d)", skipped, len(conversation), len(out))
	}
	return out
}

// copyMap creates a shallow copy of a map.
func copyMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// smartPruneConversation: Optimization 43
// For long conversations (>30 messages), keep the last 10 messages in full
// and compress older messages to just role + essential facts (file changes, errors, decisions).
// This reduces token usage by 40-60% while preserving critical context.
func smartPruneConversation(conversation []map[string]interface{}) []map[string]interface{} {
	const keepRecent = 10
	if len(conversation) <= keepRecent {
		return conversation
	}

	// Split: old messages to compress, recent messages to keep in full
	splitIdx := len(conversation) - keepRecent
	oldMessages := conversation[:splitIdx]
	recentMessages := conversation[splitIdx:]

	// Build compressed summary of old messages
	var fileChanges []string
	var errors []string
	var decisions []string
	roundCount := 0

	for _, msg := range oldMessages {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)

		if content == "" {
			continue
		}

		// Extract file write results
		if strings.Contains(content, "Successfully wrote") || strings.Contains(content, "write_file") {
			if len(content) > 100 {
				content = content[:100]
			}
			fileChanges = append(fileChanges, content)
		}
		// Extract errors
		if strings.HasPrefix(content, "Error:") || strings.HasPrefix(content, "❌") ||
			strings.HasPrefix(content, "⚠️") || strings.Contains(content, "failed") {
			if len(content) > 150 {
				content = content[:150]
			}
			errors = append(errors, content)
		}
		// Extract assistant decisions (short key phrases)
		if role == "assistant" && len(content) > 20 && len(content) < 500 {
			lower := strings.ToLower(content)
			if strings.Contains(lower, "approach") || strings.Contains(lower, "plan") ||
				strings.Contains(lower, "will") || strings.Contains(lower, "implement") {
				if len(content) > 150 {
					content = content[:150]
				}
				decisions = append(decisions, content)
			}
		}
		// Count rounds (user messages)
		if role == "user" {
			roundCount++
		}
	}

	// Build compressed system message
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[对话压缩] 原始 %d 条消息（%d 轮对话）已压缩\n", len(conversation), roundCount))

	if len(fileChanges) > 0 {
		sb.WriteString(fmt.Sprintf("\n已修改文件 (%d):\n", len(fileChanges)))
		limit := len(fileChanges)
		if limit > 10 {
			limit = 10
		}
		for _, fc := range fileChanges[:limit] {
			sb.WriteString(fmt.Sprintf("  - %s\n", fc))
		}
	}

	if len(errors) > 0 {
		sb.WriteString(fmt.Sprintf("\n遇到的错误 (%d):\n", len(errors)))
		limit := len(errors)
		if limit > 5 {
			limit = 5
		}
		for _, e := range errors[:limit] {
			sb.WriteString(fmt.Sprintf("  - %s\n", e))
		}
	}

	if len(decisions) > 0 {
		sb.WriteString(fmt.Sprintf("\n关键决策 (%d):\n", len(decisions)))
		limit := len(decisions)
		if limit > 5 {
			limit = 5
		}
		for _, d := range decisions[:limit] {
			sb.WriteString(fmt.Sprintf("  - %s\n", d))
		}
	}

	// Build result: compressed summary + recent messages
	result := make([]map[string]interface{}, 0, keepRecent+1)
	result = append(result, map[string]interface{}{
		"role":    "system",
		"content": sb.String(),
	})
	result = append(result, recentMessages...)

	log.Printf("[Prefilter:smart] compressed %d messages to %d (+ summary)", len(conversation), len(result))
	return result
}

// ═══════════════════════════════════════════════════════════════════
// Optimization 3: Free Model Specific Prompt
//
// Ultra-short system prompt for free models (~200 tokens vs ~800 tokens).
// Strips verbose instructions that waste limited context window.
// ═══════════════════════════════════════════════════════════════════

func buildFreeModelPrompt(mode AgentMode) string {
	if mode == ModePlan {
		return `You are a coding agent in PLAN MODE (read-only). Analyze code and create plans.
- CANNOT modify files. Read only.
- Break tasks into clear steps with file lists.
- Output final plan as clean Markdown.`
	}

	return `You are a coding agent in ACT MODE with file access.

## RULES
1. For file changes: call write_file (COMPLETE file, not snippets)
2. After writing code: call build_module to verify
3. Final answer: list files you ACTUALLY wrote

## WORKFLOW
1. read_file → understand
2. write_file → create/modify
3. build_module → verify
4. Fix errors → rebuild (max 3 retries)
5. Answer: files modified + build status

NEVER output plans without writing. NEVER skip build_module.`
}
