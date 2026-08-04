package main

import (
	"errors"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"

	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/database"
	"github.com/moduforge/backend/internal/handler"
	"github.com/moduforge/backend/internal/middleware"
	"github.com/moduforge/backend/internal/service"
	"github.com/moduforge/backend/internal/builder"
)

type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

func main() {
	cfg := config.Load()

	// Init SQLite DB (market tables)
	dbPath := cfg.DatabasePath
	if dbPath == "" {
		dbPath = "data/moduforge.db"
	}
	db, err := database.NewSQLiteDB(dbPath)
	if err != nil {
		slog.Error("Failed to init database", "error", err)
		log.Fatalf("Failed to init database: %v", err)
	}
	defer db.Close()

	// Seed data
	if err := db.SeedAdminUser(); err != nil {
		slog.Warn("Seed admin user failed", "error", err)
	}
	if err := db.SeedMarketData(); err != nil {
		slog.Warn("Seed market data failed", "error", err)
	}
	if err := db.SeedGlossary(); err != nil {
		slog.Warn("Seed glossary failed", "error", err)
	}

	// Init WebSocket hub (singleton)
	service.GetHub()

	// Fiber app
	app := fiber.New(fiber.Config{
		BodyLimit:  32 * 1024 * 1024, // 32 MB — auto-build sends large JSON payloads
		WriteTimeout: 25 * time.Minute, // SSE streams + long LLM generation + compilation
		ReadTimeout:  2 * time.Minute,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			// Check if it's a Fiber typed error (e.g. 404, 405) and use its status code
			var fiberErr *fiber.Error
			if errors.As(err, &fiberErr) {
				return c.Status(fiberErr.Code).JSON(apiError{Error: fiberErr.Message, Code: "HTTP_ERROR"})
			}
			// S3 fix: hide internal error details in production
			isDev := os.Getenv("MODUFORGE_DEV") == "1"
			msg := "internal server error"
			if isDev {
				msg = err.Error()
			}
			log.Printf("[ERROR] %v", err) // always log full error server-side
			return c.Status(500).JSON(apiError{Error: msg, Code: "INTERNAL_ERROR"})
		},
	})

	// Structured logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	// WebSocket — registered as pre-middleware (BEFORE all other middleware)
	// to completely bypass middleware chain for WS upgrade requests.
	handler.RegisterWSRawRoute(app, cfg.JWTSecret)

	// Screen stream WebSocket — also bypasses middleware for WS upgrade
	adbSvc := service.NewADBService(db.Conn)
	handler.RegisterScreenStreamWS(app, adbSvc, cfg.JWTSecret)

	// Collaboration WebSocket — real-time editing
	collabSvc := service.NewCollaborationService(db.Conn)
	handler.RegisterCollabWSRoute(app, cfg.JWTSecret, collabSvc)

	// Middleware (runs after WS pre-middleware for non-WS paths)
	app.Use(middleware.RequestID())
	app.Use(middleware.ContentTypeCheck())
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: `{"time":"${time}","method":"${method}","path":"${path}","status":${status},"latency":"${latency}","ip":"${ip}","request_id":"${locals:request_id}"}` + "\n",
		TimeFormat: time.RFC3339,
	}))
	app.Use(cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			// Allow same-origin requests (no Origin header) and localhost for development
			if origin == "" {
				return true
			}
			// In production, restrict to specific origins via environment variable
			allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
			if allowedOrigins != "" {
				for _, o := range strings.Split(allowedOrigins, ",") {
					if strings.TrimSpace(o) == origin {
						return true
					}
				}
				return false
			}
			// Default: allow all origins (development mode)
			return true
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
	}))
	app.Use(compress.New(compress.Config{
		Next: func(c fiber.Ctx) bool {
			path := c.Path()
			// Skip compression for WebSocket and SSE streaming endpoints
			return strings.Contains(path, "/ws") ||
				strings.Contains(path, "/stream") ||
				strings.HasSuffix(path, "/chat") ||
				strings.HasSuffix(path, "/generate") ||
				strings.HasSuffix(path, "/gather") ||
				strings.HasSuffix(path, "/repair") ||
				strings.HasSuffix(path, "/agent/run") ||
				strings.HasSuffix(path, "/auto-build") ||
				strings.HasSuffix(path, "/logs")
		},
	}))

	// API rate limiting (S2 security fix: 30 req/s per IP, burst 50)
	rl := middleware.NewRateLimiter()
	app.Use(middleware.RateLimit(rl, 50, 30))

	// API routes
	api := app.Group("/api/v1")
	handler.RegisterRoutes(api, db, cfg)

	// Docs
	handler.RegisterDocs(api, cfg.StoragePath+"/../docs")
	app.Get("/docs", handler.DocsRedirect)

	// Static uploads (screenshots, etc.)
	uploadsDir := cfg.StoragePath + "/screenshots"
	if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
		os.MkdirAll(uploadsDir, 0755)
	}
	app.Get("/uploads/*", func(c fiber.Ctx) error {
		// Sanitize path to prevent traversal attacks
		fileParam := c.Params("*")
		cleanPath := filepath.Clean(fileParam)
		if strings.Contains(cleanPath, "..") {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		filePath := filepath.Join(cfg.StoragePath, cleanPath)
		// Ensure the resolved path is still within storageDir
		absStorage, _ := filepath.Abs(cfg.StoragePath)
		absFile, _ := filepath.Abs(filePath)
		if !strings.HasPrefix(absFile, absStorage) {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		return c.SendFile(filePath)
	})

	// Health check
	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"version": "2.0-lite",
		})
	})

	// Serve frontend static files from /app/dist (Docker) or ./dist (local)
	distDir := os.Getenv("DIST_DIR")
	if distDir == "" {
		distDir = "/app/dist"
	}
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		distDir = "../frontend/dist"
	}
	if _, err := os.Stat(distDir); err == nil {
		frontendFS, err := fs.Sub(os.DirFS(distDir), ".")
		if err == nil {
			serveFrontend(app, frontendFS)
			slog.Info("Frontend served", "dist_dir", distDir)
		}
	} else {
		slog.Warn("No frontend dist found, serving API only")
	}

	// Scheduled backup timer (runs every hour)
	backupSvc := service.NewBackupService(db.Conn, cfg.StoragePath+"/backups")
	recycleH := handler.NewRecycleHandler(db.Conn)
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			backupSvc.RunScheduledBackups()
			recycleH.CleanupExpiredItems()
		}
	}()

	// Global cache manager — periodic cleanup to prevent disk bloat
	cacheMgr := builder.NewGlobalCacheManager(builder.DefaultCacheConfig(cfg.StoragePath))
	cacheMgr.Start()
	defer cacheMgr.Stop()

	// Graceful shutdown
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig
		app.Shutdown()
	}()

	slog.Info("Starting server", "port", cfg.Port)
	if err := app.Listen(cfg.Port); err != nil {
		slog.Error("Server failed", "error", err)
		log.Fatalf("listen: %v", err)
	}
}

