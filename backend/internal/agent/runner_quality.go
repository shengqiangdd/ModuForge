package agent

import (
	"database/sql"
	"fmt"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════
// P2-2: QualityVerifier — Verify code quality
// ═══════════════════════════════════════════════════════════════════

// QualityVerifier checks code quality metrics.
type QualityVerifier struct {
	db *sql.DB
}

// QualityReport contains quality metrics for a file.
type QualityReport struct {
	FilePath    string
	Lines       int
	Complexity  int  // cyclomatic complexity estimate
	Duplication bool // has duplicated code patterns
	HasTests    bool
	HasComments bool
	Score       int // 0-100 quality score
	Issues      []string
}

// VerifyFile checks the quality of a file using O(n) single-pass analysis.
// Instead of 4 separate passes through lines, we do everything in one pass.
func (qv *QualityVerifier) VerifyFile(filePath string, content string) QualityReport {
	report := QualityReport{
		FilePath: filePath,
		Lines:    strings.Count(content, "\n") + 1,
		Issues:   make([]string, 0),
	}

	lines := strings.Split(content, "\n")

	// ═══════════════════════════════════════════════════════════════
	// Single-pass analysis: collect all metrics in one iteration
	// O(n) total instead of O(4n) = O(n)
	// ═══════════════════════════════════════════════════════════════

	longLines := 0
	todoCount := 0
	braceCount := 0
	maxBraceDepth := 0
	magicNumbers := 0
	hasPackage := false
	hasFunc := false
	hasInclude := false
	hasMain := false
	hasFn := false
	importParens := 0
	inImport := false
	inBlockComment := false
	hasSetE := false
	unquotedVarCount := 0
	doubleSemicolonCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineUpper := strings.ToUpper(trimmed)

		// ── Universal checks (all languages) ──

		// 1. Line length
		if len(line) > 120 {
			longLines++
		}

		// 2. TODO/FIXME/HACK
		if strings.Contains(lineUpper, "TODO") || strings.Contains(lineUpper, "FIXME") || strings.Contains(lineUpper, "HACK") {
			todoCount++
		}

		// 3. Brace balance (skip comments)
		if !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "#") {
			for _, ch := range line {
				if ch == '{' {
					braceCount++
					if braceCount > maxBraceDepth {
						maxBraceDepth = braceCount
					}
				} else if ch == '}' {
					braceCount--
				}
			}
		}

		// 4. Magic numbers (skip comments)
		if !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "#") {
			words := strings.Fields(trimmed)
			for _, word := range words {
				if len(word) > 1 && word[0] >= '2' && word[0] <= '9' {
					magicNumbers++
				}
			}
		}

		// ── Language-specific checks (single pass) ──

		// Handle block comments
		if strings.Contains(trimmed, "/*") {
			inBlockComment = true
		}
		if strings.Contains(trimmed, "*/") {
			inBlockComment = false
			continue
		}
		if inBlockComment || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Go-specific
		if strings.HasPrefix(trimmed, "package ") {
			hasPackage = true
		}
		if trimmed == "import (" {
			inImport = true
		}
		if inImport {
			for _, ch := range trimmed {
				if ch == '(' {
					importParens++
				} else if ch == ')' {
					importParens--
					if importParens <= 0 {
						inImport = false
					}
				}
			}
		}
		if strings.HasPrefix(trimmed, "func ") {
			hasFunc = true
		}

		// Rust-specific
		if strings.HasPrefix(trimmed, "fn ") || strings.Contains(trimmed, " fn ") {
			hasFn = true
		}

		// C/C++-specific
		if strings.HasPrefix(trimmed, "#include") {
			hasInclude = true
		}
		if strings.Contains(trimmed, "main(") {
			hasMain = true
		}

		// Shell-specific

		if strings.HasPrefix(trimmed, "set ") && (strings.Contains(trimmed, "-e") || strings.Contains(trimmed, "-o pipefail")) {
			hasSetE = true
		}
		if strings.Contains(trimmed, " $") && !strings.Contains(trimmed, "\"$") && !strings.Contains(trimmed, "'$") {
			unquotedVarCount++
		}

		// Double semicolons (all languages)
		if strings.Contains(trimmed, ";;") {
			doubleSemicolonCount++
		}
	}

	// ═══════════════════════════════════════════════════════════════
	// Build issues from collected metrics (no additional iteration)
	// ═══════════════════════════════════════════════════════════════

	if longLines > 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("%d 行超过120字符", longLines))
	}
	if todoCount > 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("发现 %d 个 TODO/FIXME/HACK 注释", todoCount))
	}
	if braceCount != 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("括号不平衡: { 比 } 多 %d 个（语法错误）", braceCount))
		report.Score -= 30
	}
	if maxBraceDepth > 5 {
		report.Issues = append(report.Issues, fmt.Sprintf("代码嵌套深度 %d 层，建议重构", maxBraceDepth))
		report.Complexity = maxBraceDepth
	}
	if magicNumbers > 5 {
		report.Issues = append(report.Issues, fmt.Sprintf("发现 %d 个可能的魔法数字，建议提取为常量", magicNumbers))
	}
	if doubleSemicolonCount > 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("双分号 ;; 在 %d 处", doubleSemicolonCount))
	}

	// Language-specific issue reporting
	extIdx := strings.LastIndex(filePath, ".")
	if extIdx < 0 {
		extIdx = 0 // no extension found, treat as empty
	}
	ext := strings.ToLower(filePath[extIdx:])
	switch {
	case ext == ".go":
		if !hasPackage && len(lines) > 0 {
			report.Issues = append(report.Issues, "缺少 package 声明（Go 文件必须以 package 开头）")
		}
		if importParens != 0 {
			report.Issues = append(report.Issues, "import 块括号不平衡")
		}
		if !hasFunc && len(lines) > 10 {
			report.Issues = append(report.Issues, "未发现 func 声明，可能缺少函数定义")
		}
	case ext == ".rs":
		if !hasFn && len(lines) > 10 {
			report.Issues = append(report.Issues, "未发现 fn 声明，可能缺少函数定义")
		}
	case ext == ".c", ext == ".cpp", ext == ".cc", ext == ".cxx":
		if !hasInclude && len(lines) > 5 {
			report.Issues = append(report.Issues, "未发现 #include 指令，可能缺少头文件引用")
		}
		if !hasMain && len(lines) > 10 {
			report.Issues = append(report.Issues, "未发现 main 函数，可能缺少程序入口点")
		}
	case ext == ".sh":
		if len(lines) > 0 && !strings.HasPrefix(strings.TrimSpace(lines[0]), "#!") {
			report.Issues = append(report.Issues, "缺少 shebang 行（第一行应为 #!/system/bin/sh 或 #!/bin/bash）")
		}
		if !hasSetE {
			report.Issues = append(report.Issues, "建议添加 set -euo pipefail 以增强错误处理")
		}
		if unquotedVarCount > 3 {
			report.Issues = append(report.Issues, fmt.Sprintf("发现 %d 处可能未加引号的变量，建议使用 \"$VAR\" 格式", unquotedVarCount))
		}
	}

	// ═══════════════════════════════════════════════════════════════
	// Calculate final score (no iteration over issues — use counters)
	// ═══════════════════════════════════════════════════════════════

	report.Score = 100
	// Deduct based on severity (using counters, not string matching)
	if braceCount != 0 {
		report.Score -= 30
	}
	if maxBraceDepth > 5 {
		report.Score -= 20
	}
	if longLines > 0 {
		report.Score -= 5
	}
	if magicNumbers > 5 {
		report.Score -= 10
	}
	if todoCount > 0 {
		report.Score -= 8
	}
	// Language-specific deductions
	if ext == ".go" && importParens != 0 {
		report.Score -= 15
	}
	if doubleSemicolonCount > 0 {
		report.Score -= 5
	}
	if report.Score < 0 {
		report.Score = 0
	}

	return report
}

