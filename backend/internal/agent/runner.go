package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/moduforge/backend/internal/service"
)

// agentDebug enables verbose logging for hot-path operations.
// Set MODUFORGE_DEBUG=1 to enable.
var agentDebug = os.Getenv("MODUFORGE_DEBUG") == "1"

// Optimization 42: Goroutine leak detector
// Tracks active goroutines and logs warnings if they accumulate beyond a threshold.
// Helps detect leaks from unfinished tool executions or LLM streaming.
var (
	activeGoroutines int64
	goroutineWarnAt  = 50 // warn if more than 50 goroutines active
)

func incGoroutines() int64 { return atomic.AddInt64(&activeGoroutines, 1) }
func decGoroutines()       { atomic.AddInt64(&activeGoroutines, -1) }

func checkGoroutineLeak() {
	n := atomic.LoadInt64(&activeGoroutines)
	if n > int64(goroutineWarnAt) {
		log.Printf("[Agent:LEAK] ⚠️ %d goroutines active (threshold=%d)", n, goroutineWarnAt)
	}
}

func debugLog(format string, args ...interface{}) {
	if agentDebug {
		log.Printf("[Agent:DEBUG] "+format, args...)
	}
}

const (
	defaultMaxIterations = 200
	defaultMaxResultLen  = 32768
	totalTimeout         = 1800 * time.Second // 30 minutes for complex tasks
	maxHistoryChars      = 60000
	perIterationTimeout  = 90 * time.Second // max time for a single iteration (LLM + tools)

	// Optimization 37: Per-tool execution timeouts
	// Fast tools (read-only, in-memory) get short timeouts; slow tools (build, compile) get longer ones.
	toolTimeoutFast  = 30 * time.Second  // read-only tools (read_file, list_dir, grep_search, glob_search)
	toolTimeoutWrite = 60 * time.Second  // write_file, edit_file, delete_file, etc.
	toolTimeoutSlow  = 300 * time.Second // build_module, test_module
)

// toolTimeoutForName returns the appropriate timeout for a given tool.
func toolTimeoutForName(name string) time.Duration {
	switch name {
	case "read_file", "list_dir", "grep_search", "glob_search":
		return toolTimeoutFast
	case "write_file", "write_file_batch", "edit_file", "delete_file", "delete_dir", "move_file":
		return toolTimeoutWrite
	case "syntax_checker":
		return 120 * time.Second // syntax checks need some time for compilation
	case "build_module", "test_module":
		return toolTimeoutSlow
	default:
		return toolTimeoutWrite // safe default
	}
}

// ═══════════════════════════════════════════════════════════════════
// SSE Writer Interface
// ═══════════════════════════════════════════════════════════════════

type SSEWriter interface {
	WriteSSE(data map[string]interface{}) error
	WriteSSEPlain(data string) error
	WriteSSEComment(comment string) error
	Flush() error
	IsDisconnected() bool
}

// ═══════════════════════════════════════════════════════════════════
// Agent Mode — Plan/Act separation (inspired by Cline/Roo Code)
//
// Plan mode: read-only exploration, no write_file/execute
// Act mode: full tool access including writes
// ═══════════════════════════════════════════════════════════════════

type AgentMode string

const (
	ModePlan AgentMode = "plan"
	ModeAct  AgentMode = "act"
)

// ═══════════════════════════════════════════════════════════════════
// File Checkpoint — for undo support (inspired by Cline)
// ═══════════════════════════════════════════════════════════════════

type FileCheckpoint struct {
	Path    string
	Content string
	Time    time.Time
}

// ═══════════════════════════════════════════════════════════════════
// RunConfig — per-request immutable config
// ═══════════════════════════════════════════════════════════════════

type RunConfig struct {
	UserID          string
	ProjectID       string
	ProjectContext  string
	MaxIterations   int
	MaxResultLen    int
	Mode            AgentMode // "plan" or "act" — controls tool availability
	ProviderID      string    // provider ID (e.g., "opencode-go") for DB lookups
	LLMEndpoint     string    // resolved endpoint (from handler, overrides DB lookup)
	LLMApiKey       string    // resolved API key
	LLMModel        string    // resolved model ID
	MaxOutputTokens int       // max output tokens (0 = use model default)

	// P0-Optimization: Cached resolved LLM config (endpoint, apiKey, model).
	// Populated once at Run() entry to avoid repeated DB queries per iteration.
	resolvedEndpoint string
	resolvedAPIKey   string
	resolvedModel    string
	modelTier        ModelTier
}

type AgentRunner struct {
	registry    *SkillRegistry
	apiKey      string
	endpoint    string
	model       string
	db          *sql.DB
	convStore   *service.ConversationStore
	memoryStore *service.MemoryStore
	memV2Store  *service.MemoryV2Store

	// Optimization 13: Tool definition cache (avoids rebuilding every iteration)
	toolDefCache   map[string][]ToolDef // key: mode+":"+modelName -> cached defs
	toolDefCacheMu sync.RWMutex

	// Optimization 16: Session-scoped tool result cache (persists across Run() calls)
	sessionCaches sync.Map // sessionID -> *toolResultCache
	// Optimization 17: write_file content cache (avoids redundant read_file after write)
	writeContentCache sync.Map // sessionID -> map[string]string (path -> content)
	// read_file content cache (avoids redundant disk reads within a session)
	readFileCache sync.Map // sessionID -> map[string]string (path -> content)
	// Optimization 1: Session access time tracking for TTL-based cleanup
	sessionAccessTimes sync.Map // sessionID -> time.Time (last access timestamp)

	// NEW: Enhanced modules
	auditLog       *AuditLog
	permChecker    *PermissionChecker
	sessionPersist *SessionPersistence
	depGraph       *DependencyGraph

	// Optimization 51: Performance metrics tracking
	perfMetrics *PerformanceMetrics

	// File hash cache for UNCHANGED detection in read_file
	fileHashCache *fileHashCache

	// Repo-map for global code structure indexing
	repoMap *RepoMap
}

func NewAgentRunner(registry *SkillRegistry, apiKey, endpoint, model string, db *sql.DB) *AgentRunner {
	r := &AgentRunner{
		registry:      registry,
		apiKey:        apiKey,
		endpoint:      endpoint,
		model:         model,
		db:            db,
		convStore:     service.NewConversationStore(),
		toolDefCache:  make(map[string][]ToolDef),
		fileHashCache: NewFileHashCache(),
		// Initialize new modules
		auditLog:       NewAuditLog(""),
		permChecker:    NewPermissionChecker(),
		sessionPersist: NewSessionPersistence(""),
		depGraph:       NewDependencyGraph(),
		perfMetrics:    NewPerformanceMetrics(),
	}
	go r.startSessionCacheCleanup()
	return r
}

func NewRunConfig(userID string) RunConfig {
	return RunConfig{
		UserID:        userID,
		MaxIterations: defaultMaxIterations,
		MaxResultLen:  defaultMaxResultLen,
		Mode:          ModeAct,
	}
}

func (r *AgentRunner) SetMemoryStore(ms *service.MemoryStore) {
	r.memoryStore = ms
}

// ═══════════════════════════════════════════════════════════════════
// P2-1: TaskDecomposer — Break complex tasks into subtasks
// ═══════════════════════════════════════════════════════════════════

// TaskDecomposer analyzes a task and breaks it into manageable subtasks.
type TaskDecomposer struct {
	db *sql.DB
}

// Subtask represents a piece of a larger task.
type Subtask struct {
	ID           string   `json:"id"`
	Description  string   `json:"description"`
	Status       string   `json:"status"` // pending, in_progress, completed, failed
	Dependencies []string `json:"dependencies"`
	Files        []string `json:"files,omitempty"`    // involved files
	Progress     int      `json:"progress,omitempty"` // 0-100
	StartedAt    int64    `json:"started_at,omitempty"`
	CompletedAt  int64    `json:"completed_at,omitempty"`
	RetryCount   int      `json:"retry_count,omitempty"`
}

// decomposeWithLLM asks the LLM to break down a complex task into structured subtasks.
// Returns nil if LLM is unavailable or fails.
func (r *AgentRunner) decomposeWithLLM(ctx context.Context, task string, projectContext string, cfg RunConfig) []Subtask {
	prompt := fmt.Sprintf(`Analyze the following task and break it into a list of concrete subtasks.
Return ONLY a JSON array (no markdown, no explanation). Each element must have:
- "id": short lowercase snake_case identifier (e.g. "analyze", "create_file", "verify")
- "description": one-line Chinese description of what to do
- "dependencies": array of id strings that must complete first (empty array if none)
- "files": array of file paths likely involved (empty array if unknown)

Task: %s

Project context: %s

Return the JSON array now:`, task, projectContext)

	summaryPrompt := []map[string]string{
		{"role": "system", "content": "You are a task planning assistant. Output only valid JSON arrays."},
		{"role": "user", "content": prompt},
	}

	result, err := r.callLLMSummary(ctx, cfg, summaryPrompt)
	if err != nil {
		log.Printf("[Agent] LLM task decomposition failed: %v", err)
		return nil
	}

	// Debug: log raw LLM response
	log.Printf("[Agent] LLM task decomposition raw response (len=%d): %s", len(result), result)

	// Try to extract JSON array from response
	result = strings.TrimSpace(result)
	// Strip markdown code fences if present
	if idx := strings.Index(result, "```"); idx >= 0 {
		result = strings.TrimPrefix(result[idx:], "```")
		result = strings.TrimPrefix(result, "json\n")
		result = strings.TrimPrefix(result, "json\r\n")
		if endIdx := strings.LastIndex(result, "```"); endIdx >= 0 {
			result = result[:endIdx]
		}
		result = strings.TrimSpace(result)
	}

	var raw []struct {
		ID           string   `json:"id"`
		Description  string   `json:"description"`
		Dependencies []string `json:"dependencies"`
		Files        []string `json:"files"`
	}
	if err := json.Unmarshal([]byte(result), &raw); err != nil {
		log.Printf("[Agent] LLM task decomposition parse failed: %v (raw=%s)", err, result)
		return nil
	}

	if len(raw) == 0 {
		return nil
	}

	subtasks := make([]Subtask, 0, len(raw))
	for i, r := range raw {
		id := r.ID
		if id == "" {
			id = fmt.Sprintf("step_%d", i)
		}
		subtasks = append(subtasks, Subtask{
			ID:           id,
			Description:  r.Description,
			Status:       "pending",
			Dependencies: r.Dependencies,
			Files:        r.Files,
		})
	}

	// Validate dependencies reference existing IDs
	idSet := make(map[string]bool)
	for _, s := range subtasks {
		idSet[s.ID] = true
	}
	for i := range subtasks {
		validDeps := subtasks[i].Dependencies[:0]
		for _, dep := range subtasks[i].Dependencies {
			if idSet[dep] {
				validDeps = append(validDeps, dep)
			}
		}
		subtasks[i].Dependencies = validDeps
	}

	log.Printf("[Agent] LLM decomposed task into %d subtasks", len(subtasks))
	return subtasks
}

// isComplexTask determines whether a task warrants LLM decomposition.
// Simple tasks (single action, short) are handled by keyword fallback.
func isComplexTask(task string) bool {
	taskLower := strings.ToLower(task)
	// Short tasks are not complex
	if len(task) < 15 {
		return false
	}
	// Tasks with multiple action verbs are complex
	actionCount := 0
	for _, kw := range []string{"and", "then", "also", "additionally", "同时", "并且", "然后", "接着",
		"create", "implement", "fix", "refactor", "add", "build", "optimize", "migrate",
		"实现", "创建", "修复", "重构", "优化", "迁移", "添加", "构建"} {
		if strings.Contains(taskLower, kw) {
			actionCount++
		}
	}
	if actionCount >= 2 {
		return true
	}
	// Tasks with enumeration markers (、) suggesting multiple items
	if strings.Count(task, "\u3001") >= 2 {
		return true
	}
	// Tasks mentioning "包含" (containing) with multiple features
	if strings.Contains(taskLower, "\u5305\u542b") || strings.Contains(taskLower, "include") {
		return true
	}
	// Long tasks are likely complex
	if len(task) > 80 {
		return true
	}
	return false
}

