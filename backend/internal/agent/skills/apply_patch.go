package skills

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

	// S3 storage path
	if s.storage != nil {
		content, err := s.storage.Read(ctx, s.storagePath(projectID, path))
		if err != nil {
			return "", fmt.Errorf("s3 read failed: %w", err)
		}
		newContent, applied, err := applyPatchToContent(string(content), patch)
		if err != nil {
			return "", err
		}
		contentBytes := []byte(newContent)
		if err := s.storage.Write(ctx, s.storagePath(projectID, path), contentBytes); err != nil {
			return "", fmt.Errorf("s3 write failed: %w", err)
		}
		sha256 := storage.ComputeSHA256(contentBytes)
		now := time.Now().Format(time.RFC3339)
		s.syncMetadataToDB(projectID, path, sha256, now, int64(len(contentBytes)))
		return fmt.Sprintf("Applied %d line patch(es) to %s [s3]", applied, path), nil
	}

	// Database path
	if s.db == nil || projectID == "" {
		return "", fmt.Errorf("database not available")
	}

	var content string
	err := s.db.QueryRow(
		`SELECT content FROM project_files WHERE project_id=? AND path=?`, projectID, path,
	).Scan(&content)
	if err != nil {
		return "", fmt.Errorf("file not found in database: %s", path)
	}

	newContent, applied, err := applyPatchToContent(content, patch)
	if err != nil {
		return "", err
	}

	// Write to disk
	basePath := ResolveProjectPath(s.db, s.projectPath, projectID)
	fullPath := filepath.Join(basePath, path)
	if !isPathWithin(basePath, fullPath) {
		return "", fmt.Errorf("path traversal not allowed")
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err == nil {
		os.WriteFile(fullPath, []byte(newContent), 0644) // Best effort
	}

	// Sync to database
	s.syncToDB(projectID, path, newContent)

	return fmt.Sprintf("Applied %d line patch(es) to %s", applied, path), nil
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

func (s *ApplyPatchSkill) syncToDB(projectID, path, content string) {
	if s.db == nil || projectID == "" {
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := s.db.Exec(
		`INSERT INTO project_files (project_id, path, content, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(project_id, path) DO UPDATE SET content=excluded.content, updated_at=excluded.updated_at`,
		projectID, path, content, now, now,
	)
	if err != nil {
		fmt.Printf("[ApplyPatchSkill] syncToDB failed: %v\n", err)
	}
}

func (s *ApplyPatchSkill) syncMetadataToDB(projectID, path, sha256, mtime string, size int64) {
	if s.db == nil || projectID == "" {
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := s.db.Exec(
		`INSERT INTO project_files (project_id, path, content, sha256, file_size, mtime, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(project_id, path) DO UPDATE SET sha256=excluded.sha256, file_size=excluded.file_size, mtime=excluded.mtime, updated_at=excluded.updated_at`,
		projectID, path, "", sha256, size, mtime, now, now,
	)
	if err != nil {
		fmt.Printf("[ApplyPatchSkill] syncMetadataToDB failed: %v\n", err)
	}
}

func (s *ApplyPatchSkill) storagePath(projectID, path string) string {
	return S3ObjectKey(projectID, path)
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
