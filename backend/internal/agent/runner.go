package agent

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/moduforge/backend/internal/service"
)

// agentDebug enables verbose logging for hot-path operations.
// Set MODUFORGE_DEBUG=1 to enable.
var agentDebug = os.Getenv("MODUFORGE_DEBUG") == "1"

// Optimization 42: Goroutine leak detector
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
	toolTimeoutFast  = 30 * time.Second  // read-only tools
	toolTimeoutWrite = 60 * time.Second  // write_file, edit_file, etc.
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
		return 120 * time.Second
	case "build_module", "test_module":
		return toolTimeoutSlow
	default:
		return toolTimeoutWrite
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
// ═══════════════════════════════════════════════════════════════════

type AgentMode string

const (
	ModePlan AgentMode = "plan"
	ModeAct  AgentMode = "act"
)

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
	Mode            AgentMode // "plan" or "act"
	ProviderID      string
	LLMEndpoint     string
	LLMApiKey       string
	LLMModel        string
	MaxOutputTokens int

	// P0-Optimization: Cached resolved LLM config
	resolvedEndpoint string
	resolvedAPIKey   string
	resolvedModel    string
	modelTier        ModelTier

	// Model fallback: free → paid auto-switch on repeated failures
	fallbackEndpoint string
	fallbackAPIKey   string
	fallbackModel    string
	fallbackTier     ModelTier
	fallbackActive   bool // true once we've switched to fallback
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

	monthlyCostLimit float64

	// Optimization 13: Tool definition cache
	toolDefCache   map[string][]ToolDef
	toolDefCacheMu sync.RWMutex

	// Optimization 16: Session-scoped tool result cache
	sessionCaches      sync.Map
	writeContentCache  sync.Map
	readFileCache      sync.Map
	sessionAccessTimes sync.Map

	// Enhanced modules
	auditLog       *AuditLog
	permChecker    *PermissionChecker
	securityEngine *SecurityEngine
	sessionPersist *SessionPersistence
	depGraph       *DependencyGraph

	// High-value optimization modules
	buildHealer    *BuildHealer
	atomicWriter   *AtomicWriter
	enhancedPlan   *EnhancedPlanner
	fileDepGraph   *FileDependencyGraph

	perfMetrics *PerformanceMetrics

	lastUsageSnapshot usageSnapshot

	pendingApprovals sync.Map

	fileHashCache *fileHashCache
	repoMap       *RepoMap

	progressTrackers sync.Map

	// Context caching improvements
	prefixCache     *PrefixCache
	semanticCache   *SemanticCache
	contextCondenser *ContextCondenser
	sessionLearner  *SessionLearner

	// DifferentialCache for file content change detection (cleaned up periodically)
	diffCache *DifferentialCache

	// TokenOptimizer for token-aware conversation pruning and tool result caching
	tokenOptimizer *TokenOptimizer
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
		auditLog:       NewAuditLog(""),
		permChecker:    NewPermissionChecker(),
		securityEngine: NewSecurityEngine(),
		sessionPersist: NewSessionPersistence(""),
		depGraph:       NewDependencyGraph(),
		perfMetrics:    NewPerformanceMetrics(),
		buildHealer:    NewBuildHealer(),
		atomicWriter:   NewAtomicWriter(""),
		enhancedPlan:   nil,
		fileDepGraph:   nil,
		prefixCache:      NewPrefixCache(100, 5*time.Minute),
		semanticCache:    NewSemanticCache(500, 0.85),
		contextCondenser: NewContextCondenser(30, 6, 1),
		sessionLearner:   NewSessionLearner(100),
		diffCache:        NewDifferentialCache(2 * time.Minute),
		tokenOptimizer:   NewTokenOptimizer(""),
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

func (r *AgentRunner) SetMemoryV2Store(store *service.MemoryV2Store) {
	r.memV2Store = store
}

func (r *AgentRunner) SetFileHashCache(cache *fileHashCache) {
	r.fileHashCache = cache
}

