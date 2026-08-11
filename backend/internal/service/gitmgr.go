package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type CommitInfo struct {
	Hash      string    `json:"hash"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Timestamp time.Time `json:"timestamp"`
}

// PushOptions contains options for optimized git push
type PushOptions struct {
	IncludePatterns []string `json:"include_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
	CommitMessage   string   `json:"commit_message"`
	DryRun          bool     `json:"dry_run"`
}

// Default exclusion patterns for git push
var defaultExcludePatterns = []string{
	"*.log",
	"*.tmp",
	"*.cache",
	"node_modules/",
	"__pycache__/",
	".env",
	".env.local",
	"build/",
	"dist/",
	"*.zip",
	"*.tar.gz",
	".DS_Store",
	"Thumbs.db",
	".git/",
	"*.exe",
	"*.dll",
	"*.so",
	"*.dylib",
}

type GitManagerService struct {
	projectsDir string
}

func NewGitManagerService(projectsDir string) *GitManagerService {
	return &GitManagerService{projectsDir: projectsDir}
}

func (s *GitManagerService) projectDir(projectID string) string {
	cleanID := filepath.Clean(projectID)
	if cleanID == "." || cleanID == ".." || strings.Contains(cleanID, "..") {
		return filepath.Join(s.projectsDir, "invalid")
	}
	dir := filepath.Join(s.projectsDir, cleanID)
	sp := string(filepath.Separator)
	if !(dir == s.projectsDir || strings.HasPrefix(dir, s.projectsDir+sp)) {
		return filepath.Join(s.projectsDir, "invalid")
	}
	return dir
}

func (s *GitManagerService) InitRepo(ctx context.Context, projectID string) error {
	dir := s.projectDir(projectID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "ModuForge"},
		{"git", "config", "user.email", "moduforge@local"},
	}
	for _, args := range cmds {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s failed: %v", strings.Join(args, " "), string(out))
		}
	}
	return nil
}

func (s *GitManagerService) AddAndCommit(ctx context.Context, projectID, message string) (*CommitInfo, error) {
	dir := s.projectDir(projectID)
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		if err := s.InitRepo(ctx, projectID); err != nil {
			return nil, err
		}
	}
	cmds := [][]string{
		{"git", "add", "-A"},
		{"git", "commit", "-m", message},
	}
	for _, args := range cmds {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("%s failed: %v", strings.Join(args, " "), string(out))
		}
	}
	return s.GetCurrentHash(ctx, projectID)
}

func (s *GitManagerService) ListCommits(ctx context.Context, projectID string, limit int) ([]CommitInfo, error) {
	dir := s.projectDir(projectID)
	if limit <= 0 {
		limit = 20
	}
	cmd := exec.CommandContext(ctx, "git", "log",
		fmt.Sprintf("-%d", limit),
		"--format=%H|%s|%an|%aI")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %v", string(out))
	}
	var commits []CommitInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		ts, _ := time.Parse(time.RFC3339, parts[3])
		commits = append(commits, CommitInfo{
			Hash:      parts[0],
			Message:   parts[1],
			Author:    parts[2],
			Timestamp: ts,
		})
	}
	return commits, nil
}

func (s *GitManagerService) GetDiff(ctx context.Context, projectID, hash string) (string, error) {
	dir := s.projectDir(projectID)
	cmd := exec.CommandContext(ctx, "git", "show", hash, "--stat", "--format=commit: %H%nauthor: %an%ndate: %aI%nmessage: %s%n")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git show failed: %v", string(out))
	}
	return string(out), nil
}

func (s *GitManagerService) CheckoutVersion(ctx context.Context, projectID, hash string) error {
	dir := s.projectDir(projectID)
	cmd := exec.CommandContext(ctx, "git", "checkout", hash, "--", ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout failed: %v", string(out))
	}
	return nil
}

func (s *GitManagerService) GetCurrentHash(ctx context.Context, projectID string) (*CommitInfo, error) {
	dir := s.projectDir(projectID)
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = dir
	hashOut, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git rev-parse failed: %v", string(hashOut))
	}
	hash := strings.TrimSpace(string(hashOut))

	logCmd := exec.CommandContext(ctx, "git", "log", "-1", "--format=%s|%an|%aI")
	logCmd.Dir = dir
	logOut, err := logCmd.CombinedOutput()
	if err != nil {
		return &CommitInfo{Hash: hash}, nil
	}
	parts := strings.SplitN(strings.TrimSpace(string(logOut)), "|", 3)
	ts, _ := time.Parse(time.RFC3339, parts[2])
	info := &CommitInfo{
		Hash:      hash,
		Timestamp: ts,
	}
	if len(parts) > 0 {
		info.Message = parts[0]
	}
	if len(parts) > 1 {
		info.Author = parts[1]
	}
	return info, nil
}

// ===== Branch Management =====

type BranchInfo struct {
	Name      string `json:"name"`
	IsCurrent bool   `json:"is_current"`
	Hash      string `json:"hash"`
}

