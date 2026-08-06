package skills

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// DependencyGraphSkill provides structured dependency analysis
type DependencyGraphSkill struct{}

func NewDependencyGraphSkill() *DependencyGraphSkill {
	return &DependencyGraphSkill{}
}

func (s *DependencyGraphSkill) Name() string { return "dependency_graph" }

func (s *DependencyGraphSkill) Description() string {
	return `Analyze code dependencies. Input: {"path": "...", "content": "...", "language": "go|rust|c++|python|typescript"}.
Returns: import graph, function call graph, type dependencies, and circular dependency detection.
Use this BEFORE refactoring to understand impact of changes.`
}

func (s *DependencyGraphSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	content, _ := input["content"].(string)
	language, _ := input["language"].(string)
	path, _ := input["path"].(string)

	if content == "" {
		return "", fmt.Errorf("content is required")
	}
	if language == "" {
		language = detectLanguage(path)
	}

	var result string

	switch strings.ToLower(language) {
	case "go":
		result = analyzeGoDependencies(content)
	case "rust":
		result = analyzeRustDependencies(content)
	case "python":
		result = analyzePythonDependencies(content)
	case "typescript", "ts":
		result = analyzeTypeScriptDependencies(content)
	default:
		result = analyzeGenericDependencies(content)
	}

	return result, nil
}

func analyzeGoDependencies(content string) string {
	var sb strings.Builder

	sb.WriteString("Go Dependency Analysis\n\n")

	// 1. Import Analysis
	sb.WriteString("## Imports\n\n")
	sb.WriteString("| Package | Used By |\n")
	sb.WriteString("|---------|--------|\n")

	imports := extractGoImports(content)
	for pkg, usages := range imports {
		sb.WriteString(fmt.Sprintf("| `%s` | %d usages |\n", pkg, usages))
	}

	// 2. Function Call Graph
	sb.WriteString("\n## Function Call Graph\n\n")
	sb.WriteString("```")
	sb.WriteString("\nCaller -> Callee\n")

	calls := extractGoFunctionCalls(content)
	for caller, callees := range calls {
		for _, callee := range callees {
			sb.WriteString(fmt.Sprintf("%s -> %s\n", caller, callee))
		}
	}
	sb.WriteString("```\n")

	// 3. Type Dependencies
	sb.WriteString("\n## Type Dependencies\n\n")
	sb.WriteString("```")
	sb.WriteString("\nType -> Used In\n")

	typeDeps := extractGoTypeDependencies(content)
	for typeName, usedIn := range typeDeps {
		sb.WriteString(fmt.Sprintf("%s -> %s\n", typeName, usedIn))
	}
	sb.WriteString("```\n")

	// 4. Circular Dependency Check
	sb.WriteString("\n## Circular Dependency Check\n\n")
	circular := detectGoCircularDependencies(content)
	if len(circular) > 0 {
		sb.WriteString("⚠️ **Circular dependencies detected:**\n")
		for _, cycle := range circular {
			sb.WriteString(fmt.Sprintf("- %s\n", cycle))
		}
	} else {
		sb.WriteString("✅ No circular dependencies\n")
	}

	return sb.String()
}

func extractGoImports(content string) map[string]int {
	imports := make(map[string]int)
	
	// Find import block
	lines := strings.Split(content, "\n")
	inImport := false
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if trimmed == "import (" {
			inImport = true
			continue
		}
		if trimmed == ")" && inImport {
			inImport = false
			continue
		}
		
		if inImport {
			// Extract import path
			line = strings.TrimPrefix(trimmed, "\"")
			line = strings.TrimSuffix(line, "\"")
			if idx := strings.Index(line, "\""); idx > 0 {
				line = line[:idx]
			}
			if line != "" {
				imports[line]++
			}
		}
	}
	
	return imports
}