// serveFrontend 注册 SPA 静态文件路由
func serveFrontend(app *fiber.App, fsys fs.FS) {
	// Content type map
	ctMap := map[string]string{
		".js":   "application/javascript",
		".mjs":  "application/javascript",
		".css":  "text/css",
		".html": "text/html; charset=utf-8",
		".json": "application/json",
		".svg":  "image/svg+xml",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".ico":  "image/x-icon",
		".woff": "font/woff",
		".woff2": "font/woff2",
		".ttf":  "font/ttf",
		".map":  "application/json",
	}

	app.Use(func(c fiber.Ctx) error {
		path := c.Path()

		// Skip API, WebSocket, health, docs routes — let them through
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws") || path == "/health" || path == "/docs" || strings.HasPrefix(path, "/uploads/") {
			return c.Next()
		}

		// Clean the path
		relPath := strings.TrimPrefix(path, "/")
		if relPath == "" {
			relPath = "index.html"
		}

		// Try to serve static file
		f, err := fsys.Open(relPath)
		if err == nil {
			defer f.Close()
			stat, _ := f.Stat()
			if stat != nil && !stat.IsDir() {
				ext := relPath[strings.LastIndex(relPath, "."):]
				if ct, ok := ctMap[ext]; ok {
					c.Set("Content-Type", ct)
				}
				data, _ := fs.ReadFile(fsys, relPath)
				return c.Send(data)
			}
		}

		// SPA fallback: serve index.html for all non-file routes
		data, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			return c.Next() // no frontend, pass through
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(data)
	})
}
