package main

import (
	"io/fs"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	fiberws "github.com/gofiber/websocket/v2"

	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/database"
	"github.com/moduforge/backend/internal/handler"
	"github.com/moduforge/backend/internal/service"
)

func main() {
	cfg := config.Load()

	// Init SQLite DB (market tables)
	dbPath := cfg.DatabasePath
	if dbPath == "" {
		dbPath = "data/moduforge.db"
	}
	db, err := database.NewSQLiteDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}
	defer db.Close()

	// Seed market data
	if err := db.SeedMarketData(); err != nil {
		log.Printf("Warning: seed market data failed: %v", err)
	}

	// Init WebSocket service
	wsService := service.NewWebSocketService()
	go wsService.Run()

	// Fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		},
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())
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
	distDir := "/app/dist"
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		distDir = "../frontend/dist"
	}
	if _, err := os.Stat(distDir); err == nil {
		frontendFS, err := fs.Sub(os.DirFS(distDir), ".")
		if err == nil {
			serveFrontend(app, frontendFS)
			log.Printf("Frontend served from %s", distDir)
		}
	} else {
		log.Printf("Warning: no frontend dist found, serving API only")
	}

	// Graceful shutdown
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig
		app.Shutdown()
	}()

	log.Printf("ModuForge starting on %s", cfg.Port)
	if err := app.Listen(cfg.Port); err != nil {
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

	app.Get("/*", func(c fiber.Ctx) error {
		path := c.Path()

		// Skip API and WebSocket routes
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws") || path == "/health" {
			return c.Next()
		}

		// Clean the path
		relPath := strings.TrimPrefix(path, "/")
		if relPath == "" {
			relPath = "index.html"
		}

		// Try to open the file
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

		// SPA fallback: serve index.html
		data, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(data)
	})
}
