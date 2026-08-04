package service

import (
	"crypto/rand"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// EmailConfig 邮件服务配置
type EmailConfig struct {
	SMPTHost     string `json:"smtp_host"`
	SMPTPort     int    `json:"smtp_port"`
	SMPTUser     string `json:"smtp_user"`
	SMPTPassword string `json:"smtp_password"`
	FromName     string `json:"from_name"`
	FromEmail    string `json:"from_email"`
	UseTLS       int    `json:"use_tls"` // 0=无, 1=STARTTLS, 2=SSL/TLS
	IsActive     int    `json:"is_active"`
}

// TLS 模式常量
const (
	TLSNone  = 0 // 不加密
	TLSSTART = 1 // STARTTLS (端口 587)
	TLSSSL   = 2 // 直接 SSL/TLS (端口 465)
)

type EmailService struct {
	db  *sql.DB
	cfg *EmailConfig
}

func NewEmailService(db *sql.DB) *EmailService {
	return &EmailService{db: db}
}

// GenerateCode 生成指定长度的数字验证码
func GenerateCode(length int) (string, error) {
	code := ""
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		code += fmt.Sprintf("%d", n.Int64())
	}
	return code, nil
}

// LoadConfig 从数据库加载邮件配置
func (s *EmailService) LoadConfig() (*EmailConfig, error) {
	var cfg EmailConfig
	err := s.db.QueryRow(
		`SELECT smtp_host, smtp_port, smtp_user, smtp_password, from_name, from_email, use_tls, is_active
		 FROM email_config ORDER BY id DESC LIMIT 1`,
	).Scan(&cfg.SMPTHost, &cfg.SMPTPort, &cfg.SMPTUser, &cfg.SMPTPassword, &cfg.FromName, &cfg.FromEmail, &cfg.UseTLS, &cfg.IsActive)
	if err != nil {
		return nil, fmt.Errorf("email config not found: %w", err)
	}
	s.cfg = &cfg
	return &cfg, nil
}

// getConfig 获取当前配置（带缓存）
func (s *EmailService) getConfig() *EmailConfig {
	if s.cfg != nil {
		return s.cfg
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		return nil
	}
	return cfg
}

// Send 发送邮件（自动选择连接方式）
func (s *EmailService) Send(to, subject, body string) error {
	cfg := s.getConfig()
	if cfg == nil {
		return fmt.Errorf("email not configured")
	}
	if cfg.IsActive == 0 {
		return fmt.Errorf("email service is not active")
	}

	// 构建邮件头
	from := cfg.FromEmail
	if cfg.FromName != "" {
		from = fmt.Sprintf("%s <%s>", cfg.FromName, cfg.FromEmail)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\nDate: %s\r\n\r\n%s",
		from, to, subject, time.Now().UTC().Format(time.RFC1123Z), body)

	// 根据 TLS 模式选择发送方式
	switch cfg.UseTLS {
	case TLSSSL:
		return s.sendWithSSL(cfg, to, msg)
	case TLSSTART:
		return s.sendWithSTARTTLS(cfg, to, msg)
	default:
		return s.sendPlain(cfg, to, msg)
	}
}

// sendWithSSL 通过 SSL/TLS 直接连接（端口 465）
func (s *EmailService) sendWithSSL(cfg *EmailConfig, to, msg string) error {
	addr := fmt.Sprintf("%s:%d", cfg.SMPTHost, cfg.SMPTPort)

	tlsConfig := &tls.Config{
		ServerName:         cfg.SMPTHost,
		InsecureSkipVerify: false,
	}

	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 10 * time.Second},
		"tcp", addr, tlsConfig,
	)
	if err != nil {
		return fmt.Errorf("SSL connect to %s failed: %w", addr, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.SMPTHost)
	if err != nil {
		return fmt.Errorf("smtp client creation failed: %w", err)
	}
	defer client.Close()

	// 认证
	if err := s.authenticate(client, cfg); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	// 发送邮件
	return s.doSend(client, cfg.FromEmail, to, msg)
}

// sendWithSTARTTLS 通过 STARTTLS 升级连接（端口 587/25）
func (s *EmailService) sendWithSTARTTLS(cfg *EmailConfig, to, msg string) error {
	// Format address correctly for both IPv4 and IPv6
	addr := net.JoinHostPort(cfg.SMPTHost, fmt.Sprintf("%d", cfg.SMPTPort))

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("connect to %s failed: %w", addr, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.SMPTHost)
	if err != nil {
		return fmt.Errorf("smtp client creation failed: %w", err)
	}
	defer client.Close()

	// 升级到 TLS
	tlsConfig := &tls.Config{
		ServerName:         cfg.SMPTHost,
		InsecureSkipVerify: false,
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("STARTTLS failed: %w", err)
	}

	// 认证
	if err := s.authenticate(client, cfg); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	// 发送邮件
	return s.doSend(client, cfg.FromEmail, to, msg)
}

