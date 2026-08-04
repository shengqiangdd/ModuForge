package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type ValidateSkill struct{}

func NewValidateSkill() *ValidateSkill {
	return &ValidateSkill{}
}

func (s *ValidateSkill) Name() string {
	return "validate"
}

func (s *ValidateSkill) Description() string {
	return "Validate code for security issues and best practices. Input: {\"files\": {...}}. Returns critical issues only."
}

type validationIssue struct {
	Severity string `json:"severity"`
	Rule     string `json:"rule"`
	File     string `json:"file"`
	Message  string `json:"message"`
}

func (s *ValidateSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	filesRaw, ok := input["files"]
	if !ok {
		return "", fmt.Errorf("files is required")
	}

	filesMap, ok := filesRaw.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("files must be an object")
	}

	var issues []validationIssue

	for filePath, contentRaw := range filesMap {
		content, _ := contentRaw.(string)
		if content == "" {
			continue
		}

		issues = append(issues, s.validateFile(filePath, content)...)
	}

	result := struct {
		Safe   bool               `json:"safe"`
		Issues []validationIssue  `json:"issues"`
		Score  int                `json:"score"`
	}{}

	result.Issues = issues
	result.Score = 100
	for _, issue := range issues {
		switch issue.Severity {
		case "critical":
			result.Score -= 15
		case "warning":
			result.Score -= 5
		case "info":
			result.Score -= 1
		}
	}
	if result.Score < 0 {
		result.Score = 0
	}
	result.Safe = len(issues) == 0

	// Concise output: only include critical and warning issues
	var keyIssues []validationIssue
	for _, issue := range issues {
		if issue.Severity == "critical" || issue.Severity == "warning" {
			keyIssues = append(keyIssues, issue)
		}
	}
	if len(keyIssues) == 0 && len(issues) > 0 {
		// No critical/warning issues, return summary
		return fmt.Sprintf(`{"safe":true,"score":%d,"info_count":%d,"message":"No security issues. %d info-level notes."}`, result.Score, len(issues), len(issues)), nil
	}
	result.Issues = keyIssues

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

var dangerousPatterns = []struct {
	rule    string
	pattern *regexp.Regexp
	message string
	severity string
}{
	{rule: "chmod_777", pattern: regexp.MustCompile(`chmod\s+777`), message: "Use of chmod 777 - use minimal necessary permissions", severity: "critical"},
	{rule: "eval_usage", pattern: regexp.MustCompile(`\beval\b`), message: "Use of eval can lead to command injection", severity: "critical"},
	{rule: "backtick_exec", pattern: regexp.MustCompile("`[^`]+`"), message: "Backtick command substitution - use $() instead", severity: "warning"},
	{rule: "hardcoded_key", pattern: regexp.MustCompile(`(?i)(api[_-]?key|secret|password|token)\s*[:=]\s*['"][^'"]+['"]`), message: "Possible hardcoded credential", severity: "critical"},
	{rule: "no_shebang", pattern: regexp.MustCompile(`(?m)^\s*$`), message: "Script missing shebang line, might use wrong interpreter", severity: "warning"},
}

func (s *ValidateSkill) validateFile(filePath, content string) []validationIssue {
	var issues []validationIssue
	lines := strings.Split(content, "\n")
	lang := detectLanguage(filePath)

	if lang == "shell" || lang == "unknown" {
		if len(lines) > 0 && !strings.HasPrefix(strings.TrimSpace(lines[0]), "#!") {
			issues = append(issues, validationIssue{
				Severity: "warning",
				Rule:     "missing_shebang",
				File:     filePath,
				Message:  "Script missing shebang line, might use wrong interpreter",
			})
		}
	}

	if filePath == "module.prop" {
		hasID := false
		hasName := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "id=") {
				hasID = true
			}
			if strings.HasPrefix(trimmed, "name=") {
				hasName = true
			}
		}
		if !hasID {
			issues = append(issues, validationIssue{Severity: "critical", Rule: "missing_module_id", File: filePath, Message: "module.prop missing required 'id' field"})
		}
		if !hasName {
			issues = append(issues, validationIssue{Severity: "warning", Rule: "missing_module_name", File: filePath, Message: "module.prop missing 'name' field"})
		}
	}

	for _, dp := range dangerousPatterns {
		if dp.pattern.MatchString(content) {
			if dp.rule == "no_shebang" && strings.HasPrefix(strings.TrimSpace(lines[0]), "#!") {
				continue
			}
			if dp.rule == "backtick_exec" && !strings.HasSuffix(filePath, ".sh") {
				continue
			}
			issues = append(issues, validationIssue{
				Severity: dp.severity,
				Rule:     dp.rule,
				File:     filePath,
				Message:  dp.message,
			})
		}
	}

	issues = append(issues, s.validateMultiLang(filePath, content, lang)...)

	return issues
}

