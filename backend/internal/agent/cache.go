package agent

import (
	"container/list"
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

// toolResultCache implements an O(1) LRU cache using a doubly-linked list + map.
// - get/put/moveToEnd: O(1)
// - invalidate by path: O(k) where k = number of keys containing that path (typically 1-2)
// - Eviction: O(1) from list front
type toolResultCache struct {
	entries     map[string]*list.Element // key -> list element (O(1) lookup)
	accessOrder *list.List               // doubly-linked list: front = LRU, back = MRU
	maxSize     int
	mu          sync.RWMutex // protects concurrent access from parallel tool goroutines

	// Reverse index: path -> set of cache keys (for O(1) invalidation by file path)
	pathIndex map[string]map[string]bool
}

// cacheEntry is stored in the doubly-linked list.
type cacheEntry struct {
	key    string
	result string
}

func newToolResultCache() *toolResultCache {
	return &toolResultCache{
		entries:     make(map[string]*list.Element),
		accessOrder: list.New(),
		maxSize:     toolResultCacheMax,
		pathIndex:   make(map[string]map[string]bool),
	}
}

// cacheKey builds a deterministic cache key from skill name and input params.
// O(k) using insertion sort for small key counts (typically < 5 keys).
func (c *toolResultCache) cacheKey(skillName string, input map[string]interface{}) string {
	// Collect keys, skip injected params
	var keys []string
	for k := range input {
		if k == "project_id" || k == "user_id" {
			continue
		}
		keys = append(keys, k)
	}
	// Insertion sort — O(k²) worst case but k is typically 1-3, so effectively O(k)
	for i := 1; i < len(keys); i++ {
		key := keys[i]
		j := i - 1
		for j >= 0 && keys[j] > key {
			keys[j+1] = keys[j]
			j--
		}
		keys[j+1] = key
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

// extractPath extracts a file path from a cache key for reverse indexing.
// O(k) where k = key length.
func extractPath(key string) string {
	// Keys like "read_file|path=/foo/bar.go" or "grep_search|pattern=x|project_id=y"
	// Extract path value after "path=" prefix
	idx := strings.Index(key, "path=")
	if idx < 0 {
		return ""
	}
	val := key[idx+5:]
	// Stop at next pipe or end
	if pipeIdx := strings.Index(val, "|"); pipeIdx >= 0 {
		val = val[:pipeIdx]
	}
	return val
}

// addToPathIndex adds a key to the reverse path index. O(1).
func (c *toolResultCache) addToPathIndex(key string) {
	path := extractPath(key)
	if path == "" {
		return
	}
	if c.pathIndex[path] == nil {
		c.pathIndex[path] = make(map[string]bool)
	}
	c.pathIndex[path][key] = true
}

// removeFromPathIndex removes a key from the reverse path index. O(1).
func (c *toolResultCache) removeFromPathIndex(key string) {
	path := extractPath(key)
	if path == "" {
		return
	}
	if keys, ok := c.pathIndex[path]; ok {
		delete(keys, key)
		if len(keys) == 0 {
			delete(c.pathIndex, path)
		}
	}
}

// get returns cached result if available, empty string otherwise.
// O(1): map lookup + list move-to-end.
func (c *toolResultCache) get(skillName string, input map[string]interface{}) string {
	if !c.isCacheable(skillName) {
		return ""
	}
	key := c.cacheKey(skillName, input)
	c.mu.Lock()
	elem, ok := c.entries[key]
	if ok {
		// O(1): move to back of list (most recently used)
		c.accessOrder.MoveToFront(elem)
		log.Printf("[ToolCache] HIT: %s", skillName)
		result := elem.Value.(*cacheEntry).result
		c.mu.Unlock()
		return result
	}
	c.mu.Unlock()
	return ""
}

// Put stores a result in the cache (LRU eviction).
// O(1): map insert + list append + optional eviction.
func (c *toolResultCache) put(skillName string, input map[string]interface{}, result string) {
	if !c.isCacheable(skillName) {
		return
	}
	// Don't cache error results
	if strings.HasPrefix(result, "Error:") || strings.HasPrefix(result, "❌") {
		return
	}
	// Truncate large results to cap per-entry memory usage.
	if len(result) > cacheMaxEntrySize {
		result = result[:cacheMaxEntrySize] + "\n...[cached summary — full result available via read_file]"
	}
	key := c.cacheKey(skillName, input)
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing entry — move to front (most recently used)
	if elem, exists := c.entries[key]; exists {
		elem.Value.(*cacheEntry).result = result
		c.accessOrder.MoveToFront(elem)
		log.Printf("[ToolCache] UPDATE: %s (total=%d)", skillName, len(c.entries))
		return
	}

	// Evict LRU (front of list) if full — O(1)
	if len(c.entries) >= c.maxSize {
		lruElem := c.accessOrder.Front()
		if lruElem != nil {
			lruEntry := lruElem.Value.(*cacheEntry)
			delete(c.entries, lruEntry.key)
			c.removeFromPathIndex(lruEntry.key)
			c.accessOrder.Remove(lruElem)
		}
	}

	// Insert new entry at back (most recently used) — O(1)
	entry := &cacheEntry{key: key, result: result}
	elem := c.accessOrder.PushBack(entry)
	c.entries[key] = elem
	c.addToPathIndex(key)
	log.Printf("[ToolCache] PUT: %s (total=%d)", skillName, len(c.entries))
}

// invalidate clears cache entries matching a file path (called after write_file).
// O(k) where k = number of keys containing that path (typically 1-2, effectively O(1)).
func (c *toolResultCache) invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// O(1) lookup via reverse index
	keys, ok := c.pathIndex[path]
	if !ok || len(keys) == 0 {
		return
	}

	// Copy keys to avoid modifying map during iteration
	toRemove := make([]string, 0, len(keys))
	for key := range keys {
		toRemove = append(toRemove, key)
	}

	removed := 0
	for _, key := range toRemove {
		if elem, exists := c.entries[key]; exists {
			c.accessOrder.Remove(elem)
			delete(c.entries, key)
			delete(c.pathIndex[path], key)
			removed++
		}
	}
	if len(c.pathIndex[path]) == 0 {
		delete(c.pathIndex, path)
	}
	if removed > 0 {
		log.Printf("[ToolCache] invalidated %d entries for path=%s", removed, path)
	}
}

// InvalidateBuild clears the build_module cache entry for a given project_id.
// O(1) via reverse index lookup.
func (c *toolResultCache) InvalidateBuild(projectID string) {
	if projectID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// Build keys contain "build_module" and project_id — scan pathIndex for matches
	// Since build_module keys don't have a "path=" field, we scan the entries map
	// This is O(n) but n is bounded by toolResultCacheMax (200) and only called on writes
	removed := 0
	for key, elem := range c.entries {
		if strings.Contains(key, "build_module") && strings.Contains(key, projectID) {
			c.accessOrder.Remove(elem)
			c.removeFromPathIndex(key)
			delete(c.entries, key)
			removed++
		}
	}
	if removed > 0 {
		log.Printf("[ToolCache] invalidated %d build_module entries for project=%s", removed, projectID)
	}
}

// ═══════════════════════════════════════════════════════════════════
// File Hash Cache — SHA256-based file change detection
//
// Tracks file content hashes for write_file change detection.
// read_file updates the hash but always returns content (LLM needs it to edit).
// write_file uses the hash to skip writes when content hasn't changed.
// All operations are O(1) via hash map.
// ═══════════════════════════════════════════════════════════════════

type fileHashCache struct {
	hashes map[string]string // path -> sha256 hex (O(1) lookup)
	mu     sync.RWMutex
}

// NewFileHashCache creates a new file hash cache.
func NewFileHashCache() *fileHashCache {
	return &fileHashCache{
		hashes: make(map[string]string),
	}
}

// Get returns the cached hash for a path, or empty string if not cached. O(1).
func (c *fileHashCache) Get(path string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hashes[path]
}

// Set stores a hash for a path. O(1).
func (c *fileHashCache) Set(path, hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hashes[path] = hash
}

// Invalidate removes the cached hash for a path. O(1).
func (c *fileHashCache) Invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.hashes, path)
}
