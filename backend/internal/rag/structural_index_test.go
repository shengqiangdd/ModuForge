package rag

import (
	"testing"
)

func TestParseAndIndex_Go(t *testing.T) {
	si := NewStructuralIndex()

	goCode := `package main

import (
	"fmt"
	"os"
)

type Config struct {
	Name string
}

var globalVar = "hello"

func main() {
	fmt.Println("hello")
	os.Exit(0)
}

func helper() int {
	return 42
}
`

	si.ParseAndIndex("main.go", goCode)
	elems := si.Elements()

	// Should find package, imports, struct, var, functions
	if len(elems) < 5 {
		t.Fatalf("expected at least 5 elements, got %d", len(elems))
	}

	// Check types
	types := make(map[ElementType]int)
	for _, e := range elems {
		types[e.Type]++
	}

	if types[ElementPackage] < 1 {
		t.Error("expected at least 1 package element")
	}
	if types[ElementImport] < 2 {
		t.Error("expected at least 2 import elements")
	}
	if types[ElementFunction] < 2 {
		t.Error("expected at least 2 function elements")
	}
}

func TestParseAndIndex_Shell(t *testing.T) {
	si := NewStructuralIndex()

	shellCode := `#!/system/bin/sh
MODDIR=${0%/*}
LOGFILE=/data/local/tmp/test.log

log_msg() {
  echo "$1" >> "${LOGFILE}"
}

main() {
  log_msg "starting"
}
`

	si.ParseAndIndex("test.sh", shellCode)
	elems := si.Elements()

	// Should find variables and functions
	if len(elems) < 3 {
		t.Fatalf("expected at least 3 elements, got %d", len(elems))
	}

	types := make(map[ElementType]int)
	for _, e := range elems {
		types[e.Type]++
	}

	if types[ElementShellVar] < 2 {
		t.Error("expected at least 2 shell variable elements")
	}
	if types[ElementShellFunc] < 2 {
		t.Error("expected at least 2 shell function elements")
	}
}

func TestBuildRelationGraph(t *testing.T) {
	si := NewStructuralIndex()

	goCode := `package main

import "fmt"

func main() {
	helper()
	fmt.Println("done")
}

func helper() {
	fmt.Println("helper")
}
`

	si.ParseAndIndex("main.go", goCode)
	si.BuildRelationGraph()
	graph := si.Graph()

	// helper should be called by main
	if callers, ok := graph.Callers["helper"]; ok {
		if len(callers) == 0 {
			t.Error("expected callers for helper")
		}
	} else {
		t.Error("expected callers map to contain helper")
	}

	// main should be in callees
	if _, ok := graph.Callees["main"]; !ok {
		t.Error("expected callees map to contain main")
	}
}

func TestQueryBySymbol(t *testing.T) {
	si := NewStructuralIndex()

	goCode := `package main

func main() {
	compute(42)
}

func compute(x int) int {
	return x * 2
}
`

	si.ParseAndIndex("main.go", goCode)
	si.BuildRelationGraph()

	// Query for compute
	results := si.QueryBySymbol("compute")
	if len(results) == 0 {
		t.Fatal("expected results for 'compute'")
	}

	// Should find both the definition and the caller
	foundDef := false
	foundCaller := false
	for _, r := range results {
		if r.Name == "compute" && r.Type == ElementFunction {
			foundDef = true
		}
		if r.Name == "main" {
			foundCaller = true
		}
	}

	if !foundDef {
		t.Error("expected to find compute definition")
	}
	if !foundCaller {
		t.Error("expected to find main as caller of compute")
	}
}

func TestExtractCalls(t *testing.T) {
	calls := extractCalls(`fmt.Println("hello") helper()`)
	if len(calls) < 2 {
		t.Errorf("expected at least 2 calls, got %d: %v", len(calls), calls)
	}

	// Should find Println and helper
	found := make(map[string]bool)
	for _, c := range calls {
		found[c] = true
	}
	if !found["Println"] {
		t.Error("expected Println in calls")
	}
	if !found["helper"] {
		t.Error("expected helper in calls")
	}
}

func TestIsGoKeyword(t *testing.T) {
	if !isGoKeyword("func") {
		t.Error("func should be keyword")
	}
	if !isGoKeyword("if") {
		t.Error("if should be keyword")
	}
	if isGoKeyword("helper") {
		t.Error("helper should not be keyword")
	}
	if isGoKeyword("Println") {
		t.Error("Println should not be keyword")
	}
}

func TestStructuralIndex_Empty(t *testing.T) {
	si := NewStructuralIndex()

	if len(si.Elements()) != 0 {
		t.Error("expected empty elements")
	}

	si.BuildRelationGraph()
	graph := si.Graph()
	if len(graph.Callees) != 0 {
		t.Error("expected empty callees")
	}
}
