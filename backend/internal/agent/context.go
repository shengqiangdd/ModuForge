package agent

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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

	// Tell the agent where it can actually write files. Without this the agent
	// blind-guesses absolute paths and often concludes the FS is read-only.
	if cfg.ProjectID != "" {
		storageRoot := os.Getenv("STORAGE_PATH")
		if storageRoot == "" {
			storageRoot = "/data/storage"
		}
		workdir := filepath.Join(storageRoot, "projects", cfg.ProjectID)
		sb.WriteString(fmt.Sprintf("Working directory (writable): %s\n", workdir))
		sb.WriteString(`写文件请一律使用「项目内相对路径」（例如 polyglot/goapp/main.go），不要使用 /data、/app 等绝对路径。
bash 命令默认在项目工作目录执行（cwd = 上面给出的工作目录），因此：
- 用 ` + "`pwd`" + ` 即可确认当前模型的工作目录；
- 用 ` + "`ls .`" + ` 查看项目现有文件，不要探测 / 下的系统路径；
- 构建命令（go build / cargo build / gcc）直接在相对路径下运行即可。
`)
	}

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

