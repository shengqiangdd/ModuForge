package mcp

import (
	"testing"
)

func TestNewToolSelector(t *testing.T) {
	s := NewToolSelector()
	if s == nil {
		t.Fatal("expected non-nil selector")
	}
}

func TestSelectTool_BestMatch(t *testing.T) {
	s := NewToolSelector()

	tools := []ToolDefinition{
		{Name: "code_generator", Description: "generates Go code from description"},
		{Name: "test_runner", Description: "runs unit tests"},
		{Name: "code_linter", Description: "lints Go code for issues"},
	}

	selected := s.SelectTool("generate Go code", tools)
	if selected.Name != "code_generator" {
		t.Errorf("expected code_generator, got %s", selected.Name)
	}
}

func TestSelectTool_NoMatch(t *testing.T) {
	s := NewToolSelector()

	tools := []ToolDefinition{
		{Name: "deploy", Description: "deploys module"},
	}

	selected := s.SelectTool("run tests", tools)
	// Should still return a tool (the only one available)
	if selected.Name != "deploy" {
		t.Errorf("expected deploy, got %s", selected.Name)
	}
}

func TestSelectTool_Empty(t *testing.T) {
	s := NewToolSelector()

	selected := s.SelectTool("anything", nil)
	if selected.Name != "" {
		t.Errorf("expected empty, got %s", selected.Name)
	}
}

func TestSelectTool_ExactNameMatch(t *testing.T) {
	s := NewToolSelector()

	tools := []ToolDefinition{
		{Name: "lint_code", Description: "linter"},
		{Name: "build_module", Description: "builder"},
	}

	selected := s.SelectTool("lint_code this file", tools)
	if selected.Name != "lint_code" {
		t.Errorf("expected lint_code, got %s", selected.Name)
	}
}

func TestBuildPipeline(t *testing.T) {
	s := NewToolSelector()

	tasks := []string{
		"generate code",
		"lint code",
		"run tests",
	}

	tools := []ToolDefinition{
		{Name: "code_generator", Description: "generates code"},
		{Name: "code_linter", Description: "lints code"},
		{Name: "test_runner", Description: "runs tests"},
	}

	pipeline := s.BuildPipeline(tasks, tools)

	if len(pipeline.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(pipeline.Steps))
	}

	// First step should have no dependencies
	if len(pipeline.Steps[0].DependsOn) != 0 {
		t.Error("first step should have no dependencies")
	}

	// Second step depends on first
	if len(pipeline.Steps[1].DependsOn) != 1 {
		t.Errorf("second step should depend on 1, got %d", len(pipeline.Steps[1].DependsOn))
	}

	// Third step depends on second
	if len(pipeline.Steps[2].DependsOn) != 1 {
		t.Errorf("third step should depend on 1, got %d", len(pipeline.Steps[2].DependsOn))
	}
}

func TestBuildPipeline_Empty(t *testing.T) {
	s := NewToolSelector()

	pipeline := s.BuildPipeline(nil, nil)
	if len(pipeline.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(pipeline.Steps))
	}
}

func TestScoreTool(t *testing.T) {
	s := NewToolSelector()

	tool := ToolDefinition{
		Name:        "code_generator",
		Description: "generates Go code",
		Parameters:  []ParamDef{{Name: "desc"}},
	}

	score := s.scoreTool("generate Go code", tool)
	if score <= 0 {
		t.Errorf("expected positive score, got %.1f", score)
	}
}

func TestScoreTool_NoMatch(t *testing.T) {
	s := NewToolSelector()

	tool := ToolDefinition{
		Name:        "deploy",
		Description: "deploys module",
	}

	score := s.scoreTool("generate code", tool)
	if score > 0 {
		t.Errorf("expected low score, got %.1f", score)
	}
}

func TestPipelineStep_Fields(t *testing.T) {
	step := PipelineStep{
		ToolName:      "test_tool",
		InputMapping:  "input",
		OutputMapping: "output",
		DependsOn:     []string{"prev_tool"},
	}

	if step.ToolName != "test_tool" {
		t.Errorf("expected test_tool, got %s", step.ToolName)
	}

	if len(step.DependsOn) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(step.DependsOn))
	}
}
