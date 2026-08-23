package builder

import (
	"fmt"
	"time"
)

// ChangeType describes the type of file change.
type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeModified ChangeType = "modified"
	ChangeDeleted  ChangeType = "deleted"
)

// FileChange represents a single file change.
type FileChange struct {
	Path       string     `json:"path"`
	ChangeType ChangeType `json:"change_type"`
	OldContent string     `json:"old_content,omitempty"`
	NewContent string     `json:"new_content,omitempty"`
}

// IncrementalBuilder supports incremental builds based on file changes.
type IncrementalBuilder struct{}

// NewIncrementalBuilder creates a new incremental builder.
func NewIncrementalBuilder() *IncrementalBuilder {
	return &IncrementalBuilder{}
}

// DetectChanges compares old and new file sets, returning the list of changes.
func (ib *IncrementalBuilder) DetectChanges(oldFiles, newFiles map[string]string) []FileChange {
	var changes []FileChange

	// Detect modified and deleted files
	for path, oldContent := range oldFiles {
		newContent, exists := newFiles[path]
		if !exists {
			changes = append(changes, FileChange{
				Path:       path,
				ChangeType: ChangeDeleted,
				OldContent: oldContent,
			})
		} else if oldContent != newContent {
			changes = append(changes, FileChange{
				Path:       path,
				ChangeType: ChangeModified,
				OldContent: oldContent,
				NewContent: newContent,
			})
		}
	}

	// Detect added files
	for path, newContent := range newFiles {
		if _, exists := oldFiles[path]; !exists {
			changes = append(changes, FileChange{
				Path:       path,
				ChangeType: ChangeAdded,
				NewContent: newContent,
			})
		}
	}

	return changes
}

// GetAffectedFiles returns all files affected by a change, considering dependencies.
func (ib *IncrementalBuilder) GetAffectedFiles(change FileChange, dependencyGraph map[string][]string) []string {
	affected := make(map[string]bool)
	affected[change.Path] = true

	// BFS to find all dependents (files that depend on the changed file)
	queue := []string{change.Path}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Find all files that depend on current
		for path, deps := range dependencyGraph {
			if affected[path] {
				continue
			}
			for _, dep := range deps {
				if dep == current {
					affected[path] = true
					queue = append(queue, path)
					break
				}
			}
		}
	}

	var result []string
	for path := range affected {
		result = append(result, path)
	}
	return result
}

// BuildIncremental performs an incremental build using only changed files.
func (ib *IncrementalBuilder) BuildIncremental(
	changes []FileChange,
	builder func(files map[string]string) (BuildResult, error),
) (BuildResult, error) {
	if len(changes) == 0 {
		return BuildResult{
			Success:  true,
			Stdout:   "no changes detected, skipping build",
			Duration: 0,
		}, nil
	}

	// Build file set from changes (only additions and modifications)
	files := make(map[string]string)
	for _, change := range changes {
		if change.ChangeType != ChangeDeleted {
			files[change.Path] = change.NewContent
		}
	}

	start := time.Now()
	result, err := builder(files)
	result.Duration = time.Since(start)

	if err != nil {
		result.Success = false
		result.Stderr = err.Error()
	}

	return result, nil
}

// FilterChangesByType returns changes of a specific type.
func FilterChangesByType(changes []FileChange, changeType ChangeType) []FileChange {
	var filtered []FileChange
	for _, c := range changes {
		if c.ChangeType == changeType {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// HasGoChanges checks if any changes affect Go files.
func HasGoChanges(changes []FileChange) bool {
	for _, c := range changes {
		if len(c.Path) > 3 && c.Path[len(c.Path)-3:] == ".go" {
			return true
		}
	}
	return false
}

// HasShellChanges checks if any changes affect Shell files.
func HasShellChanges(changes []FileChange) bool {
	for _, c := range changes {
		if len(c.Path) > 3 && c.Path[len(c.Path)-3:] == ".sh" {
			return true
		}
	}
	return false
}

// ChangeSummary returns a human-readable summary of changes.
func ChangeSummary(changes []FileChange) string {
	if len(changes) == 0 {
		return "no changes"
	}

	added := 0
	modified := 0
	deleted := 0
	for _, c := range changes {
		switch c.ChangeType {
		case ChangeAdded:
			added++
		case ChangeModified:
			modified++
		case ChangeDeleted:
			deleted++
		}
	}

	parts := []string{}
	if added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", added))
	}
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", modified))
	}
	if deleted > 0 {
		parts = append(parts, fmt.Sprintf("%d deleted", deleted))
	}

	return join(parts, ", ")
}

// IncrementalResult holds the result of an incremental build analysis.
type IncrementalResult struct {
	ChangedFiles []string        `json:"changed_files"`
	SkippedFiles []string        `json:"skipped_files"`
	NeedsRebuild bool            `json:"needs_rebuild"`
	Reason       string          `json:"reason,omitempty"`
	ChangedDirs  map[string]bool `json:"-"`
}

// CheckIncremental determines if an incremental build is possible.
func CheckIncremental(projectDir string, arch string) *IncrementalResult {
	// For now, always do full build — incremental requires proper file tracking
	return &IncrementalResult{
		NeedsRebuild: true,
		Reason:       "full rebuild required",
	}
}

// UpdateBuildCacheAfterBuild updates the build cache after a successful build.
func UpdateBuildCacheAfterBuild(projectDir string, arch string, target string) error {
	// Cache update handled by BuildCache — no-op for now
	return nil
}
