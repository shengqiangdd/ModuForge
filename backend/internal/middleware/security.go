package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

// SecurityHeaders adds common security headers to all responses.
// These headers protect against XSS, clickjacking, MIME sniffing, and other attacks.
func SecurityHeaders() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Prevent MIME type sniffing
		c.Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking (allow same-origin iframe for embeddable widgets)
		c.Set("X-Frame-Options", "SAMEORIGIN")

		// Enable XSS filtering in older browsers
		c.Set("X-XSS-Protection", "1; mode=block")

		// Control referrer information
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy — restrictive default for API responses
		// Only apply CSP to HTML responses (JSON doesn't need it)
		accept := c.Get("Accept", "")
		path := c.Path()
		if strings.Contains(accept, "text/html") || strings.HasSuffix(path, ".html") || path == "/docs" {
			c.Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'unsafe-inline' 'unsafe-eval' https://unpkg.com https://cdn.jsdelivr.net; "+
					"style-src 'self' 'unsafe-inline' https://unpkg.com https://cdn.jsdelivr.net; "+
					"img-src 'self' data: blob:; "+
					"font-src 'self' data:; "+
					"connect-src 'self' ws: wss:; "+
					"frame-ancestors 'self'; "+
					"form-action 'self'")
		}

		// Permissions Policy — restrict browser features
		c.Set("Permissions-Policy",
			"camera=(), microphone=(), geolocation=(), payment=(), usb=(), magnetometer=()")

		// Cache control for API responses
		if strings.HasPrefix(path, "/api/") {
			c.Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
			c.Set("Pragma", "no-cache")
		}

		return c.Next()
	}
}

// SecurityHeadersHSTS adds HSTS header for HTTPS connections.
// Only use when behind a reverse proxy that terminates TLS.
func SecurityHeadersHSTS() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		return c.Next()
	}
}
