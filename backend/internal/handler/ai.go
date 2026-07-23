package handler

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/database"
	"github.com/moduforge/backend/internal/llm"
	"github.com/moduforge/backend/internal/service"
)

func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type AIHandler struct {
	svc *service.AIService
	cfg *config.Config
	db  *database.DB
}

func NewAIHandler(svc *service.AIService, cfg *config.Config, db *database.DB) *AIHandler {
	return &AIHandler{svc: svc, cfg: cfg, db: db}
}

func (h *AIHandler) GenerateModule(c fiber.Ctx) error {
	var req struct {
		Description string `json:"description"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		slog.Error("GenerateModule: bind failed", "error", err)
		return BadRequest(c, "invalid request")
	}
	if req.Description == "" {
		slog.Warn("GenerateModule: empty description")
		return BadRequest(c, "description required")
	}
	if len(req.Description) > 500 {
		slog.Warn("GenerateModule: description too long", "len", len(req.Description))
		return BadRequest(c, "description too long (max 500)")
	}

	uid, _ := c.Locals("uid").(string)
	slog.Info("GenerateModule", "description_len", len(req.Description), "uid", uid)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("X-Accel-Buffering", "no")

	if err := h.svc.GenerateModule(c.Context(), req.Description, uid, c); err != nil {
		return InternalError(c, err.Error())
	}
	return nil
}

func (h *AIHandler) Chat(c fiber.Ctx) error {
	var req struct {
		Message string `json:"message"`
		Context string `json:"context"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		slog.Error("Chat: bind failed", "error", err)
		return BadRequest(c, "invalid request")
	}
	if req.Message == "" {
		slog.Warn("Chat: empty message")
		return BadRequest(c, "message required")
	}
	if len(req.Message) > 2000 {
		slog.Warn("Chat: message too long", "len", len(req.Message))
		return BadRequest(c, "message too long (max 2000)")
	}

	uid, _ := c.Locals("uid").(string)
	slog.Info("Chat", "message_len", len(req.Message), "uid", uid)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("X-Accel-Buffering", "no")

	if err := h.svc.Chat(c.Context(), req.Message, req.Context, uid, c); err != nil {
		return InternalError(c, err.Error())
	}
	return nil
}

func (h *AIHandler) RepairBuild(c fiber.Ctx) error {
	var req struct {
		BuildLog string `json:"build_log"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.BuildLog == "" {
		return BadRequest(c, "build_log required")
	}
	if len(req.BuildLog) > 50000 {
		return BadRequest(c, "build_log too long (max 50000)")
	}

	uid, _ := c.Locals("uid").(string)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("X-Accel-Buffering", "no")

	if err := h.svc.RepairBuild(c.Context(), req.BuildLog, uid, c); err != nil {
		return InternalError(c, err.Error())
	}
	return nil
}

// ListProviders 返回所有可用的 LLM 提供商和模型（合并用户配置）
func (h *AIHandler) ListProviders(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)

	var userConfigs map[string]struct{ Endpoint, APIKey string }
	var customProviders []llm.Provider

	if uid != "" && h.db != nil {
		configs, err := h.db.GetProviderConfigs(uid)
		if err == nil {
			userConfigs = make(map[string]struct{ Endpoint, APIKey string })
			for _, pc := range configs {
				userConfigs[pc.ID] = struct{ Endpoint, APIKey string }{Endpoint: pc.Endpoint, APIKey: pc.APIKey}
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
	effectiveKey := h.cfg.EffectiveLLMKey()
	keyConfigured := effectiveKey != ""

	return c.JSON(fiber.Map{
		"provider": h.cfg.LLMProvider,
		"model_id": h.cfg.LLMModelID,
		// Legacy fields for backward compatibility
		"legacy_endpoint": h.cfg.LLMEndpoint,
		"legacy_model":    h.cfg.LLMModel,
		"key_configured":  keyConfigured,
		// Don't expose actual keys
	})
}

func (h *AIHandler) GetPrompts(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	prompts, err := h.svc.GetPrompts(uid)
	if err != nil {
		slog.Error("GetPrompts failed", "error", err)
		return InternalError(c, "failed to load prompts")
	}
	return c.JSON(fiber.Map{"prompts": prompts})
}

func (h *AIHandler) UpdatePrompt(c fiber.Ctx) error {
	var req struct {
		Mode    string `json:"mode"`
		Content string `json:"content"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Mode == "" || req.Content == "" {
		return BadRequest(c, "mode and content required")
	}
	if len(req.Mode) > 50 {
		return BadRequest(c, "mode too long (max 50)")
	}
	if len(req.Content) > 5000 {
		return BadRequest(c, "content too long (max 5000)")
	}
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	if err := h.svc.UpdatePrompt(req.Mode, req.Content, uid); err != nil {
		slog.Error("UpdatePrompt failed", "mode", req.Mode, "error", err)
		return InternalError(c, "failed to save prompt")
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AIHandler) ResetPrompt(c fiber.Ctx) error {
	mode := c.Params("mode")
	if mode == "" {
		return BadRequest(c, "mode required")
	}
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	if err := h.svc.ResetPrompt(mode, uid); err != nil {
		slog.Error("ResetPrompt failed", "mode", mode, "error", err)
		return InternalError(c, "failed to reset prompt")
	}
	return c.JSON(fiber.Map{"status": "ok"})
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

	// If not found in presets, check custom providers
	if provider == nil && uid != "" && h.db != nil {
		cp, err := h.db.GetCustomProvider(uid, req.Provider)
		if err == nil && cp != nil {
			var models []llm.Model
			if cp.ModelsJSON != "" {
				_ = json.Unmarshal([]byte(cp.ModelsJSON), &models)
			}
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

	// Validate model exists in provider
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
	if model == nil {
		return BadRequest(c, "model not found in provider: "+req.ModelID)
	}

	// Update runtime config
	h.cfg.LLMProvider = req.Provider
	h.cfg.LLMModelID = req.ModelID

	// Also update legacy fields for backward compatibility
	h.cfg.LLMEndpoint = provider.Endpoint
	h.cfg.LLMModel = req.ModelID
	h.cfg.LLMApiKey = h.cfg.EffectiveLLMKey()

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
		ID       string `json:"id"`
		Endpoint string `json:"endpoint"`
		APIKey   string `json:"api_key"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.ID == "" {
		return BadRequest(c, "provider id required")
	}

	if err := h.db.UpsertProviderConfig(uid, req.ID, req.Endpoint, req.APIKey); err != nil {
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
