package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/agent"
	"github.com/moduforge/backend/internal/agent/registry"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/database"
	"github.com/moduforge/backend/internal/llm"
	"github.com/moduforge/backend/internal/service"
)

type AgentHandler struct {
	runner *agent.AgentRunner
	db     *database.DB
	cfg    *config.Config
}

func NewAgentHandler(cfg *config.Config, db *database.DB) *AgentHandler {
	// Auto-register all skills via init() factories — no manual registration needed
	memStore := service.NewMemoryStore(db.Conn)

	// Create file hash cache shared between skills and runner
	fileHashCache := agent.NewFileHashCache()

	deps := &registry.Deps{
		DB:            db.Conn,
		StoragePath:   cfg.StoragePath,
		LLMApiKey:     cfg.EffectiveLLMKey(),
		LLMEndpoint:   cfg.LLMEndpoint,
		LLMModel:      cfg.LLMModel,
		HTTPClient:    agent.LLMHTTPClient(),
		MemoryStore:   memStore,
		FileHashCache: fileHashCache,
	}
	registry := agent.NewSkillRegistry(deps)

	runner := agent.NewAgentRunner(
		registry,
		cfg.EffectiveLLMKey(),
		cfg.LLMEndpoint,
		cfg.LLMModel,
		db.Conn,
	)
	runner.SetMemoryStore(memStore)
	runner.SetFileHashCache(fileHashCache)

	return &AgentHandler{
		runner: runner,
		db:     db,
		cfg:    cfg,
	}
}

