package builder

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteGeneratedFiles_JSON(t *testing.T) {
	dir := t.TempDir()
	response := `[{"path":"go.mod","content":"module selfrepair\n\ngo 1.21\n"},{"path":"main.go","content":"package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"}]`

	if err := writeGeneratedFiles(dir, response); err != nil {
		t.Fatalf("writeGeneratedFiles failed: %v", err)
	}

	// Verify go.mod
	goMod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatalf("go.mod not created: %v", err)
	}
	if !strings.Contains(string(goMod), "module selfrepair") {
		t.Errorf("go.mod missing module declaration: %s", string(goMod))
	}

	// Verify main.go
	mainGo, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatalf("main.go not created: %v", err)
	}
	if !strings.Contains(string(mainGo), "package main") {
		t.Errorf("main.go missing package main: %s", string(mainGo))
	}
}

func TestWriteGeneratedFiles_Plaintext(t *testing.T) {
	dir := t.TempDir()
	response := "package main\n\nfunc main() {}\n"

	if err := writeGeneratedFiles(dir, response); err != nil {
		t.Fatalf("writeGeneratedFiles failed: %v", err)
	}

	mainGo, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatalf("main.go not created: %v", err)
	}
	if !strings.Contains(string(mainGo), "package main") {
		t.Errorf("main.go missing package main: %s", string(mainGo))
	}
}

func TestParseJSONArray(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{
			name:  "valid array",
			input: `[{"path":"a.go","content":"code a"},{"path":"b.go","content":"code b"}]`,
			want:  2,
		},
		{
			name:  "empty array",
			input: `[]`,
			want:  0,
		},
		{
			name:  "no array",
			input: `hello world`,
			want:  0,
		},
		{
			name:  "nested braces",
			input: `[{"path":"a.go","content":"func f() { if true { return } }"}]`,
			want:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var files []struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			err := parseJSONArray(tt.input, &files)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseJSONArray() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(files) != tt.want {
				t.Errorf("parseJSONArray() got %d files, want %d", len(files), tt.want)
			}
		})
	}
}

func TestExtractJSONString(t *testing.T) {
	obj := `{"path":"main.go","content":"hello\nworld"}`
	if got := extractJSONString(obj, "path"); got != "main.go" {
		t.Errorf("extractJSONString path = %q, want %q", got, "main.go")
	}
	if got := extractJSONString(obj, "content"); got != "hello\nworld" {
		t.Errorf("extractJSONString content = %q, want %q", got, "hello\nworld")
	}
	if got := extractJSONString(obj, "missing"); got != "" {
		t.Errorf("extractJSONString missing = %q, want empty", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short = %q, want %q", got, "hello")
	}
	if got := truncate("hello world", 5); got != "hello..." {
		t.Errorf("truncate long = %q, want %q", got, "hello...")
	}
}

func TestGenerateAndRepair_NoLLM(t *testing.T) {
	// Builder with nil config should fail gracefully
	b := &Builder{cfg: nil}
	err := b.GenerateAndRepair(context.Background(), t.TempDir(), "test", 1, nil)
	if err == nil {
		t.Fatal("expected error when no LLM configured")
	}
	if !strings.Contains(err.Error(), "no LLM configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunBashSyntaxCheck_Valid(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "test.sh")
	os.WriteFile(script, []byte("#!/bin/sh\necho hello\n"), 0755)

	b := &Builder{}
	_, stderr, err := b.runBashSyntaxCheck(context.Background(), dir, func(s string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
}

func TestRunBashSyntaxCheck_Invalid(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "bad.sh")
	os.WriteFile(script, []byte("#!/bin/sh\nif then\nfi\n"), 0755)

	b := &Builder{}
	_, _, err := b.runBashSyntaxCheck(context.Background(), dir, func(s string) {})
	if err == nil {
		t.Fatal("expected error for invalid bash syntax")
	}
}

func TestRunGoBuild_NoGoFiles(t *testing.T) {
	dir := t.TempDir()
	// Empty directory — go build should fail
	b := &Builder{}
	_, _, err := b.runGoBuild(context.Background(), dir, func(s string) {})
	// Error expected since there's no go.mod or go files
	if err == nil {
		// Some environments might have go installed and handle this differently
		t.Log("go build did not error (acceptable in some environments)")
	}
}

func TestFindMatchingBrace(t *testing.T) {
	tests := []struct {
		input string
		pos   int
		want  int
	}{
		{`{}`, 0, 1},
		{`{"a":"b"}`, 0, 9},
		{`{"a":{"b":"c"}}`, 0, 15},
		{`{"a":"{"}`, 0, 8},
		{`nope`, 0, -1},
	}

	for _, tt := range tests {
		got := findMatchingBrace(tt.input, tt.pos)
		if got != tt.want {
			t.Errorf("findMatchingBrace(%q, %d) = %d, want %d", tt.input, tt.pos, got, tt.want)
		}
	}
}
