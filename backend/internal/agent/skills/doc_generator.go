package skills

import (
	"context"
	"fmt"
	"strings"
)

// DocGeneratorSkill generates documentation for code
type DocGeneratorSkill struct{}

func NewDocGeneratorSkill() *DocGeneratorSkill {
	return &DocGeneratorSkill{}
}

func (s *DocGeneratorSkill) Name() string { return "doc_generator" }

func (s *DocGeneratorSkill) Description() string {
	return `Generate documentation for a source file. Input: {"path": "...", "content": "...", "language": "go|rust|c++|python|typescript"}.
Returns: README.md or doc comments with API documentation, usage examples, and architecture overview.
Use this after code_review and test_generation to complete the module documentation.`
}

func (s *DocGeneratorSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	content, _ := input["content"].(string)
	language, _ := input["language"].(string)
	path, _ := input["path"].(string)

	if content == "" {
		return "", fmt.Errorf("content is required")
	}
	if language == "" {
		language = detectLanguage(path)
	}

	var docContent string

	switch strings.ToLower(language) {
	case "go":
		docContent = generateGoDocs(path, content)
	case "rust":
		docContent = generateRustDocs(path, content)
	case "python":
		docContent = generatePythonDocs(path, content)
	case "typescript", "ts":
		docContent = generateTypeScriptDocs(path, content)
	default:
		docContent = generateGenericDocs(path, content)
	}

	return docContent, nil
}

func generateGoDocs(path, content string) string {
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

	sb.WriteString(fmt.Sprintf("# %s Package\n\n", capitalizeFirst(packageName)))
	sb.WriteString(fmt.Sprintf("**File:** `%s`\n\n", path))

	// Package documentation
	sb.WriteString("## Overview\n\n")
	sb.WriteString(fmt.Sprintf("The `%s` package provides...\n\n", packageName))

	// Extract exported types and functions
	sb.WriteString("## Types\n\n")
	sb.WriteString("| Type | Description |\n")
	sb.WriteString("|------|-------------|\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "type ") && strings.Contains(trimmed, "struct") {
			typeName := extractTypeName(trimmed, "struct")
			if typeName != "" && len(typeName) > 0 && typeName[0] >= 'A' && typeName[0] <= 'Z' {
				sb.WriteString(fmt.Sprintf("| `%s` | ... |\n", typeName))
			}
		}
		if strings.HasPrefix(trimmed, "type ") && strings.Contains(trimmed, "interface") {
			typeName := extractTypeName(trimmed, "interface")
			if typeName != "" && len(typeName) > 0 && typeName[0] >= 'A' && typeName[0] <= 'Z' {
				sb.WriteString(fmt.Sprintf("| `%s` | ... |\n", typeName))
			}
		}
	}

	sb.WriteString("\n## Functions\n\n")
	sb.WriteString("| Function | Description |\n")
	sb.WriteString("|----------|-------------|\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "func ") {
			fnName := extractGoFunctionName(trimmed)
			if fnName != "" && len(fnName) > 0 && fnName[0] >= 'A' && fnName[0] <= 'Z' {
				sb.WriteString(fmt.Sprintf("| `%s` | ... |\n", fnName))
			}
		}
	}

	sb.WriteString("\n## Usage\n\n")
	sb.WriteString("```go\n")
	sb.WriteString(fmt.Sprintf("import \"%s\"\n\n", packageName))
	sb.WriteString("// Example usage\n")
	sb.WriteString("```\n")

	return sb.String()
}

func extractTypeName(line, kind string) string {
	// type MyStruct struct
	line = strings.TrimPrefix(line, "type ")
	if idx := strings.Index(line, " "+kind); idx > 0 {
		return strings.TrimSpace(line[:idx])
	}
	return ""
}

func extractGoFunctionName(line string) string {
	// func MyFunc() or func (r *Receiver) MyFunc()
	line = strings.TrimPrefix(line, "func ")
	
	// Skip receiver
	if strings.HasPrefix(line, "(") {
		if idx := strings.Index(line, ")"); idx > 0 {
			line = strings.TrimSpace(line[idx+1:])
		}
	}
	
	// Extract name before (
	if idx := strings.Index(line, "("); idx > 0 {
		return strings.TrimSpace(line[:idx])
	}
	return ""
}

