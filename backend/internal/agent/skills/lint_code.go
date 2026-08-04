package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type LintCodeSkill struct{}

func NewLintCodeSkill() *LintCodeSkill {
	return &LintCodeSkill{}
}

func (s *LintCodeSkill) Name() string {
	return "lint_code"
}

func (s *LintCodeSkill) Description() string {
	return "Lint code for syntax and security issues. Input: {\"files\": {\"path\": \"content\", ...}} or {\"path\": \"...\", \"content\": \"...\", \"language\": \"...\"}. Returns concise issue list."
}

type lintIssue struct {
	Severity string `json:"severity"`
	Rule     string `json:"rule"`
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message"`
}

type lintReport struct {
	Safe   bool        `json:"safe"`
	Issues []lintIssue `json:"issues"`
	Score  int         `json:"score"`
}

func (s *LintCodeSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	if path, hasPath := input["path"].(string); hasPath {
		content, _ := input["content"].(string)
		lang, _ := input["language"].(string)
		if lang == "" {
			lang = detectLanguage(path)
		}
		if content == "" {
			return "", fmt.Errorf("content is required")
		}
		issues := s.lintFile(path, content, lang)
		return formatLintReport(issues), nil
	}

	filesRaw, ok := input["files"]
	if !ok {
		return "", fmt.Errorf("files or path/content is required")
	}
	filesMap, ok := filesRaw.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("files must be an object")
	}

	var allIssues []lintIssue
	for filePath, contentRaw := range filesMap {
		content, _ := contentRaw.(string)
		if content == "" {
			continue
		}
		lang := detectLanguage(filePath)
		allIssues = append(allIssues, s.lintFile(filePath, content, lang)...)
	}

	return formatLintReport(allIssues), nil
}

func formatLintReport(issues []lintIssue) string {
	score := 100
	for _, issue := range issues {
		switch issue.Severity {
		case "critical":
			score -= 15
		case "warning":
			score -= 5
		case "info":
			score -= 1
		}
	}
	if score < 0 {
		score = 0
	}

	// Concise output: group by file, show only critical and warnings
	var concise []lintIssue
	for _, issue := range issues {
		if issue.Severity == "critical" || issue.Severity == "warning" {
			concise = append(concise, issue)
		}
	}

	// If all issues are info-level, show summary only
	if len(concise) == 0 && len(issues) > 0 {
		return fmt.Sprintf(`{"safe":true,"score":%d,"info_count":%d,"message":"No critical or warning issues found. %d info-level notes."}`, score, len(issues), len(issues))
	}

	report := lintReport{
		Safe:   len(concise) == 0,
		Issues: concise,
		Score:  score,
	}
	b, _ := json.MarshalIndent(report, "", "  ")
	return string(b)
}

var shellLintPatterns = []struct {
	rule    string
	pattern *regexp.Regexp
	message string
	severity string
}{
	{rule: "unquoted_var", pattern: regexp.MustCompile(`\$[a-zA-Z_][a-zA-Z0-9_]*`), message: "Unquoted variable expansion - use \"$var\" to prevent word splitting", severity: "warning"},
	{rule: "missing_set_e", pattern: regexp.MustCompile(`(?m)^#!/`), message: "Script missing 'set -e' - add 'set -e' after shebang for safety", severity: "warning"},
	{rule: "bash_array", pattern: regexp.MustCompile(`\([^)]*\)`), message: "Array syntax not POSIX-compatible - use printf or eval instead", severity: "warning"},
	{rule: "subshell", pattern: regexp.MustCompile(`\$\(\(`), message: "Arithmetic expansion $((...)) not POSIX - use expr instead", severity: "info"},
	{rule: "process_substitution", pattern: regexp.MustCompile(`<\(`), message: "Process substitution <() not POSIX-compatible", severity: "warning"},
	{rule: "here_string", pattern: regexp.MustCompile(`<<<`), message: "Here-string <<< not POSIX-compatible", severity: "warning"},
	{rule: "double_bracket", pattern: regexp.MustCompile(`\[\[`), message: "[[ ]] is bash-only, use [ ] for POSIX sh compatibility on Android", severity: "warning"},
}

func (s *LintCodeSkill) lintFile(filePath, content, lang string) []lintIssue {
	switch lang {
	case "shell":
		return s.lintShell(filePath, content)
	case "rust":
		return s.lintRust(filePath, content)
	case "go":
		return s.lintGo(filePath, content)
	case "python":
		return s.lintPython(filePath, content)
	case "cpp", "c":
		return s.lintCpp(filePath, content)
	}
	return nil
}

