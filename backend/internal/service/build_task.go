package service

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/moduforge/backend/internal/domain"
)

// ListByProject returns build tasks for a given project, most recent first.
func (s *BuildService) ListByProject(ctx context.Context, projectID string) ([]domain.BuildTask, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, status, COALESCE(target,''), COALESCE(log,''), COALESCE(artifact_path,''), COALESCE(trigger,'manual'), COALESCE(commit_hash,''), created_at, updated_at
		 FROM build_tasks WHERE project_id=? ORDER BY created_at DESC LIMIT 50`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []domain.BuildTask
	for rows.Next() {
		var t domain.BuildTask
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Status, &t.Target, &t.Log, &t.ArtifactPath, &t.Trigger, &t.CommitHash, &t.CreatedAt, &t.UpdatedAt); err != nil {
			continue
		}
		tasks = append(tasks, t)
	}
	if tasks == nil {
		tasks = []domain.BuildTask{}
	}
	return tasks, nil
}

// DeleteBuild removes a single build task by ID.
func (s *BuildService) DeleteBuild(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM build_tasks WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("build not found")
	}
	return nil
}

// DeleteFailedBuilds removes all failed build tasks for a project.
// Uses a transaction to avoid SQLite lock contention.
func (s *BuildService) DeleteFailedBuilds(ctx context.Context, projectID string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("[BuildService] DeleteFailedBuilds begin tx failed for project %s: %v", projectID, err)
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Count first to give meaningful feedback
	var count int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM build_tasks WHERE project_id=? AND status='failed'`, projectID).Scan(&count); err != nil {
		log.Printf("[BuildService] DeleteFailedBuilds count failed for project %s: %v", projectID, err)
		return 0, fmt.Errorf("count failed builds: %w", err)
	}

	if count == 0 {
		log.Printf("[BuildService] DeleteFailedBuilds no failed builds for project %s", projectID)
		return 0, nil
	}

	result, err := tx.ExecContext(ctx,
		`DELETE FROM build_tasks WHERE project_id=? AND status='failed'`, projectID)
	if err != nil {
		log.Printf("[BuildService] DeleteFailedBuilds delete failed for project %s: %v", projectID, err)
		return 0, fmt.Errorf("delete: %w", err)
	}
	n, _ := result.RowsAffected()

	if err := tx.Commit(); err != nil {
		log.Printf("[BuildService] DeleteFailedBuilds commit failed for project %s: %v", projectID, err)
		return 0, fmt.Errorf("commit: %w", err)
	}

	log.Printf("[BuildService] DeleteFailedBuilds deleted %d failed builds for project %s", n, projectID)
	return n, nil
}

func (s *BuildService) GetWithCommit(ctx context.Context, id string) (*domain.BuildTask, error) {
	var t domain.BuildTask
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, status, COALESCE(target,''), COALESCE(log,''), COALESCE(artifact_path,''), COALESCE(trigger,'manual'), COALESCE(commit_hash,''), created_at, updated_at
		 FROM build_tasks WHERE id=?`, id,
	).Scan(&t.ID, &t.ProjectID, &t.Status, &t.Target, &t.Log, &t.ArtifactPath, &t.Trigger, &t.CommitHash, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("build task not found")
	}
	return &t, nil
}

func (s *BuildService) CancelBuild(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE build_tasks SET status=?, updated_at=datetime('now') WHERE id=? AND status IN ('pending','running')`,
		domain.BuildCancelled, id)
	return err
}

func (s *BuildService) Get(ctx context.Context, id string) (*domain.BuildTask, error) {
	var t domain.BuildTask
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, status, COALESCE(target,''), COALESCE(log,''), COALESCE(artifact_path,''), COALESCE(trigger,'manual'), COALESCE(commit_hash,''), created_at, updated_at
		 FROM build_tasks WHERE id=?`, id,
	).Scan(&t.ID, &t.ProjectID, &t.Status, &t.Target, &t.Log, &t.ArtifactPath, &t.Trigger, &t.CommitHash, &t.CreatedAt, &t.UpdatedAt)
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

// DeleteBuilds removes multiple build tasks by IDs.
func (s *BuildService) DeleteBuilds(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	// Build placeholders
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := "DELETE FROM build_tasks WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
