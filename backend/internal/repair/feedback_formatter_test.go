package repair

import (
	"strings"
	"testing"
)

func TestNewFeedbackFormatter(t *testing.T) {
	f := NewFeedbackFormatter()
	if f == nil {
		t.Fatal("expected non-nil formatter")
	}
}

func TestFormatBuildError_Empty(t *testing.T) {
	f := NewFeedbackFormatter()

	fb := f.FormatBuildError("")
	if fb.RetryStrategy.ShouldRetry {
		t.Error("should not retry for empty error")
	}
}

func TestFormatBuildError_Syntax(t *testing.T) {
	f := NewFeedbackFormatter()

	stderr := `main.go:5:1: syntax error: unexpected newline
main.go:10:2: syntax error: unexpected }
`

	fb := f.FormatBuildError(stderr)

	if len(fb.Issues) < 2 {
		t.Fatalf("expected at least 2 issues, got %d", len(fb.Issues))
	}

	// Should be classified as syntax
	for _, issue := range fb.Issues {
		if issue.Category != CategorySyntax {
			t.Errorf("expected syntax category, got %s", issue.Category)
		}
	}

	// Should retry
	if !fb.RetryStrategy.ShouldRetry {
		t.Error("should retry for syntax errors")
	}

	if fb.RetryStrategy.MaxRetries < 2 {
		t.Errorf("expected at least 2 retries, got %d", fb.RetryStrategy.MaxRetries)
	}
}

func TestFormatBuildError_TypeError(t *testing.T) {
	f := NewFeedbackFormatter()

	stderr := `main.go:8:5: cannot use "hello" (untyped string) as int`

	fb := f.FormatBuildError(stderr)

	if len(fb.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(fb.Issues))
	}

	if fb.Issues[0].Category != CategoryLogic {
		t.Errorf("expected logic category, got %s", fb.Issues[0].Category)
	}
}

func TestFormatBuildError_Undefined(t *testing.T) {
	f := NewFeedbackFormatter()

	stderr := `main.go:3:2: undefined: fmt`

	fb := f.FormatBuildError(stderr)

	if len(fb.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(fb.Issues))
	}

	if fb.Issues[0].Category != CategorySyntax {
		t.Errorf("expected syntax category, got %s", fb.Issues[0].Category)
	}

	if fb.Issues[0].Confidence < 0.8 {
		t.Errorf("expected high confidence, got %f", fb.Issues[0].Confidence)
	}
}

func TestFormatBuildError_ShellSyntax(t *testing.T) {
	f := NewFeedbackFormatter()

	stderr := `test.sh:3: syntax error near unexpected token 'fi'`

	fb := f.FormatBuildError(stderr)

	if len(fb.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(fb.Issues))
	}

	if fb.Issues[0].Category != CategorySyntax {
		t.Errorf("expected syntax category, got %s", fb.Issues[0].Category)
	}
}

func TestFormatBuildError_Warning(t *testing.T) {
	f := NewFeedbackFormatter()

	stderr := `main.go:1:1: warning: unused import "fmt"`

	fb := f.FormatBuildError(stderr)

	if len(fb.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(fb.Issues))
	}

	if fb.Issues[0].Category != CategoryConvention {
		t.Errorf("expected convention category, got %s", fb.Issues[0].Category)
	}
}

func TestFormatBuildError_Summary(t *testing.T) {
	f := NewFeedbackFormatter()

	stderr := `main.go:5:1: syntax error
main.go:8:5: cannot use "hello" as int
main.go:10:2: syntax error: unexpected }`

	fb := f.FormatBuildError(stderr)

	if fb.Summary == "" {
		t.Error("expected non-empty summary")
	}

	if !strings.Contains(fb.Summary, "syntax error") {
		t.Errorf("summary should mention syntax errors: %s", fb.Summary)
	}
}

func TestFormatLintIssues(t *testing.T) {
	f := NewFeedbackFormatter()

	lintIssues := []LintIssue{
		{File: "test.sh", Rule: "shell-shebang", Severity: "warning", Message: "missing shebang"},
		{File: "test.sh", Rule: "variable-expansion", Severity: "error", Message: "use ${VAR}"},
	}

	fb := f.FormatLintIssues(lintIssues)

	if len(fb.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(fb.Issues))
	}

	if fb.Summary == "" {
		t.Error("expected non-empty summary")
	}

	// Should retry for convention issues
	if !fb.RetryStrategy.ShouldRetry {
		t.Error("should retry for lint issues")
	}
}

func TestFormatLintIssues_Empty(t *testing.T) {
	f := NewFeedbackFormatter()

	fb := f.FormatLintIssues(nil)

	if len(fb.Issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(fb.Issues))
	}

	if fb.RetryStrategy.ShouldRetry {
		t.Error("should not retry for no issues")
	}
}

func TestFormatLintIssues_WithFix(t *testing.T) {
	f := NewFeedbackFormatter()

	lintIssues := []LintIssue{
		{
			File:     "test.sh",
			Rule:     "no-dangerous-rm",
			Severity: "error",
			Message:  "dangerous rm detected",
			Fix:      "rm -rf ${MODPATH}",
		},
	}

	fb := f.FormatLintIssues(lintIssues)

	if len(fb.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(fb.Issues))
	}

	if fb.Issues[0].Suggestion != "rm -rf ${MODPATH}" {
		t.Errorf("expected fix suggestion, got %s", fb.Issues[0].Suggestion)
	}
}

func TestRetryStrategy_Syntax(t *testing.T) {
	f := NewFeedbackFormatter()

	stderr := `main.go:5:1: syntax error
main.go:10:2: syntax error
main.go:15:3: syntax error`

	fb := f.FormatBuildError(stderr)

	// Syntax errors should allow 3 retries
	if fb.RetryStrategy.MaxRetries < 3 {
		t.Errorf("expected at least 3 retries for syntax, got %d", fb.RetryStrategy.MaxRetries)
	}
}

func TestRetryStrategy_Logic(t *testing.T) {
	f := NewFeedbackFormatter()

	stderr := `main.go:8:5: cannot use "hello" as int
main.go:12:3: type mismatch`

	fb := f.FormatBuildError(stderr)

	// Logic errors should allow 2 retries
	if fb.RetryStrategy.MaxRetries < 2 || fb.RetryStrategy.MaxRetries > 3 {
		t.Errorf("expected 2-3 retries for logic, got %d", fb.RetryStrategy.MaxRetries)
	}
}

func TestRetryStrategy_Convention(t *testing.T) {
	f := NewFeedbackFormatter()

	stderr := `main.go:1:1: warning: deprecated function`

	fb := f.FormatBuildError(stderr)

	// Convention issues should allow 1 retry
	if fb.RetryStrategy.MaxRetries < 1 {
		t.Errorf("expected at least 1 retry for convention, got %d", fb.RetryStrategy.MaxRetries)
	}
}

func TestFormattedIssue_Confidence(t *testing.T) {
	f := NewFeedbackFormatter()

	stderr := `main.go:5:1: syntax error: unexpected`

	fb := f.FormatBuildError(stderr)

	for _, issue := range fb.Issues {
		if issue.Confidence <= 0 || issue.Confidence > 1 {
			t.Errorf("confidence should be 0-1, got %f", issue.Confidence)
		}
	}
}
