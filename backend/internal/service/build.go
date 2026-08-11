package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/moduforge/backend/internal/builder"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/domain"
)

// safeID sanitizes a project or build ID to prevent path traversal.
func safeID(id string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	return re.ReplaceAllString(id, "")
}

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

// DB returns the underlying database connection for direct queries.
func (s *BuildService) DB() *sql.DB {
	return s.db
}

func (s *BuildService) getCacheKey(projectID, filesHash string) string {
	projectID = safeID(projectID)
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
	projectID = safeID(projectID)
	cacheDir := filepath.Join(s.cfg.StoragePath, "build-cache", projectID)
	return os.RemoveAll(cacheDir)
}

// GetBuildCacheStatus returns cache statistics for a project.
func (s *BuildService) GetBuildCacheStatus(ctx context.Context, projectID string) (map[string]interface{}, error) {
	projectID = safeID(projectID)
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
		`UPDATE build_tasks SET status=?, log=log || ?, updated_at=datetime('now') WHERE id=?`,
		domain.BuildFailed, log, taskID)
}

// ReleaseInfo contains information about a GitHub Release
type ReleaseInfo struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	HTMLURL    string `json:"html_url"`
	UploadURL  string `json:"upload_url"`
}

// PublishToRelease publishes a build artifact to GitHub Release
func (s *BuildService) PublishToRelease(ctx context.Context, projectID, buildID, token string) (*ReleaseInfo, error) {
	// Get build task
	task, err := s.Get(ctx, buildID)
	if err != nil {
		return nil, fmt.Errorf("build not found: %w", err)
	}

	// Verify build belongs to project
	if task.ProjectID != projectID {
		return nil, fmt.Errorf("build does not belong to project")
	}

	// Check if build succeeded
	if task.Status != domain.BuildSuccess {
		return nil, fmt.Errorf("build did not succeed, status: %s", task.Status)
	}

	// Get artifact path
	if task.ArtifactPath == nil || *task.ArtifactPath == "" {
		return nil, fmt.Errorf("no artifact path found")
	}

	// Get project info for module name
	var projectName string
	err = s.db.QueryRowContext(ctx,
		`SELECT name FROM projects WHERE id=? AND deleted_at IS NULL`, projectID).Scan(&projectName)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	// Get version from module.prop if exists
	version := "1.0.0"
	var modulePropContent string
	err = s.db.QueryRowContext(ctx,
		`SELECT content FROM project_files WHERE project_id=? AND path='module.prop'`, projectID).Scan(&modulePropContent)
	if err == nil {
		// Parse version from module.prop
		for _, line := range strings.Split(modulePropContent, "\n") {
			if strings.HasPrefix(line, "version=") {
				version = strings.TrimPrefix(line, "version=")
				version = strings.TrimSpace(version)
				break
			}
		}
	}

	// Create release tag
	tagName := fmt.Sprintf("v%s", version)
	releaseName := fmt.Sprintf("%s v%s", projectName, version)
	releaseBody := fmt.Sprintf("## %s v%s\n\nAutomated release from ModuForge build.\n\n**Build ID:** %s\n**Architecture:** arm64\n**Build Time:** %s",
		projectName, version, buildID, task.CreatedAt)

	// GitHub API call to create release
	releaseInfo, err := s.createGitHubRelease(ctx, token, tagName, releaseName, releaseBody, *task.ArtifactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub release: %w", err)
	}

	return releaseInfo, nil
}

// createGitHubRelease makes the actual GitHub API call
func (s *BuildService) createGitHubRelease(ctx context.Context, token, tagName, name, body, artifactPath string) (*ReleaseInfo, error) {
	// Parse git remote URL to get owner/repo
	owner, repo, err := s.parseGitRemote()
	if err != nil {
		return nil, fmt.Errorf("failed to parse git remote: %w", err)
	}

	// Create release payload
	releasePayload := map[string]interface{}{
		"tag_name":   tagName,
		"name":       name,
		"body":       body,
		"draft":      false,
		"prerelease": false,
	}

	payloadBytes, err := json.Marshal(releasePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal release payload: %w", err)
	}

	// Create release via GitHub API
	releaseURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", owner, repo)
	req, err := http.NewRequestWithContext(ctx, "POST", releaseURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var releaseResp struct {
		ID       int64  `json:"id"`
		TagName  string `json:"tag_name"`
		Name     string `json:"name"`
		HTMLURL  string `json:"html_url"`
		UploadURL string `json:"upload_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&releaseResp); err != nil {
		return nil, fmt.Errorf("failed to decode release response: %w", err)
	}

	// Upload artifact if it exists
	if artifactPath != "" {
		assetName := filepath.Base(artifactPath)
		if err := s.uploadReleaseAsset(ctx, token, releaseResp.UploadURL, assetName, artifactPath); err != nil {
			// Log warning but don't fail - release was created
			log.Printf("Warning: failed to upload artifact: %v", err)
		}
	}

	return &ReleaseInfo{
		TagName:   releaseResp.TagName,
		Name:      releaseResp.Name,
		Body:      body,
		Draft:     false,
		Prerelease: false,
		HTMLURL:   releaseResp.HTMLURL,
		UploadURL: releaseResp.UploadURL,
	}, nil
}

// parseGitRemote parses the git remote URL to extract owner and repo
func (s *BuildService) parseGitRemote() (owner, repo string, err error) {
	// Try to get remote URL from git command
	cmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to get git remote URL: %w", err)
	}

	remoteURL := strings.TrimSpace(string(output))

	// Parse SSH URL: git@github.com:owner/repo.git
	if strings.HasPrefix(remoteURL, "git@") {
		// Remove git@ prefix and .git suffix
		remoteURL = strings.TrimPrefix(remoteURL, "git@")
		remoteURL = strings.TrimSuffix(remoteURL, ".git")
		// Replace : with /
		remoteURL = strings.Replace(remoteURL, ":", "/", 1)
	} else if strings.HasPrefix(remoteURL, "https://") {
		// Parse HTTPS URL: https://github.com/owner/repo.git
		remoteURL = strings.TrimPrefix(remoteURL, "https://")
		remoteURL = strings.TrimSuffix(remoteURL, ".git")
		// Remove any authentication info
		if idx := strings.Index(remoteURL, "@"); idx != -1 {
			remoteURL = remoteURL[idx+1:]
		}
	} else {
		return "", "", fmt.Errorf("unsupported remote URL format: %s", remoteURL)
	}

	// Split into owner/repo
	parts := strings.Split(remoteURL, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid remote URL format: %s", remoteURL)
	}

	// Handle case where github.com might be included
	if len(parts) >= 3 && parts[0] == "github.com" {
		owner = parts[1]
		repo = parts[2]
	} else if len(parts) == 2 {
		owner = parts[0]
		repo = parts[1]
	} else {
		return "", "", fmt.Errorf("cannot parse owner/repo from: %s", remoteURL)
	}

	return owner, repo, nil
}

// uploadReleaseAsset uploads a file to a GitHub release
func (s *BuildService) uploadReleaseAsset(ctx context.Context, token, uploadURL, fileName, filePath string) error {
	// Read the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info for size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Replace {name} placeholder in upload URL
	uploadURL = strings.Replace(uploadURL, "{name}", fileName, 1)
	uploadURL = strings.Replace(uploadURL, "{label}", "", 1)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, file)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Content-Type", "application/zip")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// Execute request
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
