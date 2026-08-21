package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ─── Streaming LLM Calls ───

// streamChatWithSystemForUser 支持用户 specific LLM 配置的流式请求
func (s *AIService) streamChatWithSystemForUser(ctx context.Context, systemPrompt, userPrompt, userID, sessionID string, w *bufio.Writer, history ...[]Message) error {
	endpoint, apiKey, model, _ := s.resolveLLMConfig(userID)
	providerID := s.cfg.LLMProvider

	if providerRequiresKey(providerID) && apiKey == "" {
		w.WriteString("data: " + `{"content":"LLM not configured. Set API key in Settings."}` + "\n\ndata: [DONE]\n\n")
		w.Flush()
		return nil
	}

	// Build the full message array: system + (bounded) history + current user
	// turn. Previously history was dropped entirely, so "continue/改一下"
	// follow-ups had no context. Cap history by token estimate to protect
	// context windows on every tier.
	var msgs []map[string]string
	if len(history) > 0 && len(history[0]) > 0 {
		msgs = buildHistoryMessages(systemPrompt, userPrompt, history[0])
	} else {
		msgs = []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		}
	}

	body := map[string]interface{}{
		"model":    model,
		"messages": msgs,
		"stream":   true,
	}
	bodyBytes, _ := json.Marshal(body)

	chatURL := ensureChatCompletionsURL(endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", chatURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		errEvt, _ := json.Marshal(map[string]string{"type": "error", "error": err.Error()})
		w.WriteString("data: " + string(errEvt) + "\n\ndata: [DONE]\n\n")
		w.Flush()
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("LLM error (HTTP %d)", resp.StatusCode)
		var errBody struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(bodyBytes, &errBody) == nil && errBody.Error.Message != "" {
			errMsg = errBody.Error.Message
		}
		errEvt, _ := json.Marshal(map[string]string{"type": "error", "error": errMsg})
		w.WriteString("data: " + string(errEvt) + "\n\ndata: [DONE]\n\n")
		w.Flush()
		return nil
	}

	scanner := bufio.NewScanner(resp.Body)

	// Keepalive goroutine — 每 15 秒发一个 SSE 注释，防止代理/客户端超时关闭连接
	keepAliveDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.WriteString(": keepalive\n\n")
				w.Flush()
			case <-keepAliveDone:
				return
			}
		}
	}()

	// Non-agent LLM usage (persisted after the stream ends)
	var usage *TokenUsageInfo

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			w.WriteString(line + "\n")
			w.Flush()
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			w.WriteString("data: [DONE]\n\n")
			w.Flush()
			break
		}
		if u := extractUsageFromChunk(data); u != nil {
			usage = u
		}
		w.WriteString(line + "\n")
		w.Flush()
	}

	close(keepAliveDone)
	s.recordNonAgentUsage(sessionID, userID, model, usage)
	return nil
}

// ─── Token Usage Tracking ───

// TokenUsageInfo mirrors the standard chat-completions usage payload.
type TokenUsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// extractUsageFromChunk parses a chat-completions SSE chunk and returns the
// usage block when present (typically the trailing chunk with empty choices).
func extractUsageFromChunk(data string) *TokenUsageInfo {
	if !strings.Contains(data, "\"usage\"") {
		return nil
	}
	var parsed struct {
		Choices []json.RawMessage `json:"choices"`
		Usage   *TokenUsageInfo   `json:"usage"`
	}
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		return nil
	}
	if parsed.Usage == nil || (parsed.Usage.TotalTokens <= 0 && len(parsed.Choices) > 0) {
		return nil
	}
	return parsed.Usage
}

// recordNonAgentUsage persists non-agent-mode (chat/generate/repair/gather/
// auto-build) LLM usage into ai_conversations.token_usage and the daily
// aggregation table so usage/cost stats cover every AI mode.
func (s *AIService) recordNonAgentUsage(sessionID, userID, model string, usage *TokenUsageInfo) {
	if s.db == nil || sessionID == "" || userID == "" || usage == nil || usage.TotalTokens <= 0 {
		return
	}
	total := int64(usage.TotalTokens)
	if _, err := s.db.Exec(
		`UPDATE ai_conversations SET token_usage = COALESCE(token_usage, 0) + ?, updated_at = datetime('now') WHERE id = ? AND user_id = ?`,
		total, sessionID, userID,
	); err != nil {
		slog.Warn("recordNonAgentUsage update conversation", "error", err)
	}
	today := time.Now().Format("2006-01-02")
	if _, err := s.db.Exec(
		`INSERT INTO ai_usage_daily (date, user_id, llm_call_count, llm_token_usage) VALUES (?, ?, 1, ?)
		 ON CONFLICT(date, user_id) DO UPDATE SET
		   llm_call_count = llm_call_count + 1,
		   llm_token_usage = llm_token_usage + excluded.llm_token_usage,
		   updated_at = CURRENT_TIMESTAMP`,
		today, userID, total,
	); err != nil {
		slog.Warn("recordNonAgentUsage daily", "error", err)
	}
}

// ─── SSE & JSON Utilities ───

// sendSSE 发送SSE事件，写入失败时返回error
func (s *AIService) sendSSE(w *bufio.Writer, data map[string]interface{}) error {
	jsonData, _ := json.Marshal(data)
	if _, err := w.WriteString("data: " + string(jsonData) + "\n\n"); err != nil {
		return err
	}
	return w.Flush()

