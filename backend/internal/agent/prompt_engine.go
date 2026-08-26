package agent

import (
	"fmt"
	"strings"
	"sync"
	"text/template"
)

// PromptTemplate represents a named prompt template.
type PromptTemplate struct {
	Name        string
	Description string
	Template    string
	Variables   []string
	Category    string // "generation", "review", "refactor", "test"
	Language    string // "go", "rust", "python", "javascript", ""
	MinQuality  float64
}

// PromptEngine manages and renders prompt templates.
type PromptEngine struct {
	mu        sync.RWMutex
	templates map[string]*PromptTemplate
}

// NewPromptEngine creates a new prompt engine with default templates.
func NewPromptEngine() *PromptEngine {
	pe := &PromptEngine{
		templates: make(map[string]*PromptTemplate),
	}
	pe.registerDefaults()
	return pe
}

// registerDefaults registers built-in prompt templates.
func (pe *PromptEngine) registerDefaults() {
	pe.Register(&PromptTemplate{
		Name:        "code_generate",
		Description: "Generate code based on specification",
		Category:    "generation",
		Template:    "你是一个资深 {{.Language}} 开发者。请根据以下需求生成高质量的代码：\n\n需求：{{.Requirement}}\n\n项目上下文：\n{{.Context}}\n\n要求：\n1. 代码必须可编译/可运行\n2. 遵循 {{.Language}} 最佳实践\n3. 包含必要的错误处理\n4. 添加关键注释\n5. 输出完整的文件内容\n\n请生成代码：",
		Variables:   []string{"Language", "Requirement", "Context"},
	})

	pe.Register(&PromptTemplate{
		Name:        "code_review",
		Description: "Review code for quality and security",
		Category:    "review",
		Template:    "你是一个资深代码审查专家。请审查以下代码：\n\n文件：{{.FilePath}}\n语言：{{.Language}}\n\n代码：\n{{.Code}}\n\n审查维度：\n1. 安全性（SQL注入、XSS、硬编码密钥等）\n2. 性能（N+1查询、内存泄漏、不必要的分配）\n3. 代码质量（命名、结构、可读性）\n4. 错误处理（是否有未处理的错误）\n5. 并发安全（竞态条件、死锁）\n\n请按严重程度排列问题，给出具体修复建议。",
		Variables:   []string{"FilePath", "Language", "Code"},
	})

	pe.Register(&PromptTemplate{
		Name:        "refactor",
		Description: "Refactor code for better structure",
		Category:    "refactor",
		Template:    "你是一个重构专家。请重构以下代码以提升质量：\n\n文件：{{.FilePath}}\n语言：{{.Language}}\n重构目标：{{.Goal}}\n\n当前代码：\n{{.Code}}\n\n要求：\n1. 保持功能不变\n2. 改善代码结构和可读性\n3. 减少重复代码\n4. 提升可测试性\n5. 输出完整的重构后代码",
		Variables:   []string{"FilePath", "Language", "Goal", "Code"},
	})

	pe.Register(&PromptTemplate{
		Name:        "test_generate",
		Description: "Generate unit tests",
		Category:    "test",
		Template:    "你是测试工程师。请为以下函数生成单元测试：\n\n函数：{{.FunctionName}}\n文件：{{.FilePath}}\n语言：{{.Language}}\n\n函数代码：\n{{.FunctionCode}}\n\n要求：\n1. 覆盖正常路径和边界情况\n2. 覆盖错误条件\n3. 使用 {{.Language}} 标准测试框架\n4. 测试名称清晰描述测试场景\n5. 每个测试用例独立可运行\n\n请生成测试代码：",
		Variables:   []string{"FunctionName", "FilePath", "Language", "FunctionCode"},
	})

	pe.Register(&PromptTemplate{
		Name:        "error_fix",
		Description: "Fix build or runtime errors",
		Category:    "generation",
		Template:    "你是一个调试专家。请修复以下错误：\n\n错误信息：\n{{.Error}}\n\n相关代码：\n{{.Code}}\n\n文件：{{.FilePath}}\n语言：{{.Language}}\n\n要求：\n1. 精确定位错误根因\n2. 给出最小化修复\n3. 解释错误原因\n4. 输出修复后的完整文件内容\n\n请修复代码：",
		Variables:   []string{"Error", "Code", "FilePath", "Language"},
	})

	pe.Register(&PromptTemplate{
		Name:        "architecture_design",
		Description: "Design system architecture",
		Category:    "generation",
		Template:    "你是一个系统架构师。请为以下需求设计架构：\n\n需求：{{.Requirement}}\n技术栈：{{.TechStack}}\n约束条件：{{.Constraints}}\n\n要求：\n1. 给出模块划分和职责\n2. 定义模块间接口\n3. 考虑可扩展性和可维护性\n4. 给出关键数据结构\n5. 标注需要特别注意的设计决策\n\n请输出架构设计文档：",
		Variables:   []string{"Requirement", "TechStack", "Constraints"},
	})

	pe.Register(&PromptTemplate{
		Name:        "documentation",
		Description: "Generate documentation",
		Category:    "generation",
		Template:    "你是技术文档专家。请为以下代码生成文档：\n\n文件：{{.FilePath}}\n语言：{{.Language}}\n\n代码：\n{{.Code}}\n\n要求：\n1. 为每个公开函数/方法编写文档注释\n2. 说明函数的用途、参数、返回值\n3. 给出使用示例\n4. 标注可能的异常情况\n5. 使用 {{.Language}} 标准文档格式\n\n请生成文档：",
		Variables:   []string{"FilePath", "Language", "Code"},
	})
}

