package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/domain"
	"github.com/moduforge/backend/internal/service"
)

type BuildHandler struct {
	svc *service.BuildService
}

func NewBuildHandler(svc *service.BuildService) *BuildHandler {
	return &BuildHandler{svc: svc}
}

func (h *BuildHandler) Create(c fiber.Ctx) error {
	projectID := c.Params("id")
	var req domain.BuildRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Target == "" {
		req.Target = "universal"
	}
	task, err := h.svc.Create(c.Context(), projectID, req.Target)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(task)
}

func (h *BuildHandler) Get(c fiber.Ctx) error {
	id := c.Params("id")
	task, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(task)
}

func (h *BuildHandler) StreamLogs(c fiber.Ctx) error {
	id := c.Params("id")
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	task, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	// SSE stream: send existing logs then keep connection open
	if _, err := c.Write([]byte("data: " + task.Log + "\n\n")); err != nil {
		return err
	}
	// Keep alive — in production, use a channel to push log updates
	select {}
}

func (h *BuildHandler) Download(c fiber.Ctx) error {
	id := c.Params("id")
	path, err := h.svc.GetArtifact(c.Context(), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendFile(*path)
}