// DecomposeTask breaks a complex task into subtasks.
// Optimization 50: Enhanced task decomposition with more patterns and dependency tracking
func (td *TaskDecomposer) DecomposeTask(task string, projectContext string) []Subtask {
	subtasks := make([]Subtask, 0)

	// Simple heuristic: detect common patterns
	taskLower := strings.ToLower(task)

	// Pattern: "create X" -> analyze, implement, test
	if strings.Contains(taskLower, "create") || strings.Contains(taskLower, "implement") || strings.Contains(taskLower, "add") {
		subtasks = append(subtasks, Subtask{
			ID:          "analyze",
			Description: "分析需求和现有代码结构",
			Status:      "pending",
		})
		subtasks = append(subtasks, Subtask{
			ID:           "implement",
			Description:  "实现功能代码",
			Status:       "pending",
			Dependencies: []string{"analyze"},
		})
		subtasks = append(subtasks, Subtask{
			ID:           "verify",
			Description:  "验证编译和功能",
			Status:       "pending",
			Dependencies: []string{"implement"},
		})
	}

	// Pattern: "fix X" -> diagnose, fix, test
	if strings.Contains(taskLower, "fix") || strings.Contains(taskLower, "repair") || strings.Contains(taskLower, "debug") {
		subtasks = append(subtasks, Subtask{
			ID:          "diagnose",
			Description: "诊断问题原因",
			Status:      "pending",
		})
		subtasks = append(subtasks, Subtask{
			ID:           "fix",
			Description:  "修复问题",
			Status:       "pending",
			Dependencies: []string{"diagnose"},
		})
		subtasks = append(subtasks, Subtask{
			ID:           "verify",
			Description:  "验证修复",
			Status:       "pending",
			Dependencies: []string{"fix"},
		})
	}

	// Pattern: "refactor X" -> analyze, plan, execute, verify
	if strings.Contains(taskLower, "refactor") || strings.Contains(taskLower, "optimize") || strings.Contains(taskLower, "improve") {
		subtasks = append(subtasks, Subtask{
			ID:          "analyze",
			Description: "分析当前代码",
			Status:      "pending",
		})
		subtasks = append(subtasks, Subtask{
			ID:           "plan",
			Description:  "制定重构计划",
			Status:       "pending",
			Dependencies: []string{"analyze"},
		})
		subtasks = append(subtasks, Subtask{
			ID:           "execute",
			Description:  "执行重构",
			Status:       "pending",
			Dependencies: []string{"plan"},
		})
		subtasks = append(subtasks, Subtask{
			ID:           "verify",
			Description:  "验证重构结果",
			Status:       "pending",
			Dependencies: []string{"execute"},
		})
	}

	// Optimization 50: New patterns for common tasks
	// Pattern: "test X" -> analyze, write tests, run tests
	if strings.Contains(taskLower, "test") || strings.Contains(taskLower, "测试") {
		subtasks = append(subtasks, Subtask{
			ID:          "analyze",
			Description: "分析需要测试的代码",
			Status:      "pending",
		})
		subtasks = append(subtasks, Subtask{
			ID:           "write_tests",
			Description:  "编写测试用例",
			Status:       "pending",
			Dependencies: []string{"analyze"},
		})
		subtasks = append(subtasks, Subtask{
			ID:           "run_tests",
			Description:  "运行测试并验证",
			Status:       "pending",
			Dependencies: []string{"write_tests"},
		})
	}

	// Pattern: "document X" -> analyze, write docs, review
	if strings.Contains(taskLower, "document") || strings.Contains(taskLower, "文档") || strings.Contains(taskLower, "readme") {
		subtasks = append(subtasks, Subtask{
			ID:          "analyze",
			Description: "分析代码结构和功能",
			Status:      "pending",
		})
		subtasks = append(subtasks, Subtask{
			ID:           "write_docs",
			Description:  "编写文档",
			Status:       "pending",
			Dependencies: []string{"analyze"},
		})
		subtasks = append(subtasks, Subtask{
			ID:           "review",
			Description:  "审查文档质量",
			Status:       "pending",
			Dependencies: []string{"write_docs"},
		})
	}

	// Pattern: "migrate X" -> analyze, plan, execute, verify
	if strings.Contains(taskLower, "migrate") || strings.Contains(taskLower, "迁移") || strings.Contains(taskLower, "升级") {
		subtasks = append(subtasks, Subtask{
			ID:          "analyze",
			Description: "分析迁移需求和影响",
			Status:      "pending",
		})
		subtasks = append(subtasks, Subtask{
			ID:           "plan",
			Description:  "制定迁移计划",
			Status:       "pending",
			Dependencies: []string{"analyze"},
		})
		subtasks = append(subtasks, Subtask{
			ID:           "execute",
			Description:  "执行迁移",
			Status:       "pending",
			Dependencies: []string{"plan"},
		})
		subtasks = append(subtasks, Subtask{
			ID:           "verify",
			Description:  "验证迁移结果",
			Status:       "pending",
			Dependencies: []string{"execute"},
		})
	}

	// If no pattern matched, treat as single task
	if len(subtasks) == 0 {
		subtasks = append(subtasks, Subtask{
			ID:          "complete",
			Description: task,
			Status:      "pending",
		})
	}

	return subtasks
}

// GetNextSubtask returns the next subtask to execute based on dependencies.
func (td *TaskDecomposer) GetNextSubtask(subtasks []Subtask) *Subtask {
	completed := make(map[string]bool)
	for _, st := range subtasks {
		if st.Status == "completed" {
			completed[st.ID] = true
		}
	}

	for i := range subtasks {
		if subtasks[i].Status != "pending" {
			continue
		}
		// Check if all dependencies are completed
		allDepsMet := true
		for _, dep := range subtasks[i].Dependencies {
			if !completed[dep] {
				allDepsMet = false
				break
			}
		}
		if allDepsMet {
			return &subtasks[i]
		}
	}

	return nil // all done or blocked
}

// ═══════════════════════════════════════════════════════════════════
// P2-2: QualityVerifier — Verify code quality
// ═══════════════════════════════════════════════════════════════════

// QualityVerifier checks code quality metrics.
type QualityVerifier struct {
	db *sql.DB
}

// QualityReport contains quality metrics for a file.
type QualityReport struct {
	FilePath    string
	Lines       int
	Complexity  int  // cyclomatic complexity estimate
	Duplication bool // has duplicated code patterns
	HasTests    bool
	HasComments bool
	Score       int // 0-100 quality score
	Issues      []string
}

// VerifyFile checks the quality of a file using O(n) single-pass analysis.
// Instead of 4 separate passes through lines, we do everything in one pass.
func (qv *QualityVerifier) VerifyFile(filePath string, content string) QualityReport {
	report := QualityReport{
		FilePath: filePath,
		Lines:    strings.Count(content, "\n") + 1,
		Issues:   make([]string, 0),
	}

	lines := strings.Split(content, "\n")

	// ═══════════════════════════════════════════════════════════════
	// Single-pass analysis: collect all metrics in one iteration
	// O(n) total instead of O(4n) = O(n)
	// ═══════════════════════════════════════════════════════════════

	longLines := 0
	todoCount := 0
	braceCount := 0
	maxBraceDepth := 0
	magicNumbers := 0
	hasPackage := false
	hasFunc := false
	hasInclude := false
	hasMain := false
	hasFn := false
	importParens := 0
	inImport := false
	inBlockComment := false
	hasSetE := false
	unquotedVarCount := 0
	doubleSemicolonCount := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineUpper := strings.ToUpper(trimmed)

		// ── Universal checks (all languages) ──

		// 1. Line length
		if len(line) > 120 {
			longLines++
		}

		// 2. TODO/FIXME/HACK
		if strings.Contains(lineUpper, "TODO") || strings.Contains(lineUpper, "FIXME") || strings.Contains(lineUpper, "HACK") {
			todoCount++
		}

		// 3. Brace balance (skip comments)
		if !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "#") {
			for _, ch := range line {
				if ch == '{' {
					braceCount++
					if braceCount > maxBraceDepth {
						maxBraceDepth = braceCount
					}
				} else if ch == '}' {
					braceCount--
				}
			}
		}

		// 4. Magic numbers (skip comments)
		if !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "#") {
			words := strings.Fields(trimmed)
			for _, word := range words {
				if len(word) > 1 && word[0] >= '2' && word[0] <= '9' {
					magicNumbers++
				}
			}
		}

		// ── Language-specific checks (single pass) ──

		// Handle block comments
		if strings.Contains(trimmed, "/*") {
			inBlockComment = true
		}
		if strings.Contains(trimmed, "*/") {
			inBlockComment = false
			continue
		}
		if inBlockComment || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Go-specific
		if strings.HasPrefix(trimmed, "package ") {
			hasPackage = true
		}
		if trimmed == "import (" {
			inImport = true
		}
		if inImport {
			for _, ch := range trimmed {
				if ch == '(' {
					importParens++
				} else if ch == ')' {
					importParens--
					if importParens <= 0 {
						inImport = false
					}
				}
			}
		}
		if strings.HasPrefix(trimmed, "func ") {
			hasFunc = true
		}

		// Rust-specific
		if strings.HasPrefix(trimmed, "fn ") || strings.Contains(trimmed, " fn ") {
			hasFn = true
		}

		// C/C++-specific
		if strings.HasPrefix(trimmed, "#include") {
			hasInclude = true
		}
		if strings.Contains(trimmed, "main(") {
			hasMain = true
		}

		// Shell-specific
		if i == 0 && strings.HasPrefix(trimmed, "#!") {
			// shebang found
		}
		if strings.HasPrefix(trimmed, "set ") && (strings.Contains(trimmed, "-e") || strings.Contains(trimmed, "-o pipefail")) {
			hasSetE = true
		}
		if strings.Contains(trimmed, " $") && !strings.Contains(trimmed, "\"$") && !strings.Contains(trimmed, "'$") {
			unquotedVarCount++
		}

		// Double semicolons (all languages)
		if strings.Contains(trimmed, ";;") {
			doubleSemicolonCount++
		}
	}

	// ═══════════════════════════════════════════════════════════════
	// Build issues from collected metrics (no additional iteration)
	// ═══════════════════════════════════════════════════════════════

	if longLines > 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("%d 行超过120字符", longLines))
	}
	if todoCount > 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("发现 %d 个 TODO/FIXME/HACK 注释", todoCount))
	}
	if braceCount != 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("括号不平衡: { 比 } 多 %d 个（语法错误）", braceCount))
		report.Score -= 30
	}
	if maxBraceDepth > 5 {
		report.Issues = append(report.Issues, fmt.Sprintf("代码嵌套深度 %d 层，建议重构", maxBraceDepth))
		report.Complexity = maxBraceDepth
	}
	if magicNumbers > 5 {
		report.Issues = append(report.Issues, fmt.Sprintf("发现 %d 个可能的魔法数字，建议提取为常量", magicNumbers))
	}
	if doubleSemicolonCount > 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("双分号 ;; 在 %d 处", doubleSemicolonCount))
	}

	// Language-specific issue reporting
	ext := strings.ToLower(filePath[strings.LastIndex(filePath, "."):])
	switch {
	case ext == ".go":
		if !hasPackage && len(lines) > 0 {
			report.Issues = append(report.Issues, "缺少 package 声明（Go 文件必须以 package 开头）")
		}
		if importParens != 0 {
			report.Issues = append(report.Issues, "import 块括号不平衡")
		}
		if !hasFunc && len(lines) > 10 {
			report.Issues = append(report.Issues, "未发现 func 声明，可能缺少函数定义")
		}
	case ext == ".rs":
		if !hasFn && len(lines) > 10 {
			report.Issues = append(report.Issues, "未发现 fn 声明，可能缺少函数定义")
		}
	case ext == ".c", ext == ".cpp", ext == ".cc", ext == ".cxx":
		if !hasInclude && len(lines) > 5 {
			report.Issues = append(report.Issues, "未发现 #include 指令，可能缺少头文件引用")
		}
		if !hasMain && len(lines) > 10 {
			report.Issues = append(report.Issues, "未发现 main 函数，可能缺少程序入口点")
		}
	case ext == ".sh":
		if len(lines) > 0 && !strings.HasPrefix(strings.TrimSpace(lines[0]), "#!") {
			report.Issues = append(report.Issues, "缺少 shebang 行（第一行应为 #!/system/bin/sh 或 #!/bin/bash）")
		}
		if !hasSetE {
			report.Issues = append(report.Issues, "建议添加 set -euo pipefail 以增强错误处理")
		}
		if unquotedVarCount > 3 {
			report.Issues = append(report.Issues, fmt.Sprintf("发现 %d 处可能未加引号的变量，建议使用 \"$VAR\" 格式", unquotedVarCount))
		}
	}

	// ═══════════════════════════════════════════════════════════════
	// Calculate final score (no iteration over issues — use counters)
	// ═══════════════════════════════════════════════════════════════

	report.Score = 100
	// Deduct based on severity (using counters, not string matching)
	if braceCount != 0 {
		report.Score -= 30
	}
	if maxBraceDepth > 5 {
		report.Score -= 20
	}
	if longLines > 0 {
		report.Score -= 5
	}
	if magicNumbers > 5 {
		report.Score -= 10
	}
	if todoCount > 0 {
		report.Score -= 8
	}
	// Language-specific deductions
	if ext == ".go" && importParens != 0 {
		report.Score -= 15
	}
	if doubleSemicolonCount > 0 {
		report.Score -= 5
	}
	if report.Score < 0 {
		report.Score = 0
	}

	return report
}

