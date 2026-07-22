package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
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
	provider *Provider
	modelID  string
	apiKey   string
	endpoint string
	client   *http.Client
}

// NewGateway 创建 Gateway（legacy 兼容：直接指定 apiKey/endpoint/model）
func NewGateway(apiKey, endpoint, model string) *Gateway {
	return &Gateway{
		provider: nil,
		modelID:  model,
		apiKey:   apiKey,
		endpoint: endpoint + "/chat/completions",
		client:   &http.Client{Timeout: 120 * time.Second},
	}
}

// NewGatewayFromProvider 根据提供商和模型创建 Gateway
func NewGatewayFromProvider(providerID, modelID, apiKey string) (*Gateway, error) {
	provider := FindProvider(providerID)
	if provider == nil {
		return nil, fmt.Errorf("unknown provider: %s", providerID)
	}

	if provider.RequiresKey && apiKey == "" {
		return nil, fmt.Errorf("API key required for provider: %s", provider.Name)
	}

	endpoint := provider.Endpoint

	// Google 使用不同的 API 格式
	if providerID == "google" {
		endpoint = fmt.Sprintf("%s/%s:generateContent?key=%s", provider.Endpoint, modelID, apiKey)
	}

	return &Gateway{
		provider: provider,
		modelID:  modelID,
		apiKey:   apiKey,
		endpoint: endpoint,
		client:   &http.Client{Timeout: 120 * time.Second},
	}, nil
}

// Model 返回当前模型 ID
func (g *Gateway) Model() string {
	return g.modelID
}

// Chat 非流式对话
func (g *Gateway) Chat(messages []Message) (string, error) {
	req := ChatRequest{
		Model:    g.modelID,
		Messages: messages,
		Stream:   false,
	}
	body, _ := json.Marshal(req)

	httpReq, err := http.NewRequest("POST", g.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if g.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)
	}

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

// StreamChat 流式对话（SSE），逐条返回原始 SSE data 行
func (g *Gateway) StreamChat(ctx context.Context, messages []Message) (<-chan string, error) {
	req := ChatRequest{
		Model:    g.modelID,
		Messages: messages,
		Stream:   true,
	}
	body, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", g.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if g.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)
	}

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm stream request: %w", err)
	}

	ch := make(chan string, 100)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		buf := make([]byte, 4096)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				ch <- string(buf[:n])
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				break
			}
		}
	}()

	return ch, nil
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
