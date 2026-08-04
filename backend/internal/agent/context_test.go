package agent

import (
	"strings"
	"testing"

	"github.com/moduforge/backend/internal/service"
)

func TestPrefilterConversation_RemovesEmptyToolResults(t *testing.T) {
	conversation := []map[string]interface{}{
		{"role": "user", "content": "hello"},
		{"role": "assistant", "content": "hi"},
		{"role": "tool", "content": ""},
		{"role": "tool", "content": "   "},
		{"role": "assistant", "content": "done"},
	}

	result := prefilterConversation(conversation)
	if len(result) != 3 {
		t.Errorf("expected 3 messages, got %d", len(result))
	}
}

func TestPrefilterConversation_RemovesConsecutiveDuplicates(t *testing.T) {
	conversation := []map[string]interface{}{
		{"role": "user", "content": "hello"},
		{"role": "assistant", "content": "thinking..."},
		{"role": "assistant", "content": "thinking..."}, // duplicate
		{"role": "assistant", "content": "done"},
	}

	result := prefilterConversation(conversation)
	if len(result) != 3 {
		t.Errorf("expected 3 messages, got %d", len(result))
	}
}

func TestPrefilterConversation_TruncatesLongToolResults(t *testing.T) {
	// Use unique content per char to avoid dedup key collision
	var sb strings.Builder
	for i := 0; i < 4001; i++ {
		sb.WriteByte(byte('a' + i%26))
	}
	longContent := sb.String()

	// Need 3+ messages to pass the early return (len <= 2)
	conversation := []map[string]interface{}{
		{"role": "user", "content": "read file"},
		{"role": "tool", "content": longContent},
		{"role": "assistant", "content": "ok"},
	}

	result := prefilterConversation(conversation)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}

	content, _ := result[1]["content"].(string)
	// After truncation: 2000 + " [truncated] " (~35) + 1000 ≈ 3035
	if len(content) > 3100 {
		t.Errorf("expected truncated content (<3100), got %d chars", len(content))
	}
	if len(content) < 2000 {
		t.Errorf("truncation too aggressive: %d chars", len(content))
	}
}

func TestPrefilterConversation_DeduplicatesToolResults(t *testing.T) {
	sameContent := "file_content_xyz_123"
	conversation := []map[string]interface{}{
		{"role": "user", "content": "read file"},
		{"role": "tool", "content": sameContent},    // 1st: count=1, kept
		{"role": "assistant", "content": "ok"},
		{"role": "tool", "content": sameContent},    // 2nd: count=2, kept
		{"role": "assistant", "content": "ok"},
		{"role": "tool", "content": sameContent},    // 3rd: count=2, skipped (>=2)
		{"role": "assistant", "content": "ok"},
		{"role": "tool", "content": sameContent},    // 4th: count=2, skipped (>=2)
	}

	result := prefilterConversation(conversation)
	toolCount := 0
	for _, msg := range result {
		if msg["role"] == "tool" {
			toolCount++
		}
	}
	// Dedup keeps first 2 occurrences, skips 3rd and 4th
	if toolCount != 2 {
		t.Errorf("expected 2 tool messages (dedup keeps first 2), got %d", toolCount)
	}
}

func TestIncrementalCompactHistory_TooFewMessages(t *testing.T) {
	r := &AgentRunner{}
	history := []service.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	result := r.incrementalCompactHistory(nil, history, nil, RunConfig{}, 1000)
	if result != nil {
		t.Error("expected nil for too few messages")
	}
}

func TestIncrementalCompactHistory_AlreadyCompacted(t *testing.T) {
	r := &AgentRunner{}
	history := []service.Message{
		{Role: "system", Content: "[上下文增量压缩] summary"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
		{Role: "user", Content: "more"},
		{Role: "assistant", Content: "done"},
	}
	result := r.incrementalCompactHistory(nil, history, nil, RunConfig{}, 50000)
	if result != nil {
		t.Error("expected nil when already compacted")
	}
}

func TestIncrementalCompactHistory_CompactsOldMessages(t *testing.T) {
	r := &AgentRunner{}

	var history []service.Message
	for i := 0; i < 10; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		history = append(history, service.Message{
			Role:    role,
			Content: "msg_" + string(rune('a'+i)),
		})
	}

	// Force compaction by passing large currentTotal
	result := r.incrementalCompactHistory(nil, history, nil, RunConfig{}, 50000)

	if result == nil {
		t.Fatal("expected compaction to happen")
	}

	// Should have: 1 summary + last 6 messages = 7
	if len(result) != 7 {
		t.Errorf("expected 7 messages (1 summary + 6 kept), got %d", len(result))
	}

	// First message should be the summary
	if result[0].Role != "system" {
		t.Errorf("expected first message to be system, got %s", result[0].Role)
	}
	if !strings.HasPrefix(result[0].Content, "[上下文增量压缩]") {
		t.Error("summary should start with [上下文增量压缩]")
	}

	// Last 6 messages (indices 4,5,6,7,8,9) should be preserved exactly
	for i := 1; i < 7; i++ {
		expectedIdx := 3 + i // history[4], history[5], ..., history[9]
		if result[i].Content != history[expectedIdx].Content {
			t.Errorf("message %d: expected '%s', got '%s'", i, history[expectedIdx].Content, result[i].Content)
		}
	}
}

func TestIncrementalCompactHistory_SmallTotalNotCompacted(t *testing.T) {
	r := &AgentRunner{}

	var history []service.Message
	for i := 0; i < 6; i++ {
		history = append(history, service.Message{
			Role:    "user",
			Content: "msg",
		})
	}

	// Small total, should not compact
	result := r.incrementalCompactHistory(nil, history, nil, RunConfig{}, 100)
	if result != nil {
		t.Error("expected nil when total is small")
	}
}

func TestEstimateConversationSize(t *testing.T) {
	r := &AgentRunner{}
	conversation := []map[string]interface{}{
		{"role": "user", "content": "hello world"},
		{"role": "assistant", "content": "hi there"},
	}

	size := r.estimateConversationSize(conversation)
	if size != 19 { // "hello world" (11) + "hi there" (8)
		t.Errorf("expected 19, got %d", size)
	}
}

func TestEstimateConversationSize_WithToolCalls(t *testing.T) {
	r := &AgentRunner{}
	tc := LLMToolCall{}
	tc.Function.Name = "read_file"
	tc.Function.Arguments = `{"path":"test.go"}`
	conversation := []map[string]interface{}{
		{
			"role":       "assistant",
			"content":    "calling tool",
			"tool_calls": []LLMToolCall{tc},
		},
	}

	size := r.estimateConversationSize(conversation)
	// "calling tool" (12) + "read_file" (9) + `{"path":"test.go"}` (18) = 39
	if size != 39 {
		t.Errorf("expected 39, got %d", size)
	}
}

func TestEstimateConversationSize_Empty(t *testing.T) {
	r := &AgentRunner{}
	size := r.estimateConversationSize(nil)
	if size != 0 {
		t.Errorf("expected 0, got %d", size)
	}
}
