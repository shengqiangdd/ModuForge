package handler

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

func AuthMiddleware(jwtSecret string) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Support Authorization header or query param ?token=
		token := ""
		if authHeader := c.Get("Authorization"); authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}
		if token == "" {
			if t := c.Query("token"); t != "" {
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
