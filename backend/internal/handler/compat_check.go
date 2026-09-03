package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type CompatCheckHandler struct {
	svc *service.CompatibilityChecker
}

func NewCompatCheckHandler() *CompatCheckHandler {
	return &CompatCheckHandler{svc: service.NewCompatibilityChecker()}
}

type CompatCheckRequest struct {
	ProjectID string            `json:"project_id"`
	Files     map[string]string `json:"files"`
}

func (h *CompatCheckHandler) Check(c fiber.Ctx) error {
	var req CompatCheckRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	if len(req.Files) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "files are required"})
	}

	result, err := h.svc.CheckCompatibility("", req.Files)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(result)
}
