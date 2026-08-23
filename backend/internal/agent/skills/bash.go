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

// validateCommand checks command against security rules (whitelist + blacklist)
func (s *BashSkill) validateCommand(command string) error {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return fmt.Errorf("empty command")
	}

	// ── Blacklist: dangerous patterns that are NEVER allowed ──
	dangerousPatterns := []struct {
		pattern string
		reason  string
	}{
		// Destructive filesystem
		{`rm\s+(-[a-zA-Z]*r[a-zA-Z]*f|-[a-zA-Z]*f[a-zA-Z]*r)\s+(/|/\*|\.\.)`, "recursive force delete root"},
		{`rm\s+(-[a-zA-Z]*r[a-zA-Z]*f|-[a-zA-Z]*f[a-zA-Z]*r)\s+/((usr|etc|var|sys|proc|dev|boot|sbin|bin)/)`, "delete system directories"},
		{`(mkfs|format)\s+(/dev/|//\\\\.\\\\)`, "format disk"},
		{`dd\s+if=/dev/(zero|random|urandom)\s+of=/dev/`, "overwrite disk"},
		{`chmod\s+(-R\s+)?777\s+/`, "chmod 777 root"},
		// Piping to shell / downloading+executing
		{`\|\s*(ba)?sh`, "pipe to shell"},
		{`\|\s*python\b.*-c`, "pipe to python"},
		{`curl\s+.*\|\s*(ba)?sh`, "download and execute"},
		{`wget\s+.*\|\s*(ba)?sh`, "download and execute"},
		{`curl\s+.*-o\s+\S+\s*&&\s*.*(ba)?sh`, "download then execute"},
		// Network backdoors
		{`nc\s+.*-e\s+/bin/(ba)?sh`, "netcat reverse shell"},
		{`/dev/tcp/`, "bash reverse shell"},
		{`/dev/udp/`, "bash udp shell"},
		// Fork bomb
		{`\(\)\s*\{.*\|.*&\s*\}`, "fork bomb"},
		// Disk overwrite
		{`>\s*/dev/sd[a-z]`, "overwrite disk device"},
		// Finding and deleting
		{`find\s+/\s+-exec\s+rm`, "find and delete from root"},
		// Sensitive file access
		{`cat\s+/etc/shadow`, "read shadow file"},
		{`cat\s+/etc/passwd.*\|`, "exfiltrate passwd"},
		// Kernel / system modification
		{`insmod\s+`, "load kernel module"},
		{`rmmod\s+`, "unload kernel module"},
		{`sysctl\s+.*-\sw`, "modify kernel sysctl"},
		// Env / credential theft
		{`env\b.*\|\s*(curl|wget)`, "exfiltrate env vars"},
		{`printenv.*\|\s*(curl|wget)`, "exfiltrate env vars"},
		{`cat\s+\.env\b.*\|\s*(curl|wget)`, "exfiltrate .env"},
	}

	for _, dp := range dangerousPatterns {
		if matched, _ := regexp.MatchString(dp.pattern, cmd); matched {
			return fmt.Errorf("🚫 命令被安全策略阻止: %s", dp.reason)
		}
	}

	// ── Root-level destructive operations ──
	blockedRootDirs := []string{
		"/bin", "/sbin", "/usr", "/etc", "/var", "/sys", "/proc",
		"/dev", "/boot", "/lib", "/lib64", "/opt", "/srv",
	}
	for _, dir := range blockedRootDirs {
		if matched, _ := regexp.MatchString(
			fmt.Sprintf(`\b(rm|mv|chmod|chown|shred|rename)\b.*\b%s\b`, regexp.QuoteMeta(dir)),
			cmd,
		); matched {
			return fmt.Errorf("🚫 命令被安全策略阻止: 操作系统目录 %s", dir)
		}
	}

	// ── Whitelist: only safe command prefixes are allowed ──
	// Extract the first command (handle pipes, &&, ||)
	firstCmd := extractFirstCommand(cmd)

	safePrefixes := []string{
		// Version control
		"git ", "git\t",
		// Package managers
		"npm ", "npx ", "yarn ", "pnpm ", "bun ",
		"pip ", "pip3 ", "uv ",
		"go ", "cargo ", "rustup ",
		"apt ", "apt-get ", "dpkg ",
		// Build & test
		"make ", "cmake ", "cargo ",
		"dotnet ", "msbuild ",
		// Project-specific safe commands
		"node ", "python ", "python3 ", "ruby ", "perl ", "php ",
		"java ", "javac ",
		// File inspection (read-only)
		"ls ", "cat ", "head ", "tail ", "wc ", "grep ", "rg ",
		"find ", "tree ", "stat ", "file ", "du ", "df ",
		"pwd", "which ", "whereis ", "type ", "echo ", "printf ",
		"diff ", "comm ", "sort ", "uniq ", "cut ", "awk ", "sed ",
		"tr ", "xargs ",
		// Git & project helpers
		"jq ", "yq ",
		// Network read-only
		"curl ", "wget ",
		// Archive
		"tar ", "unzip ", "zip ", "gzip ", "gunzip ",
		// Process
		"ps ", "top ", "htop ", "uptime ", "date ", "env", "printenv",
		// Build system
		"flutter ", "dart ", "swift ", "xcodebuild ",
		"gradle ", "mvn ",
		// Directory navigation (always safe)
		"cd ", "pushd", "popd",
		// Safe filesystem operations (within project dir only)
		"mkdir ", "touch ", "cp ", "mv ", "ln ",
		"chmod ", "chown ",
		// Compiler / toolchain
		"gcc ", "g++ ", "clang ", "rustc ",
		"tsc ", "esbuild ", "vite ",
		"docker ", "docker-compose ",
		"podman ",
		// Safe dev tools
		"eslint ", "prettier ", "biome ",
		"jest ", "vitest ", "mocha ", "pytest ", "go test",
		"cargo test",
	}

	allowed := false
	for _, prefix := range safePrefixes {
		if strings.HasPrefix(firstCmd, prefix) || firstCmd == strings.TrimSuffix(prefix, " ") {
			allowed = true
			break
		}
	}

	// Also allow commands starting with ./
	if strings.HasPrefix(firstCmd, "./") {
		allowed = true
	}

	if !allowed {
		return fmt.Errorf("🚫 命令不在白名单中: '%s'。允许的命令前缀包括: git, npm, go, python, node, ls, cat, find, mkdir, cp, mv 等", firstCmd)
	}

	return nil
}

// extractFirstCommand extracts the first command from a pipeline/chain.
func extractFirstCommand(cmd string) string {
	// Remove leading whitespace
	cmd = strings.TrimSpace(cmd)

	// Split by &&, ||, |, ;
	separators := []string{" && ", " || ", " | ", "; "}
	for _, sep := range separators {
		if idx := strings.Index(cmd, sep); idx >= 0 {
			cmd = cmd[:idx]
		}
	}

	// Extract just the command name (first word)
	cmd = strings.TrimSpace(cmd)
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}

	return parts[0]
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
