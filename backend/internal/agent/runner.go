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
	toolTimeoutFast  = 30 * time.Second  // read_file, list_dir, detect, etc.
	toolTimeoutWrite = 60 * time.Second  // write_file, create_dir, etc.
	toolTimeoutSlow  = 300 * time.Second // build_module, code_pipeline, etc.
	toolTimeoutLLM   = 180 * time.Second // web_search, generate_code (may call external APIs)
)

// toolTimeoutForName returns the appropriate timeout for a given tool.
func toolTimeoutForName(name string) time.Duration {
	switch name {
	case "read_file", "list_dir", "detect", "lint_code", "validate", "think",
		"memory_manager", "match_template", "gather_requirements":
		return toolTimeoutFast
	case "write_file", "write_file_batch", "create_dir", "delete_file", "delete_dir", "move_file":
		return toolTimeoutWrite
	case "build_module", "code_pipeline", "regression_check":
		return toolTimeoutSlow
	case "web_search", "generate_code":
		return toolTimeoutLLM
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
	UserID         string
	ProjectID      string
	ProjectContext string
	MaxIterations  int
	MaxResultLen   int
	Mode           AgentMode // "plan" or "act" — controls tool availability
	LLMEndpoint    string    // resolved endpoint (from handler, overrides DB lookup)
	LLMApiKey      string    // resolved API key
	LLMModel       string    // resolved model ID
	MaxOutputTokens int      // max output tokens (0 = use model default)
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
	sessionCaches   sync.Map // sessionID -> *toolResultCache
	// Optimization 17: write_file content cache (avoids redundant read_file after write)
	writeContentCache sync.Map // sessionID -> map[string]string (path -> content)
	// Optimization 1: Session access time tracking for TTL-based cleanup
	sessionAccessTimes sync.Map // sessionID -> time.Time (last access timestamp)
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
type cachedContent struct {
	content   string
	expiresAt time.Time
}

const writeContentCacheTTL = 5 * time.Minute

// cacheWriteContent stores the content of a successful write_file call.
// When read_file is called for the same path immediately after, it returns
// this cached content instead of re-reading from disk — saving one full I/O round.
func (r *AgentRunner) cacheWriteContent(sessionID, path, content string) {
	if sessionID == "" {
		return
	}
	val, _ := r.writeContentCache.LoadOrStore(sessionID, &sync.Map{})
	m := val.(*sync.Map)
	m.Store(path, cachedContent{content: content, expiresAt: time.Now().Add(writeContentCacheTTL)})
	debugLog("writeContentCache PUT: session=%s path=%s len=%d", sessionID, path, len(content))
}

// getCachedWriteContent returns the cached content for a path, or "" if not cached or expired.
// Only returns content for read_file calls that happen in the same session.
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
		cc := cached.(cachedContent)
		if time.Now().After(cc.expiresAt) {
			// Entry expired — remove it and return empty
			m.Delete(path)
			debugLog("writeContentCache EXPIRED: session=%s path=%s", sessionID, path)
			return ""
		}
		debugLog("writeContentCache HIT: session=%s path=%s", sessionID, path)
		return cc.content
	}
	return ""
}