// checkGoSyntax performs Go-specific syntax validation.
func (qv *QualityVerifier) checkGoSyntax(report *QualityReport, lines []string) {
	hasPackage := false
	importParens := 0
	hasFunc := false
	inImport := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check package declaration
		if strings.HasPrefix(trimmed, "package ") {
			hasPackage = true
		}

		// Check import block balance
		if trimmed == "import (" {
			inImport = true
		}
		if inImport {
			for _, ch := range trimmed {
				if ch == '(' {
					importParens++
				}
				if ch == ')' {
					importParens--
					if importParens <= 0 {
						inImport = false
					}
				}
			}
		}

		// Check for func declarations
		if strings.HasPrefix(trimmed, "func ") {
			hasFunc = true
		}

		// Check for common Go syntax errors
		if strings.Contains(trimmed, ";;") {
			report.Issues = append(report.Issues, fmt.Sprintf("双分号 ;; 在第 %d 行（Go 不需要分号）", i+1))
		}
		if strings.HasPrefix(trimmed, "var ") && strings.Contains(trimmed, "=") && !strings.Contains(trimmed, ":=") && strings.HasSuffix(trimmed, ";") {
			report.Issues = append(report.Issues, fmt.Sprintf("第 %d 行: Go 声明不需要分号结尾", i+1))
		}
	}

	if !hasPackage && len(lines) > 0 {
		report.Issues = append(report.Issues, "缺少 package 声明（Go 文件必须以 package 开头）")
	}
	if importParens != 0 {
		report.Issues = append(report.Issues, "import 块括号不平衡")
	}
	if !hasFunc && len(lines) > 10 {
		report.Issues = append(report.Issues, "未发现 func 声明，可能缺少函数定义")
	}
}

// checkRustSyntax performs Rust-specific syntax validation.
func (qv *QualityVerifier) checkRustSyntax(report *QualityReport, lines []string) {
	hasFn := false
	inComment := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Handle block comments
		if strings.Contains(trimmed, "/*") {
			inComment = true
		}
		if strings.Contains(trimmed, "*/") {
			inComment = false
			continue
		}
		if inComment || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Check for fn declarations
		if strings.HasPrefix(trimmed, "fn ") || strings.Contains(trimmed, " fn ") {
			hasFn = true
		}

		// Check for common Rust syntax errors
		if strings.Contains(trimmed, ";;") && !strings.HasPrefix(trimmed, "//") {
			report.Issues = append(report.Issues, fmt.Sprintf("双分号 ;; 在第 %d 行（Rust 语句不需要分号结尾的分号）", i+1))
		}

		// Check for missing semicolons after let/if expressions (common LLM error)
		if strings.HasPrefix(trimmed, "let ") && strings.Contains(trimmed, "=") && !strings.HasSuffix(trimmed, ";") && !strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, ",") {
			// Only flag if it looks like a simple assignment (not a block)
			if !strings.Contains(trimmed, "fn ") && !strings.Contains(trimmed, "if ") {
				report.Issues = append(report.Issues, fmt.Sprintf("第 %d 行: let 语句可能缺少分号", i+1))
			}
		}
	}

	if !hasFn && len(lines) > 10 {
		report.Issues = append(report.Issues, "未发现 fn 声明，可能缺少函数定义")
	}
}

// checkCppSyntax performs C/C++-specific syntax validation.
func (qv *QualityVerifier) checkCppSyntax(report *QualityReport, lines []string) {
	hasInclude := false
	hasMain := false
	inComment := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Handle block comments
		if strings.Contains(trimmed, "/*") {
			inComment = true
		}
		if strings.Contains(trimmed, "*/") {
			inComment = false
			continue
		}
		if inComment || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Check for #include
		if strings.HasPrefix(trimmed, "#include") {
			hasInclude = true
		}

		// Check for main function
		if strings.Contains(trimmed, "main(") {
			hasMain = true
		}

		// Check for common C/C++ syntax errors
		if strings.Contains(trimmed, ";;") && !strings.HasPrefix(trimmed, "#") {
			report.Issues = append(report.Issues, fmt.Sprintf("双分号 ;; 在第 %d 行", i+1))
		}

		// Check for missing semicolons after struct/class/enum definitions
		if (strings.HasPrefix(trimmed, "struct ") || strings.HasPrefix(trimmed, "class ") || strings.HasPrefix(trimmed, "enum ")) && strings.HasSuffix(trimmed, "}") {
			report.Issues = append(report.Issues, fmt.Sprintf("第 %d 行: 结构体/类定义后可能缺少分号", i+1))
		}
	}

	if !hasInclude && len(lines) > 5 {
		report.Issues = append(report.Issues, "未发现 #include 指令，可能缺少头文件引用")
	}
	if !hasMain && len(lines) > 10 {
		report.Issues = append(report.Issues, "未发现 main 函数，可能缺少程序入口点")
	}
}

// checkShellSyntax performs shell script syntax validation.
func (qv *QualityVerifier) checkShellSyntax(report *QualityReport, lines []string) {
	if len(lines) == 0 {
		return
	}

	// Check shebang
	firstLine := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(firstLine, "#!/") {
		report.Issues = append(report.Issues, "缺少 shebang 行（第一行应为 #!/system/bin/sh 或 #!/bin/bash）")
	}

	// Check for set -e / set -euo pipefail
	hasSetE := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "set ") && (strings.Contains(trimmed, "-e") || strings.Contains(trimmed, "-o pipefail")) {
			hasSetE = true
			break
		}
	}
	if !hasSetE {
		report.Issues = append(report.Issues, "建议添加 set -euo pipefail 以增强错误处理")
	}

	// Check for unquoted variables (common LLM error in shell scripts)
	unquotedCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "echo") {
			continue
		}
		// Simple heuristic: $VAR without quotes
		if strings.Contains(trimmed, " $") && !strings.Contains(trimmed, "\"$") && !strings.Contains(trimmed, "'$") {
			unquotedCount++
		}
	}
	if unquotedCount > 3 {
		report.Issues = append(report.Issues, fmt.Sprintf("发现 %d 处可能未加引号的变量，建议使用 \"$VAR\" 格式", unquotedCount))
	}
}

// GetQualitySummary returns a summary of quality reports with syntax-aware insights.
func (qv *QualityVerifier) GetQualitySummary(reports []QualityReport) string {
	if len(reports) == 0 {
		return "无文件需要检查"
	}

	totalScore := 0
	totalIssues := 0
	syntaxIssues := 0
	for _, r := range reports {
		totalScore += r.Score
		totalIssues += len(r.Issues)
		for _, issue := range r.Issues {
			if strings.Contains(issue, "括号不平衡") || strings.Contains(issue, "语法错误") || strings.Contains(issue, "缺少") {
				syntaxIssues++
			}
		}
	}
	avgScore := totalScore / len(reports)

	summary := "📊 代码质量报告:\n"
	summary += fmt.Sprintf("- 检查文件: %d\n", len(reports))
	summary += fmt.Sprintf("- 平均质量分: %d/100\n", avgScore)
	summary += fmt.Sprintf("- 发现问题: %d (其中语法问题: %d)\n", totalIssues, syntaxIssues)

	if syntaxIssues > 0 {
		summary += "- ⚠️ 发现潜在语法错误，建议先用 syntax_checker 工具验证再构建\n"
	}

	if avgScore >= 80 {
		summary += "- 评价: ✅ 良好"
	} else if avgScore >= 60 {
		summary += "- 评价: ⚠️ 一般，建议优化"
	} else {
		summary += "- 评价: ❌ 较差，需要重构"
	}

	return summary
}

func (r *AgentRunner) SetMemoryV2Store(store *service.MemoryV2Store) {
	r.memV2Store = store
}

// SetFileHashCache sets the file hash cache for UNCHANGED detection.
func (r *AgentRunner) SetFileHashCache(cache *fileHashCache) {
	r.fileHashCache = cache
}

// ═══════════════════════════════════════════════════════════════════
// P1-2: FileLock — Prevent race conditions on concurrent file access
// ═══════════════════════════════════════════════════════════════════

// FileLock provides per-file locking to prevent race conditions.
type FileLock struct {
	locks sync.Map // path -> *sync.Mutex
}

// fileLock is a per-file mutex wrapper.
type fileLock struct {
	mu sync.Mutex
}

// Lock acquires the lock for a specific file path.
func (fl *FileLock) Lock(path string) {
	val, _ := fl.locks.LoadOrStore(path, &fileLock{})
	l := val.(*fileLock)
	l.mu.Lock()
}

// Unlock releases the lock for a specific file path.
func (fl *FileLock) Unlock(path string) {
	if val, ok := fl.locks.Load(path); ok {
		l := val.(*fileLock)
		l.mu.Unlock()
	}
}

// ═══════════════════════════════════════════════════════════════════
// P1-3: CallBudget — Global tool call budget per session
// ═══════════════════════════════════════════════════════════════════

// CallBudget tracks tool call usage against limits.
type CallBudget struct {
	TotalCalls     int
	ReadCalls      int
	WriteCalls     int
	MaxTotal       int
	MaxRead        int
	MaxWrite       int
	BudgetExceeded bool
}

// NewCallBudget creates a new call budget with default limits.
func NewCallBudget() *CallBudget {
	return &CallBudget{
		MaxTotal: 200, // total tool calls per session
		MaxRead:  100, // read_file calls per session
		MaxWrite: 50,  // write_file calls per session
	}
}

// CanCall checks if a tool call is within budget.
func (cb *CallBudget) CanCall(toolName string) bool {
	if cb.BudgetExceeded {
		return false
	}

	cb.TotalCalls++
	if cb.TotalCalls > cb.MaxTotal {
		cb.BudgetExceeded = true
		return false
	}

	switch toolName {
	case "read_file", "list_dir":
		cb.ReadCalls++
		if cb.ReadCalls > cb.MaxRead {
			return false
		}
	case "write_file", "write_file_batch", "delete_file":
		cb.WriteCalls++
		if cb.WriteCalls > cb.MaxWrite {
			return false
		}
	}

	return true
}

// ═══════════════════════════════════════════════════════════════════
// P2-1: SelfReflection — Agent self-reflection logging
// ═══════════════════════════════════════════════════════════════════

// ReflectionEvent records an agent's self-reflection about its actions.
type ReflectionEvent struct {
	Timestamp time.Time
	ToolName  string
	Action    string // "success", "failure", "retry", "skip"
	Reason    string
	Iteration int
}

// ReflectionLog tracks agent reflections for debugging and improvement.
type ReflectionLog struct {
	events []ReflectionEvent
	mu     sync.Mutex
}

// NewReflectionLog creates a new reflection log.
func NewReflectionLog() *ReflectionLog {
	return &ReflectionLog{
		events: make([]ReflectionEvent, 0, 100),
	}
}

// Record adds a reflection event.
func (rl *ReflectionLog) Record(toolName, action, reason string, iteration int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.events = append(rl.events, ReflectionEvent{
		Timestamp: time.Now(),
		ToolName:  toolName,
		Action:    action,
		Reason:    reason,
		Iteration: iteration,
	})
}

// GetSummary returns a summary of reflections.
func (rl *ReflectionLog) GetSummary() string {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if len(rl.events) == 0 {
		return "无反思记录"
	}

	successCount := 0
	failureCount := 0
	retryCount := 0
	for _, e := range rl.events {
		switch e.Action {
		case "success":
			successCount++
		case "failure":
			failureCount++
		case "retry":
			retryCount++
		}
	}

	return fmt.Sprintf("反思统计: 成功=%d, 失败=%d, 重试=%d, 总计=%d",
		successCount, failureCount, retryCount, len(rl.events))
}

