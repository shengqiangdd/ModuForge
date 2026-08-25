package handler

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/database"
	"github.com/moduforge/backend/internal/middleware"
	"github.com/moduforge/backend/internal/service"
	"github.com/moduforge/backend/internal/storage"
)

func reg3(api fiber.Router, method, path string, mw1, mw2, mw3 fiber.Handler, handler fiber.Handler) {
	switch method {
	case "GET":
		api.Get(path, mw1, mw2, mw3, handler)
	case "POST":
		api.Post(path, mw1, mw2, mw3, handler)
	case "PUT":
		api.Put(path, mw1, mw2, mw3, handler)
	case "DELETE":
		api.Delete(path, mw1, mw2, mw3, handler)
	}
}

func reg4(api fiber.Router, method, path string, mw1, mw2, mw3, mw4 fiber.Handler, handler fiber.Handler) {
	switch method {
	case "GET":
		api.Get(path, mw1, mw2, mw3, mw4, handler)
	case "POST":
		api.Post(path, mw1, mw2, mw3, mw4, handler)
	case "PUT":
		api.Put(path, mw1, mw2, mw3, mw4, handler)
	case "DELETE":
		api.Delete(path, mw1, mw2, mw3, mw4, handler)
	}
}

// routeContext holds all shared state for route registration across sub-files.
type routeContext struct {
	api      fiber.Router
	db       *database.DB
	cfg      *config.Config
	fileRepo *service.FileContentRepo
	cache    *ResponseCache
	rateRepo fiber.Handler
	authMW   fiber.Handler
	jwtMW    fiber.Handler
	rateAuth fiber.Handler
	rateAI   fiber.Handler
	adminMW  fiber.Handler
	aiSem    chan struct{}
}

// r registers a protected route.
func (ctx *routeContext) r(method, path string, h fiber.Handler) {
	reg3(ctx.api, method, path, ctx.authMW, ctx.jwtMW, ctx.rateAuth, h)
}

// rA registers a rate-limited AI route with concurrency guard.
func (ctx *routeContext) rA(method, path string, h fiber.Handler) {
	wrapped := func(c fiber.Ctx) error {
		select {
		case ctx.aiSem <- struct{}{}:
		default:
			return c.Status(429).JSON(fiber.Map{
				"error": "AI 并发忙，请稍后再试",
				"code":  "AI_BUSY",
			})
		}
		defer func() { <-ctx.aiSem }()
		return h(c)
	}
	reg3(ctx.api, method, path, ctx.authMW, ctx.jwtMW, ctx.rateAI, wrapped)
}

// rAdmin registers an admin-only route.
func (ctx *routeContext) rAdmin(method, path string, h fiber.Handler) {
	reg4(ctx.api, method, path, ctx.authMW, ctx.jwtMW, ctx.rateAuth, ctx.adminMW, h)
}

func RegisterRoutes(api fiber.Router, db *database.DB, cfg *config.Config) {
	rateLimiter := middleware.NewRateLimiter()
	cache := GetCache()
	cache.SetTTL(cfg.CacheResponseTTL)
	authMW := middleware.APIKeyAuth(db.Conn)
	jwtMW := AuthMiddleware(cfg.JWTSecret)
	rateAuth := middleware.RateLimit(rateLimiter, cfg.RateLimitAuth, cfg.RateLimitAuth/60)
	rateAI := middleware.RateLimit(rateLimiter, cfg.RateLimitAI, cfg.RateLimitAI/60)
	rateRepo := middleware.RateLimit(rateLimiter, cfg.RateLimitRepo, cfg.RateLimitRepo/60)
	aiSem := make(chan struct{}, cfg.MaxAIConcurrency)
	adminMW := AdminOnly()

	var s3adapter *storage.S3Adapter
	if cfg.S3Endpoint != "" {
		var s3err error
		for i := 0; i < 30; i++ {
			s3adapter, s3err = storage.NewS3Adapter(storage.S3Config{
				Endpoint: cfg.S3Endpoint, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey,
				Bucket: cfg.S3Bucket, Prefix: "projects", Secure: false,
			})
			if s3err == nil {
				slog.Info("S3 storage enabled", "endpoint", cfg.S3Endpoint, "bucket", cfg.S3Bucket)
				break
			}
			slog.Warn("S3 storage init failed, retrying...", "attempt", i+1, "error", s3err)
			time.Sleep(1 * time.Second)
		}
		if s3err != nil {
			slog.Warn("S3 storage init failed after 30 retries, falling back to legacy storage", "error", s3err)
		}
	}
	fileRepo := service.NewFileContentRepo(db.Conn, s3adapter)

	ctx := &routeContext{
		api: api, db: db, cfg: cfg, fileRepo: fileRepo, cache: cache, rateRepo: rateRepo,
		authMW: authMW, jwtMW: jwtMW, rateAuth: rateAuth, rateAI: rateAI, adminMW: adminMW, aiSem: aiSem,
	}

	// OpenAPI
	openapiH := NewOpenAPIHandler()
	api.Get("/openapi.json", openapiH.ServeJSON)
	api.Get("/openapi.yaml", openapiH.ServeYAML)
	api.Get("/docs", openapiH.ServeSwaggerUI)

	// Auth public (stricter rate limit)
	rateAuthStrict := middleware.RateLimit(rateLimiter, 15, 10)
	authSvc := service.NewAuthService(db.Conn, cfg)
	authH := NewAuthHandler(authSvc)
	api.Post("/auth/register", rateAuthStrict, authH.Register)
	api.Post("/auth/login", rateAuthStrict, authH.Login)
	api.Post("/auth/refresh", authH.Refresh)
	api.Post("/auth/verify-email", authH.VerifyEmail)
	api.Post("/auth/forgot-password", rateAuthStrict, authH.ForgotPassword)
	api.Post("/auth/reset-password", rateAuthStrict, authH.ResetPassword)

	// Feature flags
	featureFlagSvc := service.NewFeatureFlagService(db.Conn)
	featureFlagH := NewFeatureFlagHandler(featureFlagSvc)
	go featureFlagSvc.Refresh(context.Background())
	featureFlagMW := middleware.NewFeatureFlagChecker(featureFlagSvc.IsEnabled).Middleware()

	// Apply feature flag middleware to the protected API group
	api.Use(featureFlagMW)

	// Global API response cache (only affects GET; excluded paths pass through)
	api.Use(CacheMiddleware(cache))

	// Admin feature flags
	ctx.rAdmin("GET", "/admin/feature-flags", featureFlagH.List)
	ctx.rAdmin("PUT", "/admin/feature-flags/:key", featureFlagH.Update)
	ctx.rAdmin("POST", "/admin/feature-flags/batch", featureFlagH.BatchUpdate)

	// Sub-register
	registerCoreRoutes(ctx, authH)
	registerAIRoutes(ctx)
	registerMarketRoutes(ctx)
	registerDeviceRoutes(ctx)
}
