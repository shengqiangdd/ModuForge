package agent

import (
	"regexp"
	"strings"
)

// CodeQualityValidator 代码质量验证器
type CodeQualityValidator struct {
	runner *AgentRunner
}

func float64Max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func float64Min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// QualityResult 质量验证结果
type QualityResult struct {
	Score       float64        `json:"score"` // 0-100
	Issues      []QualityIssue `json:"issues"`
	Suggestions []string       `json:"suggestions"`
	Metrics     QualityMetrics `json:"metrics"`
}

// QualityIssue 质量问题
type QualityIssue struct {
	Severity string `json:"severity"` // critical, high, medium, low
	Message  string `json:"message"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Rule     string `json:"rule"`
}

// QualityMetrics 质量指标
type QualityMetrics struct {
	LinesOfCode     int     `json:"lines_of_code"`
	Complexity      float64 `json:"complexity"`
	Maintainability float64 `json:"maintainability"`
	SecurityScore   float64 `json:"security_score"`
}

// NewCodeQualityValidator 创建代码质量验证器
func NewCodeQualityValidator(runner *AgentRunner) *CodeQualityValidator {
	return &CodeQualityValidator{runner: runner}
}

// Validate 验证代码质量
func (v *CodeQualityValidator) Validate(code string, language string) *QualityResult {
	result := &QualityResult{
		Score:       100,
		Issues:      make([]QualityIssue, 0),
		Suggestions: make([]string, 0),
		Metrics:     QualityMetrics{},
	}

	// 基础指标
	lines := strings.Split(code, "\n")
	result.Metrics.LinesOfCode = len(lines)

	// 检查安全问题
	v.checkSecurityIssues(code, language, result)

	// 检查代码风格
	v.checkCodeStyle(code, language, result)

	// 检查复杂度
	v.checkComplexity(code, language, result)

	// 计算最终分数
	if len(result.Issues) > 0 {
		deduction := 0.0
		for _, issue := range result.Issues {
			switch issue.Severity {
			case "critical":
				deduction += 25
			case "high":
				deduction += 15
			case "medium":
				deduction += 10
			case "low":
				deduction += 5
			}
		}
		result.Score = float64Max(0, result.Score-deduction)
	}

	// 安全分数
	result.Metrics.SecurityScore = float64Max(0, 100-deduction)

	return result
}

// checkSecurityIssues 检查安全问题
func (v *CodeQualityValidator) checkSecurityIssues(code, language string, result *QualityResult) {
	// 检查硬编码密码
	if matched, _ := regexp.MatchString(`(?i)(password|secret|api_key)\s*=\s*["'][^"']+["']`, code); matched {
		result.Issues = append(result.Issues, QualityIssue{
			Severity: "critical",
			Message:  "Hardcoded credentials detected",
			Rule:     "security-no-hardcoded-credentials",
		})
		result.Suggestions = append(result.Suggestions, "Use environment variables or secret management")
	}

	// 检查SQL注入风险
	if strings.Contains(code, "fmt.Sprintf") && strings.Contains(code, "SELECT") {
		result.Issues = append(result.Issues, QualityIssue{
			Severity: "high",
			Message:  "Potential SQL injection via string formatting",
			Rule:     "security-sql-injection",
		})
		result.Suggestions = append(result.Suggestions, "Use parameterized queries instead of string formatting")
	}

	// 检查不安全的随机数
	if language == "go" && strings.Contains(code, "math/rand") && !strings.Contains(code, "crypto/rand") {
		result.Issues = append(result.Issues, QualityIssue{
			Severity: "medium",
			Message:  "Using math/rand instead of crypto/rand for security-sensitive operations",
			Rule:     "security-insecure-random",
		})
		result.Suggestions = append(result.Suggestions, "Use crypto/rand for security-sensitive random numbers")
	}
}

// checkCodeStyle 检查代码风格
func (v *CodeQualityValidator) checkCodeStyle(code, language string, result *QualityResult) {
	lines := strings.Split(code, "\n")

	// 检查行长度
	for i, line := range lines {
		if len(line) > 120 {
			result.Issues = append(result.Issues, QualityIssue{
				Severity: "low",
				Message:  "Line exceeds 120 characters",
				Line:     i + 1,
				Rule:     "style-line-length",
			})
		}
	}

	// 检查函数长度
	funcRegex := regexp.MustCompile(`func\s+\w+\s*\([^)]*\)\s*[^{]*\{`)
	funcMatches := funcRegex.FindAllStringIndex(code, -1)

	for _, match := range funcMatches {
		startLine := len(code[:match[0]])
		// 简单估算函数长度
		if startLine > 50 {
			result.Issues = append(result.Issues, QualityIssue{
				Severity: "medium",
				Message:  "Function is too long",
				Rule:     "style-function-length",
			})
			result.Suggestions = append(result.Suggestions, "Consider breaking down long functions into smaller ones")
		}
	}
}

// checkComplexity 检查复杂度
func (v *CodeQualityValidator) checkComplexity(code, language string, result *QualityResult) {
	// 简单的圈复杂度估算
	complexity := 1.0

	// 增加分支复杂度
	branchKeywords := []string{"if", "else", "for", "switch", "case", "&&", "||"}
	for _, keyword := range branchKeywords {
		complexity += float64(strings.Count(code, keyword))
	}

	result.Metrics.Complexity = complexity

	// 可维护性指数 (简化版)
	maintainability := 100.0 - (complexity * 2) - float64(result.Metrics.LinesOfCode)*0.1
	result.Metrics.Maintainability = float64Max(0, float64Min(100, maintainability))

	if complexity > 20 {
		result.Issues = append(result.Issues, QualityIssue{
			Severity: "high",
			Message:  "High cyclomatic complexity detected",
			Rule:     "complexity-cyclomatic",
		})
		result.Suggestions = append(result.Suggestions, "Reduce complexity by extracting methods or simplifying logic")
	}
}
