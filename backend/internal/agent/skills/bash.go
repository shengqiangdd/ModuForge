package skills

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type BashSkill struct {
	projectPath string
	db          *sql.DB
}

func NewBashSkill(projectPath string) *BashSkill {
	return &BashSkill{projectPath: projectPath}
}

func NewBashSkillWithDB(projectPath string, db *sql.DB) *BashSkill {
	return &BashSkill{projectPath: projectPath, db: db}
}

func (s *BashSkill) Name() string {
	return "bash"
}

func (s *BashSkill) Description() string {
	return `Execute a shell command in the project directory.
Input: {"command": "...", "project_id": "...", "timeout": 60 (optional, seconds)}.
Returns stdout+stderr. Useful for build, test, git, and any CLI operations.
WARNING: This runs arbitrary commands — be careful with destructive operations.`
}

func (s *BashSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	command, _ := input["command"].(string)
	projectID, _ := input["project_id"].(string)
	timeoutSec := 60.0
	if v, ok := input["timeout"].(float64); ok && v > 0 {
		timeoutSec = v
	}

	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	projectPath := s.resolvePath(projectID)

	// Ensure project files exist on disk (sync from DB if needed)
	if projectID != "" {
		if err := s.syncProjectToDisk(projectID, projectPath); err != nil {
			log.Printf("[BashSkill] sync warning: %v", err)
		}
	}

	// Create context with timeout
	timeout := time.Duration(timeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Run command in project directory
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = projectPath

	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Sprintf("Command timed out after %v:\n%s", timeout, outputStr), fmt.Errorf("command timed out")
	}

	if err != nil {
		return fmt.Sprintf("Command failed: %v\n\nOutput:\n%s", err, outputStr), err
	}

	if outputStr == "" {
		return "Command completed successfully (no output)", nil
	}

	return outputStr, nil
}

func (s *BashSkill) resolvePath(projectID string) string {
	if s.projectPath == "" || projectID == "" {
		return s.projectPath
	}
	return ResolveProjectPath(s.db, s.projectPath, projectID)
}

// syncProjectToDisk ensures all project files from the database exist on disk.
// This is needed because bash runs commands on the filesystem, but files may
// only exist in the database (e.g., after write_file in a read-only container).
func (s *BashSkill) syncProjectToDisk(projectID, projectDir string) error {
	if s.db == nil || projectID == "" {
		return nil
	}

	// Check if directory already has files — skip sync if so
	entries, err := os.ReadDir(projectDir)
	if err == nil && len(entries) > 0 {
		return nil // directory already populated
	}

	// Read all files from database
	rows, err := s.db.Query(
		`SELECT path, content FROM project_files WHERE project_id=?`, projectID,
	)
	if err != nil {
		return fmt.Errorf("failed to query project files: %w", err)
	}
	defer rows.Close()

	synced := 0
	for rows.Next() {
		var path, content string
		if err := rows.Scan(&path, &content); err != nil {
			continue
		}
		fullPath := filepath.Join(projectDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("[BashSkill] mkdir failed for %s: %v", dir, err)
			continue
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			log.Printf("[BashSkill] write failed for %s: %v", fullPath, err)
			continue
		}
		synced++
	}
	if synced > 0 {
		log.Printf("[BashSkill] synced %d files from DB to disk for project %s", synced, projectID)
	}
	return nil
}

func (s *BashSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  false,
		Essential: false,
		Core:      true,
		NeedsDB:   false,
		NeedsLLM:  false,
	}
}
