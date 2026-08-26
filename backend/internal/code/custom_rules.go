package code

import (
	"regexp"
	"strings"
)

// CustomRuleEngine 自定义规则引擎
type CustomRuleEngine struct {
	Rules []CustomRule
}

// CustomRule 自定义规则
type CustomRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`     // 正则表达式
	Message     string `json:"message"`     // 告警信息
	Severity    string `json:"severity"`    // error, warning, info
	Language    string `json:"language"`    // go, javascript, python
}

// RuleResult 规则检查结果
type RuleResult struct {
	RuleID      string `json:"rule_id"`
	RuleName    string `json:"rule_name"`
	FileName    string `json:"file_name"`
	Line        int    `json:"line"`
	Message     string `json:"message"`
	Severity    string `json:"severity"`
	MatchedCode string `json:"matched_code"`
}

// NewCustomRuleEngine 创建自定义规则引擎
func NewCustomRuleEngine() *CustomRuleEngine {
	return &CustomRuleEngine{
		Rules: make([]CustomRule, 0),
	}
}

// AddRule 添加规则
func (e *CustomRuleEngine) AddRule(rule CustomRule) {
	e.Rules = append(e.Rules, rule)
}

// CheckCode 检查代码
func (e *CustomRuleEngine) CheckCode(filename string, code string, language string) []RuleResult {
	results := make([]RuleResult, 0)

	for _, rule := range e.Rules {
		if rule.Language != "" && rule.Language != language {
			continue
		}

		regex, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}

		lines := strings.Split(code, "\n")
		for i, line := range lines {
			if regex.MatchString(line) {
				results = append(results, RuleResult{
					RuleID:      rule.ID,
					RuleName:    rule.Name,
					FileName:    filename,
					Line:        i + 1,
					Message:     rule.Message,
					Severity:    rule.Severity,
					MatchedCode: line,
				})
			}
		}
	}

	return results
}

// CheckFiles 批量检查文件
func (e *CustomRuleEngine) CheckFiles(files map[string]string, language string) []RuleResult {
	allResults := make([]RuleResult, 0)
	for filename, code := range files {
		results := e.CheckCode(filename, code, language)
		allResults = append(allResults, results...)
	}
	return allResults
}
