package handler

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"
	"time"
)

func openSlogTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Skipf("sqlite3 not available: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("sqlite3 ping failed: %v", err)
	}
	// Force a single connection: sqlite :memory: databases are per-connection, so
	// without this the write (db.Exec) and read (db.QueryRow) test queries would hit
	// distinct empty in-memory DBs and report 0 rows.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE app_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		level TEXT NOT NULL,
		module TEXT,
		message TEXT NOT NULL,
		details TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create app_logs: %v", err)
	}
	return db
}

func TestDBSlogHandler_PersistsWarnAndError(t *testing.T) {
	db := openSlogTestDB(t)
	defer db.Close()

	// Wrapper must preserve stdout output (via a discardable buffer handler),
	// while also writing to a real DB. Restore the default after the test.
	prev := slog.Default()
	defer slog.SetDefault(prev)

	EnableDBLogSink(db)

	slog.Error("MCP tool failed", "tool", "create_repository", "attempt", 2)
	slog.Warn("rate limited", "server", "github")

	// Allow the async write to land.
	time.Sleep(100 * time.Millisecond)

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_logs WHERE level='ERROR'`).Scan(&n); err != nil {
		t.Fatalf("count error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 ERROR row, got %d (want the MCP failure persisted)", n)
	}

	var warnMsg, warnModule string
	if err := db.QueryRow(`SELECT message, module FROM app_logs WHERE level='WARN'`).Scan(&warnMsg, &warnModule); err != nil {
		t.Fatalf("select warn: %v", err)
	}
	if warnMsg != "rate limited" || warnModule != "github" {
		t.Fatalf("warn row = %q/%q, want message='rate limited' module='github'", warnMsg, warnModule)
	}
}

func TestDBSlogHandler_DoesNotPersistInfo(t *testing.T) {
	db := openSlogTestDB(t)
	defer db.Close()

	prev := slog.Default()
	defer slog.SetDefault(prev)

	EnableDBLogSink(db)
	slog.Info("heartbeat ok", "module", "health")

	time.Sleep(100 * time.Millisecond)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_logs`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 rows for INFO-only logging, got %d (only WARN+ should persist)", n)
	}
}

func TestDBSlogHandler_UnderlyingOutputPreserved(t *testing.T) {
	var buf syncBuffer
	db := openSlogTestDB(t)
	defer db.Close()

	base := slog.NewTextHandler(&buf, nil)
	prev := slog.Default()
	defer slog.SetDefault(prev)
	slog.SetDefault(slog.New(&dBSlogHandler{Handler: base, db: db}))

	slog.Warn("hello", "k", "v")
	time.Sleep(100 * time.Millisecond)
	if buf.Len() == 0 {
		t.Fatal("expected underlying handler output, got empty")
	}
}

// syncBuffer is a minimal thread-safe bytes.Buffer for the text handler.
type syncBuffer struct {
	mu []byte
}

func (b *syncBuffer) Write(p []byte) (int, error) { b.mu = append(b.mu, p...); return len(p), nil }
func (b *syncBuffer) Len() int                    { return len(b.mu) }

var _ = context.Background // keep import used if future edits drop it
