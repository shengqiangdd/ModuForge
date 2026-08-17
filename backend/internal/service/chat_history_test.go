package service

import (
	"strings"
	"testing"
)

// TestHistoryForLLM covers ordering, current-turn dedupe, budget trimming and
// role filtering of the chat history builder.
func TestHistoryForLLM(t *testing.T) {
	hist := []Message{
		{Role: "user", Content: "帮我写一个 Magisk 模块"},
		{Role: "assistant", Content: "好的，结构如下"},
		{Role: "user", Content: "继续"},
	}
	userPrompt := "继续" // last history entry duplicates the current prompt

	got := historyForLLM(hist, userPrompt, 20_000)
	if len(got) != 2 {
		t.Fatalf("expected 2 history turns (current turn dropped), got %d: %v", len(got), got)
	}
	if got[0].Role != "user" || !strings.Contains(got[0].Content, "Magisk") {
		t.Errorf("first kept turn wrong: %+v", got[0])
	}
	if got[1].Role != "assistant" {
		t.Errorf("second kept turn wrong: %+v", got[1])
	}

	// Budget trim keeps most recent turns.
	big := []Message{
		{Role: "user", Content: strings.Repeat("a", 100)},
		{Role: "assistant", Content: strings.Repeat("b", 100)},
		{Role: "user", Content: strings.Repeat("c", 100)},
	}
	got = historyForLLM(big, "", 250)
	if len(got) != 2 {
		t.Fatalf("budget trim: expected 2 turns, got %d", len(got))
	}
	if !strings.Contains(got[len(got)-1].Content, "c") {
		t.Errorf("latest turn should survive trim: %+v", got)
	}

	// System/other roles are filtered out (kept clean for OpenAI-style API).
	mixed := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "u1"},
		{Role: "tool", Content: "tool result"},
	}
	got = historyForLLM(mixed, "", 20_000)
	if len(got) != 1 || got[0].Role != "user" {
		t.Fatalf("role filter: expected only user turn, got %v", got)
	}

	// Empty history -> nil.
	if historyForLLM(nil, "x", 20_000) != nil {
		t.Fatal("empty history should return nil")
	}
}