package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// shellSyntaxIssue represents a single shell syntax warning.
type shellSyntaxIssue struct {
	Script string
	Line   int
	Msg    string
}

func (i shellSyntaxIssue) String() string {
	return fmt.Sprintf("%s:%d: %s", i.Script, i.Line, i.Msg)
}

// ShellSyntaxCheck performs basic syntax checks on a shell script.
// Returns issues found (warnings, not errors). Each issue includes the
// script path, line number, and a human-readable description.
func ShellSyntaxCheck(scriptPath string) []shellSyntaxIssue {
	var issues []shellSyntaxIssue

	content, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(content), "\n")
	name := filepath.Base(scriptPath)

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check: case without variable — "case in" instead of 'case "$VAR" in'
		if trimmed == "case in" || strings.HasPrefix(trimmed, "case in ") {
			issues = append(issues, shellSyntaxIssue{
				Script: name, Line: lineNum,
				Msg: "'case in' missing variable (should be 'case \"$VAR\" in')",
			})
		}

		// Check: incomplete set_perm — ends with permission digit but missing args
		if strings.Contains(trimmed, "set_perm ") && strings.HasSuffix(trimmed, " 0") {
			issues = append(issues, shellSyntaxIssue{
				Script: name, Line: lineNum,
				Msg: "set_perm appears incomplete (missing permission digits)",
			})
		}

		// Check: sleep without argument
		if trimmed == "sleep" {
			issues = append(issues, shellSyntaxIssue{
				Script: name, Line: lineNum,
				Msg: "sleep without argument",
			})
		}

		// Check: kill -9 missing PID — no $variable or number after "-9"
		if strings.Contains(trimmed, "kill -9") {
			fields := strings.Fields(trimmed)
			// "kill -9" should have at least 3 fields: kill, -9, PID
			if len(fields) < 3 {
				issues = append(issues, shellSyntaxIssue{
					Script: name, Line: lineNum,
					Msg: "kill -9 missing PID argument",
				})
			}
		}

		// Check: double echo — "echo echo"
		if strings.Contains(trimmed, "echo echo") {
			issues = append(issues, shellSyntaxIssue{
				Script: name, Line: lineNum,
				Msg: "double 'echo echo' (likely copy-paste error)",
			})
		}

		// Check: bare "return" without value in function body
		if trimmed == "return" {
			issues = append(issues, shellSyntaxIssue{
				Script: name, Line: lineNum,
				Msg: "'return' without value (use 'return 0' or 'return 1')",
			})
		}
	}

	return issues
}

// ValidateShellScripts runs ShellSyntaxCheck on common Magisk module scripts
// in the given directory. Returns all issues found across all scripts.
func ValidateShellScripts(moduleDir string, logFn func(string)) []shellSyntaxIssue {
	scripts := []string{"customize.sh", "service.sh", "uninstall.sh"}
	var allIssues []shellSyntaxIssue

	for _, name := range scripts {
		path := filepath.Join(moduleDir, name)
		issues := ShellSyntaxCheck(path)
		allIssues = append(allIssues, issues...)
	}

	if logFn != nil {
		for _, issue := range allIssues {
			logFn("  " + issue.String() + "\n")
		}
	}
	return allIssues
}
