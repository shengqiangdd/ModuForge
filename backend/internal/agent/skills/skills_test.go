package skills

import (
	"testing"
)

func TestResolveProjectPath_EmptyProjectID(t *testing.T) {
	result := ResolveProjectPath(nil, "/projects", "")
	if result != "/projects" {
		t.Errorf("expected '/projects', got %q", result)
	}
}

func TestResolveProjectPath_NilDB(t *testing.T) {
	result := ResolveProjectPath(nil, "/projects", "abc123")
	expected := "/projects/abc123"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"src/main.rs", "rust"},
		{"cmd/app.go", "go"},
		{"script.sh", "shell"},
		{"app.py", "python"},
		{"index.html", "html"},
		{"style.css", "css"},
		{"data.json", "json"},
		{"config.yaml", "yaml"},
		{"readme.md", "markdown"},
		{"Dockerfile", "dockerfile"},
		{"Makefile", "makefile"},
		{"unknown.xyz", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := detectLanguage(tt.path)
			if result != tt.expected {
				t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestDetectModuleType(t *testing.T) {
	tests := []struct {
		content  string
		expected string
	}{
		{"id=com.kernelsu.module", "kernelsu"},
		{"id=com.topjohnwu.magisk", "magisk"},
		{"id=com.bmax.apatch", "apatch"},
		{"id=com.universal.module", "universal"},
		{"", "universal"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := detectModuleType(tt.content)
			if result != tt.expected {
				t.Errorf("detectModuleType(%q) = %q, want %q", tt.content, result, tt.expected)
			}
		})
	}
}
