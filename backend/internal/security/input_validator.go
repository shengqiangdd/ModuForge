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
	// 移除路径分隔符
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	name = strings.ReplaceAll(name, "..", "")

	// 只保留安全字符
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
