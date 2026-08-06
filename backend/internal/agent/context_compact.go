package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/moduforge/backend/internal/service"
)

// isFileChangeResult reports whether a tool result records a file modification
// (write_file/edit_file/move_file) so it can be preserved during compaction.
// P0-1: edit_file results ("File edited:") were previously missed, so edits were
// silently dropped from compacted summaries even though writes were kept.
func isFileChangeResult(content string) bool {
	return strings.Contains(content, "Successfully wrote") ||
		strings.Contains(content, "File edited:") ||
		strings.Contains(content, "File moved:") ||
		strings.Contains(content, "write_file")
}

// extractFileChange extracts the first file-change line from a tool result for
// use in compaction key-facts summaries.
func extractFileChange(content string) string {
	for _, marker := range []string{"Successfully wrote", "File edited:", "File moved:"} {
		if idx := strings.Index(content, marker); idx >= 0 {
			end := strings.Index(content[idx:], "\n")
			if end < 0 {
				end = len(content[idx:])
			}
			return content[idx : idx+end]
		}
	}
	return ""
}

// containsDecision reports whether a message records a decision, conclusion, or
// plan that must be preserved in full during compaction. P1-6: mirrors the
// heuristic-compaction decision extraction so incremental compaction does not
// truncate critical user constraints and agent choices to 200 chars.
func containsDecision(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "decided") || strings.Contains(lower, "decision") ||
		strings.Contains(lower, "conclusion") || strings.Contains(lower, "chose") ||
		strings.Contains(lower, "approach") || strings.Contains(lower, "plan") ||
		strings.Contains(lower, "agree") || strings.Contains(lower, "disagree")
}

