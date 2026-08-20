package handler

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/agent"
	"github.com/moduforge/backend/internal/llm"
	"github.com/moduforge/backend/internal/service"
)

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

	// Monthly AI cost cap: reject new tasks once the estimated cost for the
	// current month reaches the configured limit (AI_MONTHLY_COST_LIMIT, USD).
	// Free models (price 0) never trip this; only paid models accumulate cost.
	if pi, po := agent.ModelPricer(req.Model); h.runner.MonthlyCostExceeded(uid, pi, po) {
		info := h.runner.CalcMonthlyCostInfo(uid, pi, po)
		return BadRequest(c, fmt.Sprintf("本月 AI 成本已达上限（$%.2f / $%.2f），请调整 AI_MONTHLY_COST_LIMIT 或下月再试", info.EstimatedCost, info.LimitUSD))
	}

	// Server-wide concurrency guard. Wait for a slot instead of rejecting:
	// Agent runs are long-lived (up to ~30 LLM rounds), so a 503 would
	// force the client to retry the whole task. Blocking keeps UX smooth
	// while still capping resource/LLM-quota pressure.
	select {
	case h.agentSem <- struct{}{}:
	case <-c.Context().Done():
		return nil
	}
	defer func() { <-h.agentSem }()

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
		h.cfg.Lock()
		saved := h.cfg.LLMProvider
		h.cfg.LLMProvider = req.ProviderID
		runCfg.LLMApiKey = h.cfg.EffectiveLLMKey()
		h.cfg.LLMProvider = saved
		h.cfg.Unlock()
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
		// Not a preset provider — try custom_providers table first, then fall back to global config
		runCfg.ProviderID = req.ProviderID
		if h.db != nil {
			var cpEndpoint, cpKey, cpModel string
			// Try by name first, then by UUID id
			err := h.db.Conn.QueryRow(
				"SELECT endpoint, api_key, COALESCE(model_id,'') FROM custom_providers WHERE name=? AND user_id=?",
				req.ProviderID, uid,
			).Scan(&cpEndpoint, &cpKey, &cpModel)
			if err != nil {
				err = h.db.Conn.QueryRow(
					"SELECT endpoint, api_key, COALESCE(model_id,'') FROM custom_providers WHERE id=? AND user_id=?",
					req.ProviderID, uid,
				).Scan(&cpEndpoint, &cpKey, &cpModel)
			}
			if err == nil && cpEndpoint != "" {
				runCfg.LLMEndpoint = cpEndpoint
				runCfg.LLMModel = req.Model
				if cpModel != "" && req.Model == "" {
					runCfg.LLMModel = cpModel
				}
				// Decode base64-encoded API key if needed
				runCfg.LLMApiKey = cpKey
				if cpKey != "" {
					if decoded, dErr := base64.StdEncoding.DecodeString(cpKey); dErr == nil {
						runCfg.LLMApiKey = string(decoded)
					}
				}
				log.Printf("[Agent] resolved custom provider=%s endpoint=%s model=%s key_len=%d",
					req.ProviderID, runCfg.LLMEndpoint, runCfg.LLMModel, len(runCfg.LLMApiKey))
			} else {
				// Fallback: first try loading from llm_config DB, then in-memory config
				resolved := false
				if h.db != nil {
					var llmProvider, llmModelID, llmEndpoint string
					if err := h.db.Conn.QueryRow(
						"SELECT provider, model_id, endpoint FROM llm_config WHERE id='default'",
					).Scan(&llmProvider, &llmModelID, &llmEndpoint); err == nil && llmProvider != "" {
						// Check if the configured provider is a preset
						if preset := llm.FindProvider(llmProvider); preset != nil {
							// P0-Fix: Don't overwrite runCfg.ProviderID with the llm_config preset.
							// Keep the original req.ProviderID so callLLMSummary (compact/plan)
							// resolves the correct custom provider, not the free preset.
							runCfg.LLMEndpoint = preset.Endpoint
							if req.Model != "" {
								runCfg.LLMModel = req.Model
							} else if llmModelID != "" {
								runCfg.LLMModel = llmModelID
							} else if len(preset.Models) > 0 {
								runCfg.LLMModel = preset.Models[0].ID
							}
							h.cfg.Lock()
							saved := h.cfg.LLMProvider
							h.cfg.LLMProvider = llmProvider
							runCfg.LLMApiKey = h.cfg.EffectiveLLMKey()
							h.cfg.LLMProvider = saved
							h.cfg.Unlock()
							resolved = true
							log.Printf("[Agent] fallback: loaded from llm_config provider=%s endpoint=%s model=%s key_len=%d (kept original providerID=%s)",
								llmProvider, runCfg.LLMEndpoint, runCfg.LLMModel, len(runCfg.LLMApiKey), runCfg.ProviderID)
						} else {
							// Configured provider is custom — look it up
							var cpEndpoint, cpKey, cpModel string
							cpErr := h.db.Conn.QueryRow(
								"SELECT endpoint, api_key, COALESCE(model_id,'') FROM custom_providers WHERE name=? AND user_id=?",
								llmProvider, uid,
							).Scan(&cpEndpoint, &cpKey, &cpModel)
							if cpErr != nil {
								cpErr = h.db.Conn.QueryRow(
									"SELECT endpoint, api_key, COALESCE(model_id,'') FROM custom_providers WHERE id=? AND user_id=?",
									llmProvider, uid,
								).Scan(&cpEndpoint, &cpKey, &cpModel)
							}
							if cpErr == nil && cpEndpoint != "" {
								runCfg.ProviderID = llmProvider
								runCfg.LLMEndpoint = cpEndpoint
								if req.Model != "" {
									runCfg.LLMModel = req.Model
								} else if cpModel != "" {
									runCfg.LLMModel = cpModel
								}
								runCfg.LLMApiKey = cpKey
								if cpKey != "" {
									if decoded, dErr := base64.StdEncoding.DecodeString(cpKey); dErr == nil {
										runCfg.LLMApiKey = string(decoded)
									}
								}
								resolved = true
								log.Printf("[Agent] fallback: loaded custom provider=%s endpoint=%s model=%s key_len=%d",
									llmProvider, runCfg.LLMEndpoint, runCfg.LLMModel, len(runCfg.LLMApiKey))
							}
						}
					}
				}
				if !resolved {
					// Final fallback: use the in-memory config (set by UpdateLLMConfig)
					runCfg.LLMEndpoint = h.cfg.LLMEndpoint
					runCfg.LLMModel = h.cfg.LLMModel
					runCfg.LLMApiKey = h.cfg.EffectiveLLMKey()
					log.Printf("[Agent] provider=%s not in presets or custom_providers, using in-memory config endpoint=%s model=%s",
						req.ProviderID, runCfg.LLMEndpoint, runCfg.LLMModel)
				}
			}
		}
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
				if err := service.SaveConversationMessage(h.db.Conn, req.SessionID, uid, "assistant", capturedAnswer, currentRound, map[string]string{
					"token_usage": captureW.tokenUsageJSON,
				}); err != nil {
					log.Printf("[Agent] PERSIST ERROR assistant: %v", err)
				} else {
					log.Printf("[Agent] PERSIST OK assistant round=%d len=%d", currentRound, len(capturedAnswer))
					// Update ai_conversations.token_usage for unified session-level aggregation
					if captureW.tokenUsageJSON != "" {
						h.db.Conn.Exec(
							"UPDATE ai_conversations SET token_usage = COALESCE(token_usage, 0) + (SELECT COALESCE(SUM(CAST(json_extract(token_usage, '$.total_tokens') AS INTEGER)), 0) FROM conversation_messages WHERE session_id = ? AND user_id = ? AND token_usage IS NOT NULL AND token_usage != '' ), updated_at = datetime('now') WHERE id = ? AND user_id = ?",
							req.SessionID, uid, req.SessionID, uid)
					}
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

	// Server-wide concurrency guard (same as Agent.Run).
	select {
	case h.agentSem <- struct{}{}:
	case <-c.Context().Done():
		return nil
	}
	defer func() { <-h.agentSem }()

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
