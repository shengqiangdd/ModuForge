package evolution

import (
	"testing"
	"time"
)

func TestNewPromptOptimizer(t *testing.T) {
	po := NewPromptOptimizer(t.TempDir())
	if po == nil {
		t.Fatal("expected non-nil optimizer")
	}
}

func TestAnalyzeAndSuggest_Empty(t *testing.T) {
	po := NewPromptOptimizer(t.TempDir())

	suggestions := po.AnalyzeAndSuggest(nil)
	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions for empty input, got %d", len(suggestions))
	}
}

func TestAnalyzeAndSuggest(t *testing.T) {
	po := NewPromptOptimizer(t.TempDir())

	experiences := []Experience{
		{ID: "e1", ErrorPattern: "undefined variable x", FixSolution: "declare x before use", SuccessRate: 0.9, Timestamp: time.Now()},
		{ID: "e2", ErrorPattern: "undefined variable x", FixSolution: "add var x int", SuccessRate: 0.85, Timestamp: time.Now()},
		{ID: "e3", ErrorPattern: "undefined variable x", FixSolution: "check scope", SuccessRate: 0.6, Timestamp: time.Now()},
	}

	suggestions := po.AnalyzeAndSuggest(experiences)

	if len(suggestions) == 0 {
		t.Fatal("expected at least 1 suggestion")
	}

	sug := suggestions[0]
	if sug.OriginalPrompt != "undefined variable x" {
		t.Errorf("expected original prompt 'undefined variable x', got %s", sug.OriginalPrompt)
	}

	if sug.SuggestedChange != "declare x before use" {
		t.Errorf("expected best fix 'declare x before use', got %s", sug.SuggestedChange)
	}

	if sug.Confidence <= 0 || sug.Confidence > 100 {
		t.Errorf("expected confidence 0-100, got %.1f", sug.Confidence)
	}
}

func TestAnalyzeAndSuggest_SingleExperience(t *testing.T) {
	po := NewPromptOptimizer(t.TempDir())

	experiences := []Experience{
		{ID: "e1", ErrorPattern: "syntax error", FixSolution: "fix syntax", SuccessRate: 0.9},
	}

	suggestions := po.AnalyzeAndSuggest(experiences)

	// Single experience shouldn't generate suggestion (need >= 2)
	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions for single experience, got %d", len(suggestions))
	}
}

func TestSaveAndApplySuggestion(t *testing.T) {
	po := NewPromptOptimizer(t.TempDir())

	sug := PromptSuggestion{
		OriginalPrompt:  "test prompt",
		SuggestedChange: "improved prompt",
		Reason:          "better success rate",
		Confidence:      85.0,
	}

	if err := po.SaveSuggestion(sug); err != nil {
		t.Fatalf("SaveSuggestion failed: %v", err)
	}

	pending := po.GetPendingSuggestions()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}

	if err := po.ApplySuggestion(pending[0].ID); err != nil {
		t.Fatalf("ApplySuggestion failed: %v", err)
	}

	pending = po.GetPendingSuggestions()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after apply, got %d", len(pending))
	}
}

func TestApplySuggestion_NotFound(t *testing.T) {
	po := NewPromptOptimizer(t.TempDir())

	err := po.ApplySuggestion("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent suggestion")
	}
}

func TestGetPendingSuggestions(t *testing.T) {
	po := NewPromptOptimizer(t.TempDir())

	po.SaveSuggestion(PromptSuggestion{OriginalPrompt: "p1", SuggestedChange: "s1"})
	po.SaveSuggestion(PromptSuggestion{OriginalPrompt: "p2", SuggestedChange: "s2"})

	pending := po.GetPendingSuggestions()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}
}

func TestGetPendingSuggestions_WithApplied(t *testing.T) {
	po := NewPromptOptimizer(t.TempDir())

	po.SaveSuggestion(PromptSuggestion{OriginalPrompt: "p1", SuggestedChange: "s1"})
	sug := PromptSuggestion{OriginalPrompt: "p2", SuggestedChange: "s2"}
	po.SaveSuggestion(sug)

	all := po.GetAll()
	if len(all) >= 2 {
		po.ApplySuggestion(all[1].ID)
	}

	pending := po.GetPendingSuggestions()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending, got %d", len(pending))
	}
}

func TestGetAllPrompts(t *testing.T) {
	po := NewPromptOptimizer(t.TempDir())

	po.SaveSuggestion(PromptSuggestion{OriginalPrompt: "p1", SuggestedChange: "s1"})
	po.SaveSuggestion(PromptSuggestion{OriginalPrompt: "p2", SuggestedChange: "s2"})

	all := po.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
}

func TestCalculateConfidence(t *testing.T) {
	// Low count, low success
	c1 := calculateConfidence(1, 0.3)
	if c1 > 50 {
		t.Errorf("expected low confidence, got %.1f", c1)
	}

	// High count, high success
	c2 := calculateConfidence(10, 0.95)
	if c2 < 70 {
		t.Errorf("expected high confidence, got %.1f", c2)
	}
}

func TestPromptSuggestion_Fields(t *testing.T) {
	sug := PromptSuggestion{
		ID:              "sug_1",
		OriginalPrompt:  "original",
		SuggestedChange: "changed",
		Reason:          "reason",
		Confidence:      85.5,
		CreatedAt:       time.Now(),
		Applied:         false,
		ExperienceIDs:   []string{"e1", "e2"},
	}

	if sug.ID != "sug_1" {
		t.Errorf("expected sug_1, got %s", sug.ID)
	}

	if len(sug.ExperienceIDs) != 2 {
		t.Errorf("expected 2 experience IDs, got %d", len(sug.ExperienceIDs))
	}
}