func (s *LintCodeSkill) lintShell(filePath, content string) []lintIssue {
	var issues []lintIssue
	lines := strings.Split(content, "\n")

	hasSetE := false
	hasShebang := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if i == 0 && strings.HasPrefix(trimmed, "#!") {
			hasShebang = true
		}

		if trimmed == "set -e" || trimmed == "set -o errexit" {
			hasSetE = true
		}

		if strings.Contains(trimmed, "`") && strings.Count(trimmed, "`") >= 2 {
			issues = append(issues, lintIssue{
				Severity: "warning", Rule: "backtick_exec", File: filePath, Line: i + 1,
				Message: "Use $() instead of backticks for command substitution",
			})
		}

		if strings.Contains(trimmed, "chmod 777") || strings.Contains(trimmed, "chmod 0777") {
			issues = append(issues, lintIssue{
				Severity: "critical", Rule: "chmod_777", File: filePath, Line: i + 1,
				Message: "Use of chmod 777 is insecure - use minimal necessary permissions",
			})
		}
	}

	if hasShebang && !hasSetE {
		issues = append(issues, lintIssue{
			Severity: "warning", Rule: "missing_set_e", File: filePath,
			Message: "Script missing 'set -e' - add it after shebang to exit on errors",
		})
	}

	if !hasShebang {
		issues = append(issues, lintIssue{
			Severity: "warning", Rule: "missing_shebang", File: filePath,
			Message: "Script missing shebang line (e.g., #!/system/bin/sh)",
		})
	}

	for _, dp := range shellLintPatterns {
		if dp.rule == "missing_set_e" && hasShebang {
			continue
		}
		matches := dp.pattern.FindAllStringIndex(content, -1)
		if len(matches) > 0 {
			issues = append(issues, lintIssue{
				Severity: dp.severity, Rule: dp.rule, File: filePath,
				Message: dp.message,
			})
		}
	}

	return issues
}

var unwrapRe = regexp.MustCompile(`\.unwrap\(\)`)
var unsafeRe = regexp.MustCompile(`\bunsafe\s*\{`)
var todoRe = regexp.MustCompile(`(?i)\bTODO\b`)
var expectRe = regexp.MustCompile(`\.expect\(`)
var panicRe = regexp.MustCompile(`\bpanic!\(`)

// Rust Atomic type misuse patterns
var atomicLoadRe = regexp.MustCompile(`\.\s*load\s*\(\s*Ordering::`)
var atomicStoreRe = regexp.MustCompile(`\.\s*store\s*\(.*Ordering::`)
var atomicSwapRe = regexp.MustCompile(`\.\s*swap\s*\(.*Ordering::`)
var atomicCompareSwapRe = regexp.MustCompile(`\.\s*compare_exchange\s*\(`)

// Rust move-out-of-shared-reference patterns
var matchRefMutRe = regexp.MustCompile(`match\s+&.*\{[^}]*Some\s*\(\s*mut\s+`)

func (s *LintCodeSkill) lintRust(filePath, content string) []lintIssue {
	var issues []lintIssue
	lines := strings.Split(content, "\n")

	// Check for Atomic type misuse - common AI mistake
	// Pattern: using .load()/.store() on non-Atomic types
	atomicTypeDeclRe := regexp.MustCompile(`let\s+(?:mut\s+)?(\w+)\s*:\s*(u32|u64|i32|i64)\s*=`)
	atomicUsedAsAtomic := make(map[string]bool)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track variable declarations with plain integer types
		if matches := atomicTypeDeclRe.FindStringSubmatch(trimmed); matches != nil {
			varName := matches[1]
			// Check if this variable is later used with Atomic methods
			for j := i + 1; j < len(lines) && j < i+50; j++ {
				nextLine := lines[j]
				if atomicLoadRe.MatchString(nextLine) && strings.Contains(nextLine, varName) {
					atomicUsedAsAtomic[varName] = true
					break
				}
				if atomicStoreRe.MatchString(nextLine) && strings.Contains(nextLine, varName) {
					atomicUsedAsAtomic[varName] = true
					break
				}
			}
		}

		// Check for Atomic methods on non-Atomic variables
		if atomicLoadRe.MatchString(trimmed) || atomicStoreRe.MatchString(trimmed) || atomicSwapRe.MatchString(trimmed) || atomicCompareSwapRe.MatchString(trimmed) {
			// Extract the variable name (simplified check)
			varRe := regexp.MustCompile(`(\w+)\s*\.\s*(?:load|store|swap|compare_exchange)\s*\(`)
			if matches := varRe.FindStringSubmatch(trimmed); matches != nil {
				varName := matches[1]
				// Check if this variable was declared as non-Atomic
				for _, line := range lines {
					if atomicTypeDeclRe.MatchString(line) && strings.Contains(line, varName) {
						issues = append(issues, lintIssue{
							Severity: "critical", Rule: "atomic_misuse", File: filePath, Line: i + 1,
							Message: fmt.Sprintf("Variable '%s' is declared as %s but used with Atomic methods (.load/.store) - use AtomicU32/AtomicU64 instead", varName, "u32/u64"),
						})
						break
					}
				}
			}
		}

		// Check for move-out-of-shared-reference in match patterns
		if matchRefMutRe.MatchString(trimmed) {
			issues = append(issues, lintIssue{
				Severity: "critical", Rule: "move_from_shared_ref", File: filePath, Line: i + 1,
				Message: "match on shared reference with 'Some(mut x)' moves data - use 'Some(ref mut x)' instead",
			})
		}

		if unwrapRe.MatchString(trimmed) {
			issues = append(issues, lintIssue{
				Severity: "warning", Rule: "unwrap_usage", File: filePath, Line: i + 1,
				Message: "Use of .unwrap() - use proper error handling with match or ? operator",
			})
		}

		if expectRe.MatchString(trimmed) {
			issues = append(issues, lintIssue{
				Severity: "info", Rule: "expect_usage", File: filePath, Line: i + 1,
				Message: "Use of .expect() - ensure the panic message provides context",
			})
		}
	}

	if unsafeRe.MatchString(content) {
		issues = append(issues, lintIssue{
			Severity: "warning", Rule: "unsafe_block", File: filePath,
			Message: "Unsafe blocks in production code require careful review",
		})
	}

	if panicRe.MatchString(content) {
		issues = append(issues, lintIssue{
			Severity: "warning", Rule: "panic_usage", File: filePath,
			Message: "panic!() should be avoided in library code - use Result types instead",
		})
	}

	return issues
}

