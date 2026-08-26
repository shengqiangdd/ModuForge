package handler

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/code"
)

var templateManager = code.NewTemplateManager()
var snippetLibrary = code.NewSnippetLibrary()

// HandleListTemplates 列出模板
func (h *AIHandler) HandleListTemplates(c fiber.Ctx) error {
	templates := templateManager.ListTemplates()
	return c.Status(200).JSON(fiber.Map{
		"templates": templates,
		"total":     len(templates),
	})
}

// HandleGetTemplate 获取模板
func (h *AIHandler) HandleGetTemplate(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return BadRequest(c, "Template ID is required")
	}

	t, ok := templateManager.GetTemplate(id)
	if !ok {
		return NotFound(c, "Template not found")
	}

	return c.Status(200).JSON(t)
}

// HandleGenerateFromTemplate 从模板生成项目
func (h *AIHandler) HandleGenerateFromTemplate(c fiber.Ctx) error {
	type request struct {
		TemplateID string            `json:"template_id"`
		Variables  map[string]string `json:"variables"`
	}

	var req request
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if req.TemplateID == "" {
		return BadRequest(c, "Template ID is required")
	}

	files, err := templateManager.GenerateProject(req.TemplateID, req.Variables)
	if err != nil {
		return c.Status(200).JSON(fiber.Map{
			"error": err.Error(),
			"valid": false,
		})
	}

	return c.Status(200).JSON(fiber.Map{
		"valid": true,
		"files": files,
	})
}

// HandleListSnippets 列出代码片段
func (h *AIHandler) HandleListSnippets(c fiber.Ctx) error {
	snippets := snippetLibrary.ListSnippets()
	return c.Status(200).JSON(fiber.Map{
		"snippets": snippets,
		"total":    len(snippets),
	})
}

// HandleSearchSnippets 搜索代码片段
func (h *AIHandler) HandleSearchSnippets(c fiber.Ctx) error {
	query := c.Query("q", "")
	language := c.Query("language", "")

	snippets := snippetLibrary.SearchSnippets(query, language)
	return c.Status(200).JSON(fiber.Map{
		"snippets": snippets,
		"total":    len(snippets),
	})
}

// HandleGetSnippet 获取代码片段
func (h *AIHandler) HandleGetSnippet(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return BadRequest(c, "Snippet ID is required")
	}

	s, ok := snippetLibrary.GetSnippet(id)
	if !ok {
		return NotFound(c, "Snippet not found")
	}

	return c.Status(200).JSON(s)
}

// HandleUseSnippet 使用代码片段
func (h *AIHandler) HandleUseSnippet(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return BadRequest(c, "Snippet ID is required")
	}

	if err := snippetLibrary.UseSnippet(id); err != nil {
		return NotFound(c, err.Error())
	}

	return c.Status(200).JSON(fiber.Map{
		"message": "Snippet usage recorded",
	})
}
