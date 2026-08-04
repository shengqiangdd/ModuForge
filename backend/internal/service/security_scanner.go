package service

import (
	"fmt"
	"regexp"
	"strings"
)

type SecurityIssue struct {
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
}

type SecurityScanResult struct {
	Safe    bool            `json:"safe"`
	Issues  []SecurityIssue `json:"issues"`
	Score   int             `json:"score"`
	Summary string          `json:"summary"`
}

type SecurityScanner struct{}

func NewSecurityScanner() *SecurityScanner {
	return &SecurityScanner{}
}

var (
	reChmod777       = regexp.MustCompile(`chmod\s+777`)
	reEvalWithVar    = regexp.MustCompile(`eval\s+\$`)
	reHardcodedKey   = regexp.MustCompile(`(?i)(api[_-]?key|api[_-]?secret|token|password|secret)\s*[=:]\s*['"][A-Za-z0-9_\-]{16,}['"]`)
	reCurlPipeSh     = regexp.MustCompile(`curl\s+.*\|\s*(sh|bash)`)
	reRmRf           = regexp.MustCompile(`rm\s+(-rf|--recursive)\s+/`)
	reUnquotedVar    = regexp.MustCompile(`[^"'\$]\$[A-Za-z_][A-Za-z0-9_]*`)
	reMissingSetEuo  = regexp.MustCompile(`set\s+-[^ ]*e[^ ]*u[^ ]*o[^ ]*pipefail`)
	reShebang        = regexp.MustCompile(`^#!`)
	reWhich          = regexp.MustCompile(`\bwhich\b`)
	reTrapExit       = regexp.MustCompile(`trap\s+.*EXIT`)
	reChmodSuggest   = regexp.MustCompile(`chmod\s+`)
	reUiPrint        = regexp.MustCompile(`\bui_print\b`)
	reAbort          = regexp.MustCompile(`\babort\b`)
	// Universal checks
	reJsEval         = regexp.MustCompile(`eval\s*\(`)
	reInnerHTML      = regexp.MustCompile(`innerHTML\s*=`)
	reDocumentWrite  = regexp.MustCompile(`document\.write\s*\(`)
	rePyExec         = regexp.MustCompile(`exec\s*\(`)
	rePickleLoads    = regexp.MustCompile(`pickle\.loads?\s*\(`)
	reSubprocessShell = regexp.MustCompile(`subprocess\.call.*shell\s*=\s*True`)
	reGoCmdConcat    = regexp.MustCompile(`exec\.Command.*\+`)
	reHtmlInlineEvent = regexp.MustCompile(`on\w+\s*=`)
	reConfigSecret   = regexp.MustCompile(`(?i)(password|secret|token)\s*[":]\s*['"][^'"]{8,}['"]`)
)

