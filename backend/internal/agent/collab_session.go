package agent

import (
	"encoding/json"
	"sync"
	"time"
)

// CollabEvent represents an event in a collaborative session.
type CollabEvent struct {
	Type      string      `json:"type"`
	SessionID string      `json:"session_id"`
	UserID    string      `json:"user_id"`
	Username  string      `json:"username"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// CursorPosition represents a user's cursor position.
type CursorPosition struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}

// EditAction represents a code edit action.
type EditAction struct {
	FilePath  string `json:"file_path"`
	Operation string `json:"operation"`
	Position  int    `json:"position"`
	Content   string `json:"content"`
	Length    int    `json:"length"`
}

// CollabSession represents a collaborative editing session.
type CollabSession struct {
	mu           sync.RWMutex
	ID           string
	ProjectID    string
	CreatedAt    time.Time
	Participants map[string]*CollabParticipant
	EventHistory []CollabEvent
	MaxHistory   int
}

// CollabParticipant represents a user in the session.
type CollabParticipant struct {
	UserID     string
	Username   string
	Cursor     CursorPosition
	JoinedAt   time.Time
	LastActive time.Time
	Color      string
}

// CollabSessionManager manages all active collaborative sessions.
type CollabSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*CollabSession
	colors   []string
	colorIdx int
}

// NewCollabSessionManager creates a new session manager.
func NewCollabSessionManager() *CollabSessionManager {
	return &CollabSessionManager{
		sessions: make(map[string]*CollabSession),
		colors: []string{
			"#FF6B6B", "#4ECDC4", "#45B7D1", "#96CEB4",
			"#FFEAA7", "#DDA0DD", "#98D8C8", "#F7DC6F",
			"#BB8FCE", "#85C1E9", "#F0B27A", "#82E0AA",
		},
	}
}

// CreateSession creates a new collaborative session.
func (csm *CollabSessionManager) CreateSession(sessionID, projectID string) *CollabSession {
	csm.mu.Lock()
	defer csm.mu.Unlock()

	session := &CollabSession{
		ID:           sessionID,
		ProjectID:    projectID,
		CreatedAt:    time.Now(),
		Participants: make(map[string]*CollabParticipant),
		EventHistory: make([]CollabEvent, 0, 100),
		MaxHistory:   1000,
	}

	csm.sessions[sessionID] = session
	return session
}

// GetSession returns a session by ID.
func (csm *CollabSessionManager) GetSession(sessionID string) (*CollabSession, bool) {
	csm.mu.RLock()
	defer csm.mu.RUnlock()
	s, ok := csm.sessions[sessionID]
	return s, ok
}

// RemoveSession removes a session.
func (csm *CollabSessionManager) RemoveSession(sessionID string) {
	csm.mu.Lock()
	defer csm.mu.Unlock()
	delete(csm.sessions, sessionID)
}

// GetActiveSessions returns the count of active sessions.
func (csm *CollabSessionManager) GetActiveSessions() int {
	csm.mu.RLock()
	defer csm.mu.RUnlock()
	return len(csm.sessions)
}

// GetNextColor returns the next unique color for a participant.
func (csm *CollabSessionManager) GetNextColor() string {
	csm.mu.Lock()
	defer csm.mu.Unlock()
	color := csm.colors[csm.colorIdx%len(csm.colors)]
	csm.colorIdx++
	return color
}

// Join adds a participant to the session.
func (cs *CollabSession) Join(userID, username string) *CollabParticipant {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	color := "#888888"
	if existing, ok := cs.Participants[userID]; ok {
		color = existing.Color
	} else {
		colorIdx := len(cs.Participants) % 12
		colors := []string{
			"#FF6B6B", "#4ECDC4", "#45B7D1", "#96CEB4",
			"#FFEAA7", "#DDA0DD", "#98D8C8", "#F7DC6F",
			"#BB8FCE", "#85C1E9", "#F0B27A", "#82E0AA",
		}
		color = colors[colorIdx]
	}

	participant := &CollabParticipant{
		UserID:     userID,
		Username:   username,
		JoinedAt:   time.Now(),
		LastActive: time.Now(),
		Color:      color,
	}

	cs.Participants[userID] = participant

	cs.addEvent(CollabEvent{
		Type:      "join",
		SessionID: cs.ID,
		UserID:    userID,
		Username:  username,
		Timestamp: time.Now(),
		Data:      map[string]string{"color": color},
	})

	return participant
}

// Leave removes a participant from the session.
func (cs *CollabSession) Leave(userID string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if p, ok := cs.Participants[userID]; ok {
		cs.addEvent(CollabEvent{
			Type:      "leave",
			SessionID: cs.ID,
			UserID:    userID,
			Username:  p.Username,
			Timestamp: time.Now(),
		})
		delete(cs.Participants, userID)
	}
}

// UpdateCursor updates a participant's cursor position.
func (cs *CollabSession) UpdateCursor(userID string, pos CursorPosition) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if p, ok := cs.Participants[userID]; ok {
		p.Cursor = pos
		p.LastActive = time.Now()

		cs.addEvent(CollabEvent{
			Type:      "cursor",
			SessionID: cs.ID,
			UserID:    userID,
			Username:  p.Username,
			Timestamp: time.Now(),
			Data:      pos,
		})
	}
}

// RecordEdit records an edit action.
func (cs *CollabSession) RecordEdit(userID string, action EditAction) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if p, ok := cs.Participants[userID]; ok {
		p.LastActive = time.Now()

		cs.addEvent(CollabEvent{
			Type:      "edit",
			SessionID: cs.ID,
			UserID:    userID,
			Username:  p.Username,
			Timestamp: time.Now(),
			Data:      action,
		})
	}
}

// BroadcastMessage broadcasts a chat message.
func (cs *CollabSession) BroadcastMessage(userID, username, message string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.addEvent(CollabEvent{
		Type:      "message",
		SessionID: cs.ID,
		UserID:    userID,
		Username:  username,
		Timestamp: time.Now(),
		Data:      map[string]string{"message": message},
	})
}

// GetParticipants returns all current participants.
func (cs *CollabSession) GetParticipants() []*CollabParticipant {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	result := make([]*CollabParticipant, 0, len(cs.Participants))
	for _, p := range cs.Participants {
		result = append(result, p)
	}
	return result
}

// GetRecentEvents returns the last N events.
func (cs *CollabSession) GetRecentEvents(n int) []CollabEvent {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if n > len(cs.EventHistory) {
		n = len(cs.EventHistory)
	}
	result := make([]CollabEvent, n)
	copy(result, cs.EventHistory[len(cs.EventHistory)-n:])
	return result
}

// GetParticipantCount returns the number of participants.
func (cs *CollabSession) GetParticipantCount() int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return len(cs.Participants)
}

// addEvent adds an event to history (must hold lock).
func (cs *CollabSession) addEvent(event CollabEvent) {
	cs.EventHistory = append(cs.EventHistory, event)
	if len(cs.EventHistory) > cs.MaxHistory {
		cs.EventHistory = cs.EventHistory[len(cs.EventHistory)-cs.MaxHistory:]
	}
}

// ToJSON serializes the session state to JSON.
func (cs *CollabSession) ToJSON() ([]byte, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return json.Marshal(map[string]interface{}{
		"id":                cs.ID,
		"project_id":        cs.ProjectID,
		"created_at":        cs.CreatedAt,
		"participants":      cs.Participants,
		"participant_count": len(cs.Participants),
	})
}
