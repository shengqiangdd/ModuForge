package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type BenchmarkAPIHandler struct {
	benchmark *service.BenchmarkService
	adb       *service.ADBService
}

func NewBenchmarkAPIHandler(bench *service.BenchmarkService, adb *service.ADBService) *BenchmarkAPIHandler {
	return &BenchmarkAPIHandler{benchmark: bench, adb: adb}
}

// POST /api/v1/benchmark/run
func (h *BenchmarkAPIHandler) RunBenchmark(c fiber.Ctx) error {
	var req struct {
		ModuleID string `json:"module_id"`
		Serial   string `json:"serial"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	before, err := h.adb.BenchmarkDevice(c.Context(), req.Serial)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "benchmark before: " + err.Error()})
	}

	after := make(map[string]interface{})
	for k, v := range before {
		after[k] = v
	}

	diff := make(map[string]interface{})
	diff["note"] = "Install module then re-run benchmark to see actual differences"

	result := &service.BenchmarkResult{
		ModuleID: req.ModuleID,
		DeviceSN: req.Serial,
		Before:   before,
		After:    after,
		Diff:     diff,
	}

	if err := h.benchmark.SaveBenchmark(c.Context(), result); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(result)
}

// GET /api/v1/benchmark/history?module_id=X
func (h *BenchmarkAPIHandler) GetHistory(c fiber.Ctx) error {
	moduleID := c.Query("module_id")
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if limit <= 0 {
		limit = 20
	}

	results, err := h.benchmark.GetBenchmarkHistory(c.Context(), moduleID, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"results": results})
}
