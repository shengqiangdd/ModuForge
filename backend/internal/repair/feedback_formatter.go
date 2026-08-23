package repair

import (
	"regexp"
	"strings"
)

// ErrorCategory classifies the type of error.
type ErrorCategory string

const (
	CategorySyntax     ErrorCategory = "syntax"
	CategoryLogic      ErrorCategory = "logic"
	CategoryConvention ErrorCategory = "convention"
)

// FormattedIssue is a processed error with actionable suggestion.
type FormattedIssue struct {
	OriginalError string        `json:"original_error"`
	Category      ErrorCategory `json:"category"`
	Suggestion    string        `json:"suggestion"`
	Confidence    float64       `json:"confidence"` // 0.0 - 1.0
}

// RetryStrategy defines how to handle retries.
type RetryStrategy struct {
	MaxRetries  int    `json:"max_retries"`
	ShouldRetry bool   `json:"should_retry"`
	Delay       string `json:"delay"`
	Reason      string `json:"reason"`
}

// FormattedFeedback is the complete formatted error report.
type FormattedFeedback struct {
	Summary       string           `json:"summary"`
	Issues        []FormattedIssue `json:"issues"`
	RetryStrategy RetryStrategy    `json:"retry_strategy"`
}

// LintIssue mirrors the quality.LintIssue type to avoid import cycles.
type LintIssue struct {
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Fix      string `json:"fix,omitempty"`
}

// FeedbackFormatter processes raw errors into actionable feedback.
type FeedbackFormatter struct{}

// NewFeedbackFormatter creates a new formatter.
func NewFeedbackFormatter() *FeedbackFormatter {
	return &FeedbackFormatter{}
}

// FormatBuildError processes stderr from a build failure.
func (f *FeedbackFormatter) FormatBuildError(stderr string) FormattedFeedback {
	if strings.TrimSpace(stderr) == "" {
		return FormattedFeedback{
			Summary:       "No error output captured",
			RetryStrategy: RetryStrategy{ShouldRetry: false},
		}
	}

	var issues []FormattedIssue
	lines := strings.Split(stderr, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		issue := classifyBuildError(line)
		if issue != nil {
			issues = append(issues, *issue)
		}
	}

	// Determine overall strategy
	strategy := determineRetryStrategy(issues)

	// Build summary
	summary := buildSummary(issues)

	return FormattedFeedback{
		Summary:       summary,
		Issues:        issues,
		RetryStrategy: strategy,
	}
}

// FormatLintIssues processes lint issues into actionable feedback.
func (f *FeedbackFormatter) FormatLintIssues(lintIssues []LintIssue) FormattedFeedback {
	var issues []FormattedIssue

	for _, li := range lintIssues {
		category := classifyLintRule(li.Rule)
		confidence := lintConfidence(li.Severity)

		suggestion := li.Fix
		if suggestion == "" {
			suggestion = li.Message
		}

		issues = append(issues, FormattedIssue{
			OriginalError: li.Message,
			Category:      category,
			Suggestion:    suggestion,
			Confidence:    confidence,
		})
	}

	strategy := determineRetryStrategy(issues)
	summary := buildSummary(issues)

	return FormattedFeedback{
		Summary:       summary,
		Issues:        issues,
		RetryStrategy: strategy,
	}
}

// ═══════════════════════════════════════════════════════
// Build error classification
// ═══════════════════════════════════════════════════════

