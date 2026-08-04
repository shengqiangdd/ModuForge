package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// PatternLearningSkill learns from successful module patterns and applies them
type PatternLearningSkill struct {
	db *sql.DB
}

func NewPatternLearningSkill(db *sql.DB) *PatternLearningSkill {
	return &PatternLearningSkill{db: db}
}

func (s *PatternLearningSkill) Name() string {
	return "pattern_learn"
}

func (s *PatternLearningSkill) Description() string {
	return "Learn from module patterns. Input: {\"action\": \"apply|suggest|discover\", \"module_type\": \"...\", \"context\": \"...\", \"user_id\": \"...\"}. Read operations are safe. 'record' and 'share' modify database."
}

type Pattern struct {
	ID          int64  `json:"id"`
	ModuleType  string `json:"module_type"`
	PatternType string `json:"pattern_type"`
	Pattern     string `json:"pattern"`
	SuccessRate float64 `json:"success_rate"`
	UsageCount  int     `json:"usage_count"`
}

// sensitivePatterns contains patterns that should NOT be shared across users
var sensitivePatterns = regexp.MustCompile(`(?i)(api[_-]?key|secret|password|token|private[_-]?key|credential|auth[_-]?key|access[_-]?key|hardcoded.*key|BEGIN.*PRIVATE)`)

// Pre-compiled redaction patterns (avoid MustCompile on every call)
var (
	redactApiKey        = regexp.MustCompile(`(?i)(api[_-]?key\s*[=:]\s*)\S+`)
	redactSecret        = regexp.MustCompile(`(?i)(secret\s*[=:]\s*)\S+`)
	redactPassword      = regexp.MustCompile(`(?i)(password\s*[=:]\s*)\S+`)
	redactToken         = regexp.MustCompile(`(?i)(token\s*[=:]\s*)\S+`)
	redactPrivateKey    = regexp.MustCompile(`(?i)(private[_-]?key\s*[=:]\s*)\S+`)
	redactPrivateKeyBlock = regexp.MustCompile(`-----BEGIN.*PRIVATE KEY-----[A-Za-z0-9+/=\s]+-----END.*PRIVATE KEY-----`)
)

func (s *PatternLearningSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	action, _ := input["action"].(string)
	moduleType, _ := input["module_type"].(string)
	pattern, _ := input["pattern"].(string)
	context, _ := input["context"].(string)
	userID, _ := input["user_id"].(string)

	switch action {
	case "record":
		return s.recordPattern(moduleType, pattern, context)
	case "apply":
		return s.applyPattern(moduleType, context)
	case "suggest":
		return s.suggestPatterns(moduleType, context)
	case "share":
		return s.sharePattern(userID, input)
	case "discover":
		return s.discoverPatterns(moduleType, input)
	default:
		return "", fmt.Errorf("unknown action: %s (use record|apply|suggest|share|discover)", action)
	}
}

