package cache

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// AICache AI 响应缓存
type AICache struct {
	cache *SmartCache
}

// NewAICache 创建 AI 缓存
func NewAICache() *AICache {
	return &AICache{
		cache: NewSmartCache(1000, 30*time.Minute),
	}
}

// GetCachedResponse 获取缓存的 AI 响应
func (ac *AICache) GetCachedResponse(prompt string, model string) (string, bool) {
	key := ac.generateKey(prompt, model)
	if val, ok := ac.cache.Get(key); ok {
		return val.(string), true
	}
	return "", false
}

// CacheResponse 缓存 AI 响应
func (ac *AICache) CacheResponse(prompt string, model string, response string, ttl time.Duration) {
	key := ac.generateKey(prompt, model)
	ac.cache.Set(key, response, ttl)
}

// GetStats 获取缓存统计
func (ac *AICache) GetStats() CacheStats {
	return ac.cache.GetStats()
}

func (ac *AICache) generateKey(prompt string, model string) string {
	h := sha256.New()
	h.Write([]byte(prompt))
	h.Write([]byte(model))
	return fmt.Sprintf("%x", h.Sum(nil))
}
