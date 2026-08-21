package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"

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

// isFreeModel checks if the model is a free model that needs prompt optimization.
func isFreeModel(model string) bool {
	freeModels := []string{"laguna-s-2.1-free", "laguna", "free", "demo"}
	lower := strings.ToLower(model)
	for _, fm := range freeModels {
		if strings.Contains(lower, fm) {
			return true
		}
	}
	return false
}

// getOptimizedSystemPrompt returns a prompt optimized for free models
// that avoids tokenizer truncation issues.
func getOptimizedSystemPrompt() string {
	return `You are an expert Magisk module developer. Generate COMPLETE, COMPILABLE code.

CRITICAL RULES (prevents code corruption):
1. Shell: ALWAYS use ${VAR} syntax, NEVER $VAR
   - CORRECT: ${MODPATH}, ${ARCH}, ${ZIPFILE}
   - WRONG: $MODPATH, $ARCH, $ZIPFILE
2. Shell: NEVER use $1, $2, $@. Use explicit names.
3. Go: Use ONLY int, string, bool, float64, error
   - NEVER use: float, int32, int64, uint
4. Go: Initialize ALL variables at declaration
5. Go: Each function max 15 lines
6. C: Initialize ALL variables at declaration
7. C: Each function max 10 lines
8. All numeric constants must be 0-100
9. Use ### filename for file headers

Generate files using this format:
### filename.ext
<code in appropriate language>`
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

	// Inject optimized system prompt if needed
	// Case 1: prompt field provided, no messages
	if req.Prompt != "" && len(req.Messages) == 0 {
		systemPrompt := "You are an expert Android Magisk/KSU module developer."
		
		_, _, model, _ := h.aiService.ResolveLLMConfig("")
		if isFreeModel(model) {
			systemPrompt = getOptimizedSystemPrompt()
		}
		
		req.Messages = []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": req.Prompt},
		}
	}
	// Case 2: messages provided but no system message → inject optimized prompt
	if len(req.Messages) > 0 {
		hasSystem := false
		for _, m := range req.Messages {
			if m["role"] == "system" {
				hasSystem = true
				break
			}
		}
		if !hasSystem {
			_, _, model, _ := h.aiService.ResolveLLMConfig("")
			if isFreeModel(model) {
				optimized := map[string]string{
					"role":    "system",
					"content": getOptimizedSystemPrompt(),
				}
				req.Messages = append([]map[string]string{optimized}, req.Messages...)
			}
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
