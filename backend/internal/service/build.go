package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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

// GetStoragePath returns the storage path for cache management.
func (s *BuildService) GetStoragePath() string {
	return s.cfg.StoragePath
}

func (s *BuildService) getCacheKey(projectID, filesHash string) string {
	return filepath.Join(s.cfg.StoragePath, "build-cache", projectID, filesHash+".zip")
}

func (s *BuildService) computeFilesHash(ctx context.Context, projectID string) (string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT path, content FROM project_files WHERE project_id=? ORDER BY path`, projectID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	h := sha256.New()
	for rows.Next() {
		var path, content string
		if err := rows.Scan(&path, &content); err != nil {
			continue
		}
		h.Write([]byte(path))
		h.Write([]byte(content))
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

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

func (s *BuildService) checkBuildCache(projectID, filesHash string) *string {
	cachePath := s.getCacheKey(projectID, filesHash)
	info, err := os.Stat(cachePath)
	if err != nil {
		return nil
	}
	if time.Since(info.ModTime()) > 24*time.Hour {
		os.Remove(cachePath)
		return nil
	}
	return &cachePath
}

func (s *BuildService) saveBuildCache(projectID, filesHash, outputPath string) error {
	cachePath := s.getCacheKey(projectID, filesHash)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return err
	}
	input, err := os.ReadFile(outputPath)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, input, 0644)
}

func (s *BuildService) ClearBuildCache(ctx context.Context, projectID string) error {
	cacheDir := filepath.Join(s.cfg.StoragePath, "build-cache", projectID)
	return os.RemoveAll(cacheDir)
}

// GetBuildCacheStatus returns cache statistics for a project.
func (s *BuildService) GetBuildCacheStatus(ctx context.Context, projectID string) (map[string]interface{}, error) {
	cacheDir := filepath.Join(s.cfg.StoragePath, "build-cache", projectID)

	var totalSize int64
	var fileCount int

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return map[string]interface{}{
			"total_size": 0,
			"file_count": 0,
			"hit_rate":   0,
		}, nil
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		totalSize += info.Size()
		fileCount++
	}

	// Get build history for hit rate calculation
	var totalBuilds, cacheHits int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(CASE WHEN log LIKE '%cache hit%' OR log LIKE '%缓存命中%' THEN 1 ELSE 0 END), 0)
		 FROM build_tasks WHERE project_id=?`, projectID).Scan(&totalBuilds, &cacheHits)
	if err != nil {
		totalBuilds = 0
		cacheHits = 0
	}

	hitRate := 0.0
	if totalBuilds > 0 {
		hitRate = float64(cacheHits) / float64(totalBuilds) * 100
	}

	return map[string]interface{}{
		"total_size":  totalSize,
		"file_count":  fileCount,
		"hit_rate":    hitRate,
		"total_builds": totalBuilds,
		"cache_hits":  cacheHits,
	}, nil
}

func (s *BuildService) Create(ctx context.Context, projectID, target string) (*domain.BuildTask, *domain.BuildCacheResponse, error) {
	return s.CreateWithArch(ctx, projectID, target, "arm64")
}

// CreateWithArch creates a build task with architecture support.
func (s *BuildService) CreateWithArch(ctx context.Context, projectID, target, arch string) (*domain.BuildTask, *domain.BuildCacheResponse, error) {
	filesHash, err := s.computeFilesHash(ctx, projectID)
	if err != nil {
		return nil, nil, fmt.Errorf("compute hash: %w", err)
	}
	if cachedPath := s.checkBuildCache(projectID, filesHash); cachedPath != nil {
		task := &domain.BuildTask{
			ID:           uuid.New().String(),
			ProjectID:    projectID,
			Status:       domain.BuildSuccess,
			Target:       target,
			Log:          "Build cache hit — using cached artifact\n",
			ArtifactPath: cachedPath,
			Trigger:      "manual",
			CreatedAt:    domain.Now(),
			UpdatedAt:    domain.Now(),
		}
		s.db.ExecContext(ctx,
			`INSERT INTO build_tasks (id, project_id, status, target, log, artifact_path, trigger, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			task.ID, projectID, task.Status, target, task.Log, *cachedPath, "manual", task.CreatedAt, task.UpdatedAt)
		return task, &domain.BuildCacheResponse{Cached: true, TaskID: task.ID}, nil
	}
	return s.createAndBuild(ctx, projectID, target, "manual", "", filesHash, arch)
}

func (s *BuildService) CreateWithTrigger(ctx context.Context, projectID, target, trigger, commitHash, arch string) (*domain.BuildTask, error) {
	if arch == "" {
		arch = "arm64"
	}
	filesHash, err := s.computeFilesHash(ctx, projectID)
	if err != nil {
		filesHash = ""
	}
	if cachedPath := s.checkBuildCache(projectID, filesHash); cachedPath != nil {
		task := &domain.BuildTask{
			ID:           uuid.New().String(),
			ProjectID:    projectID,
			Status:       domain.BuildSuccess,
			Target:       target,
			Log:          "Build cache hit — using cached artifact\n",
			ArtifactPath: cachedPath,
			Trigger:      trigger,
			CommitHash:   commitHash,
			CreatedAt:    domain.Now(),
			UpdatedAt:    domain.Now(),
		}
		s.db.ExecContext(ctx,
			`INSERT INTO build_tasks (id, project_id, status, target, log, artifact_path, trigger, commit_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			task.ID, projectID, task.Status, target, task.Log, *cachedPath, trigger, commitHash, task.CreatedAt, task.UpdatedAt)
		return task, nil
	}
	task, _, err := s.createAndBuild(ctx, projectID, target, trigger, commitHash, filesHash, arch)
	return task, err
}

func (s *BuildService) createAndBuild(ctx context.Context, projectID, target, trigger, commitHash, filesHash, arch string) (*domain.BuildTask, *domain.BuildCacheResponse, error) {
	var exists int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM projects WHERE id=? AND deleted_at IS NULL`, projectID).Scan(&exists)
	if err != nil {
		return nil, nil, fmt.Errorf("project not found")
	}

	var task domain.BuildTask
	task.ID = uuid.New().String()
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO build_tasks (id, project_id, target, log, trigger, commit_hash) VALUES (?, ?, ?, '', ?, ?)
		 RETURNING id, project_id, status, target, log, artifact_path, trigger, commit_hash, created_at, updated_at`,
		task.ID, projectID, target, trigger, commitHash,
	).Scan(&task.ID, &task.ProjectID, &task.Status, &task.Target, &task.Log, &task.ArtifactPath, &task.Trigger, &task.CommitHash, &task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		return nil, nil, err
	}

	go s.runBuild(task.ID, projectID, filesHash, arch)

	return &task, nil, nil
}

