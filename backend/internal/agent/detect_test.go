package agent

import (
	"testing"
)

func TestDetectLoop_NoLoop(t *testing.T) {
	history := map[string]int{"read_file": 2, "write_file": 1}
	unique := map[string]bool{"read_file:f1": true, "read_file:f2": true, "write_file:f1": true}
	result := detectLoop(history, unique, 3)
	if result != "" {
		t.Errorf("detectLoop should return empty for normal usage, got: %s", result)
	}
}

func TestDetectLoop_ReadFileLoop(t *testing.T) {
	// Same file read 8 times
	history := map[string]int{"read_file": 8}
	unique := map[string]bool{"read_file:same_file.rs": true}
	result := detectLoop(history, unique, 8)
	if result == "" {
		t.Error("detectLoop should detect read_file loop on same target")
	}
}

func TestDetectLoop_TotalBudget(t *testing.T) {
	history := map[string]int{"read_file": 5, "write_file": 5, "bash": 5}
	unique := map[string]bool{
		"read_file:f1": true, "read_file:f2": true, "read_file:f3": true,
		"write_file:f1": true, "write_file:f2": true, "write_file:f3": true,
		"bash:t1": true, "bash:t2": true, "bash:t3": true,
	}
	result := detectLoop(history, unique, 15)
	if result == "" {
		t.Error("detectLoop should trigger on total call budget")
	}
}

func TestDetectLoop_DifferentTargets(t *testing.T) {
	// 8 reads on different files — should NOT trigger
	history := map[string]int{"read_file": 8}
	unique := map[string]bool{
		"read_file:f1": true, "read_file:f2": true, "read_file:f3": true,
		"read_file:f4": true, "read_file:f5": true, "read_file:f6": true,
		"read_file:f7": true, "read_file:f8": true,
	}
	result := detectLoop(history, unique, 8)
	if result != "" {
		t.Errorf("detectLoop should NOT trigger for reads on different targets, got: %s", result)
	}
}

func TestClaimsFileModification(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"claims_write", "I have modified the file src/main.rs to fix the bug.", true},
		{"claims_created", "I created a new file called config.json.", true},
		{"no_claim", "The file references a configuration that needs updating.", false},
		{"chinese_claim", "已修改 ipc.rs 文件中的类型定义。", true},
		{"chinese_no_claim", "这个文件需要创建一个新目录。", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := claimsFileModification(tt.input)
			if result != tt.expected {
				t.Errorf("claimsFileModification(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildDiagnosticSummary(t *testing.T) {
	conversation := []map[string]interface{}{
		{"role": "user", "content": "Fix the bug"},
		{
			"role":    "assistant",
			"content": "",
			"tool_calls": []LLMToolCall{
				{ID: "1", Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: "read_file", Arguments: `{"path":"src/main.rs"} `}},
			},
		},
		{"role": "tool", "content": "File content here"},
		{
			"role":    "assistant",
			"content": "",
			"tool_calls": []LLMToolCall{
				{ID: "2", Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: "write_file", Arguments: `{"path":"src/main.rs","content":"fixed"}`}},
			},
		},
		{"role": "tool", "content": "File written successfully"},
	}

	result := buildDiagnosticSummary(conversation)
	if result == "" {
		t.Error("buildDiagnosticSummary should return non-empty for conversation with tool calls")
	}
	if !containsSubstr(result, "Total tool calls: 2") {
		t.Errorf("buildDiagnosticSummary should report tool call count, got: %s", result)
	}
	if !containsSubstr(result, "read: 1") {
		t.Errorf("buildDiagnosticSummary should report read count, got: %s", result)
	}
	if !containsSubstr(result, "write: 1") {
		t.Errorf("buildDiagnosticSummary should report write count, got: %s", result)
	}
}

func TestIsGarbageOutput_Unicode(t *testing.T) {
	// Valid Chinese text should not be garbage
	validText := "这是一个有效的中文回答，包含足够的内容来通过检测。"
	if isGarbageOutput(validText) {
		t.Error("isGarbageOutput should not flag valid Chinese text")
	}
}

func TestDetectLoop_GeneralSkillLoop(t *testing.T) {
	// build_module called 6 times on different files — should trigger
	history := map[string]int{"build_module": 6}
	unique := map[string]bool{
		"build_module:f1": true, "build_module:f2": true, "build_module:f3": true,
		"build_module:f4": true, "build_module:f5": true, "build_module:f6": true,
	}
	result := detectLoop(history, unique, 6)
	if result == "" {
		t.Error("detectLoop should trigger general skill loop at 6 calls")
	}
}
