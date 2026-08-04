package skills

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type MoveFileSkill struct {
	projectPath string
	db          *sql.DB
}

func NewMoveFileSkill(projectPath string, db *sql.DB) *MoveFileSkill {
	return &MoveFileSkill{projectPath: projectPath, db: db}
}

func (s *MoveFileSkill) Name() string {
	return "move_file"
}

func (s *MoveFileSkill) Description() string {
	return "Move or rename a file. Input: {\"from\": \"...\", \"to\": \"...\", \"project_id\": \"...\"}. Both paths are relative to project root."
}

func (s *MoveFileSkill) resolvePath(projectID string) string {
	if projectID == "" {
		return s.projectPath
	}
	if s.db != nil {
		var storagePath string
		err := s.db.QueryRow(`SELECT COALESCE(storage_path,'') FROM projects WHERE id=?`, projectID).Scan(&storagePath)
		if err == nil && storagePath != "" {
			return storagePath
		}
	}
	return filepath.Join(s.projectPath, projectID)
}

func (s *MoveFileSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	from, _ := input["from"].(string)
	to, _ := input["to"].(string)
	projectID, _ := input["project_id"].(string)

	if from == "" {
		return "", fmt.Errorf("from is required")
	}
	if to == "" {
		return "", fmt.Errorf("to is required")
	}
	if projectID == "" {
		return "", fmt.Errorf("project_id is required")
	}

	// Path traversal protection for both paths
	basePath := s.resolvePath(projectID)
	fullFrom := filepath.Join(basePath, from)
	fullTo := filepath.Join(basePath, to)
	if !filepath.HasPrefix(fullFrom, filepath.Clean(basePath)) {
		return "", fmt.Errorf("path traversal not allowed for source: %s", from)
	}
	if !filepath.HasPrefix(fullTo, filepath.Clean(basePath)) {
		return "", fmt.Errorf("path traversal not allowed for destination: %s", to)
	}

	// DB transaction: read source, write to destination, delete source
	if s.db != nil {
		// Read content from source
		var content string
		err := s.db.QueryRow(
			`SELECT content FROM project_files WHERE project_id=? AND path=?`,
			projectID, from,
		).Scan(&content)
		if err != nil {
			return "", fmt.Errorf("source file not found: %s", from)
		}

		// Insert at destination
		now := time.Now().Format("2006-01-02 15:04:05")
		_, err = s.db.Exec(
			`INSERT INTO project_files (project_id, path, content, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(project_id, path) DO UPDATE SET content=excluded.content, updated_at=excluded.updated_at`,
			projectID, to, content, now, now,
		)
		if err != nil {
			return "", fmt.Errorf("failed to write to destination: %w", err)
		}

		// Delete source
		_, err = s.db.Exec(
			`DELETE FROM project_files WHERE project_id=? AND path=?`,
			projectID, from,
		)
		if err != nil {
			return "", fmt.Errorf("failed to delete source file: %w", err)
		}
	}

	// Disk operation (best-effort)
	os.MkdirAll(filepath.Dir(fullTo), 0755)
	os.Rename(fullFrom, fullTo) // ignore error

	return fmt.Sprintf("File moved: %s → %s", from, to), nil
}

func (s *MoveFileSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  false,
		Essential: false,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
