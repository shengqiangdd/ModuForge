package code

import (
	"fmt"
	"regexp"
	"strings"
)

// ReviewEngine 代码审查引擎
type ReviewEngine struct {
	analyzer *MultiLangAnalyzer
}

// NewReviewEngine 创建审查引擎
func NewReviewEngine() *ReviewEngine {
	return &ReviewEngine{
		analyzer: NewMultiLangAnalyzer(),
	}
}

// ReviewIssue 审查问题
type ReviewIssue struct {
	Category    string `json:"category"`    // security, performance, style, bug, best_practice
	Severity    string `json:"severity"`    // critical, high, medium, low
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Line        int    `json:"line"`
	Suggestion  string `json:"suggestion"`
}

// ReviewResult 审查结果
type ReviewResult struct {
	Issues  []ReviewIssue `json:"issues"`
	Score   int           `json:"score"`
	Summary string        `json:"summary"`
	Stats   ReviewStats   `json:"stats"`
}

// ReviewStats 审查统计
type ReviewStats struct {
	TotalIssues    int `json:"total_issues"`
	CriticalIssues int `json:"critical_issues"`
	HighIssues     int `json:"high_issues"`
	MediumIssues   int `json:"medium_issues"`
	LowIssues      int `json:"low_issues"`
}

// ReviewCode 审查代码
func (r *ReviewEngine) ReviewCode(code string, language string) (*ReviewResult, error) {
	lines := strings.Split(code, "\n")
	issues := make([]ReviewIssue, 0)

	// 安全检查
	issues = append(issues, r.checkSecurity(code, language, lines)...)

	// 性能检查
	issues = append(issues, r.checkPerformance(code, language, lines)...)

	// 风格检查
	issues = append(issues, r.checkStyle(code, language, lines)...)

	// 最佳实践检查
	issues = append(issues, r.checkBestPractices(code, language, lines)...)

	// 计算统计
	stats := ReviewStats{
		TotalIssues: len(issues),
	}
	for _, issue := range issues {
		switch issue.Severity {
		case "critical":
			stats.CriticalIssues++
		case "high":
			stats.HighIssues++
		case "medium":
			stats.MediumIssues++
		case "low":
			stats.LowIssues++
		}
	}

	// 计算评分
	score := 100
	for _, issue := range issues {
		switch issue.Severity {
		case "critical":
			score -= 25
		case "high":
			score -= 15
		case "medium":
			score -= 8
		case "low":
			score -= 3
		}
	}
	if score < 0 {
		score = 0
	}

	summary := fmt.Sprintf("发现 %d 个问题（严重: %d, 高: %d, 中: %d, 低: %d），评分: %d/100",
		stats.TotalIssues, stats.CriticalIssues, stats.HighIssues, stats.MediumIssues, stats.LowIssues, score)

	return &ReviewResult{
		Issues:  issues,
		Score:   score,
		Summary: summary,
		Stats:   stats,
	}, nil
}

func (r *ReviewEngine) checkSecurity(code string, language string, lines []string) []ReviewIssue {
	issues := make([]ReviewIssue, 0)

	// 检查硬编码密码
	passwordRegex := regexp.MustCompile(`(?i)(password|secret|key|token)\s*[:=]\s*["'][^"']+["']`)
	for i, line := range lines {
		if passwordRegex.MatchString(line) {
			issues = append(issues, ReviewIssue{
				Category:    "security",
				Severity:    "critical",
				Title:       "硬编码凭证",
				Description: "代码中包含硬编码的密码或密钥",
				Location:    fmt.Sprintf("第 %d 行", i+1),
				Line:        i + 1,
				Suggestion:  "使用环境变量或密钥管理服务存储敏感信息",
			})
		}
	}

	// 检查SQL注入
	if strings.Contains(code, "fmt.Sprintf") && strings.Contains(code, "SELECT") {
		issues = append(issues, ReviewIssue{
			Category:    "security",
			Severity:    "critical",
			Title:       "潜在SQL注入",
			Description: "使用字符串拼接构建SQL查询",
			Suggestion:  "使用参数化查询或预编译语句",
		})
	}

	// 检查不安全的随机数
	if language == "go" && strings.Contains(code, "math/rand") && !strings.Contains(code, "crypto/rand") {
		issues = append(issues, ReviewIssue{
			Category:    "security",
			Severity:    "high",
			Title:       "不安全的随机数",
			Description: "使用 math/rand 而非 crypto/rand",
			Suggestion:  "对于安全敏感场景，使用 crypto/rand",
		})
	}

	return issues
}