// sendPlain 通过普通 SMTP 发送（无加密）
func (s *EmailService) sendPlain(cfg *EmailConfig, to, msg string) error {
	addr := fmt.Sprintf("%s:%d", cfg.SMPTHost, cfg.SMPTPort)

	auth := smtp.PlainAuth("", cfg.SMPTUser, cfg.SMPTPassword, cfg.SMPTHost)
	return smtp.SendMail(addr, auth, cfg.FromEmail, []string{to}, []byte(msg))
}

// authenticate 执行 SMTP 认证
func (s *EmailService) authenticate(client *smtp.Client, cfg *EmailConfig) error {
	if cfg.SMPTUser == "" {
		return nil
	}

	// 优先尝试 LOGIN 方式（兼容性最好）
	if ok, _ := client.Extension("LOGIN"); ok {
		return s.authLogin(client, cfg.SMPTUser, cfg.SMPTPassword)
	}

	// 回退到 PLAIN（跳过主机名校验）
	return client.Auth(plainAuthNoHost{cfg.SMPTUser, cfg.SMPTPassword})
}

// authLogin 实现 LOGIN 认证方式
func (s *EmailService) authLogin(client *smtp.Client, username, password string) error {
	encodedUser := base64.StdEncoding.EncodeToString([]byte(username))
	encodedPass := base64.StdEncoding.EncodeToString([]byte(password))

	// 使用 Cmd 方法发送 AUTH LOGIN 命令
	id, err := client.Text.Cmd("AUTH LOGIN")
	if err != nil {
		return err
	}
	client.Text.StartResponse(id)
	defer client.Text.EndResponse(id)

	// 等待 334 Username prompt
	if _, _, err := client.Text.ReadResponse(334); err != nil {
		return fmt.Errorf("AUTH LOGIN username prompt failed: %w", err)
	}

	// 发送 base64 编码的用户名
	if err := client.Text.PrintfLine("%s", encodedUser); err != nil {
		return err
	}

	// 等待 334 Password prompt
	if _, _, err := client.Text.ReadResponse(334); err != nil {
		return fmt.Errorf("AUTH LOGIN password prompt failed: %w", err)
	}

	// 发送 base64 编码的密码
	if err := client.Text.PrintfLine("%s", encodedPass); err != nil {
		return err
	}

	// 等待 235 Authentication successful
	if _, _, err := client.Text.ReadResponse(235); err != nil {
		return fmt.Errorf("AUTH LOGIN failed: %w", err)
	}

	return nil
}

// plainAuthNoHost 跳过主机名校验的 PLAIN 认证
type plainAuthNoHost struct {
	username string
	password string
}

func (a plainAuthNoHost) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "PLAIN", []byte("\x00" + a.username + "\x00" + a.password), nil
}

func (a plainAuthNoHost) Next(fromServer []byte, more bool) ([]byte, error) {
	return nil, nil
}

// doSend 执行邮件发送
func (s *EmailService) doSend(client *smtp.Client, from, to, msg string) error {
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO failed: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %w", err)
	}
	if _, err := writer.Write([]byte(msg)); err != nil {
		return fmt.Errorf("write message failed: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close data writer failed: %w", err)
	}
	return client.Quit()
}

// SendVerificationCode 发送邮箱验证码
func (s *EmailService) SendVerificationCode(to, code string) error {
	subject := "ModuForge 邮箱验证码"
	body := fmt.Sprintf(`<div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 480px; margin: 0 auto; padding: 20px;">
<h2 style="color: #8b5cf6; margin-bottom: 24px;">🔐 ModuForge 邮箱验证</h2>
<p style="color: #374151; font-size: 16px;">您的验证码是：</p>
<div style="font-size: 36px; font-weight: bold; letter-spacing: 12px; text-align: center; padding: 24px; background: linear-gradient(135deg, #f5f3ff 0%%, #ede9fe 100%%); border-radius: 16px; color: #7c3aed; margin: 16px 0;">%s</div>
<p style="color: #6b7280; font-size: 14px;">⏱ 验证码将在 <strong>10 分钟</strong>后过期</p>
<hr style="border: none; border-top: 1px solid #e5e7eb; margin: 24px 0;">
<p style="color: #9ca3af; font-size: 12px;">如果您没有请求此验证码，请忽略此邮件。</p>
</div>`, code)
	return s.Send(to, subject, body)
}

