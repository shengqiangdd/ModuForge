package handler

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v3"
)

type ToolPolicy struct {
	ID        int    `json:"id"`
	Server    string `json:"server"`
	Tool      string `json:"tool"`
	AllowAuto bool   `json:"allow_auto"`
	Mode      string `json:"mode"` // confirm/allow/deny
	UpdatedAt string `json:"updated_at"`
}

type ToolPolicyRequest struct {
	Server string `json:"server"`
	Tool   string `json:"tool"`
	Mode   string `json:"mode"` // confirm/allow/deny
}

// GetToolPolicies returns all tool execution policies
func GetToolPolicies(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		rows, err := db.Query(`
			SELECT id, server, tool, allow_auto, mode, updated_at 
			FROM mcp_tool_policies 
			ORDER BY server, tool
		`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		policies := make([]ToolPolicy, 0)
		for rows.Next() {
			var p ToolPolicy
			if err := rows.Scan(&p.ID, &p.Server, &p.Tool, &p.AllowAuto, &p.Mode, &p.UpdatedAt); err != nil {
				continue
			}
			policies = append(policies, p)
		}
		return c.JSON(policies)
	}
}

// CreateOrUpdateToolPolicy creates or updates a tool policy
func CreateOrUpdateToolPolicy(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req ToolPolicyRequest
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
		}

		if req.Server == "" || req.Tool == "" {
			return c.Status(400).JSON(fiber.Map{"error": "server and tool are required"})
		}

		mode := req.Mode
		if mode == "" {
			mode = "confirm"
		}
		if mode != "confirm" && mode != "allow" && mode != "deny" {
			return c.Status(400).JSON(fiber.Map{"error": "mode must be confirm, allow, or deny"})
		}

		allowAuto := mode == "allow"
		now := time.Now().UTC().Format("2006-01-02 15:04:05")

		_, err := db.Exec(`
			INSERT INTO mcp_tool_policies (server, tool, allow_auto, mode, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(server, tool) DO UPDATE SET 
				allow_auto = excluded.allow_auto,
				mode = excluded.mode,
				updated_at = excluded.updated_at
		`, req.Server, req.Tool, allowAuto, mode, now)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"ok": true, "message": "Policy updated"})
	}
}

// DeleteToolPolicy removes a tool policy
func DeleteToolPolicy(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		server := c.Params("server")
		tool := c.Params("tool")
		if server == "" || tool == "" {
			return c.Status(400).JSON(fiber.Map{"error": "server and tool are required"})
		}

		_, err := db.Exec("DELETE FROM mcp_tool_policies WHERE server = ? AND tool = ?", server, tool)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// GetGlobalToolPolicy returns/sets the global default tool policy
func GetGlobalToolPolicy(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		var mode string
		err := db.QueryRow("SELECT value FROM agent_settings WHERE key = 'global_tool_policy'").Scan(&mode)
		if err != nil {
			mode = "confirm" // default
		}
		return c.JSON(fiber.Map{"mode": mode})
	}
}

// SetGlobalToolPolicy sets the global default tool policy
func SetGlobalToolPolicy(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			Mode string `json:"mode"`
		}
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
		}

		if req.Mode != "confirm" && req.Mode != "allow" && req.Mode != "deny" {
			return c.Status(400).JSON(fiber.Map{"error": "mode must be confirm, allow, or deny"})
		}

		now := time.Now().UTC().Format("2006-01-02 15:04:05")
		_, err := db.Exec(`
			INSERT INTO agent_settings (key, value, updated_at) 
			VALUES ('global_tool_policy', ?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
		`, req.Mode, now)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}
