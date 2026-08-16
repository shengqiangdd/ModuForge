// Package mcp provides a lightweight Model Context Protocol (MCP) client
// using only the Go standard library. It implements the Streamable HTTP
// transport (MCP 2025-03-26 spec) — the same transport used by Claude Code,
// OpenCode, Cline and other mainstream AI agents to connect to external tool
// servers (filesystem, GitHub, database, browser, etc.).
//
// Design goals:
//   - No third-party dependencies (works in the slim Alpine container)
//   - JSON-RPC 2.0 over HTTP with SSE response parsing
//   - Session-id header persistence for stateful MCP servers
//   - tools/list + tools/call support (the two endpoints agents need)
//
// Integration: MCPManager loads server configs, then wraps each remote tool
// in a registry.Skill so the AgentRunner exposes them like native skills.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ProtocolVersion is the MCP spec revision we negotiate.
const ProtocolVersion = "2025-03-26"

// ClientInfo identifies us to the MCP server.
var ClientInfo = map[string]interface{}{
	"name":    "moduforge",
	"version": "1.0",
}

// Tool mirrors the MCP tools/list entry.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Client is a single MCP server connection (Streamable HTTP transport).
type Client struct {
	mu sync.Mutex

	Name    string // logical name, e.g. "github"
	URL     string // MCP endpoint, e.g. http://localhost:8080/mcp
	Headers map[string]string // extra headers (Authorization, etc.)

	http       *http.Client
	sessionID  string // Mcp-Session-Id returned by the server
	protocolV  string // negotiated protocol version
	serverName string
	tools      []Tool
	ready      bool

	// Diagnostics (read-only after Initialize; guarded by mu)
	connectedAt time.Time
	lastError   string
}

// ServerConfig is the JSON shape of one MCP server in the config file/env.
type ServerConfig struct {
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	// TimeoutSeconds overrides the default per-request timeout (default 30).
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
}

// NewClient creates a client for one MCP server config.
func NewClient(cfg ServerConfig) *Client {
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}
	headers := map[string]string{}
	for k, v := range cfg.Headers {
		headers[k] = v
	}
	return &Client{
		Name:    cfg.Name,
		URL:     cfg.URL,
		Headers: headers,
		http: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

// IsReady reports whether the client completed the handshake.
func (c *Client) IsReady() bool { return c.ready }

// ServerName returns the negotiated server name (for logging).
func (c *Client) ServerName() string { return c.serverName }

// Tools returns the discovered tools (only valid after Initialize).
func (c *Client) Tools() []Tool { return c.tools }

// ConnectedAt returns when the client last completed a handshake.
func (c *Client) ConnectedAt() time.Time { return c.connectedAt }

// LastError returns the most recent initialize error (empty if ready).
func (c *Client) LastError() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastError
}

// Initialize performs the MCP handshake: initialize → initialized
// notification → tools/list. It must be called before CallTool.
func (c *Client) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ready {
		return nil
	}

	// 1. initialize
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]interface{}{},
			"clientInfo":      ClientInfo,
		},
	}
	var initResult struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  *struct {
			ProtocolVersion string `json:"protocolVersion"`
			Capabilities    map[string]interface{} `json:"capabilities"`
			ServerInfo      map[string]interface{} `json:"serverInfo"`
		} `json:"result"`
		Error *JSONRPCError `json:"error"`
	}
	body, err := c.doRequest(ctx, initReq, nil)
	if err != nil {
		return fmt.Errorf("mcp[%s] initialize: %w", c.Name, err)
	}
	if err := json.Unmarshal(body, &initResult); err != nil {
		return fmt.Errorf("mcp[%s] initialize: bad response: %w", c.Name, err)
	}
	if initResult.Error != nil {
		return fmt.Errorf("mcp[%s] initialize error: %s", c.Name, initResult.Error.Message)
	}
	if initResult.Result == nil {
		return fmt.Errorf("mcp[%s] initialize: empty result", c.Name)
	}
	c.protocolV = initResult.Result.ProtocolVersion
	if name, ok := initResult.Result.ServerInfo["name"].(string); ok {
		c.serverName = name
	}
	if c.protocolV == "" {
		c.protocolV = ProtocolVersion
	}

	// 2. notifications/initialized (fire and forget, no response body)
	notif := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params":  map[string]interface{}{},
	}
	_, _ = c.doRequest(ctx, notif, nil)

	// 3. tools/list
	toolsReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}
	var toolsResult struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  *struct {
			Tools []Tool `json:"tools"`
		} `json:"result"`
		Error *JSONRPCError `json:"error"`
	}
	body, err = c.doRequest(ctx, toolsReq, nil)
	if err != nil {
		return fmt.Errorf("mcp[%s] tools/list: %w", c.Name, err)
	}
	if err := json.Unmarshal(body, &toolsResult); err != nil {
		return fmt.Errorf("mcp[%s] tools/list: bad response: %w", c.Name, err)
	}
	if toolsResult.Error != nil {
		return fmt.Errorf("mcp[%s] tools/list error: %s", c.Name, toolsResult.Error.Message)
	}
	if toolsResult.Result != nil {
		c.tools = toolsResult.Result.Tools
	}

	c.ready = true
	c.connectedAt = time.Now()
	c.lastError = ""
	slog.Info("MCP server ready", "name", c.Name, "url", c.URL, "server", c.serverName, "protocol", c.protocolV, "tools", len(c.tools))
	return nil
}

