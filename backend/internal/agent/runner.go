package agent

import (
	"context"
	"database/sql"
	"encoding/base64"
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
)

func incGoroutines() int64 { return atomic.AddInt64(&activeGoroutines, 1) }
func decGoroutines()       { atomic.AddInt64(&activeGoroutines, -1) }

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

// MCP ask-mode approval timeout: how long the runner waits for the user to
// confirm a write-tool call before treating it as denied.
const (
	mcpApprovalTimeout    = 120 * time.Second
	mcpApprovalTimeoutSec = 120
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

	// Monthly AI cost cap (USD, 0 = unlimited). Set from app config at construction.
	monthlyCostLimit float64

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
	securityEngine *SecurityEngine
	sessionPersist *SessionPersistence
	depGraph       *DependencyGraph

	// NEW: High-value optimization modules
	buildHealer    *BuildHealer        // Auto-fix build errors
	atomicWriter   *AtomicWriter       // Multi-file atomic writes
	enhancedPlan   *EnhancedPlanner    // File-level granularity planning
	fileDepGraph   *FileDependencyGraph // File dependency tracking for smart reads

	// Optimization 51: Performance metrics tracking
	perfMetrics *PerformanceMetrics

	// Last daily-usage persistence snapshot (deltas are written to ai_usage_daily)
	lastUsageSnapshot usageSnapshot

	// In-flight MCP write-tool permission requests awaiting user confirmation (ask mode)
	pendingApprovals sync.Map // requestID -> *ApprovalRequest

	// File hash cache for UNCHANGED detection in read_file
	fileHashCache *fileHashCache

	// Repo-map for global code structure indexing
	repoMap *RepoMap

	// P2: Progress tracking per session
	progressTrackers sync.Map // sessionID -> *ProgressTracker

	// NEW: Context caching improvements (based on OpenHands/Aider/GPTCache)
	prefixCache    *PrefixCache    // LLM prompt prefix optimization
	semanticCache  *SemanticCache  // Cache similar LLM responses
	contextCondenser *ContextCondenser // LLM-based context summarization
	sessionLearner *SessionLearner // Learn from successful patterns
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
		securityEngine: NewSecurityEngine(),
		sessionPersist: NewSessionPersistence(""),
		depGraph:       NewDependencyGraph(),
		perfMetrics:    NewPerformanceMetrics(),
		// NEW: High-value optimization modules
		buildHealer:    NewBuildHealer(),
		atomicWriter:   NewAtomicWriter(""),
		enhancedPlan:   nil, // initialized per-session with project path
		fileDepGraph:   nil, // initialized per-session with project path
		// NEW: Context caching improvements
		prefixCache:      NewPrefixCache(100, 5*time.Minute),
		semanticCache:    NewSemanticCache(500, 0.85),
		contextCondenser: NewContextCondenser(30, 6, 1),
		sessionLearner:   NewSessionLearner(100),
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

	for _, line := range lines {
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
	extIdx := strings.LastIndex(filePath, ".")
	if extIdx < 0 {
		extIdx = 0 // no extension found, treat as empty
	}
	ext := strings.ToLower(filePath[extIdx:])
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

// getOrCreateProgressTracker returns (or creates) a session-scoped progress tracker.
func (r *AgentRunner) getOrCreateProgressTracker(sessionID string) *ProgressTracker {
	if sessionID == "" {
		return NewProgressTracker()
	}
	if cached, ok := r.progressTrackers.Load(sessionID); ok {
		return cached.(*ProgressTracker)
	}
	tracker := NewProgressTracker()
	r.progressTrackers.Store(sessionID, tracker)
	return tracker
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
	defer r.persistDailyUsage(userID)

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
	// Sync ProviderID so callLLMSummary (context_compact.go) and other
	// downstream functions use the correct provider when loading API keys.
	if reqProviderID != "" {
		cfg.ProviderID = reqProviderID
	}
	// If resolvedAPIKey is still empty (handler fallback set endpoint but no key),
	// force-load from custom_providers table to avoid 401 errors.
	if resolvedAPIKey == "" && reqProviderID != "" && r.db != nil {
		var cpKey, cpEp string
		cpErr := r.db.QueryRow(
			"SELECT api_key, endpoint FROM custom_providers WHERE name=? AND user_id=?",
			reqProviderID, userID,
		).Scan(&cpKey, &cpEp)
		if cpErr != nil {
			cpErr = r.db.QueryRow(
				"SELECT api_key, endpoint FROM custom_providers WHERE id=? AND user_id=?",
				reqProviderID, userID,
			).Scan(&cpKey, &cpEp)
		}
		if cpErr == nil && cpKey != "" {
			if b, dErr := base64.StdEncoding.DecodeString(cpKey); dErr == nil {
				cfg.resolvedAPIKey = string(b)
			} else {
				cfg.resolvedAPIKey = cpKey
			}
			if cpEp != "" {
				cfg.resolvedEndpoint = cpEp
			}
			cfg.resolvedModel = reqModel
			if cpEp == "" {
				cpEp = cfg.resolvedEndpoint
			}
			log.Printf("[Agent] Run: force-loaded custom provider=%s endpoint=%s model=%s apiKey_len=%d",
				reqProviderID, cfg.resolvedEndpoint, cfg.resolvedModel, len(cfg.resolvedAPIKey))
		}
	}
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

	// Auto-recall relevant past memories (works for every model tier; the
	// memory_v2 skill alone is unreliable on free/small models because the
	// LLM must remember to call it proactively).
	if len(task) > 0 {
		if recalled := r.autoRecallMemory(cfg, task, 3); recalled != "" {
			systemPrompt += "\n" + recalled
		}
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
	anyWriteCalled := false       // P0-2: Track if any write tool was called
	buildModuleCalled := false    // Auto-trigger: track if build_module was called
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

	// P2: Progress tracking for this session
	progressTracker := r.getOrCreateProgressTracker(sessionID)

	// Post-execution analysis — stagnation detection, self-reflection, loop detection, progress tracking
	trp := &toolResultProcessor{
		r: r, ctx: ctx, w: w, cfg: cfg, sessionID: sessionID,
		reqProviderID: reqProviderID, reqModel: reqModel,
		stagnationDetector: stagnationDetector, m: m,
		progressTracker: progressTracker,
	}

	// P0-2: Tool retry fallback
	toolRetryFallback := &ToolRetryFallback{
		db:           r.db,
		currentModel: resolvedModel,
	}

	// P2-1: Enhanced Task planner with file-level granularity
	// Reference: OpenHands (PLAN.md) + MetaGPT (role-based SOP) + AutoGPT (loop execution)
	// NEW: Use EnhancedPlanner for file-level granularity planning
	var enhancedPlanner *EnhancedPlanner
	var enhancedPlan *EnhancedPlan
	var planInjected bool // Track if we've injected the first plan context

	// Initialize enhanced planner with project path
	if cfg.ProjectID != "" && r.db != nil {
		var storagePath string
		r.db.QueryRow(`SELECT COALESCE(storage_path,'') FROM projects WHERE id=?`, cfg.ProjectID).Scan(&storagePath)
		if storagePath != "" {
			enhancedPlanner = NewEnhancedPlanner(r.db, storagePath)
		}
	}
	if enhancedPlanner == nil {
		enhancedPlanner = NewEnhancedPlanner(r.db, "")
	}

	// Try LLM-based planning for complex tasks
	if isComplexTask(task) {
		plan, err := enhancedPlanner.GeneratePlan(ctx, task, cfg.ProjectContext, r, cfg)
		if err == nil && plan != nil {
			enhancedPlan = plan
			log.Printf("[Agent] enhanced plan generated: %d steps (depth=%d)", len(plan.Steps), plan.Depth)
		} else {
			log.Printf("[Agent] enhanced planning failed, using fallback: %v", err)
		}
	}

	// SSE: Send task plan to frontend
	if enhancedPlan != nil && len(enhancedPlan.Steps) > 0 {
		w.WriteSSE(map[string]interface{}{
			"type":    "step",
			"step":    "task_plan",
			"content": fmt.Sprintf("📋 任务计划完成，共 %d 个步骤 (深度: %d)", len(enhancedPlan.Steps), enhancedPlan.Depth),
			"plan":    enhancedPlan,
		})
		// Mark first step as in_progress
		if step := enhancedPlanner.GetCurrentStep(enhancedPlan); step != nil {
			step.Status = "in_progress"
			step.StartedAt = time.Now().Unix()
			w.WriteSSE(map[string]interface{}{
				"type":       "step",
				"step":       "task_progress",
				"step_id":    step.ID,
				"status":     "in_progress",
				"content":    step.Description,
				"progress":   enhancedPlanner.GetProgress(enhancedPlan),
				"files":      step.Files,
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
			rm := NewRepoMap(projectPath)
			r.repoMap = rm
			// P3-Fix: Generate initial repo-map with timeout protection.
			// Large projects can take seconds to scan; this prevents blocking indefinitely.
			go func() {
				rm.GenerateRepoMapWithTimeout(ctx, projectPath, 10*time.Second)
				log.Printf("[Agent] repo-map generated: %d files indexed", len(rm.fileIndex))
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
		r.perfMetrics.RecordIteration()
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

		// P2-1 V2: Inject plan context at start of each iteration
		// This ensures the LLM always knows the current task
		if enhancedPlan != nil && !planInjected {
			stepContext := enhancedPlanner.BuildContextMessage(enhancedPlan)
			if stepContext != "" {
				conversation = appendRoleMessage(conversation, "system", stepContext)
				planInjected = true
				log.Printf("[Agent] injected enhanced plan context into conversation")
			}
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

			// P2-1 V2: Task plan step completion
			if enhancedPlan != nil {
				currentStep := enhancedPlanner.GetCurrentStep(enhancedPlan)
				if currentStep != nil {
					// Check if agent indicates completion
					if enhancedPlanner.IsStepDone(answer) || iter > 2 {
						// Mark current step as completed
						currentStep.Status = "completed"
						currentStep.CompletedAt = time.Now().Unix()
						currentStep.Result = truncateString(answer, 200)

						// SSE: Notify step completion
						w.WriteSSE(map[string]interface{}{
							"type":       "step",
							"step":       "task_progress",
							"step_id":    currentStep.ID,
							"status":     "completed",
							"content":    currentStep.Description,
							"progress":   enhancedPlanner.GetProgress(enhancedPlan),
							"files":      currentStep.Files,
						})

						// Advance to next step
						nextStep := enhancedPlanner.AdvanceToNextStep(enhancedPlan)
						if nextStep != nil {
							// SSE: Notify next step start
							nextStep.Status = "in_progress"
							nextStep.StartedAt = time.Now().Unix()
							w.WriteSSE(map[string]interface{}{
								"type":       "step",
								"step":       "task_progress",
								"step_id":    nextStep.ID,
								"status":     "in_progress",
								"content":    nextStep.Description,
								"progress":   enhancedPlanner.GetProgress(enhancedPlan),
								"files":      nextStep.Files,
							})

							// Inject next step context into conversation
							stepContext := enhancedPlanner.BuildContextMessage(enhancedPlan)
							conversation = appendRoleMessage(conversation, "system", stepContext)
							log.Printf("[Agent] advancing to step: %s", nextStep.Description)
						} else {
							log.Printf("[Agent] all enhanced plan steps completed")
						}
					}
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

			// NEW: Security check for bash commands
			if st.skillName == "bash" {
				if command, ok := st.skillInput["command"].(string); ok {
					allowed, needsConfirm, riskScore, secMsg := r.securityEngine.AuditAndCheck(command, sessionID)
					if !allowed {
						log.Printf("[Security] DENIED session=%s risk=%d", sessionID, riskScore)
						w.WriteSSE(map[string]interface{}{
							"type":       "step",
							"step":       "skill_result",
							"skill":      st.skillName,
							"content":    secMsg,
							"blocked":    true,
							"risk_score": riskScore,
						})
						conversation = r.appendToolResult(conversation, sessionID, st.tc.ID, secMsg)
						continue
					}
					if needsConfirm {
						notifyData := map[string]interface{}{
							"type":       "step",
							"step":       "security_confirm",
							"skill":      st.skillName,
							"command":    command,
							"risk_score": riskScore,
							"message":    secMsg,
						}
						w.WriteSSE(notifyData)
					}
				}
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

			result, err := r.executeSkill(toolCtx, st.skillName, st.skillInput, w)
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
					retryResult, retryErr := r.executeSkill(toolCtx, st.skillName, simplified, w)
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

			// P2-1 V2: Track step progress based on tool execution
			if enhancedPlan != nil && err != nil {
				currentStep := enhancedPlanner.GetCurrentStep(enhancedPlan)
				if currentStep != nil {
					enhancedPlanner.MarkStepFailed(enhancedPlan, err.Error())
					// SSE: Notify step failure
					w.WriteSSE(map[string]interface{}{
						"type":       "step",
						"step":       "task_progress",
						"step_id":    currentStep.ID,
						"status":     currentStep.Status,
						"content":    currentStep.Description,
						"error":      err.Error(),
						"progress":   enhancedPlanner.GetProgress(enhancedPlan),
						"files":      currentStep.Files,
					})

					// If we moved to next step, inject context
					if nextStep := enhancedPlanner.GetCurrentStep(enhancedPlan); nextStep != nil && nextStep.ID != currentStep.ID {
						stepContext := enhancedPlanner.BuildContextMessage(enhancedPlan)
						conversation = appendRoleMessage(conversation, "system", stepContext)
						log.Printf("[Agent] after tool failure, advancing to step: %s", nextStep.Description)
					}
				}
			}

			// Track edit_file failures for confidence check
			if st.skillName == "edit_file" {
				anyWriteCalled = true // edit_file is a write operation
				isEditError := err != nil || strings.HasPrefix(result, "Error:") || strings.HasPrefix(result, "❌")
				if isEditError {
					editFileConsecutiveFailures++
				} else {
					editFileConsecutiveFailures = 0
				}
			}

			if st.skillName == "write_file" || st.skillName == "write_file_batch" {
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

			// Track build_module calls for auto-trigger
			if st.skillName == "build_module" {
				buildModuleCalled = true
			}

			// NEW: Build error auto-healing for build_module failures
			if st.skillName == "build_module" && err != nil {
				projectPath := ""
				if cfg.ProjectID != "" && r.db != nil {
					var storagePath string
					r.db.QueryRow(`SELECT COALESCE(storage_path,'') FROM projects WHERE id=?`, cfg.ProjectID).Scan(&storagePath)
					projectPath = storagePath
				}
				if projectPath != "" {
					healResult, shouldHeal := r.buildHealer.HandleBuildFailure(ctx, sessionID, result, projectPath, w)
					if shouldHeal && healResult.ContextForLLM != "" {
						// Inject build error context into conversation for LLM to fix
						conversation = appendRoleMessage(conversation, "system", healResult.ContextForLLM)
						log.Printf("[Agent] build_healer: injected error context for %d diagnostics", len(healResult.Diagnostics))
					} else if healResult.Strategy == HealForceAnswer || healResult.Strategy == HealAbort {
						// Too many attempts or critical error — force answer
						result = fmt.Sprintf("%s\n\n%s", result, healResult.UserMessage)
					}
				}
			}

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

	// Auto-trigger build_module: if files were written but build_module was never called,
	// automatically trigger it to ensure the module is compiled and packaged.
	if anyWriteCalled && !buildModuleCalled && cfg.ProjectID != "" {
		log.Printf("[Agent] Auto-trigger: files written but build_module not called, triggering build for project %s", cfg.ProjectID)
		w.WriteSSE(map[string]interface{}{
			"type":  "step",
			"step":  "skill_call",
			"skill": "build_module",
			"input": map[string]interface{}{"project_id": cfg.ProjectID},
		})
		buildTimeout := toolTimeoutForName("build_module")
		buildCtx, buildCancel := context.WithTimeout(ctx, buildTimeout)
		buildResult, buildErr := r.executeSkill(buildCtx, "build_module", map[string]interface{}{
			"project_id": cfg.ProjectID,
		}, w)
		buildCancel()
		if buildErr != nil {
			log.Printf("[Agent] Auto-trigger build_module failed: %v", buildErr)
		} else {
			log.Printf("[Agent] Auto-trigger build_module result: %s", truncateString(buildResult, 200))
			// Send build result to frontend
			w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "skill_result",
				"skill":   "build_module",
				"content": buildResult,
			})
		}
	}

	// Exhausted iterations — send answer if we haven't already
	sendFinalAnswer(w, cfg, lastLLMResp, answerSent)

	// Auto-store the final answer as an episodic memory so future sessions can
	// recall it without the LLM remembering to call the memory_v2 skill.
	if lastLLMResp != nil && len(lastLLMResp.Content) > 0 {
		r.autoStoreMemory(cfg.UserID, sessionID, lastLLMResp.Content)
	}
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
	result, err := r.executeSkill(toolCtx, task.skillName, task.skillInput, w)
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
// GetPerfMetrics returns the aggregated process-lifetime performance metrics
// (LLM calls/tokens, tool calls, errors, retries) for observability UIs.
func (r *AgentRunner) GetPerfMetrics() map[string]interface{} {
	return r.perfMetrics.GetSummary()
}

// GetDailyUsage returns per-day aggregated AI usage for trend charts,
// scoped to the given user. Rows are ordered ascending by date (oldest first).
func (r *AgentRunner) GetDailyUsage(limit int, userID string) []map[string]interface{} {
	if r.db == nil {
		return []map[string]interface{}{}
	}
	rows, err := r.db.Query(`SELECT date, llm_call_count, llm_token_usage, tool_call_count, error_count, retry_count
		FROM ai_usage_daily WHERE user_id = ? ORDER BY date DESC LIMIT ?`, userID, limit)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer rows.Close()
	days := []map[string]interface{}{}
	for rows.Next() {
		var date string
		var calls, tokens, tools, errs, retries int64
		if err := rows.Scan(&date, &calls, &tokens, &tools, &errs, &retries); err != nil {
			continue
		}
		days = append(days, map[string]interface{}{
			"date":             date,
			"llm_call_count":   calls,
			"llm_token_usage":  tokens,
			"tool_call_count":  tools,
			"error_count":      errs,
			"retry_count":      retries,
		})
	}
	// Reverse to ascending order for charting
	for i, j := 0, len(days)-1; i < j; i, j = i+1, j-1 {
		days[i], days[j] = days[j], days[i]
	}
	return days
}

// persistDailyUsage writes the delta since the last snapshot into ai_usage_daily
// for the current day. Called once per Run so restart losses are bounded by
// one in-flight task.
var usagePersistMu sync.Mutex

func (r *AgentRunner) persistDailyUsage(userID string) {
	if r.db == nil {
		return
	}
	usagePersistMu.Lock()
	defer usagePersistMu.Unlock()

	pm := r.perfMetrics
	pm.mu.Lock()
	calls := pm.LLMCallCount - r.lastUsageSnapshot.Calls
	tokens := pm.LLMTokenUsage - r.lastUsageSnapshot.Tokens
	tools := pm.ToolCallCount - r.lastUsageSnapshot.Tools
	errs := pm.ErrorCount - r.lastUsageSnapshot.Errors
	retries := pm.RetryCount - r.lastUsageSnapshot.Retries
	pm.mu.Unlock()

	if calls <= 0 && tokens <= 0 && tools <= 0 && errs <= 0 && retries <= 0 {
		return
	}
	today := time.Now().Format("2006-01-02")
	_, err := r.db.Exec(`INSERT INTO ai_usage_daily (date, user_id, llm_call_count, llm_token_usage, tool_call_count, error_count, retry_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(date, user_id) DO UPDATE SET
			llm_call_count = llm_call_count + excluded.llm_call_count,
			llm_token_usage = llm_token_usage + excluded.llm_token_usage,
			tool_call_count = tool_call_count + excluded.tool_call_count,
			error_count = error_count + excluded.error_count,
			retry_count = retry_count + excluded.retry_count,
			updated_at = CURRENT_TIMESTAMP`,
		today, userID, calls, tokens, tools, errs, retries)
	if err != nil {
		log.Printf("[Usage] persist daily usage: %v", err)
		return
	}
	r.lastUsageSnapshot = usageSnapshot{Calls: pm.LLMCallCount, Tokens: pm.LLMTokenUsage, Tools: pm.ToolCallCount, Errors: pm.ErrorCount, Retries: pm.RetryCount}
	log.Printf("[Usage] persisted %d calls / %d tokens for %s", calls, tokens, today)
}

type usageSnapshot struct {
	Calls   int64
	Tokens  int64
	Tools   int64
	Errors  int64
	Retries int64
}

func (r *AgentRunner) GetAuditHistory(toolName string, limit int) []AuditEntry {	return r.auditLog.GetHistory(toolName, limit)
}

// GetPermissionDenials returns recent permission denials.
func (r *AgentRunner) GetPermissionDenials(limit int) []DenialRecord {
	return r.permChecker.GetDenials(limit)
}

// GetSecurityAuditLog returns recent security audit entries.
func (r *AgentRunner) GetSecurityAuditLog(limit int) []DangerousOperation {
	return r.securityEngine.GetAuditLog(limit)
}

// GetSecurityRules returns all security rules.
func (r *AgentRunner) GetSecurityRules() []SecurityRule {
	return r.securityEngine.GetRules()
}

// CheckCommandSecurity checks a command against security rules.
func (r *AgentRunner) CheckCommandSecurity(command string) (level int, riskScore int, rules []SecurityRule) {
	l, score, matchedRules := r.securityEngine.CheckCommand(command)
	return int(l), score, matchedRules
}

// GetSessionState returns the session state for a given session ID.
func (r *AgentRunner) GetSessionState(sessionID string) *SessionState {
	return r.sessionPersist.GetOrCreate(sessionID)
}
