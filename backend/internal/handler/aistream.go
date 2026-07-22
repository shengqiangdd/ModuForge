package handler

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type AIStreamHandler struct {
	aiService *service.AIStreamService
}

func NewAIStreamHandler(aiService *service.AIStreamService) *AIStreamHandler {
	return &AIStreamHandler{aiService: aiService}
}

type StreamRequest struct {
	Messages []map[string]string `json:"messages"`
	Prompt   string              `json:"prompt"`
}

func (h *AIStreamHandler) StreamChat(c fiber.Ctx) error {
	// Set SSE headers
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Access-Control-Allow-Origin", "*")

	var req StreamRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	// If prompt is provided, convert to messages format
	if req.Prompt != "" && len(req.Messages) == 0 {
		req.Messages = []map[string]string{
			{"role": "system", "content": "You are an expert Android Magisk/KSU module developer."},
			{"role": "user", "content": req.Prompt},
		}
	}

	ch, err := h.aiService.StreamCompletion(c.Context(), req.Messages)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Stream events
	for event := range ch {
		data, _ := json.Marshal(event)
		line := fmt.Sprintf("data: %s\n\n", string(data))
		if _, err := c.Write([]byte(line)); err != nil {
			return err
		}
	}

	// Send done event
	c.Write([]byte("data: {\"type\":\"done\",\"content\":\"\"}\n\n"))

	return nil
}
