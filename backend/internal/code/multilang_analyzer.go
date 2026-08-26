package code

import (
	"fmt"
	"regexp"
	"strings"
)

// MultiLangAnalyzer 多语言代码分析器
type MultiLangAnalyzer struct {
	goAnalyzer *ASTAnalyzer
}

// NewMultiLangAnalyzer 创建多语言分析器
func NewMultiLangAnalyzer() *MultiLangAnalyzer {
	return &MultiLangAnalyzer{
		goAnalyzer: NewASTAnalyzer(),
	}
}

// MultiLangResult 多语言分析结果
type MultiLangResult struct {
	Language   string            `json:"language"`
	Lines      int               `json:"lines"`
	Functions  []FunctionInfo    `json:"functions"`
	Imports    []string          `json:"imports"`
	Classes    []ClassInfo       `json:"classes,omitempty"`
	Variables  []VariableInfo    `json:"variables,omitempty"`
	Comments   int               `json:"comments"`
	Complexity int               `json:"complexity"`
	Warnings   []string          `json:"warnings"`
	Metrics    map[string]int    `json:"metrics"`
}

// ClassInfo 类信息
type ClassInfo struct {
	Name    string       `json:"name"`
	Methods []MethodInfo `json:"methods"`
	Fields  []FieldInfo  `json:"fields"`
}

// VariableInfo 变量信息
type VariableInfo struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Const bool   `json:"const"`
}

// Analyze 分析代码
func (m *MultiLangAnalyzer) Analyze(code string, language string) (*MultiLangResult, error) {
	switch strings.ToLower(language) {
	case "go":
		return m.analyzeGo(code)
	case "javascript", "js":
		return m.analyzeJavaScript(code)
	case "typescript", "ts":
		return m.analyzeTypeScript(code)
	case "python", "py":
		return m.analyzePython(code)
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}
}

func (m *MultiLangAnalyzer) analyzeGo(code string) (*MultiLangResult, error) {
	result, err := m.goAnalyzer.Analyze(code)
	if err != nil {
		return nil, err
	}

	imports := make([]string, 0)
	for _, imp := range result.Imports {
		imports = append(imports, imp.Path)
	}

	functions := make([]FunctionInfo, 0)
	functions = append(functions, result.Functions...)

	return &MultiLangResult{
		Language:   "go",
		Lines:      result.Lines,
		Functions:  functions,
		Imports:    imports,
		Complexity: result.Complexity,
		Warnings:   result.Warnings,
		Metrics: map[string]int{
			"structs":    len(result.Structs),
			"interfaces": len(result.Interfaces),
		},
	}, nil
}

func (m *MultiLangAnalyzer) analyzeJavaScript(code string) (*MultiLangResult, error) {
	result := &MultiLangResult{
		Language:  "javascript",
		Functions: make([]FunctionInfo, 0),
		Imports:   make([]string, 0),
		Classes:   make([]ClassInfo, 0),
		Warnings:  make([]string, 0),
		Metrics:   make(map[string]int),
	}

	lines := strings.Split(code, "\n")
	result.Lines = len(lines)

	funcRegex := regexp.MustCompile(`(?:function\s+(\w+)|(?:const|let|var)\s+(\w+)\s*=\s*(?:function|\([^)]*\)\s*=>))`)
	importRegex := regexp.MustCompile(`(?:import\s+.*?from\s+['"]([^'"]+)['"]|require\s*\(\s*['"]([^'"]+)['"]\s*\))`)
	classRegex := regexp.MustCompile(`class\s+(\w+)`)
	commentRegex := regexp.MustCompile(`//|/\*|\*/`)
	complexityRegex := regexp.MustCompile(`\b(?:if|else if|for|while|switch|case|catch|&&|\|\|)\b`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if matches := funcRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			name := matches[1]
			if name == "" {
				name = matches[2]
			}
			result.Functions = append(result.Functions, FunctionInfo{
				Name:     name,
				Exported: true,
			})
		}

		if matches := importRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			path := matches[1]
			if path == "" {
				path = matches[2]
			}
			result.Imports = append(result.Imports, path)
		}

		if matches := classRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			result.Classes = append(result.Classes, ClassInfo{
				Name: matches[1],
			})
		}

		if commentRegex.MatchString(trimmed) {
			result.Comments++
		}

		result.Complexity += len(complexityRegex.FindAllString(trimmed, -1))
	}

	result.Metrics["classes"] = len(result.Classes)
	result.Metrics["comments"] = result.Comments

	return result, nil
}

func (m *MultiLangAnalyzer) analyzeTypeScript(code string) (*MultiLangResult, error) {
	result, err := m.analyzeJavaScript(code)
	if err != nil {
		return nil, err
	}
	result.Language = "typescript"

	interfaceRegex := regexp.MustCompile(`interface\s+(\w+)`)
	typeRegex := regexp.MustCompile(`type\s+(\w+)`)

	for _, line := range strings.Split(code, "\n") {
		if interfaceRegex.MatchString(line) {
			result.Metrics["interfaces"]++
		}
		if typeRegex.MatchString(line) {
			result.Metrics["types"]++
		}
	}

	return result, nil
}

func (m *MultiLangAnalyzer) analyzePython(code string) (*MultiLangResult, error) {
	result := &MultiLangResult{
		Language:  "python",
		Functions: make([]FunctionInfo, 0),
		Imports:   make([]string, 0),
		Classes:   make([]ClassInfo, 0),
		Warnings:  make([]string, 0),
		Metrics:   make(map[string]int),
	}

	lines := strings.Split(code, "\n")
	result.Lines = len(lines)

	funcRegex := regexp.MustCompile(`def\s+(\w+)\s*\(`)
	classRegex := regexp.MustCompile(`class\s+(\w+)`)
	importRegex := regexp.MustCompile(`(?:from\s+(\S+)\s+)?import\s+(\S+)`)
	commentRegex := regexp.MustCompile(`#`)
	complexityRegex := regexp.MustCompile(`\b(?:if|elif|for|while|except|and|or)\b`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if matches := funcRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			result.Functions = append(result.Functions, FunctionInfo{
				Name:     matches[1],
				Exported: !strings.HasPrefix(matches[1], "_"),
			})
		}

		if matches := classRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			result.Classes = append(result.Classes, ClassInfo{
				Name: matches[1],
			})
		}

		if matches := importRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			path := matches[1]
			if path == "" {
				path = matches[2]
			}
			result.Imports = append(result.Imports, path)
		}

		if commentRegex.MatchString(trimmed) {
			result.Comments++
		}

		result.Complexity += len(complexityRegex.FindAllString(trimmed, -1))
	}

	result.Metrics["classes"] = len(result.Classes)
	result.Metrics["comments"] = result.Comments

	return result, nil
}
