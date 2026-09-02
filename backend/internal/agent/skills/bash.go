package skills

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/moduforge/backend/internal/agent/registry"
	"github.com/moduforge/backend/internal/storage"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SecurityChecker interface for command validation.
// Implemented by agent.SecurityEngine; keeps skills decoupled from agent internals.
type SecurityChecker interface {
	ValidateCommand(command string) (level int, riskScore int, message string)
}

type BashSkill struct {
	projectPath    string
	db             *sql.DB
	securityEngine SecurityChecker        // typed interface instead of interface{}
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
func NewBashSkillWithSecurity(projectPath string, db *sql.DB, securityEngine SecurityChecker) *BashSkill {
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

	// Normalize multi-line commands: replace newlines with spaces for validation
	// This allows shell scripts with for/while/if loops to pass validation
	cmd = strings.ReplaceAll(cmd, "\n", " ")
	cmd = strings.ReplaceAll(cmd, "\r", " ")
	// Collapse multiple spaces
	for strings.Contains(cmd, "  ") {
		cmd = strings.ReplaceAll(cmd, "  ", " ")
	}
	cmd = strings.TrimSpace(cmd)

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
	// Split into sub-commands and check EVERY one (prevents "ls; rm -rf /" bypass)
	// Special case: if the command looks like a shell script (contains shell patterns),
	// allow it without strict whitelist checking. This enables Magisk module development.
	shellPatterns := []string{
		"for ", "while ", "if ", "case ", "function ",
		"do\n", "done\n", "then\n", "else\n", "fi\n", "esac\n",
		"#!/", "#!/bin/sh", "#!/system/bin/sh",
		"MODDIR=", "MODPATH=", "ui_print",
	}
	for _, pattern := range shellPatterns {
		if strings.Contains(cmd, pattern) {
			// Allow shell scripts but still check blacklist
			for _, dp := range dangerousPatterns {
				if matched, _ := regexp.MatchString(dp.pattern, cmd); matched {
					return fmt.Errorf("🚫 命令被安全策略阻止: %s", dp.reason)
				}
			}
			return nil
		}
	}

	subCmds := splitSubCommands(cmd)
	for _, sub := range subCmds {
		sub = strings.TrimSpace(sub)
		if sub == "" {
			continue
		}
		firstCmd := extractFirstCommand(sub)
		if firstCmd == "" {
			continue
		}
		// If pipe command (contains |), extract part before pipe
		if pipeIdx := strings.Index(firstCmd, "|"); pipeIdx > 0 {
			firstCmd = strings.TrimSpace(firstCmd[:pipeIdx])
		}
		if !isAllowedPrefix(firstCmd) && !strings.HasPrefix(firstCmd, "./") {
			return fmt.Errorf("🚫 子命令不在白名单中: '%s'", firstCmd)
		}
	}

	// ── SecurityEngine integration: delegate to the centralized policy engine ──
	if s.securityEngine != nil {
		level, riskScore, msg := s.securityEngine.ValidateCommand(command)
		if level == 2 { // SecurityDeny
			if msg == "" {
				msg = fmt.Sprintf("安全引擎拒绝 (risk: %d)", riskScore)
			}
			return fmt.Errorf("🚫 %s", msg)
		}
	}

	return nil
}

// safePrefixes is the whitelist of allowed command prefixes.
var safePrefixes = []string{
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
	// Shell (safe read-only / syntax check)
	"bash -n ", "bash --no-exec ", "sh -n ",
	"shellcheck ",
	// Shell built-ins (safe when used in scripts)
	"for ", "while ", "if ", "case ", "function ",
	"do", "done", "then", "else", "elif", "fi", "esac",
	"echo ", "printf ", "read ", "test ", "[", "[[",
	"source ", ". ",
	"exit ", "return ", "break ", "continue ",
	"export ", "local ", "declare ", "typeset ",
	"set ", "unset ", "shift ",
	"trap ", "wait ", "exec ",
	// Magisk module tools
	"unzip -t ",
	"ui_print ",
	"set_perm ", "set_perm_recursive ",
	"mkdir -p ",
	"cp -r ",
	"rm -rf ",
	"chcon ",
}

// isAllowedPrefix checks if a command name is in the whitelist.
func isAllowedPrefix(cmdName string) bool {
	for _, prefix := range safePrefixes {
		if strings.HasPrefix(cmdName, prefix) || cmdName == strings.TrimSuffix(prefix, " ") {
			return true
		}
	}
	return false
}

// splitSubCommands splits a compound command into individual sub-commands.
// Separators are split by priority: && > || > ;
func splitSubCommands(cmd string) []string {
	var result []string
	for _, sep := range []string{" && ", " || ", "; "} {
		parts := strings.Split(cmd, sep)
		if len(parts) > 1 {
			for _, p := range parts {
				result = append(result, splitSubCommands(strings.TrimSpace(p))...)
			}
			return result
		}
	}
	return []string{cmd}
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
