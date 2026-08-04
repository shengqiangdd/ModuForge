package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	defaultMaxIterations = 100
	defaultMaxResultLen  = 32768
	totalTimeout         = 1800 * time.Second // 30 minutes for complex tasks
	maxHistoryChars      = 30000
	summaryMaxLen        = 2000

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
	LLMEndpoint     string    // resolved endpoint (from handler, overrides DB lookup)
	LLMApiKey       string    // resolved API key
	LLMModel        string    // resolved model ID
	MaxOutputTokens int       // max output tokens (0 = use model default)
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
	// Optimization 1: Session access time tracking for TTL-based cleanup
	sessionAccessTimes sync.Map // sessionID -> time.Time (last access timestamp)

	// NEW: Enhanced modules
	auditLog       *AuditLog
	permChecker    *PermissionChecker
	sessionPersist *SessionPersistence
	depGraph       *DependencyGraph
}

func NewAgentRunner(registry *SkillRegistry, apiKey, endpoint, model string, db *sql.DB) *AgentRunner {
	r := &AgentRunner{
		registry:     registry,
		apiKey:       apiKey,
		endpoint:     endpoint,
		model:        model,
		db:           db,
		convStore:    service.NewConversationStore(),
		toolDefCache: make(map[string][]ToolDef),
		// Initialize new modules
		auditLog:       NewAuditLog(""),
		permChecker:    NewPermissionChecker(),
		sessionPersist: NewSessionPersistence(""),
		depGraph:       NewDependencyGraph(),
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
	ID           string
	Description  string
	Status       string   // pending, in_progress, completed, failed
	Dependencies []string // IDs of subtasks that must complete first
}

// DecomposeTask breaks a complex task into subtasks.
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

// VerifyFile checks the quality of a file.
func (qv *QualityVerifier) VerifyFile(filePath string, content string) QualityReport {
	report := QualityReport{
		FilePath: filePath,
		Lines:    strings.Count(content, "\n") + 1,
		Issues:   make([]string, 0),
	}

	// Check for common issues
	lines := strings.Split(content, "\n")

	// 1. Check line length
	longLines := 0
	for _, line := range lines {
		if len(line) > 120 {
			longLines++
		}
	}
	if longLines > 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("%d 行超过120字符", longLines))
	}

	// 2. Check for TODO/FIXME/HACK
	todoCount := 0
	for _, line := range lines {
		lineUpper := strings.ToUpper(strings.TrimSpace(line))
		if strings.Contains(lineUpper, "TODO") || strings.Contains(lineUpper, "FIXME") || strings.Contains(lineUpper, "HACK") {
			todoCount++
		}
	}
	if todoCount > 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("发现 %d 个 TODO/FIXME/HACK 注释", todoCount))
	}

	// 3. Check for very long functions (simple heuristic: count opening braces)
	braceCount := 0
	maxBraceDepth := 0
	for _, line := range lines {
		for _, ch := range line {
			if ch == '{' {
				braceCount++
				if braceCount > maxBraceDepth {
					maxBraceDepth = braceCount
				}
			}
			if ch == '}' {
				braceCount--
			}
		}
	}
	if maxBraceDepth > 5 {
		report.Issues = append(report.Issues, fmt.Sprintf("代码嵌套深度 %d 层，建议重构", maxBraceDepth))
		report.Complexity = maxBraceDepth
	}

	// 4. Check for magic numbers (simple heuristic)
	magicNumbers := 0
	for _, line := range lines {
		// Skip comments
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Count numbers that aren't 0 or 1
		words := strings.Fields(trimmed)
		for _, word := range words {
			if len(word) > 1 && word[0] >= '2' && word[0] <= '9' {
				// Simple heuristic for magic numbers
				magicNumbers++
			}
		}
	}
	if magicNumbers > 5 {
		report.Issues = append(report.Issues, fmt.Sprintf("发现 %d 个可能的魔法数字，建议提取为常量", magicNumbers))
	}

	// 5. Calculate score
	report.Score = 100
	for _, issue := range report.Issues {
		if strings.Contains(issue, "嵌套深度") {
			report.Score -= 20
		} else if strings.Contains(issue, "魔法数字") {
			report.Score -= 10
		} else if strings.Contains(issue, "TODO") || strings.Contains(issue, "FIXME") {
			report.Score -= 8
		} else {
			report.Score -= 5
		}
	}
	if report.Score < 0 {
		report.Score = 0
	}

	return report
}

