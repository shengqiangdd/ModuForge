package rag

import (
	"testing"
)

func TestSessionMemory_UpdateAndGet(t *testing.T) {
	m := NewSessionMemory()

	m.UpdateMemory("api", "getprop")
	m.UpdateMemory("platform", "android")

	if v, ok := m.GetMemory("api"); !ok || v != "getprop" {
		t.Errorf("expected getprop, got %s (ok=%v)", v, ok)
	}

	if v, ok := m.GetMemory("platform"); !ok || v != "android" {
		t.Errorf("expected android, got %s (ok=%v)", v, ok)
	}
}

func TestSessionMemory_Clear(t *testing.T) {
	m := NewSessionMemory()
	m.UpdateMemory("key", "value")
	m.ClearMemory()

	if _, ok := m.GetMemory("key"); ok {
		t.Error("expected key to be cleared")
	}
}

func TestSessionMemory_ContextString(t *testing.T) {
	m := NewSessionMemory()
	m.UpdateMemory("api", "getprop")
	m.UpdateMemory("platform", "android")

	ctx := m.ContextString()
	if ctx == "" {
		t.Error("expected non-empty context string")
	}

	// Should contain both key-value pairs
	if !containsSubstring(ctx, "api") || !containsSubstring(ctx, "getprop") {
		t.Error("context should contain api:getprop")
	}
}

func TestSessionMemory_AllKeys(t *testing.T) {
	m := NewSessionMemory()
	m.UpdateMemory("a", "1")
	m.UpdateMemory("b", "2")

	keys := m.AllKeys()
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestSessionMemory_Empty(t *testing.T) {
	m := NewSessionMemory()

	if _, ok := m.GetMemory("nonexistent"); ok {
		t.Error("expected false for empty memory")
	}

	if ctx := m.ContextString(); ctx != "" {
		t.Errorf("expected empty context, got %s", ctx)
	}

	keys := m.AllKeys()
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

func TestNewStatefulRAG(t *testing.T) {
	kb := NewKnowledgeBase()
	sr := NewStatefulRAG(kb)

	if sr == nil {
		t.Fatal("expected non-nil StatefulRAG")
	}

	if sr.kb != kb {
		t.Error("expected kb to be set")
	}
}

func TestStatefulRAG_UpdateMemory(t *testing.T) {
	kb := NewKnowledgeBase()
	sr := NewStatefulRAG(kb)

	sr.UpdateMemory("discussed_api", "getprop")

	if v, ok := sr.GetMemory("discussed_api"); !ok || v != "getprop" {
		t.Errorf("expected getprop, got %s", v)
	}
}

func TestStatefulRAG_ClearMemory(t *testing.T) {
	kb := NewKnowledgeBase()
	sr := NewStatefulRAG(kb)

	sr.UpdateMemory("key", "value")
	sr.ClearMemory()

	if _, ok := sr.GetMemory("key"); ok {
		t.Error("expected memory to be cleared")
	}
}

func TestStatefulRAG_SearchWithContext_NilKB(t *testing.T) {
	sr := NewStatefulRAG(nil)
	_, err := sr.SearchWithContext("test", 5)
	if err == nil {
		t.Fatal("expected error for nil KB")
	}
}

func TestStatefulRAG_SearchWithContext_EmptyKB(t *testing.T) {
	kb := NewKnowledgeBase()
	sr := NewStatefulRAG(kb)

	results, err := sr.SearchWithContext("test", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty KB, got %d", len(results))
	}
}

func TestStatefulRAG_SearchWithContext_WithMemory(t *testing.T) {
	kb := NewKnowledgeBase()
	kb.Chunks = []CodeChunk{
		{
			ID:      "1",
			Source:  "shell",
			Content: "battery level monitoring daemon service",
			Vector:  computeTFIDF("battery level monitoring daemon service", make(map[string]float64)),
		},
		{
			ID:      "2",
			Source:  "shell",
			Content: "file permission setup installer customize",
			Vector:  computeTFIDF("file permission setup installer customize", make(map[string]float64)),
		},
	}
	kb.IDF = computeIDF(kb.Chunks)
	for i := range kb.Chunks {
		kb.Chunks[i].Vector = computeTFIDF(kb.Chunks[i].Content, kb.IDF)
	}

	sr := NewStatefulRAG(kb)
	sr.UpdateMemory("topic", "battery daemon")

	results, err := sr.SearchWithContext("battery monitoring", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return results
	if len(results) == 0 {
		t.Error("expected at least 1 result")
	}

	// First result should be the battery chunk
	if len(results) > 0 && results[0].Chunk.ID != "1" {
		t.Errorf("expected first result to be chunk 1, got %s", results[0].Chunk.ID)
	}
}

func TestMemoryBoost(t *testing.T) {
	chunk := CodeChunk{
		Content: "battery level daemon service",
	}

	memory := NewSessionMemory()
	memory.UpdateMemory("topic", "battery daemon")

	boost := memoryBoost(chunk, memory)
	if boost <= 0 {
		t.Error("expected positive boost for matching keywords")
	}
}

func TestMemoryBoost_Empty(t *testing.T) {
	chunk := CodeChunk{
		Content: "battery level daemon service",
	}

	memory := NewSessionMemory()
	boost := memoryBoost(chunk, memory)
	if boost != 0 {
		t.Errorf("expected 0 boost for empty memory, got %f", boost)
	}
}

// containsSubstring checks if s contains substr (simple string search).
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
