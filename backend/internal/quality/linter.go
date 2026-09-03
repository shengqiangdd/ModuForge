package quality

import (
	"fmt"
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
		// ─── NEW RULES ───
		{
			Name:        "go-package-declaration",
			Description: "Go files must start with package declaration",
			Severity:    LintError,
			Check:       ruleGoPackageDeclaration,
		},
		{
			Name:        "go-balanced-braces",
			Description: "Go files must have balanced braces",
			Severity:    LintError,
			Check:       ruleGoBalancedBraces,
		},
		{
			Name:        "go-no-empty-assignments",
			Description: "Go files must not have empty assignments (= ;)",
			Severity:    LintError,
			Check:       ruleGoEmptyAssignments,
		},
		{
			Name:        "go-no-unterminated-strings",
			Description: "Go files must not have unterminated string literals",
			Severity:    LintError,
			Check:       ruleGoUnterminatedStrings,
		},
		{
			Name:        "module-prop-required",
			Description: "module.prop must exist with required fields",
			Severity:    LintError,
			Check:       ruleModulePropRequired,
		},
		{
			Name:        "service-sh-root-check",
			Description: "service.sh should check root before running",
			Severity:    LintWarning,
			Check:       ruleServiceShRootCheck,
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

// ═══════════════════════════════════════════════════════
// NEW RULES
// ═══════════════════════════════════════════════════════

// ruleGoPackageDeclaration checks that Go files start with package declaration.
func ruleGoPackageDeclaration(path, content string) []LintIssue {
	if !strings.HasSuffix(path, ".go") {
		return nil
	}
	lines := strings.SplitN(content, "\n", 5)
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "package ") {
			return nil
		}
	}
	return []LintIssue{{
		File:     path,
		Rule:     "go-package-declaration",
		Severity: LintError,
		Message:  "Go file missing package declaration",
		Fix:      "Add 'package main' at the beginning of the file",
	}}
}

// ruleGoBalancedBraces checks that Go files have balanced braces.
func ruleGoBalancedBraces(path, content string) []LintIssue {
	if !strings.HasSuffix(path, ".go") {
		return nil
	}
	open := strings.Count(content, "{")
	close := strings.Count(content, "}")
	if open != close {
		return []LintIssue{{
			File:     path,
			Rule:     "go-balanced-braces",
			Severity: LintError,
			Message:  fmt.Sprintf("Unbalanced braces: %d open, %d close", open, close),
			Fix:      "Ensure all opening braces { have matching closing braces }",
		}}
	}
	return nil
}

// ruleGoEmptyAssignments checks for empty assignments like "= ;" or "=;".
func ruleGoEmptyAssignments(path, content string) []LintIssue {
	if !strings.HasSuffix(path, ".go") {
		return nil
	}
	var issues []LintIssue
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, "= ;") || strings.HasSuffix(trimmed, "=;") ||
			trimmed == "= " || trimmed == "=" {
			issues = append(issues, LintIssue{
				File:     path,
				Line:     i + 1,
				Rule:     "go-no-empty-assignments",
				Severity: LintError,
				Message:  "Empty assignment (no value after =)",
				Fix:      "Provide a value: x = 0, x = \"\", or x = false",
			})
		}
	}
	return issues
}

// ruleGoUnterminatedStrings checks for unterminated string literals.
func ruleGoUnterminatedStrings(path, content string) []LintIssue {
	if !strings.HasSuffix(path, ".go") {
		return nil
	}
	var issues []LintIssue
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		// Count unescaped quotes
		quoteCount := 0
		inBacktick := false
		for j, ch := range trimmed {
			if ch == '`' {
				inBacktick = !inBacktick
			} else if ch == '"' && !inBacktick {
				if j > 0 && trimmed[j-1] == '\\' {
					continue
				}
				quoteCount++
			}
		}
		if !inBacktick && quoteCount%2 == 1 {
			issues = append(issues, LintIssue{
				File:     path,
				Line:     i + 1,
				Rule:     "go-no-unterminated-strings",
				Severity: LintError,
				Message:  "Unterminated string literal (odd number of quotes)",
				Fix:      "Close the string with a matching double quote",
			})
		}
	}
	return issues
}

// ruleModulePropRequired checks that module.prop exists with required fields.
func ruleModulePropRequired(path, content string) []LintIssue {
	if !strings.HasSuffix(path, "module.prop") {
		return nil
	}
	requiredFields := []string{"id=", "name=", "version=", "versionCode="}
	var issues []LintIssue
	for _, field := range requiredFields {
		if !strings.Contains(content, field) {
			issues = append(issues, LintIssue{
				File:     path,
				Rule:     "module-prop-required",
				Severity: LintError,
				Message:  fmt.Sprintf("module.prop missing required field: %s", field),
				Fix:      fmt.Sprintf("Add: %s<value>", field),
			})
		}
	}
	return issues
}

// ruleServiceShRootCheck checks that service.sh verifies root access.
func ruleServiceShRootCheck(path, content string) []LintIssue {
	if !strings.HasSuffix(path, "service.sh") {
		return nil
	}
	if strings.Contains(content, "uid") || strings.Contains(content, "root") ||
		strings.Contains(content, "[ $EUID") || strings.Contains(content, "$(id") {
		return nil
	}
	return []LintIssue{{
		File:     path,
		Rule:     "service-sh-root-check",
		Severity: LintWarning,
		Message:  "service.sh does not check for root access",
		Fix:      "Add: [ $(id -u) -eq 0 ] || exit 1",
	}}
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
