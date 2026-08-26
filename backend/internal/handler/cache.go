package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/cache"
)

// HandleGetCacheStats 获取缓存统计
func (h *AIHandler) HandleGetCacheStats(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	stats := h.runner.AICache().GetStats()
	return c.Status(200).JSON(stats)
}

// HandleClearCache 清除缓存
func (h *AIHandler) HandleClearCache(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	h.runner.SetAICache(cache.NewAICache())
	return c.Status(200).JSON(fiber.Map{"success": true, "message": "Cache cleared"})
}
