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

// BatchEditFileSkill provides atomic multi-file editing.
// P1-3: Added to support batch editing of multiple files in a single transaction.
type BatchEditFileSkill struct {
	projectPath string
	db          *sql.DB
}

func NewBatchEditFileSkill(projectPath string, db *sql.DB) *BatchEditFileSkill {
	return &BatchEditFileSkill{projectPath: projectPath, db: db}
}

func (s *BatchEditFileSkill) Name() string {
	return "batch_edit_file"
}

func (s *BatchEditFileSkill) Description() string {
	return `Edit multiple files atomically in a single transaction. All edits succeed or all fail.
Input: {"edits": [{"path": "...", "old_text": "...", "new_text": "..."}, ...], "project_id": "..."}.
More efficient than calling edit_file multiple times for related changes.`
}

func (s *BatchEditFileSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	editsRaw, ok := input["edits"].([]interface{})
	if !ok || len(editsRaw) == 0 {
		return "", fmt.Errorf("edits array is required")
	}
	projectID, _ := input["project_id"].(string)
	userID, _ := input["user_id"].(string)

	// Auto-create project if needed
	if projectID == "" && s.db != nil && userID != "" {
		newID := fmt.Sprintf("agent_%d", time.Now().UnixNano())
		_, err := s.db.Exec(
			`INSERT INTO projects (id, user_id, name, module_type, description)
			 VALUES (?, ?, ?, 'universal', ?)`,
			newID, userID, "Agent Workspace", "Auto-created by Agent on batch edit",
		)
		if err != nil {
			return "", fmt.Errorf("failed to auto-create project: %w", err)
		}
		projectID = newID
	}

	// Validate all edits before applying any
	type editOp struct {
		path    string
		oldText string
		newText string
	}
	var ops []editOp

	for i, e := range editsRaw {
		editMap, ok := e.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("edit %d: invalid format", i)
		}
		path, _ := editMap["path"].(string)
		oldText, _ := editMap["old_text"].(string)
		newText, _ := editMap["new_text"].(string)

		if path == "" {
			return "", fmt.Errorf("edit %d: path is required", i)
		}
		if oldText == "" {
			return "", fmt.Errorf("edit %d: old_text is required", i)
		}
		if oldText == newText {
			return "", fmt.Errorf("edit %d: old_text and new_text are identical", i)
		}

		// Read file and validate old_text exists
		projectPath := ResolveProjectPath(s.db, s.projectPath, projectID)
		fullPath := filepath.Join(projectPath, path)
		if !isPathWithin(projectPath, fullPath) {
			return "", fmt.Errorf("edit %d: path traversal not allowed", i)
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			return "", fmt.Errorf("edit %d: failed to read %s: %w", i, path, err)
		}
		content := string(data)

		if !strings.Contains(content, oldText) {
			return "", fmt.Errorf("edit %d: old_text not found in %s", i, path)
		}

		ops = append(ops, editOp{path: path, oldText: oldText, newText: newText})
	}

	// All validations passed — apply edits in a database transaction
	if s.db != nil && projectID != "" {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return "", fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback()

		now := time.Now().Format("2006-01-02 15:04:05")
		stmt, err := tx.PrepareContext(ctx,
			`INSERT INTO project_files (project_id, path, content, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(project_id, path) DO UPDATE SET content=excluded.content, updated_at=excluded.updated_at`)
		if err != nil {
			return "", fmt.Errorf("failed to prepare statement: %w", err)
		}
		defer stmt.Close()

		var editedPaths []string
		for _, op := range ops {
			// Read file content
			projectPath := ResolveProjectPath(s.db, s.projectPath, projectID)
			fullPath := filepath.Join(projectPath, op.path)

			data, err := os.ReadFile(fullPath)
			if err != nil {
				return "", fmt.Errorf("failed to read %s: %w", op.path, err)
			}
			content := string(data)

			// Apply edit
			newContent := strings.ReplaceAll(content, op.oldText, op.newText)

			// Safety check
			if len(newContent) < len(content)/10 && len(content) > 100 {
				return "", fmt.Errorf("edit to %s results in too short content (%d vs %d bytes)", op.path, len(newContent), len(content))
			}

			// Write to disk
			if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
				return "", fmt.Errorf("failed to write %s: %w", op.path, err)
			}

			// Sync to DB
			if _, err := stmt.ExecContext(ctx, projectID, op.path, newContent, now, now); err != nil {
				return "", fmt.Errorf("failed to sync %s to DB: %w", op.path, err)
			}

			editedPaths = append(editedPaths, op.path)
		}

		if err := tx.Commit(); err != nil {
			return "", fmt.Errorf("failed to commit batch edit: %w", err)
		}

		result := fmt.Sprintf("[project_id:%s] Batch edited %d files: %s", projectID, len(editedPaths), strings.Join(editedPaths, ", "))
		return result, nil
	}

	// No DB — apply edits directly to disk
	var editedPaths []string
	for _, op := range ops {
		projectPath := ResolveProjectPath(s.db, s.projectPath, projectID)
		fullPath := filepath.Join(projectPath, op.path)

		data, err := os.ReadFile(fullPath)
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", op.path, err)
		}
		content := string(data)

		newContent := strings.ReplaceAll(content, op.oldText, op.newText)

		if len(newContent) < len(content)/10 && len(content) > 100 {
			return "", fmt.Errorf("edit to %s results in too short content", op.path)
		}

		if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
			return "", fmt.Errorf("failed to write %s: %w", op.path, err)
		}

		editedPaths = append(editedPaths, op.path)
	}

	return fmt.Sprintf("Batch edited %d files: %s", len(editedPaths), strings.Join(editedPaths, ", ")), nil
}

func (s *BatchEditFileSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		Core:      true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
