package code

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// ASTAnalyzer AST代码分析器
type ASTAnalyzer struct {
	fset *token.FileSet
}

// NewASTAnalyzer 创建AST分析器
func NewASTAnalyzer() *ASTAnalyzer {
	return &ASTAnalyzer{
		fset: token.NewFileSet(),
	}
}

// AnalysisResult 分析结果
type AnalysisResult struct {
	PackageName string          `json:"package_name"`
	Imports     []ImportInfo    `json:"imports"`
	Functions   []FunctionInfo  `json:"functions"`
	Structs     []StructInfo    `json:"structs"`
	Interfaces  []InterfaceInfo `json:"interfaces"`
	Complexity  int             `json:"complexity"`
	Lines       int             `json:"lines"`
	Warnings    []string        `json:"warnings"`
}

// ImportInfo 导入信息
type ImportInfo struct {
	Path  string `json:"path"`
	Alias string `json:"alias,omitempty"`
}

// FunctionInfo 函数信息
type FunctionInfo struct {
	Name       string `json:"name"`
	Receiver   string `json:"receiver,omitempty"`
	Params     string `json:"params"`
	Returns    string `json:"returns"`
	Exported   bool   `json:"exported"`
	Complexity int    `json:"complexity"`
	Lines      int    `json:"lines"`
}

// StructInfo 结构体信息
type StructInfo struct {
	Name     string       `json:"name"`
	Exported bool         `json:"exported"`
	Fields   []FieldInfo  `json:"fields"`
	Methods  []MethodInfo `json:"methods"`
}

// InterfaceInfo 接口信息
type InterfaceInfo struct {
	Name    string       `json:"name"`
	Methods []MethodInfo `json:"methods"`
}

// FieldInfo 字段信息
type FieldInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// MethodInfo 方法信息
type MethodInfo struct {
	Name    string `json:"name"`
	Params  string `json:"params"`
	Returns string `json:"returns"`
}

// Analyze 分析Go代码
func (a *ASTAnalyzer) Analyze(code string) (*AnalysisResult, error) {
	file, err := parser.ParseFile(a.fset, "", code, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	result := &AnalysisResult{
		PackageName: file.Name.Name,
		Imports:     make([]ImportInfo, 0),
		Functions:   make([]FunctionInfo, 0),
		Structs:     make([]StructInfo, 0),
		Interfaces:  make([]InterfaceInfo, 0),
		Warnings:    make([]string, 0),
	}

	// 分析导入
	for _, imp := range file.Imports {
		importInfo := ImportInfo{Path: strings.Trim(imp.Path.Value, "\"")}
		if imp.Name != nil {
			importInfo.Alias = imp.Name.Name
		}
		result.Imports = append(result.Imports, importInfo)
	}

	// 分析声明
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			funcInfo := a.analyzeFunc(d)
			result.Functions = append(result.Functions, funcInfo)
			result.Complexity += funcInfo.Complexity

		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					a.analyzeType(s, result)
				}
			}
		}
	}

	// 统计行数
	result.Lines = strings.Count(code, "\n") + 1

	// 检查警告
	a.checkWarnings(result)

	return result, nil
}

func (a *ASTAnalyzer) analyzeFunc(d *ast.FuncDecl) FunctionInfo {
	info := FunctionInfo{
		Name:     d.Name.Name,
		Exported: ast.IsExported(d.Name.Name),
	}

	if d.Recv != nil && len(d.Recv.List) > 0 {
		if t, ok := d.Recv.List[0].Type.(*ast.StarExpr); ok {
			if ident, ok := t.X.(*ast.Ident); ok {
				info.Receiver = ident.Name
			}
		}
	}

	if d.Type.Params != nil {
		info.Params = a.formatFieldList(d.Type.Params)
	}

	if d.Type.Results != nil {
		info.Returns = a.formatFieldList(d.Type.Results)
	}

	// 计算复杂度
	info.Complexity = a.calculateComplexity(d)

	return info
}

func (a *ASTAnalyzer) analyzeType(spec *ast.TypeSpec, result *AnalysisResult) {
	switch t := spec.Type.(type) {
	case *ast.StructType:
		structInfo := StructInfo{
			Name:     spec.Name.Name,
			Exported: ast.IsExported(spec.Name.Name),
		}
		for _, field := range t.Fields.List {
			for _, name := range field.Names {
				structInfo.Fields = append(structInfo.Fields, FieldInfo{
					Name: name.Name,
					Type: a.formatExpr(field.Type),
				})
			}
		}
		result.Structs = append(result.Structs, structInfo)

	case *ast.InterfaceType:
		ifaceInfo := InterfaceInfo{
			Name: spec.Name.Name,
		}
		for _, method := range t.Methods.List {
			if fn, ok := method.Type.(*ast.FuncType); ok {
				ifaceInfo.Methods = append(ifaceInfo.Methods, MethodInfo{
					Name:    method.Names[0].Name,
					Params:  a.formatFieldList(fn.Params),
					Returns: a.formatFieldList(fn.Results),
				})
			}
		}
		result.Interfaces = append(result.Interfaces, ifaceInfo)
	}
}

func (a *ASTAnalyzer) formatFieldList(fields *ast.FieldList) string {
	if fields == nil {
		return ""
	}
	parts := make([]string, 0)
	for _, field := range fields.List {
		typeStr := a.formatExpr(field.Type)
		if len(field.Names) > 0 {
			for _, name := range field.Names {
				parts = append(parts, name.Name+" "+typeStr)
			}
		} else {
			parts = append(parts, typeStr)
		}
	}
	return strings.Join(parts, ", ")
}

func (a *ASTAnalyzer) formatExpr(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + a.formatExpr(t.X)
	case *ast.SelectorExpr:
		return a.formatExpr(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + a.formatExpr(t.Elt)
	case *ast.MapType:
		return "map[" + a.formatExpr(t.Key) + "]" + a.formatExpr(t.Value)
	default:
		return "interface{}"
	}
}

func (a *ASTAnalyzer) calculateComplexity(d *ast.FuncDecl) int {
	complexity := 1
	ast.Inspect(d.Body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
			complexity++
		case *ast.CaseClause:
			complexity++
		}
		return true
	})
	return complexity
}

func (a *ASTAnalyzer) checkWarnings(result *AnalysisResult) {
	for _, f := range result.Functions {
		if f.Complexity > 15 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Function %s has high complexity: %d", f.Name, f.Complexity))
		}
	}
	if len(result.Functions) == 0 {
		result.Warnings = append(result.Warnings, "No functions defined")
	}
	if result.Complexity > 100 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Total complexity is high: %d", result.Complexity))
	}
}
