package service

import (
	"testing"
)

func TestNewProjectService(t *testing.T) {
	svc := NewProjectService(nil, "")
	if svc == nil {
		t.Fatal("NewProjectService returned nil")
	}
	if svc.db != nil {
		t.Fatal("expected nil db")
	}
}

func TestSanitizeProjectPath(t *testing.T) {
	cases := []struct {
		name string
		path string
		ok   bool
	}{
		{"normal nested", "src/main.c", true},
		{"normal with spaces", "my file.txt", true},
		{"unicode", "模块/描述.md", true},
		{"traversal", "../../etc/passwd", false},
		{"traversal single", "..", false},
		{"traversal mixed", "src/../../data/.env", false},
		{"traversal encoded dots", "src/....//data", false},
		{"absolute", "/etc/cron.d/evil", false},
		{"absolute windows", `C:\Windows\system32\evil.exe`, false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := sanitizeProjectPath(tc.path)
			if tc.ok && err != nil {
				t.Fatalf("expected ok, got error: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.path)
			}
		})
	}
}
