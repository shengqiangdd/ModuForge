package handler

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/code"
)

// HandleDiffCode 代码版本对比
func (h *AIHandler) HandleDiffCode(c fiber.Ctx) error {
	var req code.DiffRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if req.OldCode == "" && req.NewCode == "" {
		return BadRequest(c, "Both old_code and new_code are required")
	}

	engine := code.NewDiffEngine()
	result := engine.Compare(req)

	return c.Status(200).JSON(fiber.Map{
		"valid":  true,
		"result": result,
	})
}

// HandleRuntimeProfile 运行时性能分析
func (h *AIHandler) HandleRuntimeProfile(c fiber.Ctx) error {
	profiler := code.NewRuntimeProfiler()

	// 获取多个快照
	for i := 0; i < 5; i++ {
		profiler.TakeSnapshot()
	}

	result := profiler.GetProfile()

	return c.Status(200).JSON(fiber.Map{
		"valid":  true,
		"result": result,
	})
}

// HandleSecurityScan 深度安全扫描
func (h *AIHandler) HandleSecurityScan(c fiber.Ctx) error {
	type request struct {
		Code     string `json:"code"`
		Language string `json:"language"`
	}

	var req request
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if req.Code == "" {
		return BadRequest(c, "Code is required")
	}

	if req.Language == "" {
		req.Language = "go"
	}

	scanner := code.NewSecurityScanner()
	result := scanner.ScanCode(req.Code, req.Language)

	return c.Status(200).JSON(fiber.Map{
		"valid":  true,
		"result": result,
	})
}
