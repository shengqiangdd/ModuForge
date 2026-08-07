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
)

type EditFileSkill struct {
	projectPath string
	db          *sql.DB
}

func NewEditFileSkillWithDB(projectPath string, db *sql.DB) *EditFileSkill {
	return &EditFileSkill{projectPath: projectPath, db: db}
}

func (s *EditFileSkill) Name() string {
	return "edit_file"
}

func (s *EditFileSkill) Description() string {
	return `Find and replace text in a file. All occurrences of old_text are replaced with new_text.
Input: {"path": "...", "old_text": "...", "new_text": "...", "project_id": "...", "count": 1 (optional, 0=all)}.
More efficient than write_file for targeted changes — preserves surrounding content.`
}

func (s *EditFileSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, _ := input["path"].(string)
	oldText, _ := input["old_text"].(string)
	newText, _ := input["new_text"].(string)
	projectID, _ := input["project_id"].(string)
	count := 0 // 0 = replace all
	if v, ok := input["count"].(float64); ok {
		count = int(v)
	}

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if oldText == "" {
		return "", fmt.Errorf("old_text is required")
	}
	if oldText == newText {
		return "", fmt.Errorf("old_text and new_text are identical")
	}

	projectPath := ResolveProjectPath(s.db, s.projectPath, projectID)
	fullPath := filepath.Join(projectPath, path)
	if !isPathWithin(projectPath, fullPath) {
		return "", fmt.Errorf("path traversal not allowed")
	}

	// Read existing content
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	content := string(data)

	// Check old_text exists
	if !strings.Contains(content, oldText) {
		return "", fmt.Errorf("old_text not found in %s", path)
	}

	// Count occurrences
	occurrences := strings.Count(content, oldText)
	if occurrences == 0 {
		return "", fmt.Errorf("old_text not found in %s", path)
	}

	// Replace
	var newContent string
	if count == 0 || count >= occurrences {
		// Replace all
		newContent = strings.ReplaceAll(content, oldText, newText)
	} else {
		// Replace first N occurrences
		newContent = strings.Replace(content, oldText, newText, count)
	}

	// Safety check: don't allow drastically shorter content
	if len(newContent) < len(content)/10 && len(content) > 100 {
		return "", fmt.Errorf("resulting content too short (%d vs %d bytes) — possible truncation", len(newContent), len(content))
	}

	// Write back to disk
	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Sync to DB
	if s.db != nil && projectID != "" {
		s.syncToDB(projectID, path, newContent)
	}

	replacedCount := occurrences
	if count > 0 && count < occurrences {
		replacedCount = count
	}

	return fmt.Sprintf("File edited: %s — replaced %d occurrence(s) of old_text (%d bytes → %d bytes)",
		path, replacedCount, len(content), len(newContent)), nil
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

func (s *EditFileSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: true,
		Core:      true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
