package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

// ValidateJSON validates JSON request body against a struct.
// Usage: ctx.App().Use(ValidateJSON[CreateProjectRequest]())
func ValidateJSON[T any](c fiber.Ctx) error {
	// Skip validation for GET/DELETE requests
	if c.Method() == "GET" || c.Method() == "DELETE" {
		return c.Next()
	}

	// Read and validate JSON body
	body := c.Body()
	if len(body) == 0 {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Request body is required",
		})
	}

	var req T
	if err := json.Unmarshal(body, &req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Invalid JSON: %v", err),
		})
	}

	// Store parsed request in context for handlers to use
	c.Locals("validatedBody", &req)
	return c.Next()
}

// GetValidatedBody retrieves the validated request body from context.
func GetValidatedBody[T any](c fiber.Ctx) *T {
	if v, ok := c.Locals("validatedBody").(*T); ok {
		return v
	}
	return nil
}

// RequireAuth is a simple API key validation middleware.
func RequireAuth(validKeys []string) fiber.Handler {
	return func(c fiber.Ctx) error {
		auth := c.Get("Authorization")
		if auth == "" {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authorization header required",
			})
		}

		// Support "Bearer <token>" or just "<token>"
		token := strings.TrimPrefix(auth, "Bearer ")

		for _, key := range validKeys {
			if key != "" && token == key {
				return c.Next()
			}
		}

		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid API key",
		})
	}
}

// RateLimiter provides simple in-memory rate limiting.
type RateLimiter struct {
	visits map[string][]int64
	limit  int
	window int64 // milliseconds
}

func NewRateLimiter(limit int, windowMs int64) *RateLimiter {
	return &RateLimiter{
		visits: make(map[string][]int64),
		limit:  limit,
		window: windowMs,
	}
}

func (rl *RateLimiter) Handler() fiber.Handler {
	return func(c fiber.Ctx) error {
		ip := c.IP()
		now := time.Now().UnixMilli()

		// Clean old entries
		visits := rl.visits[ip]
		valid := make([]int64, 0, len(visits))
		for _, t := range visits {
			if now-t < rl.window {
				valid = append(valid, t)
			}
		}

		if len(valid) >= rl.limit {
			return c.Status(http.StatusTooManyRequests).JSON(fiber.Map{
				"error":       "Rate limit exceeded",
				"retry_after": rl.window / 1000,
			})
		}

		rl.visits[ip] = append(valid, now)
		return c.Next()
	}
}