// setError records a failed handshake so diagnostics can surface it.
func (c *Client) setError(err error) {
	c.ready = false
	c.tools = nil
	if err != nil {
		c.lastError = err.Error()
	} else {
		c.lastError = "unknown error"
	}
}

// JSONRPCError is a JSON-RPC 2.0 error object.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// CallTool invokes a remote MCP tool and returns the text content.
// If the server returns isError=true, the content is returned with an error
// so the Agent can read the failure detail.
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.ready {
		return "", fmt.Errorf("mcp[%s] not initialized", c.Name)
	}

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": arguments,
		},
	}
	var result struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  *struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *JSONRPCError `json:"error"`
	}

	body, err := c.doRequest(ctx, req, nil)
	if err != nil {
		return "", fmt.Errorf("mcp[%s] call %s: %w", c.Name, name, err)
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("mcp[%s] call %s: bad response: %w", c.Name, name, err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("mcp[%s] call %s error: %s", c.Name, name, result.Error.Message)
	}
	if result.Result == nil {
		return "", fmt.Errorf("mcp[%s] call %s: empty result", c.Name, name)
	}

	// Concatenate text content blocks (resources/images are skipped)
	var sb strings.Builder
	for _, block := range result.Result.Content {
		if block.Type == "text" && block.Text != "" {
			sb.WriteString(block.Text)
			sb.WriteString("\n")
		}
	}
	text := strings.TrimRight(sb.String(), "\n")
	if text == "" {
		text = fmt.Sprintf("(no text content returned; isError=%v)", result.Result.IsError)
	}
	if result.Result.IsError {
		return text, fmt.Errorf("mcp[%s] %s returned error: %s", c.Name, name, text)
	}
	return text, nil
}

// doRequest sends a JSON-RPC message and returns the raw response body.
// It handles both plain JSON and SSE (text/event-stream) responses.
func (c *Client) doRequest(ctx context.Context, payload map[string]interface{}, sseProgress func(string)) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Some servers return 202 Accepted for notifications (no body)
		if resp.StatusCode == http.StatusAccepted {
			return nil, nil
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// Persist session id if provided
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.sessionID = sid
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return parseSSE(resp.Body, sseProgress)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 4MB cap
}

// parseSSE reads an SSE stream and returns the last event's data.
// MCP servers send "event: message" lines with a JSON "data:" payload.
func parseSSE(r io.Reader, progress func(string)) ([]byte, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4<<20)
	var lastData []byte
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" {
				continue
			}
			// A data line is usually the final JSON-RPC response;
			// text/event-stream framing uses "event: message" first.
			lastData = []byte(payload)
			if progress != nil {
				progress(payload)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(lastData) == 0 {
		return nil, fmt.Errorf("empty SSE stream")
	}
	return lastData, nil
}

// Ping sends a ping to keep the session alive (optional).
func (c *Client) Ping(ctx context.Context) error {
	ping := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      99,
		"method":  "ping",
		"params":  map[string]interface{}{},
	}
	_, err := c.doRequest(ctx, ping, nil)
	return err
}
