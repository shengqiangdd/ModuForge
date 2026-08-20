package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func alwaysEnabled(key string) bool  { return true }
func alwaysDisabled(key string) bool { return false }

func TestFeatureFlagMiddleware_Enabled_PassesThrough(t *testing.T) {
	app := fiber.New()
	checker := NewFeatureFlagChecker(alwaysEnabled)
	app.Use(checker.Middleware())
	app.Get("/api/v1/crash/report", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/crash/report", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for enabled feature, got %d", resp.StatusCode)
	}
}

func TestFeatureFlagMiddleware_Disabled_Returns501(t *testing.T) {
	app := fiber.New()
	checker := NewFeatureFlagChecker(alwaysDisabled)
	app.Use(checker.Middleware())
	app.Get("/api/v1/crash/report", func(c fiber.Ctx) error {
		return c.SendString("should not reach")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/crash/report", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 501 {
		t.Errorf("expected 501 for disabled feature, got %d", resp.StatusCode)
	}
}

func TestFeatureFlagMiddleware_UnmappedRoute_PassesThrough(t *testing.T) {
	app := fiber.New()
	checker := NewFeatureFlagChecker(alwaysDisabled)
	app.Use(checker.Middleware())
	app.Get("/api/v1/projects", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for unmapped route (fail-open), got %d", resp.StatusCode)
	}
}

func TestFeatureFlagMiddleware_CollaborationRoute(t *testing.T) {
	app := fiber.New()
	checker := NewFeatureFlagChecker(func(key string) bool {
		return key != "collaboration"
	})
	app.Use(checker.Middleware())
	app.Get("/api/v1/collaborators", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/collaborators", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 501 {
		t.Errorf("expected 501 for disabled collaboration, got %d", resp.StatusCode)
	}
}

func TestFeatureFlagMiddleware_BadgesRoute(t *testing.T) {
	app := fiber.New()
	checker := NewFeatureFlagChecker(func(key string) bool {
		return key != "badges"
	})
	app.Use(checker.Middleware())
	app.Get("/api/v1/badges/list", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/badges/list", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 501 {
		t.Errorf("expected 501 for disabled badges, got %d", resp.StatusCode)
	}
}

func TestFeatureFlagMiddleware_UnknownKey_FailOpen(t *testing.T) {
	app := fiber.New()
	// isEnabled always returns true, so even unknown features pass through
	checker := NewFeatureFlagChecker(alwaysEnabled)
	app.Use(checker.Middleware())
	app.Get("/api/v1/unknown-feature", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/unknown-feature", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for unknown route with fail-open, got %d", resp.StatusCode)
	}
}
