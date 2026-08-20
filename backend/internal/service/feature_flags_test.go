package service

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file::memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Create the audit_logs table that SetEnabled/SetEnabledBatch write to.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS audit_logs (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id  TEXT NOT NULL DEFAULT '',
		user_id     TEXT NOT NULL DEFAULT '',
		action      TEXT NOT NULL,
		details     TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		t.Fatalf("failed to create audit_logs table: %v", err)
	}
	return db
}

func TestNewFeatureFlagService(t *testing.T) {
	db := newTestDB(t)
	svc := NewFeatureFlagService(db)

	flags := svc.List()
	if len(flags) != len(defaultFlags) {
		t.Errorf("expected %d default flags, got %d", len(defaultFlags), len(flags))
	}
}

func TestIsEnabled_DefaultFlags(t *testing.T) {
	db := newTestDB(t)
	svc := NewFeatureFlagService(db)

	for _, df := range defaultFlags {
		if !svc.IsEnabled(df.Key) {
			t.Errorf("expected default flag %q to be enabled", df.Key)
		}
	}
}

func TestIsEnabled_UnknownKey_FailOpen(t *testing.T) {
	db := newTestDB(t)
	svc := NewFeatureFlagService(db)

	if !svc.IsEnabled("nonexistent_flag") {
		t.Error("unknown key should default to enabled (fail-open)")
	}
}

func TestSetEnabled(t *testing.T) {
	db := newTestDB(t)
	svc := NewFeatureFlagService(db)

	if err := svc.SetEnabled("crash_reporting", false, "admin-1"); err != nil {
		t.Fatalf("SetEnabled failed: %v", err)
	}
	if svc.IsEnabled("crash_reporting") {
		t.Error("expected crash_reporting to be disabled after SetEnabled")
	}

	// Re-enable
	if err := svc.SetEnabled("crash_reporting", true, "admin-1"); err != nil {
		t.Fatalf("SetEnabled re-enable failed: %v", err)
	}
	if !svc.IsEnabled("crash_reporting") {
		t.Error("expected crash_reporting to be enabled after re-enable")
	}
}

func TestSetEnabled_AuditLog(t *testing.T) {
	db := newTestDB(t)
	svc := NewFeatureFlagService(db)

	_ = svc.SetEnabled("crash_reporting", false, "admin-1")

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action='feature_flag' AND user_id='admin-1'`).Scan(&count); err != nil {
		t.Fatalf("query audit_logs failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 audit log entry, got %d", count)
	}
}

func TestList(t *testing.T) {
	db := newTestDB(t)
	svc := NewFeatureFlagService(db)

	flags := svc.List()
	seen := make(map[string]bool)
	for _, f := range flags {
		seen[f.Key] = true
	}
	for _, df := range defaultFlags {
		if !seen[df.Key] {
			t.Errorf("default flag %q missing from List()", df.Key)
		}
	}
}

func TestSetEnabledBatch(t *testing.T) {
	db := newTestDB(t)
	svc := NewFeatureFlagService(db)

	items := []struct {
		Key     string `json:"key"`
		Enabled bool   `json:"enabled"`
	}{
		{Key: "crash_reporting", Enabled: false},
		{Key: "collaboration", Enabled: false},
		{Key: "favorites", Enabled: false},
	}

	if err := svc.SetEnabledBatch(items, "admin-1"); err != nil {
		t.Fatalf("SetEnabledBatch failed: %v", err)
	}

	for _, item := range items {
		if svc.IsEnabled(item.Key) {
			t.Errorf("expected %q to be disabled after batch update", item.Key)
		}
	}

	// Check audit logs
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action='feature_flag' AND user_id='admin-1'`).Scan(&count); err != nil {
		t.Fatalf("query audit_logs failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 audit log entries from batch, got %d", count)
	}
}

func TestSetEnabledBatch_Mixed(t *testing.T) {
	db := newTestDB(t)
	svc := NewFeatureFlagService(db)

	// Disable crash_reporting first
	_ = svc.SetEnabled("crash_reporting", false, "admin-1")

	// Batch: re-enable crash_reporting and disable collaboration
	items := []struct {
		Key     string `json:"key"`
		Enabled bool   `json:"enabled"`
	}{
		{Key: "crash_reporting", Enabled: true},
		{Key: "collaboration", Enabled: false},
	}

	if err := svc.SetEnabledBatch(items, "admin-1"); err != nil {
		t.Fatalf("SetEnabledBatch failed: %v", err)
	}

	if !svc.IsEnabled("crash_reporting") {
		t.Error("expected crash_reporting to be enabled after batch")
	}
	if svc.IsEnabled("collaboration") {
		t.Error("expected collaboration to be disabled after batch")
	}
}

func TestReload(t *testing.T) {
	db := newTestDB(t)
	svc := NewFeatureFlagService(db)

	// Directly modify DB to simulate external change
	db.Exec(`UPDATE feature_flags SET enabled=0 WHERE key='crash_reporting'`)

	svc.Reload()

	if svc.IsEnabled("crash_reporting") {
		t.Error("expected crash_reporting to be disabled after Reload()")
	}
}
