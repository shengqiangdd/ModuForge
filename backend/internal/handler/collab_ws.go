package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

// CollaborationWS handles real-time collaborative editing via WebSocket
type CollaborationWS struct {
	db *service.CollaborationService
}

func NewCollaborationWS(db *service.CollaborationService) *CollaborationWS {
	return &CollaborationWS{db: db}
}

type CollabOperation struct {
	Type       string `json:"type"` // "cursor", "insert", "delete", "replace", "join", "leave"
	FilePath   string `json:"file_path,omitempty"`
	Line       int    `json:"line,omitempty"`
	Column     int    `json:"column,omitempty"`
	Content    string `json:"content,omitempty"`
	Username   string `json:"username,omitempty"`
	UserID     string `json:"user_id,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	Length     int    `json:"length,omitempty"` // for delete
}

type CollabClient struct {
	Conn   *websocket.Conn
	UserID string
	ProjectID string
	mu     sync.Mutex
}

func (c *CollabClient) SendJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteJSON(v)
}

// ProjectCollabHub manages connected clients per project
type ProjectCollabHub struct {
	mu      sync.RWMutex
	clients map[string]map[string]*CollabClient // projectID -> userID -> client
}

var collabHub *ProjectCollabHub
var collabHubOnce sync.Once

func GetCollabHub() *ProjectCollabHub {
	collabHubOnce.Do(func() {
		collabHub = &ProjectCollabHub{
			clients: make(map[string]map[string]*CollabClient),
		}
	})
	return collabHub
}

func (h *ProjectCollabHub) Subscribe(projectID, userID string, client *CollabClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[projectID] == nil {
		h.clients[projectID] = make(map[string]*CollabClient)
	}
	h.clients[projectID][userID] = client
}

func (h *ProjectCollabHub) Unsubscribe(projectID, userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients, ok := h.clients[projectID]; ok {
		delete(clients, userID)
		if len(clients) == 0 {
			delete(h.clients, projectID)
		}
	}
}

func (h *ProjectCollabHub) BroadcastToProject(projectID string, op CollabOperation, excludeUserID string) {
	h.mu.RLock()
	clients := h.clients[projectID]
	h.mu.RUnlock()

	for uid, client := range clients {
		if uid == excludeUserID {
			continue
		}
		if err := client.SendJSON(op); err != nil {
			slog.Warn("collab broadcast error", "user_id", uid, "error", err)
		}
	}
}

func (h *ProjectCollabHub) GetOnlineUsers(projectID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var users []string
	if clients, ok := h.clients[projectID]; ok {
		for uid := range clients {
			users = append(users, uid)
		}
	}
	return users
}

// RegisterCollabWSRoute registers the collaboration WebSocket endpoint
func RegisterCollabWSRoute(app *fiber.App, jwtSecret string, collabSvc *service.CollaborationService) {
	hub := GetCollabHub()
	collabWS := NewCollaborationWS(collabSvc)

	wsHandler := websocket.New(func(c *websocket.Conn) {
		// Set read limit to prevent memory exhaustion (64KB per message)
		c.SetReadLimit(64 * 1024)

		// Extract project_id from URL: /ws/collaborate/:project_id
		projectID := c.Params("project_id")
		if projectID == "" {
			c.WriteJSON(fiber.Map{"error": "project_id required"})
			c.Close()
			return
		}

		// Authenticate
		userID := ""
		if uid, ok := c.Locals("user_id").(string); ok && uid != "" {
			userID = uid
		}
		if userID == "" {
			if uid, ok := c.Locals("uid").(string); ok && uid != "" {
				userID = uid
			}
		}
		if userID == "" {
			if token := c.Query("token"); token != "" {
				claims, err := service.ParseJWT(token, jwtSecret)
				if err == nil && claims != nil {
					userID = claims.UID
				}
			}
		}

		if userID == "" {
			c.WriteJSON(fiber.Map{"error": "未授权"})
			c.Close()
			return
		}

		// Verify project access: user must be owner or collaborator
		var accessOK bool
		err := collabWS.db.GetDB().QueryRow(
			`SELECT 1 FROM projects WHERE id = ? AND user_id = ?
			 UNION
			 SELECT 1 FROM team_members WHERE project_id = ? AND user_id = ?`,
			projectID, userID, projectID, userID,
		).Scan(&accessOK)
		if err != nil || !accessOK {
			c.WriteJSON(fiber.Map{"error": "无项目访问权限"})
			c.Close()
			return
		}

		// Get username
		username := userID
		var uname string
		if err := collabWS.db.GetDB().QueryRow(
			"SELECT COALESCE(username, id) FROM users WHERE id = ?", userID).Scan(&uname); err == nil && uname != "" {
			username = uname
		}

		// Derive a context cancelled when the WS connection closes.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		client := &CollabClient{
			Conn:      c,
			UserID:    userID,
			ProjectID: projectID,
		}
		hub.Subscribe(projectID, userID, client)
		defer hub.Unsubscribe(projectID, userID)

		// Store session in DB
		collabWS.db.UpsertEditSession(ctx, &service.EditSession{
			ID:         "collab_" + projectID + "_" + userID,
			ProjectID:  projectID,
			UserID:     userID,
			Username:   username,
			ConnectedAt: time.Now(),
			LastActive:  time.Now(),
		})
		defer collabWS.db.RemoveEditSession(ctx, "collab_"+projectID+"_"+userID)

		// Notify others that user joined
		hub.BroadcastToProject(projectID, CollabOperation{
			Type:     "join",
			UserID:   userID,
			Username: username,
		}, userID)

		// Send online users list
		onlineUsers := hub.GetOnlineUsers(projectID)
		client.SendJSON(CollabOperation{
			Type:    "online_users",
			Content: strings.Join(onlineUsers, ","),
		})

		slog.Info("collab connected", "project_id", projectID, "user_id", userID)

		// Read loop
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					slog.Warn("collab read error", "user_id", userID, "error", err)
				}
				break
			}

			var op CollabOperation
			if err := json.Unmarshal(msg, &op); err != nil {
				continue
			}

			op.UserID = userID
			op.Username = username
			op.SessionID = "collab_" + projectID + "_" + userID

			switch op.Type {
			case "cursor":
				// Update cursor position
				hub.BroadcastToProject(projectID, op, userID)
				// Update in DB
				collabWS.db.UpsertEditSession(ctx, &service.EditSession{
					ID:           op.SessionID,
					ProjectID:    projectID,
					UserID:       userID,
					Username:     username,
					FilePath:     op.FilePath,
					CursorLine:   op.Line,
					CursorCol:    op.Column,
					ConnectedAt:  time.Now(),
					LastActive:   time.Now(),
				})

			case "insert", "delete", "replace":
				// Broadcast operational transformation
				hub.BroadcastToProject(projectID, op, userID)

			default:
				// Reject unknown operation types to prevent abuse
				slog.Warn("collab unknown op type", "type", op.Type, "user_id", userID)
			}
		}

		slog.Info("collab disconnected", "project_id", projectID, "user_id", userID)
		hub.BroadcastToProject(projectID, CollabOperation{
			Type:     "leave",
			UserID:   userID,
			Username: username,
		}, userID)
	}, websocket.Config{
		HandshakeTimeout: 10 * time.Second,
		AllowEmptyOrigin: true,
		EnableCompression: true,
	})

	// Register the route - note: project_id is extracted inside the handler
	app.Get("/api/v1/ws/collaborate/:project_id", wsHandler)
}

// GetCollaborationStatus returns current collab status for a project
func (h *CollaborationWS) GetCollaborationStatus(c fiber.Ctx) error {
	projectID := c.Params("id")
	hub := GetCollabHub()
	onlineUsers := hub.GetOnlineUsers(projectID)

	// Get active sessions from DB
	sessions, err := h.db.ListEditSessions(c.Context(), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"online_users": onlineUsers,
		"sessions":     sessions,
	})
}