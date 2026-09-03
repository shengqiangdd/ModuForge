package handler

import (
	"bufio"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

// BuildSSEHandler streams build progress events as Server-Sent Events.
type BuildSSEHandler struct {
	svc *service.BuildService
}

// NewBuildSSEHandler creates a new BuildSSEHandler.
func NewBuildSSEHandler(svc *service.BuildService) *BuildSSEHandler {
	return &BuildSSEHandler{svc: svc}
}

// Stream connects an SSE client to build progress events for a project.
func (h *BuildSSEHandler) Stream(c fiber.Ctx) error {
	projectID := c.Params("id")
	if projectID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "project id required"})
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	ch := service.RegisterSSEChannel(projectID)
	defer service.UnregisterSSEChannel(projectID, ch)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	c.RequestCtx().SetBodyStreamWriter(func(w *bufio.Writer) {
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				w.WriteString(fmt.Sprintf("data: %s\n\n", msg))
			case <-ticker.C:
				w.WriteString(": keepalive\n\n")
			case <-c.Context().Done():
				return
			}
			w.Flush()
		}
	})

	return nil
}
