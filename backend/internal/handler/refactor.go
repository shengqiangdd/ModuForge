package handler

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/code"
)

// HandleGetRefactorSuggestions 获取重构建议
func (h *AIHandler) HandleGetRefactorSuggestions(c fiber.Ctx) error {
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

	engine := code.NewRefactorEngine()
	result, err := engine.AnalyzeRefactoring(req.Code, req.Language)
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

// HandleSearchCode 搜索代码
func (h *AIHandler) HandleSearchCode(c fiber.Ctx) error {
	type request struct {
		Files    map[string]string `json:"files"`
		Pattern  string            `json:"pattern"`
		Language string            `json:"language"`
		Type     string            `json:"type"`
	}

	var req request
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if req.Pattern == "" {
		return BadRequest(c, "Pattern is required")
	}

	if len(req.Files) == 0 {
		return BadRequest(c, "Files are required")
	}

	if req.Language == "" {
		req.Language = "go"
	}

	if req.Type == "" {
		req.Type = "all"
	}

	engine := code.NewSearchEngine()
	query := code.SearchQuery{
		Pattern:  req.Pattern,
		Language: req.Language,
		Type:     req.Type,
	}

	results := engine.Search(req.Files, query)

	return c.Status(200).JSON(fiber.Map{
		"total":   len(results),
		"results": results,
	})
}