// Register adds a template to the engine.
func (pe *PromptEngine) Register(t *PromptTemplate) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.templates[t.Name] = t
}

// Get returns a template by name.
func (pe *PromptEngine) Get(name string) (*PromptTemplate, bool) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	t, ok := pe.templates[name]
	return t, ok
}

// ListByCategory returns templates filtered by category.
func (pe *PromptEngine) ListByCategory(category string) []*PromptTemplate {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	var result []*PromptTemplate
	for _, t := range pe.templates {
		if category == "" || t.Category == category {
			result = append(result, t)
		}
	}
	return result
}

// Render renders a template with the given variables.
func (pe *PromptEngine) Render(name string, vars map[string]string) (string, error) {
	pe.mu.RLock()
	t, ok := pe.templates[name]
	pe.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("template %q not found", name)
	}

	// Parse and execute template
	tmpl, err := template.New(name).Parse(t.Template)
	if err != nil {
		return "", fmt.Errorf("parse template %q: %w", name, err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("execute template %q: %w", name, err)
	}

	return buf.String(), nil
}

// RenderWithLanguage is a convenience method that sets the language variable.
func (pe *PromptEngine) RenderWithLanguage(name, language string, vars map[string]string) (string, error) {
	if vars == nil {
		vars = make(map[string]string)
	}
	vars["Language"] = language
	return pe.Render(name, vars)
}

// SmartSelect selects the best template for a given task description.
func (pe *PromptEngine) SmartSelect(taskDescription string) string {
	task := strings.ToLower(taskDescription)

	// Keyword-based template selection
	if strings.Contains(task, "修复") || strings.Contains(task, "fix") || strings.Contains(task, "错误") || strings.Contains(task, "error") {
		return "error_fix"
	}
	if strings.Contains(task, "测试") || strings.Contains(task, "test") {
		return "test_generate"
	}
	if strings.Contains(task, "审查") || strings.Contains(task, "review") || strings.Contains(task, "检查") {
		return "code_review"
	}
	if strings.Contains(task, "重构") || strings.Contains(task, "refactor") {
		return "refactor"
	}
	if strings.Contains(task, "架构") || strings.Contains(task, "设计") || strings.Contains(task, "design") {
		return "architecture_design"
	}
	if strings.Contains(task, "文档") || strings.Contains(task, "doc") {
		return "documentation"
	}
	// Default: code generation
	return "code_generate"
}

// GetStats returns template usage statistics.
func (pe *PromptEngine) GetStats() map[string]int {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	stats := make(map[string]int)
	for _, t := range pe.templates {
		stats[t.Category]++
	}
	return stats
}