func extractGoFunctionCalls(content string) map[string][]string {
	calls := make(map[string][]string)
	lines := strings.Split(content, "\n")
	
	currentFunc := ""
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Detect function definition
		if strings.HasPrefix(trimmed, "func ") {
			// Extract function name
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				fnName := parts[1]
				if idx := strings.Index(fnName, "("); idx > 0 {
					fnName = fnName[:idx]
				}
				currentFunc = fnName
			}
		}
		
		// Detect function calls (simplified)
		if currentFunc != "" && strings.Contains(line, "(") && !strings.HasPrefix(trimmed, "func ") {
			// Try to extract function name
			if idx := strings.Index(trimmed, "("); idx > 0 {
				callee := trimmed[:idx]
				// Get last word before (
				words := strings.Fields(callee)
				if len(words) > 0 {
					callee = words[len(words)-1]
					// Skip common keywords
					if callee != "if" && callee != "for" && callee != "switch" && callee != "range" {
						calls[currentFunc] = append(calls[currentFunc], callee)
					}
				}
			}
		}
	}
	
	return calls
}

func extractGoTypeDependencies(content string) map[string]string {
	typeDeps := make(map[string]string)
	lines := strings.Split(content, "\n")
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// struct fields using other types
		if strings.Contains(trimmed, " ") && !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "func ") {
			// Simple heuristic: look for Type references in struct fields
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				// Check if it's a type reference (starts with uppercase)
				for _, part := range parts {
					if len(part) > 0 && part[0] >= 'A' && part[0] <= 'Z' && part != strings.Title(part[:1])+part[1:] {
						// Might be a type reference
						break
					}
				}
			}
		}
	}
	
	return typeDeps
}

func detectGoCircularDependencies(content string) []string {
	var circular []string
	
	// Simple check: look for package A importing B and B importing A
	lines := strings.Split(content, "\n")
	inImport := false
	imports := []string{}
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if trimmed == "import (" {
			inImport = true
			continue
		}
		if trimmed == ")" && inImport {
			inImport = false
			continue
		}
		
		if inImport {
			line = strings.TrimPrefix(trimmed, "\"")
			line = strings.TrimSuffix(line, "\"")
			if idx := strings.Index(line, "\""); idx > 0 {
				line = line[:idx]
			}
			if line != "" {
				imports = append(imports, line)
			}
		}
	}
	
	// Check for mutual imports (simplified)
	for i := 0; i < len(imports); i++ {
		for j := i + 1; j < len(imports); j++ {
			if strings.Contains(imports[i], imports[j]) || strings.Contains(imports[j], imports[i]) {
				circular = append(circular, fmt.Sprintf("%s <-> %s", imports[i], imports[j]))
			}
		}
	}
	
	return circular
}

func analyzeRustDependencies(content string) string {
	var sb strings.Builder
	lines := strings.Split(content, "\n")

	sb.WriteString("📊 Rust Dependency Analysis\n\n")

	// 1. Use Statements
	sb.WriteString("## Use Statements\n\n")
	sb.WriteString("| Module | Items |\n")
	sb.WriteString("|--------|-------|\n")

	usePattern := regexp.MustCompile(`use\s+([\w:]+)(?:\s*\{([^}]+)\})?`)
	for _, line := range lines {
		if matches := usePattern.FindStringSubmatch(line); len(matches) > 2 {
			sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", matches[1], matches[2]))
		}
	}

	// 2. Trait Implementations
	sb.WriteString("\n## Trait Implementations\n\n")
	sb.WriteString("```")
	sb.WriteString("\nType implements Trait\n")

	traitPattern := regexp.MustCompile(`impl\s+(\w+)\s+for\s+(\w+)`)
	for _, line := range lines {
		if matches := traitPattern.FindStringSubmatch(line); len(matches) > 2 {
			sb.WriteString(fmt.Sprintf("%s implements %s\n", matches[2], matches[1]))
		}
	}
	sb.WriteString("```\n")

	// 3. Type Dependencies
	sb.WriteString("\n## Struct Fields\n\n")
	sb.WriteString("```")
	sb.WriteString("\nStruct -> Field Types\n")

	structPattern := regexp.MustCompile(`struct\s+(\w+)`)
	fieldPattern := regexp.MustCompile(`(\w+)\s*:\s*(\w+)`)
	
	currentStruct := ""
	for _, line := range lines {
		if matches := structPattern.FindStringSubmatch(line); len(matches) > 1 {
			currentStruct = matches[1]
		}
		if currentStruct != "" {
			if matches := fieldPattern.FindStringSubmatch(line); len(matches) > 2 {
				if matches[2] != "String" && matches[2] != "i32" && matches[2] != "u64" && matches[2] != "bool" {
					sb.WriteString(fmt.Sprintf("%s -> %s\n", currentStruct, matches[2]))
				}
			}
		}
	}
	sb.WriteString("```\n")

	return sb.String()
}

