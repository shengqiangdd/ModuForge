package skills

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/agent/registry"
	"github.com/moduforge/backend/internal/storage"
)

type EditFileSkill struct {
	projectPath string
	db          *sql.DB
	storage     storage.StorageAdapter // optional S3 storage backend
}

func NewEditFileSkillWithDB(projectPath string, db *sql.DB) *EditFileSkill {
	return &EditFileSkill{projectPath: projectPath, db: db}
}

// WithStorage sets the S3 storage adapter. When set, files are edited in S3
// and DB only holds metadata.
func (s *EditFileSkill) WithStorage(st storage.StorageAdapter) *EditFileSkill {
	s.storage = st
	return s
}

func (s *EditFileSkill) Name() string {
	return "edit_file"
}

func (s *EditFileSkill) Description() string {
	return "Edit a file by finding and replacing text. Input: {\"path\": \"...\", \"old_text\": \"...\", \"new_text\": \"...\", \"project_id\": \"...\"}. Uses find-and-replace to modify file content."
}

func (s *EditFileSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, _ := input["path"].(string)
	oldText, _ := input["old_text"].(string)
	newText, _ := input["new_text"].(string)
	projectID, _ := input["project_id"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if oldText == "" {
		return "", fmt.Errorf("old_text is required")
	}
	if oldText == newText {
		return "", fmt.Errorf("old_text and new_text are identical")
	}

	// S3 storage path: read from S3, edit, write back to S3
	if s.storage != nil {
		content, err := s.storage.Read(ctx, s.storagePath(projectID, path))
		if err != nil {
			return "", fmt.Errorf("s3 read failed: %w", err)
		}
		text := string(content)
		if !strings.Contains(text, oldText) {
			return "", fmt.Errorf("old_text not found in file: %s", path)
		}
		newContent := strings.ReplaceAll(text, oldText, newText)
		contentBytes := []byte(newContent)
		if err := s.storage.Write(ctx, s.storagePath(projectID, path), contentBytes); err != nil {
			return "", fmt.Errorf("s3 write failed: %w", err)
		}
		sha256 := storage.ComputeSHA256(contentBytes)
		now := time.Now().Format(time.RFC3339)
		s.syncMetadataToDB(projectID, path, sha256, now, int64(len(contentBytes)))
		return fmt.Sprintf("File edited: %s (%d bytes) [s3]", path, len(newContent)), nil
	}

	// Legacy path: read from DB, edit, write to disk, sync to DB
	// Read from database
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

	if !strings.Contains(content, oldText) {
		return "", fmt.Errorf("old_text not found in file: %s", path)
	}

	newContent := strings.ReplaceAll(content, oldText, newText)

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

	return fmt.Sprintf("File edited: %s (%d bytes)", path, len(newContent)), nil
}

func (s *EditFileSkill) syncToDB(projectID, path, content string) {
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
		fmt.Printf("[EditFileSkill] syncToDB failed: %v\n", err)
	}
}

func (s *EditFileSkill) syncMetadataToDB(projectID, path, sha256, mtime string, size int64) {
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
		fmt.Printf("[EditFileSkill] syncMetadataToDB failed: %v\n", err)
	}
}

func (s *EditFileSkill) storagePath(projectID, path string) string {
	return "projects/" + projectID + "/" + path
}

func (s *EditFileSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: true,
		Core:      true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}