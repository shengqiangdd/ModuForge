package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/config"
)

type AIService struct {
	cfg *config.Config
}

func NewAIService(cfg *config.Config) *AIService {
	return &AIService{cfg: cfg}
}

// GenerateModule 用 LLM 生成模块代码，SSE 流式返回
func (s *AIService) GenerateModule(ctx context.Context, description, moduleType string, c fiber.Ctx) error {
	prompt := fmt.Sprintf(`You are an Android module developer. Create a %s module for Magisk/KSU/APatch.
Description: %s

Generate the module files in JSON format with "files" array (each with "path" and "content").
Include at minimum: module.prop, install.sh, and any needed scripts.`, moduleType, description)
	return s.streamChat(ctx, prompt, c)
}

// Chat 通用 AI 对话
func (s *AIService) Chat(ctx context.Context, message, contextInfo string, c fiber.Ctx) error {
	prompt := message
	if contextInfo != "" {
		prompt = fmt.Sprintf("Context: %s\n\nQuestion: %s", contextInfo, message)
	}
	return s.streamChat(ctx, prompt, c)
}

// RepairBuild 分析构建日志给出修复建议
func (s *AIService) RepairBuild(ctx context.Context, buildLog string, c fiber.Ctx) error {
	prompt := fmt.Sprintf(`Analyze this Android module build log and suggest fixes:

%s

Provide step-by-step repair instructions.`, buildLog)
	return s.streamChat(ctx, prompt, c)
}

func (s *AIService) streamChat(ctx context.Context, prompt string, c fiber.Ctx) error {
	if s.cfg.LLMApiKey == "" {
		// No API key configured — return demo response
		_, err := c.Write([]byte("data: " + `{"role":"assistant","content":"LLM not configured. Set LLM_API_KEY to enable AI features.\n\nThe architecture is ready for your module files:\n- module.prop: module metadata\n- system/: system file overrides\n- META-INF/: update-binary + updater-script\n- customize.sh: installation hooks\n\nEdit files in the editor tab to build your module."}` + "\n\ndata: [DONE]\n\n"))
		return err
	}

	body := map[string]interface{}{
		"model": s.cfg.LLMModel,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": true,
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.LLMEndpoint+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.cfg.LLMApiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Fallback to demo response
		_, err := c.Write([]byte("data: " + `{"role":"assistant","content":"AI service unavailable. Please check your LLM_API_KEY and network connectivity."}` + "\n\ndata: [DONE]\n\n"))
		return err
	}
	defer resp.Body.Close()

	// Proxy SSE stream byte-by-byte
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := c.Write(buf[:n]); werr != nil {
				break
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
	}

	return nil
}
