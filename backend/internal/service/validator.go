package service

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type ValidationResult struct {
	File     string   `json:"file"`
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type ValidatorService struct{}

func NewValidatorService() *ValidatorService {
	return &ValidatorService{}
}

func (s *ValidatorService) ValidateFile(filename string, content string) ValidationResult {
	result := ValidationResult{File: filename, Valid: true}

	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".prop":
		s.validateProp(content, &result)
	case ".sh":
		s.validateShell(content, &result)
	case ".json":
		s.validateJSON(content, &result)
	case ".xml", ".conf":
		s.validateXML(content, &result)
	case ".txt", ".md":
		// no validation needed
	default:
		// no specific validation
	}

	return result
}

var propLineRegex = regexp.MustCompile(`^[a-zA-Z._0-9-]+=.*$`)

func (s *ValidatorService) validateProp(content string, result *ValidationResult) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !propLineRegex.MatchString(trimmed) {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("line %d: invalid property format (expected key=value): %s", i+1, trimmed))
		}
	}
}

func (s *ValidatorService) validateShell(content string, result *ValidationResult) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "}") && !strings.HasPrefix(line, "}") {
			result.Warnings = append(result.Warnings, fmt.Sprintf("line %d: unexpected closing brace", i+1))
		}
		inSingle := false
		inDouble := false
		for _, ch := range trimmed {
			if ch == '\'' && !inDouble {
				inSingle = !inSingle
			}
			if ch == '"' && !inSingle {
				inDouble = !inDouble
			}
		}
		if inSingle {
			result.Warnings = append(result.Warnings, fmt.Sprintf("line %d: unclosed single quote", i+1))
		}
		if inDouble {
			result.Warnings = append(result.Warnings, fmt.Sprintf("line %d: unclosed double quote", i+1))
		}
		if i == 0 && !strings.HasPrefix(trimmed, "#!") {
			result.Warnings = append(result.Warnings, "line 1: missing shebang (#!/system/bin/sh)")
		}
	}

	hasCode := false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t != "" && !strings.HasPrefix(t, "#") {
			hasCode = true
			break
		}
	}
	if !hasCode {
		result.Warnings = append(result.Warnings, "file appears to contain no executable code")
	}
}

func (s *ValidatorService) validateJSON(content string, result *ValidationResult) {
	braceCount := 0
	for _, ch := range content {
		switch ch {
		case '{', '[':
			braceCount++
		case '}', ']':
			braceCount--
		}
	}
	if braceCount != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "unbalanced braces/brackets in JSON")
	}
}

func (s *ValidatorService) validateXML(content string, result *ValidationResult) {
	if !strings.Contains(content, "<?xml") && !strings.Contains(content, "<") {
		result.Warnings = append(result.Warnings, "content does not appear to be XML")
	}
}

func (s *ValidatorService) ValidateProject(files map[string]string) []ValidationResult {
	results := make([]ValidationResult, 0, len(files))
	for filename, content := range files {
		result := s.ValidateFile(filename, content)
		results = append(results, result)
	}
	return results
}
