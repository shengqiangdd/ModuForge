package mcp

import (
	"testing"
	"time"
)

func TestNewMarketplace(t *testing.T) {
	m := NewMarketplace(t.TempDir())
	if m == nil {
		t.Fatal("expected non-nil marketplace")
	}
}

func TestRegisterAndGetTool(t *testing.T) {
	m := NewMarketplace(t.TempDir())

	tool := ToolDefinition{
		Name:        "test_tool",
		Description: "A test tool",
		Version:     "1.0",
		Author:      "tester",
		Parameters: []ParamDef{
			{Name: "input", Type: "string", Required: true, Description: "input data"},
		},
	}

	if err := m.RegisterTool(tool); err != nil {
		t.Fatalf("RegisterTool failed: %v", err)
	}

	got, ok := m.GetTool("test_tool")
	if !ok {
		t.Fatal("expected tool to be found")
	}

	if got.Name != "test_tool" {
		t.Errorf("expected test_tool, got %s", got.Name)
	}

	if got.Version != "1.0" {
		t.Errorf("expected 1.0, got %s", got.Version)
	}

	if len(got.Parameters) != 1 {
		t.Errorf("expected 1 param, got %d", len(got.Parameters))
	}
}

func TestUnregisterTool(t *testing.T) {
	m := NewMarketplace(t.TempDir())

	m.RegisterTool(ToolDefinition{Name: "to_delete", Version: "1.0"})

	if err := m.UnregisterTool("to_delete"); err != nil {
		t.Fatalf("UnregisterTool failed: %v", err)
	}

	_, ok := m.GetTool("to_delete")
	if ok {
		t.Error("expected tool to be deleted")
	}
}

func TestUnregisterTool_NotFound(t *testing.T) {
	m := NewMarketplace(t.TempDir())

	err := m.UnregisterTool("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}
}

func TestListTools(t *testing.T) {
	m := NewMarketplace(t.TempDir())

	m.RegisterTool(ToolDefinition{Name: "tool1", Version: "1.0"})
	m.RegisterTool(ToolDefinition{Name: "tool2", Version: "2.0"})

	tools := m.ListTools()
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}

func TestSearchTools(t *testing.T) {
	m := NewMarketplace(t.TempDir())

	m.RegisterTool(ToolDefinition{Name: "code_generator", Description: "generates code", Version: "1.0"})
	m.RegisterTool(ToolDefinition{Name: "test_runner", Description: "runs tests", Version: "1.0"})
	m.RegisterTool(ToolDefinition{Name: "code_linter", Description: "lints code", Version: "1.0"})

	results := m.SearchTools("code")
	if len(results) < 2 {
		t.Errorf("expected at least 2 results for 'code', got %d", len(results))
	}
}

func TestSearchTools_Empty(t *testing.T) {
	m := NewMarketplace(t.TempDir())
	m.RegisterTool(ToolDefinition{Name: "tool1", Description: "test", Version: "1.0"})

	results := m.SearchTools("")
	if len(results) != 1 {
		t.Errorf("expected 1 result for empty query, got %d", len(results))
	}
}

func TestCount(t *testing.T) {
	m := NewMarketplace(t.TempDir())

	m.RegisterTool(ToolDefinition{Name: "a", Version: "1.0"})
	m.RegisterTool(ToolDefinition{Name: "b", Version: "1.0"})

	if m.Count() != 2 {
		t.Errorf("expected 2, got %d", m.Count())
	}
}

func TestRegisterTool_EmptyName(t *testing.T) {
	m := NewMarketplace(t.TempDir())

	err := m.RegisterTool(ToolDefinition{Version: "1.0"})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestRegisterTool_Timestamp(t *testing.T) {
	m := NewMarketplace(t.TempDir())

	m.RegisterTool(ToolDefinition{Name: "ts_test", Version: "1.0"})

	got, _ := m.GetTool("ts_test")
	if got.RegisteredAt.IsZero() {
		t.Error("expected non-zero RegisteredAt")
	}
}

func TestToolDefinition_Fields(t *testing.T) {
	tool := ToolDefinition{
		Name:               "deploy",
		Description:        "Deploy module",
		PermissionRequired: "admin",
		Version:            "2.0",
		Author:             "moduforge",
	}

	if tool.PermissionRequired != "admin" {
		t.Errorf("expected admin, got %s", tool.PermissionRequired)
	}
}

func TestParamDef_Fields(t *testing.T) {
	param := ParamDef{
		Name:        "path",
		Type:        "string",
		Required:    true,
		Description: "file path",
		Default:     "/tmp",
	}

	if !param.Required {
		t.Error("expected required=true")
	}

	if param.Default != "/tmp" {
		t.Errorf("expected /tmp, got %s", param.Default)
	}
}

func TestMarketplace_Persistence(t *testing.T) {
	dir := t.TempDir()

	// Save
	m1 := NewMarketplace(dir)
	m1.RegisterTool(ToolDefinition{Name: "persist_test", Version: "1.0"})

	// Load
	m2 := NewMarketplace(dir)
	got, ok := m2.GetTool("persist_test")
	if !ok {
		t.Fatal("expected tool to persist")
	}

	if got.Name != "persist_test" {
		t.Errorf("expected persist_test, got %s", got.Name)
	}
}
