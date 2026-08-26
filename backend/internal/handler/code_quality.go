package handler

import (
	"github.com/gofiber/fiber/v3"
)

// HandleValidateCode 验证代码质量
func (h *AIHandler) HandleValidateCode(c fiber.Ctx) error {
	var req struct {
		Code     string `json:"code"`
		Language string `json:"language"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if req.Code == "" {
		return BadRequest(c, "Code is required")
	}

	if req.Language == "" {
		req.Language = "go"
	}

	result := h.runner.CodeQualityValidator().Validate(req.Code, req.Language)
	return c.Status(200).JSON(result)
}
