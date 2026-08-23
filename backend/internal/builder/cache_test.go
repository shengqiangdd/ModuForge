package builder

import (
	"testing"
	"time"
)

func TestNewBuildCache(t *testing.T) {
	c := NewBuildCache(t.TempDir())
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
}

func TestSetAndGetCache(t *testing.T) {
	c := NewBuildCache(t.TempDir())

	entry := CacheEntry{
		Key: "test_key",
		BuildResult: BuildResult{
			Success: true,
			Stdout:  "build output",
		},
		Timestamp: time.Now(),
	}

	if err := c.SetCache("test_key", entry); err != nil {
		t.Fatalf("SetCache failed: %v", err)
	}

	got, ok := c.GetCache("test_key")
	if !ok {
		t.Fatal("expected cache hit")
	}

	if !got.BuildResult.Success {
		t.Error("expected success=true")
	}

	if got.BuildResult.Stdout != "build output" {
		t.Errorf("expected 'build output', got %s", got.BuildResult.Stdout)
	}
}

func TestGetCache_Miss(t *testing.T) {
	c := NewBuildCache(t.TempDir())

	_, ok := c.GetCache("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestComputeFileHash(t *testing.T) {
	c := NewBuildCache(t.TempDir())

	files1 := map[string]string{"a.go": "content1", "b.go": "content2"}
	files2 := map[string]string{"a.go": "content1", "b.go": "content2"}
	files3 := map[string]string{"a.go": "content1", "b.go": "different"}

	hash1 := c.ComputeFileHash(files1)
	hash2 := c.ComputeFileHash(files2)
	hash3 := c.ComputeFileHash(files3)

	if hash1 != hash2 {
		t.Error("same files should produce same hash")
	}

	if hash1 == hash3 {
		t.Error("different files should produce different hash")
	}
}

func TestComputeFileHash_Deterministic(t *testing.T) {
	c := NewBuildCache(t.TempDir())

	files := map[string]string{"x.go": "code"}

	h1 := c.ComputeFileHash(files)
	h2 := c.ComputeFileHash(files)

	if h1 != h2 {
		t.Error("hash should be deterministic")
	}
}

func TestCleanup(t *testing.T) {
	c := NewBuildCache(t.TempDir())

	// Add old entry
	oldEntry := CacheEntry{
		Key:       "old",
		Timestamp: time.Now().Add(-48 * time.Hour),
	}
	c.SetCache("old", oldEntry)

	// Add new entry
	newEntry := CacheEntry{
		Key:       "new",
		Timestamp: time.Now(),
	}
	c.SetCache("new", newEntry)

	// Cleanup entries older than 24h
	removed := c.Cleanup(24 * time.Hour)
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	// New entry should still exist
	_, ok := c.GetCache("new")
	if !ok {
		t.Error("expected new entry to still exist")
	}

	// Old entry should be gone
	_, ok = c.GetCache("old")
	if ok {
		t.Error("expected old entry to be removed")
	}
}

func TestGetStats(t *testing.T) {
	c := NewBuildCache(t.TempDir())

	c.SetCache("a", CacheEntry{Key: "a"})
	c.SetCache("b", CacheEntry{Key: "b"})

	c.GetCache("a") // hit
	c.GetCache("b") // hit
	c.GetCache("c") // miss

	stats := c.GetStats()

	if stats.Hits != 2 {
		t.Errorf("expected 2 hits, got %d", stats.Hits)
	}

	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}

	if stats.TotalEntries != 2 {
		t.Errorf("expected 2 entries, got %d", stats.TotalEntries)
	}
}

func TestResetStats(t *testing.T) {
	c := NewBuildCache(t.TempDir())

	c.GetCache("nonexistent")
	c.ResetStats()

	stats := c.GetStats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Error("expected reset stats")
	}
}

func TestSize(t *testing.T) {
	c := NewBuildCache(t.TempDir())

	c.SetCache("a", CacheEntry{Key: "a"})
	c.SetCache("b", CacheEntry{Key: "b"})

	if c.Size() != 2 {
		t.Errorf("expected 2, got %d", c.Size())
	}
}

func TestComputeStringHash(t *testing.T) {
	h1 := ComputeStringHash("hello")
	h2 := ComputeStringHash("hello")
	h3 := ComputeStringHash("world")

	if h1 != h2 {
		t.Error("same input should produce same hash")
	}

	if h1 == h3 {
		t.Error("different input should produce different hash")
	}
}

func TestBuildCacheKey(t *testing.T) {
	files1 := map[string]string{"a.go": "code1"}
	files2 := map[string]string{"a.go": "code1"}
	files3 := map[string]string{"a.go": "code2"}

	key1 := BuildCacheKey(files1)
	key2 := BuildCacheKey(files2)
	key3 := BuildCacheKey(files3)

	if key1 != key2 {
		t.Error("same files should produce same key")
	}

	if key1 == key3 {
		t.Error("different files should produce different key")
	}
}

func TestCacheEntry_Fields(t *testing.T) {
	entry := CacheEntry{
		Key: "test",
		BuildResult: BuildResult{
			Success:  true,
			Stdout:   "ok",
			Stderr:   "",
			Duration: 100 * time.Millisecond,
		},
		Timestamp: time.Now(),
		Duration:  100 * time.Millisecond,
	}

	if entry.Key != "test" {
		t.Errorf("expected test, got %s", entry.Key)
	}

	if entry.BuildResult.Duration != 100*time.Millisecond {
		t.Errorf("expected 100ms, got %v", entry.BuildResult.Duration)
	}
}

func TestCacheStats_Fields(t *testing.T) {
	stats := CacheStats{
		Hits:         10,
		Misses:       5,
		HitRate:      66.7,
		TotalEntries: 20,
	}

	if stats.Hits != 10 {
		t.Errorf("expected 10, got %d", stats.Hits)
	}

	if stats.HitRate != 66.7 {
		t.Errorf("expected 66.7, got %.1f", stats.HitRate)
	}
}
