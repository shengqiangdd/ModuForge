package skills

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type WriteFileSkill struct {
	projectPath string
	db          *sql.DB
}

func NewWriteFileSkillWithDB(projectPath string, db *sql.DB) *WriteFileSkill {
	return &WriteFileSkill{projectPath: projectPath, db: db}
}

func (s *WriteFileSkill) Name() string {
	return "write_file"
}

func (s *WriteFileSkill) Description() string {
	return "Write content to a file. Parent directories are auto-created. Input: {\"path\": \"...\", \"content\": \"...\", \"project_id\": \"...\" (optional)}. Verifies content was saved correctly."
}

// resolvePath 根据 project_id 解析实际文件路径
func (s *WriteFileSkill) resolvePath(projectID string) string {
	return ResolveProjectPath(s.db, s.projectPath, projectID)
}

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

	// Primary write path: database (project_files table)
	// This works even in read-only containers
	if s.db != nil && projectID != "" {
		s.syncToDB(projectID, path, content)
	}

	// Optional: write to filesystem (may fail in read-only containers)
	basePath := s.resolvePath(projectID)
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
	// Include project_id in result so runner can detect auto-creation
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

func (s *WriteFileSkill) Metadata() SkillMeta {
	return SkillMeta{
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

	// Batch write in transaction
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

// WriteFileBatchSkill wraps WriteFileSkill.ExecuteBatch as a standalone Skill.
type WriteFileBatchSkill struct {
	inner *WriteFileSkill
}

func NewWriteFileBatchSkill(projectPath string, db *sql.DB) *WriteFileBatchSkill {
	return &WriteFileBatchSkill{
		inner: NewWriteFileSkillWithDB(projectPath, db),
	}
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

func (s *WriteFileBatchSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  false,
		Essential: false,
		Core:      true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
