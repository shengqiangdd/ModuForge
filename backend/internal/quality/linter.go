package quality

import (
	"regexp"
	"strings"
)

// LintSeverity represents the severity of a lint issue.
type LintSeverity string

const (
	LintError   LintSeverity = "error"
	LintWarning LintSeverity = "warning"
)

// LintIssue represents a single lint problem.
type LintIssue struct {
	File     string       `json:"file"`
	Line     int          `json:"line,omitempty"`
	Rule     string       `json:"rule"`
	Severity LintSeverity `json:"severity"`
	Message  string       `json:"message"`
	Fix      string       `json:"fix,omitempty"`
}

// LintRule defines a custom lint rule.
type LintRule struct {
	Name        string
	Description string
	Severity    LintSeverity
	Check       func(file string, content string) []LintIssue
}

// MagiskLinter lints Magisk module code against built-in and custom rules.
type MagiskLinter struct {
	rules []LintRule
}

// NewMagiskLinter creates a linter with all built-in rules.
func NewMagiskLinter() *MagiskLinter {
	l := &MagiskLinter{}
	l.rules = append(l.rules, l.builtinRules()...)
	return l
}

// AddRule adds a custom lint rule.
func (l *MagiskLinter) AddRule(rule LintRule) {
	l.rules = append(l.rules, rule)
}

// Lint runs all rules against the given files.
func (l *MagiskLinter) Lint(files []GeneratedFile) []LintIssue {
	var allIssues []LintIssue

	for _, f := range files {
		for _, rule := range l.rules {
			issues := rule.Check(f.Path, f.Content)
			allIssues = append(allIssues, issues...)
		}
	}

	return allIssues
}

// GeneratedFile represents a generated source file.
type GeneratedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ═══════════════════════════════════════════════════════
// Built-in rules
// ═══════════════════════════════════════════════════════

func (l *MagiskLinter) builtinRules() []LintRule {
	return []LintRule{
		{
			Name:        "must-define-modpath",
			Description: "customize.sh must define MODPATH",
			Severity:    LintError,
			Check:       ruleMustDefineModpath,
		},
		{
			Name:        "set-perm-completeness",
			Description: "set_perm must have all 4 required parameters",
			Severity:    LintError,
			Check:       ruleSetPermCompleteness,
		},
		{
			Name:        "no-dangerous-rm",
			Description: "Prohibit rm -rf / and rm -rf /*",
			Severity:    LintError,
			Check:       ruleNoDangerousRM,
		},
		{
			Name:        "shell-shebang",
			Description: ".sh files must start with shebang",
			Severity:    LintWarning,
			Check:       ruleShellShebang,
		},
		{
			Name:        "variable-expansion",
			Description: "Shell variables must use ${VAR} format",
			Severity:    LintError,
			Check:       ruleVariableExpansion,
		},
		{
			Name:        "go-error-handling",
			Description: "Go code must handle error return values",
			Severity:    LintWarning,
			Check:       ruleGoErrorHandling,
		},
	}
}

// ruleMustDefineModpath checks that customize.sh defines MODPATH.
func ruleMustDefineModpath(path, content string) []LintIssue {
	if !strings.Contains(path, "customize.sh") {
		return nil
	}
	if strings.Contains(content, "MODPATH") {
		return nil
	}
	return []LintIssue{{
		File:     path,
		Rule:     "must-define-modpath",
		Severity: LintError,
		Message:  "customize.sh does not define MODPATH",
		Fix:      "Add: MODPATH=${0%/*}",
	}}
}

