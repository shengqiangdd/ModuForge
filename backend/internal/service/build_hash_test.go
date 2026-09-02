package service

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"testing"

	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/storage"
)

// TestComputeFilesHashS3FastPath verifies that in S3 mode, computeFilesHash
// uses cached per-file sha256 metadata instead of downloading content.
// This is the P0 optimization: builds avoid full S3 downloads on cache check.
func TestComputeFilesHashS3FastPath(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Skipf("sqlite3 not available: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Skipf("sqlite3 ping failed (CGO disabled): %v", err)
	}

	// Minimal project_files schema (only columns the hash query touches)
	if _, err := db.Exec(`CREATE TABLE project_files (
		project_id TEXT NOT NULL,
		path TEXT NOT NULL,
		content TEXT,
		sha256 TEXT,
		file_size INTEGER DEFAULT 0,
		mtime TEXT,
		created_at TEXT,
		updated_at TEXT,
		PRIMARY KEY (project_id, path)
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	// Seed rows with cached sha256 (S3 authoritative mode: content NULL)
	files := []struct {
		path string
		sha  string
	}{
		{"module.prop", "aaaa"},
		{"customize.sh", "bbbb"},
		{"service.sh", "cccc"},
	}
	for _, f := range files {
		if _, err := db.Exec(`INSERT INTO project_files (project_id, path, sha256, file_size, mtime, created_at, updated_at) VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
			"p1", f.path, f.sha, 100, "2026-01-01T00:00:00Z"); err != nil {
			t.Fatalf("insert %s: %v", f.path, err)
		}
	}

	// FileContentRepo with a S3 adapter that FAILS reads — fast path must not touch it
	failingS3 := &failReadAdapter{}
	fr := NewFileContentRepo(db, failingS3)
	bs := NewBuildService(db, &config.Config{StoragePath: t.TempDir()})
	bs.SetFileContentRepo(fr)

	// First hash: all rows have cached sha256 → no S3 reads, deterministic result
	h1, err := bs.computeFilesHash(context.Background(), "p1")
	if err != nil {
		t.Fatalf("computeFilesHash: %v", err)
	}
	if h1 == "" {
		t.Fatal("empty hash")
	}
	if failingS3.reads != 0 {
		t.Fatalf("expected 0 S3 reads with cached sha256, got %d", failingS3.reads)
	}

	// Same content → same hash (cache stability)
	h2, err := bs.computeFilesHash(context.Background(), "p1")
	if err != nil {
		t.Fatalf("computeFilesHash second call: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("hash not stable: %s != %s", h1, h2)
	}

	// Change one file's sha → hash must change
	if _, err := db.Exec(`UPDATE project_files SET sha256='dddd' WHERE project_id='p1' AND path='module.prop'`); err != nil {
		t.Fatalf("update: %v", err)
	}
	h3, err := bs.computeFilesHash(context.Background(), "p1")
	if err != nil {
		t.Fatalf("computeFilesHash after change: %v", err)
	}
	if h3 == h1 {
		t.Fatal("hash did not change after content change")
	}
}

