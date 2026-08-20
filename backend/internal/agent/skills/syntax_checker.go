package skills

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"github.com/moduforge/backend/internal/agent/registry"
)

// SyntaxCheckerSkill performs pre-build syntax validation on source files.
// It catches errors before the full build, saving time and giving the agent
// immediate feedback to fix issues.
type SyntaxCheckerSkill struct {
	projectPath string
	db          *sql.DB
}

// SyntaxResult holds the result of a syntax check.
type SyntaxResult struct {
	Language string         `json:"language"`
	Passed   bool           `json:"passed"`
	Errors   []SyntaxError  `json:"errors"`
	Warnings []string       `json:"warnings"`
	Hints    []string       `json:"hints"` // agent-facing fix suggestions
}

// SyntaxError holds a single syntax error.
type SyntaxError struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
	Type    string `json:"type"` // "syntax", "undefined", "type_mismatch", "import", "missing_declaration"
}

func NewSyntaxCheckerSkill(projectPath string, db *sql.DB) *SyntaxCheckerSkill {
	return &SyntaxCheckerSkill{projectPath: projectPath, db: db}
}

func (s *SyntaxCheckerSkill) Name() string {
	return "syntax_checker"
}

func (s *SyntaxCheckerSkill) Description() string {
	return `Pre-build syntax validation for Go, Rust, C/C++ source files.
Input: {"project_id": "...", "language": "go|rust|cpp|auto"} or {}.
Runs lightweight syntax checks (go vet, cargo check, gcc -fsyntax-only) before full build.
Returns structured errors with line numbers and fix hints for the agent.`
}

func (s *SyntaxCheckerSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	projectID, _ := input["project_id"].(string)
	language, _ := input["language"].(string)
	projectPath := ResolveProjectPath(s.db, s.projectPath, projectID)

	var results []SyntaxResult

	if language == "" || language == "auto" {
		// Auto-detect and check all languages
		sources := s.detectLanguages(projectPath)
		if sources.hasGo {
			results = append(results, s.checkGo(ctx, projectPath))
		}
		if sources.hasCargo {
			results = append(results, s.checkRust(ctx, projectPath))
		}
		if sources.hasCpp {
			results = append(results, s.checkCpp(ctx, projectPath))
		}
	} else {
		switch language {
		case "go":
			results = append(results, s.checkGo(ctx, projectPath))
		case "rust":
			results = append(results, s.checkRust(ctx, projectPath))
		case "cpp", "c":
			results = append(results, s.checkCpp(ctx, projectPath))
		default:
			return "", fmt.Errorf("unsupported language: %s (use go, rust, cpp, or auto)", language)
		}
	}

	if len(results) == 0 {
		return "No source files found to check.", nil
	}

	return s.formatResults(results), nil
}

// detectLanguages scans the project directory for source files.
func (s *SyntaxCheckerSkill) detectLanguages(projectPath string) sourceInfo {
	var info sourceInfo
	_ = filepath.Walk(projectPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		name := fi.Name()
		ext := strings.ToLower(filepath.Ext(path))

		if !info.hasCargo && name == "Cargo.toml" {
			info.hasCargo = true
			info.cargoDir = filepath.Dir(path)
		}
		if !info.hasCpp {
			switch ext {
			case ".cpp", ".c", ".cc", ".cxx":
				info.hasCpp = true
			}
		}
		if !info.hasGo {
			if name == "go.mod" {
				info.hasGo = true
				info.goModDir = filepath.Dir(path)
			} else if strings.HasSuffix(path, ".go") {
				info.hasGo = true
			}
		}
		return nil
	})
	return info
}

