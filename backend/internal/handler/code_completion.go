package handler

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/code"
)

// HandleGetCompletions 获取代码补全
func (h *AIHandler) HandleGetCompletions(c fiber.Ctx) error {
	var req code.CompletionRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if req.Language == "" {
		req.Language = "go"
	}

	engine := code.NewCompletionEngine()
	result := engine.GetCompletions(req)

	return c.Status(200).JSON(result)
}
