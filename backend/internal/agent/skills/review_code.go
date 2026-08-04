package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type ReviewCodeSkill struct{}

func NewReviewCodeSkill() *ReviewCodeSkill {
	return &ReviewCodeSkill{}
}

func (s *ReviewCodeSkill) Name() string {
	return "review_code"
}

func (s *ReviewCodeSkill) Description() string {
	return "Code review focusing on naming, structure, error handling, security. Input: {\"files\": {\"path\": \"content\", ...}, \"context\": \"...\"}. Returns concise key issues."
}

type reviewSuggestion struct {
	Category string `json:"category"`
	Impact   string `json:"impact"`
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message"`
	Suggestion string `json:"suggestion"`
}

type codeReview struct {
	OverallScore      int                `json:"overall_score"`
	Maintainability   string             `json:"maintainability"`
	NamingConventions string             `json:"naming_conventions"`
	ErrorHandling     string             `json:"error_handling"`
	CodeStructure     string             `json:"code_structure"`
	Suggestions       []reviewSuggestion `json:"suggestions"`
}

func (s *ReviewCodeSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	filesRaw, ok := input["files"]
	if !ok {
		return "", fmt.Errorf("files is required")
	}
	filesMap, ok := filesRaw.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("files must be an object")
	}

	contextStr, _ := input["context"].(string)
	_ = contextStr // Available for future use in review context

	var allSuggestions []reviewSuggestion
	totalScore := 100
	fileCount := 0

	for filePath, contentRaw := range filesMap {
		content, _ := contentRaw.(string)
		if content == "" {
			continue
		}
		fileCount++
		lang := detectLanguage(filePath)
		suggestions := s.reviewFile(filePath, content, lang)
		allSuggestions = append(allSuggestions, suggestions...)
	}

	if fileCount == 0 {
		return "", fmt.Errorf("no valid files provided")
	}

	// 按严重程度加权扣分
	for _, sug := range allSuggestions {
		switch sug.Impact {
		case "high":
			totalScore -= 15
		case "medium":
			totalScore -= 5
		case "low":
			totalScore -= 2
		default:
			totalScore -= 1
		}
	}
	if totalScore < 0 {
		totalScore = 0
	}

	naming := "Good"
	errorHandling := "Good"
	structure := "Good"
	for _, sug := range allSuggestions {
		switch sug.Category {
		case "naming":
			naming = "Needs improvement"
		case "error_handling":
			errorHandling = "Needs improvement"
		case "structure":
			structure = "Needs improvement"
		}
	}

	// Concise output: only include high/medium impact suggestions
	var keySuggestions []reviewSuggestion
	for _, sug := range rankSuggestions(allSuggestions) {
		if sug.Impact == "high" || sug.Impact == "medium" {
			keySuggestions = append(keySuggestions, sug)
		}
	}
	if len(keySuggestions) == 0 {
		keySuggestions = allSuggestions
		if len(keySuggestions) > 5 {
			keySuggestions = keySuggestions[:5]
		}
	}

	review := codeReview{
		OverallScore:      totalScore,
		Maintainability:   fmt.Sprintf("%d/100 - %s", totalScore, maintainabilityLabel(totalScore)),
		NamingConventions: naming,
		ErrorHandling:     errorHandling,
		CodeStructure:     structure,
		Suggestions:       keySuggestions,
	}

	b, _ := json.MarshalIndent(review, "", "  ")
	return string(b), nil
}

func maintainabilityLabel(score int) string {
	switch {
	case score >= 90:
		return "Highly maintainable"
	case score >= 70:
		return "Maintainable with minor improvements"
	case score >= 50:
		return "Needs significant improvements"
	default:
		return "Hard to maintain - major refactoring recommended"
	}
}

func rankSuggestions(suggestions []reviewSuggestion) []reviewSuggestion {
	var high, med, low, info []reviewSuggestion
	for _, s := range suggestions {
		switch s.Impact {
		case "high":
			high = append(high, s)
		case "medium":
			med = append(med, s)
		case "low":
			low = append(low, s)
		default:
			info = append(info, s)
		}
	}
	result := append(high, med...)
	result = append(result, low...)
	result = append(result, info...)
	return result
}

var camelCaseRe = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`)
var snakeCaseRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
var pascalCaseRe = regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*$`)

func (s *ReviewCodeSkill) reviewFile(filePath, content, lang string) []reviewSuggestion {
	switch lang {
	case "shell":
		return s.reviewShell(filePath, content)
	case "rust":
		return s.reviewRust(filePath, content)
	case "go":
		return s.reviewGo(filePath, content)
	case "python":
		return s.reviewPython(filePath, content)
	case "javascript", "typescript":
		return s.reviewJavaScript(filePath, content)
	}
	return nil
}

