package builder

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	binaryCacheDir  = ".build_cache"
	cacheExpireDays = 7
)

// BinaryCacheEntry represents a cached compiled binary.
type BinaryCacheEntry struct {
	SourceHash  string `json:"source_hash"`
	BinaryPath  string `json:"binary_path"`
	Size        int64  `json:"size"`
	CreatedAt   string `json:"created_at"`
	LastUsedAt  string `json:"last_used_at"`
	HitCount    int    `json:"hit_count"`
}

// BinaryCacheIndex tracks all cached binaries for a project.
type BinaryCacheIndex struct {
	Entries map[string]BinaryCacheEntry `json:"entries"` // key = source file hash
}

// BinaryCacheStatus provides cache statistics for the API.
type BinaryCacheStatus struct {
	TotalSize   int64  `json:"total_size"`
	EntryCount  int    `json:"entry_count"`
	HitRate     float64 `json:"hit_rate"`
	Hits        int    `json:"hits"`
	Misses      int    `json:"misses"`
	OldestEntry string `json:"oldest_entry"`
	NewestEntry string `json:"newest_entry"`
}

func getBinaryCacheDir(projectDir string) string {
	return filepath.Join(projectDir, binaryCacheDir)
}

func getBinaryCacheIndexPath(projectDir string) string {
	return filepath.Join(getBinaryCacheDir(projectDir), "index.json")
}

func loadBinaryCacheIndex(projectDir string) *BinaryCacheIndex {
	path := getBinaryCacheIndexPath(projectDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return &BinaryCacheIndex{Entries: make(map[string]BinaryCacheEntry)}
	}
	var index BinaryCacheIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return &BinaryCacheIndex{Entries: make(map[string]BinaryCacheEntry)}
	}
	if index.Entries == nil {
		index.Entries = make(map[string]BinaryCacheEntry)
	}
	return &index
}

func saveBinaryCacheIndex(projectDir string, index *BinaryCacheIndex) error {
	dir := getBinaryCacheDir(projectDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache index: %w", err)
	}
	return os.WriteFile(getBinaryCacheIndexPath(projectDir), data, 0644)
}

// SourceHash computes a hash of a source file for cache lookup.
func SourceHash(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h), nil
}

// CheckBinaryCache checks if a compiled binary exists in cache.
// Returns the path to the cached binary if found, nil otherwise.
func CheckBinaryCache(projectDir, sourcePath string) *string {
	index := loadBinaryCacheIndex(projectDir)
	hash, err := SourceHash(sourcePath)
	if err != nil {
		return nil
	}

	entry, exists := index.Entries[hash]
	if !exists {
		return nil
	}

	// Check file still exists on disk
	if _, err := os.Stat(entry.BinaryPath); os.IsNotExist(err) {
		delete(index.Entries, hash)
		saveBinaryCacheIndex(projectDir, index)
		return nil
	}

	// Update usage stats
	entry.LastUsedAt = time.Now().Format(time.RFC3339)
	entry.HitCount++
	index.Entries[hash] = entry
	saveBinaryCacheIndex(projectDir, index)

	return &entry.BinaryPath
}

// StoreBinaryCache saves a compiled binary to the cache.
func StoreBinaryCache(projectDir, sourcePath, binaryPath string) error {
	index := loadBinaryCacheIndex(projectDir)

	hash, err := SourceHash(sourcePath)
	if err != nil {
		return err
	}

	info, err := os.Stat(binaryPath)
	if err != nil {
		return err
	}

	// Copy binary to cache directory
	cacheDir := getBinaryCacheDir(projectDir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	cachedBinaryPath := filepath.Join(cacheDir, hash[:12]+"_"+filepath.Base(binaryPath))
	input, err := os.ReadFile(binaryPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(cachedBinaryPath, input, 0755); err != nil {
		return err
	}

	index.Entries[hash] = BinaryCacheEntry{
		SourceHash: hash,
		BinaryPath: cachedBinaryPath,
		Size:       info.Size(),
		CreatedAt:  time.Now().Format(time.RFC3339),
		LastUsedAt: time.Now().Format(time.RFC3339),
		HitCount:   0,
	}

	return saveBinaryCacheIndex(projectDir, index)
}

// CleanupExpiredCache removes cache entries older than cacheExpireDays.
func CleanupExpiredCache(projectDir string) (int, error) {
	index := loadBinaryCacheIndex(projectDir)
	cutoff := time.Now().AddDate(0, 0, -cacheExpireDays)
	removed := 0

	for hash, entry := range index.Entries {
		createdAt, err := time.Parse(time.RFC3339, entry.CreatedAt)
		if err != nil {
			continue
		}
		if createdAt.Before(cutoff) {
			// Remove cached binary file
			os.Remove(entry.BinaryPath)
			delete(index.Entries, hash)
			removed++
		}
	}

	if removed > 0 {
		saveBinaryCacheIndex(projectDir, index)
	}

	return removed, nil
}

// GetBinaryCacheStatus returns statistics about the binary cache.
func GetBinaryCacheStatus(projectDir string) *BinaryCacheStatus {
	index := loadBinaryCacheIndex(projectDir)
	status := &BinaryCacheStatus{}

	var totalHits, totalMisses int
	for _, entry := range index.Entries {
		status.TotalSize += entry.Size
		status.EntryCount++
		totalHits += entry.HitCount

		if status.OldestEntry == "" || entry.CreatedAt < status.OldestEntry {
			status.OldestEntry = entry.CreatedAt
		}
		if status.NewestEntry == "" || entry.CreatedAt > status.NewestEntry {
			status.NewestEntry = entry.CreatedAt
		}
	}

	// Estimate misses (rebuilds without cache)
	totalMisses = status.EntryCount // Each entry was a miss once
	if totalHits+totalMisses > 0 {
		status.HitRate = float64(totalHits) / float64(totalHits+totalMisses) * 100
	}
	status.Hits = totalHits
	status.Misses = totalMisses

	return status
}

// ClearBinaryCache removes all cached binaries for a project.
func ClearBinaryCache(projectDir string) error {
	cacheDir := getBinaryCacheDir(projectDir)
	return os.RemoveAll(cacheDir)
}
