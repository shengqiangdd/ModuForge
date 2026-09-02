package skills

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/moduforge/backend/internal/storage"
)

// memHashCache is a simple in-memory FileHashCacheI for tests.
type memHashCache struct {
	mu     sync.Mutex
	hashes map[string]string
}

func newMemHashCache() *memHashCache {
	return &memHashCache{hashes: make(map[string]string)}
}

func (c *memHashCache) Get(path string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hashes[path]
}

func (c *memHashCache) Set(path, hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hashes[path] = hash
}

func (c *memHashCache) Invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.hashes, path)
}

// localFSAdapter is a simple StorageAdapter that reads from local filesystem.
// Used for testing without S3.
type localFSAdapter struct {
	basePath string
}

func newLocalFSAdapter(basePath string) *localFSAdapter {
	return &localFSAdapter{basePath: basePath}
}

func (a *localFSAdapter) Write(ctx context.Context, path string, content []byte) error {
	fullPath := filepath.Join(a.basePath, path)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, content, 0644)
}

func (a *localFSAdapter) Read(ctx context.Context, path string) ([]byte, error) {
	fullPath := filepath.Join(a.basePath, path)
	return os.ReadFile(fullPath)
}

func (a *localFSAdapter) ReadStream(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath := filepath.Join(a.basePath, path)
	return os.Open(fullPath)
}

func (a *localFSAdapter) Delete(ctx context.Context, path string) error {
	fullPath := filepath.Join(a.basePath, path)
	return os.Remove(fullPath)
}

func (a *localFSAdapter) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := filepath.Join(a.basePath, path)
	_, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func (a *localFSAdapter) Stat(ctx context.Context, path string) (*storage.FileInfo, error) {
	fullPath := filepath.Join(a.basePath, path)
	info, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &storage.FileInfo{
		Path:  path,
		Size:  info.Size(),
		MTime: info.ModTime().Format("2006-01-02T15:04:05Z"),
	}, nil
}

func (a *localFSAdapter) List(ctx context.Context, prefix string) ([]string, error) {
	var paths []string
	fullPrefix := filepath.Join(a.basePath, prefix)
	err := filepath.Walk(fullPrefix, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, _ := filepath.Rel(a.basePath, path)
			paths = append(paths, relPath)
		}
		return nil
	})
	return paths, err
}

func (a *localFSAdapter) Close() error {
	return nil
}

// writeLargeFile creates a file with more than 500 lines.
func writeLargeFile(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "large.go")
	var sb strings.Builder
	sb.WriteString("package main\n")
	for i := 0; i < 600; i++ {
		fmt.Fprintf(&sb, "// line %d\n", i)
		sb.WriteString("func f%d() int { return 0 }\n")
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadFileDifferentialCache_Unchanged(t *testing.T) {
	dir := t.TempDir()
	path := writeLargeFile(t, dir)
	cache := newMemHashCache()

	s := NewReadFileSkill(nil)
	s.storage = newLocalFSAdapter(dir)
	s.projectPath = dir
	s.SetFileHashCache(cache)

	// First read: full smart summary, caches the hash.
	first, err := s.Execute(context.Background(), map[string]interface{}{
		"path": filepath.Base(path),
	})
	if err != nil {
		t.Fatalf("first read failed: %v", err)
	}
	if !strings.Contains(first, "lines total") {
		t.Fatalf("first read should return smart summary, got: %.100s", first)
	}

	// Second read with unchanged content: differential hit -> UNCHANGED.
	second, err := s.Execute(context.Background(), map[string]interface{}{
		"path": filepath.Base(path),
	})
	if err != nil {
		t.Fatalf("second read failed: %v", err)
	}
	if !strings.Contains(second, "UNCHANGED") {
		t.Fatalf("expected UNCHANGED on second read, got: %.200s", second)
	}

	// Explicit range read must bypass the differential cache.
	rangeRead, err := s.Execute(context.Background(), map[string]interface{}{
		"path":       filepath.Base(path),
		"start_line": float64(1),
		"end_line":   float64(20),
	})
	if err != nil {
		t.Fatalf("range read failed: %v", err)
	}
	if strings.Contains(rangeRead, "UNCHANGED") {
		t.Fatalf("range read should bypass differential cache")
	}
	if !strings.Contains(rangeRead, "1:>") {
		t.Fatalf("range read should include line numbers, got: %.200s", rangeRead)
	}
}

func TestReadFileDifferentialCache_ChangedContent(t *testing.T) {
	dir := t.TempDir()
	path := writeLargeFile(t, dir)
	cache := newMemHashCache()

	s := NewReadFileSkill(nil)
	s.storage = newLocalFSAdapter(dir)
	s.projectPath = dir
	s.SetFileHashCache(cache)

	if _, err := s.Execute(context.Background(), map[string]interface{}{
		"path": filepath.Base(path),
	}); err != nil {
		t.Fatal(err)
	}

	// Modify the file -> hash differs -> full smart summary again.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\nfunc changed() {}\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	third, err := s.Execute(context.Background(), map[string]interface{}{
		"path": filepath.Base(path),
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(third, "UNCHANGED") {
		t.Fatalf("changed content should not be UNCHANGED")
	}
	if !strings.Contains(third, "lines total") {
		t.Fatalf("changed content should return smart summary, got: %.200s", third)
	}
}

func TestReadFileDifferentialCache_SmallFileAlwaysFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.txt")
	if err := os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cache := newMemHashCache()

	s := NewReadFileSkill(nil)
	s.storage = newLocalFSAdapter(dir)
	s.projectPath = dir
	s.SetFileHashCache(cache)

	first, err := s.Execute(context.Background(), map[string]interface{}{
		"path": filepath.Base(path),
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(first, "UNCHANGED") {
		t.Fatalf("small file should never be UNCHANGED (too cheap, avoid stale context)")
	}

	second, err := s.Execute(context.Background(), map[string]interface{}{
		"path": filepath.Base(path),
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(second, "UNCHANGED") {
		t.Fatalf("small file should always return full content, got UNCHANGED")
	}
	if !strings.Contains(second, "line2") {
		t.Fatalf("small file should include full content, got: %q", second)
	}
}

func TestReadFileDifferentialCache_InvalidateForcesReread(t *testing.T) {
	dir := t.TempDir()
	path := writeLargeFile(t, dir)
	cache := newMemHashCache()

	s := NewReadFileSkill(nil)
	s.storage = newLocalFSAdapter(dir)
	s.projectPath = dir
	s.SetFileHashCache(cache)

	if _, err := s.Execute(context.Background(), map[string]interface{}{
		"path": filepath.Base(path),
	}); err != nil {
		t.Fatal(err)
	}

	// Invalidate (as write_file/edit_file would do after modification).
	cache.Invalidate(filepath.Base(path))

	second, err := s.Execute(context.Background(), map[string]interface{}{
		"path": filepath.Base(path),
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(second, "UNCHANGED") {
		t.Fatalf("after invalidate, read should return full smart summary")
	}
	if !strings.Contains(second, "lines total") {
		t.Fatalf("expected smart summary after invalidate, got: %.200s", second)
	}
}
