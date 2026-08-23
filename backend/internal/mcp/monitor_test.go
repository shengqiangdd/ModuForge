package mcp

import (
	"testing"
	"time"
)

func TestNewToolMonitor(t *testing.T) {
	tm := NewToolMonitor(t.TempDir())
	if tm == nil {
		t.Fatal("expected non-nil monitor")
	}
}

func TestRecordAndGetStats(t *testing.T) {
	tm := NewToolMonitor(t.TempDir())

	tm.RecordCall(ToolCallLog{
		ToolName:  "test_tool",
		Duration:  100 * time.Millisecond,
		Success:   true,
		Timestamp: time.Now(),
	})

	tm.RecordCall(ToolCallLog{
		ToolName:  "test_tool",
		Duration:  200 * time.Millisecond,
		Success:   true,
		Timestamp: time.Now(),
	})

	stats := tm.GetStats("test_tool")

	if stats.TotalCalls != 2 {
		t.Errorf("expected 2 calls, got %d", stats.TotalCalls)
	}

	if stats.SuccessRate != 100 {
		t.Errorf("expected 100%% success rate, got %.1f%%", stats.SuccessRate)
	}

	if stats.AvgLatency <= 0 {
		t.Error("expected positive avg latency")
	}
}

func TestRecordAndGetStats_WithErrors(t *testing.T) {
	tm := NewToolMonitor(t.TempDir())

	tm.RecordCall(ToolCallLog{ToolName: "t", Success: true, Duration: 100 * time.Millisecond})
	tm.RecordCall(ToolCallLog{ToolName: "t", Success: false, ErrorCode: "E001", Duration: 50 * time.Millisecond})
	tm.RecordCall(ToolCallLog{ToolName: "t", Duration: 80 * time.Millisecond, Success: true})

	stats := tm.GetStats("t")

	if stats.TotalCalls != 3 {
		t.Errorf("expected 3, got %d", stats.TotalCalls)
	}

	if stats.ErrorRate < 30 || stats.ErrorRate > 34 {
		t.Errorf("expected ~33%% error rate, got %.1f%%", stats.ErrorRate)
	}

	if len(stats.LastErrors) != 1 {
		t.Errorf("expected 1 last error, got %d", len(stats.LastErrors))
	}
}

func TestCheckCircuitBreaker(t *testing.T) {
	tm := NewToolMonitor(t.TempDir())

	// Record 20 calls with 60% failure rate
	for i := 0; i < 20; i++ {
		tm.RecordCall(ToolCallLog{
			ToolName: "flaky",
			Success:  i < 8, // 8 success, 12 failures = 60% error rate
		})
	}

	// Circuit should trip at 50% threshold
	if !tm.CheckCircuitBreaker("flaky", 50) {
		t.Error("expected circuit breaker to trip at 50% threshold")
	}

	// Circuit should not trip at 70% threshold
	if tm.CheckCircuitBreaker("flaky", 70) {
		t.Error("expected circuit breaker NOT to trip at 70% threshold")
	}
}

func TestCheckCircuitBreaker_NotEnoughData(t *testing.T) {
	tm := NewToolMonitor(t.TempDir())

	// Only 5 calls (below minimum of 10)
	for i := 0; i < 5; i++ {
		tm.RecordCall(ToolCallLog{ToolName: "new_tool", Success: false})
	}

	// Should not trip even with 100% error rate
	if tm.CheckCircuitBreaker("new_tool", 50) {
		t.Error("expected circuit breaker NOT to trip with insufficient data")
	}
}

func TestGetDashboard(t *testing.T) {
	tm := NewToolMonitor(t.TempDir())

	tm.RecordCall(ToolCallLog{ToolName: "tool_a", Success: true})
	tm.RecordCall(ToolCallLog{ToolName: "tool_a", Success: true})
	tm.RecordCall(ToolCallLog{ToolName: "tool_b", Success: false})

	dash := tm.GetDashboard()

	if dash.TotalCalls != 3 {
		t.Errorf("expected 3 total calls, got %d", dash.TotalCalls)
	}

	if len(dash.ToolRankings) != 2 {
		t.Errorf("expected 2 tool rankings, got %d", len(dash.ToolRankings))
	}

	// tool_a should rank higher (more calls)
	if dash.ToolRankings[0].Name != "tool_a" {
		t.Errorf("expected tool_a first, got %s", dash.ToolRankings[0].Name)
	}

	if dash.OverallSuccessRate < 60 || dash.OverallSuccessRate > 70 {
		t.Errorf("expected ~66%% overall success, got %.1f%%", dash.OverallSuccessRate)
	}
}

func TestGetDashboard_Empty(t *testing.T) {
	tm := NewToolMonitor(t.TempDir())

	dash := tm.GetDashboard()
	if dash.TotalCalls != 0 {
		t.Errorf("expected 0 calls, got %d", dash.TotalCalls)
	}
}

func TestGetRecentErrors(t *testing.T) {
	tm := NewToolMonitor(t.TempDir())

	for i := 0; i < 5; i++ {
		tm.RecordCall(ToolCallLog{
			ToolName: "error_tool",
			Success:  i%2 == 0,
		})
	}

	errors := tm.GetRecentErrors("error_tool", 3)
	if len(errors) > 3 {
		t.Errorf("expected at most 3 errors, got %d", len(errors))
	}

	for _, e := range errors {
		if e.Success {
			t.Error("expected only error logs")
		}
	}
}

func TestGetRecentErrors_Empty(t *testing.T) {
	tm := NewToolMonitor(t.TempDir())

	errors := tm.GetRecentErrors("nonexistent", 5)
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errors))
	}
}

func TestRecordCall_Timestamp(t *testing.T) {
	tm := NewToolMonitor(t.TempDir())

	tm.RecordCall(ToolCallLog{ToolName: "t", Success: true})

	logs := tm.DashBoardRecentCalls("t", 1)
	// Just verify it doesn't panic
	_ = logs
}

func TestToolCallLog_Fields(t *testing.T) {
	log := ToolCallLog{
		ToolName:  "deploy",
		Input:     "module.zip",
		Output:    "deployed",
		Duration:  500 * time.Millisecond,
		Success:   true,
		ErrorCode: "",
		Timestamp: time.Now(),
	}

	if log.ToolName != "deploy" {
		t.Errorf("expected deploy, got %s", log.ToolName)
	}

	if log.Duration != 500*time.Millisecond {
		t.Errorf("expected 500ms, got %v", log.Duration)
	}
}

func TestToolStats_Fields(t *testing.T) {
	stats := ToolStats{
		TotalCalls:  100,
		SuccessRate: 95.5,
		AvgLatency:  123.4,
		ErrorRate:   4.5,
		LastErrors:  []string{"E001", "E002"},
	}

	if stats.TotalCalls != 100 {
		t.Errorf("expected 100, got %d", stats.TotalCalls)
	}

	if len(stats.LastErrors) != 2 {
		t.Errorf("expected 2, got %d", len(stats.LastErrors))
	}
}

// DashBoardRecentCalls is a helper for tests.
func (tm *ToolMonitor) DashBoardRecentCalls(toolName string, limit int) []ToolCallLog {
	return tm.GetRecentErrors(toolName, limit)
}