func (h *AgentHandler) Run(c fiber.Ctx) error {
	var req struct {
		Task           string            `json:"task"`
		SessionID      string            `json:"session_id"`
		Messages       []service.Message `json:"messages"`
		ProviderID     string            `json:"provider_id"`
		Model          string            `json:"model"`
		ProjectID      string            `json:"project_id"`
		ProjectContext string            `json:"project_context"`
		AgentMode      string            `json:"agent_mode"` // "plan" or "act"
	}
	if err := c.Bind().JSON(&req); err != nil {
		slog.Error("Agent.Run: bind failed", "error", err)
		return BadRequest(c, "invalid request")
	}
	if req.Task == "" {
		slog.Warn("Agent.Run: no task")
		return BadRequest(c, "task required")
	}
	if len(req.Task) > 20000 {
		return BadRequest(c, "task too long (max 20000)")
	}

	uid, _ := c.Locals("uid").(string)
	slog.Info("Agent.Run", "task_len", len(req.Task), "session_id", req.SessionID, "uid", uid, "project_id", req.ProjectID)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("X-Accel-Buffering", "no")

	// 构建请求级 RunConfig（每次请求独立，不修改共享 runner 状态）
	runCfg := agent.NewRunConfig(uid)
	runCfg.ProjectID = req.ProjectID
	runCfg.ProjectContext = req.ProjectContext
	// Set agent mode from request
	if req.AgentMode == "plan" {
		runCfg.Mode = agent.ModePlan
	} else {
		runCfg.Mode = agent.ModeAct
	}

	// Resolve LLM provider — same logic as UpdateLLMConfig
	// This ensures agent uses the correct endpoint/apiKey, not just the provider ID
	provider := llm.FindProvider(req.ProviderID)
	if provider != nil {
		runCfg.ProviderID = req.ProviderID
		runCfg.LLMEndpoint = provider.Endpoint
		runCfg.LLMModel = req.Model
		// Always resolve the API key for this provider from env/config,
		// not just when h.cfg.LLMProvider matches (which may be stale/empty).
		saved := h.cfg.LLMProvider
		h.cfg.LLMProvider = req.ProviderID
		runCfg.LLMApiKey = h.cfg.EffectiveLLMKey()
		h.cfg.LLMProvider = saved
		// Resolve max_output_tokens from provider's model list
		for _, m := range provider.Models {
			if m.ID == req.Model || m.Name == req.Model {
				if m.MaxTokens > 0 {
					runCfg.MaxOutputTokens = m.MaxTokens
				}
				break
			}
		}
		log.Printf("[Agent] resolved provider=%s endpoint=%s model=%s max_tokens=%d", req.ProviderID, runCfg.LLMEndpoint, runCfg.LLMModel, runCfg.MaxOutputTokens)
	} else {
		// Fallback: use the in-memory config (set by UpdateLLMConfig)
		runCfg.ProviderID = req.ProviderID
		runCfg.LLMEndpoint = h.cfg.LLMEndpoint
		runCfg.LLMModel = h.cfg.LLMModel
		runCfg.LLMApiKey = h.cfg.EffectiveLLMKey()
		log.Printf("[Agent] provider=%s not in presets, using in-memory config endpoint=%s model=%s", req.ProviderID, runCfg.LLMEndpoint, runCfg.LLMModel)
	}

	// 从数据库读取用户 Agent 设置
	h.loadAgentSettings(&runCfg)

	c.RequestCtx().SetBodyStreamWriter(func(bw *bufio.Writer) {
		w := &bufioSSEWriter{bw: bw}

		// Wrap writer to capture the final answer for persistence
		var capturedAnswer string
		var capturedSteps [][2]string // [stepType, content] pairs
		captureW := &answerCaptureWriter{
			SSEWriter: w,
			onAnswer:  func(answer string) { capturedAnswer = answer },
			onStep: func(stepType, content string) {
				capturedSteps = append(capturedSteps, [2]string{stepType, content})
			},
		}

		// === EARLY PERSISTENCE: Save user message BEFORE agent runs ===
		// This ensures user messages are never lost, even if the agent fails/disconnects.
		var currentRound int
		if req.SessionID != "" && uid != "" {
			service.EnsureConversationMessagesTable(h.db.Conn)

			// Determine current round
			var maxUserRound int
			h.db.Conn.QueryRow(
				`SELECT COALESCE(MAX(round_index), -1) FROM conversation_messages WHERE session_id=? AND user_id=? AND role='user'`,
				req.SessionID, uid,
			).Scan(&maxUserRound)

			// Check if this exact user message already exists (idempotent re-save)
			var existingUserRound int
			found := h.db.Conn.QueryRow(
				`SELECT COALESCE(MAX(round_index), -1) FROM conversation_messages WHERE session_id=? AND user_id=? AND role='user' AND content=?`,
				req.SessionID, uid, req.Task,
			).Scan(&existingUserRound)

			if found == nil && existingUserRound >= 0 {
				currentRound = existingUserRound
				log.Printf("[Agent] PERSIST re-run: user message exists at round=%d", currentRound)
			} else {
				currentRound = maxUserRound + 1
			}

			log.Printf("[Agent] PERSIST round=%d maxUserRound=%d task_len=%d", currentRound, maxUserRound, len(req.Task))

			// Save user message IMMEDIATELY (before agent runs)
			if req.Task != "" {
				var userCount int
				h.db.Conn.QueryRow(
					`SELECT COUNT(*) FROM conversation_messages WHERE session_id=? AND user_id=? AND role='user' AND round_index=?`,
					req.SessionID, uid, currentRound,
				).Scan(&userCount)
				if userCount == 0 {
					if err := service.SaveConversationMessage(h.db.Conn, req.SessionID, uid, "user", req.Task, currentRound); err != nil {
						log.Printf("[Agent] PERSIST ERROR user message: %v", err)
					} else {
						log.Printf("[Agent] PERSIST OK user message round=%d len=%d", currentRound, len(req.Task))
					}
				} else {
					log.Printf("[Agent] PERSIST skip user round=%d (already exists, count=%d)", currentRound, userCount)
				}
			} else {
				log.Printf("[Agent] PERSIST WARN req.Task is empty, skipping user message save")
			}

			// Ensure ai_conversations metadata exists (title will be updated after agent completes)
			title := req.Task
			if len([]rune(title)) > 40 {
				title = string([]rune(title)[:40]) + "..."
			}
			agentModeStr := "act"
			if req.AgentMode == "plan" {
				agentModeStr = "plan"
			}
			h.db.Conn.Exec(
				`INSERT INTO ai_conversations (id, user_id, title, mode, messages, model, project_id, agent_mode, updated_at)
				 VALUES (?, ?, ?, 'agent', '[]', ?, ?, ?, datetime('now'))
				 ON CONFLICT(id) DO UPDATE SET title=?, mode='agent', model=?, project_id=?, agent_mode=?, updated_at=datetime('now')`,
				req.SessionID, uid, title, req.Model, req.ProjectID, agentModeStr,
				title, req.Model, req.ProjectID, agentModeStr,
			)

			// Send round_sync event so frontend can update its round counter
			w.WriteSSE(map[string]interface{}{
				"type":        "round_sync",
				"round":       currentRound,
				"max_round":   currentRound,
				"session_id":  req.SessionID,
			})
		}

		// Send project info to frontend if auto-created
		if runCfg.ProjectID != "" && req.ProjectID == "" {
			w.WriteSSE(map[string]interface{}{
				"type":       "project_created",
				"project_id": runCfg.ProjectID,
				"message":    fmt.Sprintf("📁 已自动创建项目：%s", runCfg.ProjectID[:8]+"…"),
			})
		}
		h.runner.Run(c.Context(), req.Task, uid, req.Messages, req.SessionID, captureW, req.ProviderID, req.Model, runCfg)

		// Flush any remaining reasoning content
		if captureW.reasoningBuf.Len() > 0 && !captureW.flushed {
			captureW.flushed = true
			if captureW.onStep != nil {
				captureW.onStep("reasoning", captureW.reasoningBuf.String())
			}
		}

		// Debug: log what was captured
		log.Printf("[Agent] DEBUG capturedAnswer len=%d, capturedSteps count=%d", len(capturedAnswer), len(capturedSteps))
		for i, s := range capturedSteps {
			log.Printf("[Agent] DEBUG step[%d] type=%s content_len=%d", i, s[0], len(s[1]))
		}

		// === POST-AGENT PERSISTENCE: Save steps and answer ===
		if req.SessionID != "" && uid != "" {
			// Save agent steps (append, never delete previous rounds)
			for _, step := range capturedSteps {
				stepType, content := step[0], step[1]
				if stepType == "answer" {
					continue
				}
				if err := service.SaveAgentStep(h.db.Conn, req.SessionID, uid, stepType, content, currentRound); err != nil {
					log.Printf("[Agent] PERSIST ERROR step %s: %v", stepType, err)
				}
			}
			if len(capturedSteps) > 0 {
				log.Printf("[Agent] PERSIST OK %d steps round=%d", len(capturedSteps), currentRound)
			}

			// Save assistant answer (replace if same round already has one)
			if capturedAnswer != "" {
				var oldAssistantCount int
				h.db.Conn.QueryRow(
					`SELECT COUNT(*) FROM conversation_messages WHERE session_id=? AND user_id=? AND role='assistant' AND round_index=?`,
					req.SessionID, uid, currentRound,
				).Scan(&oldAssistantCount)
				if oldAssistantCount > 0 {
					h.db.Conn.Exec(
						`DELETE FROM conversation_messages WHERE session_id=? AND user_id=? AND role='assistant' AND round_index=?`,
						req.SessionID, uid, currentRound,
					)
					log.Printf("[Agent] PERSIST replaced old assistant round=%d", currentRound)
				}
				if err := service.SaveConversationMessage(h.db.Conn, req.SessionID, uid, "assistant", capturedAnswer, currentRound); err != nil {
					log.Printf("[Agent] PERSIST ERROR assistant: %v", err)
				} else {
					log.Printf("[Agent] PERSIST OK assistant round=%d len=%d", currentRound, len(capturedAnswer))
				}
			} else {
				log.Printf("[Agent] PERSIST WARN no capturedAnswer, skipping assistant save")
			}
		}

		// Flush with timeout to avoid blocking on disconnected clients
		done := make(chan struct{})
		go func() {
			bw.Flush()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			log.Printf("[Agent] bw.Flush timeout — client likely disconnected")
		}
	})
	return nil
}

