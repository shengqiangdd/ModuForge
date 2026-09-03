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

type MoveFileSkill struct {
	projectPath string
	db          *sql.DB
	storage     storage.StorageAdapter // optional S3 storage backend
}

func NewMoveFileSkill(projectPath string, db *sql.DB) *MoveFileSkill {
	return &MoveFileSkill{projectPath: projectPath, db: db}
}

// WithStorage sets the S3 storage adapter. When set, files are moved in S3.
func (s *MoveFileSkill) WithStorage(st storage.StorageAdapter) *MoveFileSkill {
	s.storage = st
	return s
}

func (s *MoveFileSkill) Name() string {
	return "move_file"
}

func (s *MoveFileSkill) Description() string {
	return "Move or rename a file. Input: {\"from\": \"...\", \"to\": \"...\", \"project_id\": \"...\"}. Both paths are relative to project root."
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
	basePath := ResolveProjectPath(s.db, s.projectPath, projectID)
	fullFrom := filepath.Join(basePath, from)
	fullTo := filepath.Join(basePath, to)
	if !isPathWithin(basePath, fullFrom) {
		return "", fmt.Errorf("path traversal not allowed for source: %s", from)
	}
	if !isPathWithin(basePath, fullTo) {
		return "", fmt.Errorf("path traversal not allowed for destination: %s", to)
	}

	// Read content from source (S3 first, DB fallback)
	content, err := readFileContent(ctx, s.storage, s.db, projectID, from)
	if err != nil {
		return "", fmt.Errorf("source file not found: %s", from)
	}

	// Write to destination (S3 write + DB metadata/fallback)
	if err := writeFileContent(ctx, s.storage, s.db, projectID, to, content); err != nil {
		return "", fmt.Errorf("failed to write to destination: %w", err)
	}

	// Delete source
	if err := deleteFileContent(ctx, s.storage, s.db, projectID, from); err != nil {
		return "", fmt.Errorf("failed to delete source file: %w", err)
	}

	// Disk operation (best-effort)
	os.MkdirAll(filepath.Dir(fullTo), 0755)
	os.Rename(fullFrom, fullTo) // ignore error

	return fmt.Sprintf("File moved: %s → %s", from, to), nil
}

func (s *MoveFileSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		Core:      true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
