package security

import (
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

// InputValidator 输入验证器
type InputValidator struct {
	maxLength       int
	maxQueryLen     int
	blockedPatterns []*regexp.Regexp
}

// NewInputValidator 创建输入验证器
func NewInputValidator() *InputValidator {
	blocked := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(union\s+select|drop\s+table|insert\s+into|delete\s+from)`),
		regexp.MustCompile(`(?i)(<script|javascript:|on\w+\s*=)`),
		regexp.MustCompile(`(?i)(\.\./|\.\.\\)`),
		regexp.MustCompile(`(?i)(\bexec\b|\beval\b|\bsystem\b|\bcmd\b)`),
		regexp.MustCompile(`(?i)(^\s*select\s+.*\s+from\s+)`),
	}

	return &InputValidator{
		maxLength:       10000,
		maxQueryLen:     1000,
		blockedPatterns: blocked,
	}
}

// ValidationResult 验证结果
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Cleaned  string   `json:"cleaned"`
	Warnings []string `json:"warnings"`
}

// ValidateString 验证字符串输入
func (v *InputValidator) ValidateString(input string, maxLength int) ValidationResult {
	result := ValidationResult{
		Valid:    true,
		Cleaned:  input,
		Warnings: make([]string, 0),
	}

	// 检查长度
	if utf8.RuneCountInString(input) > maxLength {
		result.Valid = false
		result.Warnings = append(result.Warnings, "Input exceeds maximum length")
		return result
	}

	// 检查阻塞模式
	for _, pattern := range v.blockedPatterns {
		if pattern.MatchString(input) {
			result.Valid = false
			result.Warnings = append(result.Warnings, "Input contains blocked pattern")
			break
		}
	}

	// 清理XSS
	result.Cleaned = html.EscapeString(input)

	return result
}

// ValidatePrompt 验证AI提示词
func (v *InputValidator) ValidatePrompt(prompt string) ValidationResult {
	return v.ValidateString(prompt, v.maxLength)
}

// ValidateSearchQuery 验证搜索查询
func (v *InputValidator) ValidateSearchQuery(query string) ValidationResult {
	return v.ValidateString(query, v.maxQueryLen)
}

// SanitizeFilename 清理文件名
func (v *InputValidator) SanitizeFilename(name string) string {
	// 检测路径遍历模式：如果包含 ../ 或 ..\ 则全部替换为下划线
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		// 替换所有特殊字符为下划线
		name = strings.ReplaceAll(name, "/", "_")
		name = strings.ReplaceAll(name, "\\", "_")
		name = strings.ReplaceAll(name, ".", "_")

		// 只保留安全字符
		reg := regexp.MustCompile(`[^a-zA-Z0-9_\-]`)
		name = reg.ReplaceAllString(name, "_")
		return name
	}

	// 正常文件名：只替换不安全字符，保留点
	reg := regexp.MustCompile(`[^a-zA-Z0-9_\-\.]`)
	name = reg.ReplaceAllString(name, "_")

	return name
}

// ValidateJSON 验证JSON输入
func (v *InputValidator) ValidateJSON(data []byte, maxLen int) ValidationResult {
	result := ValidationResult{
		Valid:    true,
		Warnings: make([]string, 0),
	}

	if len(data) > maxLen {
		result.Valid = false
		result.Warnings = append(result.Warnings, "JSON payload too large")
		return result
	}

	// 检查危险模式
	for _, pattern := range v.blockedPatterns {
		if pattern.Match(data) {
			result.Valid = false
			result.Warnings = append(result.Warnings, "JSON contains potentially dangerous content")
			break
		}
	}

	return result
}
