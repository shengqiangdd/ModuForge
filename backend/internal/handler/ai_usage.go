package handler

import (
	"database/sql"
	"encoding/json"
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/database"
	"github.com/moduforge/backend/internal/llm"
	"github.com/moduforge/backend/internal/service"
)

// ListProviders 返回所有可用的 LLM 提供商和模型（合并用户配置）
func (h *AIHandler) ListProviders(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)

	var userConfigs map[string]struct{ Endpoint, APIKey, ModelsJSON string }
	var customProviders []llm.Provider

	if uid != "" && h.db != nil {
		configs, err := h.db.GetProviderConfigs(uid)
		if err == nil {
			userConfigs = make(map[string]struct{ Endpoint, APIKey, ModelsJSON string })
			for _, pc := range configs {
				userConfigs[pc.ID] = struct{ Endpoint, APIKey, ModelsJSON string }{Endpoint: pc.Endpoint, APIKey: pc.APIKey, ModelsJSON: pc.ModelsJSON}
			}
		}

		customList, err := h.db.GetCustomProviders(uid)
		if err == nil {
			for _, cp := range customList {
				var models []llm.Model
				if cp.ModelsJSON != "" {
					_ = json.Unmarshal([]byte(cp.ModelsJSON), &models)
				}
				customProviders = append(customProviders, llm.Provider{
					Name:        cp.Name,
					ID:          cp.ID,
					Endpoint:    cp.Endpoint,
					Models:      models,
					RequiresKey: cp.APIKey != "",
					IsFree:      false,
					Tier:        "paid",
				})
			}
		}
	}

	providers := llm.GetMergedProviders(userConfigs, customProviders)
	return c.JSON(fiber.Map{"providers": providers})
}

// RefreshModels 从远程 API 刷新模型列表，返回与本地配置的 diff
func (h *AIHandler) RefreshModels(c fiber.Ctx) error {
	remoteModels, err := llm.FetchRemoteModels()
	if err != nil {
		return ErrorResponse(c, 502, "failed to fetch remote models: "+err.Error(), "BAD_GATEWAY")
	}

	// Build set of locally known model IDs under opencode-zen
	providers := llm.GetProviders()
	localIDs := make(map[string]bool)
	for _, p := range providers {
		for _, m := range p.Models {
			if m.Provider == "opencode-zen" {
				localIDs[m.ID] = true
			}
		}
	}

	// Build set of remote model IDs
	remoteIDs := make(map[string]bool)
	var remoteList []string
	for _, rm := range remoteModels {
		remoteIDs[rm.ID] = true
		remoteList = append(remoteList, rm.ID)
	}

	// Diff: new models (in remote but not local) and removed (in local but not remote)
	var added, removed []string
	for id := range remoteIDs {
		if !localIDs[id] {
			added = append(added, id)
		}
	}
	for id := range localIDs {
		if !remoteIDs[id] {
			removed = append(removed, id)
		}
	}

	return c.JSON(fiber.Map{
		"status":      "ok",
		"total_remote": len(remoteModels),
		"total_local":  len(localIDs),
		"added":       added,
		"removed":     removed,
		"models":      remoteList,
	})
}

// GetLLMConfig 返回当前 LLM 配置
func (h *AIHandler) GetLLMConfig(c fiber.Ctx) error {
	// Try to load from database first (persists across restarts)
	if h.db != nil {
		var provider, modelID, endpoint string
		err := h.db.Conn.QueryRow(`SELECT provider, model_id, endpoint FROM llm_config WHERE id='default'`).Scan(&provider, &modelID, &endpoint)
		if err == nil && provider != "" {
			h.cfg.Lock()
			h.cfg.LLMProvider = provider
			h.cfg.LLMModelID = modelID
			h.cfg.LLMEndpoint = endpoint
			h.cfg.Unlock()
		}
	}

	h.cfg.RLock()
	effectiveKey := h.cfg.EffectiveLLMKey()
	keyConfigured := effectiveKey != ""

	resp := c.JSON(fiber.Map{
		"provider": h.cfg.LLMProvider,
		"model_id": h.cfg.LLMModelID,
		// Legacy fields for backward compatibility
		"legacy_endpoint": h.cfg.LLMEndpoint,
		"legacy_model":    h.cfg.LLMModel,
		"key_configured":  keyConfigured,
		// Don't expose actual keys
	})
	h.cfg.RUnlock()
	return resp
}

