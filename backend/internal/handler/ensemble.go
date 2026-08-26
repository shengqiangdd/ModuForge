package handler

import (
	"github.com/gofiber/fiber/v3"
)

// EnsembleGenerate runs the same prompt across multiple models.
func (h *AgentHandler) EnsembleGenerate(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	var req struct {
		Prompt string   `json:"prompt"`
		Models []string `json:"models"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if len(req.Models) == 0 {
		req.Models = []string{"deepseek-v4-flash", "qwen3.8-max"}
	}
	result, err := h.runner.EnsembleGenerate(req.Prompt, req.Models)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}
