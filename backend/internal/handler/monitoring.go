package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
)

// HandleGetAlerts 获取活跃告警
func (h *AIHandler) HandleGetAlerts(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	alerts := h.runner.AlertManager().GetActiveAlerts()
	return c.Status(200).JSON(alerts)
}

// HandleGetAlertHistory 获取告警历史
func (h *AIHandler) HandleGetAlertHistory(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	limit, _ := strconv.Atoi(c.Query("n", "50"))
	alerts := h.runner.AlertManager().GetAlertHistory(limit)
	return c.Status(200).JSON(alerts)
}

// HandleResolveAlert 解决告警
func (h *AIHandler) HandleResolveAlert(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	alertID := c.Params("id")
	if alertID == "" {
		return BadRequest(c, "Alert ID is required")
	}

	resolved := h.runner.AlertManager().ResolveAlert(alertID)
	if !resolved {
		return NotFound(c, "Alert not found or already resolved")
	}

	return c.Status(200).JSON(fiber.Map{"success": true})
}

// HandleGetLogStats 获取日志统计
func (h *AIHandler) HandleGetLogStats(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	stats := h.runner.LogAggregator().GetStats()
	return c.Status(200).JSON(stats)
}

// HandleSearchLogs 搜索日志
func (h *AIHandler) HandleSearchLogs(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	query := c.Query("q", "")
	level := c.Query("level", "")
	limit, _ := strconv.Atoi(c.Query("limit", "100"))

	logs := h.runner.LogAggregator().Search(query, level, limit)
	return c.Status(200).JSON(logs)
}
