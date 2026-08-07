package handler

import (
	"bufio"
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
	// ACAO set by CORS middleware; do not override

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

	// Use SetBodyStreamWriter for real SSE streaming
	c.RequestCtx().SetBodyStreamWriter(func(w *bufio.Writer) {
		for event := range ch {
			data, _ := json.Marshal(event)
			line := fmt.Sprintf("data: %s\n\n", string(data))
			w.WriteString(line)
			w.Flush()
		}
		w.WriteString("data: {\"type\":\"done\",\"content\":\"\"}\n\n")
		w.Flush()
	})

	return nil
}