func (s *LintCodeSkill) lintGo(filePath, content string) []lintIssue {
	var issues []lintIssue
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, "if err != nil") {
			nextLine := ""
			if i+1 < len(lines) {
				nextLine = strings.TrimSpace(lines[i+1])
			}
			if nextLine == "" || nextLine == "}" || nextLine == "return" {
				issues = append(issues, lintIssue{
					Severity: "warning", Rule: "empty_error_check", File: filePath, Line: i + 1,
					Message: "Error check with empty or no-op body - handle the error properly",
				})
			}
		}

		if strings.Contains(trimmed, "go func()") && !strings.Contains(trimmed, "sync.WaitGroup") && !strings.Contains(trimmed, "context") {
			issues = append(issues, lintIssue{
				Severity: "warning", Rule: "goroutine_leak", File: filePath, Line: i + 1,
				Message: "Goroutine without sync.WaitGroup or context cancellation - potential leak",
			})
		}

		if strings.Contains(trimmed, "os.Exit(") || strings.Contains(trimmed, "os.Exit(") {
			issues = append(issues, lintIssue{
				Severity: "warning", Rule: "os_exit", File: filePath, Line: i + 1,
				Message: "os.Exit() skips deferred functions - use return instead",
			})
		}

		if strings.Contains(trimmed, "_ = ") {
			issues = append(issues, lintIssue{
				Severity: "info", Rule: "ignored_return", File: filePath, Line: i + 1,
				Message: "Ignoring return value with _ - consider checking the error",
			})
		}
	}

	return issues
}

func (s *LintCodeSkill) lintPython(filePath, content string) []lintIssue {
	var issues []lintIssue
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, "except:") {
			issues = append(issues, lintIssue{
				Severity: "warning", Rule: "bare_except", File: filePath, Line: i + 1,
				Message: "Bare except: catches all exceptions including SystemExit/KeyboardInterrupt",
			})
		}

		if strings.Contains(trimmed, "subprocess.") && strings.Contains(trimmed, "shell=True") {
			issues = append(issues, lintIssue{
				Severity: "warning", Rule: "shell_true", File: filePath, Line: i + 1,
				Message: "subprocess with shell=True is a security risk - use args list instead",
			})
		}

		if strings.Contains(trimmed, "def ") && strings.Contains(trimmed, "=[]") || strings.Contains(trimmed, "={}") {
			issues = append(issues, lintIssue{
				Severity: "warning", Rule: "mutable_default", File: filePath, Line: i + 1,
				Message: "Mutable default argument - use None and initialize inside function",
			})
		}

		if strings.Contains(trimmed, "eval(") || strings.Contains(trimmed, "exec(") {
			issues = append(issues, lintIssue{
				Severity: "critical", Rule: "eval_exec", File: filePath, Line: i + 1,
				Message: "eval()/exec() can execute arbitrary code - avoid if possible",
			})
		}
	}

	return issues
}

