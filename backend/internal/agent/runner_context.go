package agent

import (
	"log"
	"os"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════
// Context window management — session caches and write-content cache
// ═══════════════════════════════════════════════════════════════════

// getSessionCache returns (or creates) a session-scoped tool result cache.
func (r *AgentRunner) getSessionCache(sessionID string) *toolResultCache {
	if sessionID == "" {
		return newToolResultCache()
	}
	// Track access time for TTL-based cleanup
	r.sessionAccessTimes.Store(sessionID, time.Now())
	if cached, ok := r.sessionCaches.Load(sessionID); ok {
		return cached.(*toolResultCache)
	}
	cache := newToolResultCache()
	r.sessionCaches.Store(sessionID, cache)
	return cache
}

// getOrCreateProgressTracker returns (or creates) a session-scoped progress tracker.
func (r *AgentRunner) getOrCreateProgressTracker(sessionID string) *ProgressTracker {
	if sessionID == "" {
		return NewProgressTracker()
	}
	if cached, ok := r.progressTrackers.Load(sessionID); ok {
		return cached.(*ProgressTracker)
	}
	tracker := NewProgressTracker()
	r.progressTrackers.Store(sessionID, tracker)
	return tracker
}

// ═══════════════════════════════════════════════════════════════════
// Optimization 30: Write-through cache with TTL (5 minutes)
// Prevents stale data when files are modified externally or by other processes.
// P0-1: Now tracks file mtime for external modification detection.
// ═══════════════════════════════════════════════════════════════════

type cachedContentWithMtime struct {
	content   string
	expiresAt time.Time
	mtime     time.Time // file modification time when cached
}

const writeContentCacheTTL = 5 * time.Minute

// cacheWriteContent stores the content of a successful write_file call.
// When read_file is called for the same path immediately after, it returns
// this cached content instead of re-reading from disk — saving one full I/O round.
// P0-1: Now tracks file modification time for invalidation.
func (r *AgentRunner) cacheWriteContent(sessionID, path, content string) {
	if sessionID == "" {
		return
	}
	val, _ := r.writeContentCache.LoadOrStore(sessionID, &sync.Map{})
	m := val.(*sync.Map)
	// Get current file mtime for invalidation
	var mtime time.Time
	if info, err := os.Stat(path); err == nil {
		mtime = info.ModTime()
	} else {
		mtime = time.Now()
	}
	m.Store(path, cachedContentWithMtime{
		content:   content,
		expiresAt: time.Now().Add(writeContentCacheTTL),
		mtime:     mtime,
	})
	debugLog("writeContentCache PUT: session=%s path=%s len=%d mtime=%v", sessionID, path, len(content), mtime)
}

// getCachedWriteContent returns the cached content for a path, or "" if not cached or expired.
// P0-1: Also checks if file was modified externally (mtime mismatch).
func (r *AgentRunner) getCachedWriteContent(sessionID, path string) string {
	if sessionID == "" {
		return ""
	}
	// Track access time for TTL-based cleanup
	r.sessionAccessTimes.Store(sessionID, time.Now())
	val, ok := r.writeContentCache.Load(sessionID)
	if !ok {
		return ""
	}
	m := val.(*sync.Map)
	if cached, ok := m.Load(path); ok {
		cc := cached.(cachedContentWithMtime)
		if time.Now().After(cc.expiresAt) {
			// Entry expired — remove it and return empty
			m.Delete(path)
			debugLog("writeContentCache EXPIRED: session=%s path=%s", sessionID, path)
			return ""
		}
		// P0-1: Check if file was modified externally
		if info, err := os.Stat(path); err == nil {
			if info.ModTime().After(cc.mtime) {
				// File was modified after we cached it — invalidate cache
				m.Delete(path)
				debugLog("writeContentCache INVALIDATED (mtime changed): session=%s path=%s cacheMtime=%v fileMtime=%v",
					sessionID, path, cc.mtime, info.ModTime())
				return ""
			}
		}
		debugLog("writeContentCache HIT: session=%s path=%s", sessionID, path)
		return cc.content
	}
	return ""
}

// cacheReadFile stores the content of a successful read_file call for reuse.
func (r *AgentRunner) cacheReadFile(sessionID, path, content string) {
	if sessionID == "" {
		return
	}
	val, _ := r.readFileCache.LoadOrStore(sessionID, &sync.Map{})
	m := val.(*sync.Map)
	m.Store(path, content)
}

// getCachedReadFile returns cached content for a path, or "" if not cached.
func (r *AgentRunner) getCachedReadFile(sessionID, path string) string {
	if sessionID == "" {
		return ""
	}
	val, ok := r.readFileCache.Load(sessionID)
	if !ok {
		return ""
	}
	m := val.(*sync.Map)
	if content, ok := m.Load(path); ok {
		return content.(string)
	}
	return ""
}

// startSessionCacheCleanup runs a background goroutine that periodically evicts
// expired session caches to prevent memory leaks. Sessions that haven't been
// accessed in 30 minutes are removed from sessionCaches, writeContentCache,
// and sessionAccessTimes.
func (r *AgentRunner) startSessionCacheCleanup() {
	const (
		cleanupInterval = 5 * time.Minute
		sessionTTL      = 30 * time.Minute
	)
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	lastMemoryCleanup := time.Now()
	lastVacuum := time.Now()
	for range ticker.C {
		now := time.Now()
		expired := make([]string, 0)
		r.sessionAccessTimes.Range(func(key, value interface{}) bool {
			lastAccess := value.(time.Time)
			if now.Sub(lastAccess) > sessionTTL {
				expired = append(expired, key.(string))
			}
			return true
		})
		for _, sid := range expired {
			r.sessionCaches.Delete(sid)
			r.writeContentCache.Delete(sid)
			r.readFileCache.Delete(sid)
			r.sessionAccessTimes.Delete(sid)
			debugLog("session cache TTL expired: session=%s", sid)
		}
		if len(expired) > 0 {
			log.Printf("[Agent] session cache cleanup: evicted %d expired sessions", len(expired))
		}

		// Daily memory_v2 cleanup: delete expired entries.
		// Runs at most once per day to avoid repeated DB writes.
		if now.Sub(lastMemoryCleanup) > 24*time.Hour {
			lastMemoryCleanup = now
			if r.db != nil {
				if res, err := r.db.Exec("DELETE FROM memory_v2 WHERE expires_at IS NOT NULL AND expires_at < datetime('now')"); err == nil {
					if n, _ := res.RowsAffected(); n > 0 {
						log.Printf("[Agent] memory cleanup: deleted %d expired entries", n)
					}
				}
			}
		}

		// DifferentialCache cleanup: evict expired file content entries.
		if r.diffCache != nil {
			r.diffCache.Cleanup()
		}

		// Weekly SQLite VACUUM to reclaim space from deleted rows.
		// Runs on the same daily tick but only once a week (7*24h).
		if now.Sub(lastVacuum) > 7*24*time.Hour {
			lastVacuum = now
			if r.db != nil {
				if _, err := r.db.Exec("VACUUM"); err != nil {
					log.Printf("[Agent] VACUUM failed: %v", err)
				} else {
					log.Printf("[Agent] VACUUM completed")
				}
			}
		}
	}
}
