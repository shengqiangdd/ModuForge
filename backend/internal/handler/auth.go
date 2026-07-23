package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/domain"
	"github.com/moduforge/backend/internal/service"
)

type AuthHandler struct {
	svc *service.AuthService
}

func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

func (h *AuthHandler) Register(c fiber.Ctx) error {
	var req domain.RegisterRequest
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "请求格式无效")
	}
	if msg := ValidateUsername(req.Username); msg != "" {
		return ValidationError(c, msg)
	}
	if msg := ValidateEmail(req.Email); msg != "" {
		return ValidationError(c, msg)
	}
	if msg := ValidatePassword(req.Password); msg != "" {
		return ValidationError(c, msg)
	}
	resp, err := h.svc.Register(c.Context(), &req)
	if err != nil {
		return ErrorResponse(c, 400, err.Error(), ErrCodeConflict)
	}
	return c.Status(201).JSON(resp)
}

func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req domain.LoginRequest
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "请求格式无效")
	}
	if req.Username == "" || req.Password == "" {
		return ValidationError(c, "用户名和密码不能为空")
	}
	resp, err := h.svc.Login(c.Context(), &req)
	if err != nil {
		return Unauthorized(c, "用户名或密码错误")
	}
	return c.JSON(resp)
}
