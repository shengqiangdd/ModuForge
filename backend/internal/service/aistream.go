package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type AIStreamService struct {
	client *http.Client
}

func NewAIStreamService() *AIStreamService {
	return &AIStreamService{
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

type AIStreamEvent struct {
	Type    string `json:"type"` // "delta", "error", "done"
	Content string `json:"content"`
}

// StreamCompletion 流式调用 LLM 并返回事件 channel
func (s *AIStreamService) StreamCompletion(ctx context.Context, messages []map[string]string) (<-chan AIStreamEvent, error) {
	ch := make(chan AIStreamEvent, 50)

	go func() {
		defer close(ch)

		// 发送 mock SSE 事件（实际对接时替换为真实 LLM API）
		fullResponse := s.mockGenerateResponse(messages)

		for _, char := range fullResponse {
			select {
			case <-ctx.Done():
				return
			default:
				ch <- AIStreamEvent{
					Type:    "delta",
					Content: string(char),
				}
				time.Sleep(10 * time.Millisecond) // simulate streaming
			}
		}
		ch <- AIStreamEvent{Type: "done", Content: ""}
	}()

	return ch, nil
}

// StreamSSE 从 SSE endpoint 读取流事件
func (s *AIStreamService) StreamSSE(ctx context.Context, apiURL, apiKey string, body io.Reader) (<-chan AIStreamEvent, error) {
	ch := make(chan AIStreamEvent, 100)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, body)
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := s.client.Do(req)
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("SSE request: %w", err)
	}

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- AIStreamEvent{Type: "done"}
				return
			}
			ch <- AIStreamEvent{Type: "delta", Content: data}
		}
	}()

	return ch, nil
}

// mockGenerateResponse 生成模拟的 AI 回复
func (s *AIStreamService) mockGenerateResponse(messages []map[string]string) string {
	lastMsg := ""
	if len(messages) > 0 {
		lastMsg = messages[len(messages)-1]["content"]
	}

	lastMsg = strings.ToLower(lastMsg)

	if strings.Contains(lastMsg, "magisk") || strings.Contains(lastMsg, "module") {
		return "# Magisk Module Template\n\n## Module Structure\n```\nmodule.prop\ncustomize.sh\nsystem/\n  etc/\n  lib/\n```\n\nTo create a Magisk module:\n1. Create module.prop with metadata\n2. Add customize.sh for installation scripts\n3. Place system files in the system/ directory"
	}

	if strings.Contains(lastMsg, "system.prop") || strings.Contains(lastMsg, "property") {
		return "# system.prop Template\n\n```\n# Performance tweaks\ndebug.sf.enable_hwc_vds=1\ndebug.hwui.renderer=opengl\npersist.sys.job_delay=true\n\n# Display\ntouch.presure.scale=0.001\nro.surface_flinger.max_frame_buffer_acquired_buffers=3\n```"
	}

	return "# AI Suggestion\n\nI can help you create Magisk/KSU modules. Describe what you want:\n- System property modifications\n- Audio tweaks\n- Display/GPU optimization\n- Boot animation customization\n- Ad blocking hosts file"
}
