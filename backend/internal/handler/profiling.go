package handler

import (
	"github.com/gofiber/fiber/v3"
)

// HandleGetProfilerMetrics 获取性能指标
func (h *AIHandler) HandleGetProfilerMetrics(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Runner not initialized"})
	}

	metrics := h.runner.Profiler().GetAllMetrics()
	return c.Status(200).JSON(metrics)
}

// HandleGetMemoryProfile 获取内存概况
func (h *AIHandler) HandleGetMemoryProfile(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Runner not initialized"})
	}

	profile := h.runner.Profiler().GetMemoryProfile()
	return c.Status(200).JSON(profile)
}

// HandleResetProfiler 重置性能指标
func (h *AIHandler) HandleResetProfiler(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Runner not initialized"})
	}

	h.runner.Profiler().Reset()
	return c.Status(200).JSON(fiber.Map{"success": true})
}
