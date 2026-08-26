package code

import (
	"fmt"
	"regexp"
	"strings"
)

// SecurityScanner 深度安全扫描器
type SecurityScanner struct{}

// NewSecurityScanner 创建安全扫描器
func NewSecurityScanner() *SecurityScanner {
	return &SecurityScanner{}
}

// SecurityScanResult 安全扫描结果
type SecurityScanResult struct {
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	Score           int             `json:"score"`
	RiskLevel       string          `json:"risk_level"`
	Summary         string          `json:"summary"`
	Stats           ScanStats       `json:"stats"`
}

// Vulnerability 漏洞
type Vulnerability struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Line        int    `json:"line"`
	Pattern     string `json:"pattern"`
	Suggestion  string `json:"suggestion"`
	CWE         string `json:"cwe"`
}

// ScanStats 扫描统计
type ScanStats struct {
	TotalIssues   int `json:"total_issues"`
	CriticalCount int `json:"critical_count"`
	HighCount     int `json:"high_count"`
	MediumCount   int `json:"medium_count"`
	LowCount      int `json:"low_count"`
}

// ScanCode 深度安全扫描
func (s *SecurityScanner) ScanCode(code string, language string) *SecurityScanResult {
	result := &SecurityScanResult{
		Vulnerabilities: make([]Vulnerability, 0),
		Stats:           ScanStats{},
	}

	lines := strings.Split(code, "\n")

	// SQL注入检测
	s.checkSQLInjection(code, language, lines, &result.Vulnerabilities)

	// XSS检测
	s.checkXSS(code, language, lines, &result.Vulnerabilities)

	// 硬编码凭证
	s.checkHardcodedSecrets(code, language, lines, &result.Vulnerabilities)

	// 路径遍历
	s.checkPathTraversal(code, language, lines, &result.Vulnerabilities)

	// 命令注入
	s.checkCommandInjection(code, language, lines, &result.Vulnerabilities)

	// 不安全的反序列化
	s.checkUnsafeDeserialization(code, language, lines, &result.Vulnerabilities)

	// 弱加密算法
	s.checkWeakCrypto(code, language, lines, &result.Vulnerabilities)

	// 内存安全
	s.checkMemorySafety(code, language, lines, &result.Vulnerabilities)

	// 计算统计和评分
	s.calculateStats(result)

	return result
}

func (s *SecurityScanner) checkSQLInjection(code string, language string, lines []string, vulns *[]Vulnerability) {
	patterns := []struct {
		regex   *regexp.Regexp
		cwe     string
		title   string
		suggest string
	}{
		{
			regex:   regexp.MustCompile(`fmt\.Sprintf.*(?:SELECT|INSERT|UPDATE|DELETE)`),
			cwe:     "CWE-89",
			title:   "SQL注入风险：字符串拼接构建SQL",
			suggest: "使用参数化查询或预编译语句",
		},
		{
			regex:   regexp.MustCompile(`"` + `.*(?:SELECT|INSERT|UPDATE|DELETE).*` + `\+`),
			cwe:     "CWE-89",
			title:   "SQL注入风险：字符串拼接构建SQL",
			suggest: "使用参数化查询或预编译语句",
		},
	}

	for i, line := range lines {
		for _, p := range patterns {
			if p.regex.MatchString(line) {
				*vulns = append(*vulns, Vulnerability{
					ID:          fmt.Sprintf("SQL-INJECT-%d", len(*vulns)+1),
					Category:    "injection",
					Severity:    "critical",
					Title:       p.title,
					Description: "检测到使用字符串拼接构建SQL查询，可能导致SQL注入攻击",
					Location:    fmt.Sprintf("第 %d 行", i+1),
					Line:        i + 1,
					Pattern:     line,
					Suggestion:  p.suggest,
					CWE:         p.cwe,
				})
			}
		}
	}
}