func (s *ReviewCodeSkill) reviewShell(filePath, content string) []reviewSuggestion {
	var suggestions []reviewSuggestion
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, "function ") {
			suggestions = append(suggestions, reviewSuggestion{
				Category: "structure", Impact: "low", File: filePath, Line: i + 1,
				Message:    "Use 'func_name()' instead of 'function func_name()' for POSIX compatibility",
				Suggestion: "Remove 'function' keyword",
			})
		}

		if len(trimmed) > 120 {
			suggestions = append(suggestions, reviewSuggestion{
				Category: "structure", Impact: "low", File: filePath, Line: i + 1,
				Message:    "Line too long (>120 chars)",
				Suggestion: "Break line into multiple lines using backslash",
			})
		}

		if strings.Contains(trimmed, "rm -rf ") && strings.Contains(trimmed, "/") && !strings.Contains(trimmed, "$") {
			suggestions = append(suggestions, reviewSuggestion{
				Category: "security", Impact: "high", File: filePath, Line: i + 1,
				Message:    "rm -rf with hardcoded path - risk of accidental deletion",
				Suggestion: "Use variables for paths and add safety checks",
			})
		}

		if strings.HasPrefix(trimmed, "#") && i > 0 && strings.TrimSpace(lines[i-1]) != "" {
			if strings.HasPrefix(trimmed, "##") {
				continue
			}
		}
	}

	hasComments := false
	for _, line := range lines {
		if strings.Contains(strings.TrimSpace(line), "#") && !strings.HasPrefix(strings.TrimSpace(line), "#!") {
			hasComments = true
			break
		}
	}
	if !hasComments && len(lines) > 5 {
		suggestions = append(suggestions, reviewSuggestion{
			Category: "structure", Impact: "medium", File: filePath,
			Message:    "No comments found in script",
			Suggestion: "Add comments explaining complex logic and function purposes",
		})
	}

	return suggestions
}

func (s *ReviewCodeSkill) reviewRust(filePath, content string) []reviewSuggestion {
	var suggestions []reviewSuggestion
	lines := strings.Split(content, "\n")

	hasDocComments := false
	hasTests := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "///") || strings.HasPrefix(trimmed, "//!") {
			hasDocComments = true
		}

		if strings.HasPrefix(trimmed, "#[test]") || strings.HasPrefix(trimmed, "#[cfg(test)]") {
			hasTests = true
		}

		if strings.Contains(trimmed, "fn ") && !strings.HasPrefix(trimmed, "//") {
			if !strings.HasPrefix(trimmed, "///") && !strings.HasPrefix(trimmed, "//!") {
				prevLine := ""
				if i > 0 {
					prevLine = strings.TrimSpace(lines[i-1])
				}
				if !strings.HasPrefix(prevLine, "///") && !strings.HasPrefix(prevLine, "//!") && !strings.HasPrefix(prevLine, "#[") {
					suggestions = append(suggestions, reviewSuggestion{
						Category: "structure", Impact: "medium", File: filePath, Line: i + 1,
						Message:    "Public function missing doc comment (///)",
						Suggestion: "Add documentation comment describing purpose, arguments, and return value",
					})
				}
			}
		}

		if strings.Contains(trimmed, "fn ") && strings.Contains(trimmed, "pub ") {
			hasPublic := true
			_ = hasPublic
		}
	}

	if !hasDocComments && len(lines) > 10 {
		suggestions = append(suggestions, reviewSuggestion{
			Category: "structure", Impact: "medium", File: filePath,
			Message:    "No documentation comments found",
			Suggestion: "Add /// doc comments to all public items",
		})
	}

	if !hasTests {
		suggestions = append(suggestions, reviewSuggestion{
			Category: "structure", Impact: "low", File: filePath,
			Message:    "No tests found",
			Suggestion: "Add #[test] functions for unit testing",
		})
	}

	return suggestions
}