func (s *SecurityScanner) ScanFile(filename string, content string) SecurityScanResult {
	result := SecurityScanResult{Safe: true, Score: 100}

	lines := strings.Split(content, "\n")
	var issueList []SecurityIssue

	addIssue := func(severity, rule, msg string, line int) {
		issueList = append(issueList, SecurityIssue{
			Severity: severity,
			File:     filename,
			Line:     line,
			Rule:     rule,
			Message:  msg,
		})
		if severity == "critical" {
			result.Safe = false
		}
	}

	wholeContent := content

	// ===== Universal checks (all file types) =====
	for i, line := range lines {
		lineNum := i + 1
		// Hardcoded secrets in any file
		if reHardcodedKey.MatchString(line) {
			addIssue("critical", "HARDCODED_SECRET", "hardcoded API key/token/secret detected", lineNum)
		}
		// Dangerous patterns in JS/TS files
		if strings.HasSuffix(filename, ".js") || strings.HasSuffix(filename, ".ts") || strings.HasSuffix(filename, ".jsx") || strings.HasSuffix(filename, ".tsx") {
			if reJsEval.MatchString(line) {
				addIssue("critical", "EVAL_USAGE", "eval() usage allows code injection", lineNum)
			}
			if reInnerHTML.MatchString(line) {
				addIssue("warning", "INNERHTML", "innerHTML assignment may cause XSS", lineNum)
			}
			if reDocumentWrite.MatchString(line) {
				addIssue("warning", "DOCUMENT_WRITE", "document.write() may cause XSS", lineNum)
			}
		}
		// Dangerous patterns in Python files
		if strings.HasSuffix(filename, ".py") {
			if rePyExec.MatchString(line) {
				addIssue("critical", "EXEC_USAGE", "exec() allows arbitrary code execution", lineNum)
			}
			if rePickleLoads.MatchString(line) {
				addIssue("critical", "PICKLE_LOADS", "pickle deserialization can execute arbitrary code", lineNum)
			}
			if reSubprocessShell.MatchString(line) {
				addIssue("warning", "SHELL_TRUE", "subprocess with shell=True may cause injection", lineNum)
			}
		}
		// Dangerous patterns in Go files
		if strings.HasSuffix(filename, ".go") {
			if reGoCmdConcat.MatchString(line) {
				addIssue("warning", "CMD_CONCAT", "string concatenation in exec.Command may cause injection", lineNum)
			}
		}
		// Dangerous patterns in HTML files
		if strings.HasSuffix(filename, ".html") || strings.HasSuffix(filename, ".htm") {
			if reHtmlInlineEvent.MatchString(line) {
				addIssue("info", "INLINE_EVENT", "inline event handlers detected, consider separation", lineNum)
			}
		}
		// Dangerous patterns in config files
		if strings.HasSuffix(filename, ".json") || strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml") {
			if reConfigSecret.MatchString(line) {
				addIssue("critical", "HARDCODED_SECRET", "hardcoded secret in config file", lineNum)
			}
		}
	}

	// ===== Shell-specific checks =====
	if strings.HasSuffix(filename, ".sh") {
		for i, line := range lines {
			lineNum := i + 1

			if reChmod777.MatchString(line) {
				addIssue("critical", "CHMOD_777", "chmod 777 grants excessive permissions, use 755 or 644", lineNum)
			}
			if reEvalWithVar.MatchString(line) {
				addIssue("critical", "EVAL_VARIABLE", "eval with variable allows code injection", lineNum)
			}
			if reCurlPipeSh.MatchString(line) {
				addIssue("critical", "CURL_PIPE_SH", "curl | sh downloads and executes remote code", lineNum)
			}
			if reRmRf.MatchString(line) {
				addIssue("critical", "RM_RF_ROOT", "rm -rf / is extremely dangerous", lineNum)
			}

			matches := reUnquotedVar.FindAllString(line, -1)
			for _, m := range matches {
				addIssue("warning", "UNQUOTED_VARIABLE", fmt.Sprintf("unquoted variable %s may cause command injection", strings.TrimSpace(m)), lineNum)
			}
		}

		if !reMissingSetEuo.MatchString(wholeContent) {
			addIssue("info", "MISSING_SET_EUO", "missing 'set -euo pipefail' for safe error handling", 0)
		}

		if i := strings.Index(wholeContent, "\n"); i > 0 {
			firstLine := wholeContent[:i]
			if !reShebang.MatchString(firstLine) {
				addIssue("warning", "MISSING_SHEBANG", "missing shebang (e.g., #!/system/bin/sh)", 1)
			}
		} else if !reShebang.MatchString(wholeContent) {
			addIssue("warning", "MISSING_SHEBANG", "missing shebang (e.g., #!/system/bin/sh)", 1)
		}

		if reWhich.MatchString(wholeContent) {
			addIssue("warning", "USING_WHICH", "use 'command -v' instead of 'which' for portability", 0)
		}

		if strings.Contains(wholeContent, "mktemp") && !reTrapExit.MatchString(wholeContent) {
			addIssue("warning", "MISSING_TRAP", "temporary files created with mktemp but no trap ... EXIT cleanup", 0)
		}

		if reChmodSuggest.MatchString(wholeContent) && !reChmod777.MatchString(wholeContent) {
			addIssue("info", "FILE_PERMISSION", "verify file permissions are minimal required", 0)
		}

		if strings.Contains(wholeContent, "ui_print") || strings.Contains(wholeContent, "echo") {
			if !reUiPrint.MatchString(wholeContent) {
				addIssue("info", "USE_UI_PRINT", "use 'ui_print' for user messages in module install scripts", 0)
			}
			if strings.Contains(wholeContent, "exit") && !reAbort.MatchString(wholeContent) {
				addIssue("info", "USE_ABORT", "use 'abort' function instead of 'exit' for error handling in module scripts", 0)
			}
		}
	}

	result.Issues = issueList

	criticalCount := 0
	warningCount := 0
	infoCount := 0
	for _, issue := range issueList {
		switch issue.Severity {
		case "critical":
			criticalCount++
		case "warning":
			warningCount++
		case "info":
			infoCount++
		}
	}

	score := 100
	score -= criticalCount * 15
	score -= warningCount * 2
	score -= infoCount
	if score < 0 {
		score = 0
	}
	if criticalCount == 0 && score < 40 {
		score = 40
	}
	result.Score = score

	result.Safe = result.Safe && criticalCount == 0

	switch {
	case criticalCount > 0:
		result.Summary = fmt.Sprintf("发现 %d 个严重问题、%d 个警告、%d 个提示 — 存在安全风险", criticalCount, warningCount, infoCount)
	case warningCount > 0:
		result.Summary = fmt.Sprintf("发现 %d 个警告、%d 个提示 — 建议修复", warningCount, infoCount)
	case infoCount > 0:
		result.Summary = fmt.Sprintf("发现 %d 个提示 — 整体安全", infoCount)
	default:
		result.Summary = "未发现安全风险"
	}

	return result
}

func (s *SecurityScanner) ScanFiles(files map[string]string) SecurityScanResult {
	combined := SecurityScanResult{Safe: true, Score: 100}
	totalCritical := 0
	totalWarning := 0
	totalInfo := 0

	// Deduplicate by issue code across files (keep first occurrence)
	seen := make(map[string]bool)
	for filename, content := range files {
		res := s.ScanFile(filename, content)
		if !res.Safe {
			combined.Safe = false
		}
		for _, issue := range res.Issues {
			if seen[issue.Rule] {
				continue
			}
			seen[issue.Rule] = true
			combined.Issues = append(combined.Issues, issue)
			switch issue.Severity {
			case "critical":
				totalCritical++
			case "warning":
				totalWarning++
			case "info":
				totalInfo++
			}
		}
	}

	score := 100
	score -= totalCritical * 15
	score -= totalWarning * 2
	score -= totalInfo
	if score < 0 {
		score = 0
	}
	// Don't show 0/100 for non-critical issues
	if totalCritical == 0 && score < 40 {
		score = 40
	}
	combined.Score = score

	switch {
	case totalCritical > 0:
		combined.Summary = fmt.Sprintf("共 %d 个严重问题、%d 个警告、%d 个提示 — 存在安全风险", totalCritical, totalWarning, totalInfo)
	case totalWarning > 0:
		combined.Summary = fmt.Sprintf("共 %d 个警告、%d 个提示 — 建议修复后再构建", totalWarning, totalInfo)
	case totalInfo > 0:
		combined.Summary = fmt.Sprintf("共 %d 个提示 — 整体安全", totalInfo)
	default:
		combined.Summary = "未发现安全风险"
	}

	return combined
}