// loadAgentSettings 从数据库读取 Agent 配置并应用到 RunConfig（不修改共享 runner 状态）
func (h *AgentHandler) loadAgentSettings(cfg *agent.RunConfig) {
	if h.db == nil {
		return
	}
	// 确保表存在
	h.db.Conn.Exec(`CREATE TABLE IF NOT EXISTS agent_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	var val string
	if err := h.db.Conn.QueryRow("SELECT value FROM agent_settings WHERE key='max_iterations'").Scan(&val); err == nil {
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil && n > 0 && n <= 200 {
			cfg.MaxIterations = n
		}
	}
	if err := h.db.Conn.QueryRow("SELECT value FROM agent_settings WHERE key='max_result_len'").Scan(&val); err == nil {
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil && n > 0 && n <= 100000 {
			cfg.MaxResultLen = n
		}
	}
}

func (h *AgentHandler) ListSkills(c fiber.Ctx) error {
	var skillsList []map[string]string
	for _, s := range h.runner.ListSkills() {
		skillsList = append(skillsList, map[string]string{
			"name":        s.Name(),
			"description": s.Description(),
		})
	}
	return c.JSON(fiber.Map{"skills": skillsList})
}

// ===== Custom Skills =====

type CustomSkill struct {
	ID          int64  `json:"id"`
	UserID      string `json:"user_id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	InputSchema string `json:"input_schema"`
	IsPublic    bool   `json:"is_public"`
	CreatedAt   string `json:"created_at"`
}

func (h *AgentHandler) ListCustomSkills(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}

	rows, err := h.db.Conn.Query(
		"SELECT id, user_id, name, description, prompt, input_schema, is_public, created_at FROM custom_skills WHERE user_id = ? OR is_public = 1 ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return InternalError(c, err.Error())
	}
	defer rows.Close()

	var skills []CustomSkill
	for rows.Next() {
		var s CustomSkill
		var isPub int
		if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &s.Description, &s.Prompt, &s.InputSchema, &isPub, &s.CreatedAt); err != nil {
			continue
		}
		s.IsPublic = isPub == 1
		if s.UserID != userID {
			s.UserID = ""
		}
		skills = append(skills, s)
	}
	if skills == nil {
		skills = []CustomSkill{}
	}
	return c.JSON(fiber.Map{"skills": skills})
}