func (s *GitManagerService) ListBranches(ctx context.Context, projectID string) ([]BranchInfo, error) {
	dir := s.projectDir(projectID)
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		return []BranchInfo{}, nil
	}

	cmd := exec.CommandContext(ctx, "git", "branch", "-a", "--format=%(refname:short)|%(HEAD)")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git branch failed: %v", string(out))
	}

	var branches []BranchInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		name := strings.TrimSpace(parts[0])
		isCurrent := len(parts) > 1 && strings.TrimSpace(parts[1]) == "*"

		// Get hash for each branch
		hashCmd := exec.CommandContext(ctx, "git", "rev-parse", "refs/heads/"+name)
		hashCmd.Dir = dir
		hashOut, _ := hashCmd.CombinedOutput()
		hash := strings.TrimSpace(string(hashOut))

		branches = append(branches, BranchInfo{
			Name:      name,
			IsCurrent: isCurrent,
			Hash:      hash,
		})
	}

	if branches == nil {
		branches = []BranchInfo{}
	}
	return branches, nil
}

func (s *GitManagerService) CreateBranch(ctx context.Context, projectID, branchName string) error {
	dir := s.projectDir(projectID)
	cmd := exec.CommandContext(ctx, "git", "branch", branchName)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git branch failed: %v", string(out))
	}
	return nil
}

func (s *GitManagerService) CheckoutBranch(ctx context.Context, projectID, branchName string) error {
	dir := s.projectDir(projectID)
	cmd := exec.CommandContext(ctx, "git", "checkout", branchName)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout failed: %v", string(out))
	}
	return nil
}

func (s *GitManagerService) GetCurrentBranch(ctx context.Context, projectID string) (string, error) {
	dir := s.projectDir(projectID)
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git branch failed: %v", string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func (s *GitManagerService) Push(ctx context.Context, projectID, remote, branch string) (string, error) {
	return s.PushWithToken(ctx, projectID, remote, branch, "")
}

// PushWithToken pushes to a remote with optional authentication token.
// If token is provided, the remote URL is temporarily rewritten to include the token
// (GitHub format: https://TOKEN@github.com/user/repo.git), then restored after push.
func (s *GitManagerService) PushWithToken(ctx context.Context, projectID, remote, branch, token string) (string, error) {
	dir := s.projectDir(projectID)
	if remote == "" {
		remote = "origin"
	}
	if branch == "" {
		b, err := s.GetCurrentBranch(ctx, projectID)
		if err != nil {
			return "", err
		}
		branch = b
	}

	// If token is provided, inject it into the remote URL
	var originalURL string
	if token != "" {
		// Get current remote URL
		getURL := exec.CommandContext(ctx, "git", "remote", "get-url", remote)
		getURL.Dir = dir
		out, err := getURL.CombinedOutput()
		if err != nil {
			return string(out), fmt.Errorf("failed to get remote URL: %v", string(out))
		}
		originalURL = strings.TrimSpace(string(out))

		// Inject token: https://github.com/user/repo.git -> https://TOKEN@github.com/user/repo.git
		tokenURL := injectToken(originalURL, token)
		setURL := exec.CommandContext(ctx, "git", "remote", "set-url", remote, tokenURL)
		setURL.Dir = dir
		if out, err := setURL.CombinedOutput(); err != nil {
			return string(out), fmt.Errorf("failed to set remote URL: %v", string(out))
		}

		// Restore original URL after push (defer)
		defer func() {
			restore := exec.CommandContext(ctx, "git", "remote", "set-url", remote, originalURL)
			restore.Dir = dir
			restore.Run()
		}()
	}

	cmd := exec.CommandContext(ctx, "git", "push", remote, branch)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git push failed: %v", string(out))
	}
	return string(out), nil
}

// injectToken injects a token into a git remote URL for authentication.
// Supports HTTPS URLs (GitHub/GitLab/Bitbucket format).
func injectToken(url, token string) string {
	// https://github.com/user/repo.git -> https://TOKEN@github.com/user/repo.git
	// https://gitlab.com/user/repo.git -> https://oauth2:TOKEN@gitlab.com/user/repo.git
	if strings.HasPrefix(url, "https://") {
		host := url[8:]
		// Determine if it's GitHub (use TOKEN directly) or other (use oauth2:TOKEN prefix)
		if strings.Contains(host, "github.com") {
			return "https://" + token + "@" + host
		}
		// GitLab, Bitbucket, etc. use oauth2:TOKEN format
		return "https://oauth2:" + token + "@" + host
	}
	// SSH URLs - can't inject token, return as-is
	return url
}

func (s *GitManagerService) Pull(ctx context.Context, projectID, remote, branch string) (string, error) {
	dir := s.projectDir(projectID)
	if remote == "" {
		remote = "origin"
	}
	if branch == "" {
		b, err := s.GetCurrentBranch(ctx, projectID)
		if err != nil {
			return "", err
		}
		branch = b
	}
	cmd := exec.CommandContext(ctx, "git", "pull", remote, branch)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git pull failed: %v", string(out))
	}
	return string(out), nil
}