func (s *ReviewCodeSkill) reviewGo(filePath, content string) []reviewSuggestion {
	var suggestions []reviewSuggestion
	lines := strings.Split(content, "\n")

	hasComments := false
	hasErrorReturn := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			hasComments = true
		}

		if strings.Contains(trimmed, "error") && strings.Contains(trimmed, "return") {
			hasErrorReturn = true
		}

		if strings.Contains(trimmed, "var ") || strings.Contains(trimmed, "const ") {
			if !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "var ") && !strings.HasPrefix(trimmed, "const ") {
				continue
			}
		}

		if strings.Contains(trimmed, "http.Get") || strings.Contains(trimmed, "http.Post") || strings.Contains(trimmed, "http.DefaultClient") {
			suggestions = append(suggestions, reviewSuggestion{
				Category: "error_handling", Impact: "medium", File: filePath, Line: i + 1,
				Message:    "HTTP call without timeout context",
				Suggestion: "Use http.NewRequestWithContext with a proper timeout",
			})
		}

		if strings.Contains(trimmed, "defer ") && i+1 < len(lines) {
			nextLine := strings.TrimSpace(lines[i+1])
			if strings.Contains(nextLine, "return") {
				suggestions = append(suggestions, reviewSuggestion{
					Category: "structure", Impact: "low", File: filePath, Line: i + 1,
					Message:    "defer before immediate return",
					Suggestion: "Ensure deferred cleanup is intentional here",
				})
			}
		}
	}

	if !hasComments && len(lines) > 10 {
		suggestions = append(suggestions, reviewSuggestion{
			Category: "structure", Impact: "medium", File: filePath,
			Message:    "No comments in Go file",
			Suggestion: "Add comments to exported functions and complex logic",
		})
	}

	if !hasErrorReturn {
		suggestions = append(suggestions, reviewSuggestion{
			Category: "error_handling", Impact: "high", File: filePath,
			Message:    "No error returns found - ensure proper error propagation",
			Suggestion: "Return errors from functions that can fail",
		})
	}

	return suggestions
}

func (s *ReviewCodeSkill) reviewPython(filePath, content string) []reviewSuggestion {
	var suggestions []reviewSuggestion
	lines := strings.Split(content, "\n")

	hasTypeHints := false
	hasMainGuard := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, ": ") && strings.Contains(trimmed, "-> ") {
			hasTypeHints = true
		}

		if strings.Contains(trimmed, "if __name__") && strings.Contains(trimmed, "__main__") {
			hasMainGuard = true
		}

		if strings.Contains(trimmed, "import *") {
			suggestions = append(suggestions, reviewSuggestion{
				Category: "structure", Impact: "medium", File: filePath, Line: i + 1,
				Message:    "Wildcard import (import *) - pollutes namespace",
				Suggestion: "Import specific names instead",
			})
		}

		if strings.Contains(trimmed, "global ") {
			suggestions = append(suggestions, reviewSuggestion{
				Category: "structure", Impact: "low", File: filePath, Line: i + 1,
				Message:    "Use of global variable",
				Suggestion: "Pass values as parameters or use class attributes",
			})
		}

		if strings.Contains(trimmed, "print(") && filePath != "" && !strings.HasPrefix(trimmed, "#") {
			suggestions = append(suggestions, reviewSuggestion{
				Category: "structure", Impact: "low", File: filePath, Line: i + 1,
				Message:    "Use of print() - consider logging instead",
				Suggestion: "Use the logging module for production code",
			})
		}
	}

	if !hasMainGuard && len(lines) > 5 {
		suggestions = append(suggestions, reviewSuggestion{
			Category: "structure", Impact: "medium", File: filePath,
			Message:    "Missing if __name__ == '__main__' guard",
			Suggestion: "Add main guard to prevent code execution on import",
		})
	}

	if !hasTypeHints && len(lines) > 10 {
		suggestions = append(suggestions, reviewSuggestion{
			Category: "structure", Impact: "low", File: filePath,
			Message:    "No type hints found",
			Suggestion: "Add type hints for better readability and tooling support",
		})
	}

	return suggestions
}

func (s *ReviewCodeSkill) reviewJavaScript(filePath, content string) []reviewSuggestion {
	var suggestions []reviewSuggestion
	lines := strings.Split(content, "\n")

	hasStrict := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "\"use strict\";" || trimmed == "'use strict';" {
			hasStrict = true
		}

		if strings.Contains(trimmed, "==") && !strings.Contains(trimmed, "===") && !strings.Contains(trimmed, "==") {
			suggestions = append(suggestions, reviewSuggestion{
				Category: "error_handling", Impact: "medium", File: filePath, Line: i + 1,
				Message:    "Use == instead of ===",
				Suggestion: "Use === for strict equality comparison",
			})
		}

		if strings.Contains(trimmed, "var ") {
			suggestions = append(suggestions, reviewSuggestion{
				Category: "structure", Impact: "low", File: filePath, Line: i + 1,
				Message:    "Use of var - use let or const instead",
				Suggestion: "Replace var with const (or let if reassignment is needed)",
			})
		}
	}

	if !hasStrict && len(lines) > 3 {
		suggestions = append(suggestions, reviewSuggestion{
			Category: "error_handling", Impact: "medium", File: filePath,
			Message:    "Missing 'use strict'",
			Suggestion: "Add 'use strict'; at the top of the script",
		})
	}

	return suggestions
}

func (s *ReviewCodeSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  true,
		Essential: false,
		NeedsDB:   false,
		NeedsLLM:  true,
	}
}
