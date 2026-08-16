package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockMCPServer implements a minimal MCP Streamable-HTTP server using the
// standard library, exercising initialize → tools/list → tools/call.
func mockMCPServer(t *testing.T, sse bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		method, _ := msg["method"].(string)
		w.Header().Set("Mcp-Session-Id", "sess-123")

		switch method {
		case "initialize":
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      msg["id"],
				"result": map[string]interface{}{
					"protocolVersion": ProtocolVersion,
					"capabilities":    map[string]interface{}{},
					"serverInfo":      map[string]interface{}{"name": "mock-server", "version": "1.0"},
				},
			}
			writeResp(w, resp, sse)
		case "tools/list":
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      msg["id"],
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "echo",
							"description": "Echo the input back",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"text": map[string]interface{}{"type": "string"},
								},
								"required": []interface{}{"text"},
							},
						},
					},
				},
			}
			writeResp(w, resp, sse)
		case "tools/call":
			params, _ := msg["params"].(map[string]interface{})
			name, _ := params["name"].(string)
			args, _ := params["arguments"].(map[string]interface{})
			text, _ := args["text"].(string)
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      msg["id"],
				"result": map[string]interface{}{
					"content": []map[string]interface{}{
						{"type": "text", "text": fmt.Sprintf("echo:%s:%s", name, text)},
					},
					"isError": false,
				},
			}
			writeResp(w, resp, sse)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		default:
			http.Error(w, "unknown method: "+method, http.StatusNotFound)
		}
	}))
}

func writeResp(w http.ResponseWriter, resp interface{}, sse bool) {
	data, _ := json.Marshal(resp)
	if sse {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: message\ndata: "))
		w.Write(data)
		w.Write([]byte("\n\n"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func TestClientInitializeAndCall(t *testing.T) {
	for _, sse := range []bool{false, true} {
		srv := mockMCPServer(t, sse)
		defer srv.Close()

		client := NewClient(ServerConfig{Name: "mock", URL: srv.URL})
		ctx := context.Background()
		if err := client.Initialize(ctx); err != nil {
			t.Fatalf("sse=%v Initialize: %v", sse, err)
		}
		if !client.IsReady() {
			t.Fatalf("sse=%v expected ready", sse)
		}
		if client.ServerName() != "mock-server" {
			t.Fatalf("sse=%v server name = %q", sse, client.ServerName())
		}
		tools := client.Tools()
		if len(tools) != 1 || tools[0].Name != "echo" {
			t.Fatalf("sse=%v tools = %+v", sse, tools)
		}
		if tools[0].InputSchema == nil {
			t.Fatalf("sse=%v missing inputSchema", sse)
		}

		out, err := client.CallTool(ctx, "echo", map[string]interface{}{"text": "hello"})
		if err != nil {
			t.Fatalf("sse=%v CallTool: %v", sse, err)
		}
		if out != "echo:echo:hello" {
			t.Fatalf("sse=%v CallTool output = %q", sse, out)
		}
	}
}

func TestToolSkillNamespacing(t *testing.T) {
	srv := mockMCPServer(t, false)
	defer srv.Close()

	client := NewClient(ServerConfig{Name: "github", URL: srv.URL})
	if err := client.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	skill := NewToolSkill(client, client.Tools()[0])
	if skill.Name() != "mcp__github__echo" {
		t.Fatalf("skill name = %q, want mcp__github__echo", skill.Name())
	}
	if skill.Description() == "" {
		t.Fatal("empty description")
	}
	meta := skill.Metadata()
	if !meta.Core {
		t.Fatal("MCP tools should be Core for visibility")
	}
	params := skill.Parameters()
	if params["type"] != "object" {
		t.Fatalf("params type = %v", params["type"])
	}
	// Execute through the Skill interface
	out, err := skill.Execute(context.Background(), map[string]interface{}{"text": "skill"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "echo:echo:skill" {
		t.Fatalf("skill execute = %q", out)
	}
}

func TestLoadServerConfigsFromJSON(t *testing.T) {
	raw := `[{"name":"github","url":"http://localhost:8000/mcp","headers":{"Authorization":"Bearer x"}}]`
	var cfg []ServerConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg) != 1 || cfg[0].Name != "github" || cfg[0].Headers["Authorization"] != "Bearer x" {
		t.Fatalf("parsed = %+v", cfg)
	}
}

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"get_issue":  "get_issue",
		"Hello-World": "Hello-World",
		"a b/c":      "a_b_c",
		"":           "tool",
	}
	for in, want := range cases {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseSSE(t *testing.T) {
	stream := strings.NewReader("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1}\n\n")
	data, err := parseSSE(stream, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"id":1`) {
		t.Fatalf("parsed = %s", data)
	}
}
