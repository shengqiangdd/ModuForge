package skills

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/moduforge/backend/internal/agent/registry"
	"github.com/moduforge/backend/internal/storage"
	"os"
	"path/filepath"
)

type DeleteFileSkill struct {
	projectPath string
	db          *sql.DB
	storage     storage.StorageAdapter // optional S3 storage backend
}

func NewDeleteFileSkill(projectPath string, db *sql.DB) *DeleteFileSkill {
	return &DeleteFileSkill{projectPath: projectPath, db: db}
}

// WithStorage sets the S3 storage adapter. When set, files are deleted in S3.
func (s *DeleteFileSkill) WithStorage(st storage.StorageAdapter) *DeleteFileSkill {
	s.storage = st
	return s
}

func (s *DeleteFileSkill) Name() string {
	return "delete_file"
}

func (s *DeleteFileSkill) Description() string {
	return "Delete a file from the project. Input: {\"path\": \"...\", \"project_id\": \"...\"}. Removes from both database and disk."
}

func (s *DeleteFileSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, _ := input["path"].(string)
	projectID, _ := input["project_id"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if projectID == "" {
		return "", fmt.Errorf("project_id is required")
	}

	// Path traversal protection
	basePath := ResolveProjectPath(s.db, s.projectPath, projectID)
	fullPath := filepath.Join(basePath, path)
	if !isPathWithin(basePath, fullPath) {
		return "", fmt.Errorf("path traversal not allowed: %s", path)
	}

	// Delete from S3 + database
	if s.db != nil {
		if err := deleteFileContent(ctx, s.storage, s.db, projectID, path); err != nil {
			return "", fmt.Errorf("failed to delete: %w", err)
		}
	}

	// Delete from disk (best-effort)
	os.Remove(fullPath) // ignore error if file doesn't exist on disk

	return fmt.Sprintf("File deleted: %s", path), nil
}

func (s *DeleteFileSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		Core:      true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