// ═══════════════════════════════════════════════════════════════════
// Core Agent Loop — modern architecture
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

	// P0-Optimization: Resolve LLM config ONCE at Run() entry
	resolvedEndpoint, resolvedAPIKey, resolvedModel := r.resolveLLMConfig(userID, reqProviderID, reqModel, cfg)
	cfg.resolvedEndpoint = resolvedEndpoint
	cfg.resolvedAPIKey = resolvedAPIKey
	cfg.resolvedModel = resolvedModel
	if reqProviderID != "" {
		cfg.ProviderID = reqProviderID
	}
	// Force-load from custom_providers table if key is empty
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
	cfg.modelTier = resolveModelTierWithMaxTokens(resolvedModel, cfg.MaxOutputTokens)
	modelTier := cfg.modelTier

	// Resolve fallback model (free → paid) for auto-switch on repeated failures
	if modelTier == TierFree {
		if fbEp, fbKey, fbModel, found := r.resolveFallbackConfig(userID, resolvedEndpoint); found {
			cfg.fallbackEndpoint = fbEp
			cfg.fallbackAPIKey = fbKey
			cfg.fallbackModel = fbModel
			cfg.fallbackTier = resolveModelTierWithMaxTokens(fbModel, 0)
			log.Printf("[Agent] fallback configured: primary=%s → fallback=%s (tier=%d)", resolvedModel, fbModel, cfg.fallbackTier)
		}
	}

	compactionThreshold := compactionThresholdForTier(modelTier, cfg.MaxOutputTokens)
	if cfg.MaxResultLen == defaultMaxResultLen {
		cfg.MaxResultLen = maxResultLenForTier(modelTier)
	}
	debugLog("model=%s tier=%d compactionThreshold=%d maxResultLen=%d", resolvedModel, modelTier, compactionThreshold, cfg.MaxResultLen)

	if sessionID != "" && len(messages) > 0 {
		r.convStore.Add(sessionID, messages)
	}

	// Build system prompt based on mode
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
		systemPrompt += `
## NO PROJECT SELECTED
You are running WITHOUT a project context. This means:
- You CANNOT read or write project files (no project_id available)
- You CAN answer questions, provide advice, write code snippets, and explain concepts
- If the user asks you to create/modify files, explain that they need to select a project first, OR offer to create a new project by writing files directly (write_file auto-creates projects)
- For general coding questions, answer directly without tools
- Keep your answers focused and practical`
	}

	// Auto-recall relevant past memories
	if len(task) > 0 {
		if recalled := r.autoRecallMemory(cfg, task, 3); recalled != "" {
			systemPrompt += "\n" + recalled
		}
	}

	// Build tool definitions
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
				if len(msg.ToolCalls) > 0 {
					convMsg["tool_calls"] = msg.ToolCalls
				}
				if msg.ToolCallID != "" {
					convMsg["tool_call_id"] = msg.ToolCallID
				}
				conversation = append(conversation, convMsg)
			}
		}
	}

	// Tracking state
	m := &runMetrics{
		toolCallHistory:          make(map[string]int),
		uniqueOps:                make(map[string]bool),
		toolConsecutiveErrors:    make(map[string]int),
		toolLastResults:          make(map[string]string),
		toolConsecutiveIdentical: make(map[string]int),
		uniqueTargetsPerSkill:    make(map[string]int),
	}
	writeFileCalled := false
	anyWriteCalled := false
	buildModuleCalled := false
	baseMaxReadFilePerTurn := 25
	baseMaxWriteFilePerTurn := 20
	maxWriteFilePerTurn := baseMaxWriteFilePerTurn
	maxReadFilePerTurn := baseMaxReadFilePerTurn
	checkpoints := make([]FileCheckpoint, 0)
	consecutiveErrors := 0
	answerSent := false
	var lastLLMResp *LLMResponse
	startTime := time.Now()
	_ = checkpoints // reserved for undo support

	runPerfMetrics := NewPerformanceMetrics()
	toolCache := r.getSessionCache(sessionID)
	stagnationDetector := newStagnationDetector()
	progressTracker := r.getOrCreateProgressTracker(sessionID)

	trp := &toolResultProcessor{
		r: r, ctx: ctx, w: w, cfg: cfg, sessionID: sessionID,
		reqProviderID: reqProviderID, reqModel: reqModel,
		stagnationDetector: stagnationDetector, m: m,
		progressTracker: progressTracker,
	}

	toolRetryFallback := &ToolRetryFallback{
		db:           r.db,
		currentModel: resolvedModel,
	}

	// P2-1: Enhanced Task planner with file-level granularity
	var enhancedPlanner *EnhancedPlanner
	var enhancedPlan *EnhancedPlan
	var planInjected bool

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

	qualityVerifier := &QualityVerifier{db: r.db}
	qualityReports := make([]QualityReport, 0)
	qualitySem := make(chan struct{}, 3)
	fileLock := &FileLock{}
	callBudget := NewCallBudget()
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
			var repoMapWg sync.WaitGroup
			repoMapErrCh := make(chan error, 1)
			repoMapWg.Add(1)
			go func() {
				defer repoMapWg.Done()
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[Agent] repo-map goroutine panicked: %v", r)
						repoMapErrCh <- fmt.Errorf("repo-map panic: %v", r)
					}
				}()
				if result := rm.GenerateRepoMapWithTimeout(ctx, projectPath, 10*time.Second); result != "" {
					log.Printf("[Agent] repo-map generation result: %s", result)
					repoMapErrCh <- fmt.Errorf("%s", result)
					return
				}
				log.Printf("[Agent] repo-map generated: %d files indexed", len(rm.fileIndex))
			}()
			// Wait for repo-map in background and close channel when done
			go func() {
				repoMapWg.Wait()
				close(repoMapErrCh)
			}()
		}
	}

	// Derive skill sets from metadata
	readOnlySkills := r.registry.ReadOnlySkills()

	var iterCancel context.CancelFunc
	var iterCtx context.Context

	for iter := 0; iter < cfg.MaxIterations; iter++ {
		r.perfMetrics.RecordIteration()
		// Dynamically adjust limits based on project complexity
		if cfg.ProjectID != "" && iter > 0 {
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

		// Progress event (every 3 iterations)
		if iter%3 == 0 || iter == cfg.MaxIterations-1 {
			progressPct := float64(iter+1) / float64(cfg.MaxIterations) * 100
			w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "think",
				"content": fmt.Sprintf("思考中 (第 %d/%d 轮, %.0f%%)...", iter+1, cfg.MaxIterations, progressPct),
			})
		}

		// Inject plan context at start
		if enhancedPlan != nil && !planInjected {
			stepContext := enhancedPlanner.BuildContextMessage(enhancedPlan)
			if stepContext != "" {
				conversation = appendRoleMessage(conversation, "system", stepContext)
				planInjected = true
				log.Printf("[Agent] injected enhanced plan context into conversation")
			}
		}

		iterCtx, iterCancel = context.WithTimeout(ctx, perIterationTimeout)

		// Call LLM with keepalive
		llmDone := make(chan struct{})
		startKeepalive(iterCtx, w, llmDone, 10*time.Second)

		prefiltered := prefilterConversation(conversation)

		// Token optimization: prune old tool results to reduce token usage
		if r.tokenOptimizer != nil && len(prefiltered) > 0 {
			maxConvTokens := 100000
			prefiltered = r.tokenOptimizer.OptimizeConversation(prefiltered, maxConvTokens)
		}
		llmStartTime := time.Now()
		llmResp, err := r.callLLMWithTools(iterCtx, prefiltered, toolDefs, w, cfg.UserID, reqProviderID, reqModel, cfg)
		llmDuration := time.Since(llmStartTime)
		runPerfMetrics.RecordLLMCall(llmDuration)
		close(llmDone)
		lastLLMResp = llmResp
		if err != nil {
			runPerfMetrics.RecordError()
			var abortErr error
			conversation, consecutiveErrors, abortErr = r.handleLLMCallError(ctx, w, &cfg, conversation, consecutiveErrors, err)
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
			conversation, answerSent, writeFileCalled, _ = r.handleFinalAnswer(
				ctx, llmResp, conversation, sessionID, w, cfg, iter,
				qualityReports, qualityVerifier, reflectionLog,
				enhancedPlanner, enhancedPlan,
				writeFileCalled, anyWriteCalled, answerSent,
			)
			if answerSent {
				iterCancel()
				return nil
			}
			continue
		}

		// ── Case 2: Tool calls → execute ──
		assistantMsg := map[string]interface{}{
			"role":       "assistant",
			"content":    llmResp.Content,
			"tool_calls": llmResp.ToolCalls,
		}
		conversation = append(conversation, assistantMsg)

		// Persist assistant message with tool_calls to convStore
		if sessionID != "" {
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

		// Prepare tool tasks: validate, dedup, analyze dependencies, group
		plan := r.prepareToolTasks(llmResp, conversation, sessionID, w, cfg, readOnlySkills, maxReadFilePerTurn, maxWriteFilePerTurn)
		conversation = plan.conversation

		// Execute parallel tools
		var mu sync.Mutex
		var results []toolResult
		r.executeParallelToolBlock(ctx, plan.parallelTasks, w, toolCache, sessionID, &mu, &results, m, modelTier, cfg)

		// Execute sequential tools
		editFileConsecutiveFailures := 0
		seqState := &seqToolExecState{
			r: r, ctx: ctx, w: w, cfg: cfg, sessionID: sessionID,
			reqProviderID: reqProviderID, reqModel: reqModel,
			m: m, modelTier: modelTier, toolCache: toolCache,
			callBudget: callBudget, fileLock: fileLock,
			stagnationDetector: stagnationDetector, reflectionLog: reflectionLog,
			enhancedPlanner: enhancedPlanner, enhancedPlan: enhancedPlan,
			qualityVerifier: qualityVerifier, qualityReports: &qualityReports,
			qualitySem: qualitySem, mu: &mu, results: &results,
			startTime: startTime, toolRetryFallback: toolRetryFallback,
			writeFileCalled: &writeFileCalled, anyWriteCalled: &anyWriteCalled,
			editFileConsecutiveFailures: &editFileConsecutiveFailures,
			answerSent: &answerSent, conversation: &conversation,
		}
		seqState.executeSequentialToolBlock(plan.sequentialTasks)

		// Optimization 6: Map deduplicated tool calls back to executed counterparts
		if len(plan.skippedToolCalls) > 0 {
			for _, origTC := range plan.skippedToolCalls {
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
			iterCancel()
			return procErr
		}
		iterCancel()
		if answerSent {
			log.Printf("[Agent] answerSent=true at iter %d, breaking out of loop", iter+1)
			break
		}

		// HARD CAP: After 8 iterations with zero writes, force answer and break
		if !anyWriteCalled && iter >= 7 {
			log.Printf("[Agent] HARD CAP: %d iterations with 0 writes, forcing final answer", iter+1)
			_ = r.forceAnswer(ctx, conversation, w, sessionID, cfg, reqProviderID, reqModel,
				fmt.Sprintf("CRITICAL STOP: You have completed %d iterations without writing a single file. "+
					"This is unacceptable. You MUST now stop all tool calls and provide your final answer "+
					"based on what you have already read. Do NOT call any more tools.", iter+1))
			break
		}
	}

	// Auto-trigger build_module if files were written but build_module never called
	r.autoTriggerBuildIfNeeded(ctx, w, sessionID, cfg, anyWriteCalled, buildModuleCalled)

	// Exhausted iterations — send answer if we haven't already
	sendFinalAnswer(w, cfg, lastLLMResp, answerSent)

	// Auto-store the final answer as episodic memory
	if lastLLMResp != nil && len(lastLLMResp.Content) > 0 {
		r.autoStoreMemory(cfg.UserID, sessionID, lastLLMResp.Content)
	}
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
