package agent

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// toolResultCacheMax is the maximum number of entries per agent session.
const toolResultCacheMax = 30

// cacheMaxEntrySize caps individual cached entries to save memory.
// Results larger than this (e.g. read_file on a big file) are truncated
// with a marker so callers know the full result is available on re-fetch.
const cacheMaxEntrySize = 4096 // 4 KB

type toolResultCache struct {
	entries map[string]string // key -> result (O(1) lookup)
	order   []string          // insertion order for FIFO eviction
	maxSize int
	mu      sync.RWMutex // protects concurrent access from parallel tool goroutines
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
func (c *toolResultCache) get(skillName string, input map[string]interface{}) string {
	if !c.isCacheable(skillName) {
		return ""
	}
	key := c.cacheKey(skillName, input)
	c.mu.RLock()
	result, ok := c.entries[key]
	c.mu.RUnlock()
	if ok {
		log.Printf("[ToolCache] HIT: %s", skillName)
		return result
	}
	return ""
}

// Put stores a result in the cache.
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
	// Update existing entry
	if _, exists := c.entries[key]; exists {
		c.entries[key] = result
		return
	}
	// Evict oldest if full
	if len(c.entries) >= c.maxSize {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}
	c.entries[key] = result
	c.order = append(c.order, key)
	log.Printf("[ToolCache] PUT: %s (total=%d)", skillName, len(c.entries))
}

// Invalidate clears cache entries matching a file path (called after write_file).
func (c *toolResultCache) invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	removed := 0
	var newOrder []string
	for _, key := range c.order {
		if strings.Contains(key, path) {
			delete(c.entries, key)
			removed++
		} else {
			newOrder = append(newOrder, key)
		}
	}
	if removed > 0 {
		c.order = newOrder
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
	var newOrder []string
	for _, key := range c.order {
		if strings.Contains(key, "build_module") && strings.Contains(key, projectID) {
			delete(c.entries, key)
			removed++
		} else {
			newOrder = append(newOrder, key)
		}
	}
	if removed > 0 {
		c.order = newOrder
		log.Printf("[ToolCache] invalidated %d build_module entries for project=%s", removed, projectID)
	}
}
