package handler

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type SettingsHandler struct {
	emailSvc *service.EmailService
	db       *sql.DB
}

func NewSettingsHandler(emailSvc *service.EmailService, db *sql.DB) *SettingsHandler {
	return &SettingsHandler{emailSvc: emailSvc, db: db}
}

// GetEmailConfig 获取邮件配置
func (h *SettingsHandler) GetEmailConfig(c fiber.Ctx) error {
	cfg, err := h.emailSvc.LoadConfig()
	if err != nil {
		return c.JSON(fiber.Map{
			"configured": false,
			"error":      err.Error(),
		})
	}
	// 隐藏密码
	safeCfg := fiber.Map{
		"configured": true,
		"smtp_host":  cfg.SMPTHost,
		"smtp_port":  cfg.SMPTPort,
		"smtp_user":  cfg.SMPTUser,
		"from_name":  cfg.FromName,
		"from_email": cfg.FromEmail,
		"use_tls":    cfg.UseTLS,
		"is_active":  cfg.IsActive,
	}
	return c.JSON(safeCfg)
}

// UpdateEmailConfig 更新邮件配置
func (h *SettingsHandler) UpdateEmailConfig(c fiber.Ctx) error {
	var cfg service.EmailConfig
	if err := c.Bind().JSON(&cfg); err != nil {
		return BadRequest(c, "请求格式无效")
	}
	if err := h.emailSvc.SaveConfig(&cfg); err != nil {
		return ValidationError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true, "message": "配置已保存"})
}

// SendTestEmail 发送测试邮件
func (h *SettingsHandler) SendTestEmail(c fiber.Ctx) error {
	var req struct {
		To string `json:"to"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "请求格式无效")
	}
	if req.To == "" {
		return ValidationError(c, "收件人邮箱不能为空")
	}
	if err := h.emailSvc.SendTestEmail(req.To); err != nil {
		return InternalError(c, "测试邮件发送失败: "+err.Error())
	}
	return c.JSON(fiber.Map{"ok": true, "message": "测试邮件已发送"})
}

// TestConnection 测试 SMTP 连接
func (h *SettingsHandler) TestConnection(c fiber.Ctx) error {
	if err := h.emailSvc.TestConnection(); err != nil {
		return c.JSON(fiber.Map{
			"ok":      false,
			"message": "连接测试失败: " + err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"ok":      true,
		"message": "连接测试成功",
	})
}

// ===== Agent Settings =====

// ensureSettingsTable 确保 agent_settings 表存在
func (h *SettingsHandler) ensureSettingsTable() {
	if h.db == nil {
		return
	}
	h.db.Exec(`CREATE TABLE IF NOT EXISTS agent_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
}

// GetAgentConfig 获取 Agent 配置
func (h *SettingsHandler) GetAgentConfig(c fiber.Ctx) error {
	h.ensureSettingsTable()

	defaults := map[string]string{
		"max_iterations": "50",
		"max_result_len": "32768",
	}

	result := make(map[string]string)
	for k, v := range defaults {
		var val string
		if h.db != nil {
			err := h.db.QueryRow("SELECT value FROM agent_settings WHERE key=?", k).Scan(&val)
			if err == nil {
				result[k] = val
				continue
			}
		}
		result[k] = v
	}
	return c.JSON(result)
}

// UpdateAgentConfig 更新 Agent 配置
func (h *SettingsHandler) UpdateAgentConfig(c fiber.Ctx) error {
	h.ensureSettingsTable()

	var req struct {
		MaxIterations *int `json:"max_iterations"`
		MaxResultLen  *int `json:"max_result_len"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "请求格式无效")
	}

	if h.db == nil {
		return InternalError(c, "数据库未连接")
	}

	if req.MaxIterations != nil {
		v := *req.MaxIterations
		if v < 1 || v > 200 {
			return ValidationError(c, "迭代次数范围: 1-200")
		}
		if _, err := h.db.Exec("INSERT OR REPLACE INTO agent_settings (key, value, updated_at) VALUES (?, ?, datetime('now'))", "max_iterations", v); err != nil {
			return InternalError(c, err.Error())
		}
	}
	if req.MaxResultLen != nil {
		v := *req.MaxResultLen
		if v < 500 || v > 100000 {
			return ValidationError(c, "结果长度范围: 500-100000")
		}
		if _, err := h.db.Exec("INSERT OR REPLACE INTO agent_settings (key, value, updated_at) VALUES (?, ?, datetime('now'))", "max_result_len", v); err != nil {
			return InternalError(c, err.Error())
		}
	}

	return c.JSON(fiber.Map{"ok": true, "message": "配置已保存"})
}
