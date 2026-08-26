package agent

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// TestGenerator generates unit tests for generated code.
type TestGenerator struct {
	mu     sync.Mutex
	runner *AgentRunner
}

// FileTestGenResult represents the result of file-level test generation.
type FileTestGenResult struct {
	File     string
	TestFile string
	Tests    []TestGenCase
	Coverage float64
	Errors   []string
}

// TestCase represents a single test case.
type TestGenCase struct {
	Name     string
	Function string
	Input    string
	Expected string
	Pass     bool
}

// FunctionInfo holds extracted function metadata.
type FunctionInfo struct {
	Name      string   `json:"name"`
	Params    []string `json:"params"`
	Returns   []string `json:"returns"`
	StartLine int      `json:"start_line"`
	EndLine   int      `json:"end_line"`
}

// TestGenResult is the result of code-level test generation.
type TestGenResult struct {
	Code     string         `json:"code"`
	Coverage float64        `json:"coverage"`
	Funcs    []FunctionInfo `json:"funcs"`
}

// NewTestGenerator creates a new test generator.
func NewTestGenerator(runner *AgentRunner) *TestGenerator {
	return &TestGenerator{runner: runner}
}

// GenerateTestsForFile generates unit tests for a Go file.
func (tg *TestGenerator) GenerateTestsForFile(filePath string, content string) (*FileTestGenResult, error) {
	tg.mu.Lock()
	defer tg.mu.Unlock()

	// Extract functions from content
	functions := extractFunctions(content)
	if len(functions) == 0 {
		return nil, fmt.Errorf("no functions found in %s", filePath)
	}

	// Generate test cases for each function
	var testCases []TestGenCase
	for _, fn := range functions {
		cases := tg.generateTestCases(fn, content)
		testCases = append(testCases, cases...)
	}

	// Generate test file content
	testPath := strings.TrimSuffix(filePath, ".go") + "_test.go"

	return &FileTestGenResult{
		File:     filePath,
		TestFile: testPath,
		Tests:    testCases,
		Coverage: estimateCoverage(testCases),
	}, nil
}

// ExtractFunctions extracts function information from code based on language.
func (tg *TestGenerator) ExtractFunctions(code string, language string) ([]FunctionInfo, error) {
	tg.mu.Lock()
	defer tg.mu.Unlock()

	switch strings.ToLower(language) {
	case "go", "golang":
		return tg.extractGoFunctions(code)
	case "javascript", "js", "typescript", "ts":
		return tg.extractJSFunctions(code)
	case "python", "py":
		return tg.extractPythonFunctions(code)
	default:
		return tg.extractGoFunctions(code) // fallback to Go-style parsing
	}
}

// extractGoFunctions extracts function info from Go source code.
func (tg *TestGenerator) extractGoFunctions(code string) ([]FunctionInfo, error) {
	var funcs []FunctionInfo
	lines := strings.Split(code, "\n")

	// Match: func FuncName( or func (r *Type) MethodName(
	goFuncRe := regexp.MustCompile(`^func\s+(?:\([^)]+\)\s+)?(\w+)\s*\(`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "func ") || strings.HasPrefix(trimmed, "func Test") {
			continue
		}

		matches := goFuncRe.FindStringSubmatch(trimmed)
		if matches == nil {
			continue
		}

		fi := FunctionInfo{
			Name:      matches[1],
			StartLine: i + 1,
		}

		// Extract parameters
		if idx := strings.Index(trimmed, "("); idx >= 0 {
			if endIdx := tg.findMatchingParen(trimmed, idx); endIdx > idx {
				paramStr := trimmed[idx+1 : endIdx]
				fi.Params = tg.parseParams(paramStr)
			}
		}

		// Extract return types
		if idx := strings.Index(trimmed, ")"); idx >= 0 {
			rest := strings.TrimSpace(trimmed[idx+1:])
			if rest != "" && !strings.HasPrefix(rest, "{") && rest != "" {
				fi.Returns = []string{rest}
			}
		}

		// Find end line (next func or end of file)
		fi.EndLine = len(lines)
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "func " || goFuncRe.MatchString(strings.TrimSpace(lines[j])) {
				fi.EndLine = j
				break
			}
		}

		funcs = append(funcs, fi)
	}

	return funcs, nil
}

