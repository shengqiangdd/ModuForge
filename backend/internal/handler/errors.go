package handler

import (
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v3"
)

type APIError struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

const (
	ErrCodeInvalidInput   = "INVALID_INPUT"
	ErrCodeNotFound       = "NOT_FOUND"
	ErrCodeUnauthorized   = "UNAUTHORIZED"
	ErrCodeForbidden      = "FORBIDDEN"
	ErrCodeInternal       = "INTERNAL_ERROR"
	ErrCodeConflict       = "CONFLICT"
	ErrCodeValidation     = "VALIDATION_ERROR"
)

func ErrorResponse(c fiber.Ctx, status int, message string, code string) error {
	return c.Status(status).JSON(APIError{Error: message, Code: code})
}

func BadRequest(c fiber.Ctx, msg string) error {
	return ErrorResponse(c, 400, msg, ErrCodeInvalidInput)
}

func NotFound(c fiber.Ctx, msg string) error {
	return ErrorResponse(c, 404, msg, ErrCodeNotFound)
}

func Unauthorized(c fiber.Ctx, msg string) error {
	return ErrorResponse(c, 401, msg, ErrCodeUnauthorized)
}

func Forbidden(c fiber.Ctx, msg string) error {
	return ErrorResponse(c, 403, msg, ErrCodeForbidden)
}

func InternalError(c fiber.Ctx, msg string) error {
	return ErrorResponse(c, 500, msg, ErrCodeInternal)
}

func ValidationError(c fiber.Ctx, msg string) error {
	return ErrorResponse(c, 422, msg, ErrCodeValidation)
}

// ValidateProjectName checks project name constraints.
func ValidateProjectName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "项目名称不能为空"
	}
	if len(name) < 2 {
		return "项目名称至少需要2个字符"
	}
	if len(name) > 64 {
		return "项目名称不能超过64个字符"
	}
	re := regexp.MustCompile(`^[a-zA-Z0-9_\-\p{Han} ]+$`)
	if !re.MatchString(name) {
		return "项目名称包含不允许的字符（允许：字母、数字、中文、下划线、连字符、空格）"
	}
	return ""
}

// ValidateEmail checks basic email format.
func ValidateEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return "邮箱不能为空"
	}
	re := regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	if !re.MatchString(email) {
		return "邮箱格式不正确"
	}
	if len(email) > 128 {
		return "邮箱不能超过128个字符"
	}
	return ""
}

// ValidatePassword checks password strength.
func ValidatePassword(password string) string {
	if password == "" {
		return "密码不能为空"
	}
	if len(password) < 6 {
		return "密码至少需要6个字符"
	}
	if len(password) > 128 {
		return "密码不能超过128个字符"
	}
	return ""
}

// ValidateUsername checks username constraints.
func ValidateUsername(username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return "用户名不能为空"
	}
	if len(username) < 2 {
		return "用户名至少需要2个字符"
	}
	if len(username) > 32 {
		return "用户名不能超过32个字符"
	}
	re := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	if !re.MatchString(username) {
		return "用户名只能包含字母、数字和下划线"
	}
	return ""
}
