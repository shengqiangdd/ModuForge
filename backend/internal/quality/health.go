package quality

import (
	"regexp"
	"strings"
)

// IssueType categorizes health issues.
type IssueType string

const (
	IssueComplexity  IssueType = "complexity"
	IssueStyle       IssueType = "style"
	IssuePerformance IssueType = "performance"
)

// Severity levels.
const (
	SevError   = "error"
	SevWarning = "warning"
	SevInfo    = "info"
)

// Thresholds for health checks.
const (
	MaxCyclomaticComplexity = 10
	MaxLineCount            = 500
	MaxFunctionCount        = 20
)

// HealthIssue represents a single code health problem.
type HealthIssue struct {
	Type        IssueType `json:"type"`
	Line        int       `json:"line,omitempty"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	Suggestion  string    `json:"suggestion,omitempty"`
}

// HealthReport is the result of code health analysis.
type HealthReport struct {
	CyclomaticComplexity int           `json:"cyclomatic_complexity"`
	LineCount            int           `json:"line_count"`
	FunctionCount        int           `json:"function_count"`
	Issues               []HealthIssue `json:"issues"`
	Score                float64       `json:"score"` // 0-100
}

// CodeHealthAnalyzer analyzes code quality metrics.
type CodeHealthAnalyzer struct{}

// NewCodeHealthAnalyzer creates a new analyzer.
func NewCodeHealthAnalyzer() *CodeHealthAnalyzer {
	return &CodeHealthAnalyzer{}
}

// AnalyzeGoCode analyzes Go source code and returns a health report.
func (a *CodeHealthAnalyzer) AnalyzeGoCode(code string) HealthReport {
	lines := strings.Split(code, "\n")
	report := HealthReport{
		LineCount: len(lines),
	}

	// Count functions
	report.FunctionCount = countGoFunctions(code)

	// Calculate cyclomatic complexity
	report.CyclomaticComplexity = calcGoComplexity(code)

	// Find issues
	report.Issues = append(report.Issues, a.checkGoComplexity(report.CyclomaticComplexity)...)
	report.Issues = append(report.Issues, a.checkLineCount(report.LineCount)...)
	report.Issues = append(report.Issues, a.checkFunctionCount(report.FunctionCount)...)
	report.Issues = append(report.Issues, a.checkGoStyle(code, lines)...)

	// Calculate score (0-100)
	report.Score = calcScore(report)

	return report
}

// AnalyzeShellCode analyzes Shell script and returns a health report.
func (a *CodeHealthAnalyzer) AnalyzeShellCode(code string) HealthReport {
	lines := strings.Split(code, "\n")
	report := HealthReport{
		LineCount: len(lines),
	}

	// Count functions
	report.FunctionCount = countShellFunctions(code)

	// Calculate cyclomatic complexity
	report.CyclomaticComplexity = calcShellComplexity(code)

	// Find issues
	report.Issues = append(report.Issues, a.checkShellComplexity(report.CyclomaticComplexity)...)
	report.Issues = append(report.Issues, a.checkLineCount(report.LineCount)...)
	report.Issues = append(report.Issues, a.checkShellStyle(code, lines)...)

	report.Score = calcScore(report)

	return report
}

// ═══════════════════════════════════════════════════════
// Go analysis helpers
// ═══════════════════════════════════════════════════════

var goFuncRe = regexp.MustCompile(`(?m)^func\s+(?:\(\s*\w+\s+\S+\s*\)\s+)?(\w+)`)

func countGoFunctions(code string) int {
	return len(goFuncRe.FindAllString(code, -1))
}

func calcGoComplexity(code string) int {
	complexity := 1
	decisionPoints := []string{
		`\bif\b`, `\belse\s+if\b`, `\bfor\b`, `\brange\b`,
		`\bswitch\b`, `\bcase\b`, `\bselect\b`,
		`\&\&`, `\|\|`,
		`\bcase\s+`, // switch cases
	}

	for _, pattern := range decisionPoints {
		re := regexp.MustCompile(pattern)
		complexity += len(re.FindAllString(code, -1))
	}

	return complexity
}

func (a *CodeHealthAnalyzer) checkGoComplexity(complexity int) []HealthIssue {
	var issues []HealthIssue
	if complexity > MaxCyclomaticComplexity {
		issues = append(issues, HealthIssue{
			Type:        IssueComplexity,
			Severity:    SevError,
			Description: "cyclomatic complexity exceeds threshold",
			Suggestion:  "refactor into smaller functions",
		})
	}
	return issues
}

func (a *CodeHealthAnalyzer) checkGoStyle(code string, lines []string) []HealthIssue {
	var issues []HealthIssue

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for bare $VAR (shouldn't appear in Go, but common mistake)
		if strings.Contains(line, "$") && !strings.Contains(line, "$\"") {
			issues = append(issues, HealthIssue{
				Type:        IssueStyle,
				Line:        i + 1,
				Severity:    SevWarning,
				Description: "shell variable syntax in Go code",
				Suggestion:  "remove $ prefix for Go variables",
			})
		}

		// Check for TODO/FIXME
		if strings.Contains(strings.ToUpper(trimmed), "TODO") || strings.Contains(strings.ToUpper(trimmed), "FIXME") {
			issues = append(issues, HealthIssue{
				Type:        IssueStyle,
				Line:        i + 1,
				Severity:    SevInfo,
				Description: "TODO/FIXME comment found",
				Suggestion:  "address before production release",
			})
		}
	}

	return issues
}

// ═══════════════════════════════════════════════════════
// Shell analysis helpers
// ═══════════════════════════════════════════════════════

var shellFuncRe = regexp.MustCompile(`(?m)^\w+\s*\(\)\s*\{`)

func countShellFunctions(code string) int {
	return len(shellFuncRe.FindAllString(code, -1))
}

func calcShellComplexity(code string) int {
	complexity := 1
	lines := strings.Split(code, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Shell decision points
		if strings.Contains(trimmed, "if ") || strings.Contains(trimmed, "if[") {
			complexity++
		}
		if strings.Contains(trimmed, "elif ") || strings.Contains(trimmed, "elif[") {
			complexity++
		}
		if strings.Contains(trimmed, "case ") {
			complexity++
		}
		if strings.Contains(trimmed, "&&") || strings.Contains(trimmed, "||") {
			complexity++
		}
	}

	return complexity
}

func (a *CodeHealthAnalyzer) checkShellComplexity(complexity int) []HealthIssue {
	var issues []HealthIssue
	if complexity > MaxCyclomaticComplexity {
		issues = append(issues, HealthIssue{
			Type:        IssueComplexity,
			Severity:    SevWarning,
			Description: "shell script complexity is high",
			Suggestion:  "break into smaller functions or simplify conditionals",
		})
	}
	return issues
}

func (a *CodeHealthAnalyzer) checkShellStyle(code string, lines []string) []HealthIssue {
	var issues []HealthIssue

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check shebang
		if i == 0 && !strings.HasPrefix(trimmed, "#!/") {
			issues = append(issues, HealthIssue{
				Type:        IssueStyle,
				Line:        1,
				Severity:    SevWarning,
				Description: "missing shebang line",
				Suggestion:  "add #!/system/bin/sh at the beginning",
			})
		}

		// Check bare $VAR
		re := regexp.MustCompile(`\$[A-Za-z_][A-Za-z0-9_]*[^{(]`)
		if re.MatchString(trimmed) && !strings.HasPrefix(trimmed, "#") {
			issues = append(issues, HealthIssue{
				Type:        IssueStyle,
				Line:        i + 1,
				Severity:    SevError,
				Description: "use ${VAR} instead of bare $VAR",
				Suggestion:  "replace $VAR with ${VAR}",
			})
		}
	}

	return issues
}

// ═══════════════════════════════════════════════════════
// Common helpers
// ═══════════════════════════════════════════════════════

func (a *CodeHealthAnalyzer) checkLineCount(lineCount int) []HealthIssue {
	var issues []HealthIssue
	if lineCount > MaxLineCount {
		issues = append(issues, HealthIssue{
			Type:        IssueComplexity,
			Severity:    SevWarning,
			Description: "file exceeds recommended line count",
			Suggestion:  "consider splitting into multiple files",
		})
	}
	return issues
}

func (a *CodeHealthAnalyzer) checkFunctionCount(funcCount int) []HealthIssue {
	var issues []HealthIssue
	if funcCount > MaxFunctionCount {
		issues = append(issues, HealthIssue{
			Type:        IssueComplexity,
			Severity:    SevWarning,
			Description: "too many functions in single file",
			Suggestion:  "consider splitting into packages or modules",
		})
	}
	return issues
}

func calcScore(report HealthReport) float64 {
	score := 100.0

	// Deduct for complexity
	if report.CyclomaticComplexity > 5 {
		score -= float64(report.CyclomaticComplexity-5) * 5
	}

	// Deduct for line count
	if report.LineCount > 200 {
		score -= float64(report.LineCount-200) * 0.1
	}

	// Deduct for issues
	for _, issue := range report.Issues {
		switch issue.Severity {
		case SevError:
			score -= 15
		case SevWarning:
			score -= 5
		case SevInfo:
			score -= 1
		}
	}

	if score < 0 {
		score = 0
	}

	return score
}
