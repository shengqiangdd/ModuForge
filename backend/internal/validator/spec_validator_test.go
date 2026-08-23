package validator

import (
	"strings"
	"testing"
)

func TestNewSpecValidator(t *testing.T) {
	v := NewSpecValidator()
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
}

func TestValidateSpec_GoodSpec(t *testing.T) {
	v := NewSpecValidator()

	spec := Spec{
		Name:         "battery-monitor",
		Version:      "1.0",
		Description:  "Monitors battery",
		TargetSystem: "Magisk",
		Files: []SpecFile{
			{Path: "module.prop", Purpose: "metadata", Language: "prop", RequiredVars: []string{"MODID"}},
			{Path: "customize.sh", Purpose: "installer", Language: "shell"},
		},
		BoundaryConditions: []string{"Android 8.0+"},
		TestCases:          []TestCase{{Name: "install"}},
	}

	errs := v.ValidateSpec(spec)

	// Should have no errors
	for _, e := range errs {
		if e.Severity == SeverityError {
			t.Errorf("unexpected error: %s - %s", e.Field, e.Message)
		}
	}
}

func TestValidateSpec_MissingName(t *testing.T) {
	v := NewSpecValidator()

	spec := Spec{
		Version: "1.0",
		Files:   []SpecFile{{Path: "module.prop"}},
	}

	errs := v.ValidateSpec(spec)
	found := false
	for _, e := range errs {
		if e.Field == "name" && e.Severity == SeverityError {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for missing name")
	}
}

func TestValidateSpec_MissingVersion(t *testing.T) {
	v := NewSpecValidator()

	spec := Spec{
		Name:  "test",
		Files: []SpecFile{{Path: "module.prop"}},
	}

	errs := v.ValidateSpec(spec)
	found := false
	for _, e := range errs {
		if e.Field == "version" && e.Severity == SeverityError {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for missing version")
	}
}

func TestValidateSpec_NoFiles(t *testing.T) {
	v := NewSpecValidator()

	spec := Spec{
		Name:    "test",
		Version: "1.0",
	}

	errs := v.ValidateSpec(spec)
	found := false
	for _, e := range errs {
		if e.Field == "files" && e.Severity == SeverityError {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for no files")
	}
}

func TestValidateSpec_DuplicateVars(t *testing.T) {
	v := NewSpecValidator()

	spec := Spec{
		Name:    "test",
		Version: "1.0",
		Files: []SpecFile{
			{Path: "a.sh", RequiredVars: []string{"MODPATH"}},
			{Path: "b.sh", RequiredVars: []string{"MODPATH"}},
		},
	}

	errs := v.ValidateSpec(spec)
	found := false
	for _, e := range errs {
		if e.Field == "files.b.sh.required_vars" && strings.Contains(e.Message, "already declared") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning for duplicate variable")
	}
}

func TestValidateSpec_AbsolutePath(t *testing.T) {
	v := NewSpecValidator()

	spec := Spec{
		Name:    "test",
		Version: "1.0",
		Files: []SpecFile{
			{Path: "/system/bin/test"},
		},
	}

	errs := v.ValidateSpec(spec)
	found := false
	for _, e := range errs {
		if e.Severity == SeverityError && strings.Contains(e.Message, "absolute") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for absolute path")
	}
}

func TestValidateSpec_EmptyPath(t *testing.T) {
	v := NewSpecValidator()

	spec := Spec{
		Name:    "test",
		Version: "1.0",
		Files: []SpecFile{
			{Path: ""},
		},
	}

	errs := v.ValidateSpec(spec)
	found := false
	for _, e := range errs {
		if e.Severity == SeverityError && strings.Contains(e.Message, "empty") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for empty path")
	}
}

func TestValidateSpec_NoTestCases(t *testing.T) {
	v := NewSpecValidator()

	spec := Spec{
		Name:    "test",
		Version: "1.0",
		Files:   []SpecFile{{Path: "a.sh"}, {Path: "b.sh"}},
	}

	errs := v.ValidateSpec(spec)
	found := false
	for _, e := range errs {
		if e.Field == "test_cases" && e.Severity == SeverityWarning {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning for no test cases")
	}
}

func TestValidateSpec_EmptyTestCaseName(t *testing.T) {
	v := NewSpecValidator()

	spec := Spec{
		Name:    "test",
		Version: "1.0",
		Files:   []SpecFile{{Path: "a.sh"}},
		TestCases: []TestCase{
			{Name: ""},
		},
	}

	errs := v.ValidateSpec(spec)
	found := false
	for _, e := range errs {
		if e.Severity == SeverityError && strings.Contains(e.Message, "name") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for empty test case name")
	}
}

func TestValidateCodeVsSpec_GoodMatch(t *testing.T) {
	v := NewSpecValidator()

	spec := Spec{
		Name:    "test",
		Version: "1.0",
		Files: []SpecFile{
			{
				Path:      "main.go",
				Functions: []FuncSpec{{Name: "main"}, {Name: "helper"}},
			},
		},
	}

	files := []GeneratedFile{
		{Path: "main.go", Content: "package main\n\nfunc main() {}\n\nfunc helper() {}"},
	}

	errs := v.ValidateCodeVsSpec(spec, files)
	for _, e := range errs {
		if e.Severity == SeverityError {
			t.Errorf("unexpected error: %s", e.Message)
		}
	}
}

func TestValidateCodeVsSpec_MissingFile(t *testing.T) {
	v := NewSpecValidator()

	spec := Spec{
		Name:    "test",
		Version: "1.0",
		Files: []SpecFile{
			{Path: "main.go"},
			{Path: "helper.go"},
		},
	}

	files := []GeneratedFile{
		{Path: "main.go", Content: "package main"},
	}

	errs := v.ValidateCodeVsSpec(spec, files)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "helper.go") && strings.Contains(e.Message, "not found") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for missing file helper.go")
	}
}

func TestValidateCodeVsSpec_MissingFunction(t *testing.T) {
	v := NewSpecValidator()

	spec := Spec{
		Name:    "test",
		Version: "1.0",
		Files: []SpecFile{
			{
				Path:      "main.go",
				Functions: []FuncSpec{{Name: "main"}, {Name: "monitor"}},
			},
		},
	}

	files := []GeneratedFile{
		{Path: "main.go", Content: "package main\n\nfunc main() {}"},
	}

	errs := v.ValidateCodeVsSpec(spec, files)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "monitor") && strings.Contains(e.Message, "not found") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for missing function monitor")
	}
}

