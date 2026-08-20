package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

// FeatureFlagHandler provides admin API for managing feature flags.
type FeatureFlagHandler struct {
	svc *service.FeatureFlagService
}

// NewFeatureFlagHandler creates a new handler.
func NewFeatureFlagHandler(svc *service.FeatureFlagService) *FeatureFlagHandler {
	return &FeatureFlagHandler{svc: svc}
}

// List returns all feature flags.
func (h *FeatureFlagHandler) List(c fiber.Ctx) error {
	flags := h.svc.List()
	return c.JSON(fiber.Map{"flags": flags})
}

// UpdateRequest is the request body for updating a feature flag.
type UpdateRequest struct {
	Enabled bool `json:"enabled"`
}

// Update toggles a single feature flag.
func (h *FeatureFlagHandler) Update(c fiber.Ctx) error {
	key := c.Params("key")
	var req UpdateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式无效"})
	}

	uid, _ := c.Locals("user_id").(string)

	if err := h.svc.SetEnabled(key, req.Enabled, uid); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "更新失败: " + err.Error()})
	}

	return c.JSON(fiber.Map{
		"key":     key,
		"enabled": req.Enabled,
		"status":  "ok",
	})
}

// BatchUpdateRequest is the request body for batch-updating feature flags.
type BatchUpdateRequest struct {
	Flags []struct {
		Key     string `json:"key"`
		Enabled bool   `json:"enabled"`
	} `json:"flags"`
}

// BatchUpdate toggles multiple feature flags in one request.
func (h *FeatureFlagHandler) BatchUpdate(c fiber.Ctx) error {
	var req BatchUpdateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求格式无效"})
	}
	if len(req.Flags) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "flags 列表不能为空"})
	}

	uid, _ := c.Locals("user_id").(string)

	if err := h.svc.SetEnabledBatch(req.Flags, uid); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "批量更新失败: " + err.Error()})
	}

	return c.JSON(fiber.Map{
		"updated": len(req.Flags),
		"status":  "ok",
	})
}
