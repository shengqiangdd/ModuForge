package security

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
)

// SecurityMiddleware 安全中间件
type SecurityMiddleware struct {
	validator *InputValidator
	rateLimit map[string][]time.Time
	maxReqs   int
	window    time.Duration
}

// NewSecurityMiddleware 创建安全中间件
func NewSecurityMiddleware() *SecurityMiddleware {
	return &SecurityMiddleware{
		validator: NewInputValidator(),
		rateLimit: make(map[string][]time.Time),
		maxReqs:   100,
		window:    time.Minute,
	}
}

// RateLimit 速率限制中间件
func (sm *SecurityMiddleware) RateLimit(c fiber.Ctx) error {
	ip := c.IP()
	now := time.Now()

	// 清理过期记录
	reqs := sm.rateLimit[ip]
	validReqs := make([]time.Time, 0)
	for _, t := range reqs {
		if now.Sub(t) <= sm.window {
			validReqs = append(validReqs, t)
		}
	}

	// 检查速率限制
	if len(validReqs) >= sm.maxReqs {
		return c.Status(429).JSON(fiber.Map{
			"error":      "Too many requests",
			"retry_after": sm.window.Seconds(),
		})
	}

	// 记录请求
	validReqs = append(validReqs, now)
	sm.rateLimit[ip] = validReqs

	return c.Next()
}

// SecurityHeaders 安全头中间件
func (sm *SecurityMiddleware) SecurityHeaders(c fiber.Ctx) error {
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("X-Frame-Options", "DENY")
	c.Set("X-XSS-Protection", "1; mode=block")
	c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	c.Set("Content-Security-Policy", "default-src 'self'")
	c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

	return c.Next()
}

// InputSanitize 输入清理中间件
func (sm *SecurityMiddleware) InputSanitize(c fiber.Ctx) error {
	// 检查查询参数
	queries := c.Queries()
	for key, value := range queries {
		result := sm.validator.ValidateSearchQuery(value)
		if !result.Valid {
			log.Printf("Blocked suspicious query param: %s=%s", key, value)
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid input detected",
			})
		}
	}

	return c.Next()
}

// LogSecurityEvent 记录安全事件
func (sm *SecurityMiddleware) LogSecurityEvent(event string, details map[string]interface{}) {
	log.Printf("[SECURITY] %s: %v", event, details)
}
