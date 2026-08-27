package skills

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/moduforge/backend/internal/agent/registry"
	"github.com/moduforge/backend/internal/storage"
)

// ApplyPatchSkill applies line-based patches to a file.
// Each patch line is "lineNumber|newContent" (1-based line numbers).
type ApplyPatchSkill struct {
	projectPath string
	db          *sql.DB
	storage     storage.StorageAdapter // optional S3 storage backend
}

func NewApplyPatchSkillWithDB(projectPath string, db *sql.DB) *ApplyPatchSkill {
	return &ApplyPatchSkill{projectPath: projectPath, db: db}
}

// WithStorage sets the S3 storage adapter.
func (s *ApplyPatchSkill) WithStorage(st storage.StorageAdapter) *ApplyPatchSkill {
	s.storage = st
	return s
}

func (s *ApplyPatchSkill) Name() string {
	return "apply_patch"
}

func (s *ApplyPatchSkill) Description() string {
	return "Apply line-based patches to a file. Each patch line is 'lineNumber|newContent' (1-based). Useful for editing specific lines without rewriting the whole file."
}

func (s *ApplyPatchSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, _ := input["path"].(string)
	patch, _ := input["patch"].(string)
	projectID, _ := input["project_id"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if patch == "" {
		return "", fmt.Errorf("patch is required")
	}

	// S3 is the sole source of truth for file content
	if s.storage == nil {
		return "", fmt.Errorf("s3 not configured")
	}
	content, err := readFileContent(ctx, s.storage, s.db, projectID, path)
	if err != nil {
		return "", fmt.Errorf("read failed: %w", err)
	}
	newContent, applied, err := applyPatchToContent(content, patch)
	if err != nil {
		return "", err
	}
	if err := writeFileContent(ctx, s.storage, s.db, projectID, path, newContent); err != nil {
		return "", fmt.Errorf("write failed: %w", err)
	}
	return fmt.Sprintf("Applied %d line patch(es) to %s [s3]", applied, path), nil
}

// applyPatchToContent parses the patch string and applies line-based edits.
// Patch format: each line is "lineNumber|newContent" (1-based).
// Returns the new content, number of applied patches, and any error.
func applyPatchToContent(content string, patch string) (string, int, error) {
	lines := strings.Split(content, "\n")
	applied := 0

	for _, patchLine := range strings.Split(patch, "\n") {
		patchLine = strings.TrimSpace(patchLine)
		if patchLine == "" {
			continue
		}

		parts := strings.SplitN(patchLine, "|", 2)
		if len(parts) != 2 {
			continue // skip malformed lines
		}

		lineNum, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || lineNum < 1 || lineNum > len(lines) {
			continue // skip invalid line numbers
		}

		lines[lineNum-1] = parts[1]
		applied++
	}

	if applied == 0 {
		return content, 0, fmt.Errorf("no valid patches applied (check line numbers are within file range)")
	}

	return strings.Join(lines, "\n"), applied, nil
}

func (s *ApplyPatchSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: true,
		Core:      true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
