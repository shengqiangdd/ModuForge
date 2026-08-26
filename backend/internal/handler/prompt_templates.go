package handler

import (
	"github.com/gofiber/fiber/v3"
)

// ListPromptTemplates returns all available prompt templates.
func (h *AgentHandler) ListPromptTemplates(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	category := c.Query("category", "")
	templates := h.runner.ListPromptTemplates(category)
	return c.JSON(templates)
}

// SelectPromptTemplate selects the best template for a task.
func (h *AgentHandler) SelectPromptTemplate(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	var req struct {
		Task string `json:"task"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	templateName := h.runner.SelectPromptTemplate(req.Task)
	return c.JSON(fiber.Map{"template": templateName})
}
