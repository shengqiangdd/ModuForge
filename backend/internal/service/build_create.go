package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/moduforge/backend/internal/builder"
	"github.com/moduforge/backend/internal/domain"
)

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

func (s *BuildService) TriggerBuildFromGit(ctx context.Context, projectID, commitHash string) (*domain.BuildTask, error) {
	return s.CreateWithTrigger(ctx, projectID, "universal", "git", commitHash, "arm64")
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

	// Collect project files to a temp directory (S3 first, DB fallback)
	projectDir, err := os.MkdirTemp("", "moduforge-build-*")
	if err != nil {
		s.failBuild(ctx, taskID, fmt.Sprintf("Error creating temp dir: %v\n", err))
		return
	}
	defer os.RemoveAll(projectDir)

	var fileCount int
	scanFiles := make(map[string]string)

	if s.fr != nil {
		// S3-first: export all project files from the object store.
		files, err := s.fr.ReadAll(ctx, projectID)
		if err != nil {
			s.failBuild(ctx, taskID, fmt.Sprintf("Error reading files: %v\n", err))
			return
		}
		for _, f := range files {
			content, err := s.fr.ReadOne(ctx, projectID, f.Path)
			if err != nil {
				continue
			}
			fullPath := filepath.Join(projectDir, filepath.Clean(f.Path))
			if !strings.HasPrefix(fullPath, projectDir+string(os.PathSeparator)) && fullPath != projectDir {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				continue
			}
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				continue
			}
			scanFiles[f.Path] = content
			fileCount++
		}
	} else {
		rows, err := s.db.QueryContext(ctx,
			`SELECT path, content FROM project_files WHERE project_id=?`, projectID)
		if err != nil {
			s.failBuild(ctx, taskID, fmt.Sprintf("Error reading files: %v\n", err))
			return
		}
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
		rows.Close()
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
		`UPDATE build_tasks SET status=?, log=log || ?, updated_at=datetime('now') WHERE id=?`,
		domain.BuildFailed, log, taskID)
}