// extractJSFunctions extracts function info from JavaScript/TypeScript source code.
func (tg *TestGenerator) extractJSFunctions(code string) ([]FunctionInfo, error) {
	var funcs []FunctionInfo
	lines := strings.Split(code, "\n")

	// Match: function name(, const name = (, name(, async function name(
	jsFuncRe := regexp.MustCompile(`(?:^|\s)(?:async\s+)?function\s+(\w+)\s*\(|(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\(|(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?(?:\([^)]*\)|[\w]+)\s*=>`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		matches := jsFuncRe.FindStringSubmatch(trimmed)
		if matches == nil {
			continue
		}

		name := matches[1]
		if name == "" {
			name = matches[2]
		}
		if name == "" {
			name = matches[3]
		}
		if name == "" {
			continue
		}

		fi := FunctionInfo{
			Name:      name,
			StartLine: i + 1,
			EndLine:   len(lines),
		}

		// Extract parameters
		if idx := strings.Index(trimmed, "("); idx >= 0 {
			if endIdx := tg.findMatchingParen(trimmed, idx); endIdx > idx {
				paramStr := trimmed[idx+1 : endIdx]
				fi.Params = tg.parseParams(paramStr)
			}
		}

		funcs = append(funcs, fi)
	}

	return funcs, nil
}

// extractPythonFunctions extracts function info from Python source code.
func (tg *TestGenerator) extractPythonFunctions(code string) ([]FunctionInfo, error) {
	var funcs []FunctionInfo
	lines := strings.Split(code, "\n")

	pyFuncRe := regexp.MustCompile(`^\s*def\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*(\S+))?`)

	for i, line := range lines {
		matches := pyFuncRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		fi := FunctionInfo{
			Name:      matches[1],
			StartLine: i + 1,
			EndLine:   len(lines),
		}

		if matches[2] != "" {
			fi.Params = tg.parseParams(matches[2])
		}
		if matches[3] != "" {
			fi.Returns = []string{matches[3]}
		}

		// Find end line based on indentation
		baseIndent := len(line) - len(strings.TrimLeft(line, " \t"))
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "" {
				continue
			}
			indent := len(lines[j]) - len(strings.TrimLeft(lines[j], " \t"))
			if indent <= baseIndent && strings.TrimSpace(lines[j]) != "" {
				fi.EndLine = j
				break
			}
		}

		funcs = append(funcs, fi)
	}

	return funcs, nil
}

// GenerateTests generates test code for extracted functions.
func (tg *TestGenerator) GenerateTests(code string, language string, funcs []FunctionInfo) (string, error) {
	if len(funcs) == 0 {
		return "", fmt.Errorf("no functions to test")
	}

	switch strings.ToLower(language) {
	case "go", "golang":
		return tg.generateGoTests(code, funcs)
	case "javascript", "js", "typescript", "ts":
		return tg.generateJSTests(code, funcs)
	case "python", "py":
		return tg.generatePythonTests(code, funcs)
	default:
		return tg.generateGoTests(code, funcs)
	}
}

// generateGoTests generates Go test code.
func (tg *TestGenerator) generateGoTests(code string, funcs []FunctionInfo) (string, error) {
	var sb strings.Builder
	sb.WriteString("package main\n\n")
	sb.WriteString("import \"testing\"\n\n")

	for _, fn := range funcs {
		c := cases.Title(language.English)
		testName := "Test" + c.String(fn.Name)
		sb.WriteString(fmt.Sprintf("func %s(t *testing.T) {\n", testName))
		sb.WriteString(fmt.Sprintf("\t// TODO: implement test for %s\n", fn.Name))
		sb.WriteString(fmt.Sprintf("\tt.Run(\"%s\", func(t *testing.T) {\n", fn.Name))
		sb.WriteString("\t\t// test body\n")
		sb.WriteString("\t})\n")
		sb.WriteString("}\n\n")
	}

	return sb.String(), nil
}

// generateJSTests generates JavaScript/TypeScript test code.
func (tg *TestGenerator) generateJSTests(code string, funcs []FunctionInfo) (string, error) {
	var sb strings.Builder
	sb.WriteString("describe('module', () => {\n")

	for _, fn := range funcs {
		sb.WriteString(fmt.Sprintf("  describe('%s', () => {\n", fn.Name))
		sb.WriteString("    it('should work correctly', () => {\n")
		sb.WriteString(fmt.Sprintf("      // TODO: implement test for %s\n", fn.Name))
		sb.WriteString("    });\n")
		sb.WriteString("  });\n\n")
	}

	sb.WriteString("});\n")
	return sb.String(), nil
}

