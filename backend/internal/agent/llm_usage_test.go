package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockSSEWriter is a minimal SSEWriter for tests.
type mockSSEWriter struct {
	events []map[string]interface{}
}

func (m *mockSSEWriter) WriteSSE(data map[string]interface{}) error {
	m.events = append(m.events, data)
	return nil
}
func (m *mockSSEWriter) WriteSSEPlain(data string) error { return nil }
func (m *mockSSEWriter) WriteSSEComment(comment string) error { return nil }
func (m *mockSSEWriter) IsDisconnected() bool            { return false }
func (m *mockSSEWriter) Flush() error { return nil }

// TestParseStreamingResponse_CapturesUsage verifies that the usage field from
// the final streaming chunk (empty choices) is captured, not skipped.
func TestParseStreamingResponse_CapturesUsage(t *testing.T) {
	runner := NewAgentRunner(nil, "", "", "", nil)

	// OpenAI-style stream: content chunk, finish chunk, then final chunk with empty choices + usage.
	stream := []map[string]interface{}{
		{"choices": []map[string]interface{}{{"delta": map[string]interface{}{"content": "你好"}, "finish_reason": ""}}},
		{"choices": []map[string]interface{}{{"delta": map[string]interface{}{}, "finish_reason": "stop"}}},
		{"choices": []map[string]interface{}{}, "usage": map[string]interface{}{"prompt_tokens": 123, "completion_tokens": 45, "total_tokens": 168}},
	}
	var sb strings.Builder
	for _, chunk := range stream {
		b, _ := json.Marshal(chunk)
		sb.WriteString("data: ")
		sb.Write(b)
		sb.WriteString("\n\n")
	}
	sb.WriteString("data: [DONE]\n\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sb.String()))
	}))
	defer srv.Close()

	httpResp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("http.Get: %v", err)
	}
	defer httpResp.Body.Close()

	writer := &mockSSEWriter{}
	result, err := runner.parseStreamingResponse(context.Background(), httpResp, writer)
	if err != nil {
		t.Fatalf("parseStreamingResponse error: %v", err)
	}
	if result.Content != "你好" {
		t.Fatalf("content = %q, want 你好", result.Content)
	}
	if result.TokenUsage == nil {
		t.Fatal("TokenUsage is nil, want usage captured from final chunk")
	}
	if result.TokenUsage.PromptTokens != 123 || result.TokenUsage.CompletionTokens != 45 || result.TokenUsage.TotalTokens != 168 {
		t.Fatalf("usage = %+v, want prompt=123 completion=45 total=168", result.TokenUsage)
	}
}