// cleanSessionCaches removes session-scoped caches (called on session end/disconnect).
func (r *AgentRunner) cleanSessionCaches(sessionID string) {
	if sessionID == "" {
		return
	}
	r.sessionCaches.Delete(sessionID)
	r.writeContentCache.Delete(sessionID)
	r.sessionAccessTimes.Delete(sessionID)
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
			conversation = append(conversation, map[string]interface{}{
				"role": "system", "content": "[Previous conversation]",
			})
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
	const maxWriteFilePerTurn = 15 // safety limit per iteration (raised from 5: large projects often need 10+ files per turn)
	const maxReadFilePerTurn = 10 // limit read_file calls per iteration (raised from 4: OpenCode-style tools need more reads)
	checkpoints := make([]FileCheckpoint, 0) // file change history for undo
	consecutiveErrors := 0
	answerSent := false
	var lastLLMResp *LLMResponse

	// Optimization 1: Session-scoped tool result cache (persists across Run() calls)
	toolCache := r.getSessionCache(sessionID)

	// Optimization 24: Self-reflection tracking — detect repeated tool failures
	toolConsecutiveErrors := make(map[string]int)  // skill name -> consecutive error count
	toolLastResults := make(map[string]string)     // skill name -> last result (for pattern detection)
	toolConsecutiveIdentical := make(map[string]int) // skill name -> consecutive identical calls

	// Derive skill sets from metadata (no hardcoded maps)
	readOnlySkills := r.registry.ReadOnlySkills()

	for iter := 0; iter < cfg.MaxIterations; iter++ {
		writeFileCount = 0 // reset per-iteration counter
		readFileCount = 0
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
		go func() {
			incGoroutines()
			defer decGoroutines()
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					_ = w.WriteSSE(map[string]interface{}{"type": "step", "step": "think", "content": ""})
				case <-llmDone:
					return
				}
			}
		}()

		// Optimization 2: Prefilter conversation to remove waste
		prefiltered := prefilterConversation(conversation)

		llmResp, err := r.callLLMWithTools(ctx, prefiltered, toolDefs, w, cfg.UserID, reqProviderID, reqModel, cfg)
		close(llmDone)
		lastLLMResp = llmResp
		if err != nil {
			// Classify error: permanent errors stop immediately, transient ones retry
			if !isRetryableError(err) {
				log.Printf("[Agent] non-retryable error: %v", err)
				w.WriteSSE(map[string]interface{}{"type": "error", "error": userFriendlyError(err)})
				w.WriteSSEPlain("[DONE]")
				return err
			}
			consecutiveErrors++
			if consecutiveErrors >= 3 {
				log.Printf("[Agent] retry limit reached after %d attempts: %v", consecutiveErrors, err)
				w.WriteSSE(map[string]interface{}{"type": "error", "error": userFriendlyError(err)})
				w.WriteSSEPlain("[DONE]")
				return err
			}
			// Auto-compact on context-too-long errors before retrying
			errStr := err.Error()
			if strings.Contains(errStr, "context_length_exceeded") || strings.Contains(errStr, "maximum context length") || strings.Contains(errStr, "max_tokens") {
				log.Printf("[Agent] context too long, compacting conversation before retry...")
				compacted, cErr := r.compactConversation(ctx, conversation, w, cfg)
				if cErr == nil {
					conversation = compacted
					w.WriteSSE(map[string]interface{}{
						"type":    "step",
						"step":    "think",
						"content": "📋 上下文过长，已自动压缩，正在重试...",
					})
				}
			}
			// Backoff: 1s, 2s, 4s (exponential)
			backoff := time.Duration(1<<(consecutiveErrors-1)) * time.Second
			log.Printf("[Agent] LLM error (attempt %d): %v, retrying in %v...", consecutiveErrors, err, backoff)
			// Notify frontend before sleeping so safety timer resets
			w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "think",
				"content": fmt.Sprintf("⚠️ LLM 调用失败 (attempt %d/%d)，%v 后重试...", consecutiveErrors, 3, backoff),
			})
			// Keepalive during sleep — send real data events, not just comments
			// Some proxies/CDNs drop connections with only SSE comments
			sleepDone := make(chan struct{})
			go func() {
				incGoroutines()
				defer decGoroutines()
				ticker := time.NewTicker(3 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						_ = w.WriteSSE(map[string]interface{}{"type": "step", "step": "think", "content": ""})
					case <-sleepDone:
						return
					}
				}
			}()
			time.Sleep(backoff)
			close(sleepDone)
			conversation = append(conversation, map[string]interface{}{
				"role":    "user",
				"content": fmt.Sprintf("[System: LLM call failed with error: %v. Please try again.]", err),
			})
			continue
		}
		consecutiveErrors = 0

		debugLog("iter=%d mode=%s role=%s contentLen=%d toolCalls=%d",
			iter+1, cfg.Mode, llmResp.Role, len(llmResp.Content), len(llmResp.ToolCalls))

		// ── Case 1: Final answer ──
		if llmResp.Role == "assistant" && len(llmResp.ToolCalls) == 0 {
			answer := cleanAnswer(llmResp.Content)

			// Auto-retry if answer was truncated by max_tokens
			if llmResp.FinishReason == "length" && iter < cfg.MaxIterations-1 {
				log.Printf("[Agent] answer truncated (finish_reason=length, len=%d), requesting continuation...", len(answer))
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "think",
					"content": "⚠️ 答案被截断，正在请求续写...",
				})
				conversation = append(conversation, map[string]interface{}{
					"role": "assistant", "content": answer,
				})
				conversation = append(conversation, map[string]interface{}{
					"role":    "user",
					"content": "你的回答被截断了。请继续完成上面的回答，从上次中断的地方接着写。不要重复已有内容。",
				})
				iter++
				continue
			}

			// If answer is garbled, retry once
			if isGarbageOutput(answer) && iter < cfg.MaxIterations-1 {
				debugLog("garbage answer detected in main loop (len=%d), retrying...", len(answer))
				conversation = append(conversation, map[string]interface{}{
					"role": "assistant", "content": answer,
				})
				conversation = append(conversation, map[string]interface{}{
					"role":    "user",
					"content": "Your previous answer was garbled/unreadable. Please provide a clear, well-formatted Markdown answer. Do NOT use tools.",
				})
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
					conversation = append(conversation, map[string]interface{}{
						"role": "assistant", "content": answer,
					})
					conversation = append(conversation, map[string]interface{}{
						"role":    "user",
						"content": "你提到修改了文件但没有调用 write_file。请调用 write_file 保存更改，或者直接回答。",
					})
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
			tc        LLMToolCall
			skillName string
			skillInput map[string]interface{}
			parallel  bool
		}
		var tasks []toolTask
		for _, tc := range llmResp.ToolCalls {
			skillName := tc.Function.Name
			var skillInput map[string]interface{}
			json.Unmarshal([]byte(tc.Function.Arguments), &skillInput)

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
				toolResultMsg := map[string]interface{}{
					"role":         "tool",
					"tool_call_id": tc.ID,
					"content":      blocked,
				}
				conversation = append(conversation, toolResultMsg)
				// Persist to convStore
				if sessionID != "" {
					r.convStore.Append(sessionID, service.Message{
						Role:       "tool",
						Content:    blocked,
						ToolCallID: tc.ID,
					})
				}
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
					toolResultMsg := map[string]interface{}{
						"role":         "tool",
						"tool_call_id": tc.ID,
						"content":      blocked,
					}
					conversation = append(conversation, toolResultMsg)
					// Persist to convStore
					if sessionID != "" {
						r.convStore.Append(sessionID, service.Message{
							Role:       "tool",
							Content:    blocked,
							ToolCallID: tc.ID,
						})
					}
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
					toolResultMsg := map[string]interface{}{
						"role":         "tool",
						"tool_call_id": tc.ID,
						"content":      blocked,
					}
					conversation = append(conversation, toolResultMsg)
					// Persist to convStore
					if sessionID != "" {
						r.convStore.Append(sessionID, service.Message{
							Role:       "tool",
							Content:    blocked,
							ToolCallID: tc.ID,
						})
					}
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
				toolResultMsg := map[string]interface{}{
					"role":         "tool",
					"tool_call_id": tc.ID,
					"content":      paramErr,
				}
				conversation = append(conversation, toolResultMsg)
				// Persist to convStore
				if sessionID != "" {
					r.convStore.Append(sessionID, service.Message{
						Role:       "tool",
						Content:    paramErr,
						ToolCallID: tc.ID,
					})
				}
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

		// Execute tools: parallel for read-only, sequential for write/side-effect
		var mu sync.Mutex
		var results []struct {
			tc     LLMToolCall
			result string
		}

		// Group parallel tasks
		var parallelTasks []toolTask
		var sequentialTasks []toolTask
		for _, t := range tasks {
			if t.parallel {
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
		for _, st := range sequentialTasks {
			// Notify frontend
			w.WriteSSE(map[string]interface{}{
				"type":  "step",
				"step":  "skill_call",
				"skill": st.skillName,
				"input": st.skillInput,
			})

			// Keepalive during execution
			skillDone := make(chan struct{})
			go func() {
				incGoroutines()
				defer decGoroutines()
				ticker := time.NewTicker(10 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						_ = w.WriteSSE(map[string]interface{}{"type": "step", "step": "think", "content": ""})
					case <-skillDone:
						return
					case <-ctx.Done():
						return
					}
				}
			}()

			// Execute with timeout
			toolTimeout := toolTimeoutForName(st.skillName)
			toolCtx, toolCancel := context.WithTimeout(ctx, toolTimeout)
			result, err := r.executeSkill(toolCtx, st.skillName, st.skillInput)
			toolCancel()
			close(skillDone)

			if toolCtx.Err() == context.DeadlineExceeded {
				result = fmt.Sprintf("⚠️ Tool execution timed out after %v", toolTimeout)
			} else if err != nil && st.skillName == "write_file" {
				log.Printf("[Agent] write_file failed: %v, providing error context to LLM", err)
				result = fmt.Sprintf("Write failed: %v. Please check the path and content, then try again.", err)
			} else if err != nil {
				result = fmt.Sprintf("Error: %v", err)
			}

			if st.skillName == "write_file" {
				writeFileCalled = true
				if path, ok := st.skillInput["path"].(string); ok {
					toolCache.invalidate(path)
					// Optimization 17: Cache written content for immediate read-back
					if content, ok := st.skillInput["content"].(string); ok && err == nil {
						r.cacheWriteContent(sessionID, path, content)
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
				toolResultMsg := map[string]interface{}{
					"role":         "tool",
					"tool_call_id": wf.tc.ID,
					"content":      wf.result,
				}
				conversation = append(conversation, toolResultMsg)
				// Persist to convStore
				if sessionID != "" {
					r.convStore.Append(sessionID, service.Message{
						Role:       "tool",
						Content:    wf.result,
						ToolCallID: wf.tc.ID,
					})
				}
			} else {
				// Multiple consecutive write_files — merge into summary
				var paths []string
				for _, wf := range pendingWriteFiles {
					paths = append(paths, wf.path)
				}
				merged := fmt.Sprintf("✅ Successfully wrote %d files: %s",
					len(pendingWriteFiles), strings.Join(paths, ", "))
				// Add one merged result for the first tool_call_id, skip the rest
				toolResultMsg := map[string]interface{}{
					"role":         "tool",
					"tool_call_id": pendingWriteFiles[0].tc.ID,
					"content":      merged,
				}
				conversation = append(conversation, toolResultMsg)
				// Persist to convStore
				if sessionID != "" {
					r.convStore.Append(sessionID, service.Message{
						Role:       "tool",
						Content:    merged,
						ToolCallID: pendingWriteFiles[0].tc.ID,
					})
				}
				// For the remaining write_file calls, add empty acknowledges
				for _, wf := range pendingWriteFiles[1:] {
					conversation = append(conversation, map[string]interface{}{
						"role":         "tool",
						"tool_call_id": wf.tc.ID,
						"content":      "(merged into previous result)",
					})
					// Persist to convStore
					if sessionID != "" {
						r.convStore.Append(sessionID, service.Message{
							Role:       "tool",
							Content:    "(merged into previous result)",
							ToolCallID: wf.tc.ID,
						})
					}
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
			toolResultMsg := map[string]interface{}{
				"role":         "tool",
				"tool_call_id": res.tc.ID,
				"content":      res.result,
			}
			conversation = append(conversation, toolResultMsg)

			// Persist tool result to convStore
			if sessionID != "" {
				r.convStore.Append(sessionID, service.Message{
					Role:       "tool",
					Content:    res.result,
					ToolCallID: res.tc.ID,
				})
			}

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
					conversation = append(conversation, map[string]interface{}{
						"role":    "system",
						"content": diagnostic,
					})
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
		if readOnlyCount >= 6 && writeCount == 0 && iter >= 2 {
			diagnostic := fmt.Sprintf(
				"⚠️ [Read-Only Loop] You have called read tools %d times without any write/edit operations. "+
					"You have already read enough code. NOW you MUST: "+
					"(1) Use edit_file for targeted fixes, or write_file to rewrite files completely. "+
					"(2) Then call build_module to verify. "+
					"DO NOT read any more files. Start writing code immediately.",
				readOnlyCount)
			log.Printf("[Agent] read-only loop detected: %d reads, %d writes", readOnlyCount, writeCount)
			conversation = append(conversation, map[string]interface{}{
				"role":    "system",
				"content": diagnostic,
			})
			w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "think",
				"content": fmt.Sprintf("🔄 检测到只读循环（%d 次读取，0 次写入），已注入编辑提醒", readOnlyCount),
			})
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

		// Smart loop detection
		if reason := detectLoop(toolCallHistory, uniqueOps, totalToolCalls); reason != "" {
			debugLog("loop detected: %s", reason)
			return r.forceAnswer(ctx, conversation, w, sessionID, cfg, reqProviderID, reqModel, reason)
		}
	}

	// Exhausted iterations — send answer if we haven't already
	if !answerSent {
		if lastLLMResp != nil && lastLLMResp.Content != "" {
			answer := cleanAnswer(lastLLMResp.Content)
			if answer != "" {
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "answer",
					"content": answer,
				})
			} else {
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "answer",
					"content": fmt.Sprintf("⚠️ Agent 已完成 %d 轮迭代，但未生成最终回答。请检查上方的工具调用步骤了解执行过程。\n\n💡 提示：你可以继续发送消息让 Agent 继续完成任务。", cfg.MaxIterations),
				})
			}
		} else {
			w.WriteSSE(map[string]interface{}{
				"type":    "step",
				"step":    "answer",
				"content": fmt.Sprintf("⚠️ Agent 已完成 %d 轮迭代，但未生成最终回答。请检查上方的工具调用步骤了解执行过程。\n\n💡 提示：你可以继续发送消息让 Agent 继续完成任务。", cfg.MaxIterations),
			})
		}
	}
	w.WriteSSEPlain("[DONE]")
	// Clean up write-content cache for this session (tool result cache persists)
	r.writeContentCache.Delete(sessionID)
	return nil
}
