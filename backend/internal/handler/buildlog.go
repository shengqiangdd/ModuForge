package handler

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type BuildLogHandler struct {
	logService *service.BuildLogService
}

func NewBuildLogHandler(logService *service.BuildLogService) *BuildLogHandler {
	return &BuildLogHandler{logService: logService}
}

func (h *BuildLogHandler) GetBuildLog(c fiber.Ctx) error {
	buildID := c.Query("build_id")
	if buildID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "build_id required"})
	}

	ch, err := h.logService.StreamLogs(c.Context(), buildID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	// Set SSE headers
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Access-Control-Allow-Origin", "*")

	for entry := range ch {
		data, _ := json.Marshal(entry)
		line := fmt.Sprintf("data: %s\n\n", string(data))
		if _, err := c.Write([]byte(line)); err != nil {
			return err
		}
	}

	return nil
}
