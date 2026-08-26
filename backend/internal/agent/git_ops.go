package agent

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// GitOps provides git version control operations.
type GitOps struct {
	mu         sync.Mutex
	repoDir    string
	branch     string
	lastCommit string
}

// GitCommit represents a git commit.
type GitCommit struct {
	Hash      string    `json:"hash"`
	ShortHash string    `json:"short_hash"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Date      time.Time `json:"date"`
	Branch    string    `json:"branch"`
}

// GitDiff represents a file diff.
type GitDiff struct {
	File      string `json:"file"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// GitStatus represents the working tree status.
type GitStatus struct {
	Branch    string    `json:"branch"`
	Staged    []GitDiff `json:"staged"`
	Unstaged  []GitDiff `json:"unstaged"`
	Untracked []string  `json:"untracked"`
	Clean     bool      `json:"clean"`
}

// NewGitOps creates a new git operations handler.
func NewGitOps(repoDir string) *GitOps {
	return &GitOps{
		repoDir: repoDir,
	}
}

// runGit executes a git command and returns output.
func (g *GitOps) runGit(args ...string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	cmd := exec.Command("git", args...)
	cmd.Dir = g.repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// IsRepo checks if the directory is a git repository.
func (g *GitOps) IsRepo() bool {
	_, err := g.runGit("rev-parse", "--is-inside-work-tree")
	return err == nil
}

// Init initializes a new git repository.
func (g *GitOps) Init() error {
	_, err := g.runGit("init")
	return err
}

// Status returns the working tree status.
func (g *GitOps) Status() (*GitStatus, error) {
	branch, err := g.runGit("branch", "--show-current")
	if err != nil {
		branch = "unknown"
	}

	output, err := g.runGit("status", "--porcelain")
	if err != nil {
		return nil, err
	}

	status := &GitStatus{
		Branch:    branch,
		Staged:    make([]GitDiff, 0),
		Unstaged:  make([]GitDiff, 0),
		Untracked: make([]string, 0),
	}

	if output == "" {
		status.Clean = true
		return status, nil
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		indexStatus := line[0]
		workStatus := line[1]
		file := strings.TrimSpace(line[3:])

		if indexStatus == '?' {
			status.Untracked = append(status.Untracked, file)
		} else {
			diff := GitDiff{
				File:   file,
				Status: g.mapStatus(indexStatus, workStatus),
			}
			if indexStatus != ' ' && indexStatus != '?' {
				status.Staged = append(status.Staged, diff)
			}
			if workStatus != ' ' && workStatus != '?' {
				status.Unstaged = append(status.Unstaged, diff)
			}
		}
	}

	return status, nil
}

// mapStatus maps git status codes to human-readable strings.
func (g *GitOps) mapStatus(index, work byte) string {
	switch {
	case index == 'A':
		return "added"
	case index == 'D' || work == 'D':
		return "deleted"
	case index == 'R':
		return "renamed"
	default:
		return "modified"
	}
}

// Diff returns the diff of unstaged changes.
func (g *GitOps) Diff() ([]GitDiff, error) {
	output, err := g.runGit("diff", "--stat")
	if err != nil {
		return nil, err
	}

	var diffs []GitDiff
	if output == "" {
		return diffs, nil
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, " ") {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 {
			diffs = append(diffs, GitDiff{
				File:   strings.TrimSpace(parts[0]),
				Status: "modified",
			})
		}
	}

	return diffs, nil
}

// DiffStaged returns the diff of staged changes.
func (g *GitOps) DiffStaged() ([]GitDiff, error) {
	output, err := g.runGit("diff", "--cached", "--stat")
	if err != nil {
		return nil, err
	}

	var diffs []GitDiff
	if output == "" {
		return diffs, nil
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, " ") {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 {
			diffs = append(diffs, GitDiff{
				File:   strings.TrimSpace(parts[0]),
				Status: "staged",
			})
		}
	}

	return diffs, nil
}

// Log returns recent commits.
func (g *GitOps) Log(n int) ([]GitCommit, error) {
	if n <= 0 {
		n = 10
	}

	format := "--pretty=format:%H|%h|%s|%an|%ai"
	output, err := g.runGit("log", fmt.Sprintf("-%d", n), format)
	if err != nil {
		return nil, err
	}

	var commits []GitCommit
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}

		date, _ := time.Parse("2006-01-02 15:04:05 -0700", parts[4])
		commits = append(commits, GitCommit{
			Hash:      parts[0],
			ShortHash: parts[1],
			Message:   parts[2],
			Author:    parts[3],
			Date:      date,
		})
	}

	return commits, nil
}

