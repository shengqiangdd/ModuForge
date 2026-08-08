package handler

import (
	"log"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/agent/prompts"
)

// MDPromptsHandler handles MD-based prompt management
type MDPromptsHandler struct{}

// NewMDPromptsHandler creates a new handler
func NewMDPromptsHandler() *MDPromptsHandler {
	return &MDPromptsHandler{}
}

// ListMDPrompts returns all available MD prompt files
func (h *MDPromptsHandler) ListMDPrompts(c fiber.Ctx) error {
	promptList := prompts.ListAllPrompts()
	return c.JSON(fiber.Map{
		"prompts": promptList,
		"count":   len(promptList),
	})
}

// GetMDPrompt returns the content of a specific MD file
func (h *MDPromptsHandler) GetMDPrompt(c fiber.Ctx) error {
	name := c.Params("name")
	if name == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "name parameter required",
		})
	}

	content, err := prompts.GetPrompt(name)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "prompt not found",
		})
	}

	return c.JSON(fiber.Map{
		"name":    name,
		"content": content,
	})
}

// UpdateMDPrompt updates a MD prompt file
func (h *MDPromptsHandler) UpdateMDPrompt(c fiber.Ctx) error {
	name := c.Params("name")
	if name == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "name parameter required",
		})
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.Content == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "content required",
		})
	}

	// Validate content length (prevent abuse)
	if len(req.Content) > 50000 {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "content too long (max 50000 characters)",
		})
	}

	if err := prompts.UpdatePrompt(name, req.Content); err != nil {
		log.Printf("Failed to update prompt %s: %v", name, err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to update prompt",
		})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"name":   name,
	})
}

// ResetMDPrompt resets a MD prompt to its default content
func (h *MDPromptsHandler) ResetMDPrompt(c fiber.Ctx) error {
	name := c.Params("name")
	if name == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "name parameter required",
		})
	}

	if err := prompts.ResetPrompt(name); err != nil {
		log.Printf("Failed to reset prompt %s: %v", name, err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to reset prompt",
		})
	}

	// Get the default content
	content, _ := prompts.GetPrompt(name)
	return c.JSON(fiber.Map{
		"status":  "ok",
		"name":    name,
		"content": content,
	})
}

// ReloadMDPrompts forces a reload of all MD prompts
func (h *MDPromptsHandler) ReloadMDPrompts(c fiber.Ctx) error {
	prompts.Reload()
	return c.JSON(fiber.Map{
		"status":  "ok",
		"message": "prompts reloaded",
	})
}

// RegisterMDPromptRoutes registers the MD prompt routes
func RegisterMDPromptRoutes(api fiber.Router, handler *MDPromptsHandler) {
	// MD-based prompts (for viewing/editing embedded prompts)
	api.Get("/md-prompts", handler.ListMDPrompts)
	api.Get("/md-prompts/:name", handler.GetMDPrompt)
	api.Put("/md-prompts/:name", handler.UpdateMDPrompt)
	api.Post("/md-prompts/:name/reset", handler.ResetMDPrompt)
	api.Post("/md-prompts/reload", handler.ReloadMDPrompts)
}
