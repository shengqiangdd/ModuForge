package code

import (
	"strings"
)

// ProjectAnalytics 项目分析器
type ProjectAnalytics struct{}

// NewProjectAnalytics 创建项目分析器
func NewProjectAnalytics() *ProjectAnalytics {
	return &ProjectAnalytics{}
}

// ComplexityResult 复杂度分析结果
type ComplexityResult struct {
	TotalFiles    int                `json:"total_files"`
	TotalLines    int                `json:"total_lines"`
	AvgComplexity float64            `json:"avg_complexity"`
	MaxComplexity int                `json:"max_complexity"`
	Files         []FileComplexity   `json:"files"`
}

// FileComplexity 文件复杂度
type FileComplexity struct {
	FileName      string `json:"file_name"`
	Lines         int    `json:"lines"`
	Complexity    int    `json:"complexity"`
	RiskLevel     string `json:"risk_level"` // low, medium, high, critical
}

// AnalyzeComplexity 分析代码复杂度
func (pa *ProjectAnalytics) AnalyzeComplexity(files map[string]string) *ComplexityResult {
	result := &ComplexityResult{
		Files: make([]FileComplexity, 0),
	}

	totalComplexity := 0
	maxComplexity := 0

	for filename, code := range files {
		lines := strings.Split(code, "\n")
		complexity := pa.calculateCyclomaticComplexity(code)
		
		riskLevel := "low"
		if complexity > 50 {
			riskLevel = "critical"
		} else if complexity > 20 {
			riskLevel = "high"
		} else if complexity > 10 {
			riskLevel = "medium"
		}

		fc := FileComplexity{
			FileName:   filename,
			Lines:      len(lines),
			Complexity: complexity,
			RiskLevel:  riskLevel,
		}

		result.Files = append(result.Files, fc)
		result.TotalLines += len(lines)
		totalComplexity += complexity
		if complexity > maxComplexity {
			maxComplexity = complexity
		}
	}

	result.TotalFiles = len(files)
	result.MaxComplexity = maxComplexity
	if result.TotalFiles > 0 {
		result.AvgComplexity = float64(totalComplexity) / float64(result.TotalFiles)
	}

	return result
}

// calculateCyclomaticComplexity 计算圈复杂度
func (pa *ProjectAnalytics) calculateCyclomaticComplexity(code string) int {
	complexity := 1
	keywords := []string{
		"if ", "else if", "for ", "while ", "switch ", "case ",
		"&&", "||", "catch ", "select ", "go ", "defer ",
	}

	lines := strings.Split(code, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "//") {
			continue
		}

		for _, keyword := range keywords {
			if strings.Contains(line, keyword) {
				complexity++
			}
		}
	}

	return complexity
}
