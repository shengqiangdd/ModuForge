package skills

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

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

	// S3 is the sole source of truth for file content
	if s.storage == nil {
		return "", fmt.Errorf("s3 not configured")
	}
	content, err := readFileContent(ctx, s.storage, s.db, projectID, path)
	if err != nil {
		return "", fmt.Errorf("read failed: %w", err)
	}
	if !strings.Contains(content, oldText) {
		return "", fmt.Errorf("old_text not found in file: %s", path)
	}
	newContent := strings.ReplaceAll(content, oldText, newText)
	if err := writeFileContent(ctx, s.storage, s.db, projectID, path, newContent); err != nil {
		return "", fmt.Errorf("write failed: %w", err)
	}
	return fmt.Sprintf("File edited: %s (%d bytes) [s3]", path, len(newContent)), nil
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
