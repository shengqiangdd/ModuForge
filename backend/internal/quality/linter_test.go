package quality

import (
	"strings"
	"testing"
)

func TestNewMagiskLinter(t *testing.T) {
	l := NewMagiskLinter()
	if l == nil {
		t.Fatal("expected non-nil linter")
	}
	if len(l.rules) < 6 {
		t.Errorf("expected at least 6 built-in rules, got %d", len(l.rules))
	}
}

func TestLint_NoIssues(t *testing.T) {
	l := NewMagiskLinter()

	files := []GeneratedFile{
		{
			Path:    "module.prop",
			Content: "id=test\nname=Test\nversion=1.0",
		},
	}

	issues := l.Lint(files)
	// module.prop should have no issues
	for _, issue := range issues {
		if issue.File == "module.prop" {
			t.Errorf("unexpected issue for module.prop: %s", issue.Message)
		}
	}
}

func TestLint_MustDefineModpath(t *testing.T) {
	l := NewMagiskLinter()

	files := []GeneratedFile{
		{
			Path:    "customize.sh",
			Content: "#!/system/bin/sh\necho hello",
		},
	}

	issues := l.Lint(files)
	found := false
	for _, issue := range issues {
		if issue.Rule == "must-define-modpath" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected must-define-modpath issue")
	}
}

func TestLint_MustDefineModpath_OK(t *testing.T) {
	l := NewMagiskLinter()

	files := []GeneratedFile{
		{
			Path:    "customize.sh",
			Content: "#!/system/bin/sh\nMODPATH=${0%/*}\necho ${MODPATH}",
		},
	}

	issues := l.Lint(files)
	for _, issue := range issues {
		if issue.Rule == "must-define-modpath" {
			t.Error("should not have must-define-modpath issue when MODPATH is defined")
		}
	}
}

func TestLint_NoDangerousRM(t *testing.T) {
	l := NewMagiskLinter()

	files := []GeneratedFile{
		{
			Path:    "uninstall.sh",
			Content: "#!/system/bin/sh\nrm -rf /",
		},
	}

	issues := l.Lint(files)
	found := false
	for _, issue := range issues {
		if issue.Rule == "no-dangerous-rm" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected no-dangerous-rm issue")
	}
}

func TestLint_ShellShebang(t *testing.T) {
	l := NewMagiskLinter()

	files := []GeneratedFile{
		{
			Path:    "test.sh",
			Content: "echo hello",
		},
	}

	issues := l.Lint(files)
	found := false
	for _, issue := range issues {
		if issue.Rule == "shell-shebang" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected shell-shebang issue")
	}
}

func TestLint_VariableExpansion(t *testing.T) {
	l := NewMagiskLinter()

	files := []GeneratedFile{
		{
			Path:    "test.sh",
			Content: "#!/system/bin/sh\necho $HOME",
		},
	}

	issues := l.Lint(files)
	found := false
	for _, issue := range issues {
		if issue.Rule == "variable-expansion" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected variable-expansion issue")
	}
}

func TestLint_SetPermCompleteness(t *testing.T) {
	l := NewMagiskLinter()

	files := []GeneratedFile{
		{
			Path:    "customize.sh",
			Content: "#!/system/bin/sh\nset_perm ${MODPATH}/bin/test",
		},
	}

	issues := l.Lint(files)
	found := false
	for _, issue := range issues {
		if issue.Rule == "set-perm-completeness" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected set-perm-completeness issue")
	}
}

func TestLint_SetPermCompleteness_OK(t *testing.T) {
	l := NewMagiskLinter()

	files := []GeneratedFile{
		{
			Path:    "customize.sh",
			Content: "#!/system/bin/sh\nset_perm ${MODPATH}/bin/test 0 0 0755",
		},
	}

	issues := l.Lint(files)
	for _, issue := range issues {
		if issue.Rule == "set-perm-completeness" {
			t.Error("should not have set-perm-completeness issue with all params")
		}
	}
}

func TestLint_GoErrorHandling(t *testing.T) {
	l := NewMagiskLinter()

	files := []GeneratedFile{
		{
			Path:    "main.go",
			Content: "package main\n\nfunc main() {\n\tos.ReadFile(\"test.txt\")\n}",
		},
	}

	issues := l.Lint(files)
	found := false
	for _, issue := range issues {
		if issue.Rule == "go-error-handling" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected go-error-handling issue")
	}
}

func TestLint_CustomRule(t *testing.T) {
	l := NewMagiskLinter()

	// Add custom rule
	l.AddRule(LintRule{
		Name:        "no-prints",
		Description: "No print statements allowed",
		Severity:    LintWarning,
		Check: func(file, content string) []LintIssue {
			if strings.Contains(content, "fmt.Print") {
				return []LintIssue{{
					File:     file,
					Rule:     "no-prints",
					Severity: LintWarning,
					Message:  "print statement found",
				}}
			}
			return nil
		},
	})

	files := []GeneratedFile{
		{
			Path:    "main.go",
			Content: "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}",
		},
	}

	issues := l.Lint(files)
	found := false
	for _, issue := range issues {
		if issue.Rule == "no-prints" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected custom no-prints issue")
	}
}

func TestLint_MultipleFiles(t *testing.T) {
	l := NewMagiskLinter()

	files := []GeneratedFile{
		{Path: "customize.sh", Content: "#!/system/bin/sh\necho test"},
		{Path: "service.sh", Content: "echo test"},
	}

	issues := l.Lint(files)

	// customize.sh should have MODPATH issue
	// service.sh should have shebang issue
	customizeHasIssue := false
	serviceHasIssue := false
	for _, issue := range issues {
		if issue.File == "customize.sh" {
			customizeHasIssue = true
		}
		if issue.File == "service.sh" {
			serviceHasIssue = true
		}
	}

	if !customizeHasIssue {
		t.Error("expected issue for customize.sh")
	}
	if !serviceHasIssue {
		t.Error("expected issue for service.sh")
	}
}

func TestLintIssue_Fields(t *testing.T) {
	issue := LintIssue{
		File:     "test.sh",
		Line:     5,
		Rule:     "test-rule",
		Severity: LintError,
		Message:  "test message",
		Fix:      "test fix",
	}

	if issue.File != "test.sh" {
		t.Errorf("expected test.sh, got %s", issue.File)
	}

	if issue.Line != 5 {
		t.Errorf("expected line 5, got %d", issue.Line)
	}
}
