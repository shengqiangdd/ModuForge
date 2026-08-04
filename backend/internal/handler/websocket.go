package handler

import (
	"log/slog"
	"time"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

// RegisterWSRoute registers the WebSocket endpoint using gofiber/contrib/v3/websocket.
// This properly integrates with Fiber v3's middleware chain (unlike raw fasthttp/websocket).
func RegisterWSRoute(api fiber.Router, jwtSecret string) {
	// Register the WebSocket upgrade check as middleware,
	// and the actual WS handler after it.
	//
	// NOTE: gofiber/websocket handles the HTTP→WS upgrade internally.
	// We must NOT register a separate fiber.Handler before it that calls c.Next(),
	// because that would pass the already-upgraded context through.
	// Instead, we put auth logic INSIDE the websocket.New handler.
	wsHandler := websocket.New(func(c *websocket.Conn) {
		userID := ""

		// 1) Try Fiber locals (set by auth middleware if route is protected)
		if uid, ok := c.Locals("user_id").(string); ok && uid != "" {
			userID = uid
		}
		if userID == "" {
			if uid, ok := c.Locals("uid").(string); ok && uid != "" {
				userID = uid
			}
		}

		// 2) Fallback: extract token from query param (browser WS can't send headers)
		if userID == "" {
			if token := c.Query("token"); token != "" {
				claims, err := service.ParseJWT(token, jwtSecret)
				if err == nil && claims != nil {
					userID = claims.UID
				}
			}
		}

		if userID == "" {
			slog.Warn("ws: missing user_id")
			c.WriteJSON(fiber.Map{"error": "未授权"})
			c.Close()
			return
		}

		slog.Info("ws connected", "user_id", userID)

		hub := service.GetHub()
		client := hub.Subscribe(userID, c.Conn)
		defer func() {
			hub.Unsubscribe(userID, client)
			c.Close()
		}()

		// Send periodic pings to keep the connection alive
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		done := make(chan struct{})
		defer close(done)

		go func() {
			for {
				select {
				case <-ticker.C:
					client.SendJSON(service.WSEvent{Event: "ping", Data: nil})
				case <-done:
					return
				}
			}
		}()

		// Read loop — block until client disconnects
		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					slog.Warn("ws read error", "user_id", userID, "error", err)
				}
				break
			}
		}
	})

	// websocket.New returns a fiber.Handler that:
	// 1. Checks for WebSocket upgrade request
	// 2. If not a WS upgrade → returns 426 Upgrade Required
	// 3. If WS upgrade → performs the upgrade and calls our handler above
	api.Get("/ws", wsHandler)
}
