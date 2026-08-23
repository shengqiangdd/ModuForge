package evolution

import (
	"testing"
	"time"
)

func TestNewReviewStore(t *testing.T) {
	s := NewReviewStore(t.TempDir())
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestGenerateReview(t *testing.T) {
	s := NewReviewStore(t.TempDir())

	report := s.GenerateReview(
		"task_1",
		"Build battery monitor",
		true,
		30*time.Second,
		1500,
		[]string{"minor syntax issue"},
	)

	if report.TaskID != "task_1" {
		t.Errorf("expected task_1, got %s", report.TaskID)
	}

	if !report.Success {
		t.Error("expected success=true")
	}

	if report.Duration != 30*time.Second {
		t.Errorf("expected 30s, got %v", report.Duration)
	}

	if report.TokensUsed != 1500 {
		t.Errorf("expected 1500 tokens, got %d", report.TokensUsed)
	}

	if len(report.Issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(report.Issues))
	}

	if len(report.ImprovementSuggestions) == 0 {
		t.Error("expected improvement suggestions")
	}
}

func TestSaveAndLoadReview(t *testing.T) {
	s := NewReviewStore(t.TempDir())

	report := s.GenerateReview("task_1", "test", true, 10*time.Second, 500, nil)

	if err := s.SaveReview(report); err != nil {
		t.Fatalf("SaveReview failed: %v", err)
	}

	all := s.GetAll()
	if len(all) != 1 {
		t.Fatalf("expected 1 review, got %d", len(all))
	}

	if all[0].TaskID != "task_1" {
		t.Errorf("expected task_1, got %s", all[0].TaskID)
	}
}

func TestGetWeeklySummary_Empty(t *testing.T) {
	s := NewReviewStore(t.TempDir())

	summary := s.GetWeeklySummary()
	if summary.TotalTasks != 0 {
		t.Errorf("expected 0 tasks, got %d", summary.TotalTasks)
	}
}

func TestGetWeeklySummary(t *testing.T) {
	s := NewReviewStore(t.TempDir())

	// Save some reviews
	for i := 0; i < 5; i++ {
		report := s.GenerateReview(
			"task_"+string(rune('0'+i)),
			"requirement",
			i%2 == 0, // alternating success
			10*time.Second,
			500,
			[]string{"syntax error"},
		)
		s.SaveReview(report)
	}

	summary := s.GetWeeklySummary()

	if summary.TotalTasks != 5 {
		t.Errorf("expected 5 tasks, got %d", summary.TotalTasks)
	}

	if summary.SuccessRate < 40 || summary.SuccessRate > 60 {
		t.Errorf("expected ~50%% success rate, got %.1f%%", summary.SuccessRate)
	}

	if summary.AvgDuration <= 0 {
		t.Error("expected positive avg duration")
	}

	if len(summary.TopIssues) == 0 {
		t.Error("expected top issues")
	}
}

func TestGetAllReviews(t *testing.T) {
	s := NewReviewStore(t.TempDir())

	s.SaveReview(s.GenerateReview("t1", "r1", true, 5*time.Second, 100, nil))
	s.SaveReview(s.GenerateReview("t2", "r2", false, 10*time.Second, 200, []string{"error"}))

	all := s.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
}

func TestGenerateSuggestions(t *testing.T) {
	suggestions := generateSuggestions([]string{"syntax error", "timeout occurred"}, false)

	if len(suggestions) < 3 {
		t.Errorf("expected at least 3 suggestions, got %d", len(suggestions))
	}

	// Should have the failure suggestion
	hasFailure := false
	for _, s := range suggestions {
		if containsSubstring(s, "failed task") {
			hasFailure = true
			break
		}
	}
	if !hasFailure {
		t.Error("expected failure suggestion")
	}
}

func TestCapabilityScores(t *testing.T) {
	s := NewReviewStore(t.TempDir())

	// Save successful reviews
	for i := 0; i < 10; i++ {
		s.SaveReview(s.GenerateReview(
			"task_"+string(rune('0'+i)),
			"requirement",
			true,
			5*time.Second,
			100,
			nil,
		))
	}

	summary := s.GetWeeklySummary()

	if summary.CapabilityRadar.CodeGeneration < 80 {
		t.Errorf("expected code generation > 80, got %.1f", summary.CapabilityRadar.CodeGeneration)
	}
}

func TestReviewReport_Fields(t *testing.T) {
	report := ReviewReport{
		TaskID:                 "t1",
		Requirement:            "build module",
		Success:                true,
		Duration:               30 * time.Second,
		TokensUsed:             1000,
		Issues:                 []string{"issue1"},
		ImprovementSuggestions: []string{"suggestion1"},
		Timestamp:              time.Now(),
	}

	if report.TaskID != "t1" {
		t.Errorf("expected t1, got %s", report.TaskID)
	}

	if report.TokensUsed != 1000 {
		t.Errorf("expected 1000, got %d", report.TokensUsed)
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
