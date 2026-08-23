package versioning

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSnapshotManager(t *testing.T) {
	sm := NewSnapshotManager("/tmp/project")
	if sm == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestTakeAndRestoreSnapshot(t *testing.T) {
	// Create project directory
	projectDir := t.TempDir()
	sm := NewSnapshotManager(projectDir)

	// Create some files
	os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main\n\nfunc main() {}"), 0644)
	os.WriteFile(filepath.Join(projectDir, "module.prop"), []byte("id=test\nname=Test"), 0644)
	os.MkdirAll(filepath.Join(projectDir, "src"), 0755)
	os.WriteFile(filepath.Join(projectDir, "src", "helper.go"), []byte("package src"), 0644)

	// Take snapshot
	id, err := sm.TakeSnapshot(projectDir, "initial")
	if err != nil {
		t.Fatalf("TakeSnapshot failed: %v", err)
	}

	if id == "" {
		t.Fatal("expected non-empty snapshot ID")
	}

	// Modify files
	os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main\n\nfunc main() {\n\tprintln(\"modified\")\n}"), 0644)
	os.Remove(filepath.Join(projectDir, "module.prop"))

	// Verify modification
	data, _ := os.ReadFile(filepath.Join(projectDir, "main.go"))
	if len(data) < 50 {
		t.Fatal("expected modified file to be larger")
	}

	// Restore snapshot
	if err := sm.RestoreSnapshot(projectDir, string(id)); err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}

	// Verify restoration
	data, err = os.ReadFile(filepath.Join(projectDir, "main.go"))
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}

	if string(data) != "package main\n\nfunc main() {}" {
		t.Errorf("file not restored correctly: %s", string(data))
	}

	// module.prop should be restored
	if _, err := os.Stat(filepath.Join(projectDir, "module.prop")); os.IsNotExist(err) {
		t.Error("module.prop not restored")
	}
}

func TestListSnapshots(t *testing.T) {
	projectDir := t.TempDir()
	sm := NewSnapshotManager(projectDir)

	// Create file
	os.WriteFile(filepath.Join(projectDir, "test.go"), []byte("package main"), 0644)

	// Take multiple snapshots
	sm.TakeSnapshot(projectDir, "first")
	sm.TakeSnapshot(projectDir, "second")
	sm.TakeSnapshot(projectDir, "third")

	snapshots := sm.ListSnapshots(projectDir)
	if len(snapshots) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(snapshots))
	}

	// Should be sorted by timestamp descending
	if snapshots[0].Timestamp.Before(snapshots[2].Timestamp) {
		t.Error("snapshots not sorted by timestamp")
	}

	// Check labels
	labels := make(map[string]bool)
	for _, s := range snapshots {
		labels[s.Label] = true
	}
	if !labels["first"] || !labels["second"] || !labels["third"] {
		t.Error("missing expected labels")
	}
}

func TestDeleteSnapshot(t *testing.T) {
	projectDir := t.TempDir()
	sm := NewSnapshotManager(projectDir)

	os.WriteFile(filepath.Join(projectDir, "test.go"), []byte("package main"), 0644)

	id, _ := sm.TakeSnapshot(projectDir, "to-delete")

	snapshots := sm.ListSnapshots(projectDir)
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}

	if err := sm.DeleteSnapshot(string(id)); err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}

	snapshots = sm.ListSnapshots(projectDir)
	if len(snapshots) != 0 {
		t.Errorf("expected 0 snapshots after delete, got %d", len(snapshots))
	}
}

func TestSnapshotInfo_Fields(t *testing.T) {
	info := SnapshotInfo{
		ID:        "snap_123",
		Label:     "test",
		FileCount: 10,
		TotalSize: 1024,
	}

	if info.ID != "snap_123" {
		t.Errorf("expected snap_123, got %s", info.ID)
	}

	if info.FileCount != 10 {
		t.Errorf("expected 10 files, got %d", info.FileCount)
	}
}

func TestRestoreSnapshot_NotFound(t *testing.T) {
	projectDir := t.TempDir()
	sm := NewSnapshotManager(projectDir)

	err := sm.RestoreSnapshot(projectDir, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent snapshot")
	}
}

func TestListSnapshots_Empty(t *testing.T) {
	projectDir := t.TempDir()
	sm := NewSnapshotManager(projectDir)

	snapshots := sm.ListSnapshots(projectDir)
	if len(snapshots) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snapshots))
	}
}

func TestTakeSnapshot_Label(t *testing.T) {
	projectDir := t.TempDir()
	sm := NewSnapshotManager(projectDir)

	os.WriteFile(filepath.Join(projectDir, "test.go"), []byte("package main"), 0644)

	id, err := sm.TakeSnapshot(projectDir, "my-label")
	if err != nil {
		t.Fatalf("TakeSnapshot failed: %v", err)
	}

	snapshots := sm.ListSnapshots(projectDir)
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}

	if snapshots[0].Label != "my-label" {
		t.Errorf("expected label my-label, got %s", snapshots[0].Label)
	}

	if snapshots[0].ID != id {
		t.Errorf("expected ID %s, got %s", id, snapshots[0].ID)
	}
}

func TestTakeSnapshot_SkipsModuForge(t *testing.T) {
	projectDir := t.TempDir()
	sm := NewSnapshotManager(projectDir)

	// Create files
	os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main"), 0644)
	os.MkdirAll(filepath.Join(projectDir, ".moduforge", "cache"), 0755)
	os.WriteFile(filepath.Join(projectDir, ".moduforge", "cache", "data.json"), []byte("{}"), 0644)

	id, err := sm.TakeSnapshot(projectDir, "test")
	if err != nil {
		t.Fatalf("TakeSnapshot failed: %v", err)
	}

	// Restore to clean directory
	restoreDir := t.TempDir()
	if err := sm.RestoreSnapshot(restoreDir, string(id)); err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}

	// .moduforge should not be in restored files
	if _, err := os.Stat(filepath.Join(restoreDir, ".moduforge")); !os.IsNotExist(err) {
		t.Error(".moduforge should not be restored")
	}

	// main.go should be restored
	if _, err := os.Stat(filepath.Join(restoreDir, "main.go")); err != nil {
		t.Error("main.go should be restored")
	}
}

func TestSnapshotManager_EmptyProject(t *testing.T) {
	projectDir := t.TempDir()
	sm := NewSnapshotManager(projectDir)

	id, err := sm.TakeSnapshot(projectDir, "empty")
	if err != nil {
		t.Fatalf("TakeSnapshot failed: %v", err)
	}

	snapshots := sm.ListSnapshots(projectDir)
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}

	if snapshots[0].FileCount != 0 {
		t.Errorf("expected 0 files, got %d", snapshots[0].FileCount)
	}
}