func (s *PatternLearningSkill) recordPattern(moduleType string, pattern string, context string) (string, error) {
	if moduleType == "" || pattern == "" {
		return "", fmt.Errorf("module_type and pattern are required")
	}

	_, err := s.db.Exec(`
		INSERT INTO module_patterns (module_type, pattern_type, pattern, context, success_count, total_count)
		VALUES (?, 'success', ?, ?, 1, 1)
		ON CONFLICT(module_type, pattern) DO UPDATE SET
			success_count = success_count + 1,
			total_count = total_count + 1
	`, moduleType, pattern, context)
	if err != nil {
		return "", fmt.Errorf("record pattern: %w", err)
	}

	result := map[string]interface{}{
		"action":      "record",
		"success":     true,
		"module_type": moduleType,
		"pattern":     pattern,
		"message":     fmt.Sprintf("已记录成功模式: %s", pattern[:min(50, len(pattern))]),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *PatternLearningSkill) applyPattern(moduleType string, context string) (string, error) {
	rows, err := s.db.Query(`
		SELECT pattern, success_count, total_count
		FROM module_patterns
		WHERE module_type = ? AND total_count > 0
		ORDER BY (success_count * 1.0 / total_count) DESC, usage_count DESC
		LIMIT 5
	`, moduleType)
	if err != nil {
		return "", fmt.Errorf("query patterns: %w", err)
	}
	defer rows.Close()

	var patterns []map[string]interface{}
	for rows.Next() {
		var pattern string
		var successCount, totalCount int
		if rows.Scan(&pattern, &successCount, &totalCount) == nil {
			rate := float64(successCount) / float64(totalCount) * 100
			patterns = append(patterns, map[string]interface{}{
				"pattern":      pattern,
				"success_rate": fmt.Sprintf("%.1f%%", rate),
				"usage_count":  successCount,
			})
		}
	}

	if len(patterns) == 0 {
		return fmt.Sprintf(`{"action":"apply","success":true,"message":"暂无 %s 类型的成功模式记录","patterns":[]}`, moduleType), nil
	}

	result := map[string]interface{}{
		"action":      "apply",
		"success":     true,
		"module_type": moduleType,
		"patterns":    patterns,
		"message":     fmt.Sprintf("找到 %d 个 %s 类型的成功模式", len(patterns), moduleType),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *PatternLearningSkill) suggestPatterns(moduleType string, context string) (string, error) {
	rows, err := s.db.Query(`
		SELECT module_type, pattern, success_count, total_count
		FROM module_patterns
		WHERE total_count > 0
		ORDER BY (success_count * 1.0 / total_count) DESC
		LIMIT 10
	`)
	if err != nil {
		return "", fmt.Errorf("query patterns: %w", err)
	}
	defer rows.Close()

	var suggestions []map[string]interface{}
	seen := make(map[string]bool)

	for rows.Next() {
		var mType, pattern string
		var successCount, totalCount int
		if rows.Scan(&mType, &pattern, &successCount, &totalCount) == nil {
			if !seen[pattern] {
				seen[pattern] = true
				rate := float64(successCount) / float64(totalCount) * 100
				suggestions = append(suggestions, map[string]interface{}{
					"module_type":  mType,
					"pattern":      pattern,
					"success_rate": fmt.Sprintf("%.1f%%", rate),
					"applicable":   mType == moduleType || moduleType == "",
				})
			}
		}
	}

	if context != "" {
		suggestions = append(suggestions, map[string]interface{}{
			"pattern":    "使用模块化设计，将功能拆分为独立组件",
			"applicable": true,
			"importance": "high",
		})
		suggestions = append(suggestions, map[string]interface{}{
			"pattern":    "添加完善的错误处理和日志记录",
			"applicable": true,
			"importance": "high",
		})
	}

	result := map[string]interface{}{
		"action":      "suggest",
		"success":     true,
		"module_type": moduleType,
		"suggestions": suggestions,
		"message":     fmt.Sprintf("提供了 %d 条模式建议", len(suggestions)),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *PatternLearningSkill) sharePattern(userID string, input map[string]interface{}) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("user_id is required for sharing patterns")
	}

	patternID := 0
	if v, ok := input["pattern_id"].(float64); ok {
		patternID = int(v)
	}

	moduleType, _ := input["module_type"].(string)
	pattern, _ := input["pattern"].(string)

	if patternID <= 0 && (moduleType == "" || pattern == "") {
		return "", fmt.Errorf("provide pattern_id or module_type + pattern to share")
	}

	if containsSensitiveData(pattern) {
		return "", fmt.Errorf("pattern contains sensitive data (API keys, secrets, etc.) and cannot be shared for privacy protection")
	}

	var successRate float64
	var usageCount int

	if patternID > 0 {
		err := s.db.QueryRow(`
			SELECT success_count, total_count FROM module_patterns WHERE id = ?
		`, patternID).Scan(&usageCount, &successRate)
		if err != nil {
			return "", fmt.Errorf("pattern not found: %w", err)
		}
		if usageCount > 0 {
			successRate = float64(usageCount) / float64(usageCount) * 100
		}
	} else {
		err := s.db.QueryRow(`
			SELECT success_count, total_count FROM module_patterns WHERE module_type = ? AND pattern = ?
		`, moduleType, pattern).Scan(&usageCount, &successRate)
		if err != nil {
			return "", fmt.Errorf("pattern not found: %w", err)
		}
		if usageCount > 0 {
			successRate = float64(usageCount) / float64(usageCount) * 100
		}
	}

	sanitizedPattern := sanitizePattern(pattern)

	_, err := s.db.Exec(`
		INSERT INTO shared_patterns (user_id, module_type, pattern, success_rate, usage_count, is_shared, shared_at)
		VALUES (?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, module_type, pattern) DO UPDATE SET
			is_shared = 1,
			shared_at = CURRENT_TIMESTAMP,
			success_rate = excluded.success_rate,
			usage_count = excluded.usage_count
	`, userID, moduleType, sanitizedPattern, successRate, usageCount)
	if err != nil {
		return "", fmt.Errorf("share pattern: %w", err)
	}

	result := map[string]interface{}{
		"action":       "share",
		"success":      true,
		"user_id":      userID,
		"module_type":  moduleType,
		"pattern":      sanitizedPattern,
		"success_rate": fmt.Sprintf("%.1f%%", successRate),
		"message":      "模式已标记为可共享，敏感信息已被过滤",
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *PatternLearningSkill) discoverPatterns(moduleType string, input map[string]interface{}) (string, error) {
	currentUserID, _ := input["user_id"].(string)
	minSuccessRate := 50.0
	if v, ok := input["min_success_rate"].(float64); ok {
		minSuccessRate = v
	}

	var rows *sql.Rows
	var err error

	if moduleType != "" {
		rows, err = s.db.Query(`
			SELECT sp.id, sp.user_id, sp.module_type, sp.pattern, sp.success_rate, sp.usage_count, sp.shared_at
			FROM shared_patterns sp
			WHERE sp.is_shared = 1
			  AND sp.module_type = ?
			  AND sp.success_rate >= ?
			  AND sp.user_id != ?
			ORDER BY sp.success_rate DESC, sp.usage_count DESC
			LIMIT 20
		`, moduleType, minSuccessRate, currentUserID)
	} else {
		rows, err = s.db.Query(`
			SELECT sp.id, sp.user_id, sp.module_type, sp.pattern, sp.success_rate, sp.usage_count, sp.shared_at
			FROM shared_patterns sp
			WHERE sp.is_shared = 1
			  AND sp.success_rate >= ?
			  AND sp.user_id != ?
			ORDER BY sp.success_rate DESC, sp.usage_count DESC
			LIMIT 20
		`, minSuccessRate, currentUserID)
	}
	if err != nil {
		return "", fmt.Errorf("discover patterns: %w", err)
	}
	defer rows.Close()

	type DiscoveredPattern struct {
		ID          int64  `json:"id"`
		UserID      string `json:"user_id"`
		ModuleType  string `json:"module_type"`
		Pattern     string `json:"pattern"`
		SuccessRate float64 `json:"success_rate"`
		UsageCount  int    `json:"usage_count"`
		SharedAt    string `json:"shared_at"`
	}

	var discovered []DiscoveredPattern
	for rows.Next() {
		var dp DiscoveredPattern
		if err := rows.Scan(&dp.ID, &dp.UserID, &dp.ModuleType, &dp.Pattern, &dp.SuccessRate, &dp.UsageCount, &dp.SharedAt); err == nil {
			discovered = append(discovered, dp)
		}
	}

	result := map[string]interface{}{
		"action":      "discover",
		"success":     true,
		"module_type": moduleType,
		"min_rate":    fmt.Sprintf("%.1f%%", minSuccessRate),
		"patterns":    discovered,
		"count":       len(discovered),
		"message":     fmt.Sprintf("发现了 %d 个其他用户共享的成功模式", len(discovered)),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func containsSensitiveData(pattern string) bool {
	if sensitivePatterns.MatchString(pattern) {
		return true
	}
	lower := strings.ToLower(pattern)
	sensitiveKeywords := []string{
		"api_key=", "api-key=", "secret=", "password=",
		"token=", "private_key=", "private-key=",
		"ssh-rsa", "BEGIN CERTIFICATE", "sk-",
	}
	for _, kw := range sensitiveKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func sanitizePattern(pattern string) string {
	result := pattern
	result = redactApiKey.ReplaceAllString(result, "${1}[REDACTED]")
	result = redactSecret.ReplaceAllString(result, "${1}[REDACTED]")
	result = redactPassword.ReplaceAllString(result, "${1}[REDACTED]")
	result = redactToken.ReplaceAllString(result, "${1}[REDACTED]")
	result = redactPrivateKey.ReplaceAllString(result, "${1}[REDACTED]")
	result = redactPrivateKeyBlock.ReplaceAllString(result, "[REDACTED PRIVATE KEY]")
	return result
}

func (s *PatternLearningSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  false,
		Essential: false,
		NeedsDB:   true,
		NeedsLLM:  true,
	}
}
