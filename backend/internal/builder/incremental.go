package builder

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileMtimeRecord tracks the modification time and hash of a source file
// for incremental compilation decisions.
type FileMtimeRecord struct {
	Path    string `json:"path"`
	Mtime   int64  `json:"mtime"`
	Hash    string `json:"hash"`
	Binary  string `json:"binary,omitempty"` // path to compiled binary
	Size    int64  `json:"size"`
}

// BuildCacheJSON is the structure stored in .build_cache.json at the project root.
type BuildCacheJSON struct {
	Files       map[string]FileMtimeRecord `json:"files"`
	BuildTime   string                     `json:"build_time"`
	Arch        string                     `json:"arch"`
	Target      string                     `json:"target"`
	FullRebuild bool                       `json:"full_rebuild"`
}

// loadBuildCache reads .build_cache.json from the project directory.
func loadBuildCache(projectDir string) *BuildCacheJSON {
	cachePath := filepath.Join(projectDir, ".build_cache.json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return &BuildCacheJSON{Files: make(map[string]FileMtimeRecord)}
	}
	var cache BuildCacheJSON
	if err := json.Unmarshal(data, &cache); err != nil {
		return &BuildCacheJSON{Files: make(map[string]FileMtimeRecord)}
	}
	if cache.Files == nil {
		cache.Files = make(map[string]FileMtimeRecord)
	}
	return &cache
}

// saveBuildCache writes the build cache to .build_cache.json.
func saveBuildCache(projectDir string, cache *BuildCacheJSON) error {
	cachePath := filepath.Join(projectDir, ".build_cache.json")
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, data, 0644)
}

// fileHash computes SHA-256 of a file's content.
func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h), nil
}

// fileMtime returns the modification time of a file as unix nanoseconds.
func fileMtime(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.ModTime().UnixNano(), nil
}

// IncrementalResult holds the outcome of an incremental compilation check.
type IncrementalResult struct {
	NeedsRebuild   bool     `json:"needs_rebuild"`   // true if full rebuild needed
	ChangedFiles   []string `json:"changed_files"`    // files that changed since last build
	UnchangedFiles []string `json:"unchanged_files"`  // files unchanged (will be skipped)
	NewFiles       []string `json:"new_files"`        // files not seen before
	RemovedFiles   []string `json:"removed_files"`    // files that were deleted
	Reason         string   `json:"reason"`           // why full rebuild was triggered
}

