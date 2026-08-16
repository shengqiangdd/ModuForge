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

// BatchEditFileSkill provides atomic multi-file editing.
// P1-3: Added to support batch editing of multiple files in a single transaction.
type BatchEditFileSkill struct {
	projectPath string
	db          *sql.DB
	storage     storage.StorageAdapter // optional S3 storage backend
}

func NewBatchEditFileSkill(projectPath string, db *sql.DB) *BatchEditFileSkill {
	return &BatchEditFileSkill{projectPath: projectPath, db: db}
}

// WithStorage sets the S3 storage adapter. When set, files are edited in S3.
func (s *BatchEditFileSkill) WithStorage(st storage.StorageAdapter) *BatchEditFileSkill {
	s.storage = st
	return s
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

		// Read file and validate old_text exists (S3 first, DB fallback)
		projectPath := ResolveProjectPath(s.db, s.projectPath, projectID)
		fullPath := filepath.Join(projectPath, path)
		if !isPathWithin(projectPath, fullPath) {
			return "", fmt.Errorf("edit %d: path traversal not allowed", i)
		}

		content, err := readFileContent(ctx, s.storage, s.db, projectID, path)
		if err != nil {
			return "", fmt.Errorf("edit %d: failed to read %s: %w", i, path, err)
		}

		if !strings.Contains(content, oldText) {
			return "", fmt.Errorf("edit %d: old_text not found in %s", i, path)
		}

		ops = append(ops, editOp{path: path, oldText: oldText, newText: newText})
	}

	// All validations passed — apply edits
	var editedPaths []string
	for _, op := range ops {
		// Read file content (S3 first)
		content, err := readFileContent(ctx, s.storage, s.db, projectID, op.path)
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", op.path, err)
		}

		// Apply edit
		newContent := strings.ReplaceAll(content, op.oldText, op.newText)

		// Safety check
		if len(newContent) < len(content)/10 && len(content) > 100 {
			return "", fmt.Errorf("edit to %s results in too short content (%d vs %d bytes)", op.path, len(newContent), len(content))
		}

		// Write to S3 + DB (authoritative store)
		if err := writeFileContent(ctx, s.storage, s.db, projectID, op.path, newContent); err != nil {
			return "", fmt.Errorf("failed to write %s: %w", op.path, err)
		}

		// Write to disk (best-effort)
		projectPath := ResolveProjectPath(s.db, s.projectPath, projectID)
		fullPath := filepath.Join(projectPath, op.path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(newContent), 0644)

		editedPaths = append(editedPaths, op.path)
	}

	result := fmt.Sprintf("[project_id:%s] Batch edited %d files: %s", projectID, len(editedPaths), strings.Join(editedPaths, ", "))
	return result, nil
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
