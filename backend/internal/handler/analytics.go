package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type AnalyticsHandler struct {
	analytics *service.AnalyticsService
}

func NewAnalyticsHandler(analytics *service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{analytics: analytics}
}

func (h *AnalyticsHandler) BuildStats(c fiber.Ctx) error {
	stats, err := h.analytics.GetBuildStats()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(stats)
}

func (h *AnalyticsHandler) BuildTrends(c fiber.Ctx) error {
	days, _ := strconv.Atoi(c.Query("days", "30"))
	trends, err := h.analytics.GetBuildTrends(days)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"trends": trends})
}

func (h *AnalyticsHandler) ModuleStats(c fiber.Ctx) error {
	stats, err := h.analytics.GetModuleStats()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(stats)
}

func (h *AnalyticsHandler) SystemStats(c fiber.Ctx) error {
	stats, err := h.analytics.GetSystemStats()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(stats)
}
