package service

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
)

// BenchmarkProjectService benchmarks the hot paths of project service:
// file listing, saving, and searching.
func BenchmarkProjectService(b *testing.B) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		b.Skipf("sqlite3 not available: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		b.Skipf("sqlite3 ping failed (CGO disabled): %v", err)
	}

	// Create tables
	db.Exec(`CREATE TABLE projects (id TEXT PRIMARY KEY, user_id TEXT, name TEXT, created_at TEXT, deleted_at TEXT)`)
	db.Exec(`CREATE TABLE project_files (id INTEGER PRIMARY KEY AUTOINCREMENT, project_id TEXT, path TEXT, created_at TEXT, updated_at TEXT, sha256 TEXT DEFAULT '', file_size INTEGER DEFAULT 0, mtime TEXT DEFAULT '')`)

	// Insert a project
	db.Exec(`INSERT INTO projects (id, user_id, name, created_at) VALUES ('bench-project', 'bench-user', 'bench', datetime('now'))`)

	// Insert files
	for i := 0; i < 100; i++ {
		path := fmt.Sprintf("file_%d.go", i)
		db.Exec(`INSERT INTO project_files (project_id, path, sha256, file_size, mtime, created_at, updated_at) VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
			"bench-project", path, "dummy_sha256", 100, "2026-01-01T00:00:00Z")
	}

	svc := NewProjectService(db, "")

	b.Run("ListFiles", func(b *testing.B) {
		ctx := context.Background()
		for i := 0; i < b.N; i++ {
			svc.ListFiles(ctx, "bench-project", "bench-user")
		}
	})

	b.Run("GetFile", func(b *testing.B) {
		ctx := context.Background()
		for i := 0; i < b.N; i++ {
			svc.GetFile(ctx, "bench-project", "file_0.go", "bench-user")
		}
	})

	b.Run("SaveFile", func(b *testing.B) {
		ctx := context.Background()
		for i := 0; i < b.N; i++ {
			svc.SaveFile(ctx, "bench-project", "bench_file.go", "package main", "bench-user")
		}
	})

	b.Run("SearchAll", func(b *testing.B) {
		ctx := context.Background()
		for i := 0; i < b.N; i++ {
			svc.SearchAll(ctx, "bench-user", "bench")
		}
	})
}

// BenchmarkProjectServiceParallel runs the same benchmarks with parallel access
// to measure concurrency overhead.
func BenchmarkProjectServiceParallel(b *testing.B) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		b.Skipf("sqlite3 not available: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		b.Skipf("sqlite3 ping failed (CGO disabled): %v", err)
	}

	db.Exec(`CREATE TABLE projects (id TEXT PRIMARY KEY, user_id TEXT, name TEXT, created_at TEXT, deleted_at TEXT)`)
	db.Exec(`CREATE TABLE project_files (id INTEGER PRIMARY KEY AUTOINCREMENT, project_id TEXT, path TEXT, created_at TEXT, updated_at TEXT, sha256 TEXT DEFAULT '', file_size INTEGER DEFAULT 0, mtime TEXT DEFAULT '')`)
	db.Exec(`INSERT INTO projects (id, user_id, name, created_at) VALUES ('bench-project', 'bench-user', 'bench', datetime('now'))`)
	for i := 0; i < 100; i++ {
		path := fmt.Sprintf("file_%d.go", i)
		db.Exec(`INSERT INTO project_files (project_id, path, sha256, file_size, mtime, created_at, updated_at) VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
			"bench-project", path, "dummy_sha256", 100, "2026-01-01T00:00:00Z")
	}

	svc := NewProjectService(db, "")

	b.Run("Parallel-ListFiles", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			ctx := context.Background()
			for pb.Next() {
				svc.ListFiles(ctx, "bench-project", "bench-user")
			}
		})
	})

	b.Run("Parallel-GetFile", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			ctx := context.Background()
			for pb.Next() {
				svc.GetFile(ctx, "bench-project", "file_0.go", "bench-user")
			}
		})
	})
}
