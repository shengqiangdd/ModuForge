package skills

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

type DeleteFileSkill struct {
	projectPath string
	db          *sql.DB
}

func NewDeleteFileSkill(projectPath string, db *sql.DB) *DeleteFileSkill {
	return &DeleteFileSkill{projectPath: projectPath, db: db}
}

func (s *DeleteFileSkill) Name() string {
	return "delete_file"
}

func (s *DeleteFileSkill) Description() string {
	return "Delete a file from the project. Input: {\"path\": \"...\", \"project_id\": \"...\"}. Removes from both database and disk."
}

func (s *DeleteFileSkill) resolvePath(projectID string) string {
	return ResolveProjectPath(s.db, s.projectPath, projectID)
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
	basePath := s.resolvePath(projectID)
	fullPath := filepath.Join(basePath, path)
	if !isPathWithin(basePath, fullPath) {
		return "", fmt.Errorf("path traversal not allowed: %s", path)
	}

	// Delete from database
	if s.db != nil {
		result, err := s.db.Exec(
			`DELETE FROM project_files WHERE project_id=? AND path=?`,
			projectID, path,
		)
		if err != nil {
			return "", fmt.Errorf("failed to delete from database: %w", err)
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return "", fmt.Errorf("file not found in project: %s", path)
		}
	}

	// Delete from disk (best-effort)
	os.Remove(fullPath) // ignore error if file doesn't exist on disk

	return fmt.Sprintf("File deleted: %s", path), nil
}

func (s *DeleteFileSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  false,
		Essential: false,
		Core:      true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
