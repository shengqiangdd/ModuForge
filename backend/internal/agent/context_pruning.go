package agent

import (
	"fmt"
	"log"
	"strings"

	"github.com/moduforge/backend/internal/agent/prompts"
)

func prefilterConversation(conversation []map[string]interface{}) []map[string]interface{} {
	if len(conversation) <= 2 {
		return conversation
	}

	// Optimization 43: If conversation is very long (>30 messages), apply aggressive pruning
	// Keep last 10 messages in full, compress older ones to essential info only
	if len(conversation) > 30 {
		return smartPruneConversation(conversation)
	}

	// Optimization 44: Truncate old tool results to save tokens
	// This is a lightweight operation that runs before the more expensive filtering
	if len(conversation) > 6 {
		conversation = smartTruncateOldToolResults(conversation)
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

	// OPTIMIZED: Single-pass filtering with inline tool_call tracking
	// Instead of 4 separate passes, we do everything in 2 passes:
	// Pass 1: Filter messages AND collect tool_call/response relationships
	// Pass 2: Fix consistency (remove orphaned tool messages)

	// Track which tool_call_ids have responses and which assistant tool_calls exist
	toolCallIDsSeen := make(map[string]bool)    // tool_call_ids from assistant messages
	toolResponseIDsSeen := make(map[string]bool) // tool_call_ids from tool responses

	for _, msg := range conversation {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)

		// Track tool_call relationships
		if role == "assistant" {
			if toolCalls, ok := msg["tool_calls"].([]LLMToolCall); ok && len(toolCalls) > 0 {
				for _, tc := range toolCalls {
					toolCallIDsSeen[tc.ID] = true
				}
			}
		}
		if role == "tool" {
			if toolCallID, ok := msg["tool_call_id"].(string); ok && toolCallID != "" {
				toolResponseIDsSeen[toolCallID] = true
			}
		}

		// Skip empty tool results (waste tokens) - BUT only if not required by tool_calls
		if role == "tool" && strings.TrimSpace(content) == "" {
			if toolCallID, ok := msg["tool_call_id"].(string); ok && toolCallID != "" {
				if toolCallIDsSeen[toolCallID] {
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
			msgCopy := make(map[string]interface{}, len(msg))
			for k, v := range msg {
				msgCopy[k] = v
			}
			msgCopy["content"] = newContent
			msg = msgCopy
		}

		result = append(result, msg)
		prevRole = role
		prevContent = content
	}

	// Pass 2: Fix tool_calls/tool response consistency (single pass)
	// Remove tool_calls from assistant messages if their tool responses are missing
	// Remove orphaned tool messages (tool responses without preceding tool_calls)
	for i, msg := range result {
		role, _ := msg["role"].(string)

		if role == "assistant" {
			if toolCalls, ok := msg["tool_calls"].([]LLMToolCall); ok && len(toolCalls) > 0 {
				var validCalls []LLMToolCall
				for _, tc := range toolCalls {
					// Keep if: no ID (shouldn't happen) OR response exists in filtered result
					if tc.ID == "" || toolResponseIDsSeen[tc.ID] {
						validCalls = append(validCalls, tc)
					} else {
						skipped++
					}
				}
				if len(validCalls) != len(toolCalls) {
					result[i] = copyMap(msg)
					result[i]["tool_calls"] = validCalls
				}
			}
		}

		// Remove orphaned tool messages (tool responses without preceding tool_calls)
		if role == "tool" {
			if toolCallID, ok := msg["tool_call_id"].(string); ok && toolCallID != "" {
				if !toolCallIDsSeen[toolCallID] {
					// Orphaned: tool response without assistant tool_call
					result[i] = nil // mark for removal
					skipped++
				}
			}
		}
	}

	// Compact result to remove nil entries (orphaned tool messages)
	if skipped > 0 {
		compactIdx := 0
		for _, msg := range result {
			if msg != nil {
				result[compactIdx] = msg
				compactIdx++
			}
		}
		result = result[:compactIdx]
	}

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

// Optimization 44: Smart Tool Result Truncation + P1-1 enhancement
// For old tool results (>6 messages ago), truncate to essential info only.
// P1-1: Enhanced to compress read_file results more aggressively.
func smartTruncateOldToolResults(conversation []map[string]interface{}) []map[string]interface{} {
	if len(conversation) <= 6 {
		return conversation
	}

	result := make([]map[string]interface{}, len(conversation))
	truncated := 0

	for i, msg := range conversation {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)

		// Only truncate old tool results (more than 6 messages ago)
		if role == "tool" && i < len(conversation)-6 && len(content) > 500 {
			var truncatedContent string
			// P1-1: More aggressive truncation for write/edit results
			// These contain success messages that can be safely compressed
			if strings.Contains(content, "File written successfully:") || strings.Contains(content, "File edited:") {
				// Write/edit results: keep only status message
				truncatedContent = content
				if len(truncatedContent) > 300 {
					truncatedContent = truncatedContent[:300] + "..."
				}
			} else if len(content) > 4000 {
				// Large tool results: keep first 500 chars + last 200 chars
				truncatedContent = content[:500] + "\n... [truncated to save context] ...\n" + content[len(content)-200:]
			} else {
				// Normal truncation: keep first 200 chars + last 100 chars
				truncatedContent = content[:200] + "\n... [truncated for efficiency] ...\n" + content[len(content)-100:]
			}
			msgCopy := copyMap(msg)
			msgCopy["content"] = truncatedContent
			result[i] = msgCopy
			truncated++
		} else {
			result[i] = msg
		}
	}

	if truncated > 0 {
		log.Printf("[Optimization44] truncated %d old tool results to save tokens", truncated)
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

		// Extract file write/edit results
		if isFileChangeResult(content) {
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
	// Try loading from MD files first
	p, err := prompts.Load("free")
	if err == nil {
		return p.Full
	}

	// Fallback to hardcoded
	if mode == ModePlan {
		return `You are a coding agent in PLAN MODE (read-only). Analyze code and create plans.
- CANNOT modify files. Read only.
- Break tasks into clear steps with file lists.
- Output final plan as clean Markdown.`
	}

	return `You are a coding agent in ACT MODE with file access.

## ⚠️ CRITICAL: YOU MUST USE TOOLS
Your job is to MODIFY CODE, not analyze it.

### MANDATORY WORKFLOW:
1. read_file → understand current code
2. write_file or edit_file → MAKE THE CHANGES
3. build_module → verify compilation
4. If build fails → fix → rebuild (max 3 retries)

### YOUR RESPONSE MUST INCLUDE write_file OR edit_file CALL
If you only read files and don't write, you FAILED.

### TOOLS
- edit_file(path, old_text, new_text) — for changes to existing files
- write_file(path, content) — for new files or complete rewrites
- build_module(project_id) — compile and package

### WHEN BUILD FAILS
1. Read the error message
2. Fix with edit_file
3. Rebuild

NEVER output plans without writing. NEVER skip build_module.`
}

// ═══════════════════════════════════════════════════════════════════
// Helper Functions
// ═══════════════════════════════════════════════════════════════════