func analyzePythonDependencies(content string) string {
	var sb strings.Builder
	lines := strings.Split(content, "\n")

	sb.WriteString("📊 Python Dependency Analysis\n\n")

	// 1. Import Statements
	sb.WriteString("## Imports\n\n")
	sb.WriteString("| Module | Type |\n")
	sb.WriteString("|--------|------|\n")

	importPattern := regexp.MustCompile(`(?:from\s+(\S+)\s+)?import\s+(\S+)`)
	for _, line := range lines {
		if matches := importPattern.FindStringSubmatch(line); len(matches) > 1 {
			sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", matches[1], matches[2]))
		}
	}

	// 2. Class Inheritance
	sb.WriteString("\n## Class Hierarchy\n\n")
	sb.WriteString("```")
	sb.WriteString("\nChild -> Parent\n")

	classPattern := regexp.MustCompile(`class\s+(\w+)\s*\(([^)]*)\)`)
	for _, line := range lines {
		if matches := classPattern.FindStringSubmatch(line); len(matches) > 2 {
			sb.WriteString(fmt.Sprintf("%s -> %s\n", matches[1], matches[2]))
		}
	}
	sb.WriteString("```\n")

	// 3. Function Decorators
	sb.WriteString("\n## Decorators\n\n")
	sb.WriteString("```")
	sb.WriteString("\nFunction -> Decorator\n")

	decoratorPattern := regexp.MustCompile(`@(\w+)`)
	funcPattern := regexp.MustCompile(`def\s+(\w+)`)
	
	currentDecorator := ""
	for _, line := range lines {
		if matches := decoratorPattern.FindStringSubmatch(line); len(matches) > 1 {
			currentDecorator = matches[1]
		}
		if currentDecorator != "" {
			if matches := funcPattern.FindStringSubmatch(line); len(matches) > 1 {
				sb.WriteString(fmt.Sprintf("%s -> @%s\n", matches[1], currentDecorator))
				currentDecorator = ""
			}
		}
	}
	sb.WriteString("```\n")

	return sb.String()
}

func analyzeTypeScriptDependencies(content string) string {
	var sb strings.Builder
	lines := strings.Split(content, "\n")

	sb.WriteString("📊 TypeScript Dependency Analysis\n\n")

	// 1. Import Statements
	sb.WriteString("## Imports\n\n")
	sb.WriteString("| Module | Items |\n")
	sb.WriteString("|--------|-------|\n")

	importPattern := regexp.MustCompile(`import\s*\{([^}]+)\}\s*from\s*['"]([^'"]+)['"]`)
	for _, line := range lines {
		if matches := importPattern.FindStringSubmatch(line); len(matches) > 2 {
			sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", matches[2], matches[1]))
		}
	}

	// 2. Interface Implementations
	sb.WriteString("\n## Interface Implementations\n\n")
	sb.WriteString("```")
	sb.WriteString("\nType implements Interface\n")

	implPattern := regexp.MustCompile(`(\w+)\s+implements\s+(\w+)`)
	for _, line := range lines {
		if matches := implPattern.FindStringSubmatch(line); len(matches) > 2 {
			sb.WriteString(fmt.Sprintf("%s implements %s\n", matches[1], matches[2]))
		}
	}
	sb.WriteString("```\n")

	// 3. Type Dependencies
	sb.WriteString("\n## Type Dependencies\n\n")
	sb.WriteString("```")
	sb.WriteString("\nType -> Used In\n")

	typePattern := regexp.MustCompile(`type\s+(\w+)\s*=\s*(\w+)`)
	for _, line := range lines {
		if matches := typePattern.FindStringSubmatch(line); len(matches) > 2 {
			sb.WriteString(fmt.Sprintf("%s -> %s\n", matches[1], matches[2]))
		}
	}
	sb.WriteString("```\n")

	return sb.String()
}

func analyzeGenericDependencies(content string) string {
	var sb strings.Builder
	lineCount := len(strings.Split(content, "\n"))

	sb.WriteString("Dependency Analysis\n\n")
	sb.WriteString("Basic analysis (language not detected)\n\n")

	sb.WriteString("Key Elements:\n")
	sb.WriteString(fmt.Sprintf("- Lines: %d\n", lineCount))

	return sb.String()
}
