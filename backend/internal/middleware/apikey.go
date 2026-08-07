package middleware

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

func APIKeyAuth(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		apiKey := c.Get("X-API-Key")
		if apiKey == "" {
			return c.Next()
		}

		h := sha256.New()
		h.Write([]byte(apiKey))
		hash := fmt.Sprintf("%x", h.Sum(nil))

		var userID, permissions string
		var expiresAt sql.NullString
		err := db.QueryRow(
			`SELECT user_id, permissions, expires_at FROM api_keys WHERE key_hash = ?`, hash,
		).Scan(&userID, &permissions, &expiresAt)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "invalid API key"})
		}

		if expiresAt.Valid && expiresAt.String != "" {
			expTime, err := time.Parse("2006-01-02 15:04:05", expiresAt.String)
			if err == nil && time.Now().After(expTime) {
				return c.Status(401).JSON(fiber.Map{"error": "API key expired"})
			}
		}

		var perms []string
		json.Unmarshal([]byte(permissions), &perms)

		db.Exec(`UPDATE api_keys SET last_used_at = datetime('now') WHERE key_hash = ?`, hash)

		c.Locals("uid", userID)
		c.Locals("auth_type", "apikey")
		c.Locals("permissions", perms)
		return c.Next()
	}
}

func RequirePermission(perm string) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Check API key permissions
		authType, _ := c.Locals("auth_type").(string)
		if authType == "apikey" {
			perms, _ := c.Locals("permissions").([]string)
			for _, p := range perms {
				if p == perm || p == "admin" {
					return c.Next()
				}
			}
			if strings.HasSuffix(perm, ":write") {
				for _, p := range perms {
					if p == "write" || p == "admin" {
						return c.Next()
					}
				}
			}
			return c.Status(403).JSON(fiber.Map{"error": "insufficient permissions"})
		}
		// JWT auth: check role-based permissions
		role, _ := c.Locals("role").(string)
		if role == "admin" {
			return c.Next()
		}
		// For non-admin users, only allow general (non-admin) permissions
		if perm == "admin" || strings.HasPrefix(perm, "admin:") {
			return c.Status(403).JSON(fiber.Map{"error": "insufficient permissions"})
		}
		// Standard non-admin users get access to general permissions
		return c.Next()
	}
}
