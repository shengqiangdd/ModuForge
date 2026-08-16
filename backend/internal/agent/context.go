package agent

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"unsafe"

	"github.com/moduforge/backend/internal/agent/prompts"
)

// conversationSizeCache caches the estimated size of a conversation to avoid
// repeated full scans. Key: pointer to first element (identity), Value: size.
// This avoids the double-scan problem where estimateConversationSize is called
// twice per iteration (once for compaction check, once implicitly via prefilter).
type conversationSizeCache struct {
	lastPointer uintptr
	lastLen     int
	lastSize    int
}

var globalConvSizeCache = &conversationSizeCache{}

func (r *AgentRunner) estimateConversationSize(conversation []map[string]interface{}) int {
	// Fast path: if conversation pointer and length haven't changed, return cached size
	if len(conversation) > 0 {
		ptr := uintptr(unsafe.Pointer(&conversation[0]))
		if ptr == globalConvSizeCache.lastPointer && len(conversation) == globalConvSizeCache.lastLen {
			return globalConvSizeCache.lastSize
		}
	}

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

	// Cache the result
	if len(conversation) > 0 {
		ptr := uintptr(unsafe.Pointer(&conversation[0]))
		globalConvSizeCache.lastPointer = ptr
		globalConvSizeCache.lastLen = len(conversation)
		globalConvSizeCache.lastSize = total
	}

	return total
}

// ═══════════════════════════════════════════════════════════════════
// System Prompt — mode-aware (loads from MD files)
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
	// Load prompts from MD files
	modeStr := "act"
	switch mode {
	case ModePlan:
		modeStr = "plan"
	case ModeAct:
		modeStr = "act"
	}

	p, err := prompts.Load(modeStr)
	if err != nil {
		// Fallback to hardcoded if MD loading fails
		log.Printf("[Prompts] Failed to load MD prompts, using fallback: %v", err)
		return r.buildFallbackPrompt(mode)
	}

	return p.Full
}

// buildFallbackPrompt provides a minimal fallback if MD loading fails
func (r *AgentRunner) buildFallbackPrompt(mode AgentMode) string {
	if mode == ModePlan {
		return `You are an AI coding agent in PLAN MODE (read-only). Analyze code and create implementation plans.

## RULES
- CANNOT modify files or execute write commands
- Read files to understand current state before planning
- Break tasks into clear, actionable steps with file lists
- Identify risks and edge cases

## OUTPUT FORMAT
Your FINAL answer MUST be clean Markdown inside <answer> tags.
Do NOT output raw tool call syntax.`
	}

	return `You are an expert AI coding agent in ACT MODE. You have FULL access to read, write, and build files.

## 3 NON-NEGOTIABLE RULES
1. If a task requires file changes, you MUST call write_file for EACH file.
2. After writing code, you MUST call build_module to verify it compiles.
3. Your FINAL answer lists files you ACTUALLY wrote.

## WORKFLOW
1. read_file → understand current state
2. write_file → create/modify each file
3. build_module → verify compilation
4. If build fails: fix and rebuild (max 3 retries)

## CRITICAL
You are evaluated on whether you ACTUALLY WROTE FILES AND VERIFIED THE BUILD.`
}

// ═══════════════════════════════════════════════════════════════════
// Project Context
// ═══════════════════════════════════════════════════════════════════

func (r *AgentRunner) buildProjectContext(cfg RunConfig) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n## CURRENT PROJECT\nProject ID: %s\n", cfg.ProjectID))

	if cfg.ProjectContext != "" {
		sb.WriteString(fmt.Sprintf("Description:\n%s\n", cfg.ProjectContext))
	}

	// Use repo-map summary instead of full file listing (saves tokens)
	if r.repoMap != nil {
		repoMapSummary := r.repoMap.GetRepoMapSummary()
		if repoMapSummary != "" {
			sb.WriteString("\n" + repoMapSummary + "\n")
		}
	} else if r.db != nil {
		// Fallback: list files from DB if repo-map not available
		rows, err := r.db.Query(
			`SELECT path, COALESCE(file_size, length(content), 0) as size FROM project_files WHERE project_id=? ORDER BY path`,
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
3. Use edit_file for targeted changes (preferred) or write_file for full rewrites (project_id auto-injected)
4. ONLY modify files that need changes — do NOT rewrite unchanged files
5. After making changes, call build_module to verify
6. Explain what you changed

## ⚡ MANDATORY: WRITE CODE — DO NOT JUST READ

You are an ENGINEER, not a REVIEWER. Your job is to MODIFY CODE, not just read it.
- After reading 3-5 files, you MUST start writing/editing code
- NEVER read the same file more than 2 times — if you need it again, use edit_file directly
- Your task is COMPLETE ONLY when you have called write_file, edit_file, OR build_module
- If you only read files without writing, you have FAILED the task
- The system WILL force you to answer if you read more than 10 times without writing

## CRITICAL RULES
- NEVER call write_file without first reading the file with read_file (once only)
- NEVER read the same file more than 2 times — use edit_file for changes
- NEVER write empty or whitespace-only content
- NEVER rewrite files that don't need changes
- You MUST call at least one write/edit tool before providing your answer`)

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

// isFileChangeConfirmation checks if an assistant message confirms a file change
func isFileChangeConfirmation(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "written") ||
		strings.Contains(lower, "modified") ||
		strings.Contains(lower, "created") ||
		strings.Contains(lower, "updated")
}

// isErrorReport checks if a message is reporting an error
func isErrorReport(content string) bool {
	return strings.HasPrefix(content, "Error:") ||
		strings.HasPrefix(content, "❌") ||
		strings.HasPrefix(content, "⚠️") ||
		strings.Contains(content, "failed") ||
		strings.Contains(content, "error")
}

// isPlanOutline checks if a message is a plan outline
func isPlanOutline(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "plan") ||
		strings.Contains(lower, "approach") ||
		strings.Contains(lower, "step") ||
		strings.Contains(lower, "implement")
}

// truncateMessage truncates a message to maxLen, adding ellipsis if truncated
func truncateMessage(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}