func generateRustDocs(path, content string) string {
	var sb strings.Builder
	lines := strings.Split(content, "\n")

	sb.WriteString("# Module Documentation\n\n")
	sb.WriteString(fmt.Sprintf("**File:** `%s`\n\n", path))

	sb.WriteString("## Overview\n\n")
	sb.WriteString("This module provides...\n\n")

	sb.WriteString("## Public API\n\n")
	sb.WriteString("| Item | Type | Description |\n")
	sb.WriteString("|------|------|-------------|\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "pub struct ") {
			structName := strings.TrimPrefix(trimmed, "pub struct ")
			if idx := strings.Index(structName, "{"); idx > 0 {
				structName = strings.TrimSpace(structName[:idx])
			}
			if idx := strings.Index(structName, "("); idx > 0 {
				structName = strings.TrimSpace(structName[:idx])
			}
			sb.WriteString(fmt.Sprintf("| `%s` | struct | ... |\n", structName))
		}
		if strings.HasPrefix(trimmed, "pub enum ") {
			enumName := strings.TrimPrefix(trimmed, "pub enum ")
			if idx := strings.Index(enumName, "{"); idx > 0 {
				enumName = strings.TrimSpace(enumName[:idx])
			}
			sb.WriteString(fmt.Sprintf("| `%s` | enum | ... |\n", enumName))
		}
		if strings.HasPrefix(trimmed, "pub fn ") {
			fnName := strings.TrimPrefix(trimmed, "pub fn ")
			if idx := strings.Index(fnName, "("); idx > 0 {
				fnName = strings.TrimSpace(fnName[:idx])
			}
			sb.WriteString(fmt.Sprintf("| `%s` | function | ... |\n", fnName))
		}
	}

	sb.WriteString("\n## Examples\n\n")
	sb.WriteString("```rust\n")
	sb.WriteString("// Example usage\n")
	sb.WriteString("```\n")

	return sb.String()
}

func generatePythonDocs(path, content string) string {
	var sb strings.Builder
	lines := strings.Split(content, "\n")

	sb.WriteString("# Module Documentation\n\n")
	sb.WriteString(fmt.Sprintf("**File:** `%s`\n\n", path))

	sb.WriteString("## Overview\n\n")
	sb.WriteString("This module provides...\n\n")

	sb.WriteString("## API Reference\n\n")
	sb.WriteString("| Function | Parameters | Returns | Description |\n")
	sb.WriteString("|----------|------------|---------|-------------|\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "def ") && !strings.HasPrefix(trimmed, "def __") {
			fnName := extractPythonFunctionName(trimmed)
			if fnName != "" && len(fnName) > 1 && fnName[0] >= 'a' && fnName[0] <= 'z' {
				// Extract parameters
				params := extractPythonParams(trimmed)
				sb.WriteString(fmt.Sprintf("| `%s` | %s | ... | ... |\n", fnName, params))
			}
		}
	}

	sb.WriteString("\n## Usage\n\n")
	sb.WriteString("```python\n")
	sb.WriteString("import " + strings.TrimSuffix(path, ".py") + "\n\n")
	sb.WriteString("# Example usage\n")
	sb.WriteString("```\n")

	return sb.String()
}

func extractPythonParams(line string) string {
	if idx := strings.Index(line, "("); idx > 0 {
		endIdx := strings.Index(line[idx:], ")")
		if endIdx > 0 {
			params := line[idx+1 : idx+endIdx]
			// Remove 'self' parameter
			params = strings.ReplaceAll(params, "self, ", "")
			params = strings.ReplaceAll(params, "self", "")
			return strings.TrimSpace(params)
		}
	}
	return ""
}

func generateTypeScriptDocs(path, content string) string {
	var sb strings.Builder
	lines := strings.Split(content, "\n")

	sb.WriteString("# Module Documentation\n\n")
	sb.WriteString(fmt.Sprintf("**File:** `%s`\n\n", path))

	sb.WriteString("## Overview\n\n")
	sb.WriteString("This module provides...\n\n")

	sb.WriteString("## API Reference\n\n")
	sb.WriteString("| Export | Type | Description |\n")
	sb.WriteString("|--------|------|-------------|\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "export function ") {
			fnName := strings.TrimPrefix(trimmed, "export function ")
			if idx := strings.Index(fnName, "("); idx > 0 {
				fnName = strings.TrimSpace(fnName[:idx])
			}
			sb.WriteString(fmt.Sprintf("| `%s` | function | ... |\n", fnName))
		}
		if strings.HasPrefix(trimmed, "export const ") {
			constName := strings.TrimPrefix(trimmed, "export const ")
			if idx := strings.Index(constName, " ="); idx > 0 {
				constName = strings.TrimSpace(constName[:idx])
			}
			sb.WriteString(fmt.Sprintf("| `%s` | constant | ... |\n", constName))
		}
		if strings.HasPrefix(trimmed, "export interface ") {
			ifaceName := strings.TrimPrefix(trimmed, "export interface ")
			if idx := strings.Index(ifaceName, "{"); idx > 0 {
				ifaceName = strings.TrimSpace(ifaceName[:idx])
			}
			sb.WriteString(fmt.Sprintf("| `%s` | interface | ... |\n", ifaceName))
		}
	}

	sb.WriteString("\n## Usage\n\n")
	sb.WriteString("```typescript\n")
	sb.WriteString("// Example usage\n")
	sb.WriteString("```\n")

	return sb.String()
}

func generateGenericDocs(path, content string) string {
	var sb strings.Builder

	sb.WriteString("# Documentation\n\n")
	sb.WriteString(fmt.Sprintf("**File:** `%s`\n\n", path))

	sb.WriteString("## Overview\n\n")
	sb.WriteString("This file provides...\n\n")

	sb.WriteString("## Contents\n\n")
	sb.WriteString("```")
	sb.WriteString("\n// Key functions and structures\n")
	sb.WriteString("```\n")

	return sb.String()
}

// capitalizeFirst converts the first character of s to uppercase.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:] // ASCII uppercase (package names are ASCII)
}