// PushWithOptions pushes to a remote with advanced options including file filtering
func (s *GitManagerService) PushWithOptions(ctx context.Context, projectID, remote, branch, token string, opts PushOptions) (string, error) {
	dir := s.projectDir(projectID)
	if remote == "" {
		remote = "origin"
	}
	if branch == "" {
		b, err := s.GetCurrentBranch(ctx, projectID)
		if err != nil {
			return "", err
		}
		branch = b
	}

	// Merge default exclude patterns with user-provided patterns
	excludePatterns := append(defaultExcludePatterns, opts.ExcludePatterns...)

	// Create .gitignore file with exclusion patterns
	gitignorePath := filepath.Join(dir, ".gitignore")
	if err := s.createGitignore(gitignorePath, excludePatterns); err != nil {
		return "", fmt.Errorf("failed to create .gitignore: %v", err)
	}

	// If token is provided, inject it into the remote URL
	var originalURL string
	if token != "" {
		// Get current remote URL
		getURL := exec.CommandContext(ctx, "git", "remote", "get-url", remote)
		getURL.Dir = dir
		out, err := getURL.CombinedOutput()
		if err != nil {
			return string(out), fmt.Errorf("failed to get remote URL: %v", string(out))
		}
		originalURL = strings.TrimSpace(string(out))

		// Inject token: https://github.com/user/repo.git -> https://TOKEN@github.com/user/repo.git
		tokenURL := injectToken(originalURL, token)
		setURL := exec.CommandContext(ctx, "git", "remote", "set-url", remote, tokenURL)
		setURL.Dir = dir
		if out, err := setURL.CombinedOutput(); err != nil {
			return string(out), fmt.Errorf("failed to set remote URL: %v", string(out))
		}

		// Restore original URL after push (defer)
		defer func() {
			restore := exec.CommandContext(ctx, "git", "remote", "set-url", remote, originalURL)
			restore.Dir = dir
			restore.Run()
		}()
	}

	// Add files with patterns
	if len(opts.IncludePatterns) > 0 {
		// Add specific files
		for _, pattern := range opts.IncludePatterns {
			cmd := exec.CommandContext(ctx, "git", "add", pattern)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				return string(out), fmt.Errorf("git add %s failed: %v", pattern, string(out))
			}
		}
	} else {
		// Add all files (respects .gitignore)
		cmd := exec.CommandContext(ctx, "git", "add", "-A")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return string(out), fmt.Errorf("git add failed: %v", string(out))
		}
	}

	// Check if there are changes to commit
	statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	statusCmd.Dir = dir
	statusOut, err := statusCmd.CombinedOutput()
	if err != nil {
		return string(statusOut), fmt.Errorf("git status failed: %v", string(statusOut))
	}

	// If no changes, skip commit and push
	if strings.TrimSpace(string(statusOut)) == "" {
		return "No changes to commit", nil
	}

	// Commit with custom message
	commitMsg := opts.CommitMessage
	if commitMsg == "" {
		commitMsg = "Auto-commit: " + time.Now().Format("2006-01-02 15:04:05")
	}

	commitCmd := exec.CommandContext(ctx, "git", "commit", "-m", commitMsg)
	commitCmd.Dir = dir
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return string(out), fmt.Errorf("git commit failed: %v", string(out))
	}

	// Dry run check
	if opts.DryRun {
		return "Dry run: changes committed but not pushed", nil
	}

	// Push to remote
	pushCmd := exec.CommandContext(ctx, "git", "push", remote, branch)
	pushCmd.Dir = dir
	out, err := pushCmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git push failed: %v", string(out))
	}

	return string(out), nil
}

// createGitignore creates or updates .gitignore file with exclusion patterns
func (s *GitManagerService) createGitignore(path string, patterns []string) error {
	// Read existing .gitignore if it exists
	var existingPatterns []string
	if data, err := os.ReadFile(path); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				existingPatterns = append(existingPatterns, line)
			}
		}
	}

	// Merge patterns (avoid duplicates)
	allPatterns := make(map[string]bool)
	for _, p := range existingPatterns {
		allPatterns[p] = true
	}
	for _, p := range patterns {
		allPatterns[p] = true
	}

	// Write .gitignore
	var content strings.Builder
	content.WriteString("# Auto-generated by ModuForge Git Manager\n")
	content.WriteString("# Optimized for module development\n\n")
	for pattern := range allPatterns {
		content.WriteString(pattern + "\n")
	}

	return os.WriteFile(path, []byte(content.String()), 0644)
}

// GetFilesToPush returns list of files that would be pushed (for preview)
func (s *GitManagerService) GetFilesToPush(ctx context.Context, projectID string, opts PushOptions) ([]string, error) {
	dir := s.projectDir(projectID)
	
	// Create temporary .gitignore
	excludePatterns := append(defaultExcludePatterns, opts.ExcludePatterns...)
	gitignorePath := filepath.Join(dir, ".gitignore.tmp")
	if err := s.createGitignore(gitignorePath, excludePatterns); err != nil {
		return nil, err
	}
	defer os.Remove(gitignorePath) // Clean up

	// Get list of files that would be added
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git ls-files failed: %v", string(out))
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}
