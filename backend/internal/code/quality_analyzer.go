package code

import (
	"fmt"
	"strings"
)

// QualityAnalyzer 代码质量分析器
type QualityAnalyzer struct {
	multiLangAnalyzer *MultiLangAnalyzer
	reviewEngine      *ReviewEngine
	refactorEngine    *RefactorEngine
}

// NewQualityAnalyzer 创建质量分析器
func NewQualityAnalyzer() *QualityAnalyzer {
	return &QualityAnalyzer{
		multiLangAnalyzer: NewMultiLangAnalyzer(),
		reviewEngine:      NewReviewEngine(),
		refactorEngine:    NewRefactorEngine(),
	}
}

// QualityReport 质量报告
type QualityReport struct {
	Summary     QualitySummary      `json:"summary"`
	Metrics     QualityMetrics      `json:"metrics"`
	Issues      []QualityIssue      `json:"issues"`
	Suggestions []RefactorSuggestion `json:"suggestions"`
	Score       int                 `json:"score"`
	Grade       string              `json:"grade"`
}

// QualitySummary 质量摘要
type QualitySummary struct {
	TotalFiles     int    `json:"total_files"`
	TotalLines     int    `json:"total_lines"`
	TotalFunctions int    `json:"total_functions"`
	TotalIssues    int    `json:"total_issues"`
	AnalysisTime   string `json:"analysis_time"`
}

// QualityMetrics 质量指标
type QualityMetrics struct {
	AverageComplexity float64        `json:"average_complexity"`
	MaxComplexity     int            `json:"max_complexity"`
	AverageLineLength float64        `json:"average_line_length"`
	CommentRatio      float64        `json:"comment_ratio"`
	DuplicationRatio  float64        `json:"duplication_ratio"`
	TestCoverage      float64        `json:"test_coverage"`
	Maintainability   float64        `json:"maintainability"`
	SecurityScore     float64        `json:"security_score"`
	PerformanceScore  float64        `json:"performance_score"`
	ByLanguage        map[string]int `json:"by_language"`
}

// QualityIssue 质量问题
type QualityIssue struct {
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Count       int    `json:"count"`
}

// AnalyzeProject 分析项目
func (q *QualityAnalyzer) AnalyzeProject(files map[string]string, language string) (*QualityReport, error) {
	report := &QualityReport{
		Summary:     QualitySummary{},
		Metrics:     QualityMetrics{ByLanguage: make(map[string]int)},
		Issues:      make([]QualityIssue, 0),
		Suggestions: make([]RefactorSuggestion, 0),
	}

	totalLines := 0
	totalComplexity := 0
	maxComplexity := 0
	totalFunctions := 0

	for fileName, code := range files {
		_ = fileName
		// 分析代码结构
		result, err := q.multiLangAnalyzer.Analyze(code, language)
		if err != nil {
			continue
		}

		report.Summary.TotalFiles++
		totalLines += result.Lines
		totalFunctions += len(result.Functions)

		// 计算复杂度
		for _, f := range result.Functions {
			totalComplexity += f.Complexity
			if f.Complexity > maxComplexity {
				maxComplexity = f.Complexity
			}
		}

		// 审查代码
		reviewResult, err := q.reviewEngine.ReviewCode(code, language)
		if err == nil {
			for _, issue := range reviewResult.Issues {
				found := false
				for i := range report.Issues {
					if report.Issues[i].Category == issue.Category && report.Issues[i].Severity == issue.Severity {
						report.Issues[i].Count++
						found = true
						break
					}
				}
				if !found {
					report.Issues = append(report.Issues, QualityIssue{
						Category:    issue.Category,
						Severity:    issue.Severity,
						Title:       issue.Title,
						Description: issue.Description,
						Count:       1,
					})
				}
			}
		}

		// 获取重构建议
		refactorResult, err := q.refactorEngine.AnalyzeRefactoring(code, language)
		if err == nil {
			report.Suggestions = append(report.Suggestions, refactorResult.Suggestions...)
		}

		// 统计语言
		report.Metrics.ByLanguage[language]++
	}

	report.Summary.TotalLines = totalLines
	report.Summary.TotalFunctions = totalFunctions
	report.Summary.TotalIssues = len(report.Issues)

	// 计算平均复杂度
	if totalFunctions > 0 {
		report.Metrics.AverageComplexity = float64(totalComplexity) / float64(totalFunctions)
	}
	report.Metrics.MaxComplexity = maxComplexity

	// 计算指标
	if totalLines > 0 {
		report.Metrics.AverageLineLength = float64(totalLines) / float64(report.Summary.TotalFiles)
	}

	// 计算评分
	report.Score = q.calculateScore(report)
	report.Grade = q.getGrade(report.Score)

	return report, nil
}

