package handler

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

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
