package agent

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// ═══════════════════════════════════════════════════════════════════
// ValidateGeneratedCode — 完整代码质量验证管道
// ═══════════════════════════════════════════════════════════════════

// ValidationResult contains the result of code validation.
type ValidationResult struct {
	Passed      bool     `json:"passed"`
	Score       int      `json:"score"`        // 0-100
	GofmtIssues int      `json:"gofmt_issues"` // files with formatting issues
	VetIssues   int      `json:"vet_issues"`   // files with vet warnings
	SecIssues   int      `json:"sec_issues"`   // security issues found
	ComplexHigh int      `json:"complex_high"` // files with high complexity
	Issues      []string `json:"issues"`
	Warnings    []string `json:"warnings"`
}

// ValidateGeneratedCode runs static analysis, security scanning, and quality checks on generated code.
func ValidateGeneratedCode(projectDir string) *ValidationResult {
	result := &ValidationResult{
		Passed:   true,
		Score:    100,
		Issues:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	// 1. Run gofmt check (Go files)
	gofmtIssues := runGofmtCheck(projectDir)
	result.GofmtIssues = gofmtIssues
	if gofmtIssues > 0 {
		result.Score -= gofmtIssues * 5
		result.Issues = append(result.Issues, fmt.Sprintf("%d 个 Go 文件格式不规范", gofmtIssues))
	}

	// 2. Run go vet (Go files)
	vetIssues := runGoVet(projectDir)
	result.VetIssues = vetIssues
	if vetIssues > 0 {
		result.Score -= vetIssues * 10
		result.Issues = append(result.Issues, fmt.Sprintf("go vet 发现 %d 个问题", vetIssues))
	}

	// 3. Security scan
	secIssues := runSecurityScan(projectDir)
	result.SecIssues = len(secIssues)
	result.Issues = append(result.Issues, secIssues...)
	result.Score -= len(secIssues) * 15

	// 4. Complexity check
	complexFiles := runComplexityCheck(projectDir)
	result.ComplexHigh = len(complexFiles)
	result.Warnings = append(result.Warnings, complexFiles...)
	result.Score -= len(complexFiles) * 3

	// 5. Dependency validation
	depIssues := runDependencyCheck(projectDir)
	if len(depIssues) > 0 {
		result.Score -= len(depIssues) * 5
		result.Issues = append(result.Issues, depIssues...)
	}

	// Clamp score
	if result.Score < 0 {
		result.Score = 0
	}
	if result.Score >= 80 {
		result.Passed = true
	} else {
		result.Passed = false
	}

	return result
}

// runGofmtCheck checks Go files for formatting issues.
func runGofmtCheck(projectDir string) int {
	goFiles := findGoFiles(projectDir)
	issues := 0
	for _, f := range goFiles {
		cmd := exec.Command("gofmt", "-l", f)
		output, err := cmd.CombinedOutput()
		if err == nil && len(strings.TrimSpace(string(output))) > 0 {
			issues++
		}
	}
	return issues
}

// runGoVet runs go vet on the project.
func runGoVet(projectDir string) int {
	goFiles := findGoFiles(projectDir)
	if len(goFiles) == 0 {
		return 0
	}
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Count error lines
		lines := strings.Split(string(output), "\n")
		count := 0
		for _, line := range lines {
			if strings.Contains(line, "Error:") || strings.Contains(line, "error:") {
				count++
			}
		}
		return count
	}
	return 0
}

// runSecurityScan scans code for common security issues.
func runSecurityScan(projectDir string) []string {
	var issues []string

	// Patterns that indicate security concerns
	securityPatterns := []struct {
		pattern  string
		message  string
		severity string
	}{
		{`os\.Getenv\s*\(\s*"[^"]*"\s*\)`, "环境变量直接使用，建议验证输入", "low"},
		{`exec\.Command\s*\([^)]*strings\.ReplaceAll`, "exec.Command 使用字符串拼接，存在注入风险", "high"},
		{`fmt\.Sprintf\s*\(\s*"[^"]*%s[^"]*".*query`, "SQL 查询使用字符串拼接，存在注入风险", "high"},
		{`http\.Get\s*\(`, "HTTP GET 未设置超时", "low"},
		{`ioutil\.ReadFile|os\.ReadFile.*\+`, "文件路径拼接，验证路径安全", "medium"},
		{`math/rand`, "使用 math/rand（不安全），建议使用 crypto/rand", "medium"},
		{`password.*=.*"[^"]+"`, "硬编码密码", "high"},
		{`secret.*=.*"[^"]+"`, "硬编码密钥", "high"},
		{`api_key.*=.*"[^"]+"`, "硬编码 API Key", "high"},
	}

	goFiles := findGoFiles(projectDir)
	cFiles := findCFiles(projectDir)
	allFiles := append(goFiles, cFiles...)

	for _, f := range allFiles {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		contentStr := string(content)

		for _, sp := range securityPatterns {
			re := regexp.MustCompile(sp.pattern)
			if re.MatchString(contentStr) {
				relPath, _ := filepath.Rel(projectDir, f)
				issues = append(issues, fmt.Sprintf("[%s] %s: %s", sp.severity, relPath, sp.message))
			}
		}
	}

	return issues
}

// runComplexityCheck checks for overly complex code.
func runComplexityCheck(projectDir string) []string {
	var warnings []string

	goFiles := findGoFiles(projectDir)
	for _, f := range goFiles {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lines := strings.Split(string(content), "\n")

		// Check function length
		inFunc := false
		funcStart := 0
		funcName := ""
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "func ") {
				inFunc = true
				funcStart = i
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					funcName = parts[1]
					if idx := strings.Index(funcName, "("); idx > 0 {
						funcName = funcName[:idx]
					}
				}
			}
			if inFunc && trimmed == "}" {
				funcLen := i - funcStart
				if funcLen > 100 {
					relPath, _ := filepath.Rel(projectDir, f)
					warnings = append(warnings, fmt.Sprintf("%s: 函数 %s 长度 %d 行（建议 <100 行）", relPath, funcName, funcLen))
				}
				inFunc = false
			}
		}
	}

	return warnings
}

// runDependencyCheck validates go.mod dependencies.
func runDependencyCheck(projectDir string) []string {
	var issues []string

	goModPath := filepath.Join(projectDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return issues // No go.mod, skip
	}

	// Check for go.sum existence
	goSumPath := filepath.Join(projectDir, "go.sum")
	if _, err := os.Stat(goSumPath); os.IsNotExist(err) {
		issues = append(issues, "缺少 go.sum 文件，依赖未正确初始化")
	}

	// Check for unused imports by running goimports (if available)
	// This is a heuristic — full check requires goimports tool
	content, err := os.ReadFile(goModPath)
	if err == nil {
		contentStr := string(content)
		// Check for common issues
		if strings.Contains(contentStr, "replace ../") {
			issues = append(issues, "go.mod 包含本地 replace 指令，不适合生产环境")
		}
	}

	return issues
}

// findGoFiles finds all .go files in a directory recursively.
func findGoFiles(dir string) []string {
	var files []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// findCFiles finds all .c files in a directory recursively.
func findCFiles(dir string) []string {
	var files []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && (strings.HasSuffix(path, ".c") || strings.HasSuffix(path, ".cpp")) {
			files = append(files, path)
		}
		return nil
	})
	return files
}
