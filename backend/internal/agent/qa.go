package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Severity levels for test issues.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityInfo    = "info"
)

// TestIssue represents a single problem found during testing.
type TestIssue struct {
	File        string `json:"file"`
	Line        int    `json:"line,omitempty"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// TestReport is the complete result of integration testing.
type TestReport struct {
	Passed  bool        `json:"passed"`
	Failed  bool        `json:"failed"`
	Issues  []TestIssue `json:"issues,omitempty"`
	Summary string      `json:"summary,omitempty"`
}

// QA runs integration tests on generated Magisk module code.
type QA struct {
	caller llmCaller
}

// NewQA creates a QA agent with resolved LLM configuration.
func NewQA() *QA {
	endpoint, apiKey, model := resolveAgentLLM()
	if endpoint == "" || apiKey == "" {
		return nil
	}
	return &QA{
		caller: &builderLLMCaller{
			endpoint: endpoint,
			apiKey:   apiKey,
			model:    model,
		},
	}
}

// RunIntegrationTest checks generated files against Magisk conventions.
func (qa *QA) RunIntegrationTest(ctx context.Context, design ModuleDesign, files []GeneratedFile) (TestReport, error) {
	report := TestReport{Passed: true}

	// Create temp directory for building
	tmpDir, err := os.MkdirTemp("", "qa-test-*")
	if err != nil {
		return report, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write all files to temp dir
	for _, f := range files {
		path := filepath.Join(tmpDir, f.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			continue
		}
		os.WriteFile(path, []byte(f.Content), 0644)
	}

	// ═══════════════════════════════════════════════════════
	// Check 1: module.prop completeness
	// ═══════════════════════════════════════════════════════
	qa.checkModuleProp(tmpDir, files, &report)

	// ═══════════════════════════════════════════════════════
	// Check 2: Shell script syntax
	// ═══════════════════════════════════════════════════════
	qa.checkShellSyntax(tmpDir, files, &report)

	// ═══════════════════════════════════════════════════════
	// Check 3: Go compilation
	// ═══════════════════════════════════════════════════════
	qa.checkGoCompilation(tmpDir, files, &report)

	// ═══════════════════════════════════════════════════════
	// Check 4: Magisk conventions
	// ═══════════════════════════════════════════════════════
	qa.checkMagiskConventions(files, &report)

	// ═══════════════════════════════════════════════════════
	// Check 5: set_perm usage
	// ═══════════════════════════════════════════════════════
	qa.checkSetPermUsage(files, &report)

	report.Failed = len(report.Issues) > 0
	report.Passed = !report.Failed

	// Generate summary
	if report.Passed {
		report.Summary = fmt.Sprintf("All %d checks passed.", len(files))
	} else {
		errCount, warnCount := 0, 0
		for _, issue := range report.Issues {
			if issue.Severity == SeverityError {
				errCount++
			} else {
				warnCount++
			}
		}
		report.Summary = fmt.Sprintf("Found %d error(s) and %d warning(s) in %d files.",
			errCount, warnCount, len(files))
	}

	return report, nil
}

// checkModuleProp verifies module.prop has required fields.
func (qa *QA) checkModuleProp(tmpDir string, files []GeneratedFile, report *TestReport) {
	var propContent string
	for _, f := range files {
		if f.Path == "module.prop" {
			propContent = f.Content
			break
		}
	}

	if propContent == "" {
		report.Issues = append(report.Issues, TestIssue{
			File:        "module.prop",
			Severity:    SeverityError,
			Description: "module.prop file is missing",
			Suggestion:  "Add module.prop with required fields: id, name, version, versionCode, author, description",
		})
		return
	}

	requiredFields := []string{"id=", "name=", "version=", "versionCode=", "author=", "description="}
	for _, field := range requiredFields {
		if !strings.Contains(propContent, field) {
			report.Issues = append(report.Issues, TestIssue{
				File:        "module.prop",
				Severity:    SeverityError,
				Description: fmt.Sprintf("module.prop missing required field: %s", strings.TrimSuffix(field, "=")),
				Suggestion:  fmt.Sprintf("Add %s value to module.prop", field),
			})
		}
	}

	// Check for quotes around values (common mistake)
	lines := strings.Split(propContent, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "=\"") || strings.Contains(line, "='") {
			report.Issues = append(report.Issues, TestIssue{
				File:        "module.prop",
				Line:        i + 1,
				Severity:    SeverityWarning,
				Description: "module.prop values should not be quoted",
				Suggestion:  "Use key=value, not key=\"value\"",
			})
		}
	}
}

// checkShellSyntax runs bash -n on shell scripts.
func (qa *QA) checkShellSyntax(tmpDir string, files []GeneratedFile, report *TestReport) {
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".sh") {
			continue
		}

		scriptPath := filepath.Join(tmpDir, f.Path)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "bash", "-n", scriptPath)
		stderr, err := cmd.CombinedOutput()
		if err != nil {
			report.Issues = append(report.Issues, TestIssue{
				File:        f.Path,
				Severity:    SeverityError,
				Description: fmt.Sprintf("Shell syntax error: %s", strings.TrimSpace(string(stderr))),
				Suggestion:  "Fix shell syntax errors",
			})
		}
	}
}

// checkGoCompilation runs go build on Go files.
func (qa *QA) checkGoCompilation(tmpDir string, files []GeneratedFile, report *TestReport) {
	hasGo := false
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".go") {
			hasGo = true
			break
		}
	}
	if !hasGo {
		return
	}

	// Find go binary
	goPath, err := exec.LookPath("go")
	if err != nil {
		for _, p := range []string{"/usr/local/go/bin/go", "/usr/bin/go"} {
			if _, statErr := os.Stat(p); statErr == nil {
				goPath = p
				break
			}
		}
	}
	if goPath == "" {
		report.Issues = append(report.Issues, TestIssue{
			Severity:    SeverityWarning,
			Description: "Go compiler not found, skipping Go compilation check",
		})
		return
	}

	// Check if go.mod exists
	goModExists := false
	for _, f := range files {
		if f.Path == "go.mod" {
			goModExists = true
			break
		}
	}
	if !goModExists {
		report.Issues = append(report.Issues, TestIssue{
			Severity:    SeverityWarning,
			Description: "No go.mod found, Go compilation may fail",
			Suggestion:  "Add go.mod with module path and Go version",
		})
		return
	}

	// Try to build
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, goPath, "build", "-o", "/dev/null", ".")
	cmd.Dir = tmpDir
	stderr, err := cmd.CombinedOutput()
	if err != nil {
		// Parse errors
		errLines := strings.Split(string(stderr), "\n")
		for _, line := range errLines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Try to extract file:line:col format
			re := regexp.MustCompile(`(?:\.\/)?([^:]+\.go):(\d+):\d+:\s*(.+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 4 {
				lineNum := 0
				fmt.Sscanf(matches[2], "%d", &lineNum)
				report.Issues = append(report.Issues, TestIssue{
					File:        matches[1],
					Line:        lineNum,
					Severity:    SeverityError,
					Description: matches[3],
					Suggestion:  "Fix Go compilation error",
				})
			} else {
				report.Issues = append(report.Issues, TestIssue{
					Severity:    SeverityError,
					Description: fmt.Sprintf("Go build error: %s", line),
					Suggestion:  "Fix Go compilation error",
				})
			}
		}
	}
}

