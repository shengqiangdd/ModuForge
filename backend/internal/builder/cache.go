package builder

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// BuildResult holds the outcome of a build operation.
type BuildResult struct {
	Success  bool          `json:"success"`
	Stdout   string        `json:"stdout,omitempty"`
	Stderr   string        `json:"stderr,omitempty"`
	Duration time.Duration `json:"duration"`
}

// CacheEntry stores a cached build result.
type CacheEntry struct {
	Key         string        `json:"key"`
	BuildResult BuildResult   `json:"build_result"`
	Timestamp   time.Time     `json:"timestamp"`
	Duration    time.Duration `json:"duration"`
}

// CacheStats holds cache hit/miss statistics.
type CacheStats struct {
	Hits         int     `json:"hits"`
	Misses       int     `json:"misses"`
	HitRate      float64 `json:"hit_rate"`
	TotalEntries int     `json:"total_entries"`
}

// BuildCache provides intelligent caching for build operations.
type BuildCache struct {
	mu      sync.RWMutex
	dir     string
	entries map[string]CacheEntry
	hits    int
	misses  int
}

// NewBuildCache creates a cache backed by JSON in dataDir.
func NewBuildCache(dataDir string) *BuildCache {
	return &BuildCache{
		dir:     dataDir,
		entries: make(map[string]CacheEntry),
	}
}

// GetCache retrieves a cached entry by key.
func (c *BuildCache) GetCache(key string) (*CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.load()
	entry, ok := c.entries[key]
	if ok {
		c.hits++
		return &entry, true
	}
	c.misses++
	return nil, false
}

// SetCache stores a build result under the given key.
func (c *BuildCache) SetCache(key string, entry CacheEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load: %w", err)
	}

	entry.Key = key
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	c.entries[key] = entry
	return c.save()
}

// ComputeFileHash calculates SHA256 hash of a file set.
func (c *BuildCache) ComputeFileHash(files map[string]string) string {
	// Sort keys for deterministic hash
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(files[k]))
		h.Write([]byte{0})
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

// Cleanup removes cache entries older than maxAge.
func (c *BuildCache) Cleanup(maxAge time.Duration) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.load()

	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for key, entry := range c.entries {
		if entry.Timestamp.Before(cutoff) {
			delete(c.entries, key)
			removed++
		}
	}

	if removed > 0 {
		c.save()
	}

	return removed
}

// GetStats returns cache hit/miss statistics.
func (c *BuildCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.load()

	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}

	return CacheStats{
		Hits:         c.hits,
		Misses:       c.misses,
		HitRate:      hitRate,
		TotalEntries: len(c.entries),
	}
}

// ResetStats resets hit/miss counters.
func (c *BuildCache) ResetStats() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits = 0
	c.misses = 0
}

// Size returns the number of cached entries.
func (c *BuildCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	c.load()
	return len(c.entries)
}

// ═══════════════════════════════════════════════════════
// Internal helpers
// ═══════════════════════════════════════════════════════

func (c *BuildCache) load() error {
	path := filepath.Join(c.dir, "build_cache.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var entries []CacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	c.entries = make(map[string]CacheEntry, len(entries))
	for _, e := range entries {
		c.entries[e.Key] = e
	}
	return nil
}

func (c *BuildCache) save() error {
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return err
	}
	var entries []CacheEntry
	for _, e := range c.entries {
		entries = append(entries, e)
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.dir, "build_cache.json"), data, 0644)
}

// ComputeStringHash computes SHA256 of a string.
func ComputeStringHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

// BuildCacheKey creates a cache key from file contents.
func BuildCacheKey(files map[string]string) string {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k + "\x00" + files[k] + "\x00"))
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

// HashFiles computes a combined hash for multiple files on disk.
func HashFiles(paths []string) (string, error) {
	h := sha256.New()

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", path, err)
		}
		h.Write([]byte(path))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// relativePath computes relative path from base.
func relativePath(path, base string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// joinPath joins path components.
func joinPath(parts ...string) string {
	return filepath.Join(parts...)
}

// ensureDir creates a directory if it doesn't exist.
func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// readFile reads a file and returns its contents.
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// writeFile writes data to a file.
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

// contains checks if a string slice contains a value.
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// unique returns unique strings from a slice.
func unique(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// join concatenates strings with a separator.
func join(slice []string, sep string) string {
	return strings.Join(slice, sep)
}

// split splits a string by separator.
func split(s, sep string) []string {
	return strings.Split(s, sep)
}

// trimSpace trims whitespace from a string.
func trimSpace(s string) string {
	return strings.TrimSpace(s)
}
