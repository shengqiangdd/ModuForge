package feedback

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveAndGetFeedback(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	fb := Feedback{
		Prompt:        "Create a battery monitor",
		GeneratedCode: "package main\n// battery monitor",
		Action:        ActionAccept,
		Timestamp:     time.Now(),
	}

	if err := store.SaveFeedback(fb); err != nil {
		t.Fatalf("SaveFeedback failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(dir, "feedback.json")); err != nil {
		t.Fatalf("feedback.json not created: %v", err)
	}

	// Get recent
	feedbacks, err := store.GetRecentFeedback(10)
	if err != nil {
		t.Fatalf("GetRecentFeedback failed: %v", err)
	}

	if len(feedbacks) != 1 {
		t.Fatalf("expected 1 feedback, got %d", len(feedbacks))
	}

	if feedbacks[0].Prompt != "Create a battery monitor" {
		t.Errorf("unexpected prompt: %s", feedbacks[0].Prompt)
	}

	if feedbacks[0].Action != ActionAccept {
		t.Errorf("unexpected action: %s", feedbacks[0].Action)
	}

	if feedbacks[0].ID == "" {
		t.Error("expected auto-generated ID")
	}
}

func TestGetRecentFeedback_Limit(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	for i := 0; i < 5; i++ {
		store.SaveFeedback(Feedback{
			Prompt:    "test prompt",
			Action:    ActionAccept,
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
		})
	}

	feedbacks, err := store.GetRecentFeedback(3)
	if err != nil {
		t.Fatalf("GetRecentFeedback failed: %v", err)
	}

	if len(feedbacks) != 3 {
		t.Errorf("expected 3 feedback, got %d", len(feedbacks))
	}

	// Should be sorted by timestamp descending
	for i := 1; i < len(feedbacks); i++ {
		if feedbacks[i].Timestamp.After(feedbacks[i-1].Timestamp) {
			t.Errorf("feedback not sorted: %v after %v", feedbacks[i].Timestamp, feedbacks[i-1].Timestamp)
		}
	}
}

func TestAnalyzePatterns(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Add mixed feedback
	store.SaveFeedback(Feedback{
		Prompt:    "battery monitor daemon",
		Action:    ActionAccept,
		Timestamp: time.Now(),
	})
	store.SaveFeedback(Feedback{
		Prompt:       "battery monitor",
		Action:       ActionReject,
		ErrorSummary: "undefined: unused variable x",
		Timestamp:    time.Now(),
	})
	store.SaveFeedback(Feedback{
		Prompt:       "battery monitor",
		Action:       ActionReject,
		ErrorSummary: "undefined: unused variable y",
		Timestamp:    time.Now(),
	})
	store.SaveFeedback(Feedback{
		Prompt:       "battery monitor",
		Action:       ActionEdit,
		ErrorSummary: "syntax error: unexpected",
		Timestamp:    time.Now(),
	})

	report, err := store.AnalyzePatterns()
	if err != nil {
		t.Fatalf("AnalyzePatterns failed: %v", err)
	}

	if !strings.Contains(report, "Feedback Summary") {
		t.Error("report missing summary")
	}

	if !strings.Contains(report, "Accept") {
		t.Error("report missing accept count")
	}

	if !strings.Contains(report, "Reject") {
		t.Error("report missing reject count")
	}

	if !strings.Contains(report, "Top Error Patterns") {
		t.Error("report missing error patterns")
	}

	if !strings.Contains(report, "unused variable") {
		t.Error("report missing specific error pattern")
	}
}

func TestAnalyzePatterns_Empty(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	report, err := store.AnalyzePatterns()
	if err != nil {
		t.Fatalf("AnalyzePatterns failed: %v", err)
	}

	if !strings.Contains(report, "No feedback data") {
		t.Errorf("expected empty message, got: %s", report)
	}
}

func TestSaveFeedback_AutoID(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	fb := Feedback{
		Prompt: "test",
		Action: ActionAccept,
	}

	if err := store.SaveFeedback(fb); err != nil {
		t.Fatalf("SaveFeedback failed: %v", err)
	}

	feedbacks, _ := store.GetRecentFeedback(1)
	if len(feedbacks) == 0 {
		t.Fatal("no feedback saved")
	}

	if feedbacks[0].ID == "" {
		t.Error("expected auto-generated ID")
	}

	if !strings.HasPrefix(feedbacks[0].ID, "fb_") {
		t.Errorf("expected ID prefix fb_, got: %s", feedbacks[0].ID)
	}
}

func TestMultipleSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	actions := []Action{ActionAccept, ActionReject, ActionEdit, ActionAccept}
	for _, a := range actions {
		store.SaveFeedback(Feedback{
			Prompt:    "test",
			Action:    a,
			Timestamp: time.Now(),
		})
	}

	feedbacks, err := store.GetRecentFeedback(100)
	if err != nil {
		t.Fatalf("GetRecentFeedback failed: %v", err)
	}

	if len(feedbacks) != 4 {
		t.Errorf("expected 4 feedback, got %d", len(feedbacks))
	}

	// Verify persistence — create new store
	store2 := NewStore(dir)
	feedbacks2, err := store2.GetRecentFeedback(100)
	if err != nil {
		t.Fatalf("GetRecentFeedback (new store) failed: %v", err)
	}

	if len(feedbacks2) != 4 {
		t.Errorf("expected 4 feedback after reload, got %d", len(feedbacks2))
	}
}

func TestAnalyzePatterns_Keywords(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Add rejected feedback with common keywords
	for i := 0; i < 3; i++ {
		store.SaveFeedback(Feedback{
			Prompt:    "battery monitor service daemon",
			Action:    ActionReject,
			Timestamp: time.Now(),
		})
	}

	report, err := store.AnalyzePatterns()
	if err != nil {
		t.Fatalf("AnalyzePatterns failed: %v", err)
	}

	if !strings.Contains(report, "Common Keywords") {
		t.Error("report missing keyword analysis")
	}

	if !strings.Contains(report, "battery") {
		t.Error("report missing 'battery' keyword")
	}
}
