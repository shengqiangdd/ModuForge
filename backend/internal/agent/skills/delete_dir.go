package skills

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
			return "", fmt.Errorf("deleting entire project (path='.') requires confirm=true. This will remove ALL files in the project.")
		}
	}

	// Path traversal protection
	basePath := s.resolvePath(projectID)
	fullPath := filepath.Join(basePath, path)
	if !filepath.HasPrefix(fullPath, filepath.Clean(basePath)) {
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

// countFilesInDir counts files in a directory path from DB
func countFilesInDir(db *sql.DB, projectID, dirPath string) int {
	if db == nil {
		return 0
	}
	likePattern := dirPath + "/%"
	if dirPath == "." {
		likePattern = "%"
	}
	var count int
	db.QueryRow(
		`SELECT COUNT(*) FROM project_files WHERE project_id=? AND path LIKE ?`,
		projectID, likePattern,
	).Scan(&count)
	return count
}

// listFilesInDir returns files in a directory from DB
func listFilesInDir(db *sql.DB, projectID, dirPath string) []string {
	if db == nil {
		return nil
	}
	likePattern := dirPath + "/%"
	if dirPath == "." {
		likePattern = "%"
	}
	rows, err := db.Query(
		`SELECT path FROM project_files WHERE project_id=? AND path LIKE ? ORDER BY path`,
		projectID, likePattern,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err == nil {
			files = append(files, p)
		}
	}
	return files
}

// isDirEmpty checks if a directory path has any files in DB
func isDirEmpty(db *sql.DB, projectID, dirPath string) bool {
	likePattern := dirPath + "/%"
	var count int
	db.QueryRow(
		`SELECT COUNT(*) FROM project_files WHERE project_id=? AND path LIKE ?`,
		projectID, likePattern,
	).Scan(&count)
	return count == 0
}

// getImmediateChildren returns immediate child entries (files and subdirs) under a path
func getImmediateChildren(db *sql.DB, projectID, dirPath string) map[string]bool {
	if db == nil {
		return nil
	}
	likePattern := dirPath + "/%"
	if dirPath == "." {
		likePattern = "%"
	}
	rows, err := db.Query(
		`SELECT path FROM project_files WHERE project_id=? AND path LIKE ? ORDER BY path`,
		projectID, likePattern,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	children := make(map[string]bool)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err == nil {
			// Get the part after the directory path
			rel := strings.TrimPrefix(p, dirPath+"/")
			// Get first component
			if idx := strings.Index(rel, "/"); idx >= 0 {
				children[rel[:idx]+"/"] = true // directory
			} else {
				children[rel] = false // file
			}
		}
	}
	return children
}

func (s *DeleteDirSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  false,
		Essential: false,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
