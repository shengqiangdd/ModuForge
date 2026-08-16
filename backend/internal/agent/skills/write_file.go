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

// resolvePath 根据 project_id 解析实际文件路径
// syncToDB 将文件内容同步到 project_files 表（upsert）
func (s *WriteFileSkill) syncToDB(projectID, path, content string) {
	if s.db == nil || projectID == "" {
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	// SQLite: INSERT OR REPLACE with UNIQUE(project_id, path)
	_, err := s.db.Exec(
		`INSERT INTO project_files (project_id, path, content, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(project_id, path) DO UPDATE SET content=excluded.content, updated_at=excluded.updated_at`,
		projectID, path, content, now, now,
	)
	if err != nil {
		// fallback: DELETE + INSERT
		_, err = s.db.Exec(
			`DELETE FROM project_files WHERE project_id=? AND path=?`,
			projectID, path,
		)
		if err == nil {
			_, err = s.db.Exec(
				`INSERT INTO project_files (project_id, path, content, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?)`,
				projectID, path, content, now, now,
			)
		}
		if err != nil {
			fmt.Printf("[WriteFileSkill] syncToDB failed: %v\n", err)
		}
	}
}

// syncMetadataToDB records file metadata (sha256, size, mtime) in the DB.
// Used when the S3 storage adapter is active (content is in S3, not DB).
func (s *WriteFileSkill) syncMetadataToDB(projectID, path, sha256, mtime string, size int64) {
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
		fmt.Printf("[WriteFileSkill] syncMetadataToDB failed: %v\n", err)
	}
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

	// Safety check: compare with existing content in database
	if s.db != nil && projectID != "" {
		var existingContent string
		err := s.db.QueryRow(
			`SELECT content FROM project_files WHERE project_id=? AND path=?`, projectID, path,
		).Scan(&existingContent)
		if err == nil && len(existingContent) > 0 {
			// Guard: reject if new content is drastically shorter than existing (likely truncation/garbage)
			if len(content) < len(existingContent)/20 && len(existingContent) > 500 {
				return "", fmt.Errorf("content too short (%d bytes vs existing %d bytes) - possible truncation. Aborting to prevent data loss", len(content), len(existingContent))
			}
		}
	}

	// S3 storage path: primary write to S3, then record metadata in DB
	if s.storage != nil {
		contentBytes := []byte(content)
		if err := s.storage.Write(ctx, s.storagePath(projectID, path), contentBytes); err != nil {
			return "", fmt.Errorf("s3 write failed: %w", err)
		}
		sha256 := storage.ComputeSHA256(contentBytes)
		now := time.Now().Format(time.RFC3339)
		s.syncMetadataToDB(projectID, path, sha256, now, int64(len(contentBytes)))
		statusMsg := fmt.Sprintf("File written successfully: %s (%d bytes) [s3]", path, len(content))
		if projectID != "" {
			statusMsg = fmt.Sprintf("[project_id:%s] %s", projectID, statusMsg)
		}
		return statusMsg, nil
	}

	// Legacy path: write to DB + disk (dual-write)
	// Primary write path: database (project_files table)
	if s.db != nil && projectID != "" {
		s.syncToDB(projectID, path, content)
	}

	// Optional: write to filesystem (may fail in read-only containers)
	basePath := ResolveProjectPath(s.db, s.projectPath, projectID)
	fullPath := filepath.Join(basePath, path)
	if !isPathWithin(basePath, fullPath) {
		return "", fmt.Errorf("path traversal not allowed")
	}

	diskWritten := false
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err == nil {
		if err := os.WriteFile(fullPath, []byte(content), 0644); err == nil {
			diskWritten = true
		}
	}

	// Build status message
	statusMsg := fmt.Sprintf("File written successfully: %s (%d bytes)", path, len(content))
	if diskWritten {
		statusMsg += " [disk+db]"
	} else {
		statusMsg += " [db only] NOTE: 磁盘不可写，文件存储在数据库中。如果需要编译，请使用 build_module，它会自动从 DB 导出文件到磁盘。"
	}
	if projectID != "" {
		statusMsg = fmt.Sprintf("[project_id:%s] %s", projectID, statusMsg)
	}

	// Verify database write
	if s.db != nil && projectID != "" {
		var dbContent string
		err := s.db.QueryRow(
			`SELECT content FROM project_files WHERE project_id=? AND path=?`, projectID, path,
		).Scan(&dbContent)
		if err != nil {
			statusMsg += "\nWarning: database verification failed: " + err.Error()
		} else if dbContent != content {
			statusMsg += "\nWarning: database content mismatch"
		}
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
	if s.storage != nil {
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
			if err := s.storage.Write(ctx, s.storagePath(projectID, path), contentBytes); err != nil {
				continue
			}
			sha256 := storage.ComputeSHA256(contentBytes)
			s.syncMetadataToDB(projectID, path, sha256, now, int64(len(contentBytes)))
			paths = append(paths, path)
		}
		result := fmt.Sprintf("[project_id:%s] Batch wrote %d files to S3: %s", projectID, len(paths), strings.Join(paths, ", "))
		return result, nil
	}

	// Legacy batch path: write to DB
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

		var paths []string
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
			if _, err := stmt.ExecContext(ctx, projectID, path, content, now, now); err != nil {
				continue
			}
			paths = append(paths, path)
		}

		if err := tx.Commit(); err != nil {
			return "", fmt.Errorf("failed to commit batch: %w", err)
		}

		result := fmt.Sprintf("[project_id:%s] Batch wrote %d files: %s", projectID, len(paths), strings.Join(paths, ", "))
		return result, nil
	}

	return "", fmt.Errorf("database not available for batch write")
}

// storagePath constructs the S3 path for a project file.
// NOTE: the S3Adapter prepends its configured prefix ("projects"), so we pass
// the project-relative key here — DO NOT prefix with "projects/" again.
func (s *WriteFileSkill) storagePath(projectID, path string) string {
	return S3ObjectKey(projectID, path)
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