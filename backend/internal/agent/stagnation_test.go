package agent

import (
	"fmt"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════
// P0-1: StagnationDetector Tests
// ═══════════════════════════════════════════════════════════════════

func TestStagnationDetector_RecordToolCall(t *testing.T) {
	sd := newStagnationDetector()
	sd.maxIdenticalRepeats = 3
	sd.maxStagnationRounds = 5

	// Test: No stagnation with different calls
	args1 := map[string]interface{}{"path": "file1.go"}
	args2 := map[string]interface{}{"path": "file2.go"}

	stagnant, _ := sd.RecordToolCall("read_file", args1, "content1")
	if stagnant {
		t.Error("Expected no stagnation on first call")
	}

	stagnant, _ = sd.RecordToolCall("read_file", args2, "content2")
	if stagnant {
		t.Error("Expected no stagnation with different args")
	}
}

func TestStagnationDetector_IdenticalRepeats(t *testing.T) {
	sd := newStagnationDetector()
	sd.maxIdenticalRepeats = 3
	sd.maxStagnationRounds = 5

	args := map[string]interface{}{"path": "file1.go"}

	// First call - no stagnation
	stagnant, _ := sd.RecordToolCall("read_file", args, "content")
	if stagnant {
		t.Error("Expected no stagnation on first call")
	}

	// Second call - no stagnation yet
	stagnant, _ = sd.RecordToolCall("read_file", args, "content")
	if stagnant {
		t.Error("Expected no stagnation on second call")
	}

	// Third call - stagnation detected
	stagnant, reason := sd.RecordToolCall("read_file", args, "content")
	if !stagnant {
		t.Error("Expected stagnation after 3 identical calls")
	}
	if reason == "" {
		t.Error("Expected non-empty reason")
	}
}

func TestStagnationDetector_ConsecutiveIdenticalResults(t *testing.T) {
	sd := newStagnationDetector()
	sd.maxIdenticalRepeats = 10
	sd.maxStagnationRounds = 5

	// 5 identical results should trigger stagnation
	for i := 0; i < 4; i++ {
		args := map[string]interface{}{"path": fmt.Sprintf("file%d.go", i)}
		sd.RecordToolCall("read_file", args, "same result")
	}

	// 5th call should trigger stagnation
	args := map[string]interface{}{"path": "file5.go"}
	stagnant, _ := sd.RecordToolCall("read_file", args, "same result")
	if !stagnant {
		t.Error("Expected stagnation after 5 consecutive identical results")
	}
}

func TestStagnationDetector_ResetNoWrite(t *testing.T) {
	sd := newStagnationDetector()
	sd.maxConsecutiveNoWrite = 15

	// Simulate no writes
	for i := 0; i < 14; i++ {
		if sd.RecordNoWrite() {
			t.Error("Expected no stagnation before limit")
		}
	}

	// Reset counter
	sd.ResetNoWrite()

	// Should not trigger stagnation after reset
	if sd.RecordNoWrite() {
		t.Error("Expected no stagnation after reset")
	}
}

// ═══════════════════════════════════════════════════════════════════
// P0-2: ToolRetryFallback Tests
// ═══════════════════════════════════════════════════════════════════

func TestToolRetryFallback_GetFallback(t *testing.T) {
	trf := &ToolRetryFallback{
		currentModel: "deepseek-v3",
	}

	tests := []struct {
		name                string
		err                 error
		consecutiveFailures int
		expectedFallback    FallbackStrategy
	}{
		{
			name:                "tool not found",
			err:                 fmt.Errorf("skill not found"),
			consecutiveFailures: 0,
			expectedFallback:    FallbackSimplifyTask,
		},
		{
			name:                "timeout first attempt",
			err:                 fmt.Errorf("timeout exceeded"),
			consecutiveFailures: 0,
			expectedFallback:    FallbackRetrySame,
		},
		{
			name:                "timeout after failures",
			err:                 fmt.Errorf("timeout exceeded"),
			consecutiveFailures: 3,
			expectedFallback:    FallbackSimplifyTask,
		},
		{
			name:                "rate limited",
			err:                 fmt.Errorf("rate limit 429"),
			consecutiveFailures: 0,
			expectedFallback:    FallbackSwitchModel,
		},
		{
			name:                "context too long",
			err:                 fmt.Errorf("context_length exceeded"),
			consecutiveFailures: 0,
			expectedFallback:    FallbackForceAnswer,
		},
		{
			name:                "general error after 3 failures",
			err:                 fmt.Errorf("some error"),
			consecutiveFailures: 3,
			expectedFallback:    FallbackSimplifyTask,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fallback := trf.GetFallback("write_file", tt.err, tt.consecutiveFailures)
			if fallback != tt.expectedFallback {
				t.Errorf("Expected %v, got %v", tt.expectedFallback, fallback)
			}
		})
	}
}

