package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewArchitect_NoLLM(t *testing.T) {
	a := NewArchitect()
	if a != nil {
		t.Log("LLM configured, skipping no-LLM test")
		return
	}
}

func TestDesignModule_NilArchitect(t *testing.T) {
	var a *Architect
	_, err := a.DesignModule(context.Background(), TaskGraph{}, "test")
	if err == nil {
		t.Fatal("expected error for nil Architect")
	}
}

func TestModuleDesign_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	design := ModuleDesign{
		ModuleName:   "test-module",
		Version:      "1.0",
		Description:  "A test module",
		TargetSystem: "Android 8.0+",
		Author:       "ModuForge",
		Files: []FileDesign{
			{
				Path:     "module.prop",
				Purpose:  "metadata",
				Language: "prop",
				Variables: []VarDef{
					{Name: "MODID", Type: "string", Default: "test_module"},
				},
			},
			{
				Path:     "customize.sh",
				Purpose:  "installer",
				Language: "shell",
				Functions: []FuncSignature{
					{Name: "install_module", Signature: "install_module()"},
				},
			},
		},
	}

	// Save
	if err := design.SaveDesign(dir); err != nil {
		t.Fatalf("SaveDesign failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(dir, "module_design.json")); err != nil {
		t.Fatalf("module_design.json not found: %v", err)
	}

	// Load
	loaded, err := LoadDesign(dir)
	if err != nil {
		t.Fatalf("LoadDesign failed: %v", err)
	}

	if loaded.ModuleName != "test-module" {
		t.Errorf("expected module_name test-module, got %s", loaded.ModuleName)
	}

	if loaded.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", loaded.Version)
	}

	if len(loaded.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(loaded.Files))
	}

	if loaded.Files[0].Path != "module.prop" {
		t.Errorf("expected first file module.prop, got %s", loaded.Files[0].Path)
	}
}

func TestModuleDesign_JSON(t *testing.T) {
	design := ModuleDesign{
		ModuleName:  "battery-monitor",
		Version:     "2.0",
		Description: "Monitors battery",
		Files: []FileDesign{
			{
				Path:     "src/main.go",
				Purpose:  "daemon",
				Language: "go",
				Functions: []FuncSignature{
					{Name: "main", Signature: "func main()"},
				},
				Variables: []VarDef{
					{Name: "checkInterval", Type: "int", Default: "60"},
				},
			},
		},
	}

	data, err := json.Marshal(design)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed ModuleDesign
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed.Files[0].Language != "go" {
		t.Errorf("expected language go, got %s", parsed.Files[0].Language)
	}

	if len(parsed.Files[0].Functions) != 1 {
		t.Errorf("expected 1 function, got %d", len(parsed.Files[0].Functions))
	}
}

func TestLoadDesign_NotFound(t *testing.T) {
	_, err := LoadDesign("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}