func (q *QualityAnalyzer) calculateScore(report *QualityReport) int {
	score := 100

	// 基于问题扣分
	for _, issue := range report.Issues {
		switch issue.Severity {
		case "critical":
			score -= issue.Count * 10
		case "high":
			score -= issue.Count * 5
		case "medium":
			score -= issue.Count * 2
		case "low":
			score -= issue.Count * 1
		}
	}

	// 基于复杂度扣分
	if report.Metrics.AverageComplexity > 10 {
		score -= 10
	} else if report.Metrics.AverageComplexity > 5 {
		score -= 5
	}

	if score < 0 {
		score = 0
	}

	return score
}

func (q *QualityAnalyzer) getGrade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

// AnalyzeFile 分析单个文件
func (q *QualityAnalyzer) AnalyzeFile(code string, language string) (*QualityReport, error) {
	files := map[string]string{
		"code": code,
	}
	return q.AnalyzeProject(files, language)
}

// GetSummary 获取分析摘要
func (q *QualityAnalyzer) GetSummary(report *QualityReport) string {
	return fmt.Sprintf("分析完成：%d 个文件，%d 行代码，%d 个函数，%d 个问题，评分: %d (%s)",
		report.Summary.TotalFiles,
		report.Summary.TotalLines,
		report.Summary.TotalFunctions,
		report.Summary.TotalIssues,
		report.Score,
		report.Grade,
	)
}

// GetIssuesBySeverity 按严重程度获取问题
func (q *QualityAnalyzer) GetIssuesBySeverity(report *QualityReport, severity string) []QualityIssue {
	issues := make([]QualityIssue, 0)
	for _, issue := range report.Issues {
		if issue.Severity == severity {
			issues = append(issues, issue)
		}
	}
	return issues
}

// GetIssuesByCategory 按类别获取问题
func (q *QualityAnalyzer) GetIssuesByCategory(report *QualityReport, category string) []QualityIssue {
	issues := make([]QualityIssue, 0)
	for _, issue := range report.Issues {
		if issue.Category == category {
			issues = append(issues, issue)
		}
	}
	return issues
}

// FormatReport 格式化报告为文本
func (q *QualityAnalyzer) FormatReport(report *QualityReport) string {
	var sb strings.Builder

	sb.WriteString("=== 代码质量报告 ===\n\n")
	sb.WriteString(fmt.Sprintf("评分: %d/100 (%s)\n", report.Score, report.Grade))
	sb.WriteString(fmt.Sprintf("文件数: %d\n", report.Summary.TotalFiles))
	sb.WriteString(fmt.Sprintf("代码行数: %d\n", report.Summary.TotalLines))
	sb.WriteString(fmt.Sprintf("函数数: %d\n", report.Summary.TotalFunctions))
	sb.WriteString(fmt.Sprintf("问题数: %d\n", report.Summary.TotalIssues))
	sb.WriteString(fmt.Sprintf("平均复杂度: %.1f\n", report.Metrics.AverageComplexity))
	sb.WriteString(fmt.Sprintf("最大复杂度: %d\n", report.Metrics.MaxComplexity))

	if len(report.Issues) > 0 {
		sb.WriteString("\n--- 问题列表 ---\n")
		for _, issue := range report.Issues {
			sb.WriteString(fmt.Sprintf("[%s] %s (x%d): %s\n", issue.Severity, issue.Title, issue.Count, issue.Description))
		}
	}

	if len(report.Suggestions) > 0 {
		sb.WriteString("\n--- 重构建议 ---\n")
		for _, suggestion := range report.Suggestions {
			sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", suggestion.Severity, suggestion.Title, suggestion.Description))
		}
	}

	return sb.String()
}
