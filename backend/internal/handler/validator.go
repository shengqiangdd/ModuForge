package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type ValidatorHandler struct {
	validator *service.ValidatorService
}

func NewValidatorHandler(validator *service.ValidatorService) *ValidatorHandler {
	return &ValidatorHandler{validator: validator}
}

type ValidateRequest struct {
	Files map[string]string `json:"files"`
}

type ValidateFileRequest struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

func (h *ValidatorHandler) ValidateFiles(c fiber.Ctx) error {
	var req ValidateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	results := h.validator.ValidateProject(req.Files)
	return c.JSON(fiber.Map{"results": results})
}

func (h *ValidatorHandler) ValidateFile(c fiber.Ctx) error {
	var req ValidateFileRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	result := h.validator.ValidateFile(req.Filename, req.Content)
	return c.JSON(result)
}