// SendPasswordReset 发送密码重置邮件
func (s *EmailService) SendPasswordReset(to, code string) error {
	subject := "ModuForge 密码重置"
	body := fmt.Sprintf(`<div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 480px; margin: 0 auto; padding: 20px;">
<h2 style="color: #8b5cf6; margin-bottom: 24px;">🔑 ModuForge 密码重置</h2>
<p style="color: #374151; font-size: 16px;">您的密码重置验证码是：</p>
<div style="font-size: 36px; font-weight: bold; letter-spacing: 12px; text-align: center; padding: 24px; background: linear-gradient(135deg, #fef2f2 0%%, #fee2e2 100%%); border-radius: 16px; color: #dc2626; margin: 16px 0;">%s</div>
<p style="color: #6b7280; font-size: 14px;">⏱ 验证码将在 <strong>1 小时</strong>后过期</p>
<hr style="border: none; border-top: 1px solid #e5e7eb; margin: 24px 0;">
<p style="color: #9ca3af; font-size: 12px;">如果您没有请求重置密码，请忽略此邮件。您的密码不会被更改，除非您使用此验证码。</p>
</div>`, code)
	return s.Send(to, subject, body)
}

// SendTestEmail 发送测试邮件
func (s *EmailService) SendTestEmail(to string) error {
	subject := "ModuForge 测试邮件"
	body := `<div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 480px; margin: 0 auto; padding: 20px;">
<h2 style="color: #10b981; margin-bottom: 24px;">✅ SMTP 配置测试成功</h2>
<p style="color: #374151; font-size: 16px;">恭喜！您的邮件服务配置正确。</p>
<div style="padding: 16px; background: #ecfdf5; border-radius: 12px; border-left: 4px solid #10b981; margin: 16px 0;">
<p style="color: #065f46; margin: 0;">📧 邮件系统已就绪，可以发送验证码和通知邮件。</p>
</div>
<p style="color: #6b7280; font-size: 14px;">此邮件由 ModuForge 自动发送。</p>
</div>`
	return s.Send(to, subject, body)
}

// SaveConfig 保存邮件配置
func (s *EmailService) SaveConfig(cfg *EmailConfig) error {
	if cfg.SMPTHost == "" {
		return fmt.Errorf("SMTP host is required")
	}
	if cfg.SMPTPort <= 0 || cfg.SMPTPort > 65535 {
		return fmt.Errorf("invalid SMTP port: %d", cfg.SMPTPort)
	}
	if cfg.SMPTUser == "" {
		return fmt.Errorf("SMTP username is required")
	}
	if cfg.SMPTPassword == "" {
		return fmt.Errorf("SMTP password is required")
	}
	if cfg.FromEmail == "" {
		return fmt.Errorf("from email is required")
	}
	if !strings.Contains(cfg.FromEmail, "@") {
		return fmt.Errorf("invalid from email format")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM email_config`); err != nil {
		return fmt.Errorf("failed to clear old config: %w", err)
	}

	if _, err := tx.Exec(
		`INSERT INTO email_config (smtp_host, smtp_port, smtp_user, smtp_password, from_name, from_email, use_tls, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		cfg.SMPTHost, cfg.SMPTPort, cfg.SMPTUser, cfg.SMPTPassword, cfg.FromName, cfg.FromEmail, cfg.UseTLS, cfg.IsActive,
	); err != nil {
		return fmt.Errorf("failed to insert config: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit config: %w", err)
	}

	s.cfg = cfg
	return nil
}

// TestConnection 测试 SMTP 连接
func (s *EmailService) TestConnection() error {
	cfg := s.getConfig()
	if cfg == nil {
		return fmt.Errorf("email not configured")
	}

	addr := net.JoinHostPort(cfg.SMPTHost, fmt.Sprintf("%d", cfg.SMPTPort))

	switch cfg.UseTLS {
	case TLSSSL:
		tlsConfig := &tls.Config{
			ServerName:         cfg.SMPTHost,
			InsecureSkipVerify: false,
		}
		conn, err := tls.DialWithDialer(
			&net.Dialer{Timeout: 10 * time.Second},
			"tcp", addr, tlsConfig,
		)
		if err != nil {
			return fmt.Errorf("SSL connection failed: %w", err)
		}
		conn.Close()
		return nil

	case TLSSTART:
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		conn, err := dialer.Dial("tcp", addr)
		if err != nil {
			return fmt.Errorf("TCP connection failed: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, cfg.SMPTHost)
		if err != nil {
			return fmt.Errorf("SMTP client creation failed: %w", err)
		}
		defer client.Close()

		tlsConfig := &tls.Config{
			ServerName:         cfg.SMPTHost,
			InsecureSkipVerify: false,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("STARTTLS failed: %w", err)
		}
		return nil

	default:
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		conn, err := dialer.Dial("tcp", addr)
		if err != nil {
			return fmt.Errorf("TCP connection failed: %w", err)
		}
		conn.Close()
		return nil
	}
}
