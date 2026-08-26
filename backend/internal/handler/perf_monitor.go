package handler

import (
	"github.com/gofiber/fiber/v3"
)

// GetPerfSummary returns the current performance summary.
func (h *AgentHandler) GetPerfSummary(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	summary := h.runner.GetPerfSummary()
	return c.JSON(summary)
}

// GetPerfHistory returns the last N performance snapshots.
func (h *AgentHandler) GetPerfHistory(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	n := 60
	snaps := h.runner.GetPerfHistory(n)
	return c.JSON(snaps)
}

// GetModelStats returns LLM model performance stats.
func (h *AgentHandler) GetModelStats(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	stats := h.runner.GetModelStats()
	return c.JSON(stats)
}
