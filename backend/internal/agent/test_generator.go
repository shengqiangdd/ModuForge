package agent

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// TestGenerator generates unit tests for generated code.
type TestGenerator struct {
	mu sync.Mutex
}

// TestResult represents the result of test generation.
type TestResult struct {
	File     string
	TestFile string
	Tests    []TestCase
	Coverage float64
	Errors   []string
}

// TestCase represents a single test case.
type TestCase struct {
	Name     string
	Function string
	Input    string
	Expected string
	Pass     bool
}

// NewTestGenerator creates a new test generator.
func NewTestGenerator() *TestGenerator {
	return &TestGenerator{}
}

// GenerateTests generates unit tests for a Go file.
func (tg *TestGenerator) GenerateTests(filePath string, content string) (*TestResult, error) {
	tg.mu.Lock()
	defer tg.mu.Unlock()

	// Extract functions from content
	functions := extractFunctions(content)
	if len(functions) == 0 {
		return nil, fmt.Errorf("no functions found in %s", filePath)
	}

	// Generate test cases for each function
	var testCases []TestCase
	for _, fn := range functions {
		cases := tg.generateTestCases(fn, content)
		testCases = append(testCases, cases...)
	}

	// Generate test file content
	testPath := strings.TrimSuffix(filePath, ".go") + "_test.go"

	return &TestResult{
		File:     filePath,
		TestFile: testPath,
		Tests:    testCases,
		Coverage: estimateCoverage(testCases),
	}, nil
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
func (tg *TestGenerator) generateTestCases(funcSig string, content string) []TestCase {
	// Extract function name
	name := extractFuncName(funcSig)

	// Generate basic test cases
	return []TestCase{
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
func generateTestFile(filePath string, cases []TestCase) string {
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
func estimateCoverage(cases []TestCase) float64 {
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