func TestToolRetryFallback_SimplifyTaskInput(t *testing.T) {
	trf := &ToolRetryFallback{}

	// Test: Simplify long content
	input := map[string]interface{}{
		"path":    "test.go",
		"content": string(make([]byte, 10000)),
	}
	simplified := trf.SimplifyTaskInput("write_file", input)
	if content, ok := simplified["content"].(string); ok {
		if len(content) > 5100 {
			t.Error("Expected content to be truncated")
		}
	}

	// Test: Simplify complex bash command
	input = map[string]interface{}{
		"command": "cd /tmp && rm -rf * && mkdir test",
	}
	simplified = trf.SimplifyTaskInput("bash", input)
	if cmd, ok := simplified["command"].(string); ok {
		if cmd != "cd /tmp" {
			t.Errorf("Expected simplified command, got: %s", cmd)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════
// P2-1: TaskDecomposer Tests
// ═══════════════════════════════════════════════════════════════════

func TestTaskDecomposer_DecomposeTask(t *testing.T) {
	td := &TaskDecomposer{}

	tests := []struct {
		name            string
		task            string
		expectedCount   int
		expectedFirstID string
	}{
		{
			name:            "create task",
			task:            "Create a new login module",
			expectedCount:   3,
			expectedFirstID: "analyze",
		},
		{
			name:            "fix task",
			task:            "Fix the crash in main.go",
			expectedCount:   3,
			expectedFirstID: "diagnose",
		},
		{
			name:            "refactor task",
			task:            "Refactor the authentication code",
			expectedCount:   4,
			expectedFirstID: "analyze",
		},
		{
			name:            "unknown task",
			task:            "Do something random",
			expectedCount:   1,
			expectedFirstID: "complete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subtasks := td.DecomposeTask(tt.task, "")
			if len(subtasks) != tt.expectedCount {
				t.Errorf("Expected %d subtasks, got %d", tt.expectedCount, len(subtasks))
			}
			if subtasks[0].ID != tt.expectedFirstID {
				t.Errorf("Expected first subtask ID %s, got %s", tt.expectedFirstID, subtasks[0].ID)
			}
		})
	}
}

func TestTaskDecomposer_GetNextSubtask(t *testing.T) {
	td := &TaskDecomposer{}

	subtasks := []Subtask{
		{ID: "analyze", Description: "Analyze", Status: "completed", Dependencies: []string{}},
		{ID: "implement", Description: "Implement", Status: "pending", Dependencies: []string{"analyze"}},
		{ID: "verify", Description: "Verify", Status: "pending", Dependencies: []string{"implement"}},
	}

	// Next should be "implement" since "analyze" is completed
	next := td.GetNextSubtask(subtasks)
	if next == nil {
		t.Fatal("Expected next subtask")
	}
	if next.ID != "implement" {
		t.Errorf("Expected implement, got %s", next.ID)
	}

	// Complete "implement"
	subtasks[1].Status = "completed"
	next = td.GetNextSubtask(subtasks)
	if next == nil {
		t.Fatal("Expected next subtask")
	}
	if next.ID != "verify" {
		t.Errorf("Expected verify, got %s", next.ID)
	}

	// Complete "verify"
	subtasks[2].Status = "completed"
	next = td.GetNextSubtask(subtasks)
	if next != nil {
		t.Error("Expected no next subtask")
	}
}

// ═══════════════════════════════════════════════════════════════════
// P2-2: QualityVerifier Tests
// ═══════════════════════════════════════════════════════════════════

func TestQualityVerifier_VerifyFile(t *testing.T) {
	qv := &QualityVerifier{}

	// Test: Good quality file
	goodCode := `package main

import "fmt"

// Helper function
func helper() {
	fmt.Println("hello")
}
`
	report := qv.VerifyFile("main.go", goodCode)
	if report.Score < 80 {
		t.Errorf("Expected high score for good code, got %d", report.Score)
	}

	// Test: Bad quality file with issues
	badCode := `package main

func main() {
	if true {
		if true {
			if true {
				if true {
					if true {
						if true {
							fmt.Println("deeply nested")
						}
					}
				}
			}
		}
	}
}

// FIXME: needs cleanup
// FIXME: temporary workaround
// FIXME: remove before release
// FIXME: refactor later
// FIXME: technical debt
`
	report = qv.VerifyFile("bad.go", badCode)
	if report.Score >= 80 {
		t.Errorf("Expected low score for bad code, got %d", report.Score)
	}
	if len(report.Issues) == 0 {
		t.Error("Expected issues to be reported")
	}
}

func TestQualityVerifier_GetQualitySummary(t *testing.T) {
	qv := &QualityVerifier{}

	reports := []QualityReport{
		{FilePath: "a.go", Score: 90, Issues: []string{}},
		{FilePath: "b.go", Score: 80, Issues: []string{"line too long"}},
		{FilePath: "c.go", Score: 70, Issues: []string{"FIXME found", "deep nesting"}},
	}

	summary := qv.GetQualitySummary(reports)
	if summary == "" {
		t.Error("Expected non-empty summary")
	}
}

func TestToolCallSignature(t *testing.T) {
	args1 := map[string]interface{}{"path": "file.go", "content": "test"}
	args2 := map[string]interface{}{"path": "file.go", "content": "test"}
	args3 := map[string]interface{}{"path": "other.go", "content": "test"}

	sig1 := toolCallSignature("write_file", args1)
	sig2 := toolCallSignature("write_file", args2)
	sig3 := toolCallSignature("write_file", args3)

	if sig1 != sig2 {
		t.Error("Expected same signature for same args")
	}
	if sig1 == sig3 {
		t.Error("Expected different signature for different args")
	}
}
