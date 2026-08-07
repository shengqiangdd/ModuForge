package skills

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"github.com/moduforge/backend/internal/agent/registry"
)

// SecurityScanSkill performs static security scanning on source code.
// Detects: SQL injection, command injection, path traversal, hardcoded secrets,
// insecure crypto, unsafe deserialization, XSS, and SSRF patterns.
type SecurityScanSkill struct{}

func NewSecurityScanSkill() *SecurityScanSkill {
	return &SecurityScanSkill{}
}

func (s *SecurityScanSkill) Name() string { return "security_scan" }

func (s *SecurityScanSkill) Description() string {
	return "Static security scan on source code. Input: {\"path\": \"...\", \"content\": \"...\", \"language\": \"go|rust|c++|shell|python|typescript\"}. Detects: SQL injection, command injection, path traversal, hardcoded secrets, insecure crypto, XSS, SSRF. Returns categorized vulnerabilities with severity and fix suggestions."
}

type SecurityFinding struct {
	Severity    string `json:"severity"`    // critical, high, medium, low, info
	Category    string `json:"category"`    // injection, secrets, crypto, xss, ssrf, path_traversal, insecure
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
	CWE         string `json:"cwe"` // CWE ID e.g. "CWE-89"
}

func (s *SecurityScanSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	content, _ := input["content"].(string)
	language, _ := input["language"].(string)
	path, _ := input["path"].(string)

	if content == "" {
		return "", fmt.Errorf("content is required")
	}
	if language == "" {
		language = detectLanguage(path)
	}

	var findings []SecurityFinding
	lines := strings.Split(content, "\n")

	// Universal patterns (all languages)
	findings = append(findings, scanUniversalSecrets(lines)...)
	findings = append(findings, scanUniversalInsecurePatterns(lines)...)

	// Language-specific patterns
	switch strings.ToLower(language) {
	case "go":
		findings = append(findings, scanGoSecurity(lines)...)
	case "rust":
		findings = append(findings, scanRustSecurity(lines)...)
	case "python":
		findings = append(findings, scanPythonSecurity(lines)...)
	case "shell":
		findings = append(findings, scanShellSecurity(lines)...)
	case "c++", "c":
		findings = append(findings, scanCppSecurity(lines)...)
	case "typescript", "javascript":
		findings = append(findings, scanJSSecurity(lines)...)
	}

	return formatSecurityReport(path, findings, len(lines)), nil
}

func (s *SecurityScanSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  true,
		Essential: false,
		Core:      false,
		NeedsDB:   false,
		NeedsLLM:  false,
	}
}

// ── Universal Secret Detection ──