func (s *SecurityScanner) checkXSS(code string, language string, lines []string, vulns *[]Vulnerability) {
	xssPatterns := []struct {
		regex   *regexp.Regexp
		cwe     string
		title   string
		suggest string
	}{
		{
			regex:   regexp.MustCompile(`innerHTML\s*=`),
			cwe:     "CWE-79",
			title:   "XSS风险：innerHTML直接赋值",
			suggest: "使用textContent或sanitize输入",
		},
		{
			regex:   regexp.MustCompile(`document\.write\(`),
			cwe:     "CWE-79",
			title:   "XSS风险：document.write直接输出",
			suggest: "使用DOM API安全地更新内容",
		},
		{
			regex:   regexp.MustCompile(`\$\(` + `[^)]*\)` + `\.html\(`),
			cwe:     "CWE-79",
			title:   "XSS风险：jQuery .html()直接输出",
			suggest: "使用.text()或sanitize输入",
		},
	}

	for i, line := range lines {
		for _, p := range xssPatterns {
			if p.regex.MatchString(line) {
				*vulns = append(*vulns, Vulnerability{
					ID:          fmt.Sprintf("XSS-%d", len(*vulns)+1),
					Category:    "xss",
					Severity:    "high",
					Title:       p.title,
					Description: "检测到潜在的跨站脚本攻击风险",
					Location:    fmt.Sprintf("第 %d 行", i+1),
					Line:        i + 1,
					Pattern:     line,
					Suggestion:  p.suggest,
					CWE:         p.cwe,
				})
			}
		}
	}
}

