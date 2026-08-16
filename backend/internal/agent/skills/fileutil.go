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

// readFileContent returns file content with S3-first semantics and a DB
// content-column fallback for legacy data / no-S3 environments.
func readFileContent(ctx context.Context, st storage.StorageAdapter, db *sql.DB, projectID, path string) (string, error) {
	if st != nil {
		data, err := st.Read(ctx, S3ObjectKey(projectID, path))
		if err == nil {
			return string(data), nil
		}
		slog.Warn("s3 read failed, falling back to db content", "project", projectID, "path", path, "error", err)
	}
	if db == nil {
		return "", fmt.Errorf("no storage backend available")
	}
	var content string
	err := db.QueryRowContext(ctx,
		`SELECT COALESCE(content,'') FROM project_files WHERE project_id=? AND path=?`, projectID, path,
	).Scan(&content)
	if err != nil {
		return "", err
	}
	return content, nil
}

// writeFileContent persists content with S3 as the authoritative store:
// S3 write first, then DB metadata (content NULL). Falls back to the DB
// content column when S3 is not configured.
func writeFileContent(ctx context.Context, st storage.StorageAdapter, db *sql.DB, projectID, path, content string) error {
	if db == nil {
		return fmt.Errorf("database not available")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	sha := storage.ComputeSHA256([]byte(content))
	size := int64(len(content))

	if st != nil {
		if err := st.Write(ctx, S3ObjectKey(projectID, path), []byte(content)); err != nil {
			return fmt.Errorf("s3 write failed: %w", err)
		}
		_, err := db.ExecContext(ctx,
			`INSERT INTO project_files (project_id, path, content, created_at, updated_at, sha256, file_size, mtime)
			 VALUES (?, ?, NULL, datetime('now'), datetime('now'), ?, ?, ?)
			 ON CONFLICT(project_id, path) DO UPDATE SET content=NULL, updated_at=datetime('now'), sha256=?, file_size=?, mtime=?`,
			projectID, path, sha, size, now, sha, size, now)
		return err
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO project_files (project_id, path, content, created_at, updated_at, sha256, file_size, mtime)
		 VALUES (?, ?, ?, datetime('now'), datetime('now'), ?, ?, ?)
		 ON CONFLICT(project_id, path) DO UPDATE SET content=?, updated_at=datetime('now'), sha256=?, file_size=?, mtime=?`,
		projectID, path, content, sha, size, now, content, sha, size, now)
	return err
}

// deleteFileContent removes content from S3 (if available) and the DB row.
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

// readAllFileContents returns all project files as map[path]content (S3 first).
func readAllFileContents(ctx context.Context, st storage.StorageAdapter, db *sql.DB, projectID string) (map[string]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}
	rows, err := db.QueryContext(ctx,
		`SELECT path FROM project_files WHERE project_id=?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var path string
		if rows.Scan(&path) != nil {
			continue
		}
		content, err := readFileContent(ctx, st, db, projectID, path)
		if err != nil {
			continue
		}
		result[path] = content
	}
	return result, nil
}

// storageLabel returns a human-readable label for logging.
func storageLabel(st storage.StorageAdapter) string {
	if st == nil {
		return "DB"
	}
	return "S3"
}
