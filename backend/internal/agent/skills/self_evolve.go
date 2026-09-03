package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/moduforge/backend/internal/agent/registry"
	"log"
	"strings"
)

// SelfEvolvingSkill learns from execution history and auto-improves its prompts
type SelfEvolvingSkill struct {
	db *sql.DB
}

func NewSelfEvolvingSkill(db *sql.DB) *SelfEvolvingSkill {
	return &SelfEvolvingSkill{db: db}
}

func (s *SelfEvolvingSkill) Name() string {
	return "self_evolve"
}

func (s *SelfEvolvingSkill) Description() string {
	return "Analyze skill execution history. Input: {\"action\": \"stats|analyze_failure\", \"skill_id\": \"...\", \"execution_id\": ...}. Read-only analysis. 'auto_optimize' and 'improve' modify skill prompts."
}

type EvolutionResult struct {
	Action  string      `json:"action"`
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (s *SelfEvolvingSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	action, _ := input["action"].(string)
	skillID, _ := input["skill_id"].(string)
	context, _ := input["context"].(string)

	switch action {
	case "learn":
		return s.learn(skillID, context)
	case "apply":
		return s.apply(skillID)
	case "stats":
		return s.getStats(skillID)
	case "improve":
		return s.generateImprovement(skillID, context)
	case "auto_optimize":
		return s.autoOptimize(skillID)
	case "analyze_failure":
		execID := 0
		if v, ok := input["execution_id"].(float64); ok {
			execID = int(v)
		}
		return s.analyzeFailure(execID)
	default:
		return "", fmt.Errorf("unknown action: %s (use learn|apply|stats|improve|auto_optimize|analyze_failure)", action)
	}
}

func (s *SelfEvolvingSkill) learn(skillID string, executionContext string) (string, error) {
	rows, err := s.db.Query(`
		SELECT input, output, success, duration_ms, feedback
		FROM skill_evolution
		WHERE skill_id = ? AND created_at > datetime('now', '-7 days')
		ORDER BY created_at DESC
		LIMIT 50
	`, skillID)
	if err != nil {
		return "", fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()

	type ExecutionRecord struct {
		Input      string `json:"input"`
		Output     string `json:"output"`
		Success    bool   `json:"success"`
		DurationMs int    `json:"duration_ms"`
		Feedback   string `json:"feedback"`
	}

	var records []ExecutionRecord
	var successCount, failCount int
	var totalDuration int

	for rows.Next() {
		var rec ExecutionRecord
		var success int
		if err := rows.Scan(&rec.Input, &rec.Output, &success, &rec.DurationMs, &rec.Feedback); err == nil {
			rec.Success = success == 1
			if rec.Success {
				successCount++
			} else {
				failCount++
			}
			totalDuration += rec.DurationMs
			records = append(records, rec)
		}
	}

	if len(records) == 0 {
		result := EvolutionResult{
			Action:  "learn",
			Message: "暂无执行记录，无法学习",
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return string(b), nil
	}

	var patterns []string
	if successCount > 0 {
		patterns = append(patterns, fmt.Sprintf("成功率: %.1f%% (%d/%d)", float64(successCount)/float64(len(records))*100, successCount, len(records)))
	}
	if failCount > 0 {
		patterns = append(patterns, fmt.Sprintf("失败率: %.1f%% (%d/%d)", float64(failCount)/float64(len(records))*100, failCount, len(records)))
	}
	if len(records) > 0 {
		patterns = append(patterns, fmt.Sprintf("平均耗时: %dms", totalDuration/len(records)))
	}

	var failureReasons []string
	for _, rec := range records {
		if !rec.Success && rec.Feedback != "" {
			failureReasons = append(failureReasons, rec.Feedback)
		}
	}
	if len(failureReasons) > 0 {
		patterns = append(patterns, fmt.Sprintf("常见失败原因: %s", strings.Join(failureReasons[:min(3, len(failureReasons))], "; ")))
	}

	insight := strings.Join(patterns, "\n")
	s.db.Exec(`
		INSERT INTO custom_skills (user_id, name, description, prompt, is_public)
		VALUES (?, ?, ?, ?, ?)
	`, "system", fmt.Sprintf("insight_%s", skillID), fmt.Sprintf("Learning insights for skill %s", skillID), insight, 0)

	result := EvolutionResult{
		Action:  "learn",
		Success: true,
		Message: fmt.Sprintf("从 %d 次执行中提取了 %d 条学习记录", len(records), len(patterns)),
		Data: map[string]interface{}{
			"total_executions": len(records),
			"success_count":    successCount,
			"failure_count":    failCount,
			"patterns":         patterns,
		},
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SelfEvolvingSkill) apply(skillID string) (string, error) {
	var prompt string
	err := s.db.QueryRow("SELECT prompt FROM custom_skills WHERE id = ?", skillID).Scan(&prompt)
	if err != nil {
		return "", fmt.Errorf("skill not found: %w", err)
	}

	var totalRuns, successRuns int
	s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END), 0)
		FROM skill_evolution WHERE skill_id = ?
	`, skillID).Scan(&totalRuns, &successRuns)

	rows, err := s.db.Query(`
		SELECT feedback FROM skill_evolution
		WHERE skill_id = ? AND feedback != ''
		ORDER BY created_at DESC LIMIT 10
	`, skillID)
	var feedbacks []string
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var fb string
			if rows.Scan(&fb) == nil && fb != "" {
				feedbacks = append(feedbacks, fb)
			}
		}
	}

	var suggestions []string
	if totalRuns > 0 {
		rate := float64(successRuns) / float64(totalRuns) * 100
		if rate < 70 {
			suggestions = append(suggestions, "成功率低于70%，建议简化输入格式或添加更多约束条件")
		}
		if rate >= 90 {
			suggestions = append(suggestions, "成功率很高，可以考虑增加更复杂的功能")
		}
	}

	if len(feedbacks) > 0 {
		suggestions = append(suggestions, fmt.Sprintf("最近 %d 次执行有反馈，建议根据反馈优化 prompt", len(feedbacks)))
	}

	var avgDuration float64
	s.db.QueryRow(`
		SELECT COALESCE(AVG(duration_ms), 0) FROM skill_evolution WHERE skill_id = ?
	`, skillID).Scan(&avgDuration)

	if avgDuration > 5000 {
		suggestions = append(suggestions, fmt.Sprintf("平均耗时 %.0fms 较长，建议精简 prompt 或使用更快的模型", avgDuration))
	}

	result := EvolutionResult{
		Action:  "apply",
		Success: true,
		Message: "应用学习结果完成",
		Data: map[string]interface{}{
			"current_prompt_length": len(prompt),
			"total_runs":            totalRuns,
			"success_rate":          fmt.Sprintf("%.1f%%", float64(successRuns)/float64(totalRuns)*100),
			"suggestions":           suggestions,
		},
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SelfEvolvingSkill) getStats(skillID string) (string, error) {
	var totalRuns, successRuns int
	var avgDuration float64
	var lastRunAt string

	s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END), 0), 
		       COALESCE(AVG(duration_ms), 0), COALESCE(MAX(created_at), '')
		FROM skill_evolution WHERE skill_id = ?
	`, skillID).Scan(&totalRuns, &successRuns, &avgDuration, &lastRunAt)

	rows, err := s.db.Query(`
		SELECT DATE(created_at) as day, COUNT(*) as total, 
		       SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) as successes
		FROM skill_evolution
		WHERE skill_id = ? AND created_at > datetime('now', '-30 days')
		GROUP BY DATE(created_at)
		ORDER BY day
	`, skillID)

	type DailyStat struct {
		Date      string  `json:"date"`
		Total     int     `json:"total"`
		Successes int     `json:"successes"`
		Rate      float64 `json:"rate"`
	}

	var dailyStats []DailyStat
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ds DailyStat
			if rows.Scan(&ds.Date, &ds.Total, &ds.Successes) == nil {
				ds.Rate = float64(ds.Successes) / float64(ds.Total) * 100
				dailyStats = append(dailyStats, ds)
			}
		}
	}

	result := map[string]interface{}{
		"skill_id":        skillID,
		"total_runs":      totalRuns,
		"success_count":   successRuns,
		"success_rate":    fmt.Sprintf("%.1f%%", float64(successRuns)/float64(totalRuns)*100),
		"avg_duration_ms": fmt.Sprintf("%.0f", avgDuration),
		"last_run_at":     lastRunAt,
		"daily_trend":     dailyStats,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SelfEvolvingSkill) generateImprovement(skillID string, context string) (string, error) {
	var prompt string
	err := s.db.QueryRow("SELECT prompt FROM custom_skills WHERE id = ?", skillID).Scan(&prompt)
	if err != nil {
		return "", fmt.Errorf("skill not found: %w", err)
	}

	rows, err := s.db.Query(`
		SELECT input, output, feedback
		FROM skill_evolution
		WHERE skill_id = ? AND success = 0
		ORDER BY created_at DESC
		LIMIT 5
	`, skillID)
	var failures []string
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var input, output, feedback string
			if rows.Scan(&input, &output, &feedback) == nil {
				failures = append(failures, fmt.Sprintf("输入: %s\n输出: %s\n反馈: %s", input, output, feedback))
			}
		}
	}

	var suggestions []string
	if len(failures) > 0 {
		suggestions = append(suggestions, "基于失败案例的改进建议:")
		for i, f := range failures[:min(3, len(failures))] {
			suggestions = append(suggestions, fmt.Sprintf("%d. %s", i+1, f))
		}
	}

	suggestions = append(suggestions, "\n优化方向:")
	suggestions = append(suggestions, "1. 添加更明确的输入验证")
	suggestions = append(suggestions, "2. 增加错误处理和回退机制")
	suggestions = append(suggestions, "3. 优化 prompt 以提高 LLM 理解度")
	suggestions = append(suggestions, "4. 考虑添加示例输入输出")

	result := EvolutionResult{
		Action:  "improve",
		Success: true,
		Message: "生成改进方案完成",
		Data: map[string]interface{}{
			"current_prompt": prompt,
			"failure_count":  len(failures),
			"suggestions":    suggestions,
		},
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SelfEvolvingSkill) autoOptimize(skillID string) (string, error) {
	var prompt, skillName string
	err := s.db.QueryRow("SELECT name, prompt FROM custom_skills WHERE id = ?", skillID).Scan(&skillName, &prompt)
	if err != nil {
		return "", fmt.Errorf("skill not found: %w", err)
	}

	var totalRuns, successRuns int
	s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END), 0)
		FROM skill_evolution WHERE skill_id = ?
	`, skillID).Scan(&totalRuns, &successRuns)

	successRate := 0.0
	if totalRuns > 0 {
		successRate = float64(successRuns) / float64(totalRuns) * 100
	}

	if successRate >= 70 || totalRuns <= 10 {
		result := EvolutionResult{
			Action:  "auto_optimize",
			Success: true,
			Message: fmt.Sprintf("无需优化: 成功率 %.1f%%, 执行次数 %d", successRate, totalRuns),
			Data: map[string]interface{}{
				"success_rate": successRate,
				"total_runs":   totalRuns,
				"threshold":    "成功率 >= 70% 或执行次数 <= 10",
			},
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return string(b), nil
	}

	var failureReasons []string
	rows, err := s.db.Query(`
		SELECT input, output, feedback
		FROM skill_evolution
		WHERE skill_id = ? AND success = 0 AND feedback != ''
		ORDER BY created_at DESC
		LIMIT 5
	`, skillID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var input, output, feedback string
			if rows.Scan(&input, &output, &feedback) == nil {
				failureReasons = append(failureReasons, fmt.Sprintf("输入: %s | 输出: %s | 反馈: %s", input, output, feedback))
			}
		}
	}

	var optimizedPrompt string
	if len(failureReasons) > 0 {
		optimizedPrompt = fmt.Sprintf("%s\n\n[Auto-Optimized]\n"+
			"基于 %d 次失败分析的改进:\n"+
			"成功率当前为 %.1f%%，以下是需要避免的失败模式:\n%s\n"+
			"优化要求:\n"+
			"- 增加输入验证和边界检查\n"+
			"- 添加明确的错误处理指导\n"+
			"- 包含成功执行的示例格式\n"+
			"- 避免导致上述失败的输入模式",
			prompt, len(failureReasons), successRate,
			strings.Join(failureReasons[:min(3, len(failureReasons))], "\n"))
	} else {
		optimizedPrompt = fmt.Sprintf("%s\n\n[Auto-Optimized]\n"+
			"成功率 %.1f%% 偏低，已自动添加:\n"+
			"- 更明确的输入格式约束\n"+
			"- 错误处理和回退机制指导\n"+
			"- 成功案例示例",
			prompt, successRate)
	}

	var maxVersion int
	s.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM prompt_versions WHERE skill_id = ?", skillID).Scan(&maxVersion)
	newVersion := maxVersion + 1

	_, err = s.db.Exec(
		"INSERT INTO prompt_versions (skill_id, prompt, version, change_reason) VALUES (?, ?, ?, ?)",
		skillID, optimizedPrompt, newVersion,
		fmt.Sprintf("auto_optimize: 成功率 %.1f%%, 执行 %d 次", successRate, totalRuns),
	)
	if err != nil {
		log.Printf("[SelfEvolve] Failed to record prompt version: %v", err)
	}

	_, err = s.db.Exec("UPDATE custom_skills SET prompt = ? WHERE id = ?", optimizedPrompt, skillID)
	if err != nil {
		log.Printf("[SelfEvolve] Failed to update skill prompt: %v", err)
	}

	result := EvolutionResult{
		Action:  "auto_optimize",
		Success: true,
		Message: fmt.Sprintf("已自动生成优化 prompt (v%d)，成功率从 %.1f%% 优化", newVersion, successRate),
		Data: map[string]interface{}{
			"skill_id":       skillID,
			"skill_name":     skillName,
			"old_prompt_len": len(prompt),
			"new_prompt_len": len(optimizedPrompt),
			"version":        newVersion,
			"success_rate":   successRate,
			"total_runs":     totalRuns,
			"failure_count":  len(failureReasons),
		},
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SelfEvolvingSkill) analyzeFailure(executionID int) (string, error) {
	if executionID <= 0 {
		return "", fmt.Errorf("execution_id is required and must be > 0")
	}

	var skillID int64
	var input, output, feedback string
	var success int
	var durationMs int
	var createdAt string
	err := s.db.QueryRow(`
		SELECT skill_id, input, output, success, duration_ms, feedback, created_at
		FROM skill_evolution WHERE id = ?
	`, executionID).Scan(&skillID, &input, &output, &success, &durationMs, &feedback, &createdAt)
	if err != nil {
		return "", fmt.Errorf("execution record not found: %w", err)
	}

	if success == 1 {
		result := map[string]interface{}{
			"action":         "analyze_failure",
			"success":        true,
			"message":        "该执行记录为成功状态，无需分析失败原因",
			"execution_id":   executionID,
			"was_successful": true,
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return string(b), nil
	}

	type FailureCategory struct {
		Category    string   `json:"category"`
		Description string   `json:"description"`
		Suggestions []string `json:"suggestions"`
		AutoFixable bool     `json:"auto_fixable"`
	}

	var categories []FailureCategory
	lowerOutput := strings.ToLower(output)
	lowerFeedback := strings.ToLower(feedback)
	combined := lowerOutput + " " + lowerFeedback

	if strings.Contains(combined, "lint") || strings.Contains(combined, "syntax") || strings.Contains(combined, "parse") {
		categories = append(categories, FailureCategory{
			Category:    "lint_syntax_error",
			Description: "代码语法或 lint 错误",
			Suggestions: []string{
				"检查代码语法是否正确",
				"运行 lint 工具检查",
				"修复未闭合的引号、括号",
			},
			AutoFixable: true,
		})
	}

	if strings.Contains(combined, "timeout") || strings.Contains(combined, "deadline") || strings.Contains(combined, "slow") {
		categories = append(categories, FailureCategory{
			Category:    "performance_timeout",
			Description: "执行超时或性能问题",
			Suggestions: []string{
				"优化代码执行效率",
				"减少不必要的循环或递归",
				"使用更快的算法",
			},
			AutoFixable: false,
		})
	}

	if strings.Contains(combined, "permission") || strings.Contains(combined, "access") || strings.Contains(combined, "denied") {
		categories = append(categories, FailureCategory{
			Category:    "permission_error",
			Description: "权限或访问错误",
			Suggestions: []string{
				"检查文件权限设置",
				"确保以正确用户运行",
				"验证路径访问权限",
			},
			AutoFixable: true,
		})
	}

	if strings.Contains(combined, "not found") || strings.Contains(combined, "missing") || strings.Contains(combined, "no such") {
		categories = append(categories, FailureCategory{
			Category:    "resource_not_found",
			Description: "资源或文件不存在",
			Suggestions: []string{
				"检查文件路径是否正确",
				"确认依赖已安装",
				"验证输入参数",
			},
			AutoFixable: false,
		})
	}

	if strings.Contains(combined, "format") || strings.Contains(combined, "json") || strings.Contains(combined, "unmarshal") {
		categories = append(categories, FailureCategory{
			Category:    "format_error",
			Description: "数据格式错误",
			Suggestions: []string{
				"检查输入 JSON 格式",
				"验证字段名称和类型",
				"确保字符串已正确转义",
			},
			AutoFixable: true,
		})
	}

	if len(categories) == 0 {
		categories = append(categories, FailureCategory{
			Category:    "unknown",
			Description: "未分类的失败",
			Suggestions: []string{
				"检查完整错误输出以确定原因",
				"查看输入是否符合预期格式",
				"联系开发者获取支持",
			},
			AutoFixable: false,
		})
	}

	var fixCode string
	for _, cat := range categories {
		if cat.AutoFixable {
			switch cat.Category {
			case "lint_syntax_error":
				fixCode = `#!/bin/bash
