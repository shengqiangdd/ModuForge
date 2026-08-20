package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	cacheDir := filepath.Join(s.cfg.StoragePath, "build-cache", projectID)
	return os.RemoveAll(cacheDir)
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
		"total_size":  totalSize,
		"file_count":  fileCount,
		"hit_rate":    hitRate,
		"total_builds": totalBuilds,
		"cache_hits":  cacheHits,
	}, nil
}
