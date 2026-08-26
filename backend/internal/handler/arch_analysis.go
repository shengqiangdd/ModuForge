package handler

import (
	"github.com/gofiber/fiber/v3"
)

// AnalyzeProject performs architecture analysis on the project.
func (h *AgentHandler) AnalyzeProject(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	projectDir := c.Query("dir", ".")
	report, err := h.runner.AnalyzeProject(projectDir)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(report)
}
