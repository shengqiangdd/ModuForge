package agent

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"unsafe"
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

## CHAIN-OF-THOUGHT
Before calling any tool, think through the problem step by step. Explain your reasoning briefly before each action.

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

## CHAIN-OF-THOUGHT
Before calling any tool, think through the problem step by step. Explain your reasoning briefly before each action.

## WORKFLOW (follow this order, never skip steps)
1. read_file → understand current state
2. write_file → create/modify each file (COMPLETE content, not snippets)
3. build_module → verify compilation
4. If build fails: read error, fix with write_file, rebuild (max 3 retries)
5. test_module → validate module files (module.prop, shell scripts); if test files exist (*_test.go, *_test.rs, *_spec.js, etc.) run them via bash (go test ./... / cargo test / npm test)
6. If tests fail: read error, fix with write_file, rebuild and retest
7. Answer: "I modified X files: [list]. Build status: [pass/fail]. Tests: [pass/fail]."

## ANTI-PATTERNS (NEVER do these)
- Outputting a "plan" without calling write_file
- Listing "missing files" without creating them
- Saying "需要创建 X" without write_file
- Reading files and only outputting analysis
- Skipping build_module after writing code
- Claiming tests pass without running them

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
- write_file_batch(files) → write many files in one transaction
- grep_search(pattern) → search code across all files (like grep -rn)
- glob_search(pattern) → find files by name (e.g., "**/*.go")
- list_dir(path) → list files in a directory (project structure)
- bash(command) → run shell commands (build, test, git) — e.g. go test ./..., cargo test
- build_module(project_id) → compile + package ZIP
- test_module(files, test_type) → validate module files (module.prop, shell syntax, permissions)
- delete_file(path) / delete_dir(path) / move_file(source, destination) → file management

## TOOL RULES
- edit_file for changes <30% of file (MOST common)
- write_file for new files or complete rewrites ONLY
- ALWAYS read_file BEFORE edit_file/write_file
- grep_search/glob_search help locate code; read_file directly is fine when you already know the path
- After writing, ALWAYS call build_module
- If tests exist, run test_module and language tests (go test/cargo test) after a successful build
- build_module fails? Read error → fix → rebuild (max 3 retries)

## WORKFLOW
1. grep_search → find relevant code (or read_file directly if the path is known)
2. read_file → understand context
3. edit_file → make changes (or write_file for new files)
4. build_module → verify compilation
5. test_module → validate module files; run language tests if test files exist
6. Report: files changed + build status + test status

## CODE STYLE
- Match the existing style of the files you modify
- Go: gofmt format, run go vet; keep functions small with meaningful names
- Rust: rustfmt format (cargo fmt), consider cargo clippy
- Shell: follow POSIX /bin/sh style for Magisk scripts, quote variables ("$VAR")
- JavaScript/TypeScript: prettier/eslint conventions
- Never leave dead code, debug prints, or TODO stubs behind

CRITICAL: You are evaluated on whether you ACTUALLY WROTE FILES, VERIFIED THE BUILD, AND RAN THE TESTS.
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

	// OPTIMIZED: Single-pass filtering with inline tool_call tracking
	// Instead of 4 separate passes, we do everything in 2 passes:
	// Pass 1: Filter messages AND collect tool_call/response relationships
	// Pass 2: Fix consistency (remove orphaned tool messages)

	// Track which tool_call_ids have responses and which assistant tool_calls exist
	toolCallIDsSeen := make(map[string]bool)   // tool_call_ids from assistant messages
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
			msg = copyMap(msg)
			msg["content"] = newContent
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
5. test_module → validate module files; run language tests (go test/cargo test) if test files exist
6. Answer: files modified + build status + test status

NEVER output plans without writing. NEVER skip build_module.`
}
