package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *BuildService) checkBuildCache(projectID, filesHash string) *string {
	cachePath := s.getCacheKey(projectID, filesHash)
	info, err := os.Stat(cachePath)
	if err != nil {
		return nil
	}
	if time.Since(info.ModTime()) > 24*time.Hour {
		os.Remove(cachePath)
		return nil
	}
	return &cachePath
}

func (s *BuildService) saveBuildCache(projectID, filesHash, outputPath string) error {
	cachePath := s.getCacheKey(projectID, filesHash)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return err
	}
	input, err := os.ReadFile(outputPath)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, input, 0644)
}

func (s *BuildService) ClearBuildCache(ctx context.Context, projectID string) error {
	projectID = safeID(projectID)

	// 清除文件系统缓存
	cacheDir := filepath.Join(s.cfg.StoragePath, "build-cache", projectID)
	if err := os.RemoveAll(cacheDir); err != nil {
		return err
	}

	// 清除数据库中的缓存命中记录（将log中的缓存命中标记清除）
	_, err := s.db.ExecContext(ctx,
		`UPDATE build_tasks SET log = REPLACE(REPLACE(log, '[CACHE] 缓存命中，使用缓存产物', ''), 'cache hit', '') 
		 WHERE project_id = ? AND (log LIKE '%cache hit%' OR log LIKE '%缓存命中%')`,
		projectID)
	if err != nil {
		// 忽略数据库错误，文件缓存已清除
		log.Printf("[BuildCache] Warning: failed to clear cache hits in DB: %v", err)
	}

	return nil
}

