package handler

import (
	"context"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/agent/registry"
)

// SkillsHandler handles skill management
type SkillsHandler struct {
	registry *registry.SkillRegistry
}

// NewSkillsHandler creates a new handler
func NewSkillsHandler(reg *registry.SkillRegistry) *SkillsHandler {
	return &SkillsHandler{registry: reg}
}

// SkillInfo holds metadata about a skill
type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsBuiltin   bool   `json:"is_builtin"`
}

// ListSkills returns all available skills
func (h *SkillsHandler) ListSkills(c fiber.Ctx) error {
	if h.registry == nil {
		return c.JSON(fiber.Map{
			"skills": []SkillInfo{},
			"count":  0,
		})
	}

	skills := h.registry.List()
	var skillList []SkillInfo

	for _, skill := range skills {
		skillList = append(skillList, SkillInfo{
			Name:        skill.Name(),
			Description: skill.Description(),
			IsBuiltin:   true, // All registered skills are builtin
		})
	}

	return c.JSON(fiber.Map{
		"skills": skillList,
		"count":  len(skillList),
	})
}

// GetSkill returns details about a specific skill
func (h *SkillsHandler) GetSkill(c fiber.Ctx) error {
	name := c.Params("name")
	if name == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "name parameter required",
		})
	}

	if h.registry == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "skill not found",
		})
	}

	skill, err := h.registry.Get(name)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "skill not found",
		})
	}

	return c.JSON(fiber.Map{
		"name":        skill.Name(),
		"description": skill.Description(),
		"is_builtin":  true,
	})
}

// ExecuteSkill executes a skill with the given input
func (h *SkillsHandler) ExecuteSkill(c fiber.Ctx) error {
	name := c.Params("name")
	if name == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "name parameter required",
		})
	}

	var req struct {
		Input map[string]interface{} `json:"input"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if h.registry == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "skill not found",
		})
	}

	skill, err := h.registry.Get(name)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "skill not found",
		})
	}

	result, err := skill.Execute(c.Context(), req.Input)
	if err != nil {
		log.Printf("Failed to execute skill %s: %v", name, err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to execute skill",
		})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"result": result,
	})
}

// RegisterSkillRoutes registers the skill routes
func RegisterSkillRoutes(api fiber.Router, handler *SkillsHandler) {
	// Skills API
	api.Get("/skills", handler.ListSkills)
	api.Get("/skills/:name", handler.GetSkill)
	api.Post("/skills/:name/execute", handler.ExecuteSkill)
}
