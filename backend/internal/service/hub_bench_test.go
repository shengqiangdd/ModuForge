package service

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
)

// BenchmarkWebSocketHub benchmarks the WebSocket Hub notify functions
// under concurrent load to verify no contention or deadlocks.
func BenchmarkWebSocketHub(b *testing.B) {
	hub := GetHub()

	// Subscribe N clients
	const numClients = 50
	for i := 0; i < numClients; i++ {
		uid := fmt.Sprintf("user_%d", i)
		// Use a nil conn — will fail on send, but we only benchmark the iteration
		client := hub.Subscribe(uid, nil)
		hub.SubscribeProject(fmt.Sprintf("project_%d", i%10), client)
	}

	b.ResetTimer()

	b.Run("NotifyUser", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			hub.NotifyUser("user_0", "test", "payload")
		}
	})

	b.Run("NotifyProject", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			hub.NotifyProject("project_0", "test", "payload")
		}
	})

	b.Run("Parallel-NotifyUser", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				hub.NotifyUser("user_0", "test", "payload")
			}
		})
	})

	b.Run("Parallel-NotifyProject", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				hub.NotifyProject("project_0", "test", "payload")
			}
		})
	})
}

// BenchmarkProjectServiceConcurrentOwnership benchmarks the ownership check
// hot path under concurrent access (the pattern fixed in T2/T3).
func BenchmarkProjectServiceConcurrentOwnership(b *testing.B) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		b.Skipf("sqlite3 not available: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		b.Skipf("sqlite3 ping failed (CGO disabled): %v", err)
	}

	db.Exec(`CREATE TABLE projects (id TEXT PRIMARY KEY, user_id TEXT, name TEXT, created_at TEXT, deleted_at TEXT)`)
	db.Exec(`CREATE TABLE project_files (id INTEGER PRIMARY KEY AUTOINCREMENT, project_id TEXT, path TEXT, content TEXT, created_at TEXT, updated_at TEXT)`)
	db.Exec(`INSERT INTO projects (id, user_id, name, created_at) VALUES ('bench-project', 'bench-user', 'bench', datetime('now'))`)
	for i := 0; i < 50; i++ {
		path := fmt.Sprintf("file_%d.go", i)
		db.Exec(`INSERT INTO project_files (project_id, path, content, created_at, updated_at) VALUES (?, ?, ?, datetime('now'), datetime('now'))`,
			"bench-project", path, "package main\n\nfunc main() {}\n")
	}

	svc := NewProjectService(db, "")
	ctx := context.Background()

	b.Run("Parallel-ListFiles", func(b *testing.B) {
		var mu sync.Mutex
		counter := 0
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				mu.Lock()
				uid := fmt.Sprintf("bench-user-%d", counter)
				counter++
				mu.Unlock()
				svc.ListFiles(ctx, "bench-project", uid)
			}
		})
	})
}