// ═══════════════════════════════════════════════════════════════════
// P0-1: StagnationDetector — Smart loop termination
// ═══════════════════════════════════════════════════════════════════

// StagnationDetector tracks agent progress and detects when it's stuck in a loop.
// It monitors: repeated tool calls, lack of write operations, and identical results.
// StagnationDetector tracks agent progress and detects when it's stuck in a loop.
// O(1) operations via hash-based counters instead of linear scans.
type StagnationDetector struct {
	// Sliding window of last N tool call signatures (ring buffer)
	lastToolCalls         []string
	lastToolCallsIdx      int      // ring buffer write position
	lastToolCallsCount    int      // number of entries in ring buffer
	// Sliding window of last N tool results (ring buffer)
	lastResults           []string
	lastResultsIdx        int      // ring buffer write position
	lastResultsCount      int      // number of entries in ring buffer

	// O(1) counters for stagnation detection
	signatureCounts       map[string]int // tool signature -> count in current window
	resultStreak          int            // consecutive identical results
	lastResultSig         string         // last result signature for streak detection

	consecutiveNoWrite    int      // iterations without write_file call
	maxConsecutiveNoWrite int      // threshold to force answer
	maxIdenticalRepeats   int      // max times same tool+args can repeat
	maxStagnationRounds   int      // rounds with no meaningful progress
	stagnationCount       int      // current stagnation counter

	windowSize            int      // size of sliding window (max of lastToolCalls/lastResults capacity)
}

// toolCallSignature creates a compact signature for a tool call (tool name + args hash).
// O(k) where k = args size (bounded by 100 char truncation).
func toolCallSignature(name string, args map[string]interface{}) string {
	// Fast path: skip JSON marshaling for nil/empty args
	if len(args) == 0 {
		return name
	}
	// Simple hash: just use first 100 chars of JSON args
	argStr, _ := json.Marshal(args)
	if len(argStr) > 100 {
		argStr = argStr[:100]
	}
	return name + ":" + string(argStr)
}

// resultSignature creates a compact signature for a tool result.
// O(1) — just truncation.
func resultSignature(result string) string {
	if len(result) > 200 {
		return result[:200]
	}
	return result
}

// newStagnationDetector creates a new detector with O(1) operations.
func newStagnationDetector() *StagnationDetector {
	const windowSize = 15
	return &StagnationDetector{
		lastToolCalls:         make([]string, windowSize),
		lastResults:           make([]string, windowSize),
		signatureCounts:       make(map[string]int, windowSize),
		maxConsecutiveNoWrite: 30,
		maxIdenticalRepeats:   15,
		maxStagnationRounds:   25,
		windowSize:            windowSize,
	}
}

// addSignature adds a signature to the ring buffer and updates the O(1) counter.
func (sd *StagnationDetector) addSignature(sig string) {
	// If window is full, remove the oldest entry from counter
	if sd.lastToolCallsCount == sd.windowSize {
		oldSig := sd.lastToolCalls[sd.lastToolCallsIdx]
		sd.signatureCounts[oldSig]--
		if sd.signatureCounts[oldSig] <= 0 {
			delete(sd.signatureCounts, oldSig)
		}
	} else {
		sd.lastToolCallsCount++
	}

	// Add new signature
	sd.lastToolCalls[sd.lastToolCallsIdx] = sig
	sd.signatureCounts[sig]++
	sd.lastToolCallsIdx = (sd.lastToolCallsIdx + 1) % sd.windowSize
}

// RecordToolCall records a tool call and returns true if stagnation detected.
// O(1) via hash-based counter instead of linear scan.
func (sd *StagnationDetector) RecordToolCall(name string, args map[string]interface{}, result string) (stagnant bool, reason string) {
	sig := toolCallSignature(name, args)

	// O(1): Add to ring buffer and update counter
	sd.addSignature(sig)

	// O(1): Check if same tool+args repeated maxIdenticalRepeats times
	count := sd.signatureCounts[sig]
	if count >= sd.maxIdenticalRepeats {
		return true, fmt.Sprintf("工具 '%s' 已重复调用 %d 次（相同参数），建议换一种方式或直接给出答案", name, count)
	}

	// O(1): Track result streak for stagnation detection
	resSig := resultSignature(result)
	if resSig == sd.lastResultSig && resSig != "" {
		sd.resultStreak++
	} else {
		sd.resultStreak = 1
		sd.lastResultSig = resSig
	}

	// Check if result streak exceeds stagnation threshold
	if sd.resultStreak >= sd.maxStagnationRounds {
		sd.stagnationCount++
		if sd.stagnationCount >= 1 {
			return true, "连续多轮返回相同结果，Agent 可能陷入循环，建议直接给出当前进度的答案"
		}
	} else {
		sd.stagnationCount = 0
	}

	return false, ""
}

// RecordNoWrite tracks iterations without write_file. O(1).
func (sd *StagnationDetector) RecordNoWrite() bool {
	sd.consecutiveNoWrite++
	if sd.consecutiveNoWrite >= sd.maxConsecutiveNoWrite {
		return true // force answer
	}
	return false
}

// ResetNoWrite resets the no-write counter (called when write_file is executed). O(1).
func (sd *StagnationDetector) ResetNoWrite() {
	sd.consecutiveNoWrite = 0
}

// ═══════════════════════════════════════════════════════════════════
// P0-2: ToolRetryFallback — Fallback strategies for failed tools
// ═══════════════════════════════════════════════════════════════════

// ToolRetryFallback provides fallback strategies when tool calls fail.
type ToolRetryFallback struct {
	db           *sql.DB
	currentModel string
}

// FallbackStrategy represents a fallback action.
type FallbackStrategy int

const (
	FallbackRetrySame FallbackStrategy = iota
	FallbackSimplifyTask
	FallbackSwitchModel
	FallbackForceAnswer
)

// GetFallback determines the best fallback strategy for a failed tool call.
func (trf *ToolRetryFallback) GetFallback(toolName string, err error, consecutiveFailures int) FallbackStrategy {
	errStr := err.Error()

	// If tool not found, try alternative
	if strings.Contains(errStr, "not found") || strings.Contains(errStr, "unknown skill") {
		return FallbackSimplifyTask
	}

	// If timeout, try with simpler input
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		if consecutiveFailures >= 2 {
			return FallbackSimplifyTask
		}
		return FallbackRetrySame
	}

	// If rate limited, switch model
	if strings.Contains(errStr, "rate") || strings.Contains(errStr, "429") {
		return FallbackSwitchModel
	}

	// If context too long, force answer
	if strings.Contains(errStr, "context_length") || strings.Contains(errStr, "max_tokens") {
		return FallbackForceAnswer
	}

	// Default: after 3 failures, simplify
	if consecutiveFailures >= 3 {
		return FallbackSimplifyTask
	}

	return FallbackRetrySame
}

// SimplifyTaskInput creates a simplified version of the tool input.
func (trf *ToolRetryFallback) SimplifyTaskInput(toolName string, input map[string]interface{}) map[string]interface{} {
	simplified := make(map[string]interface{})
	for k, v := range input {
		simplified[k] = v
	}

	switch toolName {
	case "write_file":
		// If content is too long, truncate and add marker
		if content, ok := simplified["content"].(string); ok && len(content) > 5000 {
			simplified["content"] = content[:5000] + "\n// ... (truncated due to size limit)"
		}
	case "bash":
		// If command is complex, simplify
		if cmd, ok := simplified["command"].(string); ok {
			if strings.Contains(cmd, "&&") {
				// Take only first command
				parts := strings.SplitN(cmd, "&&", 2)
				simplified["command"] = strings.TrimSpace(parts[0])
			}
		}
	case "build_module":
		// No simplification needed
	}

	return simplified
}

// ═══════════════════════════════════════════════════════════════════
// P0-2: ErrorClassifier — Classify errors and determine recovery strategy
// ═══════════════════════════════════════════════════════════════════

// ErrorCategory represents the type of error encountered.
type ErrorCategory int

const (
	ErrorUnknown      ErrorCategory = iota
	ErrorNetwork                    // Network timeout, connection refused
	ErrorAuth                       // Authentication failed, permission denied
	ErrorRateLimit                  // Rate limit exceeded (429)
	ErrorContext                    // Context too long
	ErrorToolNotFound               // Tool/skill not found
	ErrorPermission                 // File permission denied
	ErrorDiskSpace                  // Disk full
	ErrorSyntax                     // Code syntax error
	ErrorBuild                      // Build/compile error
)

// ClassifyError determines the error category from an error message.
func ClassifyError(errMsg string) ErrorCategory {
	msg := strings.ToLower(errMsg)

	// Network errors
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "connection refused") || strings.Contains(msg, "dial tcp") {
		return ErrorNetwork
	}

	// Auth errors
	if strings.Contains(msg, "unauthorized") || strings.Contains(msg, "401") ||
		strings.Contains(msg, "authentication") || strings.Contains(msg, "api key") {
		return ErrorAuth
	}

	// Rate limit
	if strings.Contains(msg, "rate") || strings.Contains(msg, "429") ||
		strings.Contains(msg, "too many requests") {
		return ErrorRateLimit
	}

	// Context too long
	if strings.Contains(msg, "context_length") || strings.Contains(msg, "maximum context") ||
		strings.Contains(msg, "max_tokens") || strings.Contains(msg, "token limit") {
		return ErrorContext
	}

	// Tool not found
	if strings.Contains(msg, "not found") || strings.Contains(msg, "unknown skill") ||
		strings.Contains(msg, "no such skill") {
		return ErrorToolNotFound
	}

	// Permission
	if strings.Contains(msg, "permission denied") || strings.Contains(msg, "access denied") ||
		strings.Contains(msg, "eacces") {
		return ErrorPermission
	}

	// Disk space
	if strings.Contains(msg, "no space") || strings.Contains(msg, "disk full") ||
		strings.Contains(msg, "enospc") {
		return ErrorDiskSpace
	}

	// Syntax errors
	if strings.Contains(msg, "syntax error") || strings.Contains(msg, "unexpected token") ||
		strings.Contains(msg, "parse error") {
		return ErrorSyntax
	}

	// Build errors
	if strings.Contains(msg, "build failed") || strings.Contains(msg, "compile error") ||
		strings.Contains(msg, "cannot find package") || strings.Contains(msg, "undefined:") {
		return ErrorBuild
	}

	return ErrorUnknown
}

// RecoveryStrategy represents the recommended recovery action.
type RecoveryStrategy int

const (
	RecoveryRetrySame      RecoveryStrategy = iota // Retry with same parameters
	RecoverySimplifyInput                          // Simplify task input
	RecoverySwitchModel                            // Switch to different model
	RecoveryForceAnswer                            // Force agent to provide answer
	RecoverySkipTool                               // Skip this tool and continue
	RecoveryCompactContext                         // Compact context and retry
	RecoveryAbort                                  // Abort immediately
)

// GetRecoveryStrategy determines the best recovery strategy for an error.
func GetRecoveryStrategy(category ErrorCategory, consecutiveFailures int) RecoveryStrategy {
	switch category {
	case ErrorNetwork:
		// Network: retry with backoff, after 3 failures abort
		if consecutiveFailures >= 3 {
			return RecoveryAbort
		}
		return RecoveryRetrySame

	case ErrorAuth:
		// Auth: try different provider, after 2 attempts abort
		if consecutiveFailures >= 2 {
			return RecoveryAbort
		}
		return RecoverySwitchModel

	case ErrorRateLimit:
		// Rate limit: switch model immediately
		return RecoverySwitchModel

	case ErrorContext:
		// Context too long: compact first, then force answer
		if consecutiveFailures >= 2 {
			return RecoveryForceAnswer
		}
		return RecoveryCompactContext

	case ErrorToolNotFound:
		// Tool not found: skip and continue
		return RecoverySkipTool

	case ErrorPermission:
		// Permission: skip and inform user
		return RecoverySkipTool

	case ErrorDiskSpace:
		// Disk full: abort immediately
		return RecoveryAbort

	case ErrorSyntax:
		// Syntax error: simplify input (maybe truncation caused it)
		if consecutiveFailures >= 2 {
			return RecoveryForceAnswer
		}
		return RecoverySimplifyInput

	case ErrorBuild:
		// Build error: let agent fix it
		return RecoverySkipTool

	default:
		// Unknown: retry once, then force answer
		if consecutiveFailures >= 2 {
			return RecoveryForceAnswer
		}
		return RecoveryRetrySame
	}
}