// checkMagiskConventions checks for Magisk-specific patterns.
func (qa *QA) checkMagiskConventions(files []GeneratedFile, report *TestReport) {
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".sh") {
			continue
		}

		content := f.Content

		// Check shebang
		if !strings.HasPrefix(strings.TrimSpace(content), "#!/system/bin/sh") {
			report.Issues = append(report.Issues, TestIssue{
				File:        f.Path,
				Severity:    SeverityWarning,
				Description: "Shell script should start with #!/system/bin/sh",
				Suggestion:  "Add shebang: #!/system/bin/sh",
			})
		}

		// Check for bare $VAR (not ${VAR})
		re := regexp.MustCompile(`\$[A-Za-z_][A-Za-z0-9_]*[^{(]`)
		matches := re.FindAllString(content, -1)
		for _, m := range matches {
			// Skip $() command substitution and $$ PID
			if strings.HasPrefix(m, "$(") || strings.HasPrefix(m, "$$") {
				continue
			}
			report.Issues = append(report.Issues, TestIssue{
				File:        f.Path,
				Severity:    SeverityError,
				Description: fmt.Sprintf("Use ${VAR} instead of bare $VAR: %s", m),
				Suggestion:  "Replace $VAR with ${VAR} for reliability",
			})
			break // One issue per file is enough
		}

		// Check for dangerous rm
		if strings.Contains(content, "rm -rf /") || strings.Contains(content, "rm -rf /*") {
			report.Issues = append(report.Issues, TestIssue{
				File:        f.Path,
				Severity:    SeverityError,
				Description: "Dangerous rm -rf / detected",
				Suggestion:  "Only remove module-specific files",
			})
		}
	}
}

// checkSetPermUsage checks that binaries have proper permissions.
func (qa *QA) checkSetPermUsage(files []GeneratedFile, report *TestReport) {
	hasGo := false
	hasCustomize := false
	hasSetPerm := false

	for _, f := range files {
		if strings.HasSuffix(f.Path, ".go") {
			hasGo = true
		}
		if f.Path == "customize.sh" {
			hasCustomize = true
			if strings.Contains(f.Content, "set_perm") {
				hasSetPerm = true
			}
		}
	}

	if hasGo && hasCustomize && !hasSetPerm {
		report.Issues = append(report.Issues, TestIssue{
			File:        "customize.sh",
			Severity:    SeverityError,
			Description: "Module has Go binaries but customize.sh does not call set_perm",
			Suggestion:  "Add: set_perm ${MODPATH}/system/bin/<binary> 0 0 0755",
		})
	}
}
