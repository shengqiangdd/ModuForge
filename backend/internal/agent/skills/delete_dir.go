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

type DeleteDirSkill struct {
	projectPath string
	db          *sql.DB
	storage     storage.StorageAdapter // optional S3 storage backend
}

func NewDeleteDirSkill(projectPath string, db *sql.DB) *DeleteDirSkill {
	return &DeleteDirSkill{projectPath: projectPath, db: db}
}

// WithStorage sets the S3 storage adapter. When set, files are deleted in S3.
func (s *DeleteDirSkill) WithStorage(st storage.StorageAdapter) *DeleteDirSkill {
	s.storage = st
	return s
}

func (s *DeleteDirSkill) Name() string {
	return "delete_dir"
}

func (s *DeleteDirSkill) Description() string {
	return "Delete a directory and all files within it. Input: {\"path\": \"...\", \"project_id\": \"...\"}. Use path=\".\" to delete entire project."
}

func (s *DeleteDirSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, _ := input["path"].(string)
	projectID, _ := input["project_id"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if projectID == "" {
		return "", fmt.Errorf("project_id is required")
	}

	// Safety check: confirm deletion of entire project
	if path == "." || path == "" {
		confirm, _ := input["confirm"].(bool)
		if !confirm {
			return "", fmt.Errorf("deleting entire project (path='.') requires confirm=true. This will remove ALL files in the project")
		}
	}

	// Path traversal protection
	basePath := ResolveProjectPath(s.db, s.projectPath, projectID)
	fullPath := filepath.Join(basePath, path)
	if !isPathWithin(basePath, fullPath) {
		return "", fmt.Errorf("path traversal not allowed: %s", path)
	}

	// Delete from S3 + database
	if s.db != nil {
		// Build the LIKE pattern for matching files under this directory
		likePattern := path + "/%"
		if path == "." {
			likePattern = "%"
		}

		// List files under the directory
		rows, err := s.db.Query(
			`SELECT path FROM project_files WHERE project_id=? AND (path LIKE ? OR path=?)`,
			projectID, likePattern, path,
		)
		if err != nil {
			return "", fmt.Errorf("failed to list files: %w", err)
		}
		var paths []string
		for rows.Next() {
			var p string
			if rows.Scan(&p) == nil {
				paths = append(paths, p)
			}
		}
		rows.Close()

		// Delete each file from S3 + DB
		deleted := 0
		for _, p := range paths {
			if err := deleteFileContent(ctx, s.storage, s.db, projectID, p); err != nil {
				continue
			}
			deleted++
		}

		// Delete from disk (best-effort)
		os.RemoveAll(fullPath) // ignore error

		return fmt.Sprintf("Directory deleted: %s (%d files removed)", path, deleted), nil
	}

	// No DB: just delete from disk
	err := os.RemoveAll(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to delete directory %s: %w", path, err)
	}

	return fmt.Sprintf("Directory deleted: %s", path), nil
}

func (s *DeleteDirSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		Core:      true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
