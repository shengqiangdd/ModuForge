package code

import (
	"regexp"
	"strings"
)

// APIDocGenerator API文档生成器
type APIDocGenerator struct{}

// NewAPIDocGenerator 创建API文档生成器
func NewAPIDocGenerator() *APIDocGenerator {
	return &APIDocGenerator{}
}

// APIDocRequest API文档请求
type APIDocRequest struct {
	Code     string `json:"code"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Version  string `json:"version"`
}

// APIDocResult API文档结果
type APIDocResult struct {
	Title       string        `json:"title"`
	Version     string        `json:"version"`
	Description string        `json:"description"`
	Endpoints   []APIEndpoint `json:"endpoints"`
	Schemas     []APISchema   `json:"schemas"`
}

// APIEndpoint API端点
type APIEndpoint struct {
	Method      string       `json:"method"`
	Path        string       `json:"path"`
	Description string       `json:"description"`
	Parameters  []APIParam   `json:"parameters"`
	RequestBody *APISchema   `json:"request_body,omitempty"`
	Responses   []APIResponse `json:"responses"`
}

// APIParam API参数
type APIParam struct {
	Name        string `json:"name"`
	In          string `json:"in"` // path, query, header
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// APIResponse API响应
type APIResponse struct {
	StatusCode  int        `json:"status_code"`
	Description string     `json:"description"`
	Schema      *APISchema `json:"schema,omitempty"`
}

// APISchema API模式
type APISchema struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties,omitempty"`
}

// GenerateDoc 生成API文档
func (g *APIDocGenerator) GenerateDoc(req APIDocRequest) *APIDocResult {
	result := &APIDocResult{
		Title:       req.Title,
		Version:     req.Version,
		Description: "Auto-generated API documentation",
		Endpoints:   make([]APIEndpoint, 0),
		Schemas:     make([]APISchema, 0),
	}

	lines := strings.Split(req.Code, "\n")

	// 提取函数注释
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 检测路由定义
		if strings.Contains(trimmed, "Get(") || strings.Contains(trimmed, "Post(") ||
			strings.Contains(trimmed, "Put(") || strings.Contains(trimmed, "Delete(") {
			endpoint := g.parseRoute(lines, i, trimmed)
			if endpoint != nil {
				result.Endpoints = append(result.Endpoints, *endpoint)
			}
		}

		// 检测类型定义
		if strings.HasPrefix(trimmed, "type ") && strings.Contains(trimmed, "struct") {
			schema := g.parseStruct(lines, i)
			if schema != nil {
				result.Schemas = append(result.Schemas, *schema)
			}
		}
	}

	return result
}

// parseRoute 解析路由
func (g *APIDocGenerator) parseRoute(lines []string, index int, line string) *APIEndpoint {
	// 提取HTTP方法
	method := ""
	if strings.Contains(line, "Get(") {
		method = "GET"
	} else if strings.Contains(line, "Post(") {
		method = "POST"
	} else if strings.Contains(line, "Put(") {
		method = "PUT"
	} else if strings.Contains(line, "Delete(") {
		method = "DELETE"
	}

	// 提取路径
	pathRegex := regexp.MustCompile(`"([^"]+)"`)
	matches := pathRegex.FindStringSubmatch(line)
	if len(matches) < 2 {
		return nil
	}
	path := matches[1]

	// 从注释中提取描述
	description := ""
	if index > 0 {
		comment := strings.TrimSpace(lines[index-1])
		if strings.HasPrefix(comment, "//") {
			description = strings.TrimPrefix(comment, "//")
			description = strings.TrimSpace(description)
		}
	}

	return &APIEndpoint{
		Method:      method,
		Path:        path,
		Description: description,
		Parameters:  make([]APIParam, 0),
		Responses:   make([]APIResponse, 0),
	}
}

// parseStruct 解析结构体
func (g *APIDocGenerator) parseStruct(lines []string, index int) *APISchema {
	if index >= len(lines) {
		return nil
	}

	line := lines[index]
	nameRegex := regexp.MustCompile(`type\s+(\w+)\s+struct`)
	matches := nameRegex.FindStringSubmatch(line)
	if len(matches) < 2 {
		return nil
	}

	schema := &APISchema{
		Name:       matches[1],
		Type:       "object",
		Properties: make(map[string]string),
	}

	// 解析字段
	for i := index + 1; i < len(lines) && i < index+20; i++ {
		fieldLine := strings.TrimSpace(lines[i])
		if strings.HasPrefix(fieldLine, "}") {
			break
		}

		fieldRegex := regexp.MustCompile(`(\w+)\s+(\w+)`)
		fieldMatches := fieldRegex.FindStringSubmatch(fieldLine)
		if len(fieldMatches) >= 3 {
			schema.Properties[fieldMatches[1]] = fieldMatches[2]
		}
	}

	return schema
}
