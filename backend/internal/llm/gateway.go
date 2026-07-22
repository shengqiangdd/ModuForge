package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// Message LLM 对话消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest LLM 对话请求
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// ChatResponse LLM 对话响应（非流式）
type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

// Gateway LLM 网关，统一 OpenAI/Claude/其他
type Gateway struct {
	apiKey   string
	endpoint string
	model    string
	client   *http.Client
}

func NewGateway(apiKey, endpoint, model string) *Gateway {
	return &Gateway{
		apiKey:   apiKey,
		endpoint: endpoint,
		model:    model,
		client:   &http.Client{},
	}
}

// Chat 非流式对话
func (g *Gateway) Chat(messages []Message) (string, error) {
	req := ChatRequest{
		Model:    g.model,
		Messages: messages,
		Stream:   false,
	}
	body, _ := json.Marshal(req)

	httpReq, err := http.NewRequest("POST", g.endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("llm parse: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("llm no choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// FileSpec 模块文件规格
type FileSpec struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

var jsonBlockRe = regexp.MustCompile("(?s)```json\\s*(.*?)\\s*```")

// ParseModuleOutput 从 LLM 原始输出中解析文件列表
func ParseModuleOutput(raw string) ([]FileSpec, error) {
	var jsonStr string

	// 尝试从 markdown code block 提取
	if m := jsonBlockRe.FindStringSubmatch(raw); len(m) > 1 {
		jsonStr = m[1]
	} else {
		// 尝试找裸 JSON 对象
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start >= 0 && end > start {
			jsonStr = raw[start : end+1]
		}
	}

	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in LLM output")
	}

	var result struct {
		Files []FileSpec `json:"files"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("parse module JSON: %w", err)
	}

	if len(result.Files) == 0 {
		return nil, fmt.Errorf("no files in LLM output")
	}

	return result.Files, nil
}
