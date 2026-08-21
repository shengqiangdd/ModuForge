package middleware

import (
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

type TokenBucket struct {
	capacity   float64
	rate       float64
	tokens     float64
	lastCheck  time.Time
	lastAccess time.Time
	mu         sync.Mutex
}

func NewTokenBucket(capacity, rate float64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		rate:       rate,
		tokens:     capacity,
		lastCheck:  time.Now(),
		lastAccess: time.Now(),
	}
}

func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.lastAccess = time.Now()
	now := time.Now()
	elapsed := now.Sub(tb.lastCheck).Seconds()
	tb.tokens = tb.tokens + elapsed*tb.rate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastCheck = now

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

type RateLimiter struct {
	buckets map[string]*TokenBucket
	mu      sync.RWMutex
}

func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*TokenBucket),
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for ip, b := range rl.buckets {
			b.mu.Lock()
			idle := time.Since(b.lastAccess)
			b.mu.Unlock()
			if idle > 10*time.Minute {
				delete(rl.buckets, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) getBucket(key string, capacity, rate float64) *TokenBucket {
	rl.mu.RLock()
	b, ok := rl.buckets[key]
	rl.mu.RUnlock()
	if ok {
		return b
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()
	if b, ok = rl.buckets[key]; ok {
		return b
	}
	b = NewTokenBucket(capacity, rate)
	rl.buckets[key] = b
	return b
}

func RateLimit(rl *RateLimiter, capacity, rate float64) fiber.Handler {
	return func(c fiber.Ctx) error {
		ip := c.IP()
		b := rl.getBucket(ip, capacity, rate)
		if !b.Allow() {
			return c.Status(429).JSON(fiber.Map{
				"error":       "请求过于频繁，请稍后再试",
				"code":        "RATE_LIMITED",
				"retry_after": "1s",
			})
		}
		return c.Next()
	}
}

// RateLimitWithSkip is a global DoS-guard limiter that ALSO skips a set of
// path prefixes. Use it for a coarse whole-app guard: endpoints that are
// purely local computation (e.g. /repo/smart-select) must NOT be throttled —
// they consume no external resource and a tight global bucket would wrongly
// 429 them (previously happened because a global 50/30 bucket shared between
// /repo/tree and /repo/smart-select drained the token in the same second).
func RateLimitWithSkip(rl *RateLimiter, capacity, rate float64, skip ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		path := c.Path()
		for _, pre := range skip {
			if strings.HasPrefix(path, pre) {
				return c.Next() // skip throttling for local-compute endpoints
			}
		}
		ip := c.IP()
		b := rl.getBucket(ip, capacity, rate)
		if !b.Allow() {
			return c.Status(429).JSON(fiber.Map{
				"error":       "请求过于频繁，请稍后再试",
				"code":        "RATE_LIMITED",
				"retry_after": "1s",
			})
		}
		return c.Next()
	}
}
