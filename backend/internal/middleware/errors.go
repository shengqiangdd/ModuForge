package middleware

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/gofiber/fiber/v3"
)

// AppError represents a structured application error.
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *AppError) Error() string { return e.Message }

// Common error constructors
func ErrBadRequest(msg string) *AppError {
	return &AppError{Code: "BAD_REQUEST", Message: msg, Status: 400}
}

func ErrUnauthorized(msg string) *AppError {
	return &AppError{Code: "UNAUTHORIZED", Message: msg, Status: 401}
}

func ErrForbidden(msg string) *AppError {
	return &AppError{Code: "FORBIDDEN", Message: msg, Status: 403}
}

func ErrNotFound(msg string) *AppError {
	return &AppError{Code: "NOT_FOUND", Message: msg, Status: 404}
}

func ErrConflict(msg string) *AppError {
	return &AppError{Code: "CONFLICT", Message: msg, Status: 409}
}

func ErrRateLimited(msg string) *AppError {
	return &AppError{Code: "RATE_LIMITED", Message: msg, Status: 429}
}

func ErrInternal(msg string) *AppError {
	return &AppError{Code: "INTERNAL_ERROR", Message: msg, Status: 500}
}

func ErrFeatureDisabled(key string) *AppError {
	return &AppError{Code: "FEATURE_DISABLED", Message: fmt.Sprintf("功能 %s 已禁用", key), Status: 501}
}

// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	RequestID string `json:"request_id,omitempty"`
}

// ErrorHandler returns a Fiber error handler that produces structured JSON errors.
func ErrorHandler() fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		code := 500
		errCode := "INTERNAL_ERROR"
		errMsg := "Internal server error"

		if appErr, ok := err.(*AppError); ok {
			code = appErr.Status
			errCode = appErr.Code
			errMsg = appErr.Message
		} else if fiberErr, ok := err.(*fiber.Error); ok {
			code = fiberErr.Code
			errMsg = fiberErr.Message
			switch code {
			case 400:
				errCode = "BAD_REQUEST"
			case 401:
				errCode = "UNAUTHORIZED"
			case 403:
				errCode = "FORBIDDEN"
			case 404:
				errCode = "NOT_FOUND"
			case 429:
				errCode = "RATE_LIMITED"
			}
		}

		// Log server errors
		if code >= 500 {
			slog.Error("server error",
				"code", code,
				"error", errMsg,
				"path", c.Path(),
				"method", c.Method(),
				"request_id", c.Locals("request_id"),
				"stack", string(debug.Stack()),
			)
		}

		resp := ErrorResponse{
			Error:     errMsg,
			Code:      errCode,
			RequestID: fmt.Sprintf("%v", c.Locals("request_id")),
		}

		return c.Status(code).JSON(resp)
	}
}

// RecoverMiddleware catches panics and returns structured 500 errors.
func RecoverMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered",
					"error", r,
					"path", c.Path(),
					"method", c.Method(),
					"request_id", c.Locals("request_id"),
					"stack", string(debug.Stack()),
				)
				_ = c.Status(500).JSON(ErrorResponse{
					Error:     "Internal server error",
					Code:      "PANIC",
					RequestID: fmt.Sprintf("%v", c.Locals("request_id")),
				})
			}
		}()
		return c.Next()
	}
}

// RequestLogger logs each request with timing and status.
func RequestLogger() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		latency := time.Since(start)

		status := c.Response().StatusCode()
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		slog.Log(c.Context(), level, "request",
			"method", c.Method(),
			"path", c.Path(),
			"status", status,
			"latency", latency.String(),
			"ip", c.IP(),
			"request_id", c.Locals("request_id"),
		)
		return err
	}
}
