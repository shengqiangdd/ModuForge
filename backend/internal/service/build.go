package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/moduforge/backend/internal/builder"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/domain"
)

type BuildService struct {
	db  *sql.DB
	cfg *config.Config
}

func NewBuildService(db *sql.DB, cfg *config.Config) *BuildService {
	return &BuildService{db: db, cfg: cfg}
}

func (s *BuildService) Create(ctx context.Context, projectID, target string) (*domain.BuildTask, error) {
	// Verify project exists
	var exists int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM projects WHERE id=? AND deleted_at IS NULL`, projectID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("project not found")
	}

	var task domain.BuildTask
	task.ID = uuid.New().String()
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO build_tasks (id, project_id, target) VALUES (?, ?, ?)
		 RETURNING id, project_id, status, target, log, artifact_path, created_at, updated_at`,
		task.ID, projectID, target,
	).Scan(&task.ID, &task.ProjectID, &task.Status, &task.Target, &task.Log, &task.ArtifactPath, &task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		return nil, err
	}

	// Start async build
	go s.runBuild(task.ID, projectID)

	return &task, nil
}

func (s *BuildService) Get(ctx context.Context, id string) (*domain.BuildTask, error) {
	var t domain.BuildTask
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, status, target, log, artifact_path, created_at, updated_at
		 FROM build_tasks WHERE id=?`, id,
	).Scan(&t.ID, &t.ProjectID, &t.Status, &t.Target, &t.Log, &t.ArtifactPath, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("build task not found")
	}
	return &t, nil
}

func (s *BuildService) GetArtifact(ctx context.Context, id string) (*string, error) {
	t, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if t.ArtifactPath == nil {
		return nil, fmt.Errorf("artifact not ready")
	}
	return t.ArtifactPath, nil
}

func (s *BuildService) runBuild(taskID, projectID string) {
	ctx := context.Background()

	// Mark as running
	s.db.ExecContext(ctx,
		`UPDATE build_tasks SET status=?, log=?, updated_at=datetime('now') WHERE id=?`,
		domain.BuildRunning, "Collecting files...\n", taskID)

	// Get project module type as build target
	var target string
	err := s.db.QueryRowContext(ctx,
		`SELECT module_type FROM projects WHERE id=?`, projectID).Scan(&target)
	if err != nil {
		s.failBuild(ctx, taskID, fmt.Sprintf("Error reading project: %v\n", err))
		return
	}

	// Collect project files to a temp directory
	projectDir, err := os.MkdirTemp("", "moduforge-build-*")
	if err != nil {
		s.failBuild(ctx, taskID, fmt.Sprintf("Error creating temp dir: %v\n", err))
		return
	}
	defer os.RemoveAll(projectDir)

	rows, err := s.db.QueryContext(ctx,
		`SELECT path, content FROM project_files WHERE project_id=?`, projectID)
	if err != nil {
		s.failBuild(ctx, taskID, fmt.Sprintf("Error reading files: %v\n", err))
		return
	}
	defer rows.Close()

	var fileCount int
	for rows.Next() {
		var path, content string
		if err := rows.Scan(&path, &content); err != nil {
			continue
		}
		fullPath := filepath.Join(projectDir, filepath.Clean(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			continue
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			continue
		}
		fileCount++
	}

	if fileCount == 0 {
		s.failBuild(ctx, taskID, "No files in project\n")
		return
	}

	s.db.ExecContext(ctx,
		`UPDATE build_tasks SET status=?, log=?, updated_at=datetime('now') WHERE id=?`,
		domain.BuildRunning,
		fmt.Sprintf("Collecting %d files...\nValidating module structure...\nPackaging...\n", fileCount),
		taskID)

	// Build using the real builder
	b := builder.NewBuilder(s.cfg)
	artifactPath, err := b.Build(ctx, projectDir, target, taskID)
	if err != nil {
		s.failBuild(ctx, taskID, fmt.Sprintf("Build failed: %v\n", err))
		return
	}

	log := fmt.Sprintf("Collecting %d files...\nValidating module structure...\nPackaging...\nBuild complete!\n", fileCount)
	s.db.ExecContext(ctx,
		`UPDATE build_tasks SET status=?, log=?, artifact_path=?, updated_at=datetime('now') WHERE id=?`,
		domain.BuildSuccess, log, artifactPath, taskID)
}

func (s *BuildService) failBuild(ctx context.Context, taskID, log string) {
	s.db.ExecContext(ctx,
		`UPDATE build_tasks SET status=?, log=?, updated_at=datetime('now') WHERE id=?`,
		domain.BuildFailed, log, taskID)
}