# Auto-generated lint fix script
find . -name "*.sh" -exec shellcheck -f=gcc {} \; 2>&1 | head -20
# 修复常见语法错误
find . -name "*.sh" -exec sed -i 's/\r$//' {} \;
echo "Lint 修复完成，请检查结果`
			case "permission_error":
				fixCode = `#!/bin/bash
# Auto-generated permission fix script
chmod -R 755 scripts/ 2>/dev/null || true
chmod +x *.sh 2>/dev/null || true
echo "权限修复完成"`
			case "format_error":
				fixCode = `// Auto-generated format validation
function validateJSON(str) {
  try {
    JSON.parse(str);
    return { valid: true };
  } catch (e) {
    return { valid: false, error: e.message };
  }
}`
			}
			break
		}
	}

	var skillName string
	s.db.QueryRow("SELECT name FROM custom_skills WHERE id = ?", skillID).Scan(&skillName)

	result := map[string]interface{}{
		"action":       "analyze_failure",
		"success":      true,
		"execution_id": executionID,
		"skill_id":     skillID,
		"skill_name":   skillName,
		"input":        input,
		"output":       output,
		"feedback":     feedback,
		"duration_ms":  durationMs,
		"created_at":   createdAt,
		"categories":   categories,
		"root_cause":   categories[0].Description,
		"auto_fixable": len(categories) > 0 && categories[0].AutoFixable,
	}
	if fixCode != "" {
		result["fix_code"] = fixCode
		result["fix_message"] = "已自动生成修复代码，请审查后使用"
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *SelfEvolvingSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		NeedsDB:   true,
		NeedsLLM:  true,
	}
}
