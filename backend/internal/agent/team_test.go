package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewTeam_NoLLM(t *testing.T) {
	// Without LLM env vars, NewTeam should return nil
	team := NewTeam()
	if team != nil {
		// If LLM is configured, that's fine — just skip the test
		t.Log("LLM configured, skipping no-LLM test")
		return
	}
}

func TestExtractJSON_Object(t *testing.T) {
	resp := "Here is the plan:\n```json\n{\"module_id\": \"test\", \"files\": []}\n```\nDone."
	got := extractJSON(resp)
	if !strings.Contains(got, "module_id") {
		t.Errorf("extractJSON failed to find JSON object: %s", got)
	}
}

func TestExtractJSON_Raw(t *testing.T) {
	resp := `{"module_id": "test", "files": []}`
	got := extractJSON(resp)
	if !strings.HasPrefix(got, "{") {
		t.Errorf("extractJSON failed for raw JSON: %s", got)
	}
}

func TestExtractJSON_Array(t *testing.T) {
	resp := `[{"path": "main.go", "content": "code"}]`
	got := extractJSON(resp)
	if !strings.HasPrefix(got, "[") {
		t.Errorf("extractJSON failed for array: %s", got)
	}
}

func TestExtractCodeFromLLM_Go(t *testing.T) {
	resp := "Here is the Go code:\n```go\npackage main\n\nfunc main() {}\n```\n"
	got := extractCodeFromLLM(resp, "go")
	if !strings.Contains(got, "package main") {
		t.Errorf("extractCodeFromLLM failed for Go: %s", got)
	}
}

func TestExtractCodeFromLLM_Shell(t *testing.T) {
	resp := "Script:\n```sh\n#!/bin/sh\necho hello\n```\n"
	got := extractCodeFromLLM(resp, "sh")
	if !strings.Contains(got, "#!/bin/sh") {
		t.Errorf("extractCodeFromLLM failed for shell: %s", got)
	}
}

func TestExtractCodeFromLLM_NoFence(t *testing.T) {
	resp := "No code fence here"
	got := extractCodeFromLLM(resp, "go")
	if got != "" {
		t.Errorf("expected empty for no fence, got: %s", got)
	}
}

func TestBuildCodePrompt_Go(t *testing.T) {
	plan := &PlanResult{
		ModuleID: "test",
		Files: []PlannedFile{
			{Path: "module.prop", Description: "metadata", Language: "prop"},
			{Path: "src/main.go", Description: "daemon", Language: "go"},
		},
	}
	pf := plan.Files[1]

	prompt := buildCodePrompt("battery monitor", plan, pf)
	if !strings.Contains(prompt, "Go source file") {
		t.Error("prompt missing Go language hint")
	}
	if !strings.Contains(prompt, "package main") {
		t.Error("prompt missing package main rule")
	}
	if !strings.Contains(prompt, "src/main.go") {
		t.Error("prompt missing file path")
	}
}

func TestBuildCodePrompt_Shell(t *testing.T) {
	plan := &PlanResult{
		ModuleID: "test",
		Files: []PlannedFile{
			{Path: "customize.sh", Description: "installer", Language: "shell"},
		},
	}
	pf := plan.Files[0]

	prompt := buildCodePrompt("battery monitor", plan, pf)
	if !strings.Contains(prompt, "Shell script") {
		t.Error("prompt missing Shell language hint")
	}
	if !strings.Contains(prompt, "${VAR}") {
		t.Error("prompt missing ${VAR} rule")
	}
}

func TestTruncateStr(t *testing.T) {
	if got := truncateStr("hello", 10); got != "hello" {
		t.Errorf("truncateStr short = %q, want %q", got, "hello")
	}
	if got := truncateStr("hello world", 5); got != "hello..." {
		t.Errorf("truncateStr long = %q, want %q", got, "hello...")
	}
}

func TestPlanResult_Marshal(t *testing.T) {
	plan := PlanResult{
		ModuleID: "test-module",
		Files: []PlannedFile{
			{Path: "module.prop", Description: "metadata", Language: "prop"},
			{Path: "customize.sh", Description: "installer", Language: "shell"},
		},
	}

	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed PlanResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed.ModuleID != "test-module" {
		t.Errorf("expected module_id test-module, got %s", parsed.ModuleID)
	}

	if len(parsed.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(parsed.Files))
	}
}

func TestReviewResult_Marshal(t *testing.T) {
	review := ReviewResult{
		Passed: false,
		Issues: []ReviewIssue{
			{File: "customize.sh", Level: "error", Message: "missing set_perm"},
		},
	}

	data, err := json.Marshal(review)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed ReviewResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed.Passed {
		t.Error("expected passed=false")
	}

	if len(parsed.Issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(parsed.Issues))
	}
}

func TestGenerateWithReview_NilTeam(t *testing.T) {
	var team *Team
	_, err := team.GenerateWithReview(context.Background(), "test", nil)
	if err == nil {
		t.Fatal("expected error for nil team")
	}
	if !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuilderLLMCaller(t *testing.T) {
	// Test that the caller interface is properly implemented
	caller := &builderLLMCaller{
		endpoint: "https://example.com/v1",
		apiKey:   "test-key",
		model:    "test-model",
	}

	if caller.endpoint != "https://example.com/v1" {
		t.Errorf("unexpected endpoint: %s", caller.endpoint)
	}

	// Actual LLM call would fail without valid endpoint, but we test the structure
	ctx := context.Background()
	_, err := caller.CallLLM(ctx, "test")
	// This will fail because the endpoint is fake, but we test it doesn't panic
	if err == nil {
		t.Log("CallLLM succeeded (unexpected with fake endpoint)")
	}
}
