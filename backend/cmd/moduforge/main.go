package main

import (
	"log"
	"os"
	"os/signal"

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

	// Graceful shutdown
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig
		app.Shutdown()
	}()

	log.Printf("ModuForge Lite starting on %s", cfg.Port)
	if err := app.Listen(cfg.Port); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
