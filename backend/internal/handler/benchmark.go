package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type BenchmarkHandler struct {
	adb *service.ADBService
}

func NewBenchmarkHandler(adb *service.ADBService) *BenchmarkHandler {
	return &BenchmarkHandler{adb: adb}
}

// POST /api/v1/adb/benchmark
func (h *BenchmarkHandler) Benchmark(c fiber.Ctx) error {
	var req struct {
		Serial string `json:"serial"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}

	result, err := h.adb.BenchmarkDevice(c.Context(), req.Serial)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}
