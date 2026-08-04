package skills

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

type DeleteDirSkill struct {
	projectPath string
	db          *sql.DB
}

func NewDeleteDirSkill(projectPath string, db *sql.DB) *DeleteDirSkill {
	return &DeleteDirSkill{projectPath: projectPath, db: db}
}

func (s *DeleteDirSkill) Name() string {
	return "delete_dir"
}

func (s *DeleteDirSkill) Description() string {
	return "Delete a directory and all files within it. Input: {\"path\": \"...\", \"project_id\": \"...\"}. Use path=\".\" to delete entire project."
}

func (s *DeleteDirSkill) resolvePath(projectID string) string {
	return ResolveProjectPath(s.db, s.projectPath, projectID)
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
	basePath := s.resolvePath(projectID)
	fullPath := filepath.Join(basePath, path)
	if !isPathWithin(basePath, fullPath) {
		return "", fmt.Errorf("path traversal not allowed: %s", path)
	}

	// Delete from database
	if s.db != nil {
		// Build the LIKE pattern for matching files under this directory
		likePattern := path + "/%"
		if path == "." {
			likePattern = "%"
		}

		// Delete all files under the directory
		result, err := s.db.Exec(
			`DELETE FROM project_files WHERE project_id=? AND (path LIKE ? OR path=?)`,
			projectID, likePattern, path,
		)
		if err != nil {
			return "", fmt.Errorf("failed to delete from database: %w", err)
		}
		affected, _ := result.RowsAffected()

		// Delete from disk (best-effort)
		os.RemoveAll(fullPath) // ignore error

		return fmt.Sprintf("Directory deleted: %s (%d files removed)", path, affected), nil
	}

	// No DB: just delete from disk
	err := os.RemoveAll(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to delete directory %s: %w", path, err)
	}

	return fmt.Sprintf("Directory deleted: %s", path), nil
}

func (s *DeleteDirSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  false,
		Essential: false,
		Core:      true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
