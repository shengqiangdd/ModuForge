package rag

import (
	"testing"
)

func TestEmbedAtLevels_Go(t *testing.T) {
	he := NewHierarchicalEmbedder()

	goCode := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}

func helper() int {
	return 42
}
`

	vectors := he.EmbedAtLevels(goCode, "main.go")

	// Should have file + 2 functions + snippets
	if len(vectors) < 4 {
		t.Fatalf("expected at least 4 vectors, got %d", len(vectors))
	}

	// Check levels
	levels := make(map[EmbedLevel]int)
	for _, v := range vectors {
		levels[v.Level]++
	}

	if levels[LevelFile] != 1 {
		t.Errorf("expected 1 file-level vector, got %d", levels[LevelFile])
	}
	if levels[LevelFunction] < 2 {
		t.Errorf("expected at least 2 function-level vectors, got %d", levels[LevelFunction])
	}
	if levels[LevelSnippet] < 1 {
		t.Errorf("expected at least 1 snippet-level vector, got %d", levels[LevelSnippet])
	}
}

func TestEmbedAtLevels_Shell(t *testing.T) {
	he := NewHierarchicalEmbedder()

	shellCode := `#!/system/bin/sh
MODDIR=${0%/*}

log_msg() {
  echo "$1"
}

main() {
  log_msg "hello"
}
`

	vectors := he.EmbedAtLevels(shellCode, "test.sh")

	if len(vectors) < 3 {
		t.Fatalf("expected at least 3 vectors, got %d", len(vectors))
	}

	// Should have file-level
	hasFile := false
	for _, v := range vectors {
		if v.Level == LevelFile {
			hasFile = true
			break
		}
	}
	if !hasFile {
		t.Error("expected file-level vector")
	}
}

func TestSearchHierarchical(t *testing.T) {
	he := NewHierarchicalEmbedder()

	code1 := `package main
import "fmt"
func main() {
	fmt.Println("battery monitor")
}`

	code2 := `package main
import "os"
func helper() {
	os.Exit(0)
}`

	var allVectors []LayeredVector
	allVectors = append(allVectors, he.EmbedAtLevels(code1, "battery.go")...)
	allVectors = append(allVectors, he.EmbedAtLevels(code2, "helper.go")...)

	results := he.SearchHierarchical("battery monitor", allVectors, 3)

	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}

	// First result should be from battery.go
	if results[0].Chunk.Source != "battery.go" {
		t.Errorf("expected first result from battery.go, got %s", results[0].Chunk.Source)
	}
}

func TestSearchHierarchical_Empty(t *testing.T) {
	he := NewHierarchicalEmbedder()
	results := he.SearchHierarchical("test", nil, 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty vectors, got %d", len(results))
	}
}

func TestSplitGoFunctions(t *testing.T) {
	code := `package main

func main() {
	fmt.Println("hello")
}

func helper() int {
	x := 42
	return x
}
`

	functions := splitGoFunctions(splitLines(code), "main.go")
	if len(functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(functions))
	}

	if functions[0].Name != "main" {
		t.Errorf("expected first function main, got %s", functions[0].Name)
	}
	if functions[1].Name != "helper" {
		t.Errorf("expected second function helper, got %s", functions[1].Name)
	}
}

func TestSplitShellFunctions(t *testing.T) {
	code := `#!/system/bin/sh
log_msg() {
  echo "$1"
}
main() {
  log_msg "hello"
}
`

	functions := splitShellFunctions(splitLines(code), "test.sh")
	if len(functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(functions))
	}

	if functions[0].Name != "log_msg" {
		t.Errorf("expected first function log_msg, got %s", functions[0].Name)
	}
	if functions[1].Name != "main" {
		t.Errorf("expected second function main, got %s", functions[1].Name)
	}
}

func TestComputeTFIDFGlobal(t *testing.T) {
	vec := computeTFIDFGlobal("hello world hello foo")
	if len(vec) == 0 {
		t.Fatal("expected non-empty vector")
	}

	// "hello" appears twice, should have higher TF
	if vec["hello"] <= vec["world"] {
		t.Error("hello (2x) should score higher than world (1x)")
	}
}

func TestLayeredVector_JSON(t *testing.T) {
	v := LayeredVector{
		Level:  LevelFunction,
		Vector: map[string]float64{"test": 1.5},
		Source: "main.go",
		Metadata: map[string]string{
			"function": "main",
		},
	}

	if v.Level != LevelFunction {
		t.Errorf("expected LevelFunction, got %s", v.Level)
	}

	if v.Vector["test"] != 1.5 {
		t.Errorf("expected 1.5, got %f", v.Vector["test"])
	}
}

// splitLines is a helper for tests.
func splitLines(s string) []string {
	lines := splitString(s, "\n")
	return lines
}

func splitString(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
		}
	}
	result = append(result, s[start:])
	return result
}
