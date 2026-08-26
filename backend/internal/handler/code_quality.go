package handler

import (
	"github.com/gofiber/fiber/v3"
)

// HandleValidateCode 验证代码质量
func (h *AgentHandler) HandleValidateCode(c *fiber.Ctx) error {
	var req struct {
		Code     string `json:"code"`
		Language string `json:"language"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString("Invalid request body")
	}

	if req.Code == "" {
		return c.Status(400).SendString("Code is required")
	}

	if req.Language == "" {
		req.Language = "go"
	}

	runner := h.runner
	if runner == nil || runner.CodeQualityValidator() == nil {
		return c.Status(503).SendString("Code quality validator not available")
	}

	result := runner.CodeQualityValidator().Validate(req.Code, req.Language)
	return c.JSON(result)
}
