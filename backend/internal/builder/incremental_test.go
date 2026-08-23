package builder

import (
	"fmt"
	"testing"
	"time"
)

func TestNewIncrementalBuilder(t *testing.T) {
	ib := NewIncrementalBuilder()
	if ib == nil {
		t.Fatal("expected non-nil builder")
	}
}

func TestDetectChanges_Added(t *testing.T) {
	ib := NewIncrementalBuilder()

	old := map[string]string{"a.go": "content"}
	new := map[string]string{"a.go": "content", "b.go": "new"}

	changes := ib.DetectChanges(old, new)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}

	if changes[0].ChangeType != ChangeAdded {
		t.Errorf("expected added, got %s", changes[0].ChangeType)
	}

	if changes[0].Path != "b.go" {
		t.Errorf("expected b.go, got %s", changes[0].Path)
	}
}

func TestDetectChanges_Modified(t *testing.T) {
	ib := NewIncrementalBuilder()

	old := map[string]string{"a.go": "old content"}
	new := map[string]string{"a.go": "new content"}

	changes := ib.DetectChanges(old, new)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}

	if changes[0].ChangeType != ChangeModified {
		t.Errorf("expected modified, got %s", changes[0].ChangeType)
	}

	if changes[0].OldContent != "old content" {
		t.Errorf("expected 'old content', got %s", changes[0].OldContent)
	}
}

func TestDetectChanges_Deleted(t *testing.T) {
	ib := NewIncrementalBuilder()

	old := map[string]string{"a.go": "content", "b.go": "deleted"}
	new := map[string]string{"a.go": "content"}

	changes := ib.DetectChanges(old, new)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}

	if changes[0].ChangeType != ChangeDeleted {
		t.Errorf("expected deleted, got %s", changes[0].ChangeType)
	}
}

func TestDetectChanges_NoChanges(t *testing.T) {
	ib := NewIncrementalBuilder()

	old := map[string]string{"a.go": "content"}
	new := map[string]string{"a.go": "content"}

	changes := ib.DetectChanges(old, new)
	if len(changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(changes))
	}
}

func TestDetectChanges_Multiple(t *testing.T) {
	ib := NewIncrementalBuilder()

	old := map[string]string{"a.go": "old", "b.go": "keep"}
	new := map[string]string{"a.go": "new", "b.go": "keep", "c.go": "add"}

	changes := ib.DetectChanges(old, new)
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(changes))
	}

	types := make(map[ChangeType]int)
	for _, c := range changes {
		types[c.ChangeType]++
	}

	if types[ChangeModified] != 1 {
		t.Errorf("expected 1 modified, got %d", types[ChangeModified])
	}

	if types[ChangeAdded] != 1 {
		t.Errorf("expected 1 added, got %d", types[ChangeAdded])
	}
}

func TestGetAffectedFiles(t *testing.T) {
	ib := NewIncrementalBuilder()

	// a.go depends on nothing
	// b.go depends on a.go
	// c.go depends on b.go
	depGraph := map[string][]string{
		"a.go": {},
		"b.go": {"a.go"},
		"c.go": {"b.go"},
	}

	change := FileChange{Path: "a.go", ChangeType: ChangeModified}
	affected := ib.GetAffectedFiles(change, depGraph)

	// Should affect a.go, b.go, and c.go
	if len(affected) != 3 {
		t.Errorf("expected 3 affected, got %d: %v", len(affected), affected)
	}
}

func TestGetAffectedFiles_NoDependents(t *testing.T) {
	ib := NewIncrementalBuilder()

	depGraph := map[string][]string{
		"a.go": {},
		"b.go": {},
	}

	change := FileChange{Path: "a.go", ChangeType: ChangeModified}
	affected := ib.GetAffectedFiles(change, depGraph)

	if len(affected) != 1 {
		t.Errorf("expected 1 affected, got %d", len(affected))
	}
}

func TestBuildIncremental_NoChanges(t *testing.T) {
	ib := NewIncrementalBuilder()

	called := false
	builder := func(files map[string]string) (BuildResult, error) {
		called = true
		return BuildResult{Success: true}, nil
	}

	result, err := ib.BuildIncremental(nil, builder)
	if err != nil {
		t.Fatalf("BuildIncremental failed: %v", err)
	}

	if !result.Success {
		t.Error("expected success")
	}

	if called {
		t.Error("builder should not be called for no changes")
	}
}

