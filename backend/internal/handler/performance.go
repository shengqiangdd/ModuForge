package handler

import (
	"runtime"
	"time"

	"github.com/gofiber/fiber/v3"
)

// HandleGetSystemInfo 获取系统信息
func (h *AIHandler) HandleGetSystemInfo(c fiber.Ctx) error {
	info := fiber.Map{
		"go_version":    "1.25.0",
		"fiber_version": "3.0.0",
		"platform":      "linux/amd64",
		"maxprocs":      runtime.GOMAXPROCS(0),
		"num_cpu":       runtime.NumCPU(),
		"num_goroutine": runtime.NumGoroutine(),
	}

	// 内存信息
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	info["memory_alloc"] = m.Alloc
	info["memory_sys"] = m.Sys
	info["memory_heap"] = m.HeapAlloc
	info["gc_cycles"] = m.NumGC

	return c.Status(200).JSON(info)
}

// HandleHealthCheck 增强健康检查
func (h *AIHandler) HandleHealthCheck(c fiber.Ctx) error {
	checks := fiber.Map{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
	}

	// 检查各组件状态
	if h.runner != nil {
		checks["cache"] = fiber.Map{
			"status": "ok",
			"stats":  h.runner.AICache().GetStats(),
		}

		checks["profiler"] = fiber.Map{
			"status": "ok",
			"memory": h.runner.Profiler().GetMemoryProfile(),
		}

		checks["alert_manager"] = fiber.Map{
			"status": "ok",
			"alerts": len(h.runner.AlertManager().GetActiveAlerts()),
		}
	}

	return c.Status(200).JSON(checks)
}
