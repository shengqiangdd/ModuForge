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

## CRITICAL RULES (prevents code corruption):

### Shell Script Rules (MUST FOLLOW):
1. ALWAYS use ${VAR} syntax, NEVER $VAR alone
   - CORRECT: ${MODPATH}, ${ARCH}, ${ZIPFILE}, ${TMPDIR}
   - WRONG: $MODPATH, $ARCH, $ZIPFILE
2. NEVER use $1, $2, $@. Use explicit parameter names.
3. Each function max 20 lines
4. Use if/then/else, avoid complex logic

### Go Code Rules (MUST FOLLOW):
1. Use ONLY these types: int, string, bool, float64, error
   - NEVER use: float, int32, int64, uint, byte
2. Initialize ALL variables when declaring:
   - CORRECT: count := 0
   - WRONG: var count int
3. Each function max 15 lines
4. Use fmt.Println, os.ReadFile, time.Sleep only
5. NO cgo, NO syscall, NO unsafe packages

### C Code Rules (MUST FOLLOW):
1. Initialize ALL variables when declaring
2. Each function max 10 lines
3. Use printf, scanf, fopen only
4. NO complex pointers, NO dynamic allocation

### General Rules:
1. All numeric constants must be 0-100
2. Keep code SIMPLE and SHORT
3. Use ### filename.ext for file headers
4. Each file should be under 50 lines

## Output Format:
### module.prop
id=modulename
name=Module Name
version=1.0
versionCode=1
author=Developer
description=Module description

### customize.sh
#!/system/bin/sh
# Install script
SKIPUNZIP=1
ui_print "- Installing module..."

### service.sh
#!/system/bin/sh
# Service script
MODDIR=${0%/*}
# Start services

### src/main.go (if needed)
package main
import "fmt"
func main() {
    fmt.Println("Hello")
}

Generate files one by one. Keep code short and simple.`
}

// getStructuredGenerationPrompt returns a prompt that guides the model
// to generate code in smaller, more manageable chunks.
func getStructuredGenerationPrompt(moduleType string) string {
	base := getOptimizedSystemPrompt()
	
	switch moduleType {
	case "shell-only":
		return base + `

IMPORTANT: Generate ONLY Shell scripts (module.prop, customize.sh, service.sh).
Do NOT generate Go or C code. Keep each file under 30 lines.`
	case "simple-go":
		return base + `

IMPORTANT: Generate a SIMPLE Go program.
- Max 20 lines total
- Use only fmt, os, time packages
- No complex logic, just print and exit
- Initialize all variables at declaration`
	case "mixed":
		return base + `

IMPORTANT: Generate Shell scripts AND a simple Go program.
- Shell scripts: module.prop, customize.sh, service.sh
- Go program: src/main.go (max 20 lines)
- Keep Go code extremely simple`
	default:
		return base
	}
}

// detectModuleType analyzes the user prompt to determine the module type
func detectModuleType(prompt string) string {
	lower := strings.ToLower(prompt)
	
	// Check for shell-only indicators
	shellOnly := strings.Contains(lower, "shell") || 
		strings.Contains(lower, "bash") ||
		strings.Contains(lower, "script") ||
		(!strings.Contains(lower, "go") && !strings.Contains(lower, "golang") && 
		 !strings.Contains(lower, "c ") && !strings.Contains(lower, "c++"))
	
	// Check for Go indicators
	hasGo := strings.Contains(lower, "go ") || 
		strings.Contains(lower, "golang") ||
		strings.Contains(lower, ".go") ||
		strings.Contains(lower, "go program")
	
	// Check for C indicators
	hasC := strings.Contains(lower, " c ") || 
		strings.Contains(lower, "c++") ||
		strings.Contains(lower, ".c") ||
		strings.Contains(lower, "c program")
	
	if shellOnly && !hasGo && !hasC {
		return "shell-only"
	} else if hasGo && !hasC {
		return "simple-go"
	} else if hasGo || hasC {
		return "mixed"
	}
	
	return "default"
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
			moduleType := detectModuleType(req.Prompt)
			systemPrompt = getStructuredGenerationPrompt(moduleType)
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
				// Detect module type from user message
				userMsg := ""
				for _, m := range req.Messages {
					if m["role"] == "user" {
						userMsg = m["content"]
						break
					}
				}
				moduleType := detectModuleType(userMsg)
				optimized := map[string]string{
					"role":    "system",
					"content": getStructuredGenerationPrompt(moduleType),
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