func (r *ReviewEngine) checkPerformance(code string, language string, lines []string) []ReviewIssue {
	issues := make([]ReviewIssue, 0)

	// 检查循环中的字符串拼接
	if strings.Contains(code, "for") && strings.Contains(code, "+=") && strings.Contains(code, "string") {
		issues = append(issues, ReviewIssue{
			Category:    "performance",
			Severity:    "medium",
			Title:       "循环中字符串拼接",
			Description: "在循环中使用 + 拼接字符串会影响性能",
			Suggestion:  "使用 strings.Builder 或 bytes.Buffer",
		})
	}

	// 检查未使用的变量（简单检查）
	varRegex := regexp.MustCompile(`:=\s*\w+\s*$`)
	for i, line := range lines {
		if varRegex.MatchString(strings.TrimSpace(line)) {
			issues = append(issues, ReviewIssue{
				Category:    "performance",
				Severity:    "low",
				Title:       "可能未使用的变量",
				Description: fmt.Sprintf("第 %d 行定义的变量可能未被使用", i+1),
				Location:    fmt.Sprintf("第 %d 行", i+1),
				Line:        i + 1,
			})
		}
	}

	return issues
}

func (r *ReviewEngine) checkStyle(code string, language string, lines []string) []ReviewIssue {
	issues := make([]ReviewIssue, 0)

	// 检查行长度
	for i, line := range lines {
		if len(line) > 120 {
			issues = append(issues, ReviewIssue{
				Category:    "style",
				Severity:    "low",
				Title:       "行过长",
				Description: fmt.Sprintf("第 %d 行超过 120 字符", i+1),
				Location:    fmt.Sprintf("第 %d 行", i+1),
				Line:        i + 1,
				Suggestion:  "拆分为多行或提取变量",
			})
		}
	}

	// 检查函数长度
	funcCount := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "func ") {
			funcCount++
		}
	}
	if funcCount > 50 {
		issues = append(issues, ReviewIssue{
			Category:    "style",
			Severity:    "medium",
			Title:       "文件函数过多",
			Description: fmt.Sprintf("文件包含 %d 个函数，建议拆分", funcCount),
			Suggestion:  "按功能模块拆分文件",
		})
	}

	return issues
}

func (r *ReviewEngine) checkBestPractices(code string, language string, lines []string) []ReviewIssue {
	issues := make([]ReviewIssue, 0)

	// 检查错误处理（Go）
	if language == "go" {
		errIgnoreRegex := regexp.MustCompile(`_\s*=\s*\w+\(`)
		for i, line := range lines {
			if errIgnoreRegex.MatchString(line) {
				issues = append(issues, ReviewIssue{
					Category:    "best_practice",
					Severity:    "medium",
					Title:       "忽略错误返回值",
					Description: fmt.Sprintf("第 %d 行忽略了函数返回值", i+1),
					Location:    fmt.Sprintf("第 %d 行", i+1),
					Line:        i + 1,
					Suggestion:  "处理错误返回值或使用 _ 显式忽略",
				})
			}
		}
	}

	// 检查TODO注释
	for i, line := range lines {
		if strings.Contains(strings.ToUpper(line), "TODO") {
			issues = append(issues, ReviewIssue{
				Category:    "best_practice",
				Severity:    "low",
				Title:       "TODO注释",
				Description: fmt.Sprintf("第 %d 行包含TODO注释", i+1),
				Location:    fmt.Sprintf("第 %d 行", i+1),
				Line:        i + 1,
				Suggestion:  "完成TODO项或创建issue跟踪",
			})
		}
	}

	return issues
}
