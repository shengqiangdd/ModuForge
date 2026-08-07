package middleware

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	_ "github.com/mattn/go-sqlite3"
	"github.com/moduforge/backend/internal/config"
)

func TestJWTAuth_MissingToken(t *testing.T) {
	app := fiber.New()
	cfg := &config.Config{JWTSecret: "test-secret"}
	app.Get("/protected", JWTAuth(cfg), func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestJWTAuth_ValidToken(t *testing.T) {
	app := fiber.New()
	cfg := &config.Config{JWTSecret: "test-secret"}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-123",
	})
	tokenStr, _ := token.SignedString([]byte(cfg.JWTSecret))

	app.Get("/protected", JWTAuth(cfg), func(c fiber.Ctx) error {
		uid := c.Locals("user_id")
		if uid != "user-123" {
			t.Errorf("expected user_id user-123, got %v", uid)
		}
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	app := fiber.New()
	cfg := &config.Config{JWTSecret: "test-secret"}

	app.Get("/protected", JWTAuth(cfg), func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected?token=invalid-token", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestJWTAuth_QueryParam(t *testing.T) {
	app := fiber.New()
	cfg := &config.Config{JWTSecret: "test-secret"}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-456",
	})
	tokenStr, _ := token.SignedString([]byte(cfg.JWTSecret))

	app.Get("/protected", JWTAuth(cfg), func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected?token="+tokenStr, nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSecurityHeaders(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeaders())
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, _ := app.Test(req)

	checks := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "SAMEORIGIN",
		"X-XSS-Protection":       "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for key, expected := range checks {
		got := resp.Header.Get(key)
		if got != expected {
			t.Errorf("header %s: expected %q, got %q", key, expected, got)
		}
	}

	// Check cache headers for API routes
	apiReq := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	apiResp, _ := app.Test(apiReq)
	if apiResp.Header.Get("Cache-Control") != "no-store, no-cache, must-revalidate, private" {
		t.Errorf("API route should have cache-control header, got %q", apiResp.Header.Get("Cache-Control"))
	}
}

func TestSecurityHeadersHSTS(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeadersHSTS())
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, _ := app.Test(req)

	if got := resp.Header.Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains; preload" {
		t.Errorf("expected HSTS header, got %q", got)
	}
}

func TestTokenBucket(t *testing.T) {
	tb := NewTokenBucket(10, 10)

	// Should allow immediately
	if !tb.Allow() {
		t.Error("expected Allow() to return true for initial tokens")
	}

	// Consume all tokens
	for i := 0; i < 9; i++ {
		tb.Allow()
	}

	// Should be empty now
	if tb.Allow() {
		t.Error("expected Allow() to return false when bucket is empty")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	tb := NewTokenBucket(5, 100) // high rate for quick refill

	// Consume all
	for i := 0; i < 5; i++ {
		tb.Allow()
	}

	// Wait for refill
	time.Sleep(50 * time.Millisecond)

	// Should have refilled at least 5 tokens (100/s * 0.05s = 5)
	if !tb.Allow() {
		t.Error("expected Allow() to return true after refill")
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter()
	app := fiber.New()
	app.Use(RateLimit(rl, 5, 10))

	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	// First 5 requests should succeed
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		resp, _ := app.Test(req)
		if resp.StatusCode != 200 {
			t.Errorf("request %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}

	// 6th request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	resp, _ := app.Test(req)
	if resp.StatusCode != 429 {
		t.Errorf("expected 429 for rate limited request, got %d", resp.StatusCode)
	}
}

func TestAPIKeyAuth_WithDB(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Skipf("sqlite3 not available: %v", err)
	}
	defer db.Close()

	// Verify the driver works
	if err := db.Ping(); err != nil {
		t.Skipf("sqlite3 ping failed: %v", err)
	}

	// Create api_keys table and insert a test key
	if _, err := db.Exec(`CREATE TABLE api_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		name TEXT NOT NULL,
		key_hash TEXT NOT NULL,
		key_prefix TEXT NOT NULL,
		permissions TEXT DEFAULT '["read"]',
		last_used_at DATETIME,
		expires_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Hash of "test-key-123"
	h := sha256.New()
	h.Write([]byte("test-key-123"))
	hash := fmt.Sprintf("%x", h.Sum(nil))

	if _, err := db.Exec(`INSERT INTO api_keys (user_id, name, key_hash, key_prefix, permissions) VALUES (?, ?, ?, ?, ?)`,
		"user-1", "test-key", hash, "test", `["read","write"]`); err != nil {
		t.Fatalf("failed to insert key: %v", err)
	}

	app := fiber.New()
	app.Use(APIKeyAuth(db))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	t.Run("valid key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-API-Key", "test-key-123")
		resp, _ := app.Test(req)
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("invalid key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-API-Key", "wrong-key")
		resp, _ := app.Test(req)
		if resp.StatusCode != 401 {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("no key header passes through", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		resp, _ := app.Test(req)
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}