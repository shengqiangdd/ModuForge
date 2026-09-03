package skills

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/storage"
)

// S3ObjectKey returns the S3 object key for a project file. The S3Adapter
// already prepends its configured prefix (e.g. "projects"), so callers pass
// the project-relative key: "<projectID>/<path>".
func S3ObjectKey(projectID, path string) string {
	return projectID + "/" + strings.TrimPrefix(path, "/")
}

// readFileContent returns file content from S3 only.
// Returns an error if S3 is not configured or the file is not found.
func readFileContent(ctx context.Context, st storage.StorageAdapter, db *sql.DB, projectID, path string) (string, error) {
	if st == nil {
		return "", fmt.Errorf("s3 not configured")
	}
	data, err := st.Read(ctx, S3ObjectKey(projectID, path))
	if err != nil {
		slog.Warn("s3 read failed", "project", projectID, "path", path, "error", err)
		return "", fmt.Errorf("s3 read failed: %w", err)
	}
	return string(data), nil
}

// writeFileContent persists content to S3 and updates DB metadata.
func writeFileContent(ctx context.Context, st storage.StorageAdapter, db *sql.DB, projectID, path, content string) error {
	if st == nil {
		return fmt.Errorf("s3 not configured")
	}
	if db == nil {
		return fmt.Errorf("database not available")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	sha := storage.ComputeSHA256([]byte(content))
	size := int64(len(content))

	// Write content to S3
	if err := st.Write(ctx, S3ObjectKey(projectID, path), []byte(content)); err != nil {
		return fmt.Errorf("s3 write failed: %w", err)
	}

	// Update DB metadata only (no content column)
	_, err := db.ExecContext(ctx,
		`INSERT INTO project_files (project_id, path, created_at, updated_at, sha256, file_size, mtime)
		 VALUES (?, ?, datetime('now'), datetime('now'), ?, ?, ?)
		 ON CONFLICT(project_id, path) DO UPDATE SET updated_at=datetime('now'), sha256=?, file_size=?, mtime=?`,
		projectID, path, sha, size, now, sha, size, now)
	return err
}

// deleteFileContent removes content from S3 and the DB metadata row.
func deleteFileContent(ctx context.Context, st storage.StorageAdapter, db *sql.DB, projectID, path string) error {
	if st != nil {
		if err := st.Delete(ctx, S3ObjectKey(projectID, path)); err != nil {
			return fmt.Errorf("s3 delete failed: %w", err)
		}
	}
	if db == nil {
		return nil
	}
	_, err := db.ExecContext(ctx,
		`DELETE FROM project_files WHERE project_id=? AND path=?`, projectID, path)
	return err
}

// syncMetadataToDB records file metadata (sha256, size, mtime) in the DB.
func syncMetadataToDB(db *sql.DB, projectID, path, sha256, mtime string, size int64) {
	if db == nil || projectID == "" {
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec(
		`INSERT INTO project_files (project_id, path, created_at, updated_at, sha256, file_size, mtime)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(project_id, path) DO UPDATE SET sha256=excluded.sha256, file_size=excluded.file_size, mtime=excluded.mtime, updated_at=excluded.updated_at`,
		projectID, path, now, now, sha256, size, mtime,
	)
	if err != nil {
		slog.Warn("syncMetadataToDB failed", "project", projectID, "path", path, "error", err)
	}
}

// storageLabel returns a human-readable label for logging.
func storageLabel(st storage.StorageAdapter) string {
	if st == nil {
		return "DB"
	}
	return "S3"
}
