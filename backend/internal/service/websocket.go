package service

import (
	"log/slog"
	"sync"

	"github.com/fasthttp/websocket"
)

type WSEvent struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

type WSClient struct {
	UserID string
	Conn   *websocket.Conn
	mu     sync.Mutex
}

func (c *WSClient) SendJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteJSON(v)
}

type Hub struct {
	mu        sync.RWMutex
	clients   map[string]map[*WSClient]bool
	projectMu sync.RWMutex
	projects  map[string]map[*WSClient]bool
}

var globalHub *Hub
var hubOnce sync.Once

func GetHub() *Hub {
	hubOnce.Do(func() {
		globalHub = &Hub{
			clients:  make(map[string]map[*WSClient]bool),
			projects: make(map[string]map[*WSClient]bool),
		}
	})
	return globalHub
}

func (h *Hub) Subscribe(userID string, conn *websocket.Conn) *WSClient {
	client := &WSClient{UserID: userID, Conn: conn}
	h.mu.Lock()
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[*WSClient]bool)
	}
	h.clients[userID][client] = true
	h.mu.Unlock()
	slog.Debug("ws subscribe", "user_id", userID)
	return client
}

func (h *Hub) Unsubscribe(userID string, client *WSClient) {
	h.mu.Lock()
	if clients, ok := h.clients[userID]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.clients, userID)
		}
	}
	h.mu.Unlock()

	h.projectMu.Lock()
	for pid, members := range h.projects {
		delete(members, client)
		if len(members) == 0 {
			delete(h.projects, pid)
		}
	}
	h.projectMu.Unlock()
}

func (h *Hub) NotifyUser(userID string, event string, data interface{}) {
	evt := WSEvent{Event: event, Data: data}
	h.mu.RLock()
	clientSet := h.clients[userID]
	clients := make([]*WSClient, 0, len(clientSet))
	for c := range clientSet {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		if err := client.SendJSON(evt); err != nil {
			slog.Warn("ws send error", "user_id", userID, "error", err)
			h.Unsubscribe(userID, client)
		}
	}
}

func (h *Hub) NotifyProject(projectID string, event string, data interface{}) {
	evt := WSEvent{Event: event, Data: data}
	h.projectMu.RLock()
	memberSet := h.projects[projectID]
	members := make([]*WSClient, 0, len(memberSet))
	for c := range memberSet {
		members = append(members, c)
	}
	h.projectMu.RUnlock()
	for _, client := range members {
		if err := client.SendJSON(evt); err != nil {
			slog.Warn("ws project send error", "project_id", projectID, "error", err)
			if client.UserID != "" {
				h.Unsubscribe(client.UserID, client)
			}
		}
	}
}

func (h *Hub) SubscribeProject(projectID string, client *WSClient) {
	h.projectMu.Lock()
	if h.projects[projectID] == nil {
		h.projects[projectID] = make(map[*WSClient]bool)
	}
	h.projects[projectID][client] = true
	h.projectMu.Unlock()
}

func (h *Hub) NotifyUserJSON(userID string, raw []byte) {
	h.mu.RLock()
	clientSet := h.clients[userID]
	clients := make([]*WSClient, 0, len(clientSet))
	for c := range clientSet {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		client.mu.Lock()
		err := client.Conn.WriteMessage(websocket.TextMessage, raw)
		client.mu.Unlock()
		if err != nil {
			slog.Warn("ws raw send error", "user_id", userID, "error", err)
			h.Unsubscribe(userID, client)
		}
	}
}

func (h *Hub) NotifyProjectJSON(projectID string, raw []byte) {
	h.projectMu.RLock()
	memberSet := h.projects[projectID]
	members := make([]*WSClient, 0, len(memberSet))
	for c := range memberSet {
		members = append(members, c)
	}
	h.projectMu.RUnlock()
	for _, client := range members {
		client.mu.Lock()
		err := client.Conn.WriteMessage(websocket.TextMessage, raw)
		client.mu.Unlock()
		if err != nil {
			slog.Warn("ws project raw send error", "project_id", projectID, "error", err)
			if client.UserID != "" {
				h.Unsubscribe(client.UserID, client)
			}
		}
	}
}

func NotifyUser(userID string, event string, data interface{}) {
	GetHub().NotifyUser(userID, event, data)
}

func NotifyProject(projectID string, event string, data interface{}) {
	GetHub().NotifyProject(projectID, event, data)
}