// UpdateLLMConfig 更新 LLM 提供商和模型配置
func (h *AIHandler) UpdateLLMConfig(c fiber.Ctx) error {
	var req struct {
		Provider string `json:"provider"`
		ModelID  string `json:"model_id"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		slog.Error("UpdateLLMConfig: bind failed", "error", err)
		return BadRequest(c, "invalid request")
	}

	slog.Info("UpdateLLMConfig", "provider", req.Provider, "model_id", req.ModelID)

	if req.Provider == "" || req.ModelID == "" {
		slog.Warn("UpdateLLMConfig: missing fields", "provider", req.Provider, "model_id", req.ModelID)
		return BadRequest(c, "provider and model_id required")
	}
	if len(req.Provider) > 50 {
		return BadRequest(c, "provider too long (max 50)")
	}
	if len(req.ModelID) > 100 {
		return BadRequest(c, "model_id too long (max 100)")
	}

	uid, _ := c.Locals("uid").(string)
	provider := llm.FindProvider(req.Provider)
	var customAPIKey string // track custom provider's API key separately

	// If not found in presets, check custom providers
	if provider == nil && uid != "" && h.db != nil {
		cp, err := h.db.GetCustomProvider(uid, req.Provider)
		if err == nil && cp != nil {
			var models []llm.Model
			if cp.ModelsJSON != "" {
				_ = json.Unmarshal([]byte(cp.ModelsJSON), &models)
			}
			customAPIKey = cp.APIKey
			provider = &llm.Provider{
				Name:        cp.Name,
				ID:          cp.ID,
				Endpoint:    cp.Endpoint,
				Models:      models,
				RequiresKey: cp.APIKey != "",
				IsFree:      false,
				Tier:        "paid",
			}
		}
	}

	if provider == nil {
		return BadRequest(c, "unknown provider: "+req.Provider)
	}

	// Validate model exists in provider (skip for custom providers with empty models)
	model := llm.FindModel(req.Provider, req.ModelID)
	if model == nil && provider != nil {
		for _, m := range provider.Models {
			if m.ID == req.ModelID {
				mCopy := m
				model = &mCopy
				break
			}
		}
	}
	if model == nil && len(provider.Models) > 0 {
		// Only reject if provider has a model list but the model isn't in it
		return BadRequest(c, "model not found in provider: "+req.ModelID)
	}

	// Update runtime config
	h.cfg.Lock()
	h.cfg.LLMProvider = req.Provider
	h.cfg.LLMModelID = req.ModelID

	// Also update legacy fields for backward compatibility
	h.cfg.LLMEndpoint = provider.Endpoint
	h.cfg.LLMModel = req.ModelID
	// Use custom provider's API key directly; EffectiveLLMKey() only knows built-in providers
	if customAPIKey != "" {
		h.cfg.LLMApiKey = customAPIKey
	} else {
		h.cfg.LLMApiKey = h.cfg.EffectiveLLMKey()
	}
	h.cfg.Unlock()

	// Persist to database so selection survives server restarts
	if h.db != nil {
		h.db.Conn.Exec(`INSERT INTO llm_config (id, provider, model_id, endpoint, updated_at)
			VALUES ('default', ?, ?, ?, datetime('now'))
			ON CONFLICT(id) DO UPDATE SET provider=excluded.provider, model_id=excluded.model_id,
			endpoint=excluded.endpoint, updated_at=excluded.updated_at`,
			req.Provider, req.ModelID, provider.Endpoint)
	}

	return c.JSON(fiber.Map{
		"status":   "ok",
		"provider": provider.Name,
		"model":    model.Name,
	})
}

// ---------- Provider Config Management ----------

func (h *AIHandler) SaveProviderConfig(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}

	var req struct {
		ID         string `json:"id"`
		Endpoint   string `json:"endpoint"`
		APIKey     string `json:"api_key"`
		ModelsJSON string `json:"models_json"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.ID == "" {
		return BadRequest(c, "provider id required")
	}

	if err := h.db.UpsertProviderConfig(uid, req.ID, req.Endpoint, req.APIKey, req.ModelsJSON); err != nil {
		slog.Error("SaveProviderConfig: upsert failed", "error", err)
		return InternalError(c, "failed to save config")
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AIHandler) GetProviderConfigs(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}

	configs, err := h.db.GetProviderConfigs(uid)
	if err != nil {
		slog.Error("GetProviderConfigs", "error", err)
		return InternalError(c, "failed to load configs")
	}

	return c.JSON(fiber.Map{"configs": configs})
}

func (h *AIHandler) DeleteProviderConfig(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}

	providerID := c.Params("id")
	if providerID == "" {
		return BadRequest(c, "provider id required")
	}

	if err := h.db.DeleteProviderConfig(uid, providerID); err != nil {
		slog.Error("DeleteProviderConfig", "error", err)
		return InternalError(c, "failed to delete config")
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// ---------- Custom Provider Management ----------

func (h *AIHandler) CreateCustomProvider(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}

	var req struct {
		Name       string `json:"name"`
		Endpoint   string `json:"endpoint"`
		APIKey     string `json:"api_key"`
		ModelsJSON string `json:"models_json"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Name == "" || req.Endpoint == "" {
		return BadRequest(c, "name and endpoint required")
	}

	provider := &database.CustomProvider{
		ID:         newUUID(),
		UserID:     uid,
		Name:       req.Name,
		Endpoint:   req.Endpoint,
		APIKey:     req.APIKey,
		ModelsJSON: req.ModelsJSON,
	}

	if err := h.db.CreateCustomProvider(provider); err != nil {
		slog.Error("CreateCustomProvider", "error", err)
		return InternalError(c, "failed to create provider")
	}

	return c.JSON(fiber.Map{"status": "ok", "id": provider.ID})
}

func (h *AIHandler) GetCustomProviders(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}

	providers, err := h.db.GetCustomProviders(uid)
	if err != nil {
		slog.Error("GetCustomProviders", "error", err)
		return InternalError(c, "failed to load providers")
	}

	return c.JSON(fiber.Map{"providers": providers})
}

func (h *AIHandler) UpdateCustomProvider(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}

	providerID := c.Params("id")
	if providerID == "" {
		return BadRequest(c, "provider id required")
	}

	var req struct {
		Name       string `json:"name"`
		Endpoint   string `json:"endpoint"`
		APIKey     string `json:"api_key"`
		ModelsJSON string `json:"models_json"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}

	provider := &database.CustomProvider{
		ID:         providerID,
		UserID:     uid,
		Name:       req.Name,
		Endpoint:   req.Endpoint,
		APIKey:     req.APIKey,
		ModelsJSON: req.ModelsJSON,
	}

	if err := h.db.UpdateCustomProvider(provider); err != nil {
		slog.Error("UpdateCustomProvider", "error", err)
		return InternalError(c, "failed to update provider")
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AIHandler) DeleteCustomProvider(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}

	providerID := c.Params("id")
	if providerID == "" {
		return BadRequest(c, "provider id required")
	}

	if err := h.db.DeleteCustomProvider(uid, providerID); err != nil {
		slog.Error("DeleteCustomProvider", "error", err)
		return InternalError(c, "failed to delete provider")
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// ---------- AI Capability Scoring ----------

// GetAICapability 返回当前配置的 AI 能力评分
func (h *AIHandler) GetAICapability(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)

	var db *sql.DB
	if h.db != nil {
		db = h.db.Conn
	}

	score := service.EvaluateAICapability(h.cfg, db, uid)
	return c.JSON(fiber.Map{"capability": score})
}

// GET /ai/memory — list all memory for the current user
func (h *AIHandler) ListMemory(c fiber.Ctx) error {
	if h.memoryStore == nil {
		return InternalError(c, "memory store not available")
	}
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "user not authenticated")
	}
	memType := c.Query("type")

	var entries []service.MemoryEntry
	var err error
	if memType != "" {
		entries, err = h.memoryStore.ListMemory(uid, memType)
	} else {
		entries, err = h.memoryStore.ListAllMemory(uid)
	}
	if err != nil {
		return InternalError(c, "failed to list memory: "+err.Error())
	}
	if entries == nil {
		entries = []service.MemoryEntry{}
	}
	return c.JSON(fiber.Map{"entries": entries})
}

