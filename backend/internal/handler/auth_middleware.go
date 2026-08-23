package handler

import (
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

func AuthMiddleware(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := ""
		if authHeader := c.Get("Authorization"); authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}
		// DEPRECATED: query param fallback exists only for legacy WebSocket/SSE clients
		// that cannot set custom headers. New clients MUST use Authorization header.
		if token == "" {
			if t := c.Query("token"); t != "" {
				slog.Warn("JWT passed via query param (deprecated, will be removed)",
					"path", c.Path(), "remote", c.IP())
				token = t
			}
		}
		if token == "" {
			return c.Status(401).JSON(fiber.Map{"error": "missing authorization header"})
		}
		claims, err := service.ParseJWT(token, jwtSecret)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "invalid or expired token"})
		}

		// Reject 2fa_pending tokens — they are only for the 2FA verification endpoint
		if claims.Role == "2fa_pending" {
			return c.Status(403).JSON(fiber.Map{"error": "2FA verification required"})
		}

		c.Locals("uid", claims.UID)
		c.Locals("user_id", claims.UID)
		c.Locals("username", claims.Username)
		c.Locals("role", claims.Role)
		return c.Next()
	}
}

func AdminOnly() fiber.Handler {
	return func(c fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		if role != "admin" {
			return c.Status(403).JSON(fiber.Map{"error": "admin access required"})
		}
		return c.Next()
	}
}

func OptionalAuth(jwtSecret string) fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Next()
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Next()
		}
		claims, err := service.ParseJWT(parts[1], jwtSecret)
		if err == nil {
			c.Locals("uid", claims.UID)
			c.Locals("user_id", claims.UID)
			c.Locals("username", claims.Username)
			c.Locals("role", claims.Role)
		}
		return c.Next()
	}
}