// Commit creates a new commit.
func (g *GitOps) Commit(message string, files []string) (string, error) {
	if len(files) > 0 {
		args := append([]string{"add"}, files...)
		if _, err := g.runGit(args...); err != nil {
			return "", fmt.Errorf("git add: %w", err)
		}
	}

	_, err := g.runGit("commit", "-m", message)
	if err != nil {
		return "", fmt.Errorf("git commit: %w", err)
	}

	hash, _ := g.runGit("rev-parse", "HEAD")
	g.lastCommit = hash

	return hash, nil
}

// Rollback reverts to a specific commit.
func (g *GitOps) Rollback(commitHash string, hard bool) error {
	if hard {
		_, err := g.runGit("reset", "--hard", commitHash)
		return err
	}
	_, err := g.runGit("reset", "--soft", commitHash)
	return err
}

// RollbackFile reverts a specific file to a commit.
func (g *GitOps) RollbackFile(commitHash, filePath string) error {
	_, err := g.runGit("checkout", commitHash, "--", filePath)
	return err
}

// CreateBranch creates a new branch.
func (g *GitOps) CreateBranch(name string) error {
	_, err := g.runGit("checkout", "-b", name)
	if err == nil {
		g.branch = name
	}
	return err
}

// SwitchBranch switches to an existing branch.
func (g *GitOps) SwitchBranch(name string) error {
	_, err := g.runGit("checkout", name)
	if err == nil {
		g.branch = name
	}
	return err
}

// ListBranches returns all local branches.
func (g *GitOps) ListBranches() ([]string, error) {
	output, err := g.runGit("branch", "--list")
	if err != nil {
		return nil, err
	}

	var branches []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// GetFileContent returns the content of a file at a specific commit.
func (g *GitOps) GetFileContent(commitHash, filePath string) (string, error) {
	output, err := g.runGit("show", commitHash+":"+filePath)
	if err != nil {
		return "", err
	}
	return output, nil
}

// GetFileHistory returns the commit history for a specific file.
func (g *GitOps) GetFileHistory(filePath string, n int) ([]GitCommit, error) {
	if n <= 0 {
		n = 10
	}

	format := "--pretty=format:%H|%h|%s|%an|%ai"
	output, err := g.runGit("log", fmt.Sprintf("-%d", n), format, "--", filePath)
	if err != nil {
		return nil, err
	}

	var commits []GitCommit
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}

		date, _ := time.Parse("2006-01-02 15:04:05 -0700", parts[4])
		commits = append(commits, GitCommit{
			Hash:      parts[0],
			ShortHash: parts[1],
			Message:   parts[2],
			Author:    parts[3],
			Date:      date,
		})
	}

	return commits, nil
}

// Stash stashes the current changes.
func (g *GitOps) Stash(message string) error {
	args := []string{"stash"}
	if message != "" {
		args = append(args, "push", "-m", message)
	}
	_, err := g.runGit(args...)
	return err
}

// StashPop applies the most recent stash.
func (g *GitOps) StashPop() error {
	_, err := g.runGit("stash", "pop")
	return err
}

// StashList returns all stashes.
func (g *GitOps) StashList() ([]string, error) {
	output, err := g.runGit("stash", "list")
	if err != nil {
		return nil, err
	}

	var stashes []string
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			stashes = append(stashes, line)
		}
	}
	return stashes, nil
}