// DELETE /ai/memory/:type/:key — delete a specific memory
func (h *AIHandler) DeleteMemory(c fiber.Ctx) error {
	if h.memoryStore == nil {
		return InternalError(c, "memory store not available")
	}
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "user not authenticated")
	}
	memType := c.Params("type")
	key := c.Params("key")
	if memType == "" || key == "" {
		return BadRequest(c, "type and key are required")
	}
	if err := h.memoryStore.DeleteMemory(uid, memType, key); err != nil {
		return InternalError(c, "failed to delete memory: "+err.Error())
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// DELETE /ai/memory — delete all memory for the current user
func (h *AIHandler) ClearMemory(c fiber.Ctx) error {
	if h.memoryStore == nil {
		return InternalError(c, "memory store not available")
	}
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "user not authenticated")
	}
	if err := h.memoryStore.DeleteAllMemory(uid); err != nil {
		return InternalError(c, "failed to clear memory: "+err.Error())
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AIHandler) GetProjectKnowledge(c fiber.Ctx) error {
	uid := h.getUserID(c)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	pid := c.Params("project_id")
	category := c.Query("category", "")
	entries, err := h.memV2.ListKnowledge(uid, pid, category)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(entries)
}

func (h *AIHandler) SaveProjectKnowledge(c fiber.Ctx) error {
	uid := h.getUserID(c)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	pid := c.Params("project_id")
	var body struct {
		Category string `json:"category"`
		Key      string `json:"key"`
		Value    string `json:"value"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	if body.Category == "" || body.Key == "" || body.Value == "" {
		return c.Status(400).JSON(fiber.Map{"error": "category, key, value required"})
	}
	if err := h.memV2.SaveKnowledge(uid, pid, body.Category, body.Key, body.Value); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

func (h *AIHandler) DeleteProjectKnowledge(c fiber.Ctx) error {
	uid := h.getUserID(c)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	pid := c.Params("project_id")
	cat := c.Params("category")
	key := c.Params("key")
	if err := h.memV2.DeleteKnowledge(uid, pid, cat, key); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

func (h *AIHandler) GetProjectSummaries(c fiber.Ctx) error {
	uid := h.getUserID(c)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	pid := c.Params("project_id")
	summaries, err := h.memV2.GetProjectSummaries(uid, pid, 20)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(summaries)
}

func (h *AIHandler) GenerateSummary(c fiber.Ctx) error {
	uid := h.getUserID(c)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	var body struct {
		SessionID string `json:"session_id"`
		ProjectID string `json:"project_id"`
		Messages  string `json:"messages"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	// Call LLM to generate summary
	prompt := "请简要总结以下对话的关键内容、做出的决定和改动的文件。用JSON格式返回：{\"summary\": \"...\", \"key_decisions\": [\"...\"], \"files_changed\": [\"...\"]}\n\n对话内容:\n" + body.Messages

	resp, err := h.callLLMForSummary(prompt, "你是一个精准的对话总结器。只输出JSON，不要其他内容。")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse LLM response
	var result struct {
		Summary      string   `json:"summary"`
		KeyDecisions []string `json:"key_decisions"`
		FilesChanged []string `json:"files_changed"`
	}
	json.Unmarshal([]byte(resp), &result)

	if err := h.memV2.SaveSummary(uid, body.SessionID, body.ProjectID, result.Summary, result.KeyDecisions, result.FilesChanged); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}
