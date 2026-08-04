package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionState holds persistent state for an agent session.
type SessionState struct {
	mu sync.RWMutex

	SessionID    string                 `json:"session_id"`
	ProjectID    string                 `json:"project_id"`
	UserID       string                 `json:"user_id"`
	Mode         string                 `json:"mode"`
	ToolsEnabled map[string]bool        `json:"tools_enabled"`
	Preferences  map[string]interface{} `json:"preferences"`
	Checkpoints  []FileCheckpoint       `json:"checkpoints"`
	ToolHistory  map[string]int         `json:"tool_history"`
	LastError    string                 `json:"last_error,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// SessionPersistence manages session state persistence to disk.
type SessionPersistence struct {
	mu       sync.Mutex
	dataDir  string
	sessions map[string]*SessionState
}

// NewSessionPersistence creates a new session persistence manager.
func NewSessionPersistence(dataDir string) *SessionPersistence {
	if dataDir == "" {
		dataDir = filepath.Join(".", "data", "sessions")
	}
	os.MkdirAll(dataDir, 0755)

	sp := &SessionPersistence{
		dataDir:  dataDir,
		sessions: make(map[string]*SessionState),
	}

	// Load existing sessions from disk
	sp.loadAll()

	return sp
}

// GetOrCreate returns the session state, creating if necessary.
func (sp *SessionPersistence) GetOrCreate(sessionID string) *SessionState {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if s, ok := sp.sessions[sessionID]; ok {
		return s
	}

	s := &SessionState{
		SessionID:    sessionID,
		ToolsEnabled: make(map[string]bool),
		Preferences:  make(map[string]interface{}),
		Checkpoints:  make([]FileCheckpoint, 0),
		ToolHistory:  make(map[string]int),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	sp.sessions[sessionID] = s
	return s
}

// Save persists the session state to disk.
func (sp *SessionPersistence) Save(sessionID string) error {
	sp.mu.Lock()
	s, ok := sp.sessions[sessionID]
	sp.mu.Unlock()

	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	s.mu.Lock()
	s.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(s, "", "  ")
	s.mu.Unlock()

	if err != nil {
		return err
	}

	filePath := filepath.Join(sp.dataDir, fmt.Sprintf("%s.json", sessionID))
	return os.WriteFile(filePath, data, 0644)
}

// UpdateCheckpoint adds or updates a checkpoint.
func (sp *SessionPersistence) UpdateCheckpoint(sessionID, path, content string) {
	sp.mu.Lock()
	s, ok := sp.sessions[sessionID]
	sp.mu.Unlock()

	if !ok {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Find existing checkpoint for this path
	for i, cp := range s.Checkpoints {
		if cp.Path == path {
			s.Checkpoints[i].Content = content
			s.Checkpoints[i].Time = time.Now()
			return
		}
	}

	// Add new checkpoint
	s.Checkpoints = append(s.Checkpoints, FileCheckpoint{
		Path:    path,
		Content: content,
		Time:    time.Now(),
	})

	// Keep max 20 checkpoints
	if len(s.Checkpoints) > 20 {
		s.Checkpoints = s.Checkpoints[len(s.Checkpoints)-20:]
	}
}

// loadAll loads all sessions from disk.
func (sp *SessionPersistence) loadAll() {
	entries, err := os.ReadDir(sp.dataDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(sp.dataDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("[SessionPersistence] failed to read %s: %v", filePath, err)
			continue
		}

		var s SessionState
		if err := json.Unmarshal(data, &s); err != nil {
			log.Printf("[SessionPersistence] failed to parse %s: %v", filePath, err)
			continue
		}

		if s.ToolsEnabled == nil {
			s.ToolsEnabled = make(map[string]bool)
		}
		if s.Preferences == nil {
			s.Preferences = make(map[string]interface{})
		}
		if s.ToolHistory == nil {
			s.ToolHistory = make(map[string]int)
		}

		sp.sessions[s.SessionID] = &s
	}
	log.Printf("[SessionPersistence] loaded %d sessions", len(sp.sessions))
}