// checkGo runs `go vet` and parses errors.
func (s *SyntaxCheckerSkill) checkGo(ctx context.Context, projectPath string) SyntaxResult {
	result := SyntaxResult{Language: "go", Hints: []string{}}

	goBin := findGoBinary()
	if goBin == "" {
		result.Warnings = append(result.Warnings, "go not found, skipping syntax check")
		return result
	}

	// Find go.mod directory
	goModDir := projectPath
	if !s.hasFile(projectPath, "go.mod") {
		filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || goModDir != projectPath {
				return nil
			}
			if info.Name() == "go.mod" {
				goModDir = filepath.Dir(path)
			}
			return nil
		})
	}

	// Phase 1: go vet (catches common errors)
	cmd := exec.CommandContext(ctx, goBin, "vet", "./...")
	cmd.Dir = goModDir
	cmd.Env = append(os.Environ(), "GOOS=android", "GOARCH=arm64", "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()

	if err != nil {
		errors := parseGoVetOutput(string(output))
		result.Errors = append(result.Errors, errors...)
	}

	// Phase 2: Parse compile errors from output
	compileErrors := parseGoCompileOutput(string(output))
	result.Errors = append(result.Errors, compileErrors...)

	// Deduplicate errors
	result.Errors = dedupSyntaxErrors(result.Errors)
	result.Passed = len(result.Errors) == 0

	// Generate fix hints
	for _, e := range result.Errors {
		hint := generateGoFixHint(e)
		if hint != "" {
			result.Hints = append(result.Hints, hint)
		}
	}

	return result
}

// checkRust runs `cargo check` and parses errors.
func (s *SyntaxCheckerSkill) checkRust(ctx context.Context, projectPath string) SyntaxResult {
	result := SyntaxResult{Language: "rust", Hints: []string{}}

	sources := s.detectLanguages(projectPath)
	cargoDir := sources.cargoDir
	if cargoDir == "" {
		cargoDir = projectPath
	}

	cargoPath := findExec("cargo")
	if cargoPath == "" {
		result.Warnings = append(result.Warnings, "cargo not found, skipping syntax check")
		return result
	}

	// cargo check is faster than cargo build, validates syntax and types
	cmd := exec.CommandContext(ctx, cargoPath, "check", "--message-format=short")
	cmd.Dir = cargoDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		errors := parseCargoCheckOutput(string(output))
		result.Errors = append(result.Errors, errors...)
	}

	result.Errors = dedupSyntaxErrors(result.Errors)
	result.Passed = len(result.Errors) == 0

	for _, e := range result.Errors {
		hint := generateRustFixHint(e)
		if hint != "" {
			result.Hints = append(result.Hints, hint)
		}
	}

	return result
}

// checkCpp runs gcc/g++ syntax-only check.
func (s *SyntaxCheckerSkill) checkCpp(ctx context.Context, projectPath string) SyntaxResult {
	result := SyntaxResult{Language: "cpp", Hints: []string{}}

	compiler := findCppCompiler()
	if compiler == "" {
		result.Warnings = append(result.Warnings, "no C++ compiler found, skipping syntax check")
		return result
	}

	var srcFiles []string
	filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".cpp" || ext == ".c" || ext == ".cc" || ext == ".cxx" {
			srcFiles = append(srcFiles, path)
		}
		return nil
	})

	if len(srcFiles) == 0 {
		return result
	}

	args := append([]string{"-std=c++17", "-fsyntax-only", "-Wall", "-Wextra"}, srcFiles...)
	cmd := exec.CommandContext(ctx, compiler, args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		errors := parseCppSyntaxOutput(string(output))
		result.Errors = append(result.Errors, errors...)
	}

	result.Errors = dedupSyntaxErrors(result.Errors)
	result.Passed = len(result.Errors) == 0

	for _, e := range result.Errors {
		hint := generateCppFixHint(e)
		if hint != "" {
			result.Hints = append(result.Hints, hint)
		}
	}

	return result
}

// formatResults formats all check results into a readable string.
func (s *SyntaxCheckerSkill) formatResults(results []SyntaxResult) string {
	var b strings.Builder
	totalErrors := 0
	totalWarnings := 0
	allPassed := true

	for _, r := range results {
		if r.Passed {
			b.WriteString(fmt.Sprintf("✅ %s: syntax check passed\n", strings.ToUpper(r.Language)))
		} else {
			allPassed = false
			b.WriteString(fmt.Sprintf("❌ %s: %d error(s) found\n", strings.ToUpper(r.Language), len(r.Errors)))
			for _, e := range r.Errors {
				loc := ""
				if e.File != "" {
					loc = fmt.Sprintf("%s:%d", e.File, e.Line)
					if e.Column > 0 {
						loc = fmt.Sprintf("%s:%d", loc, e.Column)
					}
				}
				if loc != "" {
					b.WriteString(fmt.Sprintf("  [%s] %s: %s\n", e.Type, loc, e.Message))
				} else {
					b.WriteString(fmt.Sprintf("  [%s] %s\n", e.Type, e.Message))
				}
			}
			totalErrors += len(r.Errors)
		}

		for _, w := range r.Warnings {
			b.WriteString(fmt.Sprintf("  ⚠️ %s\n", w))
			totalWarnings++
		}

		if len(r.Hints) > 0 {
			b.WriteString("  💡 Fix hints:\n")
			for _, h := range r.Hints {
				b.WriteString(fmt.Sprintf("    - %s\n", h))
			}
		}
	}

	if allPassed {
		b.WriteString("\n✅ All syntax checks passed. Ready for build.\n")
	} else {
		b.WriteString(fmt.Sprintf("\n⚠️ %d error(s), %d warning(s). Fix errors before building.\n", totalErrors, totalWarnings))
	}

	return b.String()
}

