package agent

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// toolResultCacheMax is the maximum number of entries per agent session.
const toolResultCacheMax = 200

// cacheMaxEntrySize caps individual cached entries to save memory.
// Results larger than this (e.g. read_file on a big file) are truncated
// with a marker so callers know the full result is available on re-fetch.
const cacheMaxEntrySize = 65536 // 64 KB

type toolResultCache struct {
	entries     map[string]string // key -> result (O(1) lookup)
	accessOrder []string          // LRU: most recently used at end, least recently at front
	maxSize     int
	mu          sync.RWMutex // protects concurrent access from parallel tool goroutines
}

func newToolResultCache() *toolResultCache {
	return &toolResultCache{
		entries: make(map[string]string),
		maxSize: toolResultCacheMax,
	}
}

// cacheKey builds a deterministic cache key from skill name and input params.
func (c *toolResultCache) cacheKey(skillName string, input map[string]interface{}) string {
	// Sort keys for deterministic ordering
	var keys []string
	for k := range input {
		if k == "project_id" || k == "user_id" {
			continue // skip injected params
		}
		keys = append(keys, k)
	}
	// Simple sort (no imports needed for small slices)
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	var sb strings.Builder
	sb.WriteString(skillName)
	for _, k := range keys {
		sb.WriteString("|")
		sb.WriteString(k)
		sb.WriteString("=")
		if v, ok := input[k]; ok {
			if s, ok := v.(string); ok {
				sb.WriteString(s)
			} else {
				fmt.Fprintf(&sb, "%v", v)
			}
		}
	}
	return sb.String()
}

// isCacheable returns true if this tool's result can be safely cached.
func (c *toolResultCache) isCacheable(skillName string) bool {
	// Cache read-only tools + build_module (expensive operation)
	return skillName == "read_file" || skillName == "list_dir" ||
		skillName == "grep_search" || skillName == "glob_search" ||
		skillName == "build_module"
}

// Get returns cached result if available, empty string otherwise.
// LRU: moves accessed key to end of accessOrder (most recently used).
func (c *toolResultCache) get(skillName string, input map[string]interface{}) string {
	if !c.isCacheable(skillName) {
		return ""
	}
	key := c.cacheKey(skillName, input)
	c.mu.Lock()
	result, ok := c.entries[key]
	if ok {
		// LRU: move to end of accessOrder
		c.moveToEnd(key)
		log.Printf("[ToolCache] HIT: %s", skillName)
	}
	c.mu.Unlock()
	if ok {
		return result
	}
	return ""
}

// moveToEnd moves a key to the end of accessOrder (marks as most recently used).
// Must be called with c.mu held.
func (c *toolResultCache) moveToEnd(key string) {
	for i, k := range c.accessOrder {
		if k == key {
			c.accessOrder = append(c.accessOrder[:i], c.accessOrder[i+1:]...)
			c.accessOrder = append(c.accessOrder, key)
			return
		}
	}
}

// Put stores a result in the cache (LRU eviction).
func (c *toolResultCache) put(skillName string, input map[string]interface{}, result string) {
	if !c.isCacheable(skillName) {
		return
	}
	// Don't cache error results
	if strings.HasPrefix(result, "Error:") || strings.HasPrefix(result, "❌") {
		return
	}
	// Truncate large results to cap per-entry memory usage.
	// The marker tells callers the full content is still available via the tool.
	if len(result) > cacheMaxEntrySize {
		result = result[:cacheMaxEntrySize] + "\n...[cached summary — full result available via read_file]"
	}
	key := c.cacheKey(skillName, input)
	c.mu.Lock()
	defer c.mu.Unlock()
	// Update existing entry — move to end (most recently used)
	if _, exists := c.entries[key]; exists {
		c.entries[key] = result
		c.moveToEnd(key)
		log.Printf("[ToolCache] UPDATE: %s (total=%d)", skillName, len(c.entries))
		return
	}
	// Evict LRU (least recently used) if full
	if len(c.entries) >= c.maxSize {
		lru := c.accessOrder[0]
		c.accessOrder = c.accessOrder[1:]
		delete(c.entries, lru)
	}
	c.entries[key] = result
	c.accessOrder = append(c.accessOrder, key)
	log.Printf("[ToolCache] PUT: %s (total=%d)", skillName, len(c.entries))
}

// Invalidate clears cache entries matching a file path (called after write_file).
func (c *toolResultCache) invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	removed := 0
	var newAccessOrder []string
	for _, key := range c.accessOrder {
		if strings.Contains(key, path) {
			delete(c.entries, key)
			removed++
		} else {
			newAccessOrder = append(newAccessOrder, key)
		}
	}
	if removed > 0 {
		c.accessOrder = newAccessOrder
		log.Printf("[ToolCache] invalidated %d entries for path=%s", removed, path)
	}
}

// InvalidateBuild clears the build_module cache entry for a given project_id.
// Called after write_file to ensure stale build results aren't reused.
func (c *toolResultCache) InvalidateBuild(projectID string) {
	if projectID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	removed := 0
	var newAccessOrder []string
	for _, key := range c.accessOrder {
		if strings.Contains(key, "build_module") && strings.Contains(key, projectID) {
			delete(c.entries, key)
			removed++
		} else {
			newAccessOrder = append(newAccessOrder, key)
		}
	}
	if removed > 0 {
		c.accessOrder = newAccessOrder
		log.Printf("[ToolCache] invalidated %d build_module entries for project=%s", removed, projectID)
	}
}

// ═══════════════════════════════════════════════════════════════════
// File Hash Cache — SHA256-based file change detection
//
// Avoids re-reading unchanged files: read_file computes SHA256 of file
// content and returns "UNCHANGED" if the hash matches the cache.
// This saves tokens when Agent re-reads the same file multiple times.
// ═══════════════════════════════════════════════════════════════════

type fileHashCache struct {
	hashes map[string]string // path -> sha256 hex
	mu     sync.RWMutex
}

// NewFileHashCache creates a new file hash cache.
func NewFileHashCache() *fileHashCache {
	return &fileHashCache{
		hashes: make(map[string]string),
	}
}

// Get returns the cached hash for a path, or empty string if not cached.
func (c *fileHashCache) Get(path string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hashes[path]
}

// Set stores a hash for a path.
func (c *fileHashCache) Set(path, hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hashes[path] = hash
}

// Invalidate removes the cached hash for a path.
func (c *fileHashCache) Invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.hashes, path)
}
