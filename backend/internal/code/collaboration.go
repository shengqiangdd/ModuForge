package code

import (
	"sync"
	"time"
)

// CollaborationSession 协作会话
type CollaborationSession struct {
	ID          string                    `json:"id"`
	FileName    string                    `json:"file_name"`
	CreatedAt   time.Time                 `json:"created_at"`
	Participants []Collaborator           `json:"participants"`
	Cursors     map[string]CursorPosition `json:"cursors"`
	Changes     []CollaborationChange     `json:"changes"`
	Lock        sync.RWMutex             `json:"-"`
}

// Collaborator 协作者
type Collaborator struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	JoinedAt  time.Time `json:"joined_at"`
	IsActive  bool      `json:"is_active"`
	CursorPos int       `json:"cursor_pos"`
}

// CursorPosition 光标位置
type CursorPosition struct {
	UserID   string `json:"user_id"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	LastUpdate time.Time `json:"last_update"`
}

// CollaborationChange 协作变更
type CollaborationChange struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Type      string    `json:"type"` // insert, delete, replace
	Position  int       `json:"position"`
	Content   string    `json:"content"`
	Length    int       `json:"length"`
	Timestamp time.Time `json:"timestamp"`
}

// CollaborationManager 协作管理器
type CollaborationManager struct {
	sessions map[string]*CollaborationSession
	lock     sync.RWMutex
}

// NewCollaborationManager 创建协作管理器
func NewCollaborationManager() *CollaborationManager {
	return &CollaborationManager{
		sessions: make(map[string]*CollaborationSession),
	}
}

// CreateSession 创建协作会话
func (m *CollaborationManager) CreateSession(fileName string) *CollaborationSession {
	m.lock.Lock()
	defer m.lock.Unlock()

	session := &CollaborationSession{
		ID:           generateSessionID(),
		FileName:     fileName,
		CreatedAt:    time.Now(),
		Participants: make([]Collaborator, 0),
		Cursors:      make(map[string]CursorPosition),
		Changes:      make([]CollaborationChange, 0),
	}

	m.sessions[session.ID] = session
	return session
}

// GetSession 获取协作会话
func (m *CollaborationManager) GetSession(sessionID string) *CollaborationSession {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.sessions[sessionID]
}

// JoinSession 加入协作会话
func (m *CollaborationManager) JoinSession(sessionID, userID, username string) bool {
	m.lock.RLock()
	session, exists := m.sessions[sessionID]
	m.lock.RUnlock()

	if !exists {
		return false
	}

	session.Lock.Lock()
	defer session.Lock.Unlock()

	// 检查是否已加入
	for _, p := range session.Participants {
		if p.UserID == userID {
			return true
		}
	}

	session.Participants = append(session.Participants, Collaborator{
		UserID:   userID,
		Username: username,
		JoinedAt: time.Now(),
		IsActive: true,
	})

	return true
}

// LeaveSession 离开协作会话
func (m *CollaborationManager) LeaveSession(sessionID, userID string) {
	m.lock.RLock()
	session, exists := m.sessions[sessionID]
	m.lock.RUnlock()

	if !exists {
		return
	}

	session.Lock.Lock()
	defer session.Lock.Unlock()

	for i, p := range session.Participants {
		if p.UserID == userID {
			session.Participants[i].IsActive = false
			break
		}
	}

	// 删除光标
	delete(session.Cursors, userID)
}

// UpdateCursor 更新光标位置
func (m *CollaborationManager) UpdateCursor(sessionID, userID string, line, column int) {
	m.lock.RLock()
	session, exists := m.sessions[sessionID]
	m.lock.RUnlock()

	if !exists {
		return
	}

	session.Lock.Lock()
	defer session.Lock.Unlock()

	session.Cursors[userID] = CursorPosition{
		UserID:     userID,
		Line:       line,
		Column:     column,
		LastUpdate: time.Now(),
	}
}

// ApplyChange 应用变更
func (m *CollaborationManager) ApplyChange(sessionID string, change CollaborationChange) bool {
	m.lock.RLock()
	session, exists := m.sessions[sessionID]
	m.lock.RUnlock()

	if !exists {
		return false
	}

	session.Lock.Lock()
	defer session.Lock.Unlock()

	change.Timestamp = time.Now()
	session.Changes = append(session.Changes, change)

	return true
}

// GetActiveSessions 获取活跃会话列表
func (m *CollaborationManager) GetActiveSessions() []*CollaborationSession {
	m.lock.RLock()
	defer m.lock.RUnlock()

	sessions := make([]*CollaborationSession, 0)
	for _, s := range m.sessions {
		activeCount := 0
		for _, p := range s.Participants {
			if p.IsActive {
				activeCount++
			}
		}
		if activeCount > 0 {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

// generateSessionID 生成会话ID
func generateSessionID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(1)
	}
	return string(b)
}
