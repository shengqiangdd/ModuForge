package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type RegressionTestSkill struct{}

func NewRegressionTestSkill() *RegressionTestSkill {
	return &RegressionTestSkill{}
}

func (s *RegressionTestSkill) Name() string {
	return "regression_test"
}

func (s *RegressionTestSkill) Description() string {
	return "Compare current vs historical test results to detect regressions. Input: {\"project_id\": ..., \"test_results\": {...}}. Returns regression report with suggestions."
}

type regressionIssue struct {
	TestName       string `json:"test_name"`
	PreviousStatus string `json:"previous_status"`
	CurrentStatus  string `json:"current_status"`
	Detail         string `json:"detail,omitempty"`
}

type regressionReport struct {
	ProjectID      string             `json:"project_id"`
	Safe           bool               `json:"safe"`
	Regressions    []regressionIssue  `json:"regressions"`
	NewPasses      []string           `json:"new_passes"`
	TotalPrevious  int                `json:"total_previous"`
	TotalCurrent   int                `json:"total_current"`
	RegressedCount int                `json:"regressed_count"`
	ImprovedCount  int                `json:"improved_count"`
	Score          int                `json:"score"`
	Suggestions    []string           `json:"suggestions"`
	Timestamp      string             `json:"timestamp"`
}

type historicalTestResult struct {
	Timestamp string          `json:"timestamp"`
	Results   []testCaseResult `json:"results"`
}

func (s *RegressionTestSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	projectID, _ := input["project_id"].(string)
	if projectID == "" {
		return "", fmt.Errorf("project_id is required")
	}

	testResultsRaw, ok := input["test_results"]
	if !ok {
		return "", fmt.Errorf("test_results is required")
	}

	testResultsMap, ok := testResultsRaw.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("test_results must be an object")
	}

	currentCases := parseTestCases(testResultsMap)

	// Load historical results (stored in memory for now; in production use DB)
	historical := loadHistoricalResults(projectID)

	report := regressionReport{
		ProjectID:     projectID,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		TotalPrevious: len(historical),
		TotalCurrent:  len(currentCases),
	}

	prevByName := make(map[string]string)
	for _, h := range historical {
		for _, r := range h.Results {
			prevByName[r.Name] = r.Status
		}
	}

	currentByName := make(map[string]string)
	for _, c := range currentCases {
		currentByName[c.Name] = c.Status
	}

	_ = prevByName

	for name, curStatus := range currentByName {
		prevStatus, exists := prevByName[name]
		if !exists {
			continue
		}

		if prevStatus == "passed" && curStatus != "passed" {
			report.Regressions = append(report.Regressions, regressionIssue{
				TestName:       name,
				PreviousStatus: prevStatus,
				CurrentStatus:  curStatus,
				Detail:         fmt.Sprintf("测试 \"%s\" 之前通过 (passed)，现在为 %s", name, curStatus),
			})
		}

		if (prevStatus == "failed" || prevStatus == "skipped") && curStatus == "passed" {
			report.NewPasses = append(report.NewPasses, name)
		}
	}

	report.RegressedCount = len(report.Regressions)
	report.ImprovedCount = len(report.NewPasses)
	report.Safe = report.RegressedCount == 0

	report.Score = 100 - report.RegressedCount*15
	if report.Score < 0 {
		report.Score = 0
	}

	report.Suggestions = generateSuggestions(report)

	saveCurrentResults(projectID, currentCases)

	b, _ := json.MarshalIndent(report, "", "  ")
	return string(b), nil
}

type testCaseResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func parseTestCases(data map[string]interface{}) []testCaseResult {
	var cases []testCaseResult

	for key, val := range data {
		switch v := val.(type) {
		case string:
			cases = append(cases, testCaseResult{Name: key, Status: v})
		case map[string]interface{}:
			status, _ := v["status"].(string)
			if status == "" {
				status = "unknown"
			}
			cases = append(cases, testCaseResult{Name: key, Status: status})
		}
	}

	sort.Slice(cases, func(i, j int) bool {
		return cases[i].Name < cases[j].Name
	})

	return cases
}

func loadHistoricalResults(projectID string) []historicalTestResult {
	// In production, load from database
	return nil
}

func saveCurrentResults(projectID string, cases []testCaseResult) {
	// In production, persist to database
}

func generateSuggestions(report regressionReport) []string {
	var suggestions []string

	if report.RegressedCount > 0 {
		suggestions = append(suggestions,
			fmt.Sprintf("发现 %d 个回归问题，建议检查最近的代码变更", report.RegressedCount),
		)
		for _, r := range report.Regressions {
			suggestions = append(suggestions,
				fmt.Sprintf("- %s: 状态从 %s 变为 %s，请审查相关代码", r.TestName, r.PreviousStatus, r.CurrentStatus),
			)
		}
	}

	if report.ImprovedCount > 0 {
		suggestions = append(suggestions,
			fmt.Sprintf("%d 个之前失败的测试现已通过，进展良好", report.ImprovedCount),
		)
	}

	if report.Score >= 80 {
		suggestions = append(suggestions, "整体质量良好，可以继续迭代")
	} else if report.Score >= 50 {
		suggestions = append(suggestions, "存在一些问题，建议修复后重新测试")
	} else {
		suggestions = append(suggestions, "质量评分较低，建议进行全面的代码审查和测试")
	}

	return suggestions
}

func (s *RegressionTestSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  true,
		Essential: false,
		NeedsDB:   false,
		NeedsLLM:  true,
	}
}
