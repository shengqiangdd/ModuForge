package skills

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"github.com/moduforge/backend/internal/agent/registry"
	"github.com/moduforge/backend/internal/storage"
)

// SecurityChecker interface for command validation
type SecurityChecker interface {
	CheckCommand(command string) (level int, riskScore int, matchedRules []interface{})
}

type BashSkill struct {
	projectPath    string
	db             *sql.DB
	securityEngine interface{} // *agent.SecurityEngine (avoid circular import)
	storage        storage.StorageAdapter // optional S3 storage backend
}

func NewBashSkillWithDB(projectPath string, db *sql.DB) *BashSkill {
	return &BashSkill{projectPath: projectPath, db: db}
}

// WithStorage sets the S3 storage adapter. When set, files are loaded from S3.
func (s *BashSkill) WithStorage(st storage.StorageAdapter) *BashSkill {
	s.storage = st
	return s
}

// NewBashSkillWithSecurity creates a BashSkill with security engine
func NewBashSkillWithSecurity(projectPath string, db *sql.DB, securityEngine interface{}) *BashSkill {
	return &BashSkill{
		projectPath:    projectPath,
		db:             db,
		securityEngine: securityEngine,
	}
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

	// Security check - validate command before execution
	if err := s.validateCommand(command); err != nil {
		return "", err
	}

	projectPath := ResolveProjectPath(s.db, s.projectPath, projectID)

	// Auto-create project directory if it doesn't exist
	if projectPath != "" {
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			log.Printf("[BashSkill] mkdir failed for project dir %s: %v", projectPath, err)
		}
	}

	// Ensure project files exist on disk (sync from DB if needed)
	if projectID != "" {
		if err := s.syncProjectToDisk(ctx, projectID, projectPath); err != nil {
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

// validateCommand checks command against security rules
func (s *BashSkill) validateCommand(command string) error {
	// Basic dangerous command blocking (independent of security engine)
	dangerousPatterns := []string{
		`rm\s+(-[a-zA-Z]*r[a-zA-Z]*f|-[a-zA-Z]*f[a-zA-Z]*r)\s+(/|/\*|\.\.)`,
		`rm\s+(-[a-zA-Z]*r[a-zA-Z]*f|-[a-zA-Z]*f[a-zA-Z]*r)\s+/((usr|etc|var|sys|proc|dev|boot|sbin|bin)/)`,
		`(mkfs|format)\s+(/dev/|//\\\\.\\\\)`,
		`dd\s+if=/dev/(zero|random|urandom)\s+of=/dev/`,
		`chmod\s+(-R\s+)?777\s+/`,
	}

	for _, pattern := range dangerousPatterns {
		if matched, _ := regexp.MatchString(pattern, command); matched {
			return fmt.Errorf("🚫 命令被安全策略阻止: 检测到危险操作模式")
		}
	}

	return nil
}

// syncProjectToDisk ensures all project files from the database exist on disk.
// This is needed because bash runs commands on the filesystem, but files may
// only exist in the database (e.g., after write_file in a read-only container).
func (s *BashSkill) syncProjectToDisk(ctx context.Context, projectID, projectDir string) error {
	if s.db == nil || projectID == "" {
		return nil
	}

	// Read all files from database/S3
	rows, err := s.db.Query(
		`SELECT path FROM project_files WHERE project_id=?`, projectID,
	)
	if err != nil {
		return fmt.Errorf("failed to query project files: %w", err)
	}
	defer rows.Close()

	synced := 0
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			continue
		}
		content, err := readFileContent(ctx, s.storage, s.db, projectID, path)
		if err != nil {
			log.Printf("[BashSkill] read failed for %s: %v", path, err)
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
		log.Printf("[BashSkill] synced %d files from %s to disk for project %s", synced, storageLabel(s.storage), projectID)
	}
	return nil
}

func (s *BashSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		Core:      true,
		NeedsDB:   false,
		NeedsLLM:  false,
	}
}
