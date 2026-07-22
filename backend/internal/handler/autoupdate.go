package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type UpdateHandler struct {
	update *service.UpdateService
}

func NewUpdateHandler(update *service.UpdateService) *UpdateHandler {
	return &UpdateHandler{update: update}
}

// POST /api/v1/update/check
func (h *UpdateHandler) CheckUpdate(c fiber.Ctx) error {
	var req struct {
		ModuleID       string `json:"module_id"`
		CurrentVersion string `json:"current_version"`
		RepoURL        string `json:"repo_url"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	info, err := h.update.CheckModuleUpdate(c.Context(), req.ModuleID, req.CurrentVersion, req.RepoURL)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(info)
}

// POST /api/v1/update/check-all
func (h *UpdateHandler) CheckAllUpdates(c fiber.Ctx) error {
	var req struct {
		Modules []struct {
			ID, Version, RepoURL string
		} `json:"modules"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	results := h.update.CheckAllModuleUpdates(c.Context(), req.Modules)
	return c.JSON(fiber.Map{"updates": results})
}
