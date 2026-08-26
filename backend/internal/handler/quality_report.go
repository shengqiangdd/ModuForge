package handler

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/code"
)

// HandleGenerateQualityReport 生成质量报告
func (h *AIHandler) HandleGenerateQualityReport(c fiber.Ctx) error {
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

	analyzer := code.NewQualityAnalyzer()
	report, err := analyzer.AnalyzeProject(req.Files, req.Language)
	if err != nil {
		return c.Status(200).JSON(fiber.Map{
			"error": err.Error(),
			"valid": false,
		})
	}

	return c.Status(200).JSON(fiber.Map{
		"valid":  true,
		"report": report,
	})
}