// GetRecoveryMessage returns a user-friendly message for the recovery strategy.
func GetRecoveryMessage(strategy RecoveryStrategy, toolName string) string {
	switch strategy {
	case RecoveryRetrySame:
		return fmt.Sprintf("工具 '%s' 执行失败，正在重试...", toolName)
	case RecoverySimplifyInput:
		return fmt.Sprintf("工具 '%s' 输入过复杂，正在简化...", toolName)
	case RecoverySwitchModel:
		return "当前模型限流，正在切换备用模型..."
	case RecoveryForceAnswer:
		return "多次重试失败，请基于已有信息给出答案"
	case RecoverySkipTool:
		return fmt.Sprintf("跳过工具 '%s'，继续执行...", toolName)
	case RecoveryCompactContext:
		return "上下文过长，正在压缩..."
	case RecoveryAbort:
		return "多次失败，终止执行"
	default:
		return fmt.Sprintf("工具 '%s' 执行异常", toolName)
	}
}

// getSessionCache returns (or creates) a session-scoped tool result cache.
// This cache persists across multiple Run() calls in the same session,
// avoiding redundant I/O when the LLM re-reads the same file in later rounds.
func (r *AgentRunner) getSessionCache(sessionID string) *toolResultCache {
	if sessionID == "" {
		return newToolResultCache()
	}
	// Track access time for TTL-based cleanup
	r.sessionAccessTimes.Store(sessionID, time.Now())
	if cached, ok := r.sessionCaches.Load(sessionID); ok {
		return cached.(*toolResultCache)
	}
	cache := newToolResultCache()
	r.sessionCaches.Store(sessionID, cache)
	return cache
}

// Optimization 30: Write-through cache with TTL (5 minutes)
// Prevents stale data when files are modified externally or by other processes.
// P0-1: Now tracks file mtime for external modification detection.
type cachedContentWithMtime struct {
	content   string
	expiresAt time.Time
	mtime     time.Time // file modification time when cached
}

const writeContentCacheTTL = 5 * time.Minute

// cacheWriteContent stores the content of a successful write_file call.
// When read_file is called for the same path immediately after, it returns
// this cached content instead of re-reading from disk — saving one full I/O round.
// P0-1: Now tracks file modification time for invalidation.
func (r *AgentRunner) cacheWriteContent(sessionID, path, content string) {
	if sessionID == "" {
		return
	}
	val, _ := r.writeContentCache.LoadOrStore(sessionID, &sync.Map{})
	m := val.(*sync.Map)
	// Get current file mtime for invalidation
	var mtime time.Time
	if info, err := os.Stat(path); err == nil {
		mtime = info.ModTime()
	} else {
		mtime = time.Now()
	}
	m.Store(path, cachedContentWithMtime{
		content:   content,
		expiresAt: time.Now().Add(writeContentCacheTTL),
		mtime:     mtime,
	})
	debugLog("writeContentCache PUT: session=%s path=%s len=%d mtime=%v", sessionID, path, len(content), mtime)
}

// getCachedWriteContent returns the cached content for a path, or "" if not cached or expired.
// P0-1: Also checks if file was modified externally (mtime mismatch).
func (r *AgentRunner) getCachedWriteContent(sessionID, path string) string {
	if sessionID == "" {
		return ""
	}
	// Track access time for TTL-based cleanup
	r.sessionAccessTimes.Store(sessionID, time.Now())
	val, ok := r.writeContentCache.Load(sessionID)
	if !ok {
		return ""
	}
	m := val.(*sync.Map)
	if cached, ok := m.Load(path); ok {
		cc := cached.(cachedContentWithMtime)
		if time.Now().After(cc.expiresAt) {
			// Entry expired — remove it and return empty
			m.Delete(path)
			debugLog("writeContentCache EXPIRED: session=%s path=%s", sessionID, path)
			return ""
		}
		// P0-1: Check if file was modified externally
		if info, err := os.Stat(path); err == nil {
			if info.ModTime().After(cc.mtime) {
				// File was modified after we cached it — invalidate cache
				m.Delete(path)
				debugLog("writeContentCache INVALIDATED (mtime changed): session=%s path=%s cacheMtime=%v fileMtime=%v",
					sessionID, path, cc.mtime, info.ModTime())
				return ""
			}
		}
		debugLog("writeContentCache HIT: session=%s path=%s", sessionID, path)
		return cc.content
	}
	return ""
}

// cacheReadFile stores the content of a successful read_file call for reuse.
func (r *AgentRunner) cacheReadFile(sessionID, path, content string) {
	if sessionID == "" {
		return
	}
	val, _ := r.readFileCache.LoadOrStore(sessionID, &sync.Map{})
	m := val.(*sync.Map)
	m.Store(path, content)
}

// getCachedReadFile returns cached content for a path, or "" if not cached.
func (r *AgentRunner) getCachedReadFile(sessionID, path string) string {
	if sessionID == "" {
		return ""
	}
	val, ok := r.readFileCache.Load(sessionID)
	if !ok {
		return ""
	}
	m := val.(*sync.Map)
	if content, ok := m.Load(path); ok {
		return content.(string)
	}
	return ""
}

// startSessionCacheCleanup runs a background goroutine that periodically evicts
// expired session caches to prevent memory leaks. Sessions that haven't been
// accessed in 30 minutes are removed from sessionCaches, writeContentCache,
// and sessionAccessTimes.
func (r *AgentRunner) startSessionCacheCleanup() {
	const (
		cleanupInterval = 5 * time.Minute
		sessionTTL      = 30 * time.Minute
	)
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		expired := make([]string, 0)
		r.sessionAccessTimes.Range(func(key, value interface{}) bool {
			lastAccess := value.(time.Time)
			if now.Sub(lastAccess) > sessionTTL {
				expired = append(expired, key.(string))
			}
			return true
		})
		for _, sid := range expired {
			r.sessionCaches.Delete(sid)
			r.writeContentCache.Delete(sid)
			r.readFileCache.Delete(sid)
			r.sessionAccessTimes.Delete(sid)
			debugLog("session cache TTL expired: session=%s", sid)
		}
		if len(expired) > 0 {
			log.Printf("[Agent] session cache cleanup: evicted %d expired sessions", len(expired))
		}
	}
}

// findFallbackProvider queries the DB for another free-tier provider that isn't
// currently circuit-broken. Returns empty strings if no fallback is available.
func (r *AgentRunner) findFallbackProvider(userID, excludeProviderID, currentModel string) (endpoint, apiKey, model, providerID string) {
	if r.db == nil {
		return
	}
	rows, err := r.db.Query(
		`SELECT id, endpoint, api_key, model_id FROM llm_providers
		 WHERE user_id=? AND id != ? AND model_id != ''
		 ORDER BY created_at DESC LIMIT 10`,
		userID, excludeProviderID,
	)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id, ep, key, mdl string
		if err := rows.Scan(&id, &ep, &key, &mdl); err != nil {
			continue
		}
		// Only consider free-tier models as fallbacks
		tier := resolveModelTier(mdl)
		if tier != TierFree {
			continue
		}
		// Skip providers that are also circuit-broken
		if globalCircuitBreaker.IsOpen(id) {
			continue
		}
		return ep, key, mdl, id
	}
	return
}

// ═══════════════════════════════════════════════════════════════════
// Core Agent Loop — modern architecture
//
// Features:
//   - Native OpenAI function calling
//   - Plan/Act mode separation (Cline)
//   - Smart context compaction (OpenCode/Claude Code)
//   - File checkpoint for undo (Cline)
//   - Auto error recovery
//   - Smart loop detection
// ═══════════════════════════════════════════════════════════════════

