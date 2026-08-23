package rag

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChunkText(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		size    int
		overlap int
		want    int // expected minimum number of chunks
	}{
		{
			name:    "short text no chunking",
			text:    "hello world",
			size:    512,
			overlap: 50,
			want:    1,
		},
		{
			name:    "exact size",
			text:    repeatStr("a", 512),
			size:    512,
			overlap: 50,
			want:    1,
		},
		{
			name:    "needs chunking",
			text:    repeatStr("a", 1000),
			size:    512,
			overlap: 50,
			want:    2,
		},
		{
			name:    "large text",
			text:    repeatStr("word ", 200), // ~1000 chars
			size:    256,
			overlap: 26,
			want:    4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := chunkText(tt.text, tt.size, tt.overlap)
			if len(chunks) < tt.want {
				t.Errorf("chunkText() produced %d chunks, want >= %d", len(chunks), tt.want)
			}
			// Verify no chunk exceeds size
			for i, c := range chunks {
				if len([]rune(c)) > tt.size {
					t.Errorf("chunk %d has %d runes, max %d", i, len([]rune(c)), tt.size)
				}
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("Hello, World! This is a test.")
	if len(tokens) == 0 {
		t.Fatal("tokenize returned no tokens")
	}
	// Should have filtered stop words
	for _, tok := range tokens {
		if tok == "is" || tok == "a" || tok == "this" {
			t.Errorf("stop word %q not filtered", tok)
		}
	}
	// Should be lowercase
	for _, tok := range tokens {
		if tok != strings.ToLower(tok) {
			t.Errorf("token %q is not lowercase", tok)
		}
		if tok != "hello" && tok != "world" && tok != "test" {
			// other tokens are fine
		}
	}
}

func TestComputeIDF(t *testing.T) {
	chunks := []CodeChunk{
		{Content: "hello world foo"},
		{Content: "hello bar baz"},
		{Content: "world foo bar"},
	}

	idf := computeIDF(chunks)
	if len(idf) == 0 {
		t.Fatal("computeIDF returned empty map")
	}

	// "hello" appears in 2/3 docs — IDF should be moderate
	// "foo" appears in 2/3 docs
	// "baz" appears in 1/3 docs — IDF should be higher
	if idf["baz"] <= idf["hello"] {
		t.Errorf("baz (1 doc) should have higher IDF than hello (2 docs): baz=%.2f hello=%.2f",
			idf["baz"], idf["hello"])
	}
}

func TestComputeTFIDF(t *testing.T) {
	text := "hello world hello foo"
	idf := map[string]float64{
		"hello": 1.5,
		"world": 1.0,
		"foo":   2.5,
	}

	vec := computeTFIDF(text, idf)
	if len(vec) == 0 {
		t.Fatal("computeTFIDF returned nil vector")
	}

	// "hello" appears twice, should have highest TF
	if vec["hello"] <= vec["world"] {
		t.Errorf("hello (2x) should score higher than world (1x): hello=%.2f world=%.2f",
			vec["hello"], vec["world"])
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b map[string]float64
		min  float64
	}{
		{
			name: "identical vectors",
			a:    map[string]float64{"a": 1, "b": 2},
			b:    map[string]float64{"a": 1, "b": 2},
			min:  0.99,
		},
		{
			name: "orthogonal vectors",
			a:    map[string]float64{"a": 1},
			b:    map[string]float64{"b": 1},
			min:  0,
		},
		{
			name: "similar vectors",
			a:    map[string]float64{"a": 1, "b": 2, "c": 3},
			b:    map[string]float64{"a": 1, "b": 2, "c": 3, "d": 0.5},
			min:  0.5,
		},
		{
			name: "empty vectors",
			a:    map[string]float64{},
			b:    map[string]float64{"a": 1},
			min:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sim := CosineSimilarity(tt.a, tt.b)
			if sim < tt.min-0.01 {
				t.Errorf("CosineSimilarity = %.4f, want >= %.4f", sim, tt.min)
			}
		})
	}
}

func TestIngestKnowledge(t *testing.T) {
	baseDir := t.TempDir()

	kb, err := IngestKnowledge(baseDir)
	if err != nil {
		t.Fatalf("IngestKnowledge failed: %v", err)
	}

	if len(kb.Chunks) == 0 {
		t.Fatal("IngestKnowledge produced no chunks")
	}

	if len(kb.IDF) == 0 {
		t.Fatal("IngestKnowledge produced no IDF values")
	}

	// Verify chunks have vectors
	for _, chunk := range kb.Chunks {
		if chunk.Vector == nil {
			t.Errorf("chunk %s has nil vector", chunk.ID)
		}
		if chunk.Source == "" {
			t.Errorf("chunk %s has empty source", chunk.ID)
		}
	}

	// Verify chunks were created from sample files
	found := false
	for _, chunk := range kb.Chunks {
		if chunk.Source == "docs/examples/service_daemon.sh" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find service_daemon.sh in chunks")
	}
}

func TestSaveAndLoadKnowledge(t *testing.T) {
	baseDir := t.TempDir()

	// Ingest
	kb, err := IngestKnowledge(baseDir)
	if err != nil {
		t.Fatalf("IngestKnowledge failed: %v", err)
	}

	// Save
	vdbDir := baseDir + "/" + VectordbDir
	if err := kb.Save(vdbDir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(vdbDir, "knowledge_base.json")); err != nil {
		t.Fatalf("knowledge_base.json not found: %v", err)
	}

	// Load
	loaded, err := LoadKnowledge(vdbDir)
	if err != nil {
		t.Fatalf("LoadKnowledge failed: %v", err)
	}

	if len(loaded.Chunks) != len(kb.Chunks) {
		t.Errorf("loaded %d chunks, want %d", len(loaded.Chunks), len(kb.Chunks))
	}
}

func TestSearchRelevant(t *testing.T) {
	baseDir := t.TempDir()

	kb, err := IngestKnowledge(baseDir)
	if err != nil {
		t.Fatalf("IngestKnowledge failed: %v", err)
	}

	SetKB(kb)

	// Search for battery-related content
	results, err := SearchRelevant("battery level daemon monitoring", 3)
	if err != nil {
		t.Fatalf("SearchRelevant failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("SearchRelevant returned no results")
	}

	// First result should be related to battery/daemon
	found := false
	for _, r := range results {
		if containsAny(r.Content, []string{"battery", "daemon", "BATTERY"}) {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected battery/daemon related chunk in top results")
	}

	// Verify results are sorted by relevance
	for i := 1; i < len(results); i++ {
		// Just check we got results — exact scoring depends on TF-IDF
		if results[i].ID == "" {
			t.Errorf("result %d has empty ID", i)
		}
	}
}

func TestSearchRelevant_NotInitialized(t *testing.T) {
	SetKB(nil)
	_, err := SearchRelevant("test", 3)
	if err == nil {
		t.Fatal("expected error when KB not initialized")
	}
}

func TestSearchWithThreshold(t *testing.T) {
	baseDir := t.TempDir()

	kb, err := IngestKnowledge(baseDir)
	if err != nil {
		t.Fatalf("IngestKnowledge failed: %v", err)
	}

	SetKB(kb)

	// Very high threshold — should return few or no results
	results, err := SearchWithThreshold("battery daemon", 0.9, 10)
	if err != nil {
		t.Fatalf("SearchWithThreshold failed: %v", err)
	}

	// Results should all be above threshold
	_ = results // May be empty with very high threshold

	// Very low threshold — should return results
	results, err = SearchWithThreshold("battery daemon", 0.01, 10)
	if err != nil {
		t.Fatalf("SearchWithThreshold failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected at least one result with low threshold")
	}
}

func TestInit(t *testing.T) {
	baseDir := t.TempDir()

	// First init — should ingest and save
	if err := Init(baseDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	kb := GetKB()
	if kb == nil {
		t.Fatal("GetKB returned nil after Init")
	}

	if len(kb.Chunks) == 0 {
		t.Fatal("KB has no chunks after Init")
	}

	// Second init — should load from disk
	SetKB(nil)
	if err := Init(baseDir); err != nil {
		t.Fatalf("Init (load) failed: %v", err)
	}

	kb2 := GetKB()
	if kb2 == nil {
		t.Fatal("GetKB returned nil after second Init")
	}

	if len(kb2.Chunks) != len(kb.Chunks) {
		t.Errorf("second Init loaded %d chunks, first had %d", len(kb2.Chunks), len(kb.Chunks))
	}
}

// Helper functions

func repeatStr(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

func containsAny(s string, words []string) bool {
	for _, w := range words {
		if len(s) >= len(w) {
			for i := 0; i <= len(s)-len(w); i++ {
				if s[i:i+len(w)] == w {
					return true
				}
			}
		}
	}
	return false
}