// ═══════════════════════════════════════════════════════════════════
// Go-specific parsing
// ═══════════════════════════════════════════════════════════════════

// parseGoVetOutput parses `go vet` output.
func parseGoVetOutput(output string) []SyntaxError {
	var errors []SyntaxError
	lines := strings.Split(output, "\n")

	goFilePattern := regexp.MustCompile(`^(.+\.go):(\d+):(\d+):\s*(.+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if m := goFilePattern.FindStringSubmatch(line); len(m) > 4 {
			errors = append(errors, SyntaxError{
				File:    m[1],
				Line:    atoi(m[2]),
				Column:  atoi(m[3]),
				Message: m[4],
				Type:    classifyGoSyntaxError(m[4]),
			})
		}
	}
	return errors
}

// parseGoCompileOutput parses general Go compiler output.
func parseGoCompileOutput(output string) []SyntaxError {
	var errors []SyntaxError
	lines := strings.Split(output, "\n")

	// Go error format: file:line:col: message
	goPattern := regexp.MustCompile(`(.+\.go):(\d+):(\d+):\s*(.+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if m := goPattern.FindStringSubmatch(line); len(m) > 4 {
			errors = append(errors, SyntaxError{
				File:    m[1],
				Line:    atoi(m[2]),
				Column:  atoi(m[3]),
				Message: m[4],
				Type:    classifyGoSyntaxError(m[4]),
			})
		}
	}
	return errors
}

// classifyGoSyntaxError categorizes a Go error message.
func classifyGoSyntaxError(msg string) string {
	msgLower := strings.ToLower(msg)
	switch {
	case strings.Contains(msgLower, "syntax error"):
		return "syntax"
	case strings.Contains(msgLower, "undefined") || strings.Contains(msgLower, "undeclared"):
		return "undefined"
	case strings.Contains(msgLower, "cannot use") || strings.Contains(msgLower, "type mismatch"):
		return "type_mismatch"
	case strings.Contains(msgLower, "cannot find package") || strings.Contains(msgLower, "no required module"):
		return "import"
	case strings.Contains(msgLower, "imported and not used"):
		return "import"
	case strings.Contains(msgLower, "missing return"):
		return "syntax"
	case strings.Contains(msgLower, "not enough arguments") || strings.Contains(msgLower, "too many arguments"):
		return "type_mismatch"
	case strings.Contains(msgLower, "cannot call") || strings.Contains(msgLower, "has no field") || strings.Contains(msgLower, "has no method"):
		return "type_mismatch"
	case strings.Contains(msgLower, "expected") || strings.Contains(msgLower, "unexpected"):
		return "syntax"
	}
	return "syntax"
}

// generateGoFixHint creates a specific fix hint for a Go error.
func generateGoFixHint(e SyntaxError) string {
	msgLower := strings.ToLower(e.Message)
	switch {
	case strings.Contains(msgLower, "undefined") || strings.Contains(msgLower, "undeclared"):
		return fmt.Sprintf("Check if '%s' is properly declared and imported. File: %s:%d", extractIdentifier(e.Message), e.File, e.Line)
	case strings.Contains(msgLower, "cannot find package"):
		return fmt.Sprintf("Run 'go mod tidy' or add the missing package to go.mod. File: %s", e.File)
	case strings.Contains(msgLower, "imported and not used"):
		return fmt.Sprintf("Remove unused import or use the imported package. File: %s:%d", e.File, e.Line)
	case strings.Contains(msgLower, "syntax error"):
		return fmt.Sprintf("Fix syntax at line %d of %s. Check brackets, semicolons, and Go syntax rules.", e.Line, e.File)
	case strings.Contains(msgLower, "cannot use"):
		return fmt.Sprintf("Type mismatch at %s:%d. Check function signatures and return types.", e.File, e.Line)
	case strings.Contains(msgLower, "not enough arguments") || strings.Contains(msgLower, "too many arguments"):
		return fmt.Sprintf("Wrong number of arguments at %s:%d. Check function signature.", e.File, e.Line)
	case strings.Contains(msgLower, "missing return"):
		return fmt.Sprintf("Add return statement to function ending at %s:%d.", e.File, e.Line)
	}
	return ""
}

// ═══════════════════════════════════════════════════════════════════
// Rust-specific parsing
// ═══════════════════════════════════════════════════════════════════

// parseCargoCheckOutput parses `cargo check --message-format=short` output.
func parseCargoCheckOutput(output string) []SyntaxError {
	var errors []SyntaxError
	lines := strings.Split(output, "\n")

	// Short format: file:line:col: error[E0XXX]: message
	shortPattern := regexp.MustCompile(`^(.+):(\d+):(\d+):\s*error\[(E\d+)\]:\s*(.+)`)
	errorPattern := regexp.MustCompile(`^(.+):(\d+):(\d+):\s*error:\s*(.+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if m := shortPattern.FindStringSubmatch(line); len(m) > 5 {
			errors = append(errors, SyntaxError{
				File:    m[1],
				Line:    atoi(m[2]),
				Column:  atoi(m[3]),
				Message: m[5],
				Type:    classifyRustErrorCode(m[4]),
			})
		} else if m := errorPattern.FindStringSubmatch(line); len(m) > 4 {
			errors = append(errors, SyntaxError{
				File:    m[1],
				Line:    atoi(m[2]),
				Column:  atoi(m[3]),
				Message: m[4],
				Type:    classifyRustErrorMsg(m[4]),
			})
		}
	}
	return errors
}

// classifyRustErrorCode maps Rust error codes to types.
func classifyRustErrorCode(code string) string {
	switch {
	case strings.HasPrefix(code, "E0"):
		if strings.Contains(code, "0412") || strings.Contains(code, "0432") || strings.Contains(code, "0433") {
			return "import"
		}
		if strings.Contains(code, "0596") || strings.Contains(code, "0599") || strings.Contains(code, "0609") || strings.Contains(code, "0603") {
			return "undefined"
		}
		if strings.Contains(code, "0308") || strings.Contains(code, "0305") || strings.Contains(code, "0382") {
			return "type_mismatch"
		}
		if strings.Contains(code, "0001") || strings.Contains(code, "0002") || strings.Contains(code, "0003") || strings.Contains(code, "0004") {
			return "syntax"
		}
		if strings.Contains(code, "0277") || strings.Contains(code, "0282") {
			return "type_mismatch"
		}
		if strings.Contains(code, "0583") || strings.Contains(code, "0584") {
			return "missing_declaration"
		}
	}
	return "syntax"
}

// classifyRustErrorMsg classifies Rust error messages without codes.
func classifyRustErrorMsg(msg string) string {
	msgLower := strings.ToLower(msg)
	switch {
	case strings.Contains(msgLower, "cannot find") || strings.Contains(msgLower, "unresolved import"):
		return "import"
	case strings.Contains(msgLower, "no field") || strings.Contains(msgLower, "no method") || strings.Contains(msgLower, "not found"):
		return "undefined"
	case strings.Contains(msgLower, "mismatched types") || strings.Contains(msgLower, "expected") && strings.Contains(msgLower, "found"):
		return "type_mismatch"
	case strings.Contains(msgLower, "syntax error") || strings.Contains(msgLower, "unexpected"):
		return "syntax"
	}
	return "syntax"
}

// generateRustFixHint creates a specific fix hint for a Rust error.
func generateRustFixHint(e SyntaxError) string {
	switch e.Type {
	case "import":
		return fmt.Sprintf("Fix import at %s:%d. Check if the crate is in Cargo.toml dependencies and the use path is correct.", e.File, e.Line)
	case "undefined":
		return fmt.Sprintf("Undefined identifier at %s:%d. Check variable/function declarations and imports.", e.File, e.Line)
	case "type_mismatch":
		return fmt.Sprintf("Type mismatch at %s:%d. Check expected vs actual types in this expression.", e.File, e.Line)
	case "syntax":
		return fmt.Sprintf("Syntax error at %s:%d. Check brackets, semicolons, and Rust syntax.", e.File, e.Line)
	case "missing_declaration":
		return fmt.Sprintf("Missing declaration at %s:%d. Check struct/enum/fn definitions.", e.File, e.Line)
	}
	return ""
}

// ═══════════════════════════════════════════════════════════════════
// C/C++-specific parsing
// ═══════════════════════════════════════════════════════════════════

// parseCppSyntaxOutput parses gcc/g++ -fsyntax-only output.
func parseCppSyntaxOutput(output string) []SyntaxError {
	var errors []SyntaxError
	lines := strings.Split(output, "\n")

	cppPattern := regexp.MustCompile(`^(.+):(\d+):(\d+):\s*(?:error|warning):\s*(.+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if m := cppPattern.FindStringSubmatch(line); len(m) > 4 {
			errType := "syntax"
			if strings.Contains(m[4], "undeclared") || strings.Contains(m[4], "was not declared") {
				errType = "undefined"
			} else if strings.Contains(m[4], "cannot convert") || strings.Contains(m[4], "no matching function") {
				errType = "type_mismatch"
			} else if strings.Contains(m[4], "fatal error: ") || strings.Contains(m[4], "No such file") {
				errType = "import"
			}
			errors = append(errors, SyntaxError{
				File:    m[1],
				Line:    atoi(m[2]),
				Column:  atoi(m[3]),
				Message: m[4],
				Type:    errType,
			})
		}
	}
	return errors
}

// generateCppFixHint creates a specific fix hint for a C++ error.
func generateCppFixHint(e SyntaxError) string {
	msgLower := strings.ToLower(e.Message)
	switch {
	case strings.Contains(msgLower, "undeclared") || strings.Contains(msgLower, "was not declared"):
		return fmt.Sprintf("Undefined identifier at %s:%d. Check variable/function declarations and #include directives.", e.File, e.Line)
	case strings.Contains(msgLower, "fatal error:") || strings.Contains(msgLower, "no such file"):
		return fmt.Sprintf("Missing header file at %s:%d. Check #include paths and file existence.", e.File, e.Line)
	case strings.Contains(msgLower, "expected") || strings.Contains(msgLower, "unexpected"):
		return fmt.Sprintf("Syntax error at %s:%d. Check brackets, semicolons, and C++ syntax.", e.File, e.Line)
	case strings.Contains(msgLower, "cannot convert") || strings.Contains(msgLower, "no matching function"):
		return fmt.Sprintf("Type mismatch at %s:%d. Check argument types and function signatures.", e.File, e.Line)
	case strings.Contains(msgLower, "undefined reference"):
		return fmt.Sprintf("Linker error at %s:%d. Check that all referenced functions are defined.", e.File, e.Line)
	}
	return ""
}

// ═══════════════════════════════════════════════════════════════════
// Utility functions
// ═══════════════════════════════════════════════════════════════════

// extractIdentifier tries to extract a Go identifier from an error message.
func extractIdentifier(msg string) string {
	// Common patterns: "undefined: foo", "undeclared name: foo"
	m := regexp.MustCompile(`(?:undefined|undeclared name|undefined identifier)[:\s]+(\w+)`)
	if matches := m.FindStringSubmatch(msg); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// dedupSyntaxErrors removes duplicate errors.
func dedupSyntaxErrors(errors []SyntaxError) []SyntaxError {
	seen := make(map[string]bool)
	var result []SyntaxError
	for _, e := range errors {
		key := fmt.Sprintf("%s:%d:%d:%s", e.File, e.Line, e.Column, e.Message)
		if !seen[key] {
			seen[key] = true
			result = append(result, e)
		}
	}
	return result
}

// findGoBinary finds the go executable.
func findGoBinary() string {
	for _, p := range []string{"/usr/local/go/bin/go", "go", "/usr/bin/go"} {
		if path, err := exec.LookPath(p); err == nil {
			return path
		}
	}
	return ""
}

func (s *SyntaxCheckerSkill) hasFile(projectPath, name string) bool {
	_, err := os.Stat(filepath.Join(projectPath, name))
	return err == nil
}

func (s *SyntaxCheckerSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: true,
		Core:      false,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
