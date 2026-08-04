package builder

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	// Global cache limits
	DefaultMaxCacheSizeMB  = 512  // 512MB total across all projects
	DefaultMaxCacheAge     = 7 * 24 * time.Hour  // 7 days
	DefaultCleanupInterval = 1 * time.Hour       // check every hour
)

// CacheConfig holds configurable cache limits.
type CacheConfig struct {
	MaxSizeMB      int64         // Maximum total cache size in MB
	MaxAge         time.Duration // Maximum age for any cache entry
	CleanupInterval time.Duration // How often to run cleanup
	StoragePath    string        // Root storage path
}

// DefaultCacheConfig returns sensible defaults.
func DefaultCacheConfig(storagePath string) *CacheConfig {
	return &CacheConfig{
		MaxSizeMB:       DefaultMaxCacheSizeMB,
		MaxAge:          DefaultMaxCacheAge,
		CleanupInterval: DefaultCleanupInterval,
		StoragePath:     storagePath,
	}
}

// GlobalCacheManager handles cross-project cache cleanup.
type GlobalCacheManager struct {
	cfg      *CacheConfig
	stopCh   chan struct{}
	running  sync.Mutex
	stopped  bool
}

// NewGlobalCacheManager creates a new cache manager.
func NewGlobalCacheManager(cfg *CacheConfig) *GlobalCacheManager {
	return &GlobalCacheManager{
		cfg:   cfg,
		stopCh: make(chan struct{}),
	}
}

// Start begins periodic cache cleanup in the background.
func (m *GlobalCacheManager) Start() {
	m.running.Lock()
	if m.stopped {
		m.running.Unlock()
		return
	}
	m.running.Unlock()

	go func() {
		slog.Info("cache cleanup started", "interval", m.cfg.CleanupInterval, "max_mb", m.cfg.MaxSizeMB)
		ticker := time.NewTicker(m.cfg.CleanupInterval)
		defer ticker.Stop()

		// Run initial cleanup
		m.RunCleanup()

		for {
			select {
			case <-ticker.C:
				m.RunCleanup()
			case <-m.stopCh:
				slog.Info("cache cleanup stopped")
				return
			}
		}
	}()
}

// Stop halts periodic cleanup.
func (m *GlobalCacheManager) Stop() {
	m.running.Lock()
	defer m.running.Unlock()
	if !m.stopped {
		close(m.stopCh)
		m.stopped = true
	}
}

// runCleanup performs one cleanup pass across all projects.
func (m *GlobalCacheManager) RunCleanup() {
	start := time.Now()
	defer func() {
		slog.Info("cache cleanup completed", "duration", time.Since(start).Round(time.Millisecond))
	}()

	storagePath := m.cfg.StoragePath

	// 1. Clean expired binary caches (per-project .build_cache/)
	m.cleanExpiredBinaryCaches(storagePath)

	// 2. Clean expired build artifacts (storage/build-cache/)
	m.cleanExpiredBuildArtifacts(storagePath)

	// 3. Clean stale build status files
	m.cleanStaleBuildStatus(storagePath)

	// 4. Enforce global size limit (LRU eviction)
	m.enforceSizeLimit(storagePath)
}

// cleanExpiredBinaryCaches removes .build_cache entries older than MaxAge.
func (m *GlobalCacheManager) cleanExpiredBinaryCaches(storagePath string) {
	projectsDir := filepath.Join(storagePath, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return
	}

	removed := 0

	for _, project := range entries {
		if !project.IsDir() {
			continue
		}
		cacheDir := filepath.Join(projectsDir, project.Name(), ".build_cache")
		if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
			continue
		}

		// Use the per-project cleanup logic
		projectDir := filepath.Join(projectsDir, project.Name())
		if n, err := CleanupExpiredCache(projectDir); err == nil {
			removed += n
		}
	}

	if removed > 0 {
		slog.Info("cleaned expired binary cache entries", "removed", removed)
	}
}

// cleanExpiredBuildArtifacts removes build-cache zips older than MaxAge.
func (m *GlobalCacheManager) cleanExpiredBuildArtifacts(storagePath string) {
	buildCacheDir := filepath.Join(storagePath, "build-cache")
	entries, err := os.ReadDir(buildCacheDir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-m.cfg.MaxAge)
	removed := 0

	for _, project := range entries {
		if !project.IsDir() {
			continue
		}
		projectCacheDir := filepath.Join(buildCacheDir, project.Name())
		zips, err := os.ReadDir(projectCacheDir)
		if err != nil {
			continue
		}

		for _, zip := range zips {
			info, err := zip.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				os.Remove(filepath.Join(projectCacheDir, zip.Name()))
				removed++
			}
		}

		// Remove empty project dirs
		if remaining, _ := os.ReadDir(projectCacheDir); len(remaining) == 0 {
			os.Remove(projectCacheDir)
		}
	}

	if removed > 0 {
		slog.Info("cleaned expired build artifacts", "removed", removed)
	}
}

