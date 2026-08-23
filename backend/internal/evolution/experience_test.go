package evolution

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewExperienceStore(t *testing.T) {
	s := NewExperienceStore(t.TempDir())
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestSaveAndSearchExperience(t *testing.T) {
	s := NewExperienceStore(t.TempDir())

	exp := Experience{
		ErrorPattern: "undefined reference to fmt.Println",
		FixSolution:  "Add import \"fmt\" to the file",
		SuccessRate:  0.95,
		Timestamp:    time.Now(),
	}

	if err := s.SaveExperience(exp); err != nil {
		t.Fatalf("SaveExperience failed: %v", err)
	}

	// Search by pattern
	results := s.SearchByPattern("undefined reference fmt", 5)
	if len(results) == 0 {
		t.Fatal("expected at least 1 search result")
	}

	if results[0].ErrorPattern != exp.ErrorPattern {
		t.Errorf("expected same error pattern, got %s", results[0].ErrorPattern)
	}
}

func TestMarkVerified(t *testing.T) {
	s := NewExperienceStore(t.TempDir())

	exp := Experience{
		ErrorPattern: "test pattern",
		FixSolution:  "test fix",
		SuccessRate:  0.8,
		Timestamp:    time.Now(),
	}

	s.SaveExperience(exp)
	exps := s.GetAll()
	if len(exps) == 0 {
		t.Fatal("expected at least 1 experience")
	}

	if err := s.MarkVerified(exps[0].ID, true); err != nil {
		t.Fatalf("MarkVerified failed: %v", err)
	}

	// Verify
	exps = s.GetAll()
	if len(exps) > 0 && !exps[0].HumanVerified {
		t.Error("expected HumanVerified to be true")
	}
}

func TestMarkVerified_NotFound(t *testing.T) {
	s := NewExperienceStore(t.TempDir())

	err := s.MarkVerified("nonexistent", true)
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestLoadFromReport(t *testing.T) {
	s := NewExperienceStore(t.TempDir())

	// Create a report file
	report := `{
		"errors": [
			{"pattern": "syntax error", "fix": "fix syntax", "success": true},
			{"pattern": "undefined var", "fix": "declare var", "success": false}
		]
	}`
	reportPath := filepath.Join(t.TempDir(), "report.json")
	os.WriteFile(reportPath, []byte(report), 0644)

	exps, err := s.LoadFromReport(reportPath)
	if err != nil {
		t.Fatalf("LoadFromReport failed: %v", err)
	}

	if len(exps) != 2 {
		t.Fatalf("expected 2 experiences, got %d", len(exps))
	}

	if exps[0].ErrorPattern != "syntax error" {
		t.Errorf("expected 'syntax error', got %s", exps[0].ErrorPattern)
	}

	if exps[1].SuccessRate != 0 {
		t.Errorf("expected 0 success rate, got %f", exps[1].SuccessRate)
	}
}

func TestLoadFromReport_NotFound(t *testing.T) {
	s := NewExperienceStore(t.TempDir())
	_, err := s.LoadFromReport("/nonexistent/report.json")
	if err == nil {
		t.Error("expected error for nonexistent report")
	}
}

func TestSearchByPattern_NoMatch(t *testing.T) {
	s := NewExperienceStore(t.TempDir())
	s.SaveExperience(Experience{
		ErrorPattern: "test pattern",
		FixSolution:  "test fix",
	})

	results := s.SearchByPattern("zzz nonexistent zzz", 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchByPattern_VerifiedBoost(t *testing.T) {
	s := NewExperienceStore(t.TempDir())

	s.SaveExperience(Experience{
		ErrorPattern: "syntax error in go code",
		FixSolution:  "fix syntax",
		SuccessRate:  0.5,
	})
	s.SaveExperience(Experience{
		ErrorPattern:  "syntax error in go code",
		FixSolution:   "add missing brace",
		SuccessRate:   0.9,
		HumanVerified: true,
	})

	results := s.SearchByPattern("syntax error go", 5)
	if len(results) < 2 {
		t.Fatal("expected at least 2 results")
	}

	// Verified entry should rank higher
	if !results[0].HumanVerified {
		t.Error("expected verified entry to rank first")
	}
}

func TestGetAll(t *testing.T) {
	s := NewExperienceStore(t.TempDir())

	s.SaveExperience(Experience{ErrorPattern: "a", FixSolution: "fix a"})
	s.SaveExperience(Experience{ErrorPattern: "b", FixSolution: "fix b"})

	all := s.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
}

func TestCount(t *testing.T) {
	s := NewExperienceStore(t.TempDir())

	s.SaveExperience(Experience{ErrorPattern: "a", FixSolution: "fix a"})

	if s.Count() != 1 {
		t.Errorf("expected 1, got %d", s.Count())
	}
}

func TestExperience_Fields(t *testing.T) {
	exp := Experience{
		ID:            "exp_1",
		ErrorPattern:  "test",
		FixSolution:   "fix",
		SuccessRate:   0.9,
		Timestamp:     time.Now(),
		Source:        "report.json",
		HumanVerified: true,
		ApplyCount:    5,
		SuccessCount:  4,
	}

	if exp.ID != "exp_1" {
		t.Errorf("expected exp_1, got %s", exp.ID)
	}

	if exp.ApplyCount != 5 {
		t.Errorf("expected 5, got %d", exp.ApplyCount)
	}
}
