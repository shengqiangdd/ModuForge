package agent

import (
	"strings"
	"testing"
)

func TestAppendRoleMessage(t *testing.T) {
	conv := []map[string]interface{}{{"role": "user", "content": "hi"}}
	got := appendRoleMessage(conv, "system", "note")
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}
	if got[1]["role"] != "system" || got[1]["content"] != "note" {
		t.Fatalf("unexpected appended message: %#v", got[1])
	}
}

func TestAppendToolResult_AppendsAndPersists(t *testing.T) {
	r := NewAgentRunner(nil, "k", "http://e/v1/chat/completions", "m", nil)
	got := r.appendToolResult(nil, "session-1", "call-1", "result text")
	if len(got) != 1 {
		t.Fatalf("expected 1 tool message, got %d", len(got))
	}
	msg := got[0]
	if msg["role"] != "tool" || msg["tool_call_id"] != "call-1" || msg["content"] != "result text" {
		t.Fatalf("unexpected tool message: %#v", msg)
	}
	stored := r.convStore.Get("session-1")
	if len(stored) != 1 || stored[0].Content != "result text" || stored[0].ToolCallID != "call-1" || stored[0].Role != "tool" {
		t.Fatalf("expected 1 stored tool message, got %#v", stored)
	}
}

func TestAppendToolResult_NoSessionNoPersist(t *testing.T) {
	r := NewAgentRunner(nil, "k", "http://e/v1/chat/completions", "m", nil)
	got := r.appendToolResult(nil, "", "call-1", "result text")
	if len(got) != 1 {
		t.Fatalf("expected 1 tool message, got %d", len(got))
	}
}

func TestHeuristicCompactConversation_ReducesAndPreserves(t *testing.T) {
	r := NewAgentRunner(nil, "k", "http://e/v1/chat/completions", "m", nil)
	conv := []map[string]interface{}{
		{"role": "system", "content": "system prompt"},
		{"role": "user", "content": "task 1"},
		{"role": "assistant", "content": "resp 1"},
		{"role": "user", "content": "task 2"},
		{"role": "assistant", "content": "resp 2"},
		{"role": "user", "content": "task 3"},
		{"role": "assistant", "content": "resp 3"},
		{"role": "user", "content": "task 4"},
		{"role": "assistant", "content": "resp 4"},
		{"role": "user", "content": "task 5"},
	}
	got := r.heuristicCompactConversation(conv)
	if len(got) >= len(conv) {
		t.Fatalf("expected compaction to reduce message count, got %d >= %d", len(got), len(conv))
	}
	if got[0]["role"] != "system" {
		t.Fatalf("first message should be system, got %#v", got[0])
	}
	last := got[len(got)-1]
	if last["role"] != "user" || last["content"] != "task 5" {
		t.Fatalf("last user message should be preserved, got %#v", last)
	}
}

func TestHeuristicCompactConversation_PreservesFileChanges(t *testing.T) {
	r := NewAgentRunner(nil, "k", "http://e/v1/chat/completions", "m", nil)
	conv := []map[string]interface{}{
		{"role": "system", "content": "s"},
		{"role": "tool", "content": "✅ Successfully wrote 1 files: a.go"},
		{"role": "user", "content": "task 1"},
		{"role": "assistant", "content": "r1"},
		{"role": "user", "content": "task 2"},
		{"role": "assistant", "content": "r2"},
		{"role": "user", "content": "task 3"},
		{"role": "assistant", "content": "r3"},
		{"role": "user", "content": "task 4"},
		{"role": "assistant", "content": "r4"},
		{"role": "user", "content": "task 5"},
	}
	got := r.heuristicCompactConversation(conv)
	found := false
	for _, m := range got {
		if c, _ := m["content"].(string); strings.Contains(c, "Successfully wrote") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected file-change summary to be preserved, got %#v", got)
	}
}