// GetQualitySummary returns a summary of quality reports with syntax-aware insights.
func (qv *QualityVerifier) GetQualitySummary(reports []QualityReport) string {
	if len(reports) == 0 {
		return "无文件需要检查"
	}

	totalScore := 0
	totalIssues := 0
	syntaxIssues := 0
	for _, r := range reports {
		totalScore += r.Score
		totalIssues += len(r.Issues)
		for _, issue := range r.Issues {
			if strings.Contains(issue, "括号不平衡") || strings.Contains(issue, "语法错误") || strings.Contains(issue, "缺少") {
				syntaxIssues++
			}
		}
	}
	avgScore := totalScore / len(reports)

	summary := "📊 代码质量报告:\n"
	summary += fmt.Sprintf("- 检查文件: %d\n", len(reports))
	summary += fmt.Sprintf("- 平均质量分: %d/100\n", avgScore)
	summary += fmt.Sprintf("- 发现问题: %d (其中语法问题: %d)\n", totalIssues, syntaxIssues)

	if syntaxIssues > 0 {
		summary += "- ⚠️ 发现潜在语法错误，建议先用 syntax_checker 工具验证再构建\n"
	}

	if avgScore >= 80 {
		summary += "- 评价: ✅ 良好"
	} else if avgScore >= 60 {
		summary += "- 评价: ⚠️ 一般，建议优化"
	} else {
		summary += "- 评价: ❌ 较差，需要重构"
	}

	return summary
}
