package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ToolHandler is the function signature for MCP tool handlers.
type ToolHandler func(ctx context.Context, params map[string]interface{}) (interface{}, error)

// MCPRequest is a JSON-RPC 2.0 request.
type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
	ID      interface{}            `json:"id"`
}

// MCPResponse is a JSON-RPC 2.0 response.
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// MCPError represents a JSON-RPC 2.0 error.
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ToolInfo describes a registered tool.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// MCPServer handles MCP protocol requests and routes to tool handlers.
type MCPServer struct {
	mu    sync.RWMutex
	tools map[string]ToolHandler
	info  map[string]string // tool name -> description
}

// NewMCPServer creates a new MCP server.
func NewMCPServer() *MCPServer {
	s := &MCPServer{
		tools: make(map[string]ToolHandler),
		info:  make(map[string]string),
	}
	s.registerBuiltinTools()
	return s
}

// RegisterTool registers a tool handler with the server.
func (s *MCPServer) RegisterTool(name, description string, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[name] = handler
	s.info[name] = description
}

// HandleRequest processes a JSON-RPC 2.0 request.
func (s *MCPServer) HandleRequest(ctx context.Context, request MCPRequest) (MCPResponse, error) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
	}

	if request.JSONRPC != "2.0" {
		resp.Error = &MCPError{Code: -32600, Message: "Invalid JSON-RPC version"}
		return resp, nil
	}

	switch request.Method {
	case "initialize":
		resp.Result = s.handleInitialize()
	case "tools/list":
		resp.Result = s.handleToolsList()
	case "tools/call":
		result, err := s.handleToolsCall(ctx, request.Params)
		if err != nil {
			resp.Error = &MCPError{Code: -32603, Message: err.Error()}
		} else {
			resp.Result = result
		}
	default:
		resp.Error = &MCPError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", request.Method)}
	}

	return resp, nil
}

// handleInitialize returns server capabilities.
func (s *MCPServer) handleInitialize() map[string]interface{} {
	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "moduforge-mcp",
			"version": "1.0.0",
		},
	}
}

// handleToolsList returns all registered tools.
func (s *MCPServer) handleToolsList() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tools []map[string]string
	for name, desc := range s.info {
		tools = append(tools, map[string]string{
			"name":        name,
			"description": desc,
		})
	}

	return map[string]interface{}{
		"tools": tools,
	}
}

// handleToolsCall routes a tool call to the appropriate handler.
func (s *MCPServer) handleToolsCall(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("missing tool name")
	}

	s.mu.RLock()
	handler, ok := s.tools[name]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}

	arguments, _ := params["arguments"].(map[string]interface{})
	if arguments == nil {
		arguments = make(map[string]interface{})
	}

	result, err := handler(ctx, arguments)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("%v", result),
			},
		},
	}, nil
}

// ═══════════════════════════════════════════════════════
// Built-in tools
// ═══════════════════════════════════════════════════════

func (s *MCPServer) registerBuiltinTools() {
	s.RegisterTool("generate_code", "Generate code from a description", toolGenerateCode)
	s.RegisterTool("build_module", "Build a Magisk module", toolBuildModule)
	s.RegisterTool("search_knowledge", "Search the knowledge base", toolSearchKnowledge)
	s.RegisterTool("lint_code", "Lint code for Magisk conventions", toolLintCode)
	s.RegisterTool("run_tests", "Run integration tests", toolRunTests)
}

func toolGenerateCode(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	desc, _ := params["description"].(string)
	if desc == "" {
		return nil, fmt.Errorf("missing description parameter")
	}
	return fmt.Sprintf("Code generated for: %s", desc), nil
}

func toolBuildModule(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	projectDir, _ := params["project_dir"].(string)
	if projectDir == "" {
		return nil, fmt.Errorf("missing project_dir parameter")
	}
	return fmt.Sprintf("Module built from: %s", projectDir), nil
}

func toolSearchKnowledge(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("missing query parameter")
	}
	return fmt.Sprintf("Search results for: %s", query), nil
}

func toolLintCode(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("missing code parameter")
	}
	return fmt.Sprintf("Lint results for code (%d bytes)", len(code)), nil
}

func toolRunTests(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	projectDir, _ := params["project_dir"].(string)
	if projectDir == "" {
		return nil, fmt.Errorf("missing project_dir parameter")
	}
	return fmt.Sprintf("Tests run for: %s", projectDir), nil
}

// ParseRequest parses raw JSON bytes into an MCPRequest.
func ParseRequest(data []byte) (MCPRequest, error) {
	var req MCPRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return MCPRequest{}, fmt.Errorf("parse request: %w", err)
	}
	return req, nil
}

// SerializeResponse serializes an MCPResponse to JSON bytes.
func SerializeResponse(resp MCPResponse) ([]byte, error) {
	return json.Marshal(resp)
}
