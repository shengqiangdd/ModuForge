package code

import (
	"time"
)

// TrendAnalyzer 代码趋势分析器
type TrendAnalyzer struct{}

// NewTrendAnalyzer 创建趋势分析器
func NewTrendAnalyzer() *TrendAnalyzer {
	return &TrendAnalyzer{}
}

// TrendData 趋势数据
type TrendData struct {
	Date       string `json:"date"`
	Lines      int    `json:"lines"`
	Functions  int    `json:"functions"`
	Files      int    `json:"files"`
	Complexity int    `json:"complexity"`
}

// TrendResult 趋势分析结果
type TrendResult struct {
	DataPoints  []TrendData      `json:"data_points"`
	Summary     TrendSummary     `json:"summary"`
	Predictions []TrendPrediction `json:"predictions"`
}

// TrendSummary 趋势摘要
type TrendSummary struct {
	TotalCommits   int     `json:"total_commits"`
	AverageChanges float64 `json:"average_changes"`
	GrowthRate     float64 `json:"growth_rate"`
	MostActiveFile string  `json:"most_active_file"`
	MostActiveDay  string  `json:"most_active_day"`
}

// TrendPrediction 趋势预测
type TrendPrediction struct {
	Metric     string  `json:"metric"`
	Current    int     `json:"current"`
	Predicted  int     `json:"predicted"`
	Confidence float64 `json:"confidence"`
}

// AnalyzeTrends 分析代码趋势
func (t *TrendAnalyzer) AnalyzeTrends(history []TrendData) *TrendResult {
	if len(history) == 0 {
		return &TrendResult{
			DataPoints:  make([]TrendData, 0),
			Summary:     TrendSummary{},
			Predictions: make([]TrendPrediction, 0),
		}
	}

	summary := t.calculateSummary(history)
	predictions := t.generatePredictions(history)

	return &TrendResult{
		DataPoints:  history,
		Summary:     summary,
		Predictions: predictions,
	}
}

func (t *TrendAnalyzer) calculateSummary(history []TrendData) TrendSummary {
	if len(history) == 0 {
		return TrendSummary{}
	}

	avgChanges := 0.0
	if len(history) > 1 {
		for i := 1; i < len(history); i++ {
			avgChanges += float64(history[i].Lines - history[i-1].Lines)
		}
		avgChanges /= float64(len(history) - 1)
	}

	growthRate := 0.0
	if len(history) > 1 && history[0].Lines > 0 {
		growthRate = float64(history[len(history)-1].Lines-history[0].Lines) / float64(history[0].Lines) * 100
	}

	return TrendSummary{
		TotalCommits:   len(history),
		AverageChanges: avgChanges,
		GrowthRate:     growthRate,
	}
}

func (t *TrendAnalyzer) generatePredictions(history []TrendData) []TrendPrediction {
	predictions := make([]TrendPrediction, 0)

	if len(history) < 2 {
		return predictions
	}

	lastLines := history[len(history)-1].Lines
	secondLastLines := history[len(history)-2].Lines
	change := lastLines - secondLastLines

	predictions = append(predictions, TrendPrediction{
		Metric:     "lines",
		Current:    lastLines,
		Predicted:  lastLines + change,
		Confidence: 0.7,
	})

	lastFunctions := history[len(history)-1].Functions
	secondLastFunctions := history[len(history)-2].Functions
	funcChange := lastFunctions - secondLastFunctions

	predictions = append(predictions, TrendPrediction{
		Metric:     "functions",
		Current:    lastFunctions,
		Predicted:  lastFunctions + funcChange,
		Confidence: 0.6,
	})

	return predictions
}

// GenerateSampleData 生成示例趋势数据
func (t *TrendAnalyzer) GenerateSampleData() []TrendData {
	data := make([]TrendData, 0)
	now := time.Now()

	for i := 6; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		data = append(data, TrendData{
			Date:       date.Format("2006-01-02"),
			Lines:      1000 + (6-i)*100,
			Functions:  50 + (6-i)*5,
			Files:      10 + (6-i)*2,
			Complexity: 20 + (6-i)*3,
		})
	}

	return data
}
