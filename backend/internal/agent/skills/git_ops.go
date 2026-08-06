package skills

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GitOpsSkill provides version control integration
type GitOpsSkill struct {
	projectPath string
}

func NewGitOpsSkill(projectPath string) *GitOpsSkill {
	return &GitOpsSkill{projectPath: projectPath}
}

func (s *GitOpsSkill) Name() string { return "git_ops" }

func (s *GitOpsSkill) Description() string {
	return `Perform git operations. Input: {"action": "status|diff|commit|log|branch|stash", "message": "commit message", "files": ["file1", "file2"]}.
Use this for version control: commit changes, view history, manage branches. Always commit after build_module succeeds.`
}

func (s *GitOpsSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	action, _ := input["action"].(string)
	message, _ := input["message"].(string)
	files, _ := input["files"].([]interface{})

	if action == "" {
		return "", fmt.Errorf("action is required (status|diff|commit|log|branch|stash)")
	}

	switch strings.ToLower(action) {
	case "status":
		return s.gitStatus()
	case "diff":
		return s.gitDiff()
	case "commit":
		if message == "" {
			return "", fmt.Errorf("commit message is required")
		}
		return s.gitCommit(message, files)
	case "log":
		return s.gitLog()
	case "branch":
		return s.gitBranch()
	case "stash":
		return s.gitStash()
	case "init":
		return s.gitInit()
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (s *GitOpsSkill) runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.projectPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s failed: %w\nOutput: %s", strings.Join(args, " "), err, string(out))
	}
	return string(out), nil
}

func (s *GitOpsSkill) gitStatus() (string, error) {
	out, err := s.runGit("status", "--short")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("📋 Git Status:\n%s", out), nil
}

func (s *GitOpsSkill) gitDiff() (string, error) {
	out, err := s.runGit("diff", "--stat")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("📊 Git Diff:\n%s", out), nil
}

func (s *GitOpsSkill) gitCommit(message string, files []interface{}) (string, error) {
	// Add files
	if len(files) > 0 {
		args := []string{"add"}
		for _, f := range files {
			if file, ok := f.(string); ok {
				args = append(args, file)
			}
		}
		if _, err := s.runGit(args...); err != nil {
			return "", fmt.Errorf("git add failed: %w", err)
		}
	} else {
		// Add all modified files
		if _, err := s.runGit("add", "-A"); err != nil {
			return "", fmt.Errorf("git add failed: %w", err)
		}
	}

	// Commit
	out, err := s.runGit("commit", "-m", message)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("✅ Git Commit:\n%s", out), nil
}

func (s *GitOpsSkill) gitLog() (string, error) {
	out, err := s.runGit("log", "--oneline", "-10")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("📜 Recent Commits:\n%s", out), nil
}

func (s *GitOpsSkill) gitBranch() (string, error) {
	out, err := s.runGit("branch", "-v")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("🌿 Branches:\n%s", out), nil
}

func (s *GitOpsSkill) gitStash() (string, error) {
	out, err := s.runGit("stash", "push", "-m", "Agent auto-stash")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("📦 Stash:\n%s", out), nil
}

func (s *GitOpsSkill) gitInit() (string, error) {
	// Check if already a git repo
	_, err := s.runGit("rev-parse", "--is-inside-work-tree")
	if err == nil {
		return "⚠️ Already a git repository", nil
	}

	out, err := s.runGit("init")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("🚀 Git Init:\n%s", out), nil
}
