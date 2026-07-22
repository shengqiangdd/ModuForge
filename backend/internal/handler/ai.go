package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type AIHandler struct {
	svc *service.AIService
}

func NewAIHandler(svc *service.AIService) *AIHandler {
	return &AIHandler{svc: svc}
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
