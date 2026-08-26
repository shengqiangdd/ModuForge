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

	if req.Language == "" {
		req.Language = "go"
	}

	analyzer := code.NewMultiLangAnalyzer()
	result, err := analyzer.Analyze(req.Code, req.Language)
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

// HandleAnalyzeDependencies 分析依赖关系
func (h *AIHandler) HandleAnalyzeDependencies(c fiber.Ctx) error {
	type request struct {
		Files    map[string]string `json:"files"`
		Language string            `json:"language"`
	}

	var req request
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if len(req.Files) == 0 {
		return BadRequest(c, "Files are required")
	}

	if req.Language == "" {
		req.Language = "go"
	}

	analyzer := code.NewDependencyAnalyzer()
	graph := analyzer.AnalyzeDependencies(req.Files, req.Language)

	return c.Status(200).JSON(graph)
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
