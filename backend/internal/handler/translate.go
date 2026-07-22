package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type TranslateHandler struct {
	svc *service.TranslateService
}

func NewTranslateHandler(svc *service.TranslateService) *TranslateHandler {
	return &TranslateHandler{svc: svc}
}

func (h *TranslateHandler) Translate(c fiber.Ctx) error {
	var req struct {
		Text       string `json:"text"`
		SourceLang string `json:"source_lang"`
		TargetLang string `json:"target_lang"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Text == "" || req.TargetLang == "" {
		return c.Status(400).JSON(fiber.Map{"error": "text and target_lang required"})
	}

	result, err := h.svc.Translate(c.Context(), req.Text, req.SourceLang, req.TargetLang)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"translated_text": result})
}

func (h *TranslateHandler) TranslateProps(c fiber.Ctx) error {
	var req struct {
		Props      map[string]string `json:"props"`
		TargetLang string            `json:"target_lang"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.TargetLang == "" {
		return c.Status(400).JSON(fiber.Map{"error": "target_lang required"})
	}

	result, err := h.svc.TranslateModuleProps(c.Context(), req.Props, req.TargetLang)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}
