package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/moduforge/backend/internal/agent/skills"
	"github.com/moduforge/backend/internal/domain"
	"github.com/moduforge/backend/internal/storage"
)

// FileContentRepo provides S3-only content access.
// S3 is the sole source of truth for file content; the DB only stores metadata
// (sha256, file_size, mtime) for fast lookups and build cache validation.
type FileContentRepo struct {
	db *sql.DB
	s3 storage.StorageAdapter
}

// NewFileContentRepo creates a repo with the given DB and S3 adapter.
func NewFileContentRepo(db *sql.DB, s3 storage.StorageAdapter) *FileContentRepo {
	return &FileContentRepo{db: db, s3: s3}
}

// S3ObjectKey returns the S3 object key (relative to the adapter prefix).
// Uses the shared implementation from skills/fileutil.go for consistency.
func (r *FileContentRepo) S3ObjectKey(projectID, path string) string {
	return skills.S3ObjectKey(projectID, path)
}

// ReadOne returns the content of a single file from S3.
// Returns an error if S3 is not configured or the file is not found.
func (r *FileContentRepo) ReadOne(ctx context.Context, projectID, path string) (string, error) {
	if r.s3 == nil {
		return "", fmt.Errorf("s3 not configured")
	}
	data, err := r.s3.Read(ctx, r.S3ObjectKey(projectID, path))
	if err != nil {
		slog.Warn("s3 read failed", "project", projectID, "path", path, "error", err)
		return "", fmt.Errorf("s3 read failed: %w", err)
	}
	return string(data), nil
}

// ReadAll returns all files (metadata only, no content) for a project, ordered by path.
func (r *FileContentRepo) ReadAll(ctx context.Context, projectID string) ([]domain.ProjectFile, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, path, created_at, updated_at, COALESCE(sha256,''), COALESCE(file_size,0), COALESCE(mtime,'')
		 FROM project_files WHERE project_id=? ORDER BY path`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []domain.ProjectFile
	for rows.Next() {
		var f domain.ProjectFile
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.Path, &f.CreatedAt, &f.UpdatedAt, &f.SHA256, &f.FileSize, &f.MTime); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

// ReadAllContent returns a map[path]content for a project from S3.
func (r *FileContentRepo) ReadAllContent(ctx context.Context, projectID string) (map[string]string, error) {
	files, err := r.ReadAll(ctx, projectID)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(files))
	for _, f := range files {
		content, err := r.ReadOne(ctx, projectID, f.Path)
		if err != nil {
			slog.Warn("failed to read file from S3", "project", projectID, "path", f.Path, "error", err)
			continue
		}
		result[f.Path] = content
	}
	return result, nil
}

// Write persists content to S3 and updates DB metadata (no content in DB).
func (r *FileContentRepo) Write(ctx context.Context, projectID, path, content string) error {
	if r.s3 == nil {
		return fmt.Errorf("s3 not configured")
	}
	if r.db == nil {
		return fmt.Errorf("db not available")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	sha := storage.ComputeSHA256([]byte(content))
	size := int64(len(content))

	// Write content to S3
	if err := r.s3.Write(ctx, r.S3ObjectKey(projectID, path), []byte(content)); err != nil {
		return fmt.Errorf("s3 write failed: %w", err)
	}

	// Update DB metadata only (no content column)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO project_files (project_id, path, created_at, updated_at, sha256, file_size, mtime)
		 VALUES (?, ?, datetime('now'), datetime('now'), ?, ?, ?)
		 ON CONFLICT(project_id, path) DO UPDATE SET updated_at=datetime('now'), sha256=?, file_size=?, mtime=?`,
		projectID, path, sha, size, now, sha, size, now)
	return err
}

// Delete removes content from S3 and the DB metadata row.
func (r *FileContentRepo) Delete(ctx context.Context, projectID, path string) error {
	if r.s3 != nil {
		if err := r.s3.Delete(ctx, r.S3ObjectKey(projectID, path)); err != nil {
			return fmt.Errorf("s3 delete failed: %w", err)
		}
	}
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM project_files WHERE project_id=? AND path=?`, projectID, path)
	return err
}
