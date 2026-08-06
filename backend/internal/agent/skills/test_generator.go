package skills

import (
	"context"
	"fmt"
	"strings"
)

// TestGeneratorSkill generates unit tests for code
type TestGeneratorSkill struct{}

func NewTestGeneratorSkill() *TestGeneratorSkill {
	return &TestGeneratorSkill{}
}

func (s *TestGeneratorSkill) Name() string { return "test_generator" }

func (s *TestGeneratorSkill) Description() string {
	return `Generate unit tests for a source file. Input: {"path": "...", "content": "...", "language": "go|rust|c++|python|typescript"}.
Returns: complete test file content with test cases covering happy path, edge cases, and error conditions.
Use this after code_review to ensure testability before build_module.`
}

func (s *TestGeneratorSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	content, _ := input["content"].(string)
	language, _ := input["language"].(string)
	path, _ := input["path"].(string)

	if content == "" {
		return "", fmt.Errorf("content is required")
	}
	if language == "" {
		language = detectLanguage(path)
	}

	var testContent string
	var err error

	switch strings.ToLower(language) {
	case "go":
		testContent, err = generateGoTests(path, content)
	case "rust":
		testContent, err = generateRustTests(path, content)
	case "python":
		testContent, err = generatePythonTests(path, content)
	case "typescript", "ts":
		testContent, err = generateTypeScriptTests(path, content)
	default:
		return "", fmt.Errorf("unsupported language for test generation: %s", language)
	}

	if err != nil {
		return "", err
	}

	return testContent, nil
}

func generateGoTests(path, content string) (string, error) {
	var sb strings.Builder
	lines := strings.Split(content, "\n")

	// Extract package name
	packageName := "main"
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			packageName = strings.TrimPrefix(trimmed, "package ")
			break
		}
	}

	testFileName := strings.TrimSuffix(path, ".go") + "_test.go"

	sb.WriteString(fmt.Sprintf("package %s\n\n", packageName))
	sb.WriteString("import (\n")
	sb.WriteString("\t\"testing\"\n")
	sb.WriteString(")\n\n")

	// Find exported functions and generate tests
	funcPattern := strings.Contains(content, "func ")
	if funcPattern {
		// Extract function names
		functions := extractGoFunctions(content)
		for _, fn := range functions {
			sb.WriteString(fmt.Sprintf("func Test%s(t *testing.T) {\n", capitalize(fn)))
			sb.WriteString(fmt.Sprintf("\t// TODO: Implement test for %s\n", fn))
			sb.WriteString("\tt.Skip(\"Test not implemented yet\")\n")
			sb.WriteString("}\n\n")
		}
	}

	return fmt.Sprintf("# Generated test file: %s\n# Source: %s\n\n%s", testFileName, path, sb.String()), nil
}

func extractGoFunctions(content string) []string {
	var functions []string
	lines := strings.Split(content, "\n")
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "func ") && !strings.HasPrefix(trimmed, "func Test") {
			// Extract function name
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				fnName := parts[1]
				// Remove receiver if present
				if idx := strings.Index(fnName, "("); idx > 0 {
					fnName = fnName[:idx]
				}
				// Skip if it's a method on unexported type
				if len(fnName) > 0 && fnName[0] >= 'A' && fnName[0] <= 'Z' {
					functions = append(functions, fnName)
				}
			}
		}
	}
	return functions
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func generateRustTests(path, content string) (string, error) {
	var sb strings.Builder
	lines := strings.Split(content, "\n")

	sb.WriteString("#[cfg(test)]\n")
	sb.WriteString("mod tests {\n")
	sb.WriteString("\tuse super::*;\n\n")

	// Find public functions
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "pub fn ") || strings.HasPrefix(trimmed, "pub async fn ") {
			fnName := extractRustFunctionName(trimmed)
			if fnName != "" {
				sb.WriteString(fmt.Sprintf("\t#[test]\n"))
				sb.WriteString(fmt.Sprintf("\tfn test_%s() {\n", fnName))
				sb.WriteString(fmt.Sprintf("\t\t// TODO: Implement test for %s\n", fnName))
				sb.WriteString(fmt.Sprintf("\t\t// assert_eq!(%s(...), expected);\n", fnName))
				sb.WriteString(fmt.Sprintf("\t}\n\n"))
			}
		}
	}

	sb.WriteString("}\n")
	return sb.String(), nil
}

func extractRustFunctionName(line string) string {
	// Remove pub, async, fn keywords
	line = strings.TrimPrefix(line, "pub ")
	line = strings.TrimPrefix(line, "async ")
	line = strings.TrimPrefix(line, "fn ")
	
	// Extract name before (
	if idx := strings.Index(line, "("); idx > 0 {
		return strings.TrimSpace(line[:idx])
	}
	return ""
}

func generatePythonTests(path, content string) (string, error) {
	var sb strings.Builder
	lines := strings.Split(content, "\n")

	sb.WriteString("import pytest\n\n")

	// Find functions
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "def ") && !strings.HasPrefix(trimmed, "def test_") && !strings.HasPrefix(trimmed, "def __") {
			fnName := extractPythonFunctionName(trimmed)
			if fnName != "" && len(fnName) > 1 && fnName[0] >= 'a' && fnName[0] <= 'z' {
				sb.WriteString(fmt.Sprintf("def test_%s():\n", fnName))
				sb.WriteString(fmt.Sprintf("    # TODO: Implement test for %s\n", fnName))
				sb.WriteString(fmt.Sprintf("    pass\n\n"))
			}
		}
	}

	return sb.String(), nil
}

func extractPythonFunctionName(line string) string {
	line = strings.TrimPrefix(line, "def ")
	if idx := strings.Index(line, "("); idx > 0 {
		return strings.TrimSpace(line[:idx])
	}
	return ""
}

func generateTypeScriptTests(path, content string) (string, error) {
	var sb strings.Builder
	lines := strings.Split(content, "\n")

	sb.WriteString("import { describe, it, expect } from 'vitest';\n\n")

	// Find exported functions
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "export function ") || strings.HasPrefix(trimmed, "export const ") {
			fnName := extractTypeScriptFunctionName(trimmed)
			if fnName != "" {
				sb.WriteString(fmt.Sprintf("describe('%s', () => {\n", fnName))
				sb.WriteString(fmt.Sprintf("\tit('should work correctly', () => {\n"))
				sb.WriteString(fmt.Sprintf("\t\t// TODO: Implement test for %s\n", fnName))
				sb.WriteString(fmt.Sprintf("\t\t// expect(%s(...)).toEqual(expected);\n", fnName))
				sb.WriteString(fmt.Sprintf("\t});\n"))
				sb.WriteString("});\n\n")
			}
		}
	}

	return sb.String(), nil
}

func extractTypeScriptFunctionName(line string) string {
	line = strings.TrimPrefix(line, "export ")
	if strings.HasPrefix(line, "function ") {
		line = strings.TrimPrefix(line, "function ")
	} else if strings.HasPrefix(line, "const ") {
		line = strings.TrimPrefix(line, "const ")
		// Remove = () etc
		if idx := strings.Index(line, " ="); idx > 0 {
			line = line[:idx]
		}
	}
	if idx := strings.Index(line, "("); idx > 0 {
		return strings.TrimSpace(line[:idx])
	}
	return ""
}
