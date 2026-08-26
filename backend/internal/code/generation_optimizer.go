package code

import (
	"strings"
)

// GenerationOptimizer 代码生成优化器
type GenerationOptimizer struct{}

// NewGenerationOptimizer 创建生成优化器
func NewGenerationOptimizer() *GenerationOptimizer {
	return &GenerationOptimizer{}
}

// OptimizationSuggestion 优化建议
type OptimizationSuggestion struct {
	Type        string `json:"type"`        // template, pattern, validation, error_handling
	Title       string `json:"title"`
	Description string `json:"description"`
	Template    string `json:"template"` // 优化后的模板
}

// OptimizeGeneration 优化代码生成
func (o *GenerationOptimizer) OptimizeGeneration(code string, language string, context string) []OptimizationSuggestion {
	suggestions := make([]OptimizationSuggestion, 0)

	// 检查是否缺少错误处理
	if !strings.Contains(code, "error") && !strings.Contains(code, "err") && language == "go" {
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:        "error_handling",
			Title:       "添加错误处理",
			Description: "Go代码应该包含错误处理",
			Template:    o.generateErrorHandlingTemplate(language),
		})
	}

	// 检查是否缺少注释
	if !strings.Contains(code, "//") && !strings.Contains(code, "/*") {
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:        "template",
			Title:       "添加文档注释",
			Description: "为函数和类型添加文档注释",
			Template:    o.generateDocTemplate(language),
		})
	}

	// 检查是否缺少单元测试
	if !strings.Contains(code, "_test.go") && !strings.Contains(code, ".test.") && !strings.Contains(code, "_test.") {
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:        "pattern",
			Title:       "添加单元测试",
			Description: "为代码添加单元测试",
			Template:    o.generateTestTemplate(language),
		})
	}

	// 根据上下文提供建议
	if strings.Contains(context, "API") || strings.Contains(context, "api") {
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:        "validation",
			Title:       "添加输入验证",
			Description: "API端点应该验证输入参数",
			Template:    o.generateValidationTemplate(language),
		})
	}

	return suggestions
}

func (o *GenerationOptimizer) generateErrorHandlingTemplate(language string) string {
	switch language {
	case "go":
		return `if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}`
	case "javascript", "typescript":
		return `try {
  // operation
} catch (error) {
  console.error('Operation failed:', error);
  throw error;
}`
	case "python":
		return `try:
    # operation
except Exception as e:
    print(f"Operation failed: {e}")
    raise`
	default:
		return "// Add error handling"
	}
}

func (o *GenerationOptimizer) generateDocTemplate(language string) string {
	switch language {
	case "go":
		return `// FunctionName does something.
// It takes parameters and returns results.
func FunctionName(params) results {`
	case "javascript", "typescript":
		return `/**
 * FunctionName does something.
 * @param {type} param - description
 * @returns {type} description
 */`
	case "python":
		return `def function_name(params):
    """FunctionName does something.
    
    Args:
        param: description
    
    Returns:
        description
    """`
	default:
		return "// Add documentation"
	}
}

func (o *GenerationOptimizer) generateTestTemplate(language string) string {
	switch language {
	case "go":
		return `func TestFunctionName(t *testing.T) {
    // Arrange

    // Act

    // Assert
}`
	case "javascript", "typescript":
		return `describe('FunctionName', () => {
  it('should do something', () => {
    // Arrange

    // Act

    // Assert
  });
});`
	case "python":
		return `def test_function_name():
    # Arrange

    # Act

    # Assert
    pass`
	default:
		return "// Add tests"
	}
}

func (o *GenerationOptimizer) generateValidationTemplate(language string) string {
	switch language {
	case "go":
		return `if req.Field == "" {
    return BadRequest(c, "Field is required")
}`
	case "javascript", "typescript":
		return `if (!req.body.field) {
  return res.status(400).json({ error: 'Field is required' });
}`
	case "python":
		return `if not request.json.get('field'):
    return jsonify({'error': 'Field is required'}), 400`
	default:
		return "// Add validation"
	}
}
