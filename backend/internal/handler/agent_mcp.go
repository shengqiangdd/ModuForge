package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/agent/mcp"
)

// ListMCPStatus returns the connected MCP servers and their tool counts.
func (h *AgentHandler) ListMCPStatus(c fiber.Ctx) error {
	if h.mcpMgr == nil {
		return c.JSON(fiber.Map{"servers": []interface{}{}})
	}
	var servers []map[string]interface{}
	for _, cli := range h.mcpMgr.Clients() {
		tools := cli.Tools()
		toolInfos := make([]map[string]interface{}, 0, len(tools))
		for _, t := range tools {
			toolInfos = append(toolInfos, map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": t.InputSchema,
				"writes":      mcp.IsWriteTool(t),
			})
		}
		servers = append(servers, map[string]interface{}{
			"name":         cli.Name,
			"url":          cli.URL,
			"server_name":  cli.ServerName(),
			"ready":        cli.IsReady(),
			"tool_count":   len(tools),
			"tools":        toolInfos,
			"connected_at": cli.ConnectedAt().Format(time.RFC3339),
			"last_error":   cli.LastError(),
		})
	}
	if servers == nil {
		servers = []map[string]interface{}{}
	}
	return c.JSON(fiber.Map{"servers": servers})
}

// TestMCPTool calls a tool on an MCP server with the given arguments.
// Request: {"server":"github","tool":"get_issue","arguments":{...}}
func (h *AgentHandler) TestMCPTool(c fiber.Ctx) error {
	var req struct {
		Server    string                 `json:"server"`
		Tool      string                 `json:"tool"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}
	if h.mcpMgr == nil {
		return c.Status(404).JSON(fiber.Map{"error": "MCP not configured"})
	}
	cli, ok := h.mcpMgr.Get(req.Server)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"error": "MCP server not found: " + req.Server})
	}
	if !cli.IsReady() {
		return c.Status(503).JSON(fiber.Map{"error": "MCP server not ready: " + req.Server})
	}
	ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
	defer cancel()
	result, err := cli.CallTool(ctx, req.Tool, req.Arguments)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "tool call failed: " + err.Error()})
	}
	return c.JSON(fiber.Map{"server": req.Server, "tool": req.Tool, "result": result})
}

// ---------- MCP server configuration management (UI/API, persisted in DB) ----------

// mcpServerRow is the DB representation of a managed MCP server.
type mcpServerRow struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Headers string `json:"headers"` // JSON object string
	Enabled bool   `json:"enabled"`
}

// loadMCPServersFromDB connects to all enabled servers stored in the DB.
func loadMCPServersFromDB(ctx context.Context, db *sql.DB, mgr *mcp.Manager) {
	rows, err := db.QueryContext(ctx, `SELECT id, name, url, headers, enabled FROM mcp_servers WHERE enabled=1 ORDER BY name`)
	if err != nil {
		slog.Warn("MCP: cannot load servers from DB", "error", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var row mcpServerRow
		if err := rows.Scan(&row.ID, &row.Name, &row.URL, &row.Headers, &row.Enabled); err != nil {
			continue
		}
		cfg, err := rowToServerConfig(row)
		if err != nil {
			slog.Warn("MCP: skip server with invalid headers", "name", row.Name, "error", err)
			continue
		}
		mgr.AddServer(ctx, cfg)
	}
}

// rowToServerConfig converts a DB row into an mcp.ServerConfig.
func rowToServerConfig(row mcpServerRow) (mcp.ServerConfig, error) {
	var headers map[string]string
	if row.Headers == "" || row.Headers == "{}" {
		headers = map[string]string{}
	} else {
		if err := json.Unmarshal([]byte(row.Headers), &headers); err != nil {
			return mcp.ServerConfig{}, fmt.Errorf("invalid headers JSON: %w", err)
		}
	}
	return mcp.ServerConfig{
		Name:    row.Name,
		URL:     row.URL,
		Headers: headers,
	}, nil
}

// ListMCPServers returns DB-managed servers merged with env-configured ones
// and live runtime status.
// Response: {"servers":[{"id":1,"name":"github","url":"...","headers":{...},"enabled":true,
//
//	"managed":true,"ready":true,"tool_count":42,"last_error":"","connected_at":"...","tools":[...]}]}
func (h *AgentHandler) ListMCPServers(c fiber.Ctx) error {
	rows, err := h.db.Conn.Query(`SELECT id, name, url, headers, enabled FROM mcp_servers ORDER BY name`)
	if err != nil {
		return InternalError(c, err.Error())
	}
	defer rows.Close()

	servers := []map[string]interface{}{}
	seen := map[string]bool{}
	for rows.Next() {
		var row mcpServerRow
		if err := rows.Scan(&row.ID, &row.Name, &row.URL, &row.Headers, &row.Enabled); err != nil {
			continue
		}
		seen[row.Name] = true
		info := map[string]interface{}{
			"id":         row.ID,
			"name":       row.Name,
			"url":        row.URL,
			"enabled":    row.Enabled,
			"managed":    true,
			"ready":      false,
			"tool_count": 0,
			"last_error": "",
			"tools":      []map[string]interface{}{},
		}
		var headers map[string]string
		_ = json.Unmarshal([]byte(row.Headers), &headers)
		if headers == nil {
			headers = map[string]string{}
		}
		info["headers"] = headers

		if cli, ok := h.mcpMgr.Get(row.Name); ok {
			info["ready"] = cli.IsReady()
			info["tool_count"] = len(cli.Tools())
			info["server_name"] = cli.ServerName()
			info["last_error"] = cli.LastError()
			info["connected_at"] = cli.ConnectedAt().Format(time.RFC3339)
			toolInfos := make([]map[string]interface{}, 0, len(cli.Tools()))
			for _, t := range cli.Tools() {
				toolInfos = append(toolInfos, map[string]interface{}{
					"name":        t.Name,
					"description": t.Description,
					"inputSchema": t.InputSchema,
					"writes":      mcp.IsWriteTool(t),
				})
			}
			info["tools"] = toolInfos
		} else if !row.Enabled {
			info["last_error"] = "已禁用（enabled=false）"
		} else {
			info["last_error"] = "未连接（服务启动时可能失败）"
		}
		servers = append(servers, info)
	}

	// Merge env-configured servers (MCP_SERVERS / MCP_SERVERS_FILE) that are
	// not DB-managed, so the UI shows every live server. These are read-only
	// from the UI perspective (managed=false).
	for _, cli := range h.mcpMgr.Clients() {
		if seen[cli.Name] {
			continue
		}
		toolInfos := make([]map[string]interface{}, 0, len(cli.Tools()))
		for _, t := range cli.Tools() {
			toolInfos = append(toolInfos, map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": t.InputSchema,
				"writes":      mcp.IsWriteTool(t),
			})
		}
		servers = append(servers, map[string]interface{}{
			"id":         0,
			"name":       cli.Name,
			"url":        cli.URL,
			"headers":    cli.Headers,
			"enabled":    true,
			"managed":    false,
			"ready":      cli.IsReady(),
			"tool_count": len(cli.Tools()),
			"server_name": cli.ServerName(),
			"last_error": cli.LastError(),
			"connected_at": cli.ConnectedAt().Format(time.RFC3339),
			"tools":      toolInfos,
		})
	}
	if servers == nil {
		servers = []map[string]interface{}{}
	}
	return c.JSON(fiber.Map{"servers": servers})
}

// AddMCPServer creates a new MCP server config and hot-connects to it.
// Body: {"name":"github","url":"http://.../mcp","headers":{"Authorization":"Bearer x"},"enabled":true}
func (h *AgentHandler) AddMCPServer(c fiber.Ctx) error {
	var req struct {
		Name    string            `json:"name"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		Enabled *bool             `json:"enabled"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.URL) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name and url are required"})
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	headersJSON, _ := json.Marshal(req.Headers)
	if _, err := h.db.Conn.Exec(
		`INSERT INTO mcp_servers (name, url, headers, enabled) VALUES (?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET url=excluded.url, headers=excluded.headers, enabled=excluded.enabled, updated_at=CURRENT_TIMESTAMP`,
		req.Name, req.URL, string(headersJSON), boolToInt(enabled),
	); err != nil {
		return InternalError(c, err.Error())
	}

	client, err := h.mcpMgr.AddServer(c.Context(), mcp.ServerConfig{Name: req.Name, URL: req.URL, Headers: req.Headers})
	if err != nil {
		slog.Warn("MCP: add server connect failed", "name", req.Name, "error", err)
	}
	// Tool set may have changed → invalidate LLM tool-definition cache.
	h.runner.InvalidateToolCache()
	return c.JSON(fiber.Map{"server": req.Name, "ready": client.IsReady(), "last_error": client.LastError()})
}

// UpdateMCPServer updates an existing server config and reconnects.
func (h *AgentHandler) UpdateMCPServer(c fiber.Ctx) error {
	name := c.Params("name")
	var req struct {
		Name    string            `json:"name"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		Enabled *bool             `json:"enabled"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}
	// Rename support: when body.name differs, update the row name too.
	newName := req.Name
	if newName == "" {
		newName = name
	}
	if strings.TrimSpace(newName) == "" || strings.TrimSpace(req.URL) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name and url are required"})
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	headersJSON, _ := json.Marshal(req.Headers)

	res, err := h.db.Conn.Exec(
		`UPDATE mcp_servers SET name=?, url=?, headers=?, enabled=?, updated_at=CURRENT_TIMESTAMP WHERE name=?`,
		newName, req.URL, string(headersJSON), boolToInt(enabled), name,
	)
	if err != nil {
		return InternalError(c, err.Error())
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "server not found: " + name})
	}

	// Hot-reconnect: drop old client, connect the new config (if enabled).
	h.mcpMgr.RemoveServer(name)
	ready := false
	lastErr := ""
	if enabled {
		client, err := h.mcpMgr.AddServer(c.Context(), mcp.ServerConfig{Name: newName, URL: req.URL, Headers: req.Headers})
		ready = client.IsReady()
		lastErr = client.LastError()
		if err != nil {
			slog.Warn("MCP: update server connect failed", "name", newName, "error", err)
		}
	} else {
		// Disabled: leave disconnected; DB row keeps enabled=0.
		lastErr = "已禁用（enabled=false）"
	}
	h.runner.InvalidateToolCache()
	return c.JSON(fiber.Map{"server": newName, "ready": ready, "last_error": lastErr})
}

// DeleteMCPServer removes a server config and disconnects it.
func (h *AgentHandler) DeleteMCPServer(c fiber.Ctx) error {
	name := c.Params("name")
	res, err := h.db.Conn.Exec(`DELETE FROM mcp_servers WHERE name=?`, name)
	if err != nil {
		return InternalError(c, err.Error())
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "server not found: " + name})
	}
	h.mcpMgr.RemoveServer(name)
	h.runner.InvalidateToolCache()
	return c.JSON(fiber.Map{"deleted": name})
}

// ReconnectMCPServer re-runs the handshake for an existing server.
func (h *AgentHandler) ReconnectMCPServer(c fiber.Ctx) error {
	name := c.Params("name")
	client, err := h.mcpMgr.Reconnect(c.Context(), name)
	if err != nil {
		if client != nil {
			return c.Status(502).JSON(fiber.Map{"error": err.Error(), "ready": false, "last_error": client.LastError()})
		}
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	h.runner.InvalidateToolCache()
	return c.JSON(fiber.Map{"server": name, "ready": true, "tool_count": len(client.Tools())})
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ---------- MCP tool permission policies (Claude Code-style permission mode) ----------

// ListMCPPolicies returns all configured write-tool permission policies.
// Response: {"policies":[{"server":"github","tool":"push_files","allow_auto":true,"mode":"allow"}]}
func (h *AgentHandler) ListMCPPolicies(c fiber.Ctx) error {
	rows, err := h.db.Conn.Query(`SELECT server, tool, allow_auto, COALESCE(mode, CASE WHEN allow_auto=1 THEN 'allow' ELSE 'deny' END) FROM mcp_tool_policies ORDER BY server, tool`)
	if err != nil {
		return InternalError(c, err.Error())
	}
	defer rows.Close()
	policies := []map[string]interface{}{}
	for rows.Next() {
		var server, tool, mode string
		var allow int
		if err := rows.Scan(&server, &tool, &allow, &mode); err != nil {
			continue
		}
		policies = append(policies, map[string]interface{}{
			"server":     server,
			"tool":       tool,
			"allow_auto": allow == 1,
			"mode":       mode,
		})
	}
	if policies == nil {
		policies = []map[string]interface{}{}
	}
	return c.JSON(fiber.Map{"policies": policies})
}

// SetMCPPolicy upserts the permission mode for a single tool.
// Body: {"mode":"allow"} | {"mode":"deny"} | {"mode":"ask"} | legacy {"allow_auto":true}
func (h *AgentHandler) SetMCPPolicy(c fiber.Ctx) error {
	server := c.Params("server")
	tool := c.Params("tool")
	if server == "" || tool == "" {
		return c.Status(400).JSON(fiber.Map{"error": "server and tool are required"})
	}
	var req struct {
		AllowAuto bool   `json:"allow_auto"`
		Mode      string `json:"mode"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}
	mode := req.Mode
	if mode == "" {
		// Legacy callers pass allow_auto only.
		if req.AllowAuto {
			mode = "allow"
		} else {
			mode = "deny"
		}
	}
	switch mode {
	case "allow", "deny", "ask":
	default:
		return c.Status(400).JSON(fiber.Map{"error": "mode must be one of: allow, deny, ask"})
	}
	allowAuto := 0
	if mode == "allow" {
		allowAuto = 1
	}
	_, err := h.db.Conn.Exec(
		`INSERT INTO mcp_tool_policies (server, tool, allow_auto, mode) VALUES (?, ?, ?, ?)
		 ON CONFLICT(server, tool) DO UPDATE SET allow_auto=excluded.allow_auto, mode=excluded.mode, updated_at=CURRENT_TIMESTAMP`,
		server, tool, allowAuto, mode,
	)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"server": server, "tool": tool, "allow_auto": allowAuto == 1, "mode": mode})
}

// ConfirmMCPApproval resolves a pending ask-mode permission request.
// Body: {"request_id":"...", "allow":true|false}
func (h *AgentHandler) ConfirmMCPApproval(c fiber.Ctx) error {
	var req struct {
		RequestID string `json:"request_id"`
		Allow     bool   `json:"allow"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.RequestID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "request_id is required"})
	}
	if !h.runner.ResolveApproval(req.RequestID, req.Allow) {
		return c.Status(404).JSON(fiber.Map{"error": "permission request not found or already resolved", "request_id": req.RequestID})
	}
	return c.JSON(fiber.Map{"request_id": req.RequestID, "allow": req.Allow, "resolved": true})
}