// generatePythonTests generates Python test code.
func (tg *TestGenerator) generatePythonTests(code string, funcs []FunctionInfo) (string, error) {
	var sb strings.Builder
	sb.WriteString("import pytest\n\n")

	for _, fn := range funcs {
		testName := "test_" + fn.Name
		sb.WriteString(fmt.Sprintf("def %s():\n", testName))
		sb.WriteString(fmt.Sprintf("    # TODO: implement test for %s\n", fn.Name))
		sb.WriteString("    pass\n\n")
	}

	return sb.String(), nil
}

// EstimateCoverage estimates test coverage percentage.
func (tg *TestGenerator) EstimateCoverage(funcs []FunctionInfo, testCode string) float64 {
	if len(funcs) == 0 || testCode == "" {
		return 0
	}

	tested := 0
	for _, fn := range funcs {
		if strings.Contains(testCode, fn.Name) {
			tested++
		}
	}

	return float64(tested) / float64(len(funcs)) * 100
}

// findMatchingParen finds the index of the matching closing parenthesis.
func (tg *TestGenerator) findMatchingParen(s string, start int) int {
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(s)
}

// parseParams parses a parameter string into a slice of parameter names.
func (tg *TestGenerator) parseParams(paramStr string) []string {
	paramStr = strings.TrimSpace(paramStr)
	if paramStr == "" {
		return nil
	}

	var params []string
	for _, part := range strings.Split(paramStr, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Remove type annotation
		if idx := strings.Index(part, " "); idx > 0 {
			part = strings.TrimSpace(part[:idx])
		}
		// Remove default value
		if idx := strings.Index(part, "="); idx > 0 {
			part = strings.TrimSpace(part[:idx])
		}
		if part != "" && part != "_" {
			params = append(params, part)
		}
	}
	return params
}

// extractFunctions extracts function signatures from Go code.
func extractFunctions(content string) []string {
	var funcs []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Match: func FuncName( or func (r *Type) MethodName(
		if strings.HasPrefix(line, "func ") && !strings.HasPrefix(line, "func Test") {
			funcs = append(funcs, line)
		}
	}
	return funcs
}

// generateTestCases generates test cases for a function.
func (tg *TestGenerator) generateTestCases(funcSig string, content string) []TestGenCase {
	// Extract function name
	name := extractFuncName(funcSig)

	// Generate basic test cases
	return []TestGenCase{
		{
			Name:     fmt.Sprintf("Test%s_Basic", name),
			Function: name,
			Input:    "// TODO: Add test input",
			Expected: "// TODO: Add expected output",
			Pass:     false,
		},
	}
}

// extractFuncName extracts function name from signature.
func extractFuncName(sig string) string {
	// Remove "func " prefix
	sig = strings.TrimPrefix(sig, "func ")

	// Handle method receiver
	if idx := strings.Index(sig, ")"); idx > 0 {
		sig = sig[idx+1:]
	}

	// Extract name before "("
	if idx := strings.Index(sig, "("); idx > 0 {
		return strings.TrimSpace(sig[:idx])
	}
	return sig
}

// generateTestFile generates the content of a test file.
func generateTestFile(filePath string, cases []TestGenCase) string {
	pkg := filepath.Base(filepath.Dir(filePath))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("package %s\n\n", pkg))
	sb.WriteString("import \"testing\"\n\n")

	for _, tc := range cases {
		sb.WriteString(fmt.Sprintf("func %s(t *testing.T) {\n", tc.Name))
		sb.WriteString("\t// TODO: Implement test\n")
		sb.WriteString(fmt.Sprintf("\tt.Error(\"Test %s not implemented\")\n", tc.Name))
		sb.WriteString("}\n\n")
	}

	return sb.String()
}

// estimateCoverage estimates test coverage based on test cases.
func estimateCoverage(cases []TestGenCase) float64 {
	if len(cases) == 0 {
		return 0
	}
	passed := 0
	for _, c := range cases {
		if c.Pass {
			passed++
		}
	}
	return float64(passed) / float64(len(cases)) * 100
}
