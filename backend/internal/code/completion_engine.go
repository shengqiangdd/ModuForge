package code

import (
	"strings"
)

// CompletionEngine 代码补全引擎
type CompletionEngine struct {
	snippets *SnippetLibrary
}

// NewCompletionEngine 创建补全引擎
func NewCompletionEngine() *CompletionEngine {
	return &CompletionEngine{
		snippets: NewSnippetLibrary(),
	}
}

// CompletionRequest 补全请求
type CompletionRequest struct {
	Code       string `json:"code"`
	Language   string `json:"language"`
	CursorLine int    `json:"cursor_line"`
	CursorCol  int    `json:"cursor_col"`
	Context    string `json:"context"`
}

// CompletionResult 补全结果
type CompletionResult struct {
	Completions []Completion `json:"completions"`
	Context     string       `json:"context"`
}

// Completion 补全项
type Completion struct {
	Label       string `json:"label"`
	Kind        string `json:"kind"` // function, keyword, snippet, variable, type
	Detail      string `json:"detail"`
	InsertText  string `json:"insert_text"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
}

// GetCompletions 获取补全建议
func (e *CompletionEngine) GetCompletions(req CompletionRequest) *CompletionResult {
	result := &CompletionResult{
		Completions: make([]Completion, 0),
		Context:     req.Context,
	}

	// 分析当前行
	lines := strings.Split(req.Code, "\n")
	if req.CursorLine < 0 || req.CursorLine >= len(lines) {
		return result
	}

	currentLine := lines[req.CursorLine]
	prefix := strings.TrimSpace(currentLine[:min(req.CursorCol, len(currentLine))])

	// 根据语言和前缀获取补全
	switch req.Language {
	case "go":
		result.Completions = append(result.Completions, e.getGoCompletions(prefix)...)
	case "javascript", "typescript":
		result.Completions = append(result.Completions, e.getJSCompletions(prefix)...)
	case "python":
		result.Completions = append(result.Completions, e.getPythonCompletions(prefix)...)
	}

	// 从代码库中学习
	result.Completions = append(result.Completions, e.getContextCompletions(req.Code, req.Language, prefix)...)

	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (e *CompletionEngine) getGoCompletions(prefix string) []Completion {
	completions := make([]Completion, 0)

	// Go 关键字
	goKeywords := []Completion{
		{Label: "func", Kind: "keyword", Detail: "函数声明", InsertText: "func ${1:name}(${2:params}) ${3:return} {\n\t${0}\n}", Priority: 100},
		{Label: "if", Kind: "keyword", Detail: "条件语句", InsertText: "if ${1:condition} {\n\t${0}\n}", Priority: 100},
		{Label: "for", Kind: "keyword", Detail: "循环语句", InsertText: "for ${1:i} := 0; ${1:i} < ${2:n}; ${1:i}++ {\n\t${0}\n}", Priority: 100},
		{Label: "return", Kind: "keyword", Detail: "返回语句", InsertText: "return ${0}", Priority: 100},
		{Label: "struct", Kind: "keyword", Detail: "结构体声明", InsertText: "type ${1:Name} struct {\n\t${0}\n}", Priority: 100},
		{Label: "interface", Kind: "keyword", Detail: "接口声明", InsertText: "type ${1:Name} interface {\n\t${0}\n}", Priority: 100},
		{Label: "import", Kind: "keyword", Detail: "导入语句", InsertText: "import (\n\t\"${0}\"\n)", Priority: 100},
	}

	for _, kw := range goKeywords {
		if strings.HasPrefix(kw.Label, prefix) || prefix == "" {
			completions = append(completions, kw)
		}
	}

	// Go 常用代码片段
	goSnippets := []Completion{
		{Label: "http-server", Kind: "snippet", Detail: "HTTP服务器模板", InsertText: "app := fiber.New()\napp.Get(\"/\", func(c fiber.Ctx) error {\n\treturn c.JSON(fiber.Map{\n\t\t\"message\": \"Hello\",\n\t})\n})\nlog.Fatal(app.Listen(\":3000\"))", Priority: 80},
		{Label: "error-handling", Kind: "snippet", Detail: "错误处理模板", InsertText: "if err != nil {\n\treturn fmt.Errorf(\"operation failed: %w\", err)\n}", Priority: 80},
		{Label: "json-marshal", Kind: "snippet", Detail: "JSON序列化", InsertText: "data, err := json.Marshal(${1:obj})\nif err != nil {\n\treturn err\n}", Priority: 70},
	}

	for _, sn := range goSnippets {
		if strings.HasPrefix(sn.Label, prefix) || prefix == "" {
			completions = append(completions, sn)
		}
	}

	return completions
}

func (e *CompletionEngine) getJSCompletions(prefix string) []Completion {
	completions := make([]Completion, 0)

	// JavaScript 关键字
	jsKeywords := []Completion{
		{Label: "function", Kind: "keyword", Detail: "函数声明", InsertText: "function ${1:name}(${2:params}) {\n\t${0}\n}", Priority: 100},
		{Label: "const", Kind: "keyword", Detail: "常量声明", InsertText: "const ${1:name} = ${0};", Priority: 100},
		{Label: "let", Kind: "keyword", Detail: "变量声明", InsertText: "let ${1:name} = ${0};", Priority: 100},
		{Label: "async", Kind: "keyword", Detail: "异步函数", InsertText: "async function ${1:name}(${2:params}) {\n\t${0}\n}", Priority: 100},
		{Label: "await", Kind: "keyword", Detail: "等待Promise", InsertText: "await ${0}", Priority: 100},
		{Label: "class", Kind: "keyword", Detail: "类声明", InsertText: "class ${1:Name} {\n\t${0}\n}", Priority: 100},
		{Label: "import", Kind: "keyword", Detail: "导入语句", InsertText: "import { ${0} } from '${1:module}';", Priority: 100},
	}

	for _, kw := range jsKeywords {
		if strings.HasPrefix(kw.Label, prefix) || prefix == "" {
			completions = append(completions, kw)
		}
	}

	// JavaScript 常用代码片段
	jsSnippets := []Completion{
		{Label: "fetch-api", Kind: "snippet", Detail: "Fetch API调用", InsertText: "const response = await fetch('${1:url}');\nconst data = await response.json();\nconsole.log(data);", Priority: 80},
		{Label: "try-catch", Kind: "snippet", Detail: "错误处理", InsertText: "try {\n\t${0}\n} catch (error) {\n\tconsole.error('Error:', error);\n}", Priority: 80},
		{Label: "debounce", Kind: "snippet", Detail: "防抖函数", InsertText: "function debounce(func, wait) {\n\tlet timeout;\n\treturn function(...args) {\n\t\tclearTimeout(timeout);\n\t\ttimeout = setTimeout(() => func.apply(this, args), wait);\n\t};\n}", Priority: 70},
	}

	for _, sn := range jsSnippets {
		if strings.HasPrefix(sn.Label, prefix) || prefix == "" {
			completions = append(completions, sn)
		}
	}

	return completions
}

func (e *CompletionEngine) getPythonCompletions(prefix string) []Completion {
	completions := make([]Completion, 0)

	// Python 关键字
	pyKeywords := []Completion{
		{Label: "def", Kind: "keyword", Detail: "函数定义", InsertText: "def ${1:name}(${2:params}):\n\t\"\"\"${3:description}\"\"\"\n\t${0}", Priority: 100},
		{Label: "class", Kind: "keyword", Detail: "类定义", InsertText: "class ${1:Name}:\n\t\"\"\"${2:description}\"\"\"\n\t\n\tdef __init__(self${3:, params}):\n\t\t${0}", Priority: 100},
		{Label: "if", Kind: "keyword", Detail: "条件语句", InsertText: "if ${1:condition}:\n\t${0}", Priority: 100},
		{Label: "for", Kind: "keyword", Detail: "循环语句", InsertText: "for ${1:item} in ${2:iterable}:\n\t${0}", Priority: 100},
		{Label: "try", Kind: "keyword", Detail: "异常处理", InsertText: "try:\n\t${0}\nexcept ${1:Exception} as e:\n\tprint(f\"Error: {e}\")", Priority: 100},
		{Label: "with", Kind: "keyword", Detail: "上下文管理器", InsertText: "with ${1:expression} as ${2:variable}:\n\t${0}", Priority: 100},
		{Label: "import", Kind: "keyword", Detail: "导入语句", InsertText: "import ${0}", Priority: 100},
	}

	for _, kw := range pyKeywords {
		if strings.HasPrefix(kw.Label, prefix) || prefix == "" {
			completions = append(completions, kw)
		}
	}

	// Python 常用代码片段
	pySnippets := []Completion{
		{Label: "fastapi", Kind: "snippet", Detail: "FastAPI路由", InsertText: "@app.get(\"/${1:path}\")\nasync def ${2:handler}():\n\treturn {\"message\": \"${3:response}\"}", Priority: 80},
		{Label: "dataclass", Kind: "snippet", Detail: "数据类", InsertText: "@dataclass\nclass ${1:Name}:\n\t${2:field}: ${3:type}", Priority: 70},
		{Label: "context-manager", Kind: "snippet", Detail: "上下文管理器", InsertText: "from contextlib import contextmanager\n\n@contextmanager\nndef ${1:name}():\n\ttry:\n\t\tyield\n\tfinally:\n\t\t${0}", Priority: 70},
	}

	for _, sn := range pySnippets {
		if strings.HasPrefix(sn.Label, prefix) || prefix == "" {
			completions = append(completions, sn)
		}
	}

	return completions
}

func (e *CompletionEngine) getContextCompletions(code string, language string, prefix string) []Completion {
	completions := make([]Completion, 0)

	// 从当前代码中提取变量和函数
	lines := strings.Split(code, "\n")
	seen := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		switch language {
		case "go":
			// 提取函数名
			if strings.HasPrefix(line, "func ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					name := strings.Split(parts[1], "(")[0]
					if !seen[name] && (prefix == "" || strings.HasPrefix(name, prefix)) {
						completions = append(completions, Completion{
							Label:    name,
							Kind:     "function",
							Detail:   "当前文件中的函数",
							Priority: 50,
						})
						seen[name] = true
					}
				}
			}
			// 提取变量
			if strings.Contains(line, ":=") {
				parts := strings.SplitN(line, ":=", 2)
				if len(parts) == 2 {
					name := strings.TrimSpace(parts[0])
					if !seen[name] && (prefix == "" || strings.HasPrefix(name, prefix)) {
						completions = append(completions, Completion{
							Label:    name,
							Kind:     "variable",
							Detail:   "当前作用域中的变量",
							Priority: 40,
						})
						seen[name] = true
					}
				}
			}

		case "javascript", "typescript":
			// 提取函数名
			if strings.Contains(line, "function ") {
				parts := strings.Split(line, "function ")
				if len(parts) == 2 {
					name := strings.Split(parts[1], "(")[0]
					if !seen[name] && (prefix == "" || strings.HasPrefix(name, prefix)) {
						completions = append(completions, Completion{
							Label:    name,
							Kind:     "function",
							Detail:   "当前文件中的函数",
							Priority: 50,
						})
						seen[name] = true
					}
				}
			}
			// 提取变量
			if strings.Contains(line, "const ") || strings.Contains(line, "let ") || strings.Contains(line, "var ") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					name := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(parts[0], "const ", ""), "let ", ""))
					if !seen[name] && (prefix == "" || strings.HasPrefix(name, prefix)) {
						completions = append(completions, Completion{
							Label:    name,
							Kind:     "variable",
							Detail:   "当前作用域中的变量",
							Priority: 40,
						})
						seen[name] = true
					}
				}
			}

		case "python":
			// 提取函数名
			if strings.HasPrefix(line, "def ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					name := strings.Split(parts[1], "(")[0]
					if !seen[name] && (prefix == "" || strings.HasPrefix(name, prefix)) {
						completions = append(completions, Completion{
							Label:    name,
							Kind:     "function",
							Detail:   "当前文件中的函数",
							Priority: 50,
						})
						seen[name] = true
					}
				}
			}
			// 提取变量
			if strings.Contains(line, " = ") && !strings.HasPrefix(line, "def ") && !strings.HasPrefix(line, "class ") {
				parts := strings.SplitN(line, " = ", 2)
				if len(parts) == 2 {
					name := strings.TrimSpace(parts[0])
					if !seen[name] && (prefix == "" || strings.HasPrefix(name, prefix)) {
						completions = append(completions, Completion{
							Label:    name,
							Kind:     "variable",
							Detail:   "当前作用域中的变量",
							Priority: 40,
						})
						seen[name] = true
					}
				}
			}
		}
	}

	return completions
}
