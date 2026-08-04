package handler

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

// cacheEntry stores a cached response
type cacheEntry struct {
	data      []byte
	status    int
	expiresAt time.Time
}

// ResponseCache is an in-memory TTL cache for HTTP responses
type ResponseCache struct {
	entries  sync.Map
	ttl      time.Duration
	stopCh   chan struct{}
}

var globalCache *ResponseCache
var cacheOnce sync.Once

// GetCache returns the global response cache singleton
func GetCache() *ResponseCache {
	cacheOnce.Do(func() {
		globalCache = &ResponseCache{
			ttl:    5 * time.Minute,
			stopCh: make(chan struct{}),
		}
		go globalCache.cleanup()
	})
	return globalCache
}

// cleanup periodically removes expired entries
func (c *ResponseCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			c.entries.Range(func(key, value interface{}) bool {
				entry := value.(*cacheEntry)
				if now.After(entry.expiresAt) {
					c.entries.Delete(key)
				}
				return true
			})
		case <-c.stopCh:
			return
		}
	}
}

// Get retrieves a cached response if available and not expired
func (c *ResponseCache) Get(key string) ([]byte, int, bool) {
	val, ok := c.entries.Load(key)
	if !ok {
		return nil, 0, false
	}
	entry := val.(*cacheEntry)
	if time.Now().After(entry.expiresAt) {
		c.entries.Delete(key)
		return nil, 0, false
	}
	return entry.data, entry.status, true
}

// Set stores a response in the cache
func (c *ResponseCache) Set(key string, data []byte, status int) {
	c.entries.Store(key, &cacheEntry{
		data:      data,
		status:    status,
		expiresAt: time.Now().Add(c.ttl),
	})
}

// Clear removes all entries from the cache
func (c *ResponseCache) Clear() int {
	count := 0
	c.entries.Range(func(key, value interface{}) bool {
		c.entries.Delete(key)
		count++
		return true
	})
	return count
}

// Size returns the number of cached entries
func (c *ResponseCache) Size() int {
	count := 0
	c.entries.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// CacheMiddleware creates a Fiber middleware that caches GET responses
func CacheMiddleware(cache *ResponseCache) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Only cache GET requests
		if c.Method() != fiber.MethodGet {
			return c.Next()
		}

		// Build cache key from method + path + query string
		key := c.Method() + ":" + c.OriginalURL()

		// Check cache
		if data, status, ok := cache.Get(key); ok {
			c.Set("X-Cache", "HIT")
			c.Set("Content-Type", "application/json")
			return c.Status(status).Send(data)
		}

		// Cache miss — execute the handler
		err := c.Next()

		// Store successful responses (2xx) in cache
		if err == nil && c.Response().StatusCode() >= 200 && c.Response().StatusCode() < 300 {
			body := c.Response().Body()
			if len(body) > 0 {
				cache.Set(key, body, c.Response().StatusCode())
			}
		}

		return err
	}
}
