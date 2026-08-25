package handler

import (
	"strings"
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
	entries sync.Map
	ttl     time.Duration
	stopCh  chan struct{}
}

var globalCache *ResponseCache
var cacheOnce sync.Once

// defaultCacheTTL is the fallback TTL when no explicit value is configured.
const defaultCacheTTL = 30 * time.Second

// GetCache returns the global response cache singleton.
// Call SetTTL after GetCache to override the default TTL.
func GetCache() *ResponseCache {
	cacheOnce.Do(func() {
		globalCache = &ResponseCache{
			ttl:    defaultCacheTTL,
			stopCh: make(chan struct{}),
		}
		go globalCache.cleanup()
	})
	return globalCache
}

// SetTTL updates the TTL for future cache entries.
func (c *ResponseCache) SetTTL(ttl time.Duration) {
	if ttl > 0 {
		c.ttl = ttl
	}
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

// cacheExcludePrefixes lists path prefixes that must never be cached.
// These involve authentication, WebSocket upgrades, or real-time data.
var cacheExcludePrefixes = []string{
	"/api/v1/agent/",
	"/api/v1/ws",
	"/health",
}

// shouldExcludePath returns true if the request path matches any exclusion rule.
func shouldExcludePath(path string) bool {
	for _, prefix := range cacheExcludePrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// CacheMiddleware creates a Fiber middleware that caches GET responses.
// Only 2xx GET responses are cached. Excluded paths and non-GET methods
// pass through without touching the cache.
//
// Headers set:
//   - X-Cache: HIT | MISS
//   - Cache-Control: public, max-age=<ttl> (on cacheable responses)
//   - Vary: Authorization (ensures per-user cache isolation)
func CacheMiddleware(cache *ResponseCache) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Only cache GET requests
		if c.Method() != fiber.MethodGet {
			return c.Next()
		}

		path := c.Path()

		// Skip excluded endpoints (auth, websocket, health)
		if shouldExcludePath(path) {
			c.Set("Cache-Control", "no-store")
			return c.Next()
		}

		// Build cache key: method + full original URL (path + query)
		key := c.Method() + ":" + c.OriginalURL()

		// Check cache
		if data, status, ok := cache.Get(key); ok {
			c.Set("X-Cache", "HIT")
			c.Set("Cache-Control", "public, max-age="+formatDuration(cache.ttl))
			c.Set("Vary", "Authorization")
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

		// Set cache headers on the (non-cached) response too
		c.Set("X-Cache", "MISS")
		c.Set("Cache-Control", "public, max-age="+formatDuration(cache.ttl))
		c.Set("Vary", "Authorization")

		return err
	}
}

// formatDuration formats a Duration as seconds for Cache-Control max-age.
func formatDuration(d time.Duration) string {
	return formatInt(int(d.Seconds()))
}

func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
