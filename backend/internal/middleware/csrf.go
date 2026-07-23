package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

func ContentTypeCheck() fiber.Handler {
	return func(c fiber.Ctx) error {
		method := c.Method()
		if method == "POST" || method == "PUT" || method == "DELETE" || method == "PATCH" {
			ct := c.Get("Content-Type")
			if ct != "" &&
				!strings.HasPrefix(ct, "application/json") &&
				!strings.HasPrefix(ct, "multipart/form-data") &&
				!strings.HasPrefix(ct, "text/event-stream") {
				return c.Status(415).JSON(fiber.Map{"error": "unsupported content type"})
			}
		}
		return c.Next()
	}
}