func (s *SecurityScanner) checkHardcodedSecrets(code string, language string, lines []string, vulns *[]Vulnerability) {
	secretPatterns := []struct {
		regex   *regexp.Regexp
		cwe     string
		title   string
		suggest string
	}{
		{
			regex:   regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*["'][^"']+["']`),
			cwe:     "CWE-798",
			title:   "硬编码密码",
			suggest: "使用环境变量或密钥管理服务",
		},
		{
			regex:   regexp.MustCompile(`(?i)(api_key|apikey|api[-_]?secret)\s*[:=]\s*["'][^"']+["']`),
			cwe:     "CWE-798",
			title:   "硬编码API密钥",
			suggest: "使用环境变量或密钥管理服务",
		},
		{
			regex:   regexp.MustCompile(`(?i)(secret|token)\s*[:=]\s*["'][^"']+["']`),
			cwe:     "CWE-798",
			title:   "硬编码密钥/令牌",
			suggest: "使用环境变量或密钥管理服务",
		},
		{
			regex:   regexp.MustCompile(`(?i)(private_key)\s*[:=]\s*["'][^"']+["']`),
			cwe:     "CWE-798",
			title:   "硬编码私钥",
			suggest: "使用密钥管理服务，不要将私钥存储在代码中",
		},
	}

	for i, line := range lines {
		for _, p := range secretPatterns {
			if p.regex.MatchString(line) {
				*vulns = append(*vulns, Vulnerability{
					ID:          fmt.Sprintf("SECRET-%d", len(*vulns)+1),
					Category:    "secrets",
					Severity:    "critical",
					Title:       p.title,
					Description: "检测到硬编码的敏感信息，可能导致信息泄露",
					Location:    fmt.Sprintf("第 %d 行", i+1),
					Line:        i + 1,
					Pattern:     line,
					Suggestion:  p.suggest,
					CWE:         p.cwe,
				})
			}
		}
	}
}

func (s *SecurityScanner) checkPathTraversal(code string, language string, lines []string, vulns *[]Vulnerability) {
	traversalPatterns := []struct {
		regex   *regexp.Regexp
		cwe     string
		title   string
		suggest string
	}{
		{
			regex:   regexp.MustCompile(`os\.Open\(` + `[^)]*\+`),
			cwe:     "CWE-22",
			title:   "路径遍历风险：动态文件路径",
			suggest: "验证和清理文件路径，使用filepath.Clean",
		},
		{
			regex:   regexp.MustCompile(`ioutil\.ReadFile\(` + `[^)]*\+`),
			cwe:     "CWE-22",
			title:   "路径遍历风险：动态文件读取",
			suggest: "验证和清理文件路径",
		},
	}

	for i, line := range lines {
		for _, p := range traversalPatterns {
			if p.regex.MatchString(line) {
				*vulns = append(*vulns, Vulnerability{
					ID:          fmt.Sprintf("TRAV-%d", len(*vulns)+1),
					Category:    "path_traversal",
					Severity:    "high",
					Title:       p.title,
					Description: "检测到可能的路径遍历漏洞",
					Location:    fmt.Sprintf("第 %d 行", i+1),
					Line:        i + 1,
					Pattern:     line,
					Suggestion:  p.suggest,
					CWE:         p.cwe,
				})
			}
		}
	}
}

func (s *SecurityScanner) checkCommandInjection(code string, language string, lines []string, vulns *[]Vulnerability) {
	cmdPatterns := []struct {
		regex   *regexp.Regexp
		cwe     string
		title   string
		suggest string
	}{
		{
			regex:   regexp.MustCompile(`exec\.Command\(` + `[^)]*\+`),
			cwe:     "CWE-78",
			title:   "命令注入风险：动态命令参数",
			suggest: "使用白名单验证输入，避免直接拼接命令参数",
		},
		{
			regex:   regexp.MustCompile(`os/exec.*Command`),
			cwe:     "CWE-78",
			title:   "命令执行：使用os/exec包",
			suggest: "确保命令参数经过严格验证",
		},
	}

	for i, line := range lines {
		for _, p := range cmdPatterns {
			if p.regex.MatchString(line) {
				*vulns = append(*vulns, Vulnerability{
					ID:          fmt.Sprintf("CMD-%d", len(*vulns)+1),
					Category:    "command_injection",
					Severity:    "critical",
					Title:       p.title,
					Description: "检测到可能的命令注入漏洞",
					Location:    fmt.Sprintf("第 %d 行", i+1),
					Line:        i + 1,
					Pattern:     line,
					Suggestion:  p.suggest,
					CWE:         p.cwe,
				})
			}
		}
	}
}

func (s *SecurityScanner) checkUnsafeDeserialization(code string, language string, lines []string, vulns *[]Vulnerability) {
	unsafePatterns := []struct {
		regex   *regexp.Regexp
		cwe     string
		title   string
		suggest string
	}{
		{
			regex:   regexp.MustCompile(`json\.Unmarshal\(` + `[^)]*,\s*&`),
			cwe:     "CWE-502",
			title:   "不安全反序列化：JSON反序列化",
			suggest: "验证输入数据，限制反序列化的类型",
		},
	}

	for i, line := range lines {
		for _, p := range unsafePatterns {
			if p.regex.MatchString(line) {
				*vulns = append(*vulns, Vulnerability{
					ID:          fmt.Sprintf("DESER-%d", len(*vulns)+1),
					Category:    "deserialization",
					Severity:    "medium",
					Title:       p.title,
					Description: "检测到潜在的不安全反序列化",
					Location:    fmt.Sprintf("第 %d 行", i+1),
					Line:        i + 1,
					Pattern:     line,
					Suggestion:  p.suggest,
					CWE:         p.cwe,
				})
			}
		}
	}
}

func (s *SecurityScanner) checkWeakCrypto(code string, language string, lines []string, vulns *[]Vulnerability) {
	weakPatterns := []struct {
		regex   *regexp.Regexp
		cwe     string
		title   string
		suggest string
	}{
		{
			regex:   regexp.MustCompile(`md5\.New\(\)|md5\.Sum\(`),
			cwe:     "CWE-327",
			title:   "弱加密算法：MD5",
			suggest: "使用SHA-256或更强的哈希算法",
		},
		{
			regex:   regexp.MustCompile(`sha1\.New\(\)|sha1\.Sum\(`),
			cwe:     "CWE-327",
			title:   "弱加密算法：SHA-1",
			suggest: "使用SHA-256或更强的哈希算法",
		},
		{
			regex:   regexp.MustCompile(`des\.|3des\.`),
			cwe:     "CWE-327",
			title:   "弱加密算法：DES/3DES",
			suggest: "使用AES-256等更强的加密算法",
		},
	}

	for i, line := range lines {
		for _, p := range weakPatterns {
			if p.regex.MatchString(line) {
				*vulns = append(*vulns, Vulnerability{
					ID:          fmt.Sprintf("CRYPTO-%d", len(*vulns)+1),
					Category:    "weak_crypto",
					Severity:    "medium",
					Title:       p.title,
					Description: "检测到使用弱加密算法",
					Location:    fmt.Sprintf("第 %d 行", i+1),
					Line:        i + 1,
					Pattern:     line,
					Suggestion:  p.suggest,
					CWE:         p.cwe,
				})
			}
		}
	}
}

func (s *SecurityScanner) checkMemorySafety(code string, language string, lines []string, vulns *[]Vulnerability) {
	if language != "c" && language != "cpp" {
		return
	}

	unsafePatterns := []struct {
		regex   *regexp.Regexp
		cwe     string
		title   string
		suggest string
	}{
		{
			regex:   regexp.MustCompile(`strcpy\(|strcat\(|sprintf\(`),
			cwe:     "CWE-120",
			title:   "缓冲区溢出风险：不安全字符串函数",
			suggest: "使用strncpy, strncat, snprintf等安全版本",
		},
		{
			regex:   regexp.MustCompile(`gets\(`),
			cwe:     "CWE-120",
			title:   "缓冲区溢出风险：gets函数",
			suggest: "使用fgets替代gets",
		},
	}

	for i, line := range lines {
		for _, p := range unsafePatterns {
			if p.regex.MatchString(line) {
				*vulns = append(*vulns, Vulnerability{
					ID:          fmt.Sprintf("MEM-%d", len(*vulns)+1),
					Category:    "memory_safety",
					Severity:    "high",
					Title:       p.title,
					Description: "检测到潜在的内存安全漏洞",
					Location:    fmt.Sprintf("第 %d 行", i+1),
					Line:        i + 1,
					Pattern:     line,
					Suggestion:  p.suggest,
					CWE:         p.cwe,
				})
			}
		}
	}
}

func (s *SecurityScanner) calculateStats(result *SecurityScanResult) {
	result.Stats.TotalIssues = len(result.Vulnerabilities)

	for _, v := range result.Vulnerabilities {
		switch v.Severity {
		case "critical":
			result.Stats.CriticalCount++
		case "high":
			result.Stats.HighCount++
		case "medium":
			result.Stats.MediumCount++
		case "low":
			result.Stats.LowCount++
		}
	}

	// 计算安全评分
	score := 100
	for _, v := range result.Vulnerabilities {
		switch v.Severity {
		case "critical":
			score -= 25
		case "high":
			score -= 15
		case "medium":
			score -= 8
		case "low":
			score -= 3
		}
	}
	if score < 0 {
		score = 0
	}
	result.Score = score

	// 风险等级
	switch {
	case score >= 90:
		result.RiskLevel = "低风险"
	case score >= 70:
		result.RiskLevel = "中风险"
	case score >= 50:
		result.RiskLevel = "高风险"
	default:
		result.RiskLevel = "严重风险"
	}

	result.Summary = fmt.Sprintf("发现 %d 个安全漏洞（严重: %d, 高: %d, 中: %d, 低: %d），安全评分: %d/100，风险等级: %s",
		result.Stats.TotalIssues, result.Stats.CriticalCount, result.Stats.HighCount,
		result.Stats.MediumCount, result.Stats.LowCount, result.Score, result.RiskLevel)
}
