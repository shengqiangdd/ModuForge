// Package api 提供标准化的 HTTP 响应格式和错误码。
// 所有 handler 应使用本包代替 ad-hoc fiber.Map 构造响应。
package api

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
)

// --- 标准响应类型 ---

// Error 是标准错误响应体。
type Error struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

// Success 是标准成功响应体。
type Success struct {
	Status string `json:"status"`
	Data   any    `json:"data,omitempty"`
}

// --- 标准错误码 ---

const (
	CodeBadRequest       = "BAD_REQUEST"
	CodeUnauthorized     = "UNAUTHORIZED"
	CodeForbidden        = "FORBIDDEN"
	CodeNotFound         = "NOT_FOUND"
	CodeConflict         = "CONFLICT"
	CodeRateLimited      = "RATE_LIMITED"
	CodeInternalError    = "INTERNAL_ERROR"
	CodeValidationError  = "VALIDATION_ERROR"
	CodeInvalidBody      = "INVALID_BODY"
	CodeMissingField     = "MISSING_FIELD"
	CodeInvalidToken     = "INVALID_TOKEN"
	CodeExpiredToken     = "EXPIRED_TOKEN"
	CodeInsufficientPerm = "INSUFFICIENT_PERMISSIONS"
)

// --- 响应辅助函数 ---

// ErrorJSON 发送标准错误响应。
func ErrorJSON(c fiber.Ctx, status int, msg, code string) error {
	return c.Status(status).JSON(Error{Error: msg, Code: code})
}

// BadRequest 发送 400 错误。
func BadRequest(c fiber.Ctx, msg string) error {
	return ErrorJSON(c, http.StatusBadRequest, msg, CodeBadRequest)
}

// Unauthorized 发送 401 错误。
func Unauthorized(c fiber.Ctx, msg string) error {
	return ErrorJSON(c, http.StatusUnauthorized, msg, CodeUnauthorized)
}

// Forbidden 发送 403 错误。
func Forbidden(c fiber.Ctx, msg string) error {
	return ErrorJSON(c, http.StatusForbidden, msg, CodeForbidden)
}

// NotFound 发送 404 错误。
func NotFound(c fiber.Ctx, msg string) error {
	return ErrorJSON(c, http.StatusNotFound, msg, CodeNotFound)
}

// Conflict 发送 409 错误。
func Conflict(c fiber.Ctx, msg string) error {
	return ErrorJSON(c, http.StatusConflict, msg, CodeConflict)
}

// InternalError 发送 500 错误。
func InternalError(c fiber.Ctx, msg string) error {
	return ErrorJSON(c, http.StatusInternalServerError, msg, CodeInternalError)
}

// SuccessOK 发送标准 200 成功响应。
func SuccessOK(c fiber.Ctx) error {
	return c.JSON(Success{Status: "ok"})
}

// SuccessData 发送带数据的 200 成功响应。
func SuccessData(c fiber.Ctx, data any) error {
	return c.JSON(data)
}

// InvalidBody 发送 400 请求体无效错误。
func InvalidBody(c fiber.Ctx) error {
	return BadRequest(c, "Invalid request body")
}

// MissingField 发送 400 缺少必填字段错误。
func MissingField(c fiber.Ctx, field string) error {
	return ErrorJSON(c, http.StatusBadRequest, field+" is required", CodeMissingField)
}