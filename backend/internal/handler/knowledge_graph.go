package handler

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/code"
)

// HandleGetKnowledgeGraph 获取代码知识图谱
func (h *AIHandler) HandleGetKnowledgeGraph(c fiber.Ctx) error {
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

	graph := code.NewKnowledgeGraph()
	graph.BuildGraph(req.Files, req.Language)

	return c.Status(200).JSON(fiber.Map{
		"valid": true,
		"graph": graph,
		"stats": graph.GetStats(),
	})
}

// HandleGetTrends 获取代码趋势
func (h *AIHandler) HandleGetTrends(c fiber.Ctx) error {
	analyzer := code.NewTrendAnalyzer()
	data := analyzer.GenerateSampleData()
	result := analyzer.AnalyzeTrends(data)

	return c.Status(200).JSON(fiber.Map{
		"valid":  true,
		"result": result,
	})
}
