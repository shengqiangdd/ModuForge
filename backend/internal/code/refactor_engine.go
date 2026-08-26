package code

import (
	"fmt"
	"strings"
)

// RefactorEngine 重构建议引擎
type RefactorEngine struct {
	analyzer *MultiLangAnalyzer
}

// NewRefactorEngine 创建重构引擎
func NewRefactorEngine() *RefactorEngine {
	return &RefactorEngine{
		analyzer: NewMultiLangAnalyzer(),
	}
}

// RefactorSuggestion 重构建议
type RefactorSuggestion struct {
	Type        string `json:"type"`        // extract_function, rename, simplify, split_file, add_comment
	Severity    string `json:"severity"`    // high, medium, low
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`    // 函数名或位置
	Before      string `json:"before"`      // 重构前代码片段
	After       string `json:"after"`       // 重构后代码片段
}

// RefactorResult 重构分析结果
type RefactorResult struct {
	Suggestions []RefactorSuggestion `json:"suggestions"`
	Score       int                  `json:"score"` // 代码质量评分 (0-100)
	Summary     string               `json:"summary"`
}

// AnalyzeRefactoring 分析重构建议
func (r *RefactorEngine) AnalyzeRefactoring(code string, language string) (*RefactorResult, error) {
	result, err := r.analyzer.Analyze(code, language)
	if err != nil {
		return nil, err
	}

	suggestions := make([]RefactorSuggestion, 0)

	// 检查函数长度
	for _, f := range result.Functions {
		if f.Complexity > 10 {
			suggestions = append(suggestions, RefactorSuggestion{
				Type:        "split_function",
				Severity:    "high",
				Title:       fmt.Sprintf("函数 %s 复杂度过高", f.Name),
				Description: fmt.Sprintf("函数 %s 的圈复杂度为 %d，建议拆分为更小的函数", f.Name, f.Complexity),
				Location:    f.Name,
			})
		}
	}

	// 检查导入数量
	if len(result.Imports) > 10 {
		suggestions = append(suggestions, RefactorSuggestion{
			Type:        "reduce_dependencies",
			Severity:    "medium",
			Title:       "导入过多",
			Description: fmt.Sprintf("当前有 %d 个导入，建议减少依赖或使用依赖注入", len(result.Imports)),
		})
	}

	// 检查代码重复（简单检测）
	lines := strings.Split(code, "\n")
	if len(lines) > 100 {
		suggestions = append(suggestions, RefactorSuggestion{
			Type:        "split_file",
			Severity:    "medium",
			Title:       "文件过大",
			Description: fmt.Sprintf("文件有 %d 行，建议拆分为多个文件", len(lines)),
		})
	}

	// 检查命名规范
	for _, f := range result.Functions {
		if !f.Exported && strings.Contains(f.Name, "_") && language == "go" {
			suggestions = append(suggestions, RefactorSuggestion{
				Type:        "rename",
				Severity:    "low",
				Title:       fmt.Sprintf("函数 %s 命名不规范", f.Name),
				Description: "Go函数名建议使用驼峰命名法，避免下划线",
				Location:    f.Name,
			})
		}
	}

	// 计算质量评分
	score := 100
	for _, s := range suggestions {
		switch s.Severity {
		case "high":
			score -= 20
		case "medium":
			score -= 10
		case "low":
			score -= 5
		}
	}
	if score < 0 {
		score = 0
	}

	// 生成摘要
	summary := fmt.Sprintf("发现 %d 个改进建议，代码质量评分: %d/100", len(suggestions), score)

	return &RefactorResult{
		Suggestions: suggestions,
		Score:       score,
		Summary:     summary,
	}, nil
}