var cppNewRe = regexp.MustCompile(`\bnew\s+\w+[\[\(]`)
var cppDeleteRe = regexp.MustCompile(`\bdelete\s+\w+`)
var cppRawPtrRe = regexp.MustCompile(`\w+\s*\*\s*\w+\s*=`)
var cppGotoRe = regexp.MustCompile(`\bgoto\s+\w+`)
var cppSprintfRe = regexp.MustCompile(`\bsprintf\s*\(`)
var cppMemcpyRe = regexp.MustCompile(`\bmemcpy\s*\(`)
var cppUninitRe = regexp.MustCompile(`(?:int|char|float|double|long|short|void|auto)\s+\w+\s*;`)

// Additional C++ lint patterns for Android NDK
var cppRawArrayRe = regexp.MustCompile(`(?:int|char|float|double|long|short)\s+\w+\s*\[\s*\d+\s*\]`)
var cppCStyleCastRe = regexp.MustCompile(`\(\s*(?:int|char|float|double|long|short|void|unsigned|const)\s*\*?\s*\)\s*\w+`)
var cppSizeofRe = regexp.MustCompile(`sizeof\s*\(\s*\w+\s*\*?\s*\)`)

func (s *LintCodeSkill) lintCpp(filePath, content string) []lintIssue {
	var issues []lintIssue
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if cppNewRe.MatchString(trimmed) && !strings.Contains(trimmed, "unique_ptr") && !strings.Contains(trimmed, "shared_ptr") && !strings.Contains(trimmed, "make_unique") && !strings.Contains(trimmed, "make_shared") {
			issues = append(issues, lintIssue{
				Severity: "warning", Rule: "raw_new", File: filePath, Line: i + 1,
				Message: "Raw 'new' without smart pointer - use std::unique_ptr or std::shared_ptr",
			})
		}

		if cppDeleteRe.MatchString(trimmed) {
			issues = append(issues, lintIssue{
				Severity: "warning", Rule: "raw_delete", File: filePath, Line: i + 1,
				Message: "Raw 'delete' - prefer smart pointers to avoid memory leaks",
			})
		}

		if cppSprintfRe.MatchString(trimmed) {
			issues = append(issues, lintIssue{
				Severity: "critical", Rule: "sprintf_usage", File: filePath, Line: i + 1,
				Message: "sprintf() is unsafe - use snprintf() or std::format() to prevent buffer overflow",
			})
		}

		if trimmed == "goto" || strings.HasPrefix(trimmed, "goto ") {
			issues = append(issues, lintIssue{
				Severity: "warning", Rule: "goto_usage", File: filePath, Line: i + 1,
				Message: "goto statement - use structured control flow (loops, functions, exceptions)",
			})
		}

		if strings.Contains(trimmed, "using namespace std") {
			issues = append(issues, lintIssue{
				Severity: "info", Rule: "using_namespace_std", File: filePath, Line: i + 1,
				Message: "'using namespace std' pollutes global namespace - use explicit std:: prefix or selective using",
			})
		}

		if strings.Contains(trimmed, "#define ") && (strings.Contains(trimmed, "(") || strings.Contains(trimmed, "+") || strings.Contains(trimmed, "*")) {
			issues = append(issues, lintIssue{
				Severity: "info", Rule: "macro_function", File: filePath, Line: i + 1,
				Message: "Function-like macro - prefer inline functions or constexpr for type safety",
			})
		}

		if strings.Contains(trimmed, "catch(...)") {
			issues = append(issues, lintIssue{
				Severity: "warning", Rule: "catch_all", File: filePath, Line: i + 1,
				Message: "Catch-all catch(...) - catch specific exceptions instead",
			})
		}
	}

	if strings.Contains(content, "#include <stdlib.h>") || strings.Contains(content, "#include <stdio.h>") || strings.Contains(content, "#include <string.h>") {
		issues = append(issues, lintIssue{
			Severity: "info", Rule: "c_headers", File: filePath,
			Message: "C headers (<stdlib.h> etc.) - prefer C++ headers (<cstdlib>, <cstdio>, <cstring>)",
		})
	}

	// Check for raw arrays (prefer std::array or std::vector)
	if cppRawArrayRe.MatchString(content) {
		issues = append(issues, lintIssue{
			Severity: "warning", Rule: "raw_array", File: filePath,
			Message: "Raw C-style array - prefer std::array or std::vector for bounds checking",
		})
	}

	// Check for C-style casts (prefer static_cast/reinterpret_cast)
	if cppCStyleCastRe.MatchString(content) {
		issues = append(issues, lintIssue{
			Severity: "warning", Rule: "c_style_cast", File: filePath,
			Message: "C-style cast - use static_cast/reinterpret_cast for type safety",
		})
	}

	return issues
}

func (s *LintCodeSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  true,
		Essential: false,
		NeedsDB:   false,
		NeedsLLM:  true,
	}
}