func TestValidateCodeVsSpec_ExtraFile(t *testing.T) {
	v := NewSpecValidator()

	spec := Spec{
		Name:    "test",
		Version: "1.0",
		Files: []SpecFile{
			{Path: "main.go"},
		},
	}

	files := []GeneratedFile{
		{Path: "main.go", Content: "package main"},
		{Path: "extra.go", Content: "package extra"},
	}

	errs := v.ValidateCodeVsSpec(spec, files)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "extra.go") && strings.Contains(e.Message, "not in spec") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning for extra file extra.go")
	}
}

func TestValidateCodeVsSpec_Empty(t *testing.T) {
	v := NewSpecValidator()

	spec := Spec{
		Name:    "test",
		Version: "1.0",
		Files:   []SpecFile{},
	}

	files := []GeneratedFile{}
	errs := v.ValidateCodeVsSpec(spec, files)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors for empty spec, got %d", len(errs))
	}
}

func TestValidationError_Fields(t *testing.T) {
	e := ValidationError{
		Field:      "files",
		Message:    "test message",
		Severity:   SeverityError,
		Suggestion: "test suggestion",
	}

	if e.Field != "files" {
		t.Errorf("expected files, got %s", e.Field)
	}

	if e.Severity != SeverityError {
		t.Errorf("expected error severity, got %s", e.Severity)
	}
}
