package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type RepoHandler struct {
	svc *service.RepoService
}

func NewRepoHandler(svc *service.RepoService) *RepoHandler {
	return &RepoHandler{svc: svc}
}

func (h *RepoHandler) Fetch(c fiber.Ctx) error {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.URL == "" {
		return c.Status(400).JSON(fiber.Map{"error": "url required"})
	}

	info, err := h.svc.FetchRepoInfo(c.Context(), req.URL)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(info)
}

func (h *RepoHandler) FetchFiles(c fiber.Ctx) error {
	var req struct {
		URL  string `json:"url"`
		Path string `json:"path"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.URL == "" {
		return c.Status(400).JSON(fiber.Map{"error": "url required"})
	}

	files, err := h.svc.FetchRepoFiles(c.Context(), req.URL, req.Path)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(files)
}
