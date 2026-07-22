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

type GitManagerService struct {
	projectsDir string
}

func NewGitManagerService(projectsDir string) *GitManagerService {
	return &GitManagerService{projectsDir: projectsDir}
}

func (s *GitManagerService) projectDir(projectID string) string {
	return filepath.Join(s.projectsDir, projectID)
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