func TestBuildIncremental_WithChanges(t *testing.T) {
	ib := NewIncrementalBuilder()

	changes := []FileChange{
		{Path: "a.go", ChangeType: ChangeAdded, NewContent: "package main"},
	}

	var receivedFiles map[string]string
	builder := func(files map[string]string) (BuildResult, error) {
		receivedFiles = files
		return BuildResult{Success: true, Stdout: "ok"}, nil
	}

	result, err := ib.BuildIncremental(changes, builder)
	if err != nil {
		t.Fatalf("BuildIncremental failed: %v", err)
	}

	if !result.Success {
		t.Error("expected success")
	}

	if receivedFiles == nil {
		t.Fatal("expected builder to receive files")
	}

	if receivedFiles["a.go"] != "package main" {
		t.Errorf("expected 'package main', got %s", receivedFiles["a.go"])
	}
}

func TestBuildIncremental_DeleteOnly(t *testing.T) {
	ib := NewIncrementalBuilder()

	changes := []FileChange{
		{Path: "a.go", ChangeType: ChangeDeleted, OldContent: "old"},
	}

	builder := func(files map[string]string) (BuildResult, error) {
		return BuildResult{Success: true}, nil
	}

	result, err := ib.BuildIncremental(changes, builder)
	if err != nil {
		t.Fatalf("BuildIncremental failed: %v", err)
	}

	// Delete-only changes should produce empty files map
	if !result.Success {
		t.Error("expected success")
	}
}

func TestBuildIncremental_BuilderError(t *testing.T) {
	ib := NewIncrementalBuilder()

	changes := []FileChange{
		{Path: "a.go", ChangeType: ChangeAdded, NewContent: "bad code"},
	}

	builder := func(files map[string]string) (BuildResult, error) {
		return BuildResult{Success: false, Stderr: "compile error"}, fmt.Errorf("build failed")
	}

	result, err := ib.BuildIncremental(changes, builder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Success {
		t.Error("expected failure")
	}

	if result.Stderr != "compile error" {
		t.Errorf("expected 'compile error', got %s", result.Stderr)
	}
}

func TestFilterChangesByType(t *testing.T) {
	changes := []FileChange{
		{Path: "a.go", ChangeType: ChangeAdded},
		{Path: "b.go", ChangeType: ChangeModified},
		{Path: "c.go", ChangeType: ChangeAdded},
	}

	added := FilterChangesByType(changes, ChangeAdded)
	if len(added) != 2 {
		t.Errorf("expected 2 added, got %d", len(added))
	}
}

func TestHasGoChanges(t *testing.T) {
	changes := []FileChange{
		{Path: "script.sh", ChangeType: ChangeAdded},
	}

	if HasGoChanges(changes) {
		t.Error("should not detect Go changes in .sh files")
	}

	changes = append(changes, FileChange{Path: "main.go", ChangeType: ChangeModified})

	if !HasGoChanges(changes) {
		t.Error("should detect Go changes")
	}
}

func TestHasShellChanges(t *testing.T) {
	changes := []FileChange{
		{Path: "main.go", ChangeType: ChangeAdded},
	}

	if HasShellChanges(changes) {
		t.Error("should not detect Shell changes in .go files")
	}

	changes = append(changes, FileChange{Path: "build.sh", ChangeType: ChangeModified})

	if !HasShellChanges(changes) {
		t.Error("should detect Shell changes")
	}
}

func TestChangeSummary(t *testing.T) {
	changes := []FileChange{
		{Path: "a.go", ChangeType: ChangeAdded},
		{Path: "b.go", ChangeType: ChangeModified},
		{Path: "c.go", ChangeType: ChangeDeleted},
	}

	summary := ChangeSummary(changes)
	if summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestChangeSummary_Empty(t *testing.T) {
	summary := ChangeSummary(nil)
	if summary != "no changes" {
		t.Errorf("expected 'no changes', got %s", summary)
	}
}

func TestFileChange_Fields(t *testing.T) {
	change := FileChange{
		Path:       "test.go",
		ChangeType: ChangeModified,
		OldContent: "old",
		NewContent: "new",
	}

	if change.Path != "test.go" {
		t.Errorf("expected test.go, got %s", change.Path)
	}

	if change.ChangeType != ChangeModified {
		t.Errorf("expected modified, got %s", change.ChangeType)
	}
}

func TestBuildResult_Duration(t *testing.T) {
	result := BuildResult{
		Success:  true,
		Duration: 100 * time.Millisecond,
	}

	if result.Duration != 100*time.Millisecond {
		t.Errorf("expected 100ms, got %v", result.Duration)
	}
}
