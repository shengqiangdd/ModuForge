package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/llm"
	"github.com/moduforge/backend/internal/service"
)

type AIHandler struct {
	svc *service.AIService
	cfg *config.Config
}

func NewAIHandler(svc *service.AIService, cfg *config.Config) *AIHandler {
	return &AIHandler{svc: svc, cfg: cfg}
}

func (h *AIHandler) GenerateModule(c fiber.Ctx) error {
	var req struct {
		Description string `json:"description"`
		ModuleType  string `json:"module_type"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Description == "" {
		return c.Status(400).JSON(fiber.Map{"error": "description required"})
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")

	if err := h.svc.GenerateModule(c.Context(), req.Description, req.ModuleType, c); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return nil
}

func (h *AIHandler) Chat(c fiber.Ctx) error {
	var req struct {
		Message string `json:"message"`
		Context string `json:"context"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")

	if err := h.svc.Chat(c.Context(), req.Message, req.Context, c); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return nil
}

func (h *AIHandler) RepairBuild(c fiber.Ctx) error {
	var req struct {
		BuildLog string `json:"build_log"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")

	if err := h.svc.RepairBuild(c.Context(), req.BuildLog, c); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return nil
}

// ListProviders 返回所有可用的 LLM 提供商和模型
func (h *AIHandler) ListProviders(c fiber.Ctx) error {
	providers := llm.GetProviders()
	return c.JSON(fiber.Map{"providers": providers})
}

// RefreshModels 从远程 API 刷新模型列表，返回与本地配置的 diff
func (h *AIHandler) RefreshModels(c fiber.Ctx) error {
	remoteModels, err := llm.FetchRemoteModels()
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "failed to fetch remote models: " + err.Error()})
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

// UpdateLLMConfig 更新 LLM 提供商和模型配置
func (h *AIHandler) UpdateLLMConfig(c fiber.Ctx) error {
	var req struct {
		Provider string `json:"provider"`
		ModelID  string `json:"model_id"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.Provider == "" || req.ModelID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "provider and model_id required"})
	}

	// Validate provider exists
	provider := llm.FindProvider(req.Provider)
	if provider == nil {
		return c.Status(400).JSON(fiber.Map{"error": "unknown provider: " + req.Provider})
	}

	// Validate model exists in that provider
	model := llm.FindModel(req.Provider, req.ModelID)
	if model == nil {
		return c.Status(400).JSON(fiber.Map{"error": "model not found in provider: " + req.ModelID})
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