func (r *AgentRunner) smartCompressHistory(ctx context.Context, history []service.Message, w SSEWriter, cfg RunConfig) []service.Message {
	total := 0
	for _, m := range history {
		total += len(m.Content)
	}
	if total <= maxHistoryChars {
		return history
	}

	// Optimization 48: Try sliding window compaction first (zero LLM cost)
	// This is faster and cheaper than incremental or LLM compaction
	slidingCompacted := r.slidingWindowCompact(ctx, history, w, cfg)
	if len(slidingCompacted) > 0 {
		newTotal := 0
		for _, m := range slidingCompacted {
			newTotal += len(m.Content)
		}
		if newTotal <= maxHistoryChars {
			log.Printf("[Agent] sliding window compaction: %d msgs → %d msgs (was %d chars, now %d)", len(history), len(slidingCompacted), total, newTotal)
			return fixToolCallsInHistory(slidingCompacted)
		}
		// Still too large, use as input for incremental compaction
		history = slidingCompacted
		total = newTotal
	}

	// Phase 1: Incremental compaction — summarize oldest messages progressively
	compacted := r.incrementalCompactHistory(ctx, history, w, cfg, total)
	if len(compacted) > 0 {
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
	if len(compacted2) > 0 {
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
//  1. Only summarizes the oldest N messages (not the entire history)
//  2. Replaces them with a compact summary system message
//  3. Preserves the most recent messages in full
//  4. Optimization 23: Prioritizes preserving write_file/build_module results
//     (they contain critical file modification records) over read_file results.
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
	var fileChanges []string  // files that were written/modified
	var buildResults []string // build outcomes
	var decisions []string    // key decisions/plans (P1-6)

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
		if isFileChangeResult(content) {
			// Extract file paths from write/edit results
			if fc := extractFileChange(content); fc != "" {
				fileChanges = append(fileChanges, fc)
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
		} else if containsDecision(content) {
			// P1-6: Preserve decision/plan/conclusion messages in full instead of
			// truncating them to 200 chars (matches heuristic compaction behavior).
			decisions = append(decisions, content)
		} else {
			// Aggressively truncate read_file and other low-value messages
			if len(content) > 200 {
				content = content[:200] + "...[截断]"
			}
		}

		summary.WriteString(fmt.Sprintf("[%s]: %s\n\n", role, content))
	}

	// Append a key-facts section at the end for quick reference
	if len(fileChanges) > 0 || len(buildResults) > 0 || len(decisions) > 0 {
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
		if len(decisions) > 0 {
			summary.WriteString(fmt.Sprintf("Key decisions: %d\n", len(decisions)))
			limit := len(decisions)
			if limit > 5 {
				limit = 5
			}
			for _, d := range decisions[:limit] {
				summary.WriteString(fmt.Sprintf("  - %s\n", d))
			}
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

// Optimization 48: Sliding Window Compaction
// Automatically keeps the last N rounds of conversation in full,
// while summarizing earlier messages. This provides a balance between
// context preservation and token efficiency.
func (r *AgentRunner) slidingWindowCompact(ctx context.Context, history []service.Message, w SSEWriter, cfg RunConfig) []service.Message {
	const keepRounds = 5 // Keep last 5 user-assistant rounds
	const maxMessages = keepRounds * 2 + 1 // 5 rounds + system message

	if len(history) <= maxMessages {
		return history // Already within window
	}

	// Find split point: keep last maxMessages messages
	splitIdx := len(history) - maxMessages
	toCompact := history[:splitIdx]
	recentMessages := history[splitIdx:]

	// Build compressed summary of old messages
	var summary strings.Builder
	summary.WriteString("[对话窗口压缩] 早期对话已压缩，保留最近5轮完整对话\n\n")

	// Track key facts
	var fileChanges []string
	var errors []string
	var decisions []string

	for _, msg := range toCompact {
		content := msg.Content
		if content == "" {
			continue
		}

		// Extract key information
		if isFileChangeResult(content) {
			if fc := extractFileChange(content); fc != "" {
				fileChanges = append(fileChanges, fc)
			}
		} else if strings.HasPrefix(content, "Error:") || strings.HasPrefix(content, "❌") {
			if len(content) > 150 {
				content = content[:150]
			}
			errors = append(errors, content)
		} else if containsDecision(content) {
			if len(content) > 200 {
				content = content[:200]
			}
			decisions = append(decisions, content)
		}
	}

	// Build summary
	if len(fileChanges) > 0 {
		summary.WriteString(fmt.Sprintf("已修改文件 (%d):\n", len(fileChanges)))
		limit := len(fileChanges)
		if limit > 10 {
			limit = 10
		}
		for _, fc := range fileChanges[:limit] {
			summary.WriteString(fmt.Sprintf("  - %s\n", fc))
		}
	}

	if len(errors) > 0 {
		summary.WriteString(fmt.Sprintf("遇到的错误 (%d):\n", len(errors)))
		limit := len(errors)
		if limit > 5 {
			limit = 5
		}
		for _, e := range errors[:limit] {
			summary.WriteString(fmt.Sprintf("  - %s\n", e))
		}
	}

	if len(decisions) > 0 {
		summary.WriteString(fmt.Sprintf("关键决策 (%d):\n", len(decisions)))
		limit := len(decisions)
		if limit > 5 {
			limit = 5
		}
		for _, d := range decisions[:limit] {
			summary.WriteString(fmt.Sprintf("  - %s\n", d))
		}
	}

	// Build result: summary + recent messages
	result := make([]service.Message, 0, len(recentMessages)+1)
	result = append(result, service.Message{
		Role:    "system",
		Content: summary.String(),
	})
	result = append(result, recentMessages...)

	log.Printf("[Agent] slidingWindow: compacted %d messages to %d (+ summary)", len(history), len(result))
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

	summaryStr, err := r.callLLMSummary(ctx, cfg, summaryPrompt)
	if err != nil || summaryStr == "" {
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
		// Extract file paths from write/edit results
		if isFileChangeResult(content) {
			if fc := extractFileChange(content); fc != "" {
				fileChanges = append(fileChanges, fc)
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

	summaryStr, err := r.callLLMSummary(ctx, cfg, summaryPrompt)
	if err != nil {
		return conversation, fmt.Errorf("compaction LLM request failed: %w", err)
	}
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
		if role == "tool" && isFileChangeResult(content) {
			if fc := extractFileChange(content); fc != "" {
				fileChanges = append(fileChanges, fc)
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

// callLLMSummary sends a one-shot summary request to the LLM and returns the
// streamed content. Shared by compactConversation and compactHistoryViaLLM.
func (r *AgentRunner) callLLMSummary(ctx context.Context, cfg RunConfig, summaryPrompt []map[string]string) (string, error) {
	// P0-Optimization: Use cached resolved config when available.
	endpoint := cfg.resolvedEndpoint
	apiKey := cfg.resolvedAPIKey
	model := cfg.resolvedModel

	// Fallback: if not cached (e.g., called outside Run()), resolve now
	if endpoint == "" {
		endpoint, apiKey, model = r.resolveLLMConfig(cfg.UserID, cfg.ProviderID, "", cfg)
	}

	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint = endpoint + "/chat/completions"
	}

	log.Printf("[Agent] callLLMSummary: endpoint=%s model=%s apiKeyLen=%d providerID=%s", endpoint, model, len(apiKey), cfg.ProviderID)

	body := map[string]interface{}{
		"model":    model,
		"messages": summaryPrompt,
		"stream":   true, // Optimization 28: streaming reduces latency and connection hold time
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		log.Printf("[Agent] callLLMSummary: HTTP error: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	log.Printf("[Agent] callLLMSummary: response status=%d", resp.StatusCode)
	result := streamSSEContent(resp)
	log.Printf("[Agent] callLLMSummary: result length=%d", len(result))
	return result, nil
}

// streamSSEContent parses a streaming chat-completions SSE body and concatenates
// the delta content chunks.
func streamSSEContent(resp *http.Response) string {
	var summary strings.Builder
	forEachSSEChunk(resp.Body, 64*1024, func(data []byte) bool {
		var chunk streamChunk
		if err := json.Unmarshal(data, &chunk); err != nil {
			return true
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			summary.WriteString(chunk.Choices[0].Delta.Content)
		}
		return true
	})
	return summary.String()
}
