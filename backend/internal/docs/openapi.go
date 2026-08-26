package docs

import (
	"encoding/json"
)

// OpenAPISpec OpenAPI规范
type OpenAPISpec struct {
	OpenAPI    string              `json:"openapi"`
	Info       Info                `json:"info"`
	Servers    []Server            `json:"servers"`
	Paths      map[string]PathItem `json:"paths"`
	Components Components          `json:"components"`
}

// Info API信息
type Info struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

// Server 服务器信息
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

// PathItem 路径项
type PathItem map[string]Operation

// Operation 操作
type Operation struct {
	Summary     string              `json:"summary"`
	Description string              `json:"description"`
	Tags        []string            `json:"tags"`
	OperationID string              `json:"operationId"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses"`
}

// Parameter 参数
type Parameter struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required"`
	Schema   Schema `json:"schema"`
}

// RequestBody 请求体
type RequestBody struct {
	Required bool                 `json:"required"`
	Content  map[string]MediaType `json:"content"`
}

// Response 响应
type Response struct {
	Description string `json:"description"`
}

// MediaType 媒体类型
type MediaType struct {
	Schema Schema `json:"schema"`
}

// Schema 模式
type Schema struct {
	Type       string            `json:"type"`
	Properties map[string]Schema `json:"properties,omitempty"`
}

// Components 组件
type Components struct {
	Schemas map[string]Schema `json:"schemas"`
}

// GenerateModuForgeDocs 生成ModuForge API文档
func GenerateModuForgeDocs() *OpenAPISpec {
	spec := &OpenAPISpec{
		OpenAPI: "3.0.3",
		Info: Info{
			Title:       "ModuForge API",
			Description: "ModuForge AI代码生成平台API文档",
			Version:     "1.0.0",
		},
		Servers: []Server{
			{URL: "http://192.168.2.9:8086", Description: "生产服务器"},
		},
		Paths: make(map[string]PathItem),
		Components: Components{
			Schemas: make(map[string]Schema),
		},
	}

	// AI 端点
	spec.addPath("/api/ai/chat", "POST", "发送AI聊天消息", "AI聊天", "sendChatMessage")
	spec.addPath("/api/ai/generate", "POST", "生成代码", "AI生成", "generateCode")
	spec.addPath("/api/ai/repair", "POST", "修复代码", "AI修复", "repairCode")

	// 性能监控
	spec.addPath("/api/perf/summary", "GET", "获取性能摘要", "性能监控", "getPerfSummary")
	spec.addPath("/api/perf/history", "GET", "获取历史快照", "性能监控", "getPerfHistory")
	spec.addPath("/api/perf/models", "GET", "获取模型统计", "性能监控", "getModelStats")

	// 架构分析
	spec.addPath("/api/arch/analyze", "POST", "分析项目架构", "架构分析", "analyzeArch")

	// Git 操作
	spec.addPath("/api/git/status", "GET", "获取Git状态", "Git操作", "getGitStatus")
	spec.addPath("/api/git/log", "GET", "获取提交历史", "Git操作", "getGitLog")
	spec.addPath("/api/git/commit", "POST", "创建提交", "Git操作", "gitCommit")
	spec.addPath("/api/git/rollback", "POST", "回滚提交", "Git操作", "gitRollback")

	// 缓存
	spec.addPath("/api/cache/stats", "GET", "获取缓存统计", "缓存管理", "getCacheStats")
	spec.addPath("/api/cache/clear", "POST", "清除缓存", "缓存管理", "clearCache")

	// 反馈
	spec.addPath("/api/feedback", "POST", "提交反馈", "用户反馈", "submitFeedback")
	spec.addPath("/api/feedback/stats", "GET", "获取反馈统计", "用户反馈", "getFeedbackStats")

	// 代码质量
	spec.addPath("/api/quality/validate", "POST", "验证代码质量", "代码质量", "validateCode")

	// 多模型协同
	spec.addPath("/api/ensemble/generate", "POST", "多模型协同生成", "多模型协同", "ensembleGenerate")

	return spec
}

func (spec *OpenAPISpec) addPath(path, method, summary, tag, opID string) {
	if spec.Paths[path] == nil {
		spec.Paths[path] = make(PathItem)
	}

	spec.Paths[path][method] = Operation{
		Summary:     summary,
		Tags:        []string{tag},
		OperationID: opID,
		Responses: map[string]Response{
			"200": {Description: "成功"},
			"400": {Description: "请求错误"},
			"401": {Description: "未授权"},
			"500": {Description: "服务器错误"},
		},
	}
}

// ToJSON 转换为JSON
func (spec *OpenAPISpec) ToJSON() ([]byte, error) {
	return json.MarshalIndent(spec, "", "  ")
}

// ToYAML 转换为YAML (简化版)
func (spec *OpenAPISpec) ToYAML() string {
	jsonBytes, _ := spec.ToJSON()
	return string(jsonBytes)
}
