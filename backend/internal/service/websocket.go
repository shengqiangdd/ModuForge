package service

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gofiber/contrib/v3/websocket"
)

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type WSClient struct {
	Conn      *websocket.Conn
	Send      chan []byte
	ID        string
	ProjectID string
}

type WebSocketService struct {
	clients    map[string]*WSClient
	projectSub map[string]map[string]bool // projectID -> set of client IDs
	mu         sync.RWMutex
	broadcast  chan []byte
	register   chan *WSClient
	unregister chan *WSClient
}

func NewWebSocketService() *WebSocketService {
	return &WebSocketService{
		clients:    make(map[string]*WSClient),
		projectSub: make(map[string]map[string]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
	}
}

func (s *WebSocketService) Run() {
	for {
		select {
		case client := <-s.register:
			s.mu.Lock()
			s.clients[client.ID] = client
			if client.ProjectID != "" {
				if s.projectSub[client.ProjectID] == nil {
					s.projectSub[client.ProjectID] = make(map[string]bool)
				}
				s.projectSub[client.ProjectID][client.ID] = true
			}
			s.mu.Unlock()
			log.Printf("[WS] Client connected: %s (project: %s)", client.ID, client.ProjectID)

		case client := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[client.ID]; ok {
				delete(s.clients, client.ID)
				if client.ProjectID != "" {
					if subs, ok := s.projectSub[client.ProjectID]; ok {
						delete(subs, client.ID)
						if len(subs) == 0 {
							delete(s.projectSub, client.ProjectID)
						}
					}
				}
				close(client.Send)
			}
			s.mu.Unlock()
			log.Printf("[WS] Client disconnected: %s", client.ID)

		case message := <-s.broadcast:
			s.mu.RLock()
			for _, client := range s.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(s.clients, client.ID)
				}
			}
			s.mu.RUnlock()
		}
	}
}

func (s *WebSocketService) Register() chan *WSClient {
	return s.register
}

func (s *WebSocketService) Unregister() chan *WSClient {
	return s.unregister
}

func (s *WebSocketService) BroadcastToAll(msg WSMessage) {
	data, _ := json.Marshal(msg)
	s.broadcast <- data
}

func (s *WebSocketService) BroadcastToUser(uid string, msg WSMessage) {
	data, _ := json.Marshal(msg)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if client, ok := s.clients[uid]; ok {
		select {
		case client.Send <- data:
		default:
		}
	}
}

func (s *WebSocketService) BroadcastToProject(projectID string, msg WSMessage, excludeID string) {
	data, _ := json.Marshal(msg)
	s.mu.RLock()
	defer s.mu.RUnlock()
	subs, ok := s.projectSub[projectID]
	if !ok {
		return
	}
	for cid := range subs {
		if cid == excludeID {
			continue
		}
		if client, ok := s.clients[cid]; ok {
			select {
			case client.Send <- data:
			default:
			}
		}
	}
}

func (s *WebSocketService) SendBuildLog(projectID string, level, message string) {
	s.BroadcastToAll(WSMessage{
		Type: "build_log",
		Payload: map[string]interface{}{
			"project_id": projectID,
			"level":      level,
			"message":    message,
		},
	})
}

func (s *WebSocketService) SendAIStreamDelta(sessionID, delta string) {
	s.BroadcastToAll(WSMessage{
		Type: "ai_delta",
		Payload: map[string]interface{}{
			"session_id": sessionID,
			"delta":      delta,
		},
	})
}

// Collaboration broadcast helpers

func (s *WebSocketService) BroadcastCursorUpdate(projectID, userID, username, filePath string, line, col int, color string) {
	s.BroadcastToProject(projectID, WSMessage{
		Type: "collab_cursor_update",
		Payload: map[string]interface{}{
			"user_id":  userID,
			"username": username,
			"file":     filePath,
			"line":     line,
			"col":      col,
			"color":    color,
		},
	}, "")
}

func (s *WebSocketService) BroadcastSelectionUpdate(projectID, userID, username, filePath string, startLine, startCol, endLine, endCol int, color string) {
	s.BroadcastToProject(projectID, WSMessage{
		Type: "collab_selection_update",
		Payload: map[string]interface{}{
			"user_id":    userID,
			"username":   username,
			"file":       filePath,
			"start_line": startLine,
			"start_col":  startCol,
			"end_line":   endLine,
			"end_col":    endCol,
			"color":      color,
		},
	}, "")
}

func (s *WebSocketService) BroadcastEdit(projectID, userID, username, filePath, content string) {
	s.BroadcastToProject(projectID, WSMessage{
		Type: "collab_edit",
		Payload: map[string]interface{}{
			"user_id":  userID,
			"username": username,
			"file":     filePath,
			"content":  content,
		},
	}, "")
}

func (s *WebSocketService) BroadcastComment(projectID, userID, username, filePath, content string, lineNumber int) {
	s.BroadcastToProject(projectID, WSMessage{
		Type: "collab_comment",
		Payload: map[string]interface{}{
			"user_id":    userID,
			"username":   username,
			"file":       filePath,
			"content":    content,
			"line_number": lineNumber,
		},
	}, "")
}

func (s *WebSocketService) BroadcastJoin(projectID, userID, username, color string) {
	s.BroadcastToProject(projectID, WSMessage{
		Type: "collab_join",
		Payload: map[string]interface{}{
			"user_id":  userID,
			"username": username,
			"color":    color,
		},
	}, "")
}

func (s *WebSocketService) BroadcastLeave(projectID, userID, username string) {
	s.BroadcastToProject(projectID, WSMessage{
		Type: "collab_leave",
		Payload: map[string]interface{}{
			"user_id":  userID,
			"username": username,
		},
	}, "")
}

func (s *WebSocketService) ClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}