func (r *AgentRunner) Run(ctx context.Context, task string, userID string, messages []service.Message, sessionID string, w SSEWriter, reqProviderID, reqModel string, cfg RunConfig) error {
	ctx, cancel := context.WithTimeout(ctx, totalTimeout)
	defer cancel()

	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = defaultMaxIterations
	}
	if cfg.MaxResultLen <= 0 {
		cfg.MaxResultLen = defaultMaxResultLen
	}
	if cfg.Mode == "" {
		cfg.Mode = ModeAct
	}

	// P0-Optimization: Resolve LLM config ONCE at Run() entry and cache in RunConfig.
	// This avoids repeated DB queries in callLLMWithTools on every iteration.
	resolvedEndpoint, resolvedAPIKey, resolvedModel := r.resolveLLMConfig(userID, reqProviderID, reqModel, cfg)
	cfg.resolvedEndpoint = resolvedEndpoint
	cfg.resolvedAPIKey = resolvedAPIKey
	cfg.resolvedModel = resolvedModel
	cfg.modelTier = resolveModelTier(resolvedModel)
	modelTier := cfg.modelTier
	compactionThreshold := compactionThresholdForTier(modelTier)
	// Override MaxResultLen with tier-based default if user hasn't set a custom value
	if cfg.MaxResultLen == defaultMaxResultLen {
		cfg.MaxResultLen = maxResultLenForTier(modelTier)
	}
	debugLog("model=%s tier=%d compactionThreshold=%d maxResultLen=%d", resolvedModel, modelTier, compactionThreshold, cfg.MaxResultLen)

	if sessionID != "" && len(messages) > 0 {
		r.convStore.Add(sessionID, messages)
	}

	// Build system prompt based on mode — free models use ultra-short prompt
	var systemPrompt string
	if modelTier == TierFree {
		systemPrompt = buildFreeModelPrompt(cfg.Mode)
		debugLog("using free model prompt (saved ~600 tokens)")
	} else {
		systemPrompt = r.buildSystemPromptForMode(cfg.Mode)
	}

	if r.memoryStore != nil && cfg.UserID != "" {
		if prefs := r.memoryStore.LoadUserPreferences(cfg.UserID); prefs != "" {
			systemPrompt += "\n" + prefs
		}
	}

	// Inject project context
	if cfg.ProjectID != "" {
		systemPrompt += r.buildProjectContext(cfg)
	} else {
		// No project selected — guide the agent to handle this gracefully
		systemPrompt += `
## NO PROJECT SELECTED
You are running WITHOUT a project context. This means:
- You CANNOT read or write project files (no project_id available)
- You CAN answer questions, provide advice, write code snippets, and explain concepts
- If the user asks you to create/modify files, explain that they need to select a project first, OR offer to create a new project by writing files directly (write_file auto-creates projects)
- For general coding questions, answer directly without tools
- Keep your answers focused and practical`
	}

	// Build tool definitions — filtered by mode and model tier.
	// Use resolvedModel (not cfg.LLMModel) so tier filtering matches the prompt
	// branch above (free tier gets fewer tools, same as it gets a shorter prompt).
	toolDefs := r.getToolDefinitions(cfg.Mode, resolvedModel)

	// Build initial conversation
	conversation := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": task},
	}

	// Inject history with smart compaction
	if sessionID != "" {
		history := r.convStore.Get(sessionID)
		if len(history) > 0 {
			history = r.smartCompressHistory(ctx, history, w, cfg)
			conversation = appendRoleMessage(conversation, "system", "[Previous conversation]")
			for _, msg := range history {
				convMsg := map[string]interface{}{
					"role":    msg.Role,
					"content": msg.Content,
				}
				// Include tool_calls if present (for assistant messages)
				if len(msg.ToolCalls) > 0 {
					convMsg["tool_calls"] = msg.ToolCalls
				}
				// Include tool_call_id if present (for tool messages)
				if msg.ToolCallID != "" {
					convMsg["tool_call_id"] = msg.ToolCallID
				}
				conversation = append(conversation, convMsg)
			}
		}
	}

	// Tracking state (with O(1) pre-computed counters for loop detection)
	m := &runMetrics{
		toolCallHistory:          make(map[string]int),
		uniqueOps:                make(map[string]bool),
		toolConsecutiveErrors:    make(map[string]int),
		toolLastResults:          make(map[string]string),
		toolConsecutiveIdentical: make(map[string]int),
		uniqueTargetsPerSkill:    make(map[string]int),
	}
	writeFileCalled := false
	anyWriteCalled := false // P0-2: Track if any write tool was called
	// P1-2: Dynamic limits based on project complexity
	// P0-1: Reduced base limits to prevent excessive reads
	baseMaxReadFilePerTurn := 25
	baseMaxWriteFilePerTurn := 20
	maxWriteFilePerTurn := baseMaxWriteFilePerTurn
	maxReadFilePerTurn := baseMaxReadFilePerTurn
	checkpoints := make([]FileCheckpoint, 0) // file change history for undo
	consecutiveErrors := 0
	answerSent := false
	var lastLLMResp *LLMResponse
	startTime := time.Now() // NEW: Track total execution time

	// Optimization 51: Reset performance metrics for this run
	runPerfMetrics := NewPerformanceMetrics()

	// Optimization 1: Session-scoped tool result cache (persists across Run() calls)
	toolCache := r.getSessionCache(sessionID)

	// P0-1: Smart loop termination — detect stagnation (O(1) hash-based counters)
	stagnationDetector := newStagnationDetector()

	// Post-execution analysis — stagnation detection, self-reflection, loop detection
	trp := &toolResultProcessor{
		r: r, ctx: ctx, w: w, cfg: cfg, sessionID: sessionID,
		reqProviderID: reqProviderID, reqModel: reqModel,
		stagnationDetector: stagnationDetector, m: m,
	}

	// P0-2: Tool retry fallback
	toolRetryFallback := &ToolRetryFallback{
		db:           r.db,
		currentModel: resolvedModel,
	}

	// P2-1: Task decomposer for complex tasks
	taskDecomposer := &TaskDecomposer{db: r.db}
	var subtasks []Subtask
	// Try LLM-based decomposition first for complex tasks, fallback to keyword matching
	if isComplexTask(task) {
		llmSubtasks := r.decomposeWithLLM(ctx, task, cfg.ProjectContext, cfg)
		if len(llmSubtasks) > 0 {
			subtasks = llmSubtasks
		} else {
			subtasks = taskDecomposer.DecomposeTask(task, cfg.ProjectContext)
		}
	} else {
		subtasks = taskDecomposer.DecomposeTask(task, cfg.ProjectContext)
	}
	currentSubtask := taskDecomposer.GetNextSubtask(subtasks)
	if currentSubtask != nil {
		log.Printf("[Agent] task decomposed into %d subtasks, starting: %s", len(subtasks), currentSubtask.Description)
	}

	// SSE: Send task plan to frontend
	if len(subtasks) > 0 {
		w.WriteSSE(map[string]interface{}{
			"type":     "step",
			"step":     "task_plan",
			"content":  fmt.Sprintf("任务分解完成，共 %d 个子任务", len(subtasks)),
			"subtasks": subtasks,
		})
		// Mark first subtask as in_progress and notify
		if currentSubtask != nil {
			currentSubtask.Status = "in_progress"
			currentSubtask.StartedAt = time.Now().UnixMilli()
			w.WriteSSE(map[string]interface{}{
				"type":       "step",
				"step":       "task_progress",
				"subtask_id": currentSubtask.ID,
				"status":     "in_progress",
				"content":    currentSubtask.Description,
			})
		}
	}

	// P2-2: Quality verifier
	qualityVerifier := &QualityVerifier{db: r.db}
	qualityReports := make([]QualityReport, 0)
	// P2-Fix: Limit concurrent quality verification goroutines to prevent goroutine explosion.
	// Max 3 concurrent verifications; excess goroutines block on this channel.
	qualitySem := make(chan struct{}, 3)

	// P1-2: File lock for race condition prevention
	fileLock := &FileLock{}

	// P1-3: Global call budget
	callBudget := NewCallBudget()

	// P2-1: Self-reflection log
	reflectionLog := NewReflectionLog()

	// Initialize repo-map for global code structure indexing
	if cfg.ProjectID != "" {
		projectPath := ""
		if r.db != nil {
			var storagePath string
			err := r.db.QueryRow(`SELECT COALESCE(storage_path,'') FROM projects WHERE id=?`, cfg.ProjectID).Scan(&storagePath)
			if err == nil && storagePath != "" {
				projectPath = storagePath
			}
		}
		if projectPath != "" {
			r.repoMap = NewRepoMap(projectPath)
			// P3-Fix: Generate initial repo-map with timeout protection.
			// Large projects can take seconds to scan; this prevents blocking indefinitely.
			go func() {
				r.repoMap.GenerateRepoMapWithTimeout(ctx, projectPath, 10*time.Second)
				log.Printf("[Agent] repo-map generated: %d files indexed", len(r.repoMap.fileIndex))
			}()
		}
	}

	// Derive skill sets from metadata (no hardcoded maps)
	readOnlySkills := r.registry.ReadOnlySkills()

	// iterCancel/iterCtx are declared at function scope so iterCancel can be called
	// in the final return. iterCtx is reassigned each iteration with a per-iteration timeout.
	var iterCancel context.CancelFunc
	var iterCtx context.Context

	for iter := 0; iter < cfg.MaxIterations; iter++ {
		// Per-iteration timeout: create a child context with shorter deadline
		// P1-2: Dynamically adjust limits based on project complexity
		if cfg.ProjectID != "" && iter > 0 {
			// Increase limits for complex projects (more files to read/write)
			if m.totalToolCalls > 15 {
				maxReadFilePerTurn = baseMaxReadFilePerTurn + 10
				maxWriteFilePerTurn = baseMaxWriteFilePerTurn + 10
			}
			if m.totalToolCalls > 40 {
				maxReadFilePerTurn = baseMaxReadFilePerTurn + 20
				maxWriteFilePerTurn = baseMaxWriteFilePerTurn + 20
			}
			if m.totalToolCalls > 80 {
				maxReadFilePerTurn = baseMaxReadFilePerTurn + 30
				maxWriteFilePerTurn = baseMaxWriteFilePerTurn + 30
			}
		}
		select {
		case <-ctx.Done():
			if iterCancel != nil {
				iterCancel()
			}
			return ctx.Err()
		default:
		}

		if w.IsDisconnected() {
			log.Printf("[Agent] client disconnected at iteration %d", iter+1)
			r.writeContentCache.Delete(sessionID)
			r.readFileCache.Delete(sessionID)
			if iterCancel != nil {
				iterCancel()
			}
			return fmt.Errorf("client disconnected")
		}

		// Auto-compact if conversation is getting too long
		// Optimization 15: Cache size calculation to avoid double computation
		// Optimization: Skip compaction when conversation is small (<10 messages)
		// to avoid unnecessary LLM calls for compaction
		if len(conversation) >= 10 {
			convSize := r.estimateConversationSize(conversation)
			if convSize > compactionThreshold {
				debugLog("auto-compacting conversation (size=%d, messages=%d)", convSize, len(conversation))
				compacted, err := r.compactConversation(ctx, conversation, w, cfg)
				if err == nil {
					conversation = compacted
					w.WriteSSE(map[string]interface{}{
						"type":    "step",
						"step":    "think",
						"content": "📋 上下文已自动压缩，继续工作...",
					})
				}
			}
		}

		// Optimization: Only show progress event every 3 iterations to reduce SSE noise
		if iter%3 == 0 || iter == cfg.MaxIterations-1 {
			progressPct := float64(iter+1) / float64(cfg.MaxIterations) * 100
			w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "think",
				"content": fmt.Sprintf("思考中 (第 %d/%d 轮, %.0f%%)...", iter+1, cfg.MaxIterations, progressPct),
			})
		}

		// Per-iteration timeout: create a child context with shorter deadline
		// This prevents a single slow LLM call or tool execution from consuming
		// the entire 30-minute budget
		iterCtx, iterCancel = context.WithTimeout(ctx, perIterationTimeout)

		// Call LLM with keepalive — send empty think events every 10s to prevent frontend idle timeout
		llmDone := make(chan struct{})
		startKeepalive(iterCtx, w, llmDone, 10*time.Second)

		// Optimization 2: Prefilter conversation to remove waste
		prefiltered := prefilterConversation(conversation)

		// Optimization 51: Record LLM call duration
		llmStartTime := time.Now()
		llmResp, err := r.callLLMWithTools(iterCtx, prefiltered, toolDefs, w, cfg.UserID, reqProviderID, reqModel, cfg)
		llmDuration := time.Since(llmStartTime)
		runPerfMetrics.RecordLLMCall(llmDuration)
		close(llmDone)
		lastLLMResp = llmResp
		if err != nil {
			runPerfMetrics.RecordError()
			var abortErr error
			conversation, consecutiveErrors, abortErr = r.handleLLMCallError(ctx, w, cfg, conversation, consecutiveErrors, err)
			if abortErr != nil {
				iterCancel()
				return abortErr
			}
			continue
		}
		consecutiveErrors = 0

		debugLog("iter=%d mode=%s role=%s contentLen=%d toolCalls=%d llmDuration=%v",
			iter+1, cfg.Mode, llmResp.Role, len(llmResp.Content), len(llmResp.ToolCalls), llmDuration)

		// ── Case 1: Final answer ──
		if llmResp.Role == "assistant" && len(llmResp.ToolCalls) == 0 {
			answer := cleanAnswer(llmResp.Content)

			// P2-2: Append quality report if we have quality data
			if len(qualityReports) > 0 {
				qualitySummary := qualityVerifier.GetQualitySummary(qualityReports)
				answer += "\n\n" + qualitySummary
			}

			// P2-1: Append reflection summary
			reflectionSummary := reflectionLog.GetSummary()
			if reflectionSummary != "无反思记录" {
				answer += "\n\n📊 " + reflectionSummary
			}

			// P2-1: Mark subtask as completed if applicable
			if currentSubtask != nil {
				currentSubtask.Status = "completed"
				currentSubtask.CompletedAt = time.Now().UnixMilli()
				currentSubtask.Progress = 100
				// SSE: Notify subtask completion
				w.WriteSSE(map[string]interface{}{
					"type":       "step",
					"step":       "task_progress",
					"subtask_id": currentSubtask.ID,
					"status":     "completed",
					"content":    currentSubtask.Description,
					"progress":   100,
				})
				currentSubtask = taskDecomposer.GetNextSubtask(subtasks)
				// SSE: Notify next subtask start
				if currentSubtask != nil {
					currentSubtask.Status = "in_progress"
					currentSubtask.StartedAt = time.Now().UnixMilli()
					w.WriteSSE(map[string]interface{}{
						"type":       "step",
						"step":       "task_progress",
						"subtask_id": currentSubtask.ID,
						"status":     "in_progress",
						"content":    currentSubtask.Description,
					})
				}
			}

			// Auto-retry if answer was truncated by max_tokens
			if llmResp.FinishReason == "length" && iter < cfg.MaxIterations-1 {
				log.Printf("[Agent] answer truncated (finish_reason=length, len=%d), requesting continuation...", len(answer))
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "think",
					"content": "⚠️ 答案被截断，正在请求续写...",
				})
				conversation = appendRoleMessage(conversation, "assistant", answer)
				conversation = appendRoleMessage(conversation, "user",
					"你的回答被截断了。请继续完成上面的回答，从上次中断的地方接着写。不要重复已有内容。")
				iter++
				continue
			}

			// If answer is garbled, retry once
			if isGarbageOutput(answer) && iter < cfg.MaxIterations-1 {
				debugLog("garbage answer detected in main loop (len=%d), retrying...", len(answer))
				conversation = appendRoleMessage(conversation, "assistant", answer)
				conversation = appendRoleMessage(conversation, "user",
					"你的上一轮回答出现了乱码。请重新开始，使用工具完成任务。从头读取文件并继续实现功能。")
				iter++
				continue
			}
			if answer == "" {
				answer = "（Agent 未返回内容）"
			}

			// In Plan mode: check if answer includes a plan that needs approval
			if cfg.Mode == ModePlan {
				// Plan mode always returns answer directly
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "answer",
					"content": answer,
					"mode":    "plan",
				})
				answerSent = true
			} else {
			// P0-2: Enhanced declaration-execution consistency check
			// Check if answer claims modification without calling write_file/edit_file/write_file_batch
			if claimsFileModification(answer) && !writeFileCalled && !anyWriteCalled && iter < cfg.MaxIterations-1 {
				log.Printf("[Agent] answer claims modification but no write tool called (writeFileCalled=%v, anyWriteCalled=%v)", writeFileCalled, anyWriteCalled)
				conversation = appendRoleMessage(conversation, "assistant", answer)
				conversation = appendRoleMessage(conversation, "user",
					"你提到修改了文件但没有调用 write_file/edit_file。请立即调用 write_file 保存所有更改，或者直接回答。这是最后的机会。")
				// P0-2: Force one more iteration but mark that we've warned
				writeFileCalled = false // Reset to allow one more attempt
				continue
			}
			// P0-2: Additional check — if answer lists files but no writes happened
			if iter >= 2 && !anyWriteCalled && !answerSent {
				// Check if answer mentions specific file paths
				containsFilePath := strings.Contains(answer, "src/") || strings.Contains(answer, "lib/") ||
					strings.Contains(answer, ".rs") || strings.Contains(answer, ".go") ||
					strings.Contains(answer, ".js") || strings.Contains(answer, ".ts")
				if containsFilePath {
					log.Printf("[Agent] answer mentions file paths but no writes happened")
					conversation = appendRoleMessage(conversation, "user",
						"你的回答提到了文件路径，但没有实际调用 write_file。请调用 write_file 创建或修改文件，然后给出最终答案。")
					continue
				}
			}
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "answer",
					"content": answer,
				})
				answerSent = true
			}

			if sessionID != "" {
				r.convStore.Append(sessionID, service.Message{Role: "assistant", Content: answer})
			}
			w.WriteSSEPlain("[DONE]")
			iterCancel()
			return nil
		}

		// ── Case 2: Tool calls → execute (parallel for read-only tools) ──
		assistantMsg := map[string]interface{}{
			"role":       "assistant",
			"content":    llmResp.Content,
			"tool_calls": llmResp.ToolCalls,
		}
		conversation = append(conversation, assistantMsg)

		// Persist assistant message with tool_calls to convStore
		if sessionID != "" {
			// Convert LLMToolCall to service.ToolCall
			var tcList []service.ToolCall
			for _, tc := range llmResp.ToolCalls {
				tcList = append(tcList, service.ToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: service.Function{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				})
			}
			r.convStore.Append(sessionID, service.Message{
				Role:      "assistant",
				Content:   llmResp.Content,
				ToolCalls: tcList,
			})
		}

		// Prepare tool tasks: validate, dedup, analyze dependencies, group into
		// parallel-safe (read-only) and sequential (write/side-effect) sets
		plan := r.prepareToolTasks(llmResp, conversation, sessionID, w, cfg, readOnlySkills, maxReadFilePerTurn, maxWriteFilePerTurn)
		conversation = plan.conversation

		// Execute tools: parallel for read-only AND different-file writes, sequential for same-file writes
		var mu sync.Mutex
		var results []toolResult

		// Optimization 45: Worker pool for bounded concurrency
		// Instead of spawning unbounded goroutines, use a worker pool with max 8 workers
		// to prevent resource exhaustion and improve cache locality
		const maxParallelWorkers = 8
		if len(plan.parallelTasks) > 0 {
			// Use worker pool if we have more tasks than workers
			if len(plan.parallelTasks) > maxParallelWorkers {
				taskCh := make(chan toolTask, len(plan.parallelTasks))
				var wg sync.WaitGroup

				// Start workers
				for workerIdx := 0; workerIdx < maxParallelWorkers; workerIdx++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for task := range taskCh {
							incGoroutines()
							executeParallelTask(task, r, ctx, w, toolCache, sessionID, &mu, &results, m, modelTier, cfg)
							decGoroutines()
						}
					}()
				}

				// Send tasks to workers
				for _, pt := range plan.parallelTasks {
					taskCh <- pt
				}
				close(taskCh)
				wg.Wait()
			} else {
				// Few tasks: use simple goroutine-per-task (less overhead)
				var wg sync.WaitGroup
				for _, pt := range plan.parallelTasks {
					wg.Add(1)
					go func(task toolTask) {
						defer wg.Done()
						incGoroutines()
						executeParallelTask(task, r, ctx, w, toolCache, sessionID, &mu, &results, m, modelTier, cfg)
						decGoroutines()
					}(pt)
				}
				wg.Wait()
			}
		}

	// Execute sequential tasks (write/side-effect tools)
	anyWriteCalled := false
	editFileConsecutiveFailures := 0 // confidence check: track consecutive edit_file failures
	for _, st := range plan.sequentialTasks {
			// P1-3: Check call budget
			if !callBudget.CanCall(st.skillName) {
				budgetMsg := fmt.Sprintf("⚠️ 工具调用预算已用尽 (读取: %d/%d, 写入: %d/%d, 总计: %d/%d)",
					callBudget.ReadCalls, callBudget.MaxRead,
					callBudget.WriteCalls, callBudget.MaxWrite,
					callBudget.TotalCalls, callBudget.MaxTotal)
				log.Printf("[Agent] call budget exceeded: %s", budgetMsg)
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "skill_result",
					"skill":   st.skillName,
					"content": budgetMsg,
					"blocked": true,
				})
				conversation = r.appendToolResult(conversation, sessionID, st.tc.ID, budgetMsg)
				continue
			}

			// NEW: Permission check
			allowed, needsConfirm, reason := r.permChecker.CheckPermission(st.skillName, sessionID)
			if !allowed {
				denyMsg := fmt.Sprintf("❌ 权限拒绝: %s", reason)
				r.permChecker.LogDenial(sessionID, st.skillName, reason)
				log.Printf("[Agent] permission denied: %s - %s", st.skillName, reason)
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "skill_result",
					"skill":   st.skillName,
					"content": denyMsg,
					"blocked": true,
				})
				conversation = r.appendToolResult(conversation, sessionID, st.tc.ID, denyMsg)
				continue
			}

			// Notify frontend with permission info
			notifyData := map[string]interface{}{
				"type":  "step",
				"step":  "skill_call",
				"skill": st.skillName,
				"input": st.skillInput,
			}
			if needsConfirm {
				notifyData["needs_confirm"] = true
				notifyData["confirm_msg"] = r.permChecker.GetConfirmationMessage(st.skillName, st.skillInput)
			}
			w.WriteSSE(notifyData)

			// Confidence check: auto-pause after 5 consecutive edit_file failures
			if st.skillName == "edit_file" && editFileConsecutiveFailures >= 5 {
				pauseMsg := fmt.Sprintf("⚠️ [Confidence Check] edit_file 已连续失败 %d 次。Agent 可能陷入了无效的编辑循环。请确认是否继续执行，或建议 Agent 换一种方法（例如使用 write_file 重写整个文件）。", editFileConsecutiveFailures)
				log.Printf("[Agent] confidence check triggered: edit_file failed %d times consecutively", editFileConsecutiveFailures)
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "think",
					"content": pauseMsg,
				})
				// Inject system message to guide agent
				conversation = appendRoleMessage(conversation, "system",
					fmt.Sprintf("[System: Confidence check triggered. edit_file has failed %d times consecutively. "+
						"STOP using edit_file. Instead: (1) Use write_file to rewrite the entire file, "+
						"(2) Or analyze the root cause before trying again. Do NOT continue the same approach.]", editFileConsecutiveFailures))
				// Reset counter after injecting guidance so agent gets a chance
				editFileConsecutiveFailures = 0
			}

			// Keepalive during execution
			skillDone := make(chan struct{})
			startKeepalive(ctx, w, skillDone, 10*time.Second)

			// Execute with timeout
			toolTimeout := toolTimeoutForName(st.skillName)
			toolCtx, toolCancel := context.WithTimeout(ctx, toolTimeout)

			// P0-Fix: Acquire file lock for write operations.
			// BUG FIX: Previously used `defer fileLock.Unlock(path)` inside a for loop,
			// which deferred the unlock until Run() returns (not loop iteration end),
			// causing lock contention across sequential writes to different files.
			// Now use explicit unlock after executeSkill returns.
			var lockedPath string
			if st.skillName == "write_file" || st.skillName == "write_file_batch" {
				if path, ok := st.skillInput["path"].(string); ok {
					fileLock.Lock(path)
					lockedPath = path
				}
			}

			result, err := r.executeSkill(toolCtx, st.skillName, st.skillInput)
			toolCancel()
			close(skillDone)

			// Release file lock immediately after execution (not deferred to function exit)
			if lockedPath != "" {
				fileLock.Unlock(lockedPath)
			}

			if toolCtx.Err() == context.DeadlineExceeded {
				result = fmt.Sprintf("⚠️ Tool execution timed out after %v", toolTimeout)
			} else if err != nil && st.skillName == "write_file" {
				log.Printf("[Agent] write_file failed: %v, providing error context to LLM", err)
				result = fmt.Sprintf("Write failed: %v. Please check the path and content, then try again.", err)
			} else if err != nil {
				// P0-2: Classify error and determine recovery strategy
				errCategory := ClassifyError(err.Error())
				m.toolConsecutiveErrors[st.skillName]++
				recovery := GetRecoveryStrategy(errCategory, m.toolConsecutiveErrors[st.skillName])
				recoveryMsg := GetRecoveryMessage(recovery, st.skillName)
				log.Printf("[Agent] tool %s failed (attempt %d): %v, category=%d, recovery=%d",
					st.skillName, m.toolConsecutiveErrors[st.skillName], err, errCategory, recovery)

				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "think",
					"content": recoveryMsg,
				})

				switch recovery {
				case RecoveryRetrySame:
					result = fmt.Sprintf("Error: %v. Please try again.", err)
				case RecoverySimplifyInput:
					simplified := toolRetryFallback.SimplifyTaskInput(st.skillName, st.skillInput)
					retryResult, retryErr := r.executeSkill(toolCtx, st.skillName, simplified)
					if retryErr == nil {
						result = retryResult
						m.toolConsecutiveErrors[st.skillName] = 0
					} else {
						result = fmt.Sprintf("Error: %v (simplified retry also failed: %v)", err, retryErr)
					}
				case RecoverySwitchModel:
					result = fmt.Sprintf("Error: %v. Model rate limited, please try a different approach.", err)
				case RecoveryForceAnswer:
					result = fmt.Sprintf("Error: %v. Please provide your best answer based on available information.", err)
					conversation = appendRoleMessage(conversation, "user",
						"[System: Multiple tool failures. Please provide your final answer based on available information.]")
					answerSent = true
				case RecoverySkipTool:
					result = fmt.Sprintf("Skipped tool '%s' due to error: %v", st.skillName, err)
				case RecoveryCompactContext:
					result = fmt.Sprintf("Error: %v. Context will be compacted on next iteration.", err)
				case RecoveryAbort:
					w.WriteSSE(map[string]interface{}{
						"type":  "error",
						"error": fmt.Sprintf("多次执行失败，已终止: %v", err),
					})
					w.WriteSSEPlain("[DONE]")
					iterCancel()
					return err
				default:
					result = fmt.Sprintf("Error: %v", err)
				}
			} else {
				m.toolConsecutiveErrors[st.skillName] = 0 // reset on success
			}

			// P2-1: Track subtask progress based on tool execution
			if currentSubtask != nil && err != nil {
				currentSubtask.RetryCount++
				// Mark subtask as failed after 3 retries
				if currentSubtask.RetryCount >= 3 {
					currentSubtask.Status = "failed"
					w.WriteSSE(map[string]interface{}{
						"type":       "step",
						"step":       "task_progress",
						"subtask_id": currentSubtask.ID,
						"status":     "failed",
						"content":    currentSubtask.Description,
						"error":      err.Error(),
					})
					// Try to move to next subtask
					currentSubtask = taskDecomposer.GetNextSubtask(subtasks)
					if currentSubtask != nil {
						currentSubtask.Status = "in_progress"
						currentSubtask.StartedAt = time.Now().UnixMilli()
						w.WriteSSE(map[string]interface{}{
							"type":       "step",
							"step":       "task_progress",
							"subtask_id": currentSubtask.ID,
							"status":     "in_progress",
							"content":    currentSubtask.Description,
						})
					}
				}
			}

			// Track edit_file failures for confidence check
			if st.skillName == "edit_file" {
				isEditError := err != nil || strings.HasPrefix(result, "Error:") || strings.HasPrefix(result, "❌")
				if isEditError {
					editFileConsecutiveFailures++
				} else {
					editFileConsecutiveFailures = 0
				}
			}

			if st.skillName == "write_file" {
				writeFileCalled = true
				anyWriteCalled = true
				stagnationDetector.ResetNoWrite()
				if path, ok := st.skillInput["path"].(string); ok {
					toolCache.invalidate(path)
					// Invalidate file hash cache so read_file re-reads fresh content
					if r.fileHashCache != nil {
						r.fileHashCache.Invalidate(path)
					}
					// Update repo-map incrementally after write
					if r.repoMap != nil {
						if content, ok := st.skillInput["content"].(string); ok {
							r.repoMap.UpdateFile(path, content)
						}
					}
					// Optimization 17: Cache written content for immediate read-back
					if content, ok := st.skillInput["content"].(string); ok && err == nil {
						r.cacheWriteContent(sessionID, path, content)
						// P2-2: Quality verification AFTER write (deferred to background)
						// P2-Fix: Bounded by qualitySem to prevent goroutine explosion.
						go func(p, c string) {
							qualitySem <- struct{}{}        // acquire
							defer func() { <-qualitySem }() // release
							report := qualityVerifier.VerifyFile(p, c)
							mu.Lock()
							qualityReports = append(qualityReports, report)
							mu.Unlock()
							if len(report.Issues) > 0 {
								log.Printf("[Agent] quality issues in %s: %v", p, report.Issues)
							}
							// P1-1: Auto-rollback if quality score is too low
							if report.Score < 40 {
								log.Printf("[Agent] quality score %d < 40 for %s (rollback deferred)", report.Score, p)
							}
						}(path, content)
					}
				}
				// Optimization 22: Invalidate build_module cache when source files change
				if cfg.ProjectID != "" {
					toolCache.InvalidateBuild(cfg.ProjectID)
				}
				if cfg.ProjectID == "" && strings.HasPrefix(result, "[project_id:") {
					if endIdx := strings.Index(result, "]"); endIdx > 12 {
						autoPID := result[12:endIdx]
						cfg.ProjectID = autoPID
						log.Printf("[Agent] write_file auto-created project: %s", autoPID)
						w.WriteSSE(map[string]interface{}{
							"type":       "project_created",
							"project_id": autoPID,
						})
					}
				}
				if path, ok := st.skillInput["path"].(string); ok {
					checkpoints = append(checkpoints, FileCheckpoint{
						Path:    path,
						Content: result,
						Time:    time.Now(),
					})
					w.WriteSSE(map[string]interface{}{
						"type":       "checkpoint",
						"path":       path,
						"can_undo":   true,
						"checkpoint": len(checkpoints),
					})
				}
			}

			// Track operations
			m.toolCallHistory[st.skillName]++
			m.totalToolCalls++
			opKey := st.skillName
			if st.skillName == "read_file" || st.skillName == "write_file" {
				if path, ok := st.skillInput["path"].(string); ok {
					opKey = st.skillName + ":" + path
				}
			}
			m.uniqueOps[opKey] = true

			// O(1): Update pre-computed unique targets counter for loop detection
			if !strings.Contains(opKey, ":") {
				// This is a tool call without a specific target (e.g., grep_search)
				m.uniqueTargetsPerSkill[st.skillName]++
			} else if st.skillName == "read_file" || st.skillName == "write_file" {
				// For file-specific tools, count unique file paths
				if path, ok := st.skillInput["path"].(string); ok {
					// Use a composite key to track unique files per skill
					uniqueKey := st.skillName + ":" + path
					if _, exists := m.uniqueOps[uniqueKey]; !exists {
						m.uniqueTargetsPerSkill[st.skillName]++
					}
				}
			}

			// P2-1: Record reflection
			if err != nil {
				reflectionLog.Record(st.skillName, "failure", err.Error(), iter+1)
			} else {
				reflectionLog.Record(st.skillName, "success", "", iter+1)
			}

			// NEW: Audit logging
			r.auditLog.RecordToolCall(
				sessionID,
				st.skillName,
				st.tc.ID,
				st.skillInput,
				result,
				err == nil,
				time.Since(startTime),
				iter+1,
				cfg.UserID,
				cfg.ProjectID,
			)

			// NEW: Session persistence for checkpoints
			if sessionID != "" && st.skillName == "write_file" {
				if path, ok := st.skillInput["path"].(string); ok {
					if content, ok := st.skillInput["content"].(string); ok {
						r.sessionPersist.UpdateCheckpoint(sessionID, path, content)
						r.sessionPersist.Save(sessionID)
					}
				}
			}

			// Truncate large results
			result = truncateResultForModel(result, st.skillName, modelTier, cfg.MaxResultLen)

			w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "skill_result",
				"skill":   st.skillName,
				"content": result,
			})

			mu.Lock()
			appendToolResultToList(&results, st.tc, result)
			mu.Unlock()
		}

		// Optimization 6: Map deduplicated tool calls back to their executed counterparts.
		// For each skipped duplicate, find the matching executed result and reuse it.
		if len(plan.skippedToolCalls) > 0 {
			for _, origTC := range plan.skippedToolCalls {
				// Compute same FNV hash as used in prepareToolTasks
				h := fnv.New64a()
				h.Write([]byte(origTC.Function.Name))
				h.Write([]byte{':'})
				h.Write([]byte(origTC.Function.Arguments))
				dedupHash := h.Sum64()
				if executedIdx, ok := plan.seen[dedupHash]; ok {
					for _, res := range results {
						if res.tc.Function.Name == plan.deduped[executedIdx].skillName &&
							res.tc.Function.Arguments == plan.deduped[executedIdx].tc.Function.Arguments {
							mu.Lock()
							appendToolResultToList(&results, origTC, res.result)
							mu.Unlock()
							debugLog("dedup: mapped skipped %s (id=%s) to executed result (id=%s)",
								origTC.Function.Name, origTC.ID, res.tc.ID)
							break
						}
					}
				}
			}
		}

		// Post-execution analysis: stagnation detection, self-reflection, loop detection
		var procErr error
		conversation, answerSent, procErr = trp.process(iter, conversation, results, anyWriteCalled, answerSent)
		if procErr != nil {
			iterCancel() // Release iteration timeout resources
			return procErr
		}
		iterCancel() // Release iteration timeout resources
		if answerSent {
			// FIX: Break instead of continue — forceAnswer already sent the final answer.
			// Continuing would let the LLM generate more tool calls and loop forever.
			log.Printf("[Agent] answerSent=true at iter %d, breaking out of loop", iter+1)
			break
		}

		// HARD CAP: After 8 iterations with zero writes, force answer and break.
		// This prevents the LLM from generating endless tool calls even after stagnation warnings.
		if !anyWriteCalled && iter >= 7 {
			log.Printf("[Agent] HARD CAP: %d iterations with 0 writes, forcing final answer", iter+1)
			_ = r.forceAnswer(ctx, conversation, w, sessionID, cfg, reqProviderID, reqModel,
				fmt.Sprintf("CRITICAL STOP: You have completed %d iterations without writing a single file. "+
					"This is unacceptable. You MUST now stop all tool calls and provide your final answer "+
					"based on what you have already read. Do NOT call any more tools.", iter+1))
			break
		}
	}

	// Exhausted iterations — send answer if we haven't already
	sendFinalAnswer(w, cfg, lastLLMResp, answerSent)
	// Clean up write-content cache for this session (tool result cache persists)
	r.writeContentCache.Delete(sessionID)
	r.readFileCache.Delete(sessionID)

	// Optimization 51: Log performance summary
	totalDuration := time.Since(startTime)
	runPerfMetrics.TotalRunDuration = totalDuration
	perfSummary := runPerfMetrics.GetSummary()
	log.Printf("[Agent:Performance] Session=%s Duration=%v LLM_Calls=%d Tool_Calls=%d Errors=%d Retries=%d",
		sessionID, totalDuration, perfSummary["llm_call_count"], perfSummary["tool_call_count"],
		perfSummary["error_count"], perfSummary["retry_count"])

	if iterCancel != nil {
		iterCancel()
	}
	return nil
}