// GetBuildCacheStatus returns cache statistics for a project.
func (s *BuildService) GetBuildCacheStatus(ctx context.Context, projectID string) (map[string]interface{}, error) {
	projectID = safeID(projectID)
	cacheDir := filepath.Join(s.cfg.StoragePath, "build-cache", projectID)

	var totalSize int64
	var fileCount int

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return map[string]interface{}{
			"total_size": 0,
			"file_count": 0,
			"hit_rate":   0,
		}, nil
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		totalSize += info.Size()
		fileCount++
	}

	// Get build history for hit rate calculation
	var totalBuilds, cacheHits int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(CASE WHEN log LIKE '%cache hit%' OR log LIKE '%缓存命中%' THEN 1 ELSE 0 END), 0)
		 FROM build_tasks WHERE project_id=?`, projectID).Scan(&totalBuilds, &cacheHits)
	if err != nil {
		totalBuilds = 0
		cacheHits = 0
	}

	hitRate := 0.0
	if totalBuilds > 0 {
		hitRate = float64(cacheHits) / float64(totalBuilds) * 100
	}

	return map[string]interface{}{
		"total_size":   totalSize,
		"file_count":   fileCount,
		"hit_rate":     hitRate,
		"total_builds": totalBuilds,
		"cache_hits":   cacheHits,
	}, nil
}

// ClearAllBuildCaches removes build caches for ALL projects.
func (s *BuildService) ClearAllBuildCaches() error {
	buildCacheDir := filepath.Join(s.cfg.StoragePath, "build-cache")
	entries, err := os.ReadDir(buildCacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			os.RemoveAll(filepath.Join(buildCacheDir, entry.Name()))
		}
	}
	return nil
}

// ========================
// Per-source-file binary cache (SHA256-based)
// ========================

const binaryCacheTTL = 7 * 24 * time.Hour

// GetBinaryCacheKey returns the cache path for a compiled result.
// Format: <StoragePath>/binary-cache/<projectID>/<lang>/<arch>/<fileHash>.bin
func (s *BuildService) GetBinaryCacheKey(projectID, lang, arch, fileHash string) string {
	projectID = safeID(projectID)
	return filepath.Join(s.cfg.StoragePath, "binary-cache", projectID, lang, arch, fileHash+".bin")
}

// CheckSourceFileCache checks if a compiler binary is cached for the given source file.
// Returns the cached binary path if valid (within TTL), otherwise "".
func (s *BuildService) CheckSourceFileCache(projectID, lang, arch, sourcePath string) (string, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("read source file: %w", err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	cachePath := s.GetBinaryCacheKey(projectID, lang, arch, hash)

	info, err := os.Stat(cachePath)
	if err != nil {
		return "", nil // no cache entry
	}

	if time.Since(info.ModTime()) > binaryCacheTTL {
		_ = os.Remove(cachePath)
		return "", nil // expired
	}
	return cachePath, nil
}

// SaveSourceFileCache stores a compiled binary keyed by source file SHA256.
func (s *BuildService) SaveSourceFileCache(projectID, lang, arch, srcPath, binPath string) error {
	srcData, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read source for cache key: %w", err)
	}
	binData, err := os.ReadFile(binPath)
	if err != nil {
		return fmt.Errorf("read compiled binary: %w", err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(srcData))
	cachePath := s.GetBinaryCacheKey(projectID, lang, arch, hash)

	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	tmp := cachePath + ".tmp"
	if err := os.WriteFile(tmp, binData, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, cachePath)
}

// SourceFileCacheStat holds metadata about a cached compilation.
type SourceFileCacheStat struct {
	ProjectID string
	Language  string
	Arch      string
	SHA256    string
	CachePath string
	Age       time.Duration
	Size      int64
}

// ListBinaryCacheEntries returns all non-expired binary cache entries for a project.
func (s *BuildService) ListBinaryCacheEntries(ctx context.Context, projectID string) ([]SourceFileCacheStat, error) {
	projectID = safeID(projectID)
	cacheDir := filepath.Join(s.cfg.StoragePath, "binary-cache", projectID)

	var results []SourceFileCacheStat
	if _, err := os.ReadDir(cacheDir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".bin") {
			return nil
		}
		age := time.Since(info.ModTime())
		if age > binaryCacheTTL {
			os.Remove(path)
			return nil
		}
		results = append(results, SourceFileCacheStat{
			ProjectID: projectID,
			Language:  filepath.Base(filepath.Dir(filepath.Dir(path))),
			Arch:      filepath.Base(filepath.Dir(path)),
			// SHA256 is the filename without .bin suffix — kept implicit
			CachePath: path,
			Age:       age,
			Size:      info.Size(),
		})
		return nil
	})

	return results, nil
}

// CleanupExpiredBinaryCaches removes cache entries older than the configured TTL.
// Returns number of files deleted.
func (s *BuildService) CleanupExpiredBinaryCaches(projectID string) int {
	projectID = safeID(projectID)
	cacheDir := filepath.Join(s.cfg.StoragePath, "binary-cache", projectID)

	count := 0
	filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".bin") {
			return nil
		}
		if time.Since(info.ModTime()) > binaryCacheTTL {
			_ = os.Remove(path)
			count++
		}
		return nil
	})
	return count
}

// BinaryCacheInfo provides summary stats for the binary cache.
type BinaryCacheInfo struct {
	TotalEntries int   `json:"total_entries"`
	TotalSize    int64 `json:"total_size_bytes"`
	ExpiredCount int   `json:"expired_count"`
}

// GetBinaryCacheStatus returns cache statistics for the per-file binary cache.
func (s *BuildService) GetBinaryCacheStatus(ctx context.Context, projectID string) (*BinaryCacheInfo, error) {
	projectID = safeID(projectID)
	cacheDir := filepath.Join(s.cfg.StoragePath, "binary-cache", projectID)

	info := &BinaryCacheInfo{}
	if _, err := os.ReadDir(cacheDir); err != nil {
		if os.IsNotExist(err) {
			return info, nil
		}
		return nil, err
	}

	filepath.Walk(cacheDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".bin") {
			info.TotalEntries++
			info.TotalSize += fi.Size()
			if time.Since(fi.ModTime()) > binaryCacheTTL {
				info.ExpiredCount++
			}
		}
		return nil
	})

	return info, nil
}

// CopyCachedBinary copies a cached binary from the source cache path to the destination output.
func CopyCachedBinary(cachePath, outputPath string) error {
	src, err := os.Open(cachePath)
	if err != nil {
		return err
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	dst, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}
	return dst.Sync()
}

// ComputeFileSHA256 computes the SHA256 hex digest of a file.
func ComputeFileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