// cleanStaleBuildStatus removes .build_status files older than MaxAge.
func (m *GlobalCacheManager) cleanStaleBuildStatus(storagePath string) {
	projectsDir := filepath.Join(storagePath, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-m.cfg.MaxAge)
	removed := 0

	for _, project := range entries {
		if !project.IsDir() {
			continue
		}
		statusDir := filepath.Join(projectsDir, project.Name(), ".build_status")
		if _, err := os.Stat(statusDir); os.IsNotExist(err) {
			continue
		}

		files, err := os.ReadDir(statusDir)
		if err != nil {
			continue
		}

		for _, f := range files {
			info, err := f.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				os.Remove(filepath.Join(statusDir, f.Name()))
				removed++
			}
		}

		// Remove empty status dirs
		if remaining, _ := os.ReadDir(statusDir); len(remaining) == 0 {
			os.Remove(statusDir)
		}
	}

	if removed > 0 {
		slog.Info("cleaned stale build status files", "removed", removed)
	}
}

// CacheFileInfo holds info for sorting by access time (LRU).
type CacheFileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
}

// enforceSizeLimit evicts oldest entries when total cache exceeds MaxSizeMB.
func (m *GlobalCacheManager) enforceSizeLimit(storagePath string) {
	maxBytes := m.cfg.MaxSizeMB * 1024 * 1024

	// Collect all cache files with metadata
	var allFiles []CacheFileInfo
	totalSize := int64(0)

	// Binary caches
	projectsDir := filepath.Join(storagePath, "projects")
	if entries, err := os.ReadDir(projectsDir); err == nil {
		for _, project := range entries {
			if !project.IsDir() {
				continue
			}
			cacheDir := filepath.Join(projectsDir, project.Name(), ".build_cache")
			if files, err := os.ReadDir(cacheDir); err == nil {
				for _, f := range files {
					if f.Name() == "index.json" {
						continue // skip index
					}
					info, err := f.Info()
					if err != nil {
						continue
					}
					totalSize += info.Size()
					allFiles = append(allFiles, CacheFileInfo{
						Path:    filepath.Join(cacheDir, f.Name()),
						Size:    info.Size(),
						ModTime: info.ModTime(),
					})
				}
			}
		}
	}

	// Build artifacts
	buildCacheDir := filepath.Join(storagePath, "build-cache")
	if entries, err := os.ReadDir(buildCacheDir); err == nil {
		for _, project := range entries {
			if !project.IsDir() {
				continue
			}
			projectCacheDir := filepath.Join(buildCacheDir, project.Name())
			if files, err := os.ReadDir(projectCacheDir); err == nil {
				for _, f := range files {
					info, err := f.Info()
					if err != nil {
						continue
					}
					totalSize += info.Size()
					allFiles = append(allFiles, CacheFileInfo{
						Path:    filepath.Join(projectCacheDir, f.Name()),
						Size:    info.Size(),
						ModTime: info.ModTime(),
					})
				}
			}
		}
	}

	if totalSize <= maxBytes {
		return
	}

	// Sort by ModTime ascending (oldest first) for LRU eviction
	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].ModTime.Before(allFiles[j].ModTime)
	})

	// Evict until under limit
	evicted := 0
	for _, f := range allFiles {
		if totalSize <= maxBytes {
			break
		}
		if err := os.Remove(f.Path); err == nil {
			totalSize -= f.Size
			evicted++
		}
	}

	if evicted > 0 {
		slog.Info("enforced cache size limit",
			"evicted", evicted,
			"remaining_mb", totalSize/1024/1024,
			"max_mb", m.cfg.MaxSizeMB,
		)
	}
}

// GetGlobalCacheStats returns statistics across all projects.
func GetGlobalCacheStats(storagePath string) map[string]interface{} {
	var totalSize int64
	var fileCount int
	var projectCount int

	// Binary caches
	projectsDir := filepath.Join(storagePath, "projects")
	if entries, err := os.ReadDir(projectsDir); err == nil {
		for _, project := range entries {
			if !project.IsDir() {
				continue
			}
			projectHasCache := false
			cacheDir := filepath.Join(projectsDir, project.Name(), ".build_cache")
			if files, err := os.ReadDir(cacheDir); err == nil {
				for _, f := range files {
					if f.Name() == "index.json" {
						continue
					}
					info, err := f.Info()
					if err != nil {
						continue
					}
					totalSize += info.Size()
					fileCount++
					projectHasCache = true
				}
			}
			if projectHasCache {
				projectCount++
			}
		}
	}

	// Build artifacts
	buildCacheDir := filepath.Join(storagePath, "build-cache")
	if entries, err := os.ReadDir(buildCacheDir); err == nil {
		for _, project := range entries {
			if !project.IsDir() {
				continue
			}
			projectCacheDir := filepath.Join(buildCacheDir, project.Name())
			if files, err := os.ReadDir(projectCacheDir); err == nil {
				for _, f := range files {
					info, err := f.Info()
					if err != nil {
						continue
					}
					totalSize += info.Size()
					fileCount++
				}
			}
		}
	}

	return map[string]interface{}{
		"total_size_bytes": totalSize,
		"total_size_mb":    totalSize / 1024 / 1024,
		"file_count":       fileCount,
		"project_count":    projectCount,
	}
}
