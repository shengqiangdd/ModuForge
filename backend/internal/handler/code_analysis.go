package handler

import (
	"encoding/json"
	
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/code"
)

// HandleAnalyzeCode 分析代码
func (h *AIHandler) HandleAnalyzeCode(c fiber.Ctx) error {
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

	if req.Language != "go" {
		return c.Status(200).JSON(fiber.Map{
			"message":  "AST analysis currently supports Go only",
			"language": req.Language,
		})
	}

	analyzer := code.NewASTAnalyzer()
	result, err := analyzer.Analyze(req.Code)
	if err != nil {
		return c.Status(200).JSON(fiber.Map{
			"error": err.Error(),
			"valid": false,
		})
	}

	return c.Status(200).JSON(fiber.Map{
		"valid":  true,
		"result": result,
	})
}

// HandleGetCodeMetrics 获取代码度量
func (h *AIHandler) HandleGetCodeMetrics(c fiber.Ctx) error {
	metrics := fiber.Map{
		"total_files":     0,
		"total_lines":     0,
		"total_functions": 0,
		"total_structs":   0,
		"avg_complexity":  0,
	}

	return c.Status(200).JSON(metrics)
}