// ruleSetPermCompleteness checks set_perm has all required parameters.
func ruleSetPermCompleteness(path, content string) []LintIssue {
	if !strings.HasSuffix(path, ".sh") {
		return nil
	}

	var issues []LintIssue
	lines := strings.Split(content, "\n")

	re := regexp.MustCompile(`set_perm\s+`)
	for i, line := range lines {
		if !re.MatchString(line) {
			continue
		}

		// Count parameters after set_perm
		trimmed := strings.TrimSpace(line)
		idx := strings.Index(trimmed, "set_perm")
		if idx < 0 {
			continue
		}
		params := strings.Fields(trimmed[idx+len("set_perm"):])

		// set_perm requires at least 4 params: path owner group mode
		if len(params) < 4 {
			issues = append(issues, LintIssue{
				File:     path,
				Line:     i + 1,
				Rule:     "set-perm-completeness",
				Severity: LintError,
				Message:  "set_perm requires at least 4 parameters: path owner group mode",
				Fix:      "set_perm ${MODPATH}/bin/name 0 0 0755",
			})
		}
	}

	return issues
}

// ruleNoDangerousRM checks for dangerous rm commands.
func ruleNoDangerousRM(path, content string) []LintIssue {
	if !strings.HasSuffix(path, ".sh") {
		return nil
	}

	var issues []LintIssue
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.Contains(line, "rm -rf /") || strings.Contains(line, "rm -rf /*") {
			issues = append(issues, LintIssue{
				File:     path,
				Line:     i + 1,
				Rule:     "no-dangerous-rm",
				Severity: LintError,
				Message:  "dangerous rm -rf / detected",
				Fix:      "Only remove module-specific files: rm -rf ${MODPATH}",
			})
		}
	}

	return issues
}

// ruleShellShebang checks that .sh files have shebang.
func ruleShellShebang(path, content string) []LintIssue {
	if !strings.HasSuffix(path, ".sh") {
		return nil
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}

	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, "#!/") {
		return nil
	}

	return []LintIssue{{
		File:     path,
		Line:     1,
		Rule:     "shell-shebang",
		Severity: LintWarning,
		Message:  "missing shebang line",
		Fix:      "Add #!/system/bin/sh at the beginning",
	}}
}

// ruleVariableExpansion checks that shell variables use ${VAR} format.
func ruleVariableExpansion(path, content string) []LintIssue {
	if !strings.HasSuffix(path, ".sh") {
		return nil
	}

	var issues []LintIssue
	lines := strings.Split(content, "\n")

	re := regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)[^{(]`)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		matches := re.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			// Skip $$ (PID) and $() (command substitution handled separately)
			if m[1] == "$" {
				continue
			}
			issues = append(issues, LintIssue{
				File:     path,
				Line:     i + 1,
				Rule:     "variable-expansion",
				Severity: LintError,
				Message:  "use ${" + m[1] + "} instead of $" + m[1],
				Fix:      "Replace $" + m[1] + " with ${" + m[1] + "}",
			})
			break // One issue per line is enough
		}
	}

	return issues
}

// ruleGoErrorHandling checks that Go code handles errors.
func ruleGoErrorHandling(path, content string) []LintIssue {
	if !strings.HasSuffix(path, ".go") {
		return nil
	}

	var issues []LintIssue
	lines := strings.Split(content, "\n")

	// Pattern: funcCall() without error check
	errorFuncs := []string{"os.ReadFile", "os.WriteFile", "os.MkdirAll", "exec.Command"}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "if") {
			continue
		}

		for _, fn := range errorFuncs {
			if strings.Contains(trimmed, fn+"(") {
				// Check if error is handled (next lines contain if err != nil)
				if i+1 < len(lines) {
					end := i + 3
					if end > len(lines) {
						end = len(lines)
					}
					nextLines := strings.Join(lines[i:end], "\n")
					if !strings.Contains(nextLines, "err") {
						issues = append(issues, LintIssue{
							File:     path,
							Line:     i + 1,
							Rule:     "go-error-handling",
							Severity: LintWarning,
							Message:  fn + " returns error but it may not be handled",
							Fix:      "Add: if err != nil { return err }",
						})
					}
				}
			}
		}
	}

	return issues
}
