package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewSpecGenerator_NoLLM(t *testing.T) {
	sg := NewSpecGenerator()
	if sg != nil {
		t.Log("LLM configured, skipping no-LLM test")
		return
	}
}

func TestGenerateSpec_NilGenerator(t *testing.T) {
	var sg *SpecGenerator
	_, err := sg.GenerateSpec(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for nil SpecGenerator")
	}
}

func TestSpec_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	spec := Spec{
		Name:         "battery-monitor",
		Version:      "1.0",
		TargetSystem: "Magisk",
		Description:  "Monitors battery level",
		Author:       "ModuForge",
		Files: []SpecFile{
			{
				Path:         "module.prop",
				Purpose:      "metadata",
				Language:     "prop",
				RequiredVars: []string{"MODID", "MODVER"},
			},
			{
				Path:     "customize.sh",
				Purpose:  "installer",
				Language: "shell",
				Functions: []FuncSpec{
					{Name: "install_module", Params: []string{}, ReturnType: "void"},
				},
			},
		},
		BoundaryConditions: []string{"Must work on Android 8.0+", "CPU < 5%"},
		TestCases: []TestCase{
			{Name: "install succeeds", ExpectedOutput: "module installed"},
		},
	}

	// Save
	if err := spec.SaveSpec(dir); err != nil {
		t.Fatalf("SaveSpec failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(dir, "module_spec.json")); err != nil {
		t.Fatalf("module_spec.json not found: %v", err)
	}

	// Load
	loaded, err := LoadSpec(dir)
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	if loaded.Name != "battery-monitor" {
		t.Errorf("expected name battery-monitor, got %s", loaded.Name)
	}

	if loaded.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", loaded.Version)
	}

	if loaded.TargetSystem != "Magisk" {
		t.Errorf("expected target_system Magisk, got %s", loaded.TargetSystem)
	}

	if len(loaded.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(loaded.Files))
	}

	if loaded.Files[0].Path != "module.prop" {
		t.Errorf("expected first file module.prop, got %s", loaded.Files[0].Path)
	}

	if len(loaded.Files[0].RequiredVars) != 2 {
		t.Errorf("expected 2 required vars, got %d", len(loaded.Files[0].RequiredVars))
	}

	if len(loaded.BoundaryConditions) != 2 {
		t.Errorf("expected 2 boundary conditions, got %d", len(loaded.BoundaryConditions))
	}

	if len(loaded.TestCases) != 1 {
		t.Errorf("expected 1 test case, got %d", len(loaded.TestCases))
	}
}

func TestSpec_JSON(t *testing.T) {
	spec := Spec{
		Name:         "test-module",
		Version:      "2.0",
		TargetSystem: "KernelSU",
		Description:  "A test",
		Files: []SpecFile{
			{
				Path:         "src/main.go",
				Purpose:      "daemon",
				Language:     "go",
				RequiredVars: []string{"interval"},
				Functions: []FuncSpec{
					{Name: "main", ReturnType: "void"},
					{Name: "monitor", Params: []string{"interval int"}, ReturnType: "error"},
				},
			},
		},
		BoundaryConditions: []string{"no cgo"},
		TestCases: []TestCase{
			{Name: "start", Input: "valid config", ExpectedOutput: "running"},
		},
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed Spec
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed.TargetSystem != "KernelSU" {
		t.Errorf("expected KernelSU, got %s", parsed.TargetSystem)
	}

	if len(parsed.Files[0].Functions) != 2 {
		t.Errorf("expected 2 functions, got %d", len(parsed.Files[0].Functions))
	}
}

func TestLoadSpec_NotFound(t *testing.T) {
	_, err := LoadSpec("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestFuncSpec_JSON(t *testing.T) {
	fn := FuncSpec{
		Name:        "monitor",
		Params:      []string{"interval int", "threshold float64"},
		ReturnType:  "error",
		Description: "monitors battery",
	}

	data, err := json.Marshal(fn)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed FuncSpec
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed.Name != "monitor" {
		t.Errorf("expected monitor, got %s", parsed.Name)
	}

	if len(parsed.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(parsed.Params))
	}
}