// executeParallelTask executes a single parallel tool task with caching, timeout, and result tracking.
// This function is used by both the worker pool and goroutine-per-task approaches.
func executeParallelTask(task toolTask, r *AgentRunner, ctx context.Context, w SSEWriter, toolCache *toolResultCache, sessionID string, mu *sync.Mutex, results *[]toolResult, m *runMetrics, modelTier ModelTier, cfg RunConfig) {
	// Notify frontend
	w.WriteSSE(map[string]interface{}{
		"type":  "step",
		"step":  "skill_call",
		"skill": task.skillName,
		"input": task.skillInput,
	})

	// Check cache first
	if cached := toolCache.get(task.skillName, task.skillInput); cached != "" {
		debugLog("cache HIT (parallel) for %s", task.skillName)
		mu.Lock()
		appendToolResultToList(results, task.tc, cached)
		mu.Unlock()
		w.WriteSSE(map[string]interface{}{
			"type":    "step",
			"step":    "skill_result",
			"skill":   task.skillName,
			"content": cached,
		})
		return
	}

	// Optimization 17: Check write-content cache for read_file
	if task.skillName == "read_file" {
		if path, ok := task.skillInput["path"].(string); ok {
			if wc := r.getCachedWriteContent(sessionID, path); wc != "" {
				debugLog("writeContentCache HIT for read_file: %s", path)
				mu.Lock()
				appendToolResultToList(results, task.tc, wc)
				mu.Unlock()
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "skill_result",
					"skill":   task.skillName,
					"content": wc,
				})
				return
			}
			// Check read_file content cache
			if rc := r.getCachedReadFile(sessionID, path); rc != "" {
				debugLog("readFileCache HIT for read_file: %s", path)
				mu.Lock()
				appendToolResultToList(results, task.tc, rc)
				mu.Unlock()
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "skill_result",
					"skill":   task.skillName,
					"content": rc,
				})
				return
			}
		}
	}

	// Execute with timeout
	toolTimeout := toolTimeoutForName(task.skillName)
	toolCtx, toolCancel := context.WithTimeout(ctx, toolTimeout)
	defer toolCancel()
	result, err := r.executeSkill(toolCtx, task.skillName, task.skillInput)
	if toolCtx.Err() == context.DeadlineExceeded {
		result = fmt.Sprintf("⚠️ Tool execution timed out after %v", toolTimeout)
	} else if err != nil {
		result = fmt.Sprintf("Error: %v", err)
	} else {
		toolCache.put(task.skillName, task.skillInput, result)
		// Cache read_file results for reuse within session
		if task.skillName == "read_file" {
			if path, ok := task.skillInput["path"].(string); ok {
				r.cacheReadFile(sessionID, path, result)
			}
		}
	}

	// Truncate large results
	result = truncateResultForModel(result, task.skillName, modelTier, cfg.MaxResultLen)

	mu.Lock()
	appendToolResultToList(results, task.tc, result)
	// Track parallel tool calls for loop detection
	m.toolCallHistory[task.skillName]++
	m.totalToolCalls++
	opKey := task.skillName
	if path, ok := task.skillInput["path"].(string); ok {
		opKey = task.skillName + ":" + path
	}
	m.uniqueOps[opKey] = true

	// O(1): Update pre-computed unique targets counter for loop detection
	if !strings.Contains(opKey, ":") {
		m.uniqueTargetsPerSkill[task.skillName]++
	} else if task.skillName == "read_file" || task.skillName == "write_file" {
		if path, ok := task.skillInput["path"].(string); ok {
			uniqueKey := task.skillName + ":" + path
			if _, exists := m.uniqueOps[uniqueKey]; !exists {
				m.uniqueTargetsPerSkill[task.skillName]++
			}
		}
	}
	mu.Unlock()

	w.WriteSSE(map[string]interface{}{
		"type":    "step",
		"step":    "skill_result",
		"skill":   task.skillName,
		"content": result,
	})
}

// ═══════════════════════════════════════════════════════════════════
// NEW: Statistics and monitoring methods
// ═══════════════════════════════════════════════════════════════════

// GetToolStats returns tool usage statistics from audit log.
func (r *AgentRunner) GetToolStats() map[string]ToolStats {
	return r.auditLog.GetToolStats()
}

// GetAuditHistory returns recent audit entries.
func (r *AgentRunner) GetAuditHistory(toolName string, limit int) []AuditEntry {
	return r.auditLog.GetHistory(toolName, limit)
}

// GetPermissionDenials returns recent permission denials.
func (r *AgentRunner) GetPermissionDenials(limit int) []DenialRecord {
	return r.permChecker.GetDenials(limit)
}

// GetSessionState returns the session state for a given session ID.
func (r *AgentRunner) GetSessionState(sessionID string) *SessionState {
	return r.sessionPersist.GetOrCreate(sessionID)
}