// CheckIncremental compares current project files against the build cache.
// Returns which files changed and whether a full rebuild is needed.
// If any source file in the same compilation unit changed, all files in that
// unit must be recompiled (dependency trigger).
func CheckIncremental(projectDir string, arch string) *IncrementalResult {
	cache := loadBuildCache(projectDir)

	// Collect all source files
	var sourceFiles []string
	extensions := []string{".go", ".rs", ".toml"}
	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// Skip hidden dirs, build artifacts, vendor
		rel, _ := filepath.Rel(projectDir, path)
		for _, skip := range []string{".git", ".build_cache", "vendor", "target", "node_modules"} {
			if len(rel) > len(skip)+1 && (rel[:len(skip)+1] == skip+string(os.PathSeparator) || rel == skip) {
				return nil
			}
		}
		for _, ext := range extensions {
			if filepath.Ext(path) == ext {
				sourceFiles = append(sourceFiles, path)
				break
			}
		}
		return nil
	})

	result := &IncrementalResult{}

	// No previous cache -> full rebuild needed
	if cache.BuildTime == "" || len(cache.Files) == 0 {
		result.NeedsRebuild = true
		result.Reason = "no previous build cache"
		result.ChangedFiles = sourceFiles
		return result
	}

	// Architecture change -> full rebuild
	if cache.Arch != arch {
		result.NeedsRebuild = true
		result.Reason = fmt.Sprintf("architecture changed: %s -> %s", cache.Arch, arch)
		result.ChangedFiles = sourceFiles
		return result
	}

	// Build a set of current files
	currentSet := make(map[string]bool)
	for _, f := range sourceFiles {
		currentSet[f] = true
	}

	// Check for removed files
	for cachedPath := range cache.Files {
		if !currentSet[cachedPath] {
			result.RemovedFiles = append(result.RemovedFiles, cachedPath)
		}
	}

	// Check each file for changes
	changedDirs := make(map[string]bool)
	for _, f := range sourceFiles {
		record, exists := cache.Files[f]

		if !exists {
			// New file
			result.NewFiles = append(result.NewFiles, f)
			changedDirs[filepath.Dir(f)] = true
			continue
		}

		mtime, err := fileMtime(f)
		if err != nil {
			continue
		}

		// Fast check: mtime unchanged -> file not modified
		if mtime == record.Mtime {
			result.UnchangedFiles = append(result.UnchangedFiles, f)
			continue
		}

		// mtime changed -> verify with hash
		hash, err := fileHash(f)
		if err != nil {
			continue
		}

		if hash != record.Hash {
			result.ChangedFiles = append(result.ChangedFiles, f)
			changedDirs[filepath.Dir(f)] = true
		} else {
			// mtime changed but content same (e.g. touch)
			result.UnchangedFiles = append(result.UnchangedFiles, f)
		}
	}

	// If any file in a compilation directory changed, mark ALL files in that
	// directory as changed (dependency trigger - Go/Rust recompile entire package)
	if len(result.ChangedFiles) > 0 {
		changedDirSet := make(map[string]bool)
		for _, f := range result.ChangedFiles {
			changedDirSet[filepath.Dir(f)] = true
		}

		// Expand: all files in changed dirs are considered changed
		var expandedChanged []string
		for _, f := range result.ChangedFiles {
			expandedChanged = append(expandedChanged, f)
		}
		seen := make(map[string]bool)
		for _, f := range result.ChangedFiles {
			seen[f] = true
		}
		for _, f := range result.UnchangedFiles {
			if changedDirSet[filepath.Dir(f)] && !seen[f] {
				expandedChanged = append(expandedChanged, f)
				seen[f] = true
			}
		}
		result.ChangedFiles = expandedChanged
	}

	result.NeedsRebuild = len(result.ChangedFiles) > 0 || len(result.NewFiles) > 0 || len(result.RemovedFiles) > 0
	if result.NeedsRebuild {
		result.Reason = fmt.Sprintf("%d changed, %d new, %d removed", len(result.ChangedFiles), len(result.NewFiles), len(result.RemovedFiles))
	}

	return result
}

// UpdateBuildCacheAfterBuild updates .build_cache.json after a successful build.
func UpdateBuildCacheAfterBuild(projectDir, arch, target string) error {
	cache := &BuildCacheJSON{
		Files:     make(map[string]FileMtimeRecord),
		BuildTime: time.Now().Format(time.RFC3339),
		Arch:      arch,
		Target:    target,
	}

	// Walk source files and record their state
	extensions := []string{".go", ".rs", ".toml"}
	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(projectDir, path)
		for _, skip := range []string{".git", ".build_cache", "vendor", "target", "node_modules"} {
			if len(rel) > len(skip)+1 && (rel[:len(skip)+1] == skip+string(os.PathSeparator) || rel == skip) {
				return nil
			}
		}
		for _, ext := range extensions {
			if filepath.Ext(path) == ext {
				hash, err := fileHash(path)
				if err != nil {
					return nil
				}
				cache.Files[path] = FileMtimeRecord{
					Path:  path,
					Mtime: info.ModTime().UnixNano(),
					Hash:  hash,
					Size:  info.Size(),
				}
				break
			}
		}
		return nil
	})

	return saveBuildCache(projectDir, cache)
}

// ClearBuildCacheJSON removes the .build_cache.json file from the project directory.
func ClearBuildCacheJSON(projectDir string) error {
	return os.Remove(filepath.Join(projectDir, ".build_cache.json"))
}
