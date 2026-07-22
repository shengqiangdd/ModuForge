package handler

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gofiber/websocket/v2"
	"github.com/moduforge/backend/internal/service"
)

type WSHandler struct {
	ws *service.WebSocketService
}

func NewWSHandler(ws *service.WebSocketService) *WSHandler {
	return &WSHandler{ws: ws}
}

type wsInboundMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type cursorPayload struct {
	ProjectID string `json:"project_id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	FilePath  string `json:"file"`
	Line      int    `json:"line"`
	Col       int    `json:"col"`
	Color     string `json:"color"`
}

type selectionPayload struct {
	ProjectID  string `json:"project_id"`
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	FilePath   string `json:"file"`
	StartLine  int    `json:"start_line"`
	StartCol   int    `json:"start_col"`
	EndLine    int    `json:"end_line"`
	EndCol     int    `json:"end_col"`
	Color      string `json:"color"`
}

type editPayload struct {
	ProjectID string `json:"project_id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	FilePath  string `json:"file"`
	Content   string `json:"content"`
}

type joinPayload struct {
	ProjectID string `json:"project_id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Color     string `json:"color"`
}

type leavePayload struct {
	ProjectID string `json:"project_id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
}

func (h *WSHandler) HandleConnection(c *websocket.Conn) {
	clientID := c.Query("uid", c.RemoteAddr().String())
	projectID := c.Query("project_id", "")

	client := &service.WSClient{
		Conn:      c,
		Send:      make(chan []byte, 256),
		ID:        clientID,
		ProjectID: projectID,
	}

	h.ws.Register() <- client

	// Writer goroutine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer func() {
			ticker.Stop()
			c.Close()
		}()
		for {
			select {
			case message, ok := <-client.Send:
				if !ok {
					c.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}
				c.WriteMessage(websocket.TextMessage, message)
			case <-ticker.C:
				c.WriteMessage(websocket.PingMessage, nil)
			}
		}
	}()

	// Reader loop — handle collaboration messages
	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			break
		}

		var inbound wsInboundMessage
		if err := json.Unmarshal(msg, &inbound); err != nil {
			log.Printf("[WS] Invalid message from %s: %v", clientID, err)
			continue
		}

		switch inbound.Type {
		case "collab_cursor_update":
			var p cursorPayload
			if json.Unmarshal(inbound.Payload, &p) == nil && p.ProjectID != "" {
				h.ws.BroadcastCursorUpdate(p.ProjectID, p.UserID, p.Username, p.FilePath, p.Line, p.Col, p.Color)
			}

		case "collab_selection_update":
			var p selectionPayload
			if json.Unmarshal(inbound.Payload, &p) == nil && p.ProjectID != "" {
				h.ws.BroadcastSelectionUpdate(p.ProjectID, p.UserID, p.Username, p.FilePath, p.StartLine, p.StartCol, p.EndLine, p.EndCol, p.Color)
			}

		case "collab_edit":
			var p editPayload
			if json.Unmarshal(inbound.Payload, &p) == nil && p.ProjectID != "" {
				h.ws.BroadcastEdit(p.ProjectID, p.UserID, p.Username, p.FilePath, p.Content)
			}

		case "collab_join":
			var p joinPayload
			if json.Unmarshal(inbound.Payload, &p) == nil && p.ProjectID != "" {
				h.ws.BroadcastJoin(p.ProjectID, p.UserID, p.Username, p.Color)
			}

		case "collab_leave":
			var p leavePayload
			if json.Unmarshal(inbound.Payload, &p) == nil && p.ProjectID != "" {
				h.ws.BroadcastLeave(p.ProjectID, p.UserID, p.Username)
			}

		default:
			log.Printf("[WS] Unknown message type from %s: %s", clientID, inbound.Type)
		}
	}

	h.ws.Unregister() <- client
}
