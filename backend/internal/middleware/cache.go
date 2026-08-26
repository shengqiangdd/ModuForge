package middleware

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
)

// QueryCache 查询结果缓存中间件
type QueryCache struct {
	cache map[string]cacheEntry
	ttl   time.Duration
}

type cacheEntry struct {
	data      []byte
	expiresAt time.Time
}

// NewQueryCache 创建查询缓存
func NewQueryCache(ttl time.Duration) *QueryCache {
	return &QueryCache{
		cache: make(map[string]cacheEntry),
		ttl:   ttl,
	}
}

// Handler 缓存处理器
func (qc *QueryCache) Handler(c fiber.Ctx) error {
	// 只缓存GET请求
	if c.Method() != "GET" {
		return c.Next()
	}

	// 生成缓存键
	key := qc.generateKey(c)

	// 检查缓存
	if entry, ok := qc.cache[key]; ok {
		if time.Now().Before(entry.expiresAt) {
			c.Set("X-Cache", "HIT")
			return c.Status(200).Send(entry.data)
		}
		delete(qc.cache, key)
	}

	// 执行请求
	if err := c.Next(); err != nil {
		return err
	}

	// 缓存响应
	body := c.Response().Body()
	if len(body) > 0 && c.Response().StatusCode() == 200 {
		qc.cache[key] = cacheEntry{
			data:      body,
			expiresAt: time.Now().Add(qc.ttl),
		}
		c.Set("X-Cache", "MISS")
	}

	return nil
}

func (qc *QueryCache) generateKey(c fiber.Ctx) string {
	h := sha256.New()
	h.Write([]byte(c.Method()))
	h.Write([]byte(c.Path()))
	h.Write([]byte(c.Request().URI().QueryString()))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Cleanup 清理过期缓存
func (qc *QueryCache) Cleanup() {
	now := time.Now()
	for key, entry := range qc.cache {
		if now.After(entry.expiresAt) {
			delete(qc.cache, key)
		}
	}
}
