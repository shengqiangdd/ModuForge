package handler

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/code"
)

// HandleReviewCode 审查代码
func (h *AIHandler) HandleReviewCode(c fiber.Ctx) error {
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

	engine := code.NewReviewEngine()
	result, err := engine.ReviewCode(req.Code, req.Language)
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

// HandleOptimizeGeneration 优化代码生成
func (h *AIHandler) HandleOptimizeGeneration(c fiber.Ctx) error {
	type request struct {
		Code     string `json:"code"`
		Language string `json:"language"`
		Context  string `json:"context"`
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

	optimizer := code.NewGenerationOptimizer()
	suggestions := optimizer.OptimizeGeneration(req.Code, req.Language, req.Context)

	return c.Status(200).JSON(fiber.Map{
		"valid":       true,
		"suggestions": suggestions,
	})
}
