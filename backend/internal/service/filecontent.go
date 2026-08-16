package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/domain"
	"github.com/moduforge/backend/internal/storage"
)

// FileContentRepo provides S3-first content access with DB fallback.
// It is the shared helper for code paths that need project file contents
// without going through ProjectService (build, zipper, analysis handlers,
// agent skills). S3 is the authoritative truth source; the DB content column
// is only a fallback for legacy data / no-S3 environments.
type FileContentRepo struct {
	db *sql.DB
	s3 storage.StorageAdapter
}

// NewFileContentRepo creates a repo with the given DB and optional S3 adapter.
func NewFileContentRepo(db *sql.DB, s3 storage.StorageAdapter) *FileContentRepo {
	return &FileContentRepo{db: db, s3: s3}
}

// S3ObjectKey returns the S3 object key (relative to the adapter prefix).
func (r *FileContentRepo) S3ObjectKey(projectID, path string) string {
	return projectID + "/" + strings.TrimPrefix(path, "/")
}

// ReadOne returns the content of a single file (S3 first, DB fallback).
func (r *FileContentRepo) ReadOne(ctx context.Context, projectID, path string) (string, error) {
	if r.s3 != nil {
		data, err := r.s3.Read(ctx, r.S3ObjectKey(projectID, path))
		if err == nil {
			return string(data), nil
		}
		slog.Warn("s3 read failed, falling back to db content", "project", projectID, "path", path, "error", err)
	}
	var content string
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(content,'') FROM project_files WHERE project_id=? AND path=?`, projectID, path,
	).Scan(&content)
	if err != nil {
		return "", err
	}
	return content, nil
}

// ReadAll returns all files (path+content) for a project, ordered by path.
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

// ReadAllContent returns a map[path]content for a project (S3 first, DB fallback).
func (r *FileContentRepo) ReadAllContent(ctx context.Context, projectID string) (map[string]string, error) {
	files, err := r.ReadAll(ctx, projectID)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(files))
	for _, f := range files {
		content, err := r.ReadOne(ctx, projectID, f.Path)
		if err != nil {
			continue
		}
		result[f.Path] = content
	}
	return result, nil
}

// Write persists content: S3 (authoritative) + DB metadata (content NULL).
// Falls back to DB content column when S3 is not configured.
func (r *FileContentRepo) Write(ctx context.Context, projectID, path, content string) error {
	if r.db == nil {
		return fmt.Errorf("db not available")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	sha := storage.ComputeSHA256([]byte(content))
	size := int64(len(content))

	if r.s3 != nil {
		if err := r.s3.Write(ctx, r.S3ObjectKey(projectID, path), []byte(content)); err != nil {
			return fmt.Errorf("s3 write failed: %w", err)
		}
		_, err := r.db.ExecContext(ctx,
			`INSERT INTO project_files (project_id, path, content, created_at, updated_at, sha256, file_size, mtime)
			 VALUES (?, ?, NULL, datetime('now'), datetime('now'), ?, ?, ?)
			 ON CONFLICT(project_id, path) DO UPDATE SET content=NULL, updated_at=datetime('now'), sha256=?, file_size=?, mtime=?`,
			projectID, path, sha, size, now, sha, size, now)
		return err
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO project_files (project_id, path, content, created_at, updated_at, sha256, file_size, mtime)
		 VALUES (?, ?, ?, datetime('now'), datetime('now'), ?, ?, ?)
		 ON CONFLICT(project_id, path) DO UPDATE SET content=?, updated_at=datetime('now'), sha256=?, file_size=?, mtime=?`,
		projectID, path, content, sha, size, now, content, sha, size, now)
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
