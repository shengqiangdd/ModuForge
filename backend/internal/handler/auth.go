package handler

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/domain"
	"github.com/moduforge/backend/internal/service"
)

// TOTP / 2FA flow:
// POST /auth/2fa/setup   -> returns secret + qr_url
// POST /auth/2fa/enable   -> verify code + enable
// POST /auth/2fa/disable  -> verify code + disable

func (h *AuthHandler) Setup2FA(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}

	setup, err := h.svc.GenerateTOTPSecret(userID)
	if err != nil {
		return InternalError(c, err.Error())
	}

	return c.JSON(setup)
}

func (h *AuthHandler) Enable2FA(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "请求格式无效")
	}
	if req.Code == "" {
		return ValidationError(c, "验证码不能为空")
	}

	if err := h.svc.EnableTOTP(userID, req.Code); err != nil {
		return ValidationError(c, err.Error())
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AuthHandler) Disable2FA(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "请求格式无效")
	}
	if req.Code == "" {
		return ValidationError(c, "验证码不能为空")
	}

	if err := h.svc.DisableTOTP(userID, req.Code); err != nil {
		return ValidationError(c, err.Error())
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

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

func (h *AuthHandler) VerifyEmail(c fiber.Ctx) error {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Token == "" {
		return ValidationError(c, "token required")
	}
	if err := h.svc.VerifyEmail(req.Token); err != nil {
		return ValidationError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *AuthHandler) ResendVerification(c fiber.Ctx) error {
	userID := safeUserID(c)
	email := c.Query("email")
	if email == "" {
		var user service.UserProfile
		p, err := h.svc.GetProfile(userID)
		if err != nil {
			return Unauthorized(c, "user not found")
		}
		user = *p
		email = user.Email
	}
	if err := h.svc.SendVerificationEmail(userID, email); err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *AuthHandler) ForgotPassword(c fiber.Ctx) error {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Email == "" {
		return ValidationError(c, "email required")
	}
	if err := h.svc.RequestPasswordReset(req.Email); err != nil {
		return c.JSON(fiber.Map{"ok": true})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *AuthHandler) ResetPassword(c fiber.Ctx) error {
	var req struct {
		Token           string `json:"token"`
		Password        string `json:"password"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Token == "" || req.Password == "" {
		return ValidationError(c, "token and password required")
	}
	if msg := ValidatePassword(req.Password); msg != "" {
		return ValidationError(c, msg)
	}
	if err := h.svc.ResetPassword(req.Token, req.Password); err != nil {
		return ValidationError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

// ChangePassword — POST /auth/change-password
func (h *AuthHandler) ChangePassword(c fiber.Ctx) error {
	uid := safeUserID(c)
	if uid == "" {
		return Unauthorized(c, "未登录")
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "请求格式无效")
	}
	if req.OldPassword == "" || req.NewPassword == "" {
		return ValidationError(c, "当前密码和新密码均不能为空")
	}
	if msg := ValidatePassword(req.NewPassword); msg != "" {
		return ValidationError(c, msg)
	}
	if err := h.svc.ChangePassword(uid, req.OldPassword, req.NewPassword); err != nil {
		return ValidationError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *AuthHandler) GetProfile(c fiber.Ctx) error {
	userID := safeUserID(c)
	profile, err := h.svc.GetProfile(userID)
	if err != nil {
		return Unauthorized(c, err.Error())
	}
	return c.JSON(profile)
}

func (h *AuthHandler) UpdateProfile(c fiber.Ctx) error {
	userID := safeUserID(c)
	var updates map[string]string
	if err := c.Bind().JSON(&updates); err != nil {
		return BadRequest(c, "invalid request")
	}
	if err := h.svc.UpdateProfile(userID, updates); err != nil {
		return InternalError(c, err.Error())
	}
	profile, _ := h.svc.GetProfile(userID)
	return c.JSON(profile)
}

func (h *AuthHandler) UploadAvatar(c fiber.Ctx) error {
	userID := safeUserID(c)
	file, err := c.FormFile("avatar")
	if err != nil {
		return BadRequest(c, "no file uploaded")
	}
	if file.Size > 2*1024*1024 {
		return ValidationError(c, "file too large (max 2MB)")
	}
	ext := filepath.Ext(file.Filename)
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" && ext != ".webp" {
		return ValidationError(c, "unsupported file format")
	}
	avatarDir := filepath.Join("data", "storage", "avatars")
	os.MkdirAll(avatarDir, 0755)
	avatarPath := filepath.Join(avatarDir, fmt.Sprintf("%s%s", userID, ext))
	if err := c.SaveFile(file, avatarPath); err != nil {
		return InternalError(c, "failed to save avatar")
	}
	urlPath := "/uploads/avatars/" + fmt.Sprintf("%s%s", userID, ext)
	if err := h.svc.UploadAvatar(userID, urlPath); err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"avatar_url": urlPath})
}

// Refresh 刷新 JWT token（当前 token 有效期不足 1 天时自动续期）
func (h *AuthHandler) Refresh(c fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		slog.Warn("refresh: missing token", "auth_header", authHeader)
		return Unauthorized(c, "missing token")
	}

	slog.Info("refresh: attempting token refresh", "token_len", len(token))
	resp, err := h.svc.RefreshToken(c.Context(), token)
	if err != nil {
		slog.Warn("refresh: failed", "error", err)
		return Unauthorized(c, err.Error())
	}
	slog.Info("refresh: success", "new_token_len", len(resp.Token))
	return c.JSON(fiber.Map{"token": resp.Token, "user": resp.User})
}
