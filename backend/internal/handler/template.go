package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type TemplateHandler struct {
	svc *service.TemplateService
}

func NewTemplateHandler(svc *service.TemplateService) *TemplateHandler {
	return &TemplateHandler{svc: svc}
}

func (h *TemplateHandler) List(c fiber.Ctx) error {
	c.Set("Cache-Control", "public, max-age=300")
	templates := h.svc.ListTemplates()
	return c.JSON(templates)
}

func (h *TemplateHandler) Get(c fiber.Ctx) error {
	name := c.Params("name")
	c.Set("Cache-Control", "public, max-age=300")
	tmpl, err := h.svc.GetTemplate(name)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(tmpl)
}

func (h *TemplateHandler) Recommend(c fiber.Ctx) error {
	var req struct {
		Description string `json:"description"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	templates := h.svc.RecommendByDescription(req.Description)
	return c.JSON(templates)
}
