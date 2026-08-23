package rag

import (
	"regexp"
	"strings"
)

// ElementType represents the type of a code element.
type ElementType string

const (
	ElementFunction  ElementType = "function"
	ElementVariable  ElementType = "variable"
	ElementStruct    ElementType = "struct"
	ElementImport    ElementType = "import"
	ElementPackage   ElementType = "package"
	ElementShellFunc ElementType = "shell_function"
	ElementShellVar  ElementType = "shell_variable"
)

// CodeElement represents a parsed structural element from source code.
type CodeElement struct {
	Name      string      `json:"name"`
	Type      ElementType `json:"type"`
	FilePath  string      `json:"file_path"`
	Line      int         `json:"line"`
	EndLine   int         `json:"end_line"`
	Signature string      `json:"signature,omitempty"`
	Calls     []string    `json:"calls,omitempty"`
}

// RelationGraph maps symbol names to the elements that call them.
type RelationGraph struct {
	Callees map[string][]CodeElement // symbol -> elements that define it
	Callers map[string][]CodeElement // symbol -> elements that call it
}

// StructuralIndex provides structure-aware code indexing.
type StructuralIndex struct {
	elements []CodeElement
	graph    RelationGraph
}

// NewStructuralIndex creates an empty structural index.
func NewStructuralIndex() *StructuralIndex {
	return &StructuralIndex{
		elements: make([]CodeElement, 0),
		graph: RelationGraph{
			Callees: make(map[string][]CodeElement),
			Callers: make(map[string][]CodeElement),
		},
	}
}

// Elements returns all indexed elements.
func (si *StructuralIndex) Elements() []CodeElement {
	return si.elements
}

// Graph returns the relation graph.
func (si *StructuralIndex) Graph() RelationGraph {
	return si.graph
}

// ParseAndIndex parses a source file and indexes its structural elements.
func (si *StructuralIndex) ParseAndIndex(filePath, content string) {
	lines := strings.Split(content, "\n")
	ext := filePath
	if idx := strings.LastIndex(filePath, "."); idx >= 0 {
		ext = filePath[idx:]
	}

	switch ext {
	case ".go":
		si.parseGo(filePath, lines)
	case ".sh":
		si.parseShell(filePath, lines)
	}
}

// BuildRelationGraph建立调用关系图。
func (si *StructuralIndex) BuildRelationGraph() {
	si.graph = RelationGraph{
		Callees: make(map[string][]CodeElement),
		Callers: make(map[string][]CodeElement),
	}

	for _, elem := range si.elements {
		// Index by name (what this element defines)
		si.graph.Callees[elem.Name] = append(si.graph.Callees[elem.Name], elem)

		// Index by calls (what this element calls)
		for _, called := range elem.Calls {
			si.graph.Callers[called] = append(si.graph.Callers[called], elem)
		}
	}
}

// QueryBySymbol finds all elements that define or reference a symbol.
func (si *StructuralIndex) QueryBySymbol(name string) []CodeElement {
	var result []CodeElement

	// Elements that define this symbol
	if defs, ok := si.graph.Callees[name]; ok {
		result = append(result, defs...)
	}

	// Elements that call this symbol
	if callers, ok := si.graph.Callers[name]; ok {
		result = append(result, callers...)
	}

	return result
}

// ═══════════════════════════════════════════════════════
// Go parser (regex-based, not full AST)
// ═══════════════════════════════════════════════════════

var (
	goFuncRe      = regexp.MustCompile(`^func\s+(?:\(\s*\w+\s+\S+\s*\)\s+)?(\w+)\s*\(`)
	goVarRe       = regexp.MustCompile(`^var\s+(\w+)`)
	goStructRe    = regexp.MustCompile(`^type\s+(\w+)\s+struct`)
	goImportRe    = regexp.MustCompile(`^import\s+"([^"]+)"`)
	goImportBlock = regexp.MustCompile(`^import\s+\(`)
	goCallRe      = regexp.MustCompile(`(\w+)\s*\(`)
)

