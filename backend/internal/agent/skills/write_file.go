package skills

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/agent/registry"
	"github.com/moduforge/backend/internal/storage"
)

type WriteFileSkill struct {
	projectPath string
	db          *sql.DB
	storage     storage.StorageAdapter // optional S3 storage backend
}

func NewWriteFileSkillWithDB(projectPath string, db *sql.DB) *WriteFileSkill {
	return &WriteFileSkill{projectPath: projectPath, db: db}
}

// WithStorage sets the S3 storage adapter. When set, files are stored in S3
// and DB only holds metadata (sha256, size, mtime). The old dual-write
// (disk + DB content) is bypassed.
func (s *WriteFileSkill) WithStorage(st storage.StorageAdapter) *WriteFileSkill {
	s.storage = st
	return s
}

func (s *WriteFileSkill) Name() string {
	return "write_file"
}

func (s *WriteFileSkill) Description() string {
	return "Write content to a file. Parent directories are auto-created. Input: {\"path\": \"...\", \"content\": \"...\", \"project_id\": \"...\" (optional)}. Verifies content was saved correctly."
}

func (s *WriteFileSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, _ := input["path"].(string)
	content, _ := input["content"].(string)
	projectID, _ := input["project_id"].(string)
	userID, _ := input["user_id"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("content cannot be empty or whitespace-only")
	}

	// Auto-create project if none provided (lazy: only when actually writing)
	if projectID == "" && s.db != nil && userID != "" {
		newID := fmt.Sprintf("agent_%d", time.Now().UnixNano())
		_, err := s.db.Exec(
			`INSERT INTO projects (id, user_id, name, module_type, description)
			 VALUES (?, ?, ?, 'universal', ?)`,
			newID, userID, "Agent Workspace", "Auto-created by Agent on first write",
		)
		if err != nil {
			return "", fmt.Errorf("failed to auto-create project: %w", err)
		}
		projectID = newID
		fmt.Printf("[WriteFileSkill] auto-created project %s for user %s\n", projectID, userID)
	}

	// S3 is the sole source of truth for file content
	if s.storage == nil {
		return "", fmt.Errorf("s3 not configured")
	}
	contentBytes := []byte(content)
	if err := s.storage.Write(ctx, S3ObjectKey(projectID, path), contentBytes); err != nil {
		return "", fmt.Errorf("s3 write failed: %w", err)
	}
	sha256 := storage.ComputeSHA256(contentBytes)
	now := time.Now().Format(time.RFC3339)
	syncMetadataToDB(s.db, projectID, path, sha256, now, int64(len(contentBytes)))
	statusMsg := fmt.Sprintf("File written successfully: %s (%d bytes) [s3]", path, len(content))
	if projectID != "" {
		statusMsg = fmt.Sprintf("[project_id:%s] %s", projectID, statusMsg)
	}
	return statusMsg, nil
}

func (s *WriteFileSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: true,
		Core:      true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}

// ExecuteBatch writes multiple files in a single transaction.
// Input: {"files": [{"path": "...", "content": "..."}, ...], "project_id": "..."}
func (s *WriteFileSkill) ExecuteBatch(ctx context.Context, input map[string]interface{}) (string, error) {
	filesRaw, ok := input["files"].([]interface{})
	if !ok || len(filesRaw) == 0 {
		return "", fmt.Errorf("files array is required")
	}
	projectID, _ := input["project_id"].(string)
	userID, _ := input["user_id"].(string)

	// Auto-create project if needed (same logic as single Execute)
	if projectID == "" && s.db != nil && userID != "" {
		newID := fmt.Sprintf("agent_%d", time.Now().UnixNano())
		_, err := s.db.Exec(
			`INSERT INTO projects (id, user_id, name, module_type, description)
			 VALUES (?, ?, ?, 'universal', ?)`,
			newID, userID, "Agent Workspace", "Auto-created by Agent on first write",
		)
		if err != nil {
			return "", fmt.Errorf("failed to auto-create project: %w", err)
		}
		projectID = newID
	}

	// S3 batch path
	if s.storage == nil {
		return "", fmt.Errorf("s3 not configured")
	}
	var paths []string
	now := time.Now().Format(time.RFC3339)
	for _, f := range filesRaw {
		fileMap, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		path, _ := fileMap["path"].(string)
		content, _ := fileMap["content"].(string)
		if path == "" || content == "" {
			continue
		}
		contentBytes := []byte(content)
		if err := s.storage.Write(ctx, S3ObjectKey(projectID, path), contentBytes); err != nil {
			continue
		}
		sha256 := storage.ComputeSHA256(contentBytes)
		syncMetadataToDB(s.db, projectID, path, sha256, now, int64(len(contentBytes)))
		paths = append(paths, path)
	}
	result := fmt.Sprintf("[project_id:%s] Batch wrote %d files to S3: %s", projectID, len(paths), strings.Join(paths, ", "))
	return result, nil
}

// WriteFileBatchSkill wraps WriteFileSkill.ExecuteBatch as a standalone Skill.
type WriteFileBatchSkill struct {
	inner *WriteFileSkill
}

func NewWriteFileBatchSkill(projectPath string, db *sql.DB) *WriteFileBatchSkill {
	return &WriteFileBatchSkill{
		inner: NewWriteFileSkillWithDB(projectPath, db),
	}
}

// WithStorage sets the S3 storage adapter for the batch skill.
func (s *WriteFileBatchSkill) WithStorage(st storage.StorageAdapter) *WriteFileBatchSkill {
	s.inner.WithStorage(st)
	return s
}

func (s *WriteFileBatchSkill) Name() string {
	return "write_file_batch"
}

func (s *WriteFileBatchSkill) Description() string {
	return "Write multiple files in a single database transaction. Input: {\"files\": [{\"path\": \"...\", \"content\": \"...\"}, ...], \"project_id\": \"...\" (optional)}. More efficient than calling write_file multiple times."
}

func (s *WriteFileBatchSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	return s.inner.ExecuteBatch(ctx, input)
}

func (s *WriteFileBatchSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		Core:      true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
