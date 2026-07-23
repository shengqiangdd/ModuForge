package main

import (
	"io/fs"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	fiberws "github.com/gofiber/contrib/v3/websocket"

	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/database"
	"github.com/moduforge/backend/internal/handler"
	"github.com/moduforge/backend/internal/middleware"
	"github.com/moduforge/backend/internal/service"
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

	// Seed market data
	if err := db.SeedMarketData(); err != nil {
		slog.Warn("Seed market data failed", "error", err)
	}

	// Init WebSocket service
	wsService := service.NewWebSocketService()
	go wsService.Run()

	// Fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			return c.Status(500).JSON(apiError{Error: err.Error(), Code: "INTERNAL_ERROR"})
		},
	})

	// Structured logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	// Middleware
	app.Use(middleware.RequestID())
	app.Use(middleware.ContentTypeCheck())
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: `{"time":"${time}","method":"${method}","path":"${path}","status":${status},"latency":"${latency}","ip":"${ip}","request_id":"${locals:request_id}"}` + "\n",
		TimeFormat: time.RFC3339,
	}))
	app.Use(cors.New())
	app.Use(compress.New())

	// API routes
	api := app.Group("/api/v1")
	handler.RegisterRoutes(api, db, cfg)

	// WebSocket handler
	wsHandler := handler.NewWSHandler(wsService)
	app.Get("/ws", fiberws.New(func(c *fiberws.Conn) {
		wsHandler.HandleConnection(c)
	}))

	// Health check
	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"version": "2.0-lite",
			"ws":      wsService.ClientCount(),
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

		// Skip API, WebSocket, health routes — let them through
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws") || path == "/health" {
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
