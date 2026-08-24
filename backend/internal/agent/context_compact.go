package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/moduforge/backend/internal/service"
)

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
			w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "compact",
				"content": fmt.Sprintf("历史已压缩：%d 条消息 → %d 条（%d 字符 → %d 字符，零成本滑动窗口）", len(history), len(slidingCompacted), total, newTotal),
			})
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
			w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "compact",
				"content": fmt.Sprintf("历史已压缩：%d 条消息 → %d 条（增量摘要，%d 字符 → %d 字符）", len(history), len(compacted), total, newTotal),
			})
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
		w.WriteSSE(map[string]interface{}{
			"type":    "step",
			"step":    "compact",
			"content": fmt.Sprintf("历史已压缩：%d 条消息 → %d 条（LLM 摘要，%d 字符 → 摘要）", len(history), len(compacted2), total),
		})
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
	const keepRounds = 5                 // Keep last 5 user-assistant rounds
	const maxMessages = keepRounds*2 + 1 // 5 rounds + system message

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

// SmartCompact performs intelligent conversation compression.
// Preserves: system prompts, recent N rounds, key tool results.
// Compresses: middle-round long text into summaries.
func SmartCompact(messages []service.Message, maxTokens int) []service.Message {
	if estimateTokens(messages) <= maxTokens {
		return messages
	}

	result := make([]service.Message, 0, len(messages))

	for i, msg := range messages {
		// System prompts always preserved
		if msg.Role == "system" {
			result = append(result, msg)
			continue
		}

		// Last 10 messages (5 rounds) preserved in full
		if i >= len(messages)-10 {
			result = append(result, msg)
			continue
		}

		// Middle rounds: compress long assistant messages
		if msg.Role == "assistant" && len(msg.Content) > 500 {
			compressed := msg.Content[:200] + "\n... [compressed: full content preserved at original position]"
			result = append(result, service.Message{Role: msg.Role, Content: compressed})
			continue
		}

		// Middle rounds: compress long tool results
		if msg.Role == "tool" && len(msg.Content) > 300 {
			compressed := msg.Content[:150] + "\n... [tool result compressed]"
			result = append(result, service.Message{
				Role:       msg.Role,
				Content:    compressed,
				ToolCallID: msg.ToolCallID,
			})
			continue
		}

		result = append(result, msg)
	}
	return result
}

// estimateTokens provides a rough token estimate (1 token ≈ 4 chars for English,
// ~2 chars for CJK). Uses 3 as a compromise for mixed content.
func estimateTokens(messages []service.Message) int {
	total := 0
	for _, m := range messages {
		total += len(m.Content) / 3
	}
	return total
}

// estimateMapTokens estimates tokens for map-based conversation.
func estimateMapTokens(conversation []map[string]interface{}) int {
	total := 0
	for _, msg := range conversation {
		if content, ok := msg["content"].(string); ok {
			total += len(content) / 3
		}
	}
	return total
}

// smartCompactMapConversation compresses a map-based conversation.
// Preserves system prompts and last 10 messages, compresses middle content.
func smartCompactMapConversation(conversation []map[string]interface{}, maxTokens int) []map[string]interface{} {
	if estimateMapTokens(conversation) <= maxTokens {
		return conversation
	}

	result := make([]map[string]interface{}, 0, len(conversation))

	for i, msg := range conversation {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)

		// System prompts always preserved
		if role == "system" {
			result = append(result, msg)
			continue
		}

		// Last 10 messages preserved in full
		if i >= len(conversation)-10 {
			result = append(result, msg)
			continue
		}

		// Middle rounds: compress long messages
		if len(content) > 500 && role == "assistant" {
			compressed := content[:200] + "\n... [smart-compressed]"
			newMsg := make(map[string]interface{})
			for k, v := range msg {
				newMsg[k] = v
			}
			newMsg["content"] = compressed
			result = append(result, newMsg)
			continue
		}

		if len(content) > 300 && role == "tool" {
			compressed := content[:150] + "\n... [tool result compressed]"
			newMsg := make(map[string]interface{})
			for k, v := range msg {
				newMsg[k] = v
			}
			newMsg["content"] = compressed
			result = append(result, newMsg)
			continue
		}

		result = append(result, msg)
	}
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