// TestComputeFilesHashLegacyFallback verifies files without cached sha256 fall
// back to content reads (S3 first), so legacy rows still produce a stable hash.
func TestComputeFilesHashLegacyFallback(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Skipf("sqlite3 not available: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Skipf("sqlite3 ping failed (CGO disabled): %v", err)
	}

	if _, err := db.Exec(`CREATE TABLE project_files (
		project_id TEXT NOT NULL,
		path TEXT NOT NULL,
		content TEXT,
		sha256 TEXT,
		file_size INTEGER DEFAULT 0,
		mtime TEXT,
		created_at TEXT,
		updated_at TEXT,
		PRIMARY KEY (project_id, path)
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	// Seed: one row WITHOUT sha256 (legacy), one WITH
	if _, err := db.Exec(`INSERT INTO project_files (project_id, path, sha256, file_size, mtime, created_at, updated_at) VALUES
		('p1', 'module.prop', 'aaaa', 100, '2026-01-01T00:00:00Z', datetime('now'), datetime('now')),
		('p1', 'customize.sh', '', 100, '2026-01-01T00:00:00Z', datetime('now'), datetime('now'))`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	memS3 := &memReadAdapter{contents: map[string][]byte{
		"p1/customize.sh": []byte("#!/system/bin/sh\necho hi\n"),
	}}
	fr := NewFileContentRepo(db, memS3)
	bs := NewBuildService(db, &config.Config{StoragePath: t.TempDir()})
	bs.SetFileContentRepo(fr)

	h1, err := bs.computeFilesHash(context.Background(), "p1")
	if err != nil {
		t.Fatalf("computeFilesHash: %v", err)
	}
	if h1 == "" {
		t.Fatal("empty hash")
	}
	// Legacy row triggered exactly one S3 read
	if memS3.reads != 1 {
		t.Fatalf("expected 1 S3 read for legacy row, got %d", memS3.reads)
	}

	// Content change in S3 for the legacy row → hash must change
	memS3.contents["p1/customize.sh"] = []byte("#!/system/bin/sh\necho changed\n")
	h2, err := bs.computeFilesHash(context.Background(), "p1")
	if err != nil {
		t.Fatalf("computeFilesHash after s3 change: %v", err)
	}
	if h2 == h1 {
		t.Fatal("hash did not change after S3 content change")
	}
}

// failReadAdapter is a StorageAdapter whose reads always fail — proves the
// fast path never issues S3 reads.
type failReadAdapter struct {
	reads int
}

func (a *failReadAdapter) Read(ctx context.Context, key string) ([]byte, error) {
	a.reads++
	return nil, fmt.Errorf("s3 read should not be called: %s", key)
}

func (a *failReadAdapter) Write(ctx context.Context, path string, content []byte) error {
	return nil
}

func (a *failReadAdapter) ReadStream(ctx context.Context, path string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *failReadAdapter) Delete(ctx context.Context, path string) error {
	return nil
}

func (a *failReadAdapter) Exists(ctx context.Context, path string) (bool, error) {
	return false, nil
}

func (a *failReadAdapter) Stat(ctx context.Context, path string) (*storage.FileInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *failReadAdapter) List(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}

func (a *failReadAdapter) Close() error {
	return nil
}

// memReadAdapter is an in-memory StorageAdapter for fallback tests.
type memReadAdapter struct {
	contents map[string][]byte
	reads    int
}

func (a *memReadAdapter) Read(ctx context.Context, key string) ([]byte, error) {
	a.reads++
	c, ok := a.contents[key]
	if !ok {
		return nil, fmt.Errorf("not found: %s", key)
	}
	return c, nil
}

func (a *memReadAdapter) Write(ctx context.Context, path string, content []byte) error {
	a.contents[path] = content
	return nil
}

func (a *memReadAdapter) ReadStream(ctx context.Context, path string) (io.ReadCloser, error) {
	c, ok := a.contents[path]
	if !ok {
		return nil, fmt.Errorf("not found: %s", path)
	}
	return io.NopCloser(bytes.NewReader(c)), nil
}

func (a *memReadAdapter) Delete(ctx context.Context, path string) error {
	delete(a.contents, path)
	return nil
}

func (a *memReadAdapter) Exists(ctx context.Context, path string) (bool, error) {
	_, ok := a.contents[path]
	return ok, nil
}

func (a *memReadAdapter) Stat(ctx context.Context, path string) (*storage.FileInfo, error) {
	c, ok := a.contents[path]
	if !ok {
		return nil, nil
	}
	return &storage.FileInfo{
		Path: path,
		Size: int64(len(c)),
	}, nil
}

func (a *memReadAdapter) List(ctx context.Context, prefix string) ([]string, error) {
	var out []string
	for k := range a.contents {
		if bytes.HasPrefix([]byte(k), []byte(prefix)) {
			out = append(out, k)
		}
	}
	return out, nil
}

func (a *memReadAdapter) Close() error {
	return nil
}