// buildTimeout is the maximum time a build can run before being killed.
const buildTimeout = 15 * time.Minute

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

func (s *BuildService) TriggerBuildFromGit(ctx context.Context, projectID, commitHash string) (*domain.BuildTask, error) {
	return s.CreateWithTrigger(ctx, projectID, "universal", "git", commitHash, "arm64")
}

func (s *BuildService) Get(ctx context.Context, id string) (*domain.BuildTask, error) {
	var t domain.BuildTask
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, status, COALESCE(target,''), COALESCE(log,''), COALESCE(artifact_path,''), created_at, updated_at
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

func (s *BuildService) runBuild(taskID, projectID, filesHash, arch string) {
	defer func() {
		if r := recover(); r != nil {
			s.failBuild(context.Background(), taskID, fmt.Sprintf("[PANIC] %v\n", r))
		}
	}()
	// Build timeout: cancel if build takes too long
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	// Mark as running
	if _, err := s.db.ExecContext(ctx,
		`UPDATE build_tasks SET status=?, log=?, updated_at=datetime('now') WHERE id=?`,
		domain.BuildRunning, "Collecting files...\n", taskID); err != nil {
		return
	}

	target := "universal"

	// Collect project files to a temp directory (single read from DB)
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
	scanFiles := make(map[string]string)
	for rows.Next() {
		var path, content string
		if err := rows.Scan(&path, &content); err != nil {
			continue
		}
		fullPath := filepath.Join(projectDir, filepath.Clean(path))
		// Prevent path traversal: ensure fullPath is within projectDir
		if !strings.HasPrefix(fullPath, projectDir+string(os.PathSeparator)) && fullPath != projectDir {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			continue
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			continue
		}
		scanFiles[path] = content
		fileCount++
	}

	if fileCount == 0 {
		s.failBuild(ctx, taskID, "No files in project\n")
		return
	}

	// Log file count
	s.db.ExecContext(ctx,
		`UPDATE build_tasks SET log=log || ? WHERE id=?`,
		fmt.Sprintf("Found %d file(s)\n", fileCount), taskID)

	// Security scan using the already-collected files
	if len(scanFiles) > 0 {
		scanner := NewSecurityScanner()
		scanResult := scanner.ScanFiles(scanFiles)
		if !scanResult.Safe {
			s.failBuild(ctx, taskID, fmt.Sprintf("Build blocked by security scan:\n%s\n\nIssues:\n", scanResult.Summary))
			for _, issue := range scanResult.Issues {
				if issue.Severity == "critical" {
					s.db.ExecContext(ctx,
						`UPDATE build_tasks SET log=log || ? WHERE id=?`,
						fmt.Sprintf("[%s] %s:%d - %s\n", issue.Severity, issue.File, issue.Line, issue.Message),
						taskID)
				}
			}
			return
		}
		for _, issue := range scanResult.Issues {
			if issue.Severity == "warning" {
				s.db.ExecContext(ctx,
					`UPDATE build_tasks SET log=log || ? WHERE id=?`,
					fmt.Sprintf("[WARN] %s:%d - %s\n", issue.File, issue.Line, issue.Message),
					taskID)
			}
		}
	}

	s.db.ExecContext(ctx,
		`UPDATE build_tasks SET status=?, log=log || ?, updated_at=datetime('now') WHERE id=?`,
		domain.BuildRunning,
		fmt.Sprintf("Collecting %d files...\nValidating module structure...\nPackaging...\n", fileCount),
		taskID)

	// Build using the real builder with arch support
	b := builder.NewBuilder(s.cfg)
	logFn := func(msg string) {
		s.db.ExecContext(ctx,
			`UPDATE build_tasks SET log=log || ? WHERE id=?`, msg, taskID)
	}
	buildResult, err := b.BuildWithResult(ctx, projectDir, target, taskID, arch, logFn)
	if err != nil {
		s.failBuild(ctx, taskID, fmt.Sprintf("Build failed: %v\n", err))
		return
	}

	s.db.ExecContext(ctx,
		`UPDATE build_tasks SET status=?, log=log || ?, artifact_path=?, updated_at=datetime('now') WHERE id=?`,
		domain.BuildSuccess, fmt.Sprintf("Build complete! (arch=%s)\n", arch), buildResult.ArtifactPath, taskID)
	if filesHash != "" {
		s.saveBuildCache(projectID, filesHash, buildResult.ArtifactPath)
	}
}

func (s *BuildService) failBuild(ctx context.Context, taskID, log string) {
	s.db.ExecContext(ctx,
		`UPDATE build_tasks SET status=?, log=?, updated_at=datetime('now') WHERE id=?`,
		domain.BuildFailed, log, taskID)
}