func classifyBuildError(line string) *FormattedIssue {
	lower := strings.ToLower(line)

	// Go syntax errors
	if strings.Contains(lower, "syntax error") || strings.Contains(lower, "unexpected") {
		return &FormattedIssue{
			OriginalError: line,
			Category:      CategorySyntax,
			Suggestion:    "Fix syntax error",
			Confidence:    0.9,
		}
	}

	// Go compilation errors
	if strings.Contains(lower, "undefined") || strings.Contains(lower, "undeclared") {
		return &FormattedIssue{
			OriginalError: line,
			Category:      CategorySyntax,
			Suggestion:    "Add missing declaration or import",
			Confidence:    0.85,
		}
	}

	// Type errors
	if strings.Contains(lower, "cannot use") || strings.Contains(lower, "type mismatch") {
		return &FormattedIssue{
			OriginalError: line,
			Category:      CategoryLogic,
			Suggestion:    "Fix type mismatch",
			Confidence:    0.8,
		}
	}

	// Unused variables
	if strings.Contains(lower, "declared but not used") {
		return &FormattedIssue{
			OriginalError: line,
			Category:      CategorySyntax,
			Suggestion:    "Remove unused variable or use _ = varName",
			Confidence:    0.95,
		}
	}

	// Shell syntax errors
	if strings.Contains(lower, "syntax error near") || strings.Contains(lower, "unexpected token") {
		return &FormattedIssue{
			OriginalError: line,
			Category:      CategorySyntax,
			Suggestion:    "Fix shell syntax",
			Confidence:    0.9,
		}
	}

	// Convention warnings
	if strings.Contains(lower, "warning") || strings.Contains(lower, "deprecated") {
		return &FormattedIssue{
			OriginalError: line,
			Category:      CategoryConvention,
			Suggestion:    "Update deprecated usage",
			Confidence:    0.7,
		}
	}

	// Default: treat as logic error
	return &FormattedIssue{
		OriginalError: line,
		Category:      CategoryLogic,
		Suggestion:    "Review and fix the error",
		Confidence:    0.5,
	}
}

// ═══════════════════════════════════════════════════════
// Lint rule classification
// ═══════════════════════════════════════════════════════

func classifyLintRule(rule string) ErrorCategory {
	switch {
	case strings.Contains(rule, "shebang") || strings.Contains(rule, "variable-expansion"):
		return CategoryConvention
	case strings.Contains(rule, "dangerous") || strings.Contains(rule, "error-handling"):
		return CategoryLogic
	case strings.Contains(rule, "set-perm") || strings.Contains(rule, "modpath"):
		return CategorySyntax
	default:
		return CategoryConvention
	}
}

func lintConfidence(severity string) float64 {
	if severity == "error" {
		return 0.9
	}
	return 0.7
}

// ═══════════════════════════════════════════════════════
// Retry strategy
// ═══════════════════════════════════════════════════════

func determineRetryStrategy(issues []FormattedIssue) RetryStrategy {
	if len(issues) == 0 {
		return RetryStrategy{
			MaxRetries:  0,
			ShouldRetry: false,
			Delay:       "0s",
			Reason:      "no issues found",
		}
	}

	// Find dominant category
	syntax, logic, convention := 0, 0, 0
	for _, issue := range issues {
		switch issue.Category {
		case CategorySyntax:
			syntax++
		case CategoryLogic:
			logic++
		case CategoryConvention:
			convention++
		}
	}

	// Strategy based on dominant error type
	switch {
	case syntax >= logic && syntax >= convention:
		return RetryStrategy{
			MaxRetries:  3,
			ShouldRetry: true,
			Delay:       "2s",
			Reason:      "syntax errors are typically fixable with retries",
		}
	case logic >= syntax && logic >= convention:
		return RetryStrategy{
			MaxRetries:  2,
			ShouldRetry: true,
			Delay:       "5s",
			Reason:      "logic errors may need different approach",
		}
	default:
		return RetryStrategy{
			MaxRetries:  1,
			ShouldRetry: true,
			Delay:       "1s",
			Reason:      "convention issues are quick fixes",
		}
	}
}

// ═══════════════════════════════════════════════════════
// Summary
// ═══════════════════════════════════════════════════════

func buildSummary(issues []FormattedIssue) string {
	if len(issues) == 0 {
		return "No issues found"
	}

	syntax, logic, convention := 0, 0, 0
	for _, issue := range issues {
		switch issue.Category {
		case CategorySyntax:
			syntax++
		case CategoryLogic:
			logic++
		case CategoryConvention:
			convention++
		}
	}

	parts := []string{}
	if syntax > 0 {
		parts = append(parts, formatCount(syntax, "syntax error"))
	}
	if logic > 0 {
		parts = append(parts, formatCount(logic, "logic error"))
	}
	if convention > 0 {
		parts = append(parts, formatCount(convention, "convention issue"))
	}

	return strings.Join(parts, ", ")
}

func formatCount(n int, label string) string {
	if n == 1 {
		return "1 " + label
	}
	return regexp.MustCompile(`$`).ReplaceAllString(label, "s") // simple pluralization
}