// GetQualitySummary returns a summary of quality reports.
func (qv *QualityVerifier) GetQualitySummary(reports []QualityReport) string {
	if len(reports) == 0 {
		return "无文件需要检查"
	}

	totalScore := 0
	totalIssues := 0
	for _, r := range reports {
		totalScore += r.Score
		totalIssues += len(r.Issues)
	}
	avgScore := totalScore / len(reports)

	summary := "📊 代码质量报告:\n"
	summary += fmt.Sprintf("- 检查文件: %d\n", len(reports))
	summary += fmt.Sprintf("- 平均质量分: %d/100\n", avgScore)
	summary += fmt.Sprintf("- 发现问题: %d\n", totalIssues)

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

// TryLock attempts to acquire the lock without blocking.
func (fl *FileLock) TryLock(path string) bool {
	val, _ := fl.locks.LoadOrStore(path, &fileLock{})
	l := val.(*fileLock)
	return l.mu.TryLock()
}

// Cleanup removes locks for files that no longer exist.
func (fl *FileLock) Cleanup() {
	fl.locks.Range(func(key, value interface{}) bool {
		path := key.(string)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fl.locks.Delete(path)
		}
		return true
	})
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

// GetRemaining returns remaining budget for a tool type.
func (cb *CallBudget) GetRemaining(toolName string) int {
	switch toolName {
	case "read_file", "list_dir":
		return cb.MaxRead - cb.ReadCalls
	case "write_file", "write_file_batch", "delete_file":
		return cb.MaxWrite - cb.WriteCalls
	default:
		return cb.MaxTotal - cb.TotalCalls
	}
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

// GetRecent returns the last N reflection events.
func (rl *ReflectionLog) GetRecent(n int) []ReflectionEvent {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if n > len(rl.events) {
		n = len(rl.events)
	}
	return rl.events[len(rl.events)-n:]
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
type StagnationDetector struct {
	lastToolCalls         []string // last N tool call signatures (tool:hash)
	lastResults           []string // last N tool results (truncated)
	consecutiveNoWrite    int      // iterations without write_file call
	maxConsecutiveNoWrite int      // threshold to force answer
	maxIdenticalRepeats   int      // max times same tool+args can repeat
	maxStagnationRounds   int      // rounds with no meaningful progress
	stagnationCount       int      // current stagnation counter
}

// toolCallSignature creates a compact signature for a tool call (tool name + args hash).
func toolCallSignature(name string, args map[string]interface{}) string {
	// Simple hash: just use first 100 chars of JSON args
	argStr, _ := json.Marshal(args)
	if len(argStr) > 100 {
		argStr = argStr[:100]
	}
	return name + ":" + string(argStr)
}

// resultSignature creates a compact signature for a tool result.
func resultSignature(result string) string {
	if len(result) > 200 {
		return result[:200]
	}
	return result
}

// RecordToolCall records a tool call and returns true if stagnation detected.
func (sd *StagnationDetector) RecordToolCall(name string, args map[string]interface{}, result string) (stagnant bool, reason string) {
	sig := toolCallSignature(name, args)
	sd.lastToolCalls = append(sd.lastToolCalls, sig)
	if len(sd.lastToolCalls) > 10 {
		sd.lastToolCalls = sd.lastToolCalls[1:]
	}

	resSig := resultSignature(result)
	sd.lastResults = append(sd.lastResults, resSig)
	if len(sd.lastResults) > 10 {
		sd.lastResults = sd.lastResults[1:]
	}

	// Check 1: Same tool+args repeated maxIdenticalRepeats times
	count := 0
	for _, s := range sd.lastToolCalls {
		if s == sig {
			count++
		}
	}
	if count >= sd.maxIdenticalRepeats {
		return true, fmt.Sprintf("工具 '%s' 已重复调用 %d 次（相同参数），建议换一种方式或直接给出答案", name, count)
	}

	// Check 2: Same result repeated maxStagnationRounds times
	if len(sd.lastResults) >= sd.maxStagnationRounds {
		// Check if the last N results are all the same
		allSame := true
		for i := len(sd.lastResults) - sd.maxStagnationRounds + 1; i < len(sd.lastResults); i++ {
			if sd.lastResults[i] != sd.lastResults[i-1] {
				allSame = false
				break
			}
		}
		if allSame && sd.lastResults[len(sd.lastResults)-1] != "" {
			sd.stagnationCount++
			if sd.stagnationCount >= 1 { // detect immediately after maxStagnationRounds identical results
				return true, "连续多轮返回相同结果，Agent 可能陷入循环，建议直接给出当前进度的答案"
			}
		} else {
			sd.stagnationCount = 0
		}
	}

	return false, ""
}

// RecordNoWrite tracks iterations without write_file.
func (sd *StagnationDetector) RecordNoWrite() bool {
	sd.consecutiveNoWrite++
	if sd.consecutiveNoWrite >= sd.maxConsecutiveNoWrite {
		return true // force answer
	}
	return false
}

// ResetNoWrite resets the no-write counter (called when write_file is executed).
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

// GetFallbackModel returns an alternative model when current one fails.
func (trf *ToolRetryFallback) GetFallbackModel(currentModel string) string {
	// Simple fallback chain
	fallbacks := map[string]string{
		"deepseek-v3": "deepseek-v3",
		"deepseek-v4": "deepseek-v3",
		"gpt-4":       "gpt-3.5-turbo",
		"claude-3":    "claude-3-haiku",
	}

	if fallback, ok := fallbacks[currentModel]; ok {
		return fallback
	}
	return currentModel // no fallback available
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

	// Resolve model tier for adaptive limits
	_, _, resolvedModel := r.resolveLLMConfig(userID, reqProviderID, reqModel, cfg)
	modelTier := resolveModelTier(resolvedModel)
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

	// Build tool definitions — filtered by mode and model tier
	toolDefs := r.getToolDefinitions(cfg.Mode, cfg.LLMModel)

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

	// Tracking state
	toolCallHistory := make(map[string]int)
	uniqueOps := make(map[string]bool)
	totalToolCalls := 0
	writeFileCalled := false
	writeFileCount := 0
	readFileCount := 0
	// P1-2: Dynamic limits based on project complexity
	baseMaxReadFilePerTurn := 10
	baseMaxWriteFilePerTurn := 15
	maxWriteFilePerTurn := baseMaxWriteFilePerTurn
	maxReadFilePerTurn := baseMaxReadFilePerTurn
	checkpoints := make([]FileCheckpoint, 0) // file change history for undo
	consecutiveErrors := 0
	answerSent := false
	var lastLLMResp *LLMResponse
	startTime := time.Now() // NEW: Track total execution time

	// Optimization 1: Session-scoped tool result cache (persists across Run() calls)
	toolCache := r.getSessionCache(sessionID)

	// Optimization 24: Self-reflection tracking — detect repeated tool failures
	toolConsecutiveErrors := make(map[string]int)    // skill name -> consecutive error count
	toolLastResults := make(map[string]string)       // skill name -> last result (for pattern detection)
	toolConsecutiveIdentical := make(map[string]int) // skill name -> consecutive identical calls

	// P0-1: Smart loop termination — detect stagnation
	stagnationDetector := &StagnationDetector{
		lastToolCalls:         make([]string, 0, 10),
		lastResults:           make([]string, 0, 10),
		consecutiveNoWrite:    0,
		maxConsecutiveNoWrite: 15, // force answer after 15 iterations without write_file
		maxIdenticalRepeats:   3,  // stop if same tool+args repeated 3 times
		maxStagnationRounds:   5,  // stop if no progress for 5 rounds
	}

	// P0-2: Tool retry fallback
	toolRetryFallback := &ToolRetryFallback{
		db:           r.db,
		currentModel: resolvedModel,
	}

	// P2-1: Task decomposer for complex tasks
	taskDecomposer := &TaskDecomposer{db: r.db}
	subtasks := taskDecomposer.DecomposeTask(task, cfg.ProjectContext)
	currentSubtask := taskDecomposer.GetNextSubtask(subtasks)
	if currentSubtask != nil {
		log.Printf("[Agent] task decomposed into %d subtasks, starting: %s", len(subtasks), currentSubtask.Description)
	}

	// P2-2: Quality verifier
	qualityVerifier := &QualityVerifier{db: r.db}
	qualityReports := make([]QualityReport, 0)

	// P1-2: File lock for race condition prevention
	fileLock := &FileLock{}

	// P1-3: Global call budget
	callBudget := NewCallBudget()

	// P2-1: Self-reflection log
	reflectionLog := NewReflectionLog()

	// Derive skill sets from metadata (no hardcoded maps)
	readOnlySkills := r.registry.ReadOnlySkills()

	for iter := 0; iter < cfg.MaxIterations; iter++ {
		writeFileCount = 0 // reset per-iteration counter
		readFileCount = 0

		// P1-2: Dynamically adjust limits based on project complexity
		if cfg.ProjectID != "" && iter > 0 {
			// Increase limits for complex projects (more files to read/write)
			if totalToolCalls > 20 {
				maxReadFilePerTurn = baseMaxReadFilePerTurn + 5
				maxWriteFilePerTurn = baseMaxWriteFilePerTurn + 5
			}
			if totalToolCalls > 50 {
				maxReadFilePerTurn = baseMaxReadFilePerTurn + 10
				maxWriteFilePerTurn = baseMaxWriteFilePerTurn + 10
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if w.IsDisconnected() {
			log.Printf("[Agent] client disconnected at iteration %d", iter+1)
			r.writeContentCache.Delete(sessionID)
			return fmt.Errorf("client disconnected")
		}

		// Auto-compact if conversation is getting too long
		// Optimization 15: Cache size calculation to avoid double computation
		convSize := r.estimateConversationSize(conversation)
		if convSize > compactionThreshold {
			debugLog("auto-compacting conversation (size=%d)", convSize)
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

		// Progress event — show iteration progress with percentage
		progressPct := float64(iter+1) / float64(cfg.MaxIterations) * 100
		w.WriteSSE(map[string]interface{}{
			"type":    "step",
			"step":    "think",
			"content": fmt.Sprintf("思考中 (第 %d/%d 轮, %.0f%%)...", iter+1, cfg.MaxIterations, progressPct),
		})

		// Call LLM with keepalive — send empty think events every 10s to prevent frontend idle timeout
		llmDone := make(chan struct{})
		startKeepalive(ctx, w, llmDone, 10*time.Second)

		// Optimization 2: Prefilter conversation to remove waste
		prefiltered := prefilterConversation(conversation)

		llmResp, err := r.callLLMWithTools(ctx, prefiltered, toolDefs, w, cfg.UserID, reqProviderID, reqModel, cfg)
		close(llmDone)
		lastLLMResp = llmResp
		if err != nil {
			var abortErr error
			conversation, consecutiveErrors, abortErr = r.handleLLMCallError(ctx, w, cfg, conversation, consecutiveErrors, err)
			if abortErr != nil {
				return abortErr
			}
			continue
		}
		consecutiveErrors = 0

		debugLog("iter=%d mode=%s role=%s contentLen=%d toolCalls=%d",
			iter+1, cfg.Mode, llmResp.Role, len(llmResp.Content), len(llmResp.ToolCalls))

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
				currentSubtask = taskDecomposer.GetNextSubtask(subtasks)
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
					"Your previous answer was garbled/unreadable. Please provide a clear, well-formatted Markdown answer. Do NOT use tools.")
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
				// Check if answer claims modification without calling write_file
				if claimsFileModification(answer) && !writeFileCalled && iter < cfg.MaxIterations-1 {
					log.Printf("[Agent] answer claims modification but write_file not called")
					conversation = appendRoleMessage(conversation, "assistant", answer)
					conversation = appendRoleMessage(conversation, "user",
						"你提到修改了文件但没有调用 write_file。请调用 write_file 保存更改，或者直接回答。")
					continue
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

		// Optimization 1: Parallel tool execution for read-only tools
		// Separate tools into parallel-safe (read-only) and sequential (write/side-effect)
		type toolTask struct {
			tc         LLMToolCall
			skillName  string
			skillInput map[string]interface{}
			parallel   bool
		}
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
		originalTasks := tasks
		seen := make(map[string]int) // dedupKey -> index in deduped slice
		var deduped []toolTask
		var skippedToolCalls []LLMToolCall
		for _, t := range tasks {
			dedupKey := t.skillName + ":" + t.tc.Function.Arguments
			if idx, exists := seen[dedupKey]; exists {
				debugLog("dedup: skipping duplicate tool call %s (same as task %d)", t.skillName, idx)
				skippedToolCalls = append(skippedToolCalls, t.tc)
				continue
			}
			seen[dedupKey] = len(deduped)
			deduped = append(deduped, t)
		}
		if len(deduped) < len(originalTasks) {
			log.Printf("[Agent] dedup: removed %d/%d duplicate tool calls", len(originalTasks)-len(deduped), len(originalTasks))
		}
		tasks = deduped

		// Execute tools: parallel for read-only AND different-file writes, sequential for same-file writes
		var mu sync.Mutex
		var results []struct {
			tc     LLMToolCall
			result string
		}

		// NEW: Analyze dependencies for better parallelism
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

		// Execute parallel tasks concurrently
		if len(parallelTasks) > 0 {
			var wg sync.WaitGroup
			for _, pt := range parallelTasks {
				wg.Add(1)
				go func(task toolTask) {
					defer wg.Done()
					incGoroutines()
					defer decGoroutines()
					checkGoroutineLeak()
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
						results = append(results, struct {
							tc     LLMToolCall
							result string
						}{tc: task.tc, result: cached})
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
								results = append(results, struct {
									tc     LLMToolCall
									result string
								}{tc: task.tc, result: wc})
								mu.Unlock()
								w.WriteSSE(map[string]interface{}{
									"type":    "step",
									"step":    "skill_result",
									"skill":   task.skillName,
									"content": wc,
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
					}

					// Truncate large results
					if len(result) > cfg.MaxResultLen {
						if modelTier == TierFree {
							result = smartSummarizeResult(result, task.skillName, cfg.MaxResultLen)
						} else {
							result = summarizeResult(result, cfg.MaxResultLen)
						}
					}

					mu.Lock()
					results = append(results, struct {
						tc     LLMToolCall
						result string
					}{tc: task.tc, result: result})
					// Track parallel tool calls for loop detection
					toolCallHistory[task.skillName]++
					totalToolCalls++
					opKey := task.skillName
					if path, ok := task.skillInput["path"].(string); ok {
						opKey = task.skillName + ":" + path
					}
					uniqueOps[opKey] = true
					mu.Unlock()

					w.WriteSSE(map[string]interface{}{
						"type":    "step",
						"step":    "skill_result",
						"skill":   task.skillName,
						"content": result,
					})
				}(pt)
			}
			wg.Wait()
		}

		// Execute sequential tasks (write/side-effect tools)
		anyWriteCalled := false
		for _, st := range sequentialTasks {
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

			// Keepalive during execution
			skillDone := make(chan struct{})
			startKeepalive(ctx, w, skillDone, 10*time.Second)

			// Execute with timeout
			toolTimeout := toolTimeoutForName(st.skillName)
			toolCtx, toolCancel := context.WithTimeout(ctx, toolTimeout)

			// P1-2: Acquire file lock for write operations
			if st.skillName == "write_file" || st.skillName == "write_file_batch" {
				if path, ok := st.skillInput["path"].(string); ok {
					fileLock.Lock(path)
					defer fileLock.Unlock(path)
				}
			}

			result, err := r.executeSkill(toolCtx, st.skillName, st.skillInput)
			toolCancel()
			close(skillDone)

			if toolCtx.Err() == context.DeadlineExceeded {
				result = fmt.Sprintf("⚠️ Tool execution timed out after %v", toolTimeout)
			} else if err != nil && st.skillName == "write_file" {
				log.Printf("[Agent] write_file failed: %v, providing error context to LLM", err)
				result = fmt.Sprintf("Write failed: %v. Please check the path and content, then try again.", err)
			} else if err != nil {
				// P0-2: Classify error and determine recovery strategy
				errCategory := ClassifyError(err.Error())
				toolConsecutiveErrors[st.skillName]++
				recovery := GetRecoveryStrategy(errCategory, toolConsecutiveErrors[st.skillName])
				recoveryMsg := GetRecoveryMessage(recovery, st.skillName)
				log.Printf("[Agent] tool %s failed (attempt %d): %v, category=%d, recovery=%d",
					st.skillName, toolConsecutiveErrors[st.skillName], err, errCategory, recovery)

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
						toolConsecutiveErrors[st.skillName] = 0
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
					return err
				default:
					result = fmt.Sprintf("Error: %v", err)
				}
			} else {
				toolConsecutiveErrors[st.skillName] = 0 // reset on success
			}

			if st.skillName == "write_file" {
				writeFileCalled = true
				anyWriteCalled = true
				stagnationDetector.ResetNoWrite()
				if path, ok := st.skillInput["path"].(string); ok {
					toolCache.invalidate(path)
					// Optimization 17: Cache written content for immediate read-back
					if content, ok := st.skillInput["content"].(string); ok && err == nil {
						r.cacheWriteContent(sessionID, path, content)
						// P2-2: Quality verification after write
						report := qualityVerifier.VerifyFile(path, content)
						qualityReports = append(qualityReports, report)
						if len(report.Issues) > 0 {
							log.Printf("[Agent] quality issues in %s: %v", path, report.Issues)
						}
						// P1-1: Auto-rollback if quality score is too low
						if report.Score < 40 && len(checkpoints) > 0 {
							// Find the last checkpoint for this file
							for i := len(checkpoints) - 1; i >= 0; i-- {
								if checkpoints[i].Path == path && checkpoints[i].Content != "" {
									log.Printf("[Agent] quality score %d < 40, rolling back %s", report.Score, path)
									// Write back the checkpoint content
									rollbackResult, rollbackErr := r.executeSkill(ctx, "write_file", map[string]interface{}{
										"path":    path,
										"content": checkpoints[i].Content,
									})
									if rollbackErr == nil && rollbackResult != "" {
										result = fmt.Sprintf("⚠️ 代码质量过低 (score=%d)，已自动回滚到之前的版本\n\n质量问题:\n%s",
											report.Score, strings.Join(report.Issues, "\n"))
										w.WriteSSE(map[string]interface{}{
											"type":    "step",
											"step":    "think",
											"content": fmt.Sprintf("⚠️ 代码质量过低 (score=%d)，已自动回滚", report.Score),
										})
									} else {
										log.Printf("[Agent] rollback failed: %v", rollbackErr)
									}
									break
								}
							}
						}
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
			toolCallHistory[st.skillName]++
			totalToolCalls++
			opKey := st.skillName
			if st.skillName == "read_file" || st.skillName == "write_file" {
				if path, ok := st.skillInput["path"].(string); ok {
					opKey = st.skillName + ":" + path
				}
			}
			uniqueOps[opKey] = true

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
			if len(result) > cfg.MaxResultLen {
				if modelTier == TierFree {
					result = smartSummarizeResult(result, st.skillName, cfg.MaxResultLen)
				} else {
					result = summarizeResult(result, cfg.MaxResultLen)
				}
			}

			w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "skill_result",
				"skill":   st.skillName,
				"content": result,
			})

			mu.Lock()
			results = append(results, struct {
				tc     LLMToolCall
				result string
			}{tc: st.tc, result: result})
			mu.Unlock()
		}

		// Optimization 6: Map deduplicated tool calls back to their executed counterparts.
		// For each skipped duplicate, find the matching executed result and reuse it.
		if len(skippedToolCalls) > 0 {
			for _, origTC := range skippedToolCalls {
				dedupKey := origTC.Function.Name + ":" + origTC.Function.Arguments
				if executedIdx, ok := seen[dedupKey]; ok {
					for _, res := range results {
						if res.tc.Function.Name == deduped[executedIdx].skillName &&
							res.tc.Function.Arguments == deduped[executedIdx].tc.Function.Arguments {
							mu.Lock()
							results = append(results, struct {
								tc     LLMToolCall
								result string
							}{tc: origTC, result: res.result})
							mu.Unlock()
							debugLog("dedup: mapped skipped %s (id=%s) to executed result (id=%s)",
								origTC.Function.Name, origTC.ID, res.tc.ID)
							break
						}
					}
				}
			}
		}

		// P0-1: Track stagnation for each tool call
		for _, res := range results {
			// Check stagnation
			var toolInput map[string]interface{}
			if err := json.Unmarshal([]byte(res.tc.Function.Arguments), &toolInput); err != nil {
				debugLog("tool args unmarshal failed for stagnation check (%s): %v", res.tc.Function.Name, err)
			}
			if stagnant, reason := stagnationDetector.RecordToolCall(res.tc.Function.Name, toolInput, res.result); stagnant {
				log.Printf("[Agent] stagnation detected: %s", reason)
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "think",
					"content": fmt.Sprintf("⚠️ %s", reason),
				})
				// Force answer by appending a user message asking for summary
				conversation = appendRoleMessage(conversation, "user",
					fmt.Sprintf("[System: %s. Please provide a summary of what you've done so far and any remaining work.]", reason))
				answerSent = true
				break
			}

			// Track write_file calls
			if res.tc.Function.Name == "write_file" {
				anyWriteCalled = true
				stagnationDetector.ResetNoWrite()
			}
		}

		// Check no-write stagnation
		if !anyWriteCalled && !answerSent {
			if stagnationDetector.RecordNoWrite() {
				log.Printf("[Agent] no-write stagnation: %d iterations without write_file", stagnationDetector.consecutiveNoWrite)
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "think",
					"content": "⚠️ 已连续多轮未执行写入操作，请直接给出当前进度的答案或执行必要的文件修改",
				})
				conversation = appendRoleMessage(conversation, "user",
					"[System: You have not written any files for multiple iterations. Please either write the necessary files or provide your final answer based on what you've read so far.]")
				answerSent = true
			}
		}

		if answerSent {
			continue
		}

		// Add all results to conversation in order
		// Optimization 18: Merge consecutive write_file results to save tokens
		var pendingWriteFiles []struct {
			tc     LLMToolCall
			result string
			path   string
		}
		flushWriteFiles := func() {
			if len(pendingWriteFiles) == 0 {
				return
			}
			if len(pendingWriteFiles) == 1 {
				// Single write_file — add as-is
				wf := pendingWriteFiles[0]
				conversation = r.appendToolResult(conversation, sessionID, wf.tc.ID, wf.result)
			} else {
				// Multiple consecutive write_files — merge into summary
				var paths []string
				for _, wf := range pendingWriteFiles {
					paths = append(paths, wf.path)
				}
				merged := fmt.Sprintf("✅ Successfully wrote %d files: %s",
					len(pendingWriteFiles), strings.Join(paths, ", "))
				// Add one merged result for the first tool_call_id, skip the rest
				conversation = r.appendToolResult(conversation, sessionID, pendingWriteFiles[0].tc.ID, merged)
				// For the remaining write_file calls, add empty acknowledges
				for _, wf := range pendingWriteFiles[1:] {
					conversation = r.appendToolResult(conversation, sessionID, wf.tc.ID, "(merged into previous result)")
				}
				debugLog("merged %d write_file results into one message (saved ~%d tokens)",
					len(pendingWriteFiles), len(pendingWriteFiles)*50)
			}
			pendingWriteFiles = nil
		}

		for _, res := range results {
			isWrite := res.tc.Function.Name == "write_file"
			if isWrite {
				// Extract path from the result or tool call
				path := ""
				if idx := strings.Index(res.result, "[project_id:"); idx >= 0 {
					// Result contains project_id prefix, path is in the tool call
					path = res.tc.Function.Arguments
				}
				// Try to get path from arguments
				var args map[string]interface{}
				if json.Unmarshal([]byte(res.tc.Function.Arguments), &args) == nil {
					if p, ok := args["path"].(string); ok {
						path = p
					}
				}
				if path == "" {
					path = fmt.Sprintf("file_%d", len(pendingWriteFiles))
				}
				pendingWriteFiles = append(pendingWriteFiles, struct {
					tc     LLMToolCall
					result string
					path   string
				}{tc: res.tc, result: res.result, path: path})
				continue
			}
			// Non-write tool: flush pending writes first
			flushWriteFiles()
			conversation = r.appendToolResult(conversation, sessionID, res.tc.ID, res.result)

			debugLog("tool=%s resultLen=%d uniqueOps=%d totalCalls=%d",
				res.tc.Function.Name, len(res.result), len(uniqueOps), totalToolCalls)
		}
		// Flush any remaining write_file results
		flushWriteFiles()

		// Optimization 24: Self-reflection — track failures and inject diagnostic prompts
		// Update error tracking for each tool result
		for _, res := range results {
			skillName := res.tc.Function.Name
			isError := strings.HasPrefix(res.result, "Error:") || strings.HasPrefix(res.result, "❌") ||
				strings.HasPrefix(res.result, "⚠️") || strings.Contains(res.result, "failed")

			if isError {
				toolConsecutiveErrors[skillName]++
				if toolConsecutiveErrors[skillName] >= 3 {
					// Same tool failed 3 times in a row — inject reflection prompt
					diagnostic := fmt.Sprintf(
						"⚠️ [Self-Reflection] The tool '%s' has failed %d times consecutively. "+
							"Recent errors: %s. "+
							"STOP using this tool with the same approach. Instead: "+
							"(1) Analyze WHY it's failing, (2) Try a completely different approach, "+
							"(3) If stuck, use write_file to create the file directly.",
						skillName, toolConsecutiveErrors[skillName], res.result)
					log.Printf("[Agent] self-reflection triggered: %s failed %d times", skillName, toolConsecutiveErrors[skillName])
					conversation = appendRoleMessage(conversation, "system", diagnostic)
					w.WriteSSE(map[string]interface{}{
						"type":    "step",
						"step":    "think",
						"content": fmt.Sprintf("🔄 检测到 %s 连续失败 %d 次，已注入反思提示", skillName, toolConsecutiveErrors[skillName]),
					})
					toolConsecutiveErrors[skillName] = 0 // reset after injection
				}
			} else {
				toolConsecutiveErrors[skillName] = 0 // success resets counter
			}

			// Track consecutive identical tool calls (same skill + same input)
			inputKey := skillName
			if len(res.tc.Function.Arguments) > 0 {
				p := res.tc.Function.Arguments
				if len(p) > 100 {
					p = p[:100]
				}
				inputKey = skillName + ":" + p
			}
			if prev, ok := toolLastResults[inputKey]; ok && prev == res.result {
				toolConsecutiveIdentical[inputKey]++
				if toolConsecutiveIdentical[inputKey] >= 3 {
					// Same exact tool call with same result 3 times — force answer
					log.Printf("[Agent] early termination: '%s' called %d times with identical result", skillName, toolConsecutiveIdentical[inputKey])
					return r.forceAnswer(ctx, conversation, w, sessionID, cfg, reqProviderID, reqModel,
						fmt.Sprintf("You've called '%s' with identical parameters %d times and got the same result each time. This is a dead end. You must stop using tools and provide your final answer.", skillName, toolConsecutiveIdentical[inputKey]))
				}
			} else {
				toolConsecutiveIdentical[inputKey] = 0
			}
			toolLastResults[inputKey] = res.result
		}

		// Read-only loop detection: if Agent only calls read_file/grep/glob without any write/edit, inject reminder
		readOnlyCount := toolCallHistory["read_file"] + toolCallHistory["grep_search"] + toolCallHistory["glob_search"] + toolCallHistory["list_dir"]
		writeCount := toolCallHistory["write_file"] + toolCallHistory["edit_file"] + toolCallHistory["write_file_batch"]
		skipLoopDetection := false
		if readOnlyCount >= 6 && writeCount == 0 && iter >= 2 {
			diagnostic := fmt.Sprintf(
				"⚠️ [Read-Only Loop] You have called read tools %d times without any write/edit operations. "+
					"You have already read enough code. NOW you MUST: "+
					"(1) Use edit_file for targeted fixes, or write_file to rewrite files completely. "+
					"(2) Then call build_module to verify. "+
					"DO NOT read any more files. Start writing code immediately.",
				readOnlyCount)
			log.Printf("[Agent] read-only loop detected: %d reads, %d writes", readOnlyCount, writeCount)
			conversation = appendRoleMessage(conversation, "system", diagnostic)
			w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "think",
				"content": fmt.Sprintf("🔄 检测到只读循环（%d 次读取，0 次写入），已注入编辑提醒", readOnlyCount),
			})
			skipLoopDetection = true // Give Agent a chance to respond before forcing answer
		}

		// Global error cap: if total consecutive errors across all skills >= 5, force answer
		totalConsecutiveErrors := 0
		for _, count := range toolConsecutiveErrors {
			totalConsecutiveErrors += count
		}
		if totalConsecutiveErrors >= 5 {
			log.Printf("[Agent] early termination: total consecutive errors across all skills = %d", totalConsecutiveErrors)
			return r.forceAnswer(ctx, conversation, w, sessionID, cfg, reqProviderID, reqModel,
				"Multiple tools have failed consecutively. Stop using tools and provide your final answer based on what you've learned so far.")
		}

		// Smart loop detection (skip if read-only reminder was just injected)
		if !skipLoopDetection {
			if reason := detectLoop(toolCallHistory, uniqueOps, totalToolCalls); reason != "" {
				debugLog("loop detected: %s", reason)
				return r.forceAnswer(ctx, conversation, w, sessionID, cfg, reqProviderID, reqModel, reason)
			}
		}
	}

	// Exhausted iterations — send answer if we haven't already
	sendFinalAnswer(w, cfg, lastLLMResp, answerSent)
	// Clean up write-content cache for this session (tool result cache persists)
	r.writeContentCache.Delete(sessionID)
	return nil
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

// CleanupSessions removes sessions older than maxAge.
func (r *AgentRunner) CleanupSessions(maxAge time.Duration) int {
	return r.sessionPersist.Cleanup(maxAge)
}