func (s *ValidateSkill) validateMultiLang(filePath, content string, lang string) []validationIssue {
	var issues []validationIssue
	lines := strings.Split(content, "\n")

	switch lang {
	case "rust":
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
				continue
			}
			if strings.Contains(trimmed, "unsafe") && strings.Contains(trimmed, "{") {
				issues = append(issues, validationIssue{
					Severity: "critical", Rule: "unsafe_block", File: filePath,
					Message: fmt.Sprintf("Line %d: unsafe block in production code - must be justified", i+1),
				})
			}
			if strings.Contains(trimmed, ".unwrap()") {
				issues = append(issues, validationIssue{
					Severity: "warning", Rule: "unwrap_usage", File: filePath,
					Message: fmt.Sprintf("Line %d: .unwrap() can panic - use proper error handling", i+1),
				})
			}
		}
		if strings.Contains(content, "let path = ") || strings.Contains(content, "std::path::Path::new(") {
			if strings.Contains(content, "\"/data/") || strings.Contains(content, "\"/system/") {
				issues = append(issues, validationIssue{
					Severity: "warning", Rule: "hardcoded_path", File: filePath,
					Message: "Hardcoded system path detected - use configuration or environment variables",
				})
			}
		}

	case "go":
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
				continue
			}
			if strings.Contains(trimmed, "os.Exit(") {
				issues = append(issues, validationIssue{
					Severity: "critical", Rule: "os_exit", File: filePath,
					Message: fmt.Sprintf("Line %d: os.Exit() in non-main function skips deferred calls", i+1),
				})
			}
		}
		if strings.Contains(content, "var ") && strings.Count(content, "var ") > 3 {
			issues = append(issues, validationIssue{
				Severity: "info", Rule: "global_state", File: filePath,
				Message: "Excessive global variables - prefer local scope and dependency injection",
			})
		}

	case "python":
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			if strings.Contains(trimmed, "eval(") {
				issues = append(issues, validationIssue{
					Severity: "critical", Rule: "eval_usage", File: filePath,
					Message: fmt.Sprintf("Line %d: eval() allows arbitrary code execution", i+1),
				})
			}
			if strings.Contains(trimmed, "exec(") {
				issues = append(issues, validationIssue{
					Severity: "critical", Rule: "exec_usage", File: filePath,
					Message: fmt.Sprintf("Line %d: exec() allows arbitrary code execution", i+1),
				})
			}
			if strings.Contains(trimmed, "pickle.loads") || strings.Contains(trimmed, "pickle.load(") {
				issues = append(issues, validationIssue{
					Severity: "warning", Rule: "pickle_unsafe", File: filePath,
					Message: fmt.Sprintf("Line %d: pickle.load() can execute arbitrary code during deserialization", i+1),
				})
			}
		}

	case "cpp", "c":
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
				continue
			}
			if strings.Contains(trimmed, "system(") || strings.Contains(trimmed, "popen(") {
				issues = append(issues, validationIssue{
					Severity: "critical", Rule: "command_injection", File: filePath,
					Message: fmt.Sprintf("Line %d: system()/popen() with user input enables command injection", i+1),
				})
			}
			if strings.Contains(trimmed, "gets(") {
				issues = append(issues, validationIssue{
					Severity: "critical", Rule: "gets_usage", File: filePath,
					Message: fmt.Sprintf("Line %d: gets() is unsafe and removed in C11 - use fgets() or std::getline()", i+1),
				})
			}
			if strings.Contains(trimmed, "sprintf(") && !strings.Contains(trimmed, "snprintf(") {
				issues = append(issues, validationIssue{
					Severity: "critical", Rule: "sprintf_overflow", File: filePath,
					Message: fmt.Sprintf("Line %d: sprintf() can cause buffer overflow - use snprintf()", i+1),
				})
			}
			if strings.Contains(trimmed, "strcpy(") || strings.Contains(trimmed, "strcat(") {
				issues = append(issues, validationIssue{
					Severity: "warning", Rule: "unsafe_string_op", File: filePath,
					Message: fmt.Sprintf("Line %d: %s is unsafe - use strncpy()/strncat() or std::string", i+1, strings.TrimSpace(strings.Split(trimmed, "(")[0]+"()")),
				})
			}
			if strings.Contains(trimmed, "malloc(") && !strings.Contains(trimmed, "free(") {
				issues = append(issues, validationIssue{
					Severity: "info", Rule: "manual_alloc", File: filePath,
					Message: fmt.Sprintf("Line %d: raw malloc() - consider smart pointers or RAII", i+1),
				})
			}
		}
		if strings.Contains(content, "#include <cstdlib>") && strings.Contains(content, "atoi(") {
			issues = append(issues, validationIssue{
				Severity: "warning", Rule: "atoi_unsafe", File: filePath,
				Message: "atoi() has no error checking - use std::stoi() or strtol()",
			})
		}
	}

	return issues
}

func (s *ValidateSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  true,
		Essential: false,
		NeedsDB:   false,
		NeedsLLM:  true,
	}
}