var secretPatterns = []struct {
	name    string
	pattern *regexp.Regexp
	cwe     string
}{
	{"hardcoded_password", regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*["'][^"']{4,}`), "CWE-798"},
	{"hardcoded_api_key", regexp.MustCompile(`(?i)(api[_-]?key|apikey)\s*[:=]\s*["'][^"']{8,}`), "CWE-798"},
	{"hardcoded_secret", regexp.MustCompile(`(?i)(secret|token)\s*[:=]\s*["'][^"']{8,}`), "CWE-798"},
	{"private_key_block", regexp.MustCompile(`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`), "CWE-321"},
	{"aws_key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`), "CWE-798"},
	{"jwt_token", regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.`), "CWE-798"},
}

func scanUniversalSecrets(lines []string) []SecurityFinding {
	var findings []SecurityFinding
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		for _, sp := range secretPatterns {
			if sp.pattern.MatchString(line) {
				findings = append(findings, SecurityFinding{
					Severity:    "critical",
					Category:    "secrets",
					Line:        i + 1,
					Description: fmt.Sprintf("Possible hardcoded secret detected: %s", sp.name),
					Suggestion:  "Use environment variables, secrets manager, or configuration files. Never commit secrets to source code.",
					CWE:         sp.cwe,
				})
			}
		}
	}
	return findings
}

func scanUniversalInsecurePatterns(lines []string) []SecurityFinding {
	var findings []SecurityFinding
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Insecure random
		if strings.Contains(trimmed, "rand.Intn") || strings.Contains(trimmed, "math/rand") ||
			strings.Contains(trimmed, "random.randint") || strings.Contains(trimmed, "Math.random") {
			findings = append(findings, SecurityFinding{
				Severity:    "medium",
				Category:    "crypto",
				Line:        i + 1,
				Description: "Use of non-cryptographic random number generator",
				Suggestion:  "For security-sensitive operations, use crypto/rand (Go), rand::thread_rng (Rust), secrets module (Python), or crypto.getRandomValues (JS)",
				CWE:         "CWE-330",
			})
		}
	}
	return findings
}

// ── Go-specific Security ──

var goInsecurePatterns = []struct {
	name    string
	pattern *regexp.Regexp
	severity string
	cwe     string
}{
	{"sql_injection", regexp.MustCompile(`fmt\.(Sprintf|Fprintf)\s*\(\s*"[^"]*SELECT[^"]*%s`), "high", "CWE-89"},
	{"sql_injection_insert", regexp.MustCompile(`fmt\.(Sprintf|Fprintf)\s*\(\s*"[^"]*INSERT[^"]*%s`), "high", "CWE-89"},
	{"command_injection", regexp.MustCompile(`exec\.Command\s*\([^,)]*,\s*[^)]*\+`), "high", "CWE-78"},
	{"command_injection_shell", regexp.MustCompile(`exec\.Command\s*\(\s*"[^"]*sh[^"]*"\s*,\s*"[^"]*-c[^"]*"\s*,`), "high", "CWE-78"},
	{"path_traversal", regexp.MustCompile(`os\.Open\s*\([^)]*\+`), "high", "CWE-22"},
	{"unsafe_sql", regexp.MustCompile(`db\.Exec\s*\(\s*"[^"]*SELECT`), "high", "CWE-89"},
	{"tls_skip", regexp.MustCompile(`InsecureSkipVerify\s*:\s*true`), "high", "CWE-295"},
	{"md5_hash", regexp.MustCompile(`md5\.New\(\)`), "medium", "CWE-328"},
	{"http_no_tls", regexp.MustCompile(`http\.Get\s*\(`), "low", "CWE-319"},
}

func scanGoSecurity(lines []string) []SecurityFinding {
	var findings []SecurityFinding
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		for _, pat := range goInsecurePatterns {
			if pat.pattern.MatchString(line) {
				severity := pat.severity
				if severity == "" {
					severity = "medium"
				}
				findings = append(findings, SecurityFinding{
					Severity:    severity,
					Category:    "injection",
					Line:        i + 1,
					Description: fmt.Sprintf("Insecure pattern: %s", pat.name),
					Suggestion:  getGoFixSuggestion(pat.name),
					CWE:         pat.cwe,
				})
			}
		}
	}
	return findings
}

func getGoFixSuggestion(name string) string {
	switch name {
	case "sql_injection", "sql_injection_insert", "unsafe_sql":
		return "Use parameterized queries: db.Query(\"SELECT * FROM users WHERE id=?\", id)"
	case "command_injection", "command_injection_shell":
		return "Use exec.Command with fixed arguments, avoid string concatenation"
	case "path_traversal":
		return "Validate and sanitize file paths, use filepath.Clean, check against allowed directories"
	case "tls_skip":
		return "Remove InsecureSkipVerify or set to false in production"
	case "md5_hash":
		return "Use SHA-256 or stronger: crypto/sha256"
	case "http_no_tls":
		return "Use HTTPS: http.DefaultClient with TLS config"
	default:
		return "Review and fix the insecure pattern"
	}
}

// ── Rust-specific Security ──

func scanRustSecurity(lines []string) []SecurityFinding {
	var findings []SecurityFinding
	rustPatterns := []struct {
		name     string
		pattern  *regexp.Regexp
		severity string
		cwe      string
	}{
		{"unsafe_block", regexp.MustCompile(`unsafe\s*\{`), "medium", "CWE-787"},
		{"unwrap_panic", regexp.MustCompile(`\.unwrap\(\)`), "low", "CWE-248"},
		{"sql_injection", regexp.MustCompile(`format!\s*\(\s*"[^"]*SELECT[^"]*\{`), "high", "CWE-89"},
		{"command_injection", regexp.MustCompile(`Command::new\s*\([^)]*\+`), "high", "CWE-78"},
		{"path_traversal", regexp.MustCompile(`File::open\s*\([^)]*\+`), "high", "CWE-22"},
		{"transmute", regexp.MustCompile(`std::mem::transmute`), "high", "CWE-787"},
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		for _, pat := range rustPatterns {
			if pat.pattern.MatchString(line) {
				findings = append(findings, SecurityFinding{
					Severity:    pat.severity,
					Category:    "injection",
					Line:        i + 1,
					Description: fmt.Sprintf("Insecure pattern: %s", pat.name),
					Suggestion:  getRustFixSuggestion(pat.name),
					CWE:         pat.cwe,
				})
			}
		}
	}
	return findings
}

func getRustFixSuggestion(name string) string {
	switch name {
	case "unsafe_block":
		return "Minimize unsafe blocks, document safety invariants, use safe abstractions"
	case "unwrap_panic":
		return "Use proper error handling: .unwrap_or(), .map_err(), or match/if let"
	case "sql_injection":
		return "Use parameterized queries with sqlx or diesel"
	case "command_injection":
		return "Use Command::new with fixed arguments"
	case "path_traversal":
		return "Validate paths, use std::path::Path and canonicalize"
	case "transmute":
		return "Avoid transmute, use safe alternatives like From/Into traits"
	default:
		return "Review and fix the insecure pattern"
	}
}

// ── Python-specific Security ──

func scanPythonSecurity(lines []string) []SecurityFinding {
	var findings []SecurityFinding
	pyPatterns := []struct {
		name     string
		pattern  *regexp.Regexp
		severity string
		cwe      string
	}{
		{"eval_exec", regexp.MustCompile(`\b(eval|exec)\s*\(`), "critical", "CWE-95"},
		{"pickle_load", regexp.MustCompile(`pickle\.loads?\s*\(`), "high", "CWE-502"},
		{"yaml_load", regexp.MustCompile(`yaml\.load\s*\([^)]*\)`), "high", "CWE-502"},
		{"shell_true", regexp.MustCompile(`subprocess\.\w+\s*\([^)]*shell\s*=\s*True`), "high", "CWE-78"},
		{"sql_format", regexp.MustCompile(`["'].*%s.*["']\s*%\s*`), "high", "CWE-89"},
		{"sql_fstring", regexp.MustCompile(`f["'].*SELECT.*\{`), "high", "CWE-89"},
		{"tempfile_mktemp", regexp.MustCompile(`tempfile\.mktemp\s*\(`), "medium", "CWE-377"},
		{"debug_true", regexp.MustCompile(`DEBUG\s*=\s*True`), "medium", "CWE-489"},
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		for _, pat := range pyPatterns {
			if pat.pattern.MatchString(line) {
				findings = append(findings, SecurityFinding{
					Severity:    pat.severity,
					Category:    "injection",
					Line:        i + 1,
					Description: fmt.Sprintf("Insecure pattern: %s", pat.name),
					Suggestion:  getPythonFixSuggestion(pat.name),
					CWE:         pat.cwe,
				})
			}
		}
	}
	return findings
}

func getPythonFixSuggestion(name string) string {
	switch name {
	case "eval_exec":
		return "Never use eval/exec on untrusted input. Use ast.literal_eval for safe evaluation"
	case "pickle_load":
		return "Use JSON or msgpack instead of pickle for untrusted data"
	case "yaml_load":
		return "Use yaml.safe_load() or yaml.load(data, Loader=yaml.SafeLoader)"
	case "shell_true":
		return "Use subprocess.run(cmd_list, shell=False) with a list of arguments"
	case "sql_format", "sql_fstring":
		return "Use parameterized queries: cursor.execute('SELECT * FROM t WHERE id=?', (id,))"
	case "tempfile_mktemp":
		return "Use tempfile.mkstemp() or tempfile.NamedTemporaryFile() instead"
	case "debug_true":
		return "Use environment variable to control DEBUG in production"
	default:
		return "Review and fix the insecure pattern"
	}
}

// ── Shell-specific Security ──

func scanShellSecurity(lines []string) []SecurityFinding {
	var findings []SecurityFinding
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Unquoted variables in dangerous contexts
		if strings.Contains(trimmed, "$(") || strings.Contains(trimmed, "`") {
			if strings.Contains(trimmed, "rm ") || strings.Contains(trimmed, "mv ") ||
				strings.Contains(trimmed, "cp ") || strings.Contains(trimmed, "chmod ") {
				findings = append(findings, SecurityFinding{
					Severity:    "high",
					Category:    "path_traversal",
					Line:        i + 1,
					Description: "Command substitution used with dangerous command",
					Suggestion:  "Quote all variables and validate inputs before using in commands",
					CWE:         "CWE-78",
				})
			}
		}
		// Eval usage
		if strings.Contains(trimmed, "eval ") {
			findings = append(findings, SecurityFinding{
				Severity:    "high",
				Category:    "injection",
				Line:        i + 1,
				Description: "Use of eval in shell script",
				Suggestion:  "Avoid eval, use direct variable assignment or proper parsing",
				CWE:         "CWE-95",
			})
		}
		// Insecure temp files
		if strings.Contains(trimmed, "/tmp/") && !strings.Contains(trimmed, "$RANDOM") {
			findings = append(findings, SecurityFinding{
				Severity:    "medium",
				Category:    "insecure",
				Line:        i + 1,
				Description: "Hardcoded temp file path — race condition risk",
				Suggestion:  "Use mktemp to create temporary files safely",
				CWE:         "CWE-377",
			})
		}
	}
	return findings
}

// ── C/C++-specific Security ──

func scanCppSecurity(lines []string) []SecurityFinding {
	var findings []SecurityFinding
	cppPatterns := []struct {
		name     string
		pattern  *regexp.Regexp
		severity string
		cwe      string
	}{
		{"gets", regexp.MustCompile(`\bgets\s*\(`), "critical", "CWE-120"},
		{"sprintf", regexp.MustCompile(`\bsprintf\s*\(`), "high", "CWE-120"},
		{"strcpy", regexp.MustCompile(`\bstrcpy\s*\(`), "high", "CWE-120"},
		{"strcat", regexp.MustCompile(`\bstrcat\s*\(`), "high", "CWE-120"},
		{"scanf", regexp.MustCompile(`\bscanf\s*\(`), "medium", "CWE-120"},
		{"malloc_free", regexp.MustCompile(`\bfree\s*\(`), "info", "CWE-762"},
		{"system_call", regexp.MustCompile(`\bsystem\s*\(`), "high", "CWE-78"},
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		for _, pat := range cppPatterns {
			if pat.pattern.MatchString(line) {
				findings = append(findings, SecurityFinding{
					Severity:    pat.severity,
					Category:    "injection",
					Line:        i + 1,
					Description: fmt.Sprintf("Unsafe function: %s", pat.name),
					Suggestion:  getCppFixSuggestion(pat.name),
					CWE:         pat.cwe,
				})
			}
		}
	}
	return findings
}

func getCppFixSuggestion(name string) string {
	switch name {
	case "gets":
		return "Remove gets() entirely — it's deprecated. Use fgets() or std::getline"
	case "sprintf":
		return "Use snprintf() to prevent buffer overflow"
	case "strcpy":
		return "Use strncpy() or strlcpy() with bounds checking"
	case "strcat":
		return "Use strncat() or std::string concatenation"
	case "scanf":
		return "Use fgets() + sscanf() with width specifiers"
	case "system_call":
		return "Use execve() or posix_spawn() instead of system()"
	default:
		return "Review and replace with safer alternative"
	}
}

// ── JavaScript/TypeScript-specific Security ──

func scanJSSecurity(lines []string) []SecurityFinding {
	var findings []SecurityFinding
	jsPatterns := []struct {
		name     string
		pattern  *regexp.Regexp
		severity string
		cwe      string
	}{
		{"eval", regexp.MustCompile(`\beval\s*\(`), "high", "CWE-95"},
		{"innerHTML", regexp.MustCompile(`\.innerHTML\s*=`), "high", "CWE-79"},
		{"document_write", regexp.MustCompile(`document\.write\s*\(`), "medium", "CWE-79"},
		{"dangerouslySetInnerHTML", regexp.MustCompile(`dangerouslySetInnerHTML`), "high", "CWE-79"},
		{"sql_concat", regexp.MustCompile(`["'].*SELECT.*["']\s*\+\s*`), "high", "CWE-89"},
		{"regex_dos", regexp.MustCompile(`new RegExp\s*\([^)]*\+`), "medium", "CWE-1333"},
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		for _, pat := range jsPatterns {
			if pat.pattern.MatchString(line) {
				findings = append(findings, SecurityFinding{
					Severity:    pat.severity,
					Category:    "xss",
					Line:        i + 1,
					Description: fmt.Sprintf("Insecure pattern: %s", pat.name),
					Suggestion:  getJSFixSuggestion(pat.name),
					CWE:         pat.cwe,
				})
			}
		}
	}
	return findings
}

func getJSFixSuggestion(name string) string {
	switch name {
	case "eval":
		return "Avoid eval(). Use JSON.parse() for data, or Function constructor for dynamic code"
	case "innerHTML":
		return "Use textContent or a sanitization library (DOMPurify)"
	case "document_write":
		return "Use DOM manipulation methods instead"
	case "dangerouslySetInnerHTML":
		return "Sanitize HTML with DOMPurify before rendering"
	case "sql_concat":
		return "Use parameterized queries or prepared statements"
	case "regex_dos":
		return "Validate regex complexity or use re2-like engines"
	default:
		return "Review and fix the insecure pattern"
	}
}

// ── Report Formatting ──

func formatSecurityReport(path string, findings []SecurityFinding, totalLines int) string {
	if len(findings) == 0 {
		return fmt.Sprintf("Security Scan: %s — ✅ No issues found (%d lines)", path, totalLines)
	}

	criticalCount, highCount, mediumCount, lowCount := 0, 0, 0, 0
	for _, f := range findings {
		switch f.Severity {
		case "critical":
			criticalCount++
		case "high":
			highCount++
		case "medium":
			mediumCount++
		case "low":
			lowCount++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Security Scan: %s (%d lines)\n", path, totalLines))
	sb.WriteString(fmt.Sprintf("Found %d issues: 🔴 %d critical, 🟠 %d high, 🟡 %d medium, 🔵 %d low\n\n",
		len(findings), criticalCount, highCount, mediumCount, lowCount))

	// Sort by severity: critical first
	severityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3, "info": 4}
	// Simple insertion sort for small slices
	for i := 1; i < len(findings); i++ {
		for j := i; j > 0 && severityOrder[findings[j].Severity] < severityOrder[findings[j-1].Severity]; j-- {
			findings[j], findings[j-1] = findings[j-1], findings[j]
		}
	}

	for _, f := range findings {
		icon := "🔵"
		switch f.Severity {
		case "critical":
			icon = "🔴"
		case "high":
			icon = "🟠"
		case "medium":
			icon = "🟡"
		}
		sb.WriteString(fmt.Sprintf("%s [%s] Line %d: %s\n", icon, strings.ToUpper(f.Severity), f.Line, f.Description))
		if f.CWE != "" {
			sb.WriteString(fmt.Sprintf("   %s | %s\n", f.CWE, f.Suggestion))
		} else {
			sb.WriteString(fmt.Sprintf("   %s\n", f.Suggestion))
		}
	}

	return sb.String()
}
