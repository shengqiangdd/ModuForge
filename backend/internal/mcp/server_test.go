package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

func TestNewMCPServer(t *testing.T) {
	s := NewMCPServer()
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestHandleRequest_Initialize(t *testing.T) {
	s := NewMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      1,
	}

	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocol version 2024-11-05, got %v", result["protocolVersion"])
	}
}

func TestHandleRequest_ToolsList(t *testing.T) {
	s := NewMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      2,
	}

	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	tools, ok := result["tools"].([]map[string]string)
	if !ok {
		t.Fatal("expected tools array")
	}

	if len(tools) < 5 {
		t.Errorf("expected at least 5 tools, got %d", len(tools))
	}
}

func TestHandleRequest_ToolsCall(t *testing.T) {
	s := NewMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "search_knowledge",
			"arguments": map[string]interface{}{
				"query": "battery monitor",
			},
		},
		ID: 3,
	}

	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %s", resp.Error.Message)
	}

	if resp.Result == nil {
		t.Error("expected non-nil result")
	}
}

func TestHandleRequest_UnknownMethod(t *testing.T) {
	s := NewMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "unknown/method",
		ID:      4,
	}

	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}

	if resp.Error == nil {
		t.Error("expected error for unknown method")
	}

	if resp.Error.Code != -32601 {
		t.Errorf("expected code -32601, got %d", resp.Error.Code)
	}
}

func TestHandleRequest_UnknownTool(t *testing.T) {
	s := NewMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "nonexistent_tool",
		},
		ID: 5,
	}

	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}

	if resp.Error == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestHandleRequest_InvalidJSONRPC(t *testing.T) {
	s := NewMCPServer()

	req := MCPRequest{
		JSONRPC: "1.0",
		Method:  "initialize",
		ID:      6,
	}

	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}

	if resp.Error == nil {
		t.Error("expected error for invalid JSON-RPC version")
	}
}

func TestRegisterTool(t *testing.T) {
	s := NewMCPServer()

	called := false
	s.RegisterTool("custom_tool", "A custom tool", func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		called = true
		return "custom result", nil
	})

	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "custom_tool",
			"arguments": map[string]interface{}{},
		},
		ID: 7,
	}

	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %s", resp.Error.Message)
	}

	if !called {
		t.Error("custom tool handler was not called")
	}
}

func TestParseRequest(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","method":"test","id":1}`)
	req, err := ParseRequest(data)
	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}

	if req.Method != "test" {
		t.Errorf("expected method test, got %s", req.Method)
	}
}

func TestParseRequest_Invalid(t *testing.T) {
	_, err := ParseRequest([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSerializeResponse(t *testing.T) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		Result:  "ok",
		ID:      1,
	}

	data, err := SerializeResponse(resp)
	if err != nil {
		t.Fatalf("SerializeResponse failed: %v", err)
	}

	var parsed MCPResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed.Result != "ok" {
		t.Errorf("expected ok, got %v", parsed.Result)
	}
}

func TestToolGenerateCode(t *testing.T) {
	s := NewMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "generate_code",
			"arguments": map[string]interface{}{
				"description": "battery monitor",
			},
		},
		ID: 8,
	}

	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %s", resp.Error.Message)
	}
}

func TestToolGenerateCode_MissingParam(t *testing.T) {
	s := NewMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "generate_code",
			"arguments": map[string]interface{}{},
		},
		ID: 9,
	}

	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}

	if resp.Error == nil {
		t.Error("expected error for missing description")
	}
}