func (si *StructuralIndex) parseGo(filePath string, lines []string) {
	inImportBlock := false
	inFunc := false
	funcName := ""
	funcStart := 0
	braceDepth := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineNum := i + 1

		// Skip comments and empty lines
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Package declaration
		if strings.HasPrefix(trimmed, "package ") {
			pkg := strings.TrimSpace(strings.TrimPrefix(trimmed, "package "))
			si.elements = append(si.elements, CodeElement{
				Name: pkg, Type: ElementPackage, FilePath: filePath,
				Line: lineNum, EndLine: lineNum,
			})
			continue
		}

		// Import block
		if goImportBlock.MatchString(trimmed) {
			inImportBlock = true
			continue
		}
		if inImportBlock {
			if trimmed == ")" {
				inImportBlock = false
				continue
			}
			if m := goImportRe.FindStringSubmatch(trimmed); m != nil {
				si.elements = append(si.elements, CodeElement{
					Name: m[1], Type: ElementImport, FilePath: filePath,
					Line: lineNum, EndLine: lineNum,
				})
			}
			continue
		}

		// Single import
		if m := goImportRe.FindStringSubmatch(trimmed); m != nil {
			si.elements = append(si.elements, CodeElement{
				Name: m[1], Type: ElementImport, FilePath: filePath,
				Line: lineNum, EndLine: lineNum,
			})
			continue
		}

		// Function declaration
		if m := goFuncRe.FindStringSubmatch(trimmed); m != nil {
			inFunc = true
			funcName = m[1]
			funcStart = lineNum
			braceDepth = 0
			continue
		}

		// Struct declaration
		if m := goStructRe.FindStringSubmatch(trimmed); m != nil {
			si.elements = append(si.elements, CodeElement{
				Name: m[1], Type: ElementStruct, FilePath: filePath,
				Line: lineNum, EndLine: lineNum,
				Signature: trimmed,
			})
			continue
		}

		// Variable declaration
		if m := goVarRe.FindStringSubmatch(trimmed); m != nil {
			si.elements = append(si.elements, CodeElement{
				Name: m[1], Type: ElementVariable, FilePath: filePath,
				Line: lineNum, EndLine: lineNum,
			})
			continue
		}

		// Track function body
		if inFunc {
			braceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
			// Collect function calls in this line
			calls := extractCalls(trimmed)
			if len(calls) > 0 && funcName != "" {
				// Find or create the function element
				for idx := len(si.elements) - 1; idx >= 0; idx-- {
					if si.elements[idx].Name == funcName && si.elements[idx].Type == ElementFunction {
						si.elements[idx].Calls = append(si.elements[idx].Calls, calls...)
						si.elements[idx].EndLine = lineNum
						break
					}
				}
			}
			if braceDepth <= 0 && strings.Contains(trimmed, "}") {
				// End of function
				if funcName != "" {
					si.elements = append(si.elements, CodeElement{
						Name: funcName, Type: ElementFunction, FilePath: filePath,
						Line: funcStart, EndLine: lineNum,
						Signature: "func " + funcName + "()",
					})
				}
				inFunc = false
				funcName = ""
			}
		}
	}

	// Handle unclosed function
	if inFunc && funcName != "" {
		si.elements = append(si.elements, CodeElement{
			Name: funcName, Type: ElementFunction, FilePath: filePath,
			Line: funcStart, EndLine: len(lines),
			Signature: "func " + funcName + "()",
		})
	}
}

// ═══════════════════════════════════════════════════════
// Shell parser (regex-based)
// ═══════════════════════════════════════════════════════

var (
	shellFuncRe = regexp.MustCompile(`^(\w+)\s*\(\)\s*\{`)
	shellVarRe  = regexp.MustCompile(`^(\w+)=`)
	shellCallRe = regexp.MustCompile(`(\w+)\s`)
)

func (si *StructuralIndex) parseShell(filePath string, lines []string) {
	inFunc := false
	funcName := ""
	funcStart := 0
	braceDepth := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineNum := i + 1

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Function definition: name() {
		if m := shellFuncRe.FindStringSubmatch(trimmed); m != nil {
			inFunc = true
			funcName = m[1]
			funcStart = lineNum
			braceDepth = 1
			continue
		}

		// Variable assignment (top-level)
		if !inFunc {
			if m := shellVarRe.FindStringSubmatch(trimmed); m != nil {
				si.elements = append(si.elements, CodeElement{
					Name: m[1], Type: ElementShellVar, FilePath: filePath,
					Line: lineNum, EndLine: lineNum,
				})
				continue
			}
		}

		// Track function body
		if inFunc {
			braceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
			if braceDepth <= 0 && strings.Contains(trimmed, "}") {
				si.elements = append(si.elements, CodeElement{
					Name: funcName, Type: ElementShellFunc, FilePath: filePath,
					Line: funcStart, EndLine: lineNum,
					Signature: funcName + "()",
				})
				inFunc = false
				funcName = ""
			}
		}
	}
}

// extractCalls extracts function call names from a line of code.
func extractCalls(line string) []string {
	var calls []string
	seen := make(map[string]bool)

	matches := goCallRe.FindAllStringSubmatch(line, -1)
	for _, m := range matches {
		name := m[1]
		// Skip Go keywords and common false positives
		if isGoKeyword(name) || len(name) < 2 || seen[name] {
			continue
		}
		seen[name] = true
		calls = append(calls, name)
	}

	return calls
}

// isGoKeyword checks if a name is a Go keyword or builtin.
func isGoKeyword(name string) bool {
	keywords := map[string]bool{
		"if": true, "else": true, "for": true, "range": true,
		"func": true, "return": true, "var": true, "const": true,
		"type": true, "struct": true, "interface": true, "map": true,
		"slice": true, "go": true, "select": true, "case": true,
		"switch": true, "default": true, "break": true, "continue": true,
		"package": true, "import": true, "defer": true, "chan": true,
		"true": true, "false": true, "nil": true,
		"make": true, "len": true, "cap": true, "new": true, "append": true,
		"copy": true, "delete": true, "close": true, "panic": true, "recover": true,
		"print": true, "println": true, "error": true, "string": true,
		"int": true, "int64": true, "float64": true, "bool": true, "byte": true,
	}
	return keywords[name]
}
