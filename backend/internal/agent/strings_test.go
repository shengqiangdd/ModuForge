package agent

import (
	"strings"
	"testing"
)

func TestCleanAnswer_RemovesToolSyntax(t *testing.T) {
	input := `function call: read_file({"path": "src/main.rs"})
read_file
Tool 'validate' not found
Executing tool 'write_file'...
Successfully wrote to 'test.rs'
{"type":"function","function":{"name":"think","arguments":"{}"}}
Using tool think...
Call tool validate...
I'll read the file
Let me write the file
This is the actual answer that should remain.`

	result := cleanAnswer(input)

	// Should NOT contain any tool syntax
	for _, bad := range []string{
		"function call:", "Tool '", "Executing tool '",
		"Successfully wrote to", `{"type":"function"`,
		"Using tool", "Call tool",
	} {
		if contains(result, bad) {
			t.Errorf("cleanAnswer should remove %q, got:\n%s", bad, result)
		}
	}

	// Should contain the actual answer
	if !contains(result, "This is the actual answer") {
		t.Errorf("cleanAnswer should preserve actual content, got:\n%s", result)
	}
}

func TestCleanAnswer_CollapsesBlankLines(t *testing.T) {
	input := "line1\n\n\n\n\nline2"
	result := cleanAnswer(input)
	if contains(result, "\n\n\n\n") {
		t.Errorf("cleanAnswer should collapse 3+ blank lines, got:\n%s", result)
	}
}

func TestCleanAnswer_ToolNameOnly(t *testing.T) {
	// Standalone tool name lines should be removed
	input := "bash\nread_file\nHere is my answer."
	result := cleanAnswer(input)
	if contains(result, "bash") || contains(result, "read_file") {
		t.Errorf("cleanAnswer should remove standalone tool names, got:\n%s", result)
	}
	if !contains(result, "Here is my answer") {
		t.Errorf("cleanAnswer should preserve actual content, got:\n%s", result)
	}
}

func TestToolNameSet_Completeness(t *testing.T) {
	expectedTools := []string{
		"read_file", "write_file", "write_file_batch",
		"edit_file", "grep_search", "glob_search",
		"list_dir", "delete_file", "delete_dir",
		"move_file", "bash", "build_module",
		"test_module", "agent_preset", "self_evolve",
		"pattern_learn", "memory_v2", "skill_manager",
		"self_reflection", "session_summary", "skill_registry",
		"context_manager", "task_delegator", "todo_manager",
	}
	for _, tool := range expectedTools {
		if !toolNameSet[tool] {
			t.Errorf("toolNameSet missing tool: %s", tool)
		}
	}
	if len(toolNameSet) != len(expectedTools) {
		t.Errorf("toolNameSet has %d entries, expected %d", len(toolNameSet), len(expectedTools))
	}
}

func TestSanitizeReasoning(t *testing.T) {
	// Test control character removal
	input := "Hello\x00World\x01Test"
	result := sanitizeReasoning(input)
	if contains(result, "\x00") || contains(result, "\x01") {
		t.Errorf("sanitizeReasoning should remove control chars, got: %q", result)
	}

	// Test space collapse
	input = "Hello   World"
	result = sanitizeReasoning(input)
	if contains(result, "   ") {
		t.Errorf("sanitizeReasoning should collapse spaces, got: %q", result)
	}
}

func TestIsGarbageOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty", "", true},
		{"too_short", "ok", true},
		{"valid_text", "This is a valid response with enough content.", false},
		{"repeated_pattern", "｜｜｜｜｜｜｜｜｜｜｜｜", true},
		{"too_many_tags", "<<<<<<<>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isGarbageOutput(tt.input)
			if result != tt.expected {
				t.Errorf("isGarbageOutput(%q) = %v, want %v", tt.input[:min(len(tt.input), 50)], result, tt.expected)
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"bad_gateway", "Upstream request failed: Bad Gateway", true},
		{"rate_limit", "HTTP 429 rate limited", true},
		{"context_too_long", "context_length_exceeded", true},
		{"auth_error", "invalid_api_key", false},
		{"permission", "permission_denied", false},
		{"server_error", "HTTP 500 Internal Server Error", true},
		{"not_found", "HTTP 404 Not Found", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &stringError{msg: tt.input}
			result := isRetryableError(err)
			if result != tt.expected {
				t.Errorf("isRetryableError(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSplitCamelCase(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"theUserAsked", []string{"the", "User", "Asked"}},
		{"HTTP", []string{"H", "T", "T", "P"}}, // all-uppercase splits char by char
		{"get", []string{"get"}},
		{"XMLParser", []string{"X", "M", "L", "Parser"}}, // all-uppercase prefix splits
		{"getElementById", []string{"get", "Element", "By", "Id"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitCamelCase(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("splitCamelCase(%q) = %v (len=%d), want %v (len=%d)", tt.input, result, len(result), tt.expected, len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("splitCamelCase(%q)[%d] = %q, want %q", tt.input, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestSmartSummarizeResult(t *testing.T) {
	// Test that small results pass through unchanged
	small := "short result"
	result := smartSummarizeResult(small, "read_file", 1000)
	if result != small {
		t.Errorf("smartSummarizeResult should pass through small results, got: %s", result)
	}

	// Test build output summarization — needs to be > maxLen to trigger
	buildOutput := "line1\nERROR: something failed\nline3\nWARNING: deprecated\nfinished successfully\n" + strings.Repeat("x\n", 200)
	result = smartSummarizeResult(buildOutput, "build_module", 200)
	if !contains(result, "Build Output Summary") {
		t.Errorf("smartSummarizeResult should summarize build output, got: %s", result[:100])
	}

	// Test read_file summarization
	readOutput := strings.Repeat("func test() {}\n", 100)
	result = smartSummarizeResult(readOutput, "read_file", 500)
	if !contains(result, "File Summary") {
		t.Errorf("smartSummarizeResult should summarize read_file output, got: %s", result[:100])
	}
}

// stringError is a simple error type for testing
type stringError struct {
	msg string
}

func (e *stringError) Error() string { return e.msg }

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