func (h *AgentHandler) CreateCustomSkill(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Prompt      string `json:"prompt"`
		InputSchema string `json:"input_schema"`
		IsPublic    bool   `json:"is_public"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Name == "" || req.Description == "" || req.Prompt == "" {
		return ValidationError(c, "name, description, and prompt are required")
	}
	if len(req.Name) > 100 {
		return ValidationError(c, "name too long (max 100)")
	}
	if len(req.Description) > 500 {
		return ValidationError(c, "description too long (max 500)")
	}
	if len(req.Prompt) > 10000 {
		return ValidationError(c, "prompt too long (max 10000)")
	}
	if req.InputSchema == "" {
		req.InputSchema = "{}"
	}

	isPub := 0
	if req.IsPublic {
		isPub = 1
	}

	result, err := h.db.Conn.Exec(
		"INSERT INTO custom_skills (user_id, name, description, prompt, input_schema, is_public) VALUES (?, ?, ?, ?, ?, ?)",
		userID, req.Name, req.Description, req.Prompt, req.InputSchema, isPub,
	)
	if err != nil {
		return InternalError(c, err.Error())
	}

	id, _ := result.LastInsertId()
	return c.Status(201).JSON(fiber.Map{"id": id, "status": "ok"})
}

func (h *AgentHandler) UpdateCustomSkill(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}

	skillID := c.Params("id")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Prompt      string `json:"prompt"`
		InputSchema string `json:"input_schema"`
		IsPublic    bool   `json:"is_public"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}

	isPub := 0
	if req.IsPublic {
		isPub = 1
	}

	_, err := h.db.Conn.Exec(
		"UPDATE custom_skills SET name=?, description=?, prompt=?, input_schema=?, is_public=? WHERE id=? AND user_id=?",
		req.Name, req.Description, req.Prompt, req.InputSchema, isPub, skillID, userID,
	)
	if err != nil {
		return InternalError(c, err.Error())
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AgentHandler) DeleteCustomSkill(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}

	skillID := c.Params("id")
	_, err := h.db.Conn.Exec("DELETE FROM custom_skills WHERE id=? AND user_id=?", skillID, userID)
	if err != nil {
		return InternalError(c, err.Error())
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AgentHandler) ExecuteCustomSkill(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}

	skillID := c.Params("id")
	var req struct {
		Input string `json:"input"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Input == "" {
		return ValidationError(c, "input required")
	}

	var skill struct {
		Prompt string
	}
	err := h.db.Conn.QueryRow(
		"SELECT prompt FROM custom_skills WHERE id=? AND (user_id=? OR is_public=1)",
		skillID, userID,
	).Scan(&skill.Prompt)
	if err != nil {
		return NotFound(c, "skill not found")
	}

	prompt := strings.ReplaceAll(skill.Prompt, "{input}", req.Input)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("X-Accel-Buffering", "no")

	// 每次请求独立的 RunConfig
	runCfg := agent.NewRunConfig(userID)
	h.loadAgentSettings(&runCfg)

	// Reuse the agent runner with its existing registry and config
	messages := []service.Message{
		{Role: "user", Content: prompt},
	}
	c.RequestCtx().SetBodyStreamWriter(func(bw *bufio.Writer) {
		w := &bufioSSEWriter{bw: bw}
		h.runner.Run(c.Context(), prompt, userID, messages, "", w, "", "", runCfg)
		// Flush with timeout to avoid blocking on disconnected clients
		done := make(chan struct{})
		go func() {
			bw.Flush()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			log.Printf("[Agent] bw.Flush timeout (auto-build) — client likely disconnected")
		}
	})
	return nil
}

// ─── Skill Evolution (6) ───

func (h *AgentHandler) GetSkillEvolution(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}
	skillID := c.Params("id")

	var stats struct {
		TotalRuns   int     `json:"total_runs"`
		SuccessRate float64 `json:"success_rate"`
		AvgDuration float64 `json:"avg_duration_ms"`
		LastRunAt   string  `json:"last_run_at"`
	}
	h.db.Conn.QueryRow(`
		SELECT COUNT(*), 
			COALESCE(AVG(CASE WHEN success=1 THEN 1.0 ELSE 0.0 END), 0),
			COALESCE(AVG(duration_ms), 0),
			COALESCE(MAX(created_at), '')
		FROM skill_evolution WHERE skill_id=? AND user_id=?
	`, skillID, userID).Scan(&stats.TotalRuns, &stats.SuccessRate, &stats.AvgDuration, &stats.LastRunAt)

	// Recent runs
	rows, err := h.db.Conn.Query(`
		SELECT id, input, output, success, duration_ms, feedback, created_at
		FROM skill_evolution WHERE skill_id=? AND user_id=? ORDER BY created_at DESC LIMIT 20
	`, skillID, userID)
	var history []map[string]interface{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var input, output, feedback, createdAt string
			var success int
			var durationMs int
			if err := rows.Scan(&id, &input, &output, &success, &durationMs, &feedback, &createdAt); err == nil {
				history = append(history, map[string]interface{}{
					"id":          id,
					"input":       input,
					"output":      output,
					"success":     success == 1,
					"duration_ms": durationMs,
					"feedback":    feedback,
					"created_at":  createdAt,
				})
			}
		}
	}

	return c.JSON(fiber.Map{
		"stats":   stats,
		"history": history,
	})
}

func (h *AgentHandler) RecordSkillEvolution(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}
	skillID := c.Params("id")
	var req struct {
		Input      string `json:"input"`
		Output     string `json:"output"`
		Success    bool   `json:"success"`
		DurationMs int    `json:"duration_ms"`
		Feedback   string `json:"feedback"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	success := 0
	if req.Success {
		success = 1
	}
	_, err := h.db.Conn.Exec(
		"INSERT INTO skill_evolution (skill_id, user_id, input, output, success, duration_ms, feedback) VALUES (?, ?, ?, ?, ?, ?, ?)",
		skillID, userID, req.Input, req.Output, success, req.DurationMs, req.Feedback,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("记录失败: %v", err)})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AgentHandler) GetSkillOptimization(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}
	skillID := c.Params("id")

	// Get skill info
	var prompt string
	err := h.db.Conn.QueryRow("SELECT prompt FROM custom_skills WHERE id=? AND user_id=?", skillID, userID).Scan(&prompt)
	if err != nil {
		return NotFound(c, "skill not found")
	}

	// Get stats
	var totalRuns, successRuns int
	var avgDuration float64
	h.db.Conn.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END), 0), COALESCE(AVG(duration_ms), 0)
		FROM skill_evolution WHERE skill_id=? AND user_id=?
	`, skillID, userID).Scan(&totalRuns, &successRuns, &avgDuration)

	// Generate suggestions
	var suggestions []string
	if totalRuns == 0 {
		suggestions = append(suggestions, "尚无执行记录，请先运行该技能以获取优化建议")
	} else {
		rate := float64(0)
		if totalRuns > 0 {
			rate = float64(successRuns) / float64(totalRuns) * 100
		}
		if rate < 80 {
			suggestions = append(suggestions, fmt.Sprintf("成功率 %.0f%% 偏低，建议优化 prompt 以提高成功率", rate))
		}
		if avgDuration > 10000 {
			suggestions = append(suggestions, fmt.Sprintf("平均耗时 %.0fms 较长，考虑精简 prompt 或使用更快的模型", avgDuration))
		}
		if totalRuns > 10 && rate >= 90 {
			suggestions = append(suggestions, "该技能运行稳定，表现良好")
		}
		if totalRuns > 5 {
			suggestions = append(suggestions, fmt.Sprintf("已运行 %d 次，可考虑基于历史成功案例创建更多变体技能", totalRuns))
		}
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "继续使用并收集更多数据以获得优化建议")
	}

	return c.JSON(fiber.Map{
		"prompt":       prompt,
		"total_runs":   totalRuns,
		"success_rate": fmt.Sprintf("%.0f%%", float64(successRuns)/float64(totalRuns)*100),
		"avg_duration": fmt.Sprintf("%.0fms", avgDuration),
		"suggestions":  suggestions,
	})
}

// bufioSSEWriter wraps bufio.Writer and implements agent.SSEWriter.
// Unlike the old fiberSSEWriter (which had a no-op Flush), this writer
// flushes after every write so SSE events reach the client in real time.
// It tracks connection state — once a write fails, all subsequent writes
// return immediately without attempting I/O (preventing busy-loop on
// disconnected clients).
type bufioSSEWriter struct {
	bw           *bufio.Writer
	disconnected bool
	mu           sync.Mutex // serializes concurrent writes (parallel tools, keepalives)
}

func (w *bufioSSEWriter) WriteSSE(data map[string]interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.disconnected {
		return io.ErrClosedPipe
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = w.bw.Write([]byte("data: " + string(jsonBytes) + "\n\n"))
	if err != nil {
		w.disconnected = true
		return err
	}
	return w.bw.Flush()
}

func (w *bufioSSEWriter) WriteSSEPlain(data string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.disconnected {
		return io.ErrClosedPipe
	}
	_, err := w.bw.Write([]byte("data: " + data + "\n\n"))
	if err != nil {
		w.disconnected = true
		return err
	}
	return w.bw.Flush()
}

func (w *bufioSSEWriter) WriteSSEComment(comment string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.disconnected {
		return io.ErrClosedPipe
	}
	_, err := w.bw.Write([]byte(": " + comment + "\n\n"))
	if err != nil {
		w.disconnected = true
		return err
	}
	return w.bw.Flush()
}

// IsDisconnected returns true if the underlying connection has been lost.
func (w *bufioSSEWriter) IsDisconnected() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.disconnected
}

// FlushWithTimeout flushes with a timeout to avoid blocking on disconnected clients.
func (w *bufioSSEWriter) FlushWithTimeout(d time.Duration) error {
	done := make(chan error, 1)
	go func() {
		done <- w.Flush()
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(d):
		return fmt.Errorf("flush timeout after %v", d)
	}
}

func (w *bufioSSEWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.bw.Flush()
}

// answerCaptureWriter wraps an SSEWriter and intercepts agent step events
// to capture the final answer and intermediate steps for persistence.
type answerCaptureWriter struct {
	agent.SSEWriter
	onAnswer func(string)
	onStep   func(stepType, content string) // called for think/skill_call/skill_result/answer/reasoning
	// Accumulate reasoning chunks (streaming) into a single step
	reasoningBuf strings.Builder
	flushed      bool
	// Track tool calls for persistence
	toolCallsJSON string
	toolCallID    string
	mu            sync.Mutex // serializes concurrent WriteSSE from parallel tool goroutines
}

func (w *answerCaptureWriter) WriteSSE(data map[string]interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Accumulate reasoning chunks (streaming LLM extended thinking)
	if dataType, _ := data["type"].(string); dataType == "reasoning" {
		if content, _ := data["content"].(string); content != "" {
			w.reasoningBuf.WriteString(content)
		}
		return w.SSEWriter.WriteSSE(data)
	}
	// Flush accumulated reasoning when a non-reasoning event arrives
	if w.reasoningBuf.Len() > 0 && !w.flushed {
		w.flushed = true
		if w.onStep != nil {
			w.onStep("reasoning", w.reasoningBuf.String())
		}
	}
	if step, ok := data["step"].(string); ok {
		content, _ := data["content"].(string)
		skill, _ := data["skill"].(string)
		switch step {
		case "answer":
			if content != "" {
				w.onAnswer(content)
			}
			if w.onStep != nil {
				w.onStep("answer", content)
			}
		case "think":
			if w.onStep != nil && content != "" {
				w.onStep("think", content)
			}
		case "skill_call":
			if w.onStep != nil && skill != "" {
				input, _ := json.Marshal(data["input"])
				w.onStep("skill_call", fmt.Sprintf("%s: %s", skill, string(input)))
				// Capture tool_call_id if present
				if tcID, ok := data["tool_call_id"].(string); ok {
					w.toolCallID = tcID
				}
				// Capture tool_calls JSON if present
				if tcJSON, ok := data["tool_calls"].(string); ok {
					w.toolCallsJSON = tcJSON
				}
			}
		case "skill_result":
			if w.onStep != nil && content != "" {
				// Save truncated content for persistence — Agent needs tool results in multi-round history
				summary := content
				if len(summary) > 2000 {
					summary = summary[:2000] + "... [truncated]"
				}
				if blocked, _ := data["blocked"].(bool); blocked {
					summary = content
				}
				w.onStep("skill_result", summary)
			}
		}
	}
	return w.SSEWriter.WriteSSE(data)
}

// ═══════════════════════════════════════════════════════════════════
// NEW: Statistics and monitoring endpoints
// ═══════════════════════════════════════════════════════════════════

// GetToolStats returns tool usage statistics.
func (h *AgentHandler) GetToolStats(c fiber.Ctx) error {
	stats := h.runner.GetToolStats()
	return c.JSON(fiber.Map{"stats": stats})
}

// GetAuditHistory returns recent audit entries.
func (h *AgentHandler) GetAuditHistory(c fiber.Ctx) error {
	toolName := c.Query("tool", "")
	limitStr := c.Query("limit", "50")
	limit := 50
	fmt.Sscanf(limitStr, "%d", &limit)
	entries := h.runner.GetAuditHistory(toolName, limit)
	return c.JSON(fiber.Map{"entries": entries})
}

// GetPermissionDenials returns recent permission denials.
func (h *AgentHandler) GetPermissionDenials(c fiber.Ctx) error {
	limitStr := c.Query("limit", "50")
	limit := 50
	fmt.Sscanf(limitStr, "%d", &limit)
	denials := h.runner.GetPermissionDenials(limit)
	return c.JSON(fiber.Map{"denials": denials})
}

// GetSessionState returns session state for a given session ID.
func (h *AgentHandler) GetSessionState(c fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return BadRequest(c, "sessionId required")
	}
	state := h.runner.GetSessionState(sessionID)
	return c.JSON(fiber.Map{"state": state})
}
