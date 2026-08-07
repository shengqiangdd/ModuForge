package skills

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// CodeReviewSkill provides automated code review capabilities
type CodeReviewSkill struct{}

func NewCodeReviewSkill() *CodeReviewSkill {
	return &CodeReviewSkill{}
}

func (s *CodeReviewSkill) Name() string { return "code_review" }

func (s *CodeReviewSkill) Description() string {
	return "Perform automated code review on a file. Input: {\"path\": \"...\", \"content\": \"...\", \"language\": \"go|rust|c++|shell|python|typescript\"}. Returns: security issues, code smells, performance concerns, best practice violations, and improvement suggestions. Use this BEFORE build_module to catch issues early."
}

type ReviewIssue struct {
	Severity    string `json:"severity"` // critical, warning, info
	Category    string `json:"category"` // security, performance, style, bug, complexity
	Line        int    `json:"line"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

func (s *CodeReviewSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	content, _ := input["content"].(string)
	language, _ := input["language"].(string)
	path, _ := input["path"].(string)

	if content == "" {
		return "", fmt.Errorf("content is required")
	}
	if language == "" {
		language = detectLanguage(path)
	}

	var issues []ReviewIssue

	// Language-specific reviews
	switch strings.ToLower(language) {
	case "go":
		issues = reviewGo(content)
	case "rust":
		issues = reviewRust(content)
	case "c++", "cpp":
		issues = reviewCpp(content)
	case "shell", "sh", "bash":
		issues = reviewShell(content)
	case "python":
		issues = reviewPython(content)
	case "typescript", "ts", "javascript", "js":
		issues = reviewTypeScript(content)
	default:
		issues = reviewGeneric(content)
	}

	// Universal checks (all languages)
	issues = append(issues, reviewUniversal(content)...)

	return formatReviewResult(path, language, issues), nil
}

// detectLanguage is already defined in pathutil.go

// === Go Review Rules ===

func reviewGo(content string) []ReviewIssue {
	var issues []ReviewIssue
	lines := strings.Split(content, "\n")

	// Security: SQL injection
	sqlPattern := regexp.MustCompile(`fmt\.Sprintf\(.*SELECT.*%s|fmt\.Sprintf\(.*INSERT.*%s|fmt\.Sprintf\(.*UPDATE.*%s|fmt\.Sprintf\(.*DELETE.*%s`)
	for i, line := range lines {
		if sqlPattern.MatchString(line) {
			issues = append(issues, ReviewIssue{
				Severity: "critical", Category: "security", Line: i + 1,
				Description: "Potential SQL injection via fmt.Sprintf",
				Suggestion:  "Use parameterized queries: db.Query(query, args...)",
			})
		}
	}

	// Security: Command injection
	cmdPattern := regexp.MustCompile(`exec\.Command\(.*\+|os\.exec\.Command\(.*\+`)
	for i, line := range lines {
		if cmdPattern.MatchString(line) {
			issues = append(issues, ReviewIssue{
				Severity: "critical", Category: "security", Line: i + 1,
				Description: "Potential command injection via string concatenation",
				Suggestion:  "Use exec.Command with separate arguments, not shell concatenation",
			})
		}
	}

	// Bug: Unchecked errors
	errPattern := regexp.MustCompile(`^\s+\w+\s*:=\s*[^=\n]+\n\s+[^_\n]`)
	for i, line := range lines {
		if i+1 < len(lines) {
			combined := line + "\n" + lines[i+1]
			if errPattern.MatchString(combined) && strings.Contains(lines[i], "err") && !strings.Contains(lines[i+1], "err") {
				issues = append(issues, ReviewIssue{
					Severity: "warning", Category: "bug", Line: i + 1,
					Description: "Error return value not checked",
					Suggestion:  "Always check error: if err != nil { return ..., err }",
				})
			}
		}
	}

	// Performance: Goroutine leaks
	goroutinePattern := regexp.MustCompile(`go\s+func\(`)
	goroutineCount := 0
	for i, line := range lines {
		if goroutinePattern.MatchString(line) {
			goroutineCount++
			if goroutineCount > 10 {
				issues = append(issues, ReviewIssue{
					Severity: "warning", Category: "performance", Line: i + 1,
					Description: fmt.Sprintf("High goroutine count (%d) in single function", goroutineCount),
					Suggestion:  "Consider using worker pool or context cancellation",
				})
			}
		}
	}

	// Complexity: Function length
	funcStart := -1
	funcName := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "func ") {
			funcStart = i
			funcName = trimmed
		}
		if funcStart >= 0 && (trimmed == "}" || trimmed == "})") {
			length := i - funcStart
			if length > 80 {
				issues = append(issues, ReviewIssue{
					Severity: "warning", Category: "complexity", Line: funcStart + 1,
					Description: fmt.Sprintf("Function '%s' is %d lines (max recommended: 80)", funcName, length),
					Suggestion:  "Break into smaller functions for better readability",
				})
			}
			funcStart = -1
		}
	}

	// Style: Error wrapping
	wrapPattern := regexp.MustCompile(`return\s+fmt\.Errorf\("[^"]*%w`)
	for i, line := range lines {
		if !wrapPattern.MatchString(line) && strings.Contains(line, "return") && strings.Contains(line, "Errorf") && strings.Contains(line, "%v") {
			issues = append(issues, ReviewIssue{
				Severity: "info", Category: "style", Line: i + 1,
				Description: "Error not wrapped with %w",
				Suggestion:  "Use %w instead of %v to preserve error chain: fmt.Errorf(\"context: %w\", err)",
			})
		}
	}

	return issues
}

// === Rust Review Rules ===

func reviewRust(content string) []ReviewIssue {
	var issues []ReviewIssue
	lines := strings.Split(content, "\n")

	// Security: unwrap() in production code
	unwrapPattern := regexp.MustCompile(`\.unwrap\(\)`)
	for i, line := range lines {
		if unwrapPattern.MatchString(line) && !strings.Contains(line, "test") && !strings.Contains(line, "//") {
			issues = append(issues, ReviewIssue{
				Severity: "warning", Category: "security", Line: i + 1,
				Description: "unwrap() will panic on None/Err - use expect() or ? operator",
				Suggestion:  "Replace .unwrap() with .expect(\"reason\") or propagate with ?",
			})
		}
	}

	// Bug: Unhandled Option/Result
	optionPattern := regexp.MustCompile(`let\s+\w+\s*=\s*\w+\.\w+\(\).*;\s*$`)
	for i, line := range lines {
		if optionPattern.MatchString(line) && !strings.Contains(line, "if let") && !strings.Contains(line, "match") {
			issues = append(issues, ReviewIssue{
				Severity: "warning", Category: "bug", Line: i + 1,
				Description: "Possible unhandled Option/Result value",
				Suggestion:  "Use match or if let to handle None/Err cases",
			})
		}
	}

	// Performance: Unnecessary cloning
	clonePattern := regexp.MustCompile(`\.clone\(\)`)
	cloneCount := 0
	for i, line := range lines {
		if clonePattern.MatchString(line) {
			cloneCount++
			if cloneCount > 5 {
				issues = append(issues, ReviewIssue{
					Severity: "info", Category: "performance", Line: i + 1,
					Description: "High clone count - consider borrowing instead",
					Suggestion:  "Use &T references where possible to avoid allocation",
				})
				cloneCount = 0
			}
		}
	}

	// Memory: unsafe blocks
	unsafePattern := regexp.MustCompile(`unsafe\s*\{`)
	for i, line := range lines {
		if unsafePattern.MatchString(line) {
			issues = append(issues, ReviewIssue{
				Severity: "warning", Category: "security", Line: i + 1,
				Description: "unsafe block - ensure memory safety guarantees",
				Suggestion:  "Add safety comment explaining why this unsafe block is sound",
			})
		}
	}

	return issues
}

// === C++ Review Rules ===

func reviewCpp(content string) []ReviewIssue {
	var issues []ReviewIssue
	lines := strings.Split(content, "\n")

	// Security: Buffer overflow
	sprintfPattern := regexp.MustCompile(`sprintf\(|strcpy\(|strcat\(`)
	for i, line := range lines {
		if sprintfPattern.MatchString(line) {
			issues = append(issues, ReviewIssue{
				Severity: "critical", Category: "security", Line: i + 1,
				Description: "Unsafe string function - potential buffer overflow",
				Suggestion:  "Use snprintf(), strncpy(), or std::string instead",
			})
		}
	}

	// Memory: Raw pointers
	newPattern := regexp.MustCompile(`new\s+\w+[^=]*;`)
	for i, line := range lines {
		if newPattern.MatchString(line) && !strings.Contains(line, "unique_ptr") && !strings.Contains(line, "shared_ptr") {
			issues = append(issues, ReviewIssue{
				Severity: "warning", Category: "memory", Line: i + 1,
				Description: "Raw pointer allocation without smart pointer",
				Suggestion:  "Use std::unique_ptr or std::shared_ptr for automatic memory management",
			})
		}
	}

	// Bug: Dangling reference
	refPattern := regexp.MustCompile(`&\w+\s*\)`)
	for i, line := range lines {
		if refPattern.MatchString(line) && strings.Contains(line, "return") {
			issues = append(issues, ReviewIssue{
				Severity: "warning", Category: "bug", Line: i + 1,
				Description: "Returning reference to local variable",
				Suggestion:  "Return by value or ensure referenced object outlives the reference",
			})
		}
	}

	return issues
}

// === Shell Review Rules ===

func reviewShell(content string) []ReviewIssue {
	var issues []ReviewIssue
	lines := strings.Split(content, "\n")

	// Security: eval usage
	evalPattern := regexp.MustCompile(`eval\s`)
	for i, line := range lines {
		if evalPattern.MatchString(line) && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			issues = append(issues, ReviewIssue{
				Severity: "critical", Category: "security", Line: i + 1,
				Description: "eval usage - potential code injection",
				Suggestion:  "Avoid eval; use case statements or safe variable expansion",
			})
		}
	}

	// Security: Unquoted variables
	reVarRef := regexp.MustCompile(`\$\w+`)
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") { continue }
		matches := reVarRef.FindAllStringIndex(line, -1)
		for _, m := range matches {
			end := m[1]
			if end >= len(line) || (line[end] != '}' && line[end] != '"' && line[end] != '(') {
				issues = append(issues, ReviewIssue{
					Severity: "warning", Category: "security", Line: i + 1,
					Description: "Unquoted variable - word splitting and globbing risk",
					Suggestion:  "Always quote variables: \"$VAR\"",
				})
				break
			}
		}
	}

	// Bug: Missing set -euo pipefail
	if !strings.Contains(content, "set -euo pipefail") && !strings.Contains(content, "set -e") {
		issues = append(issues, ReviewIssue{
			Severity: "warning", Category: "bug", Line: 1,
			Description: "Missing 'set -euo pipefail' at script start",
			Suggestion:  "Add 'set -euo pipefail' to fail on errors and undefined variables",
		})
	}

	// Security: Temporary file without cleanup
	if strings.Contains(content, "mktemp") && !strings.Contains(content, "trap") {
		issues = append(issues, ReviewIssue{
			Severity: "warning", Category: "security", Line: 1,
			Description: "mktemp used without trap cleanup",
			Suggestion:  "Add 'trap \"rm -f $TMPFILE\" EXIT' to clean up temporary files",
		})
	}

	return issues
}

// === Python Review Rules ===

func reviewPython(content string) []ReviewIssue {
	var issues []ReviewIssue
	lines := strings.Split(content, "\n")

	// Security: Pickle usage
	picklePattern := regexp.MustCompile(`pickle\.load|pickle\.loads`)
	for i, line := range lines {
		if picklePattern.MatchString(line) {
			issues = append(issues, ReviewIssue{
				Severity: "critical", Category: "security", Line: i + 1,
				Description: "pickle.load on untrusted data - arbitrary code execution",
				Suggestion:  "Use json or msgpack for safe deserialization",
			})
		}
	}

	// Security: Shell injection
	shellPattern := regexp.MustCompile(`os\.system\(|subprocess\.call\(.*shell=True`)
	for i, line := range lines {
		if shellPattern.MatchString(line) {
			issues = append(issues, ReviewIssue{
				Severity: "critical", Category: "security", Line: i + 1,
				Description: "Shell injection risk",
				Suggestion:  "Use subprocess.run with list arguments, not shell=True",
			})
		}
	}

	// Bug: Bare except
	bareExcept := regexp.MustCompile(`except\s*:`)
	for i, line := range lines {
		if bareExcept.MatchString(line) {
			issues = append(issues, ReviewIssue{
				Severity: "warning", Category: "bug", Line: i + 1,
				Description: "Bare except catches SystemExit and KeyboardInterrupt",
				Suggestion:  "Use 'except Exception:' or specific exception types",
			})
		}
	}

	return issues
}

// === TypeScript Review Rules ===

func reviewTypeScript(content string) []ReviewIssue {
	var issues []ReviewIssue
	lines := strings.Split(content, "\n")

	// Bug: any type
	anyPattern := regexp.MustCompile(`:\s*any\b|as\s+any\b`)
	for i, line := range lines {
		if anyPattern.MatchString(line) && !strings.Contains(line, "//") {
			issues = append(issues, ReviewIssue{
				Severity: "info", Category: "style", Line: i + 1,
				Description: "Use of 'any' type disables type checking",
				Suggestion:  "Use specific types or 'unknown' for safer type narrowing",
			})
		}
	}

	// Security: innerHTML
	innerHtmlPattern := regexp.MustCompile(`innerHTML\s*=`)
	for i, line := range lines {
		if innerHtmlPattern.MatchString(line) {
			issues = append(issues, ReviewIssue{
				Severity: "critical", Category: "security", Line: i + 1,
				Description: "innerHTML assignment - XSS vulnerability",
				Suggestion:  "Use textContent or a sanitization library like DOMPurify",
			})
		}
	}

	// Performance: Missing React.memo/useMemo
	if strings.Contains(content, "export default function") && strings.Contains(content, "className") {
		issues = append(issues, ReviewIssue{
			Severity: "info", Category: "performance", Line: 1,
			Description: "React component may benefit from memoization",
			Suggestion:  "Consider React.memo() for components with stable props",
		})
	}

	return issues
}

// === Generic Review Rules ===

func reviewGeneric(content string) []ReviewIssue {
	var issues []ReviewIssue
	return issues
}

// === Universal Review Rules (all languages) ===

func reviewUniversal(content string) []ReviewIssue {
	var issues []ReviewIssue
	lines := strings.Split(content, "\n")

	// Security: Hardcoded secrets
	secretPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*["'][^"']+["']`),
		regexp.MustCompile(`(?i)(api_key|apikey|api-key)\s*[:=]\s*["'][^"']+["']`),
		regexp.MustCompile(`(?i)(secret|token)\s*[:=]\s*["'][^"']+["']`),
		regexp.MustCompile(`(?i)(access_key|secret_key)\s*[:=]\s*["'][^"']+["']`),
	}
	for i, line := range lines {
		for _, pattern := range secretPatterns {
			if pattern.MatchString(line) && !strings.Contains(line, "example") && !strings.Contains(line, "TODO") {
				issues = append(issues, ReviewIssue{
					Severity: "critical", Category: "security", Line: i + 1,
					Description: "Hardcoded secret/credential detected",
					Suggestion:  "Use environment variables or secrets manager",
				})
				break
			}
		}
	}

	// Performance: TODO/FIXME/HACK comments
	todoPattern := regexp.MustCompile(`(?i)(TODO|FIXME|HACK|XXX):?\s`)
	for i, line := range lines {
		if todoPattern.MatchString(line) {
			issues = append(issues, ReviewIssue{
				Severity: "info", Category: "complexity", Line: i + 1,
				Description: "Technical debt marker found",
				Suggestion:  "Address before production deployment",
			})
		}
	}

	// Security: Debug logging in production
	debugPattern := regexp.MustCompile(`(?i)(fmt\.Print|console\.log|print\()`)
	debugCount := 0
	for i, line := range lines {
		if debugPattern.MatchString(line) && !strings.Contains(line, "//") {
			debugCount++
			if debugCount > 5 {
				issues = append(issues, ReviewIssue{
					Severity: "warning", Category: "security", Line: i + 1,
					Description: "Excessive debug output in production code",
					Suggestion:  "Use structured logging (slog/log) with level control",
				})
				debugCount = 0
			}
		}
	}

	// Performance: Large string concatenation
	concatPattern := regexp.MustCompile(`\+\s*["'].*["']\s*\+`)
	for i, line := range lines {
		if strings.Count(line, "+") > 3 && concatPattern.MatchString(line) {
			issues = append(issues, ReviewIssue{
				Severity: "info", Category: "performance", Line: i + 1,
				Description: "Multiple string concatenations",
				Suggestion:  "Use strings.Builder, fmt.Sprintf, or template literals",
			})
		}
	}

	return issues
}

// === Format Result ===

func formatReviewResult(path, language string, issues []ReviewIssue) string {
	if len(issues) == 0 {
		return fmt.Sprintf("Code review passed for %s (%s)\nNo issues found.", path, language)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Code Review: %s (%s)\n", path, language))
	sb.WriteString(fmt.Sprintf("Found %d issue(s):\n\n", len(issues)))

	critical := 0
	warnings := 0
	info := 0

	for _, issue := range issues {
		switch issue.Severity {
		case "critical":
			critical++
			sb.WriteString(fmt.Sprintf("CRITICAL [Line %d] %s\n", issue.Line, issue.Category))
		case "warning":
			warnings++
			sb.WriteString(fmt.Sprintf("WARNING  [Line %d] %s\n", issue.Line, issue.Category))
		case "info":
			info++
			sb.WriteString(fmt.Sprintf("INFO     [Line %d] %s\n", issue.Line, issue.Category))
		}
		sb.WriteString(fmt.Sprintf("   %s\n", issue.Description))
		sb.WriteString(fmt.Sprintf("   Suggestion: %s\n\n", issue.Suggestion))
	}

	sb.WriteString(fmt.Sprintf("Summary: %d critical, %d warnings, %d info", critical, warnings, info))

	if critical > 0 {
		sb.WriteString("\n\nCRITICAL issues must be fixed before build_module!")
	}

	return sb.String()
}
