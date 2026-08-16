package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Message struct {
	Role       string        `json:"role"`
	Content    string        `json:"content"`
	ToolCalls  []ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// MessageToMap converts a Message to a map suitable for LLM API
func (m Message) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"role":    m.Role,
		"content": m.Content,
	}
	if len(m.ToolCalls) > 0 {
		result["tool_calls"] = m.ToolCalls
	}
	if m.ToolCallID != "" {
		result["tool_call_id"] = m.ToolCallID
	}
	return result
}

type Conversation struct {
	Messages []Message
}

type ConversationStore struct {
	mu          sync.RWMutex
	sessions    map[string]*Conversation
	order       []string
	maxSessions int
	maxMessages int
}

func NewConversationStore() *ConversationStore {
	return &ConversationStore{
		sessions:    make(map[string]*Conversation),
		maxSessions: 50,
		maxMessages: 200,
	}
}

func (cs *ConversationStore) Add(sessionID string, messages []Message) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	conv, ok := cs.sessions[sessionID]
	if !ok {
		if len(cs.sessions) >= cs.maxSessions {
			oldest := cs.order[0]
			delete(cs.sessions, oldest)
			cs.order = cs.order[1:]
		}
		conv = &Conversation{}
		cs.sessions[sessionID] = conv
		cs.order = append(cs.order, sessionID)
	}

	if len(messages) > cs.maxMessages {
		messages = messages[len(messages)-cs.maxMessages:]
	}
	conv.Messages = messages
}

func (cs *ConversationStore) Get(sessionID string) []Message {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	conv, ok := cs.sessions[sessionID]
	if !ok {
		return nil
	}
	result := make([]Message, len(conv.Messages))
	copy(result, conv.Messages)
	return result
}

func (cs *ConversationStore) Delete(sessionID string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if _, ok := cs.sessions[sessionID]; !ok {
		return
	}
	delete(cs.sessions, sessionID)
	// Remove from order slice
	for i, id := range cs.order {
		if id == sessionID {
			cs.order = append(cs.order[:i], cs.order[i+1:]...)
			return
		}
	}
}

func (cs *ConversationStore) Append(sessionID string, msgs ...Message) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	conv, ok := cs.sessions[sessionID]
	if !ok {
		if len(cs.sessions) >= cs.maxSessions {
			oldest := cs.order[0]
			delete(cs.sessions, oldest)
			cs.order = cs.order[1:]
		}
		conv = &Conversation{}
		cs.sessions[sessionID] = conv
		cs.order = append(cs.order, sessionID)
	}

	conv.Messages = append(conv.Messages, msgs...)
	if len(conv.Messages) > cs.maxMessages {
		excess := len(conv.Messages) - cs.maxMessages
		conv.Messages = conv.Messages[excess:]
	}
}

func EstimateTokens(content string) int {
	var total float64
	for _, r := range content {
		if unicode.Is(unicode.Han, r) {
			total += 2
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			total += 1.3
		} else {
			total += 0.5
		}
	}
	return int(total)
}

func estimateMessageTokens(msg Message) int {
	return EstimateTokens(msg.Content)
}

const tokenThreshold = 6000

func (cs *ConversationStore) CompressMessages(systemPrompt string, messages []Message) []Message {
	var totalTokens int
	systemTokens := EstimateTokens(systemPrompt)
	totalTokens += systemTokens

	for _, m := range messages {
		totalTokens += estimateMessageTokens(m)
	}

	if totalTokens <= tokenThreshold {
		result := make([]Message, 0, len(messages)+1)
		result = append(result, Message{Role: "system", Content: systemPrompt})
		result = append(result, messages...)
		return result
	}

	keepCount := 6
	if keepCount > len(messages) {
		keepCount = len(messages)
	}

	keepMessages := make([]Message, keepCount)
	copy(keepMessages, messages[len(messages)-keepCount:])

	var keepTokens int
	for _, m := range keepMessages {
		keepTokens += estimateMessageTokens(m)
	}

	compressTokens := totalTokens - systemTokens - keepTokens
	if compressTokens <= 0 {
		result := make([]Message, 0, len(messages)+1)
		result = append(result, Message{Role: "system", Content: systemPrompt})
		result = append(result, messages...)
		return result
	}

	compressMessages := messages[:len(messages)-keepCount]
	summary := compressToSummary(compressMessages)

	result := make([]Message, 0, 3+len(keepMessages))
	result = append(result, Message{Role: "system", Content: systemPrompt})
	result = append(result, Message{Role: "system", Content: summary})
	result = append(result, keepMessages...)

	var compressedTokens int
	compressedTokens += EstimateTokens(systemPrompt)
	compressedTokens += EstimateTokens(summary)
	compressedTokens += keepTokens

	if compressedTokens > tokenThreshold && len(keepMessages) > 2 {
		extraCompress := keepMessages[:len(keepMessages)-2]
		keepMessages = keepMessages[len(keepMessages)-2:]

		var sb strings.Builder
		sb.WriteString(summary)
		for _, m := range extraCompress {
			writeMessageToSummary(&sb, m)
		}
		summary = sb.String()

		result = make([]Message, 0, 3+len(keepMessages))
		result = append(result, Message{Role: "system", Content: systemPrompt})
		result = append(result, Message{Role: "system", Content: summary})
		result = append(result, keepMessages...)
	}

	return result
}

func compressToSummary(messages []Message) string {
	var sb strings.Builder
	sb.WriteString("[Previous context summary: ")
	for i, m := range messages {
		if i > 0 {
			sb.WriteString("; ")
		}
		writeMessageToSummary(&sb, m)
	}
	sb.WriteString("]")
	return sb.String()
}

func writeMessageToSummary(sb *strings.Builder, m Message) {
	roleLabel := "用户"
	if m.Role == "assistant" {
		roleLabel = "AI"
	} else if m.Role == "system" {
		roleLabel = "系统"
	}

	content := m.Content
	if utf8.RuneCountInString(content) > 100 {
		runes := []rune(content)
		content = string(runes[:100]) + "..."
	}
	sb.WriteString(roleLabel)
	sb.WriteString(": ")
	sb.WriteString(content)
}

// ===== Agent Memory Store =====

type MemoryStore struct {
	db *sql.DB
}

func NewMemoryStore(db *sql.DB) *MemoryStore {
	s := &MemoryStore{db: db}
	s.ensureTable()
	return s
}

func (ms *MemoryStore) ensureTable() {
	ms.db.Exec(`CREATE TABLE IF NOT EXISTS agent_memory (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		memory_type TEXT NOT NULL,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	ms.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_memory_unique ON agent_memory(user_id, memory_type, key)`)
}

func (ms *MemoryStore) SaveMemory(userID, memoryType, key, value string) error {
	now := time.Now()
	_, err := ms.db.Exec(
		`INSERT INTO agent_memory (user_id, memory_type, key, value, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, memory_type, key) DO UPDATE SET value = ?, updated_at = ?`,
		userID, memoryType, key, value, now, now, value, now,
	)
	return err
}

func (ms *MemoryStore) GetMemory(userID, memoryType, key string) (string, error) {
	var value string
	err := ms.db.QueryRow(
		"SELECT value FROM agent_memory WHERE user_id = ? AND memory_type = ? AND key = ?",
		userID, memoryType, key,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

type MemoryEntry struct {
	ID         int64     `json:"id"`
	UserID     string    `json:"user_id"`
	MemoryType string    `json:"memory_type"`
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (ms *MemoryStore) ListMemory(userID, memoryType string) ([]MemoryEntry, error) {
	rows, err := ms.db.Query(
		"SELECT id, user_id, memory_type, key, value, created_at, updated_at FROM agent_memory WHERE user_id = ? AND memory_type = ? ORDER BY updated_at DESC",
		userID, memoryType,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.MemoryType, &e.Key, &e.Value, &e.CreatedAt, &e.UpdatedAt); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (ms *MemoryStore) ListAllMemory(userID string) ([]MemoryEntry, error) {
	rows, err := ms.db.Query(
		"SELECT id, user_id, memory_type, key, value, created_at, updated_at FROM agent_memory WHERE user_id = ? ORDER BY memory_type, updated_at DESC",
		userID,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.MemoryType, &e.Key, &e.Value, &e.CreatedAt, &e.UpdatedAt); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (ms *MemoryStore) DeleteMemory(userID, memoryType, key string) error {
	_, err := ms.db.Exec(
		"DELETE FROM agent_memory WHERE user_id = ? AND memory_type = ? AND key = ?",
		userID, memoryType, key,
	)
	return err
}

func (ms *MemoryStore) DeleteAllMemory(userID string) error {
	_, err := ms.db.Exec("DELETE FROM agent_memory WHERE user_id = ?", userID)
	return err
}

func (ms *MemoryStore) LoadUserPreferences(userID string) string {
	var sb strings.Builder

	// Load user preferences
	entries, err := ms.ListMemory(userID, "user_preference")
	if err == nil && len(entries) > 0 {
		sb.WriteString("\n[User Preferences from memory]\n")
		for _, e := range entries {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", e.Key, e.Value))
		}
	}

	// Load project context
	projEntries, err := ms.ListMemory(userID, "project_context")
	if err == nil && len(projEntries) > 0 {
		sb.WriteString("\n[Project Context from memory]\n")
		for _, e := range projEntries {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", e.Key, e.Value))
		}
	}

	return sb.String()
}

// ===== DB-Backed Conversation Persistence =====

type ConversationSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Mode      string `json:"mode"`
	Model     string `json:"model"`
	ProjectID string `json:"project_id"`
	MessageCount int `json:"message_count"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func SaveConversation(db *sql.DB, userID, id, title, mode string, messages []Message, model string, projectID string) (string, error) {
	if id == "" {
		id = uuid.New().String()
	}
	msgJSON, _ := json.Marshal(messages)
	if title == "" {
		title = mode
		if len(messages) > 0 {
			content := messages[0].Content
			runes := []rune(content)
			if len(runes) > 40 {
				title = string(runes[:40]) + "..."
			} else {
				title = content
			}
		}
	}
	_, err := db.Exec(
		`INSERT INTO ai_conversations (id, user_id, title, mode, messages, model, project_id, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(id) DO UPDATE SET title=?, mode=?, messages=?, model=?, project_id=?, updated_at=datetime('now')`,
		id, userID, title, mode, string(msgJSON), model, projectID,
		title, mode, string(msgJSON), model, projectID,
	)
	return id, err
}

func ListConversations(db *sql.DB, userID string) ([]ConversationSummary, error) {
	rows, err := db.Query(
		`SELECT id, title, mode, model, COALESCE(project_id, ''), json_array_length(messages), created_at, updated_at
		 FROM ai_conversations WHERE user_id=? ORDER BY updated_at DESC LIMIT 100`,
		userID,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []ConversationSummary
	for rows.Next() {
		var cs ConversationSummary
		if err := rows.Scan(&cs.ID, &cs.Title, &cs.Mode, &cs.Model, &cs.ProjectID, &cs.MessageCount, &cs.CreatedAt, &cs.UpdatedAt); err != nil {
			continue
		}
		result = append(result, cs)
	}
	return result, nil
}

type ConversationData struct {
	Messages  []Message `json:"messages"`
	Mode      string    `json:"mode"`
	ProjectID string    `json:"project_id"`
}

func LoadConversation(db *sql.DB, userID, id string) (*ConversationData, error) {
	var msgJSON, mode, projectID string
	err := db.QueryRow(
		`SELECT messages, COALESCE(mode, ''), COALESCE(project_id, '') FROM ai_conversations WHERE id=? AND user_id=?`, id, userID,
	).Scan(&msgJSON, &mode, &projectID)
	if err != nil {
		return nil, err
	}
	var messages []Message
	if err := json.Unmarshal([]byte(msgJSON), &messages); err != nil {
		return nil, err
	}
	return &ConversationData{Messages: messages, Mode: mode, ProjectID: projectID}, nil
}

func DeleteConversation(db *sql.DB, userID, id string) error {
	// Delete associated messages first
	db.Exec(`DELETE FROM conversation_messages WHERE session_id=? AND user_id=?`, id, userID)
	_, err := db.Exec(`DELETE FROM ai_conversations WHERE id=? AND user_id=?`, id, userID)
	return err
}

// ===== Individual Message Persistence =====

type ConversationMessage struct {
	ID         int64    `json:"id"`
	SessionID  string   `json:"session_id"`
	UserID     string   `json:"user_id"`
	Role       string   `json:"role"`
	Content    string   `json:"content"`
	StepType   string   `json:"step_type,omitempty"` // think, skill_call, skill_result, answer, ""
	RoundIndex int      `json:"round_index"`          // which Q&A round this message/step belongs to
	CreatedAt  string   `json:"created_at"`
	ToolCalls  string   `json:"tool_calls,omitempty"`
	ToolCallID string   `json:"tool_call_id,omitempty"`
	TokenUsage string   `json:"token_usage,omitempty"` // JSON: {"prompt_tokens":N,"completion_tokens":N,"total_tokens":N}
}

func EnsureConversationMessagesTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS conversation_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		step_type TEXT DEFAULT '',
		round_index INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return err
	}
	// Migration: add columns if missing
	db.Exec(`ALTER TABLE conversation_messages ADD COLUMN step_type TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE conversation_messages ADD COLUMN round_index INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE conversation_messages ADD COLUMN token_usage TEXT DEFAULT ''`)
	// Composite index for fast session+user+time queries
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_conv_msg_session ON conversation_messages(session_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_conv_msg_user ON conversation_messages(user_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_conv_msg_session_user_time ON conversation_messages(session_id, user_id, created_at)`)
	return nil
}

func SaveConversationMessage(db *sql.DB, sessionID, userID, role, content string, roundIndex int, extraFields ...map[string]string) error {
	// Cap persisted content (assistant answers / user tasks) to avoid DB bloat.
	const maxMsgContent = 256 * 1024
	if len(content) > maxMsgContent {
		content = content[:maxMsgContent] + "\n...[truncated by server]"
	}
	toolCalls := ""
	toolCallID := ""
	tokenUsage := ""
	if len(extraFields) > 0 {
		if v, ok := extraFields[0]["tool_calls"]; ok {
			toolCalls = v
		}
		if v, ok := extraFields[0]["tool_call_id"]; ok {
			toolCallID = v
		}
		if v, ok := extraFields[0]["token_usage"]; ok {
			tokenUsage = v
		}
	}
	_, err := db.Exec(
		`INSERT INTO conversation_messages (session_id, user_id, role, content, round_index, tool_calls, tool_call_id, token_usage) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, userID, role, content, roundIndex, toolCalls, toolCallID, tokenUsage,
	)
	return err
}

// SaveAgentStep saves an agent intermediate step (think/skill_call/skill_result/answer).
// Content is truncated to prevent DB bloat from oversized tool outputs.
func SaveAgentStep(db *sql.DB, sessionID, userID, stepType, content string, roundIndex int) error {
	const maxStepContent = 64 * 1024 // 64KB cap per intermediate step
	if len(content) > maxStepContent {
		content = content[:maxStepContent] + "\n...[truncated by server]"
	}
	_, err := db.Exec(
		`INSERT INTO conversation_messages (session_id, user_id, role, content, step_type, round_index) VALUES (?, ?, 'agent', ?, ?, ?)`,
		sessionID, userID, content, stepType, roundIndex,
	)
	return err
}

func GetConversationMessages(db *sql.DB, sessionID, userID string) ([]ConversationMessage, string, error) {
	rows, err := db.Query(
		`SELECT id, session_id, user_id, role, content, COALESCE(step_type, ''), COALESCE(round_index, 0), created_at, COALESCE(tool_calls, ''), COALESCE(tool_call_id, ''), COALESCE(token_usage, '')
		 FROM conversation_messages WHERE session_id=? AND user_id=?
		 ORDER BY created_at ASC`,
		sessionID, userID,
	)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var result []ConversationMessage
	const maxReadContent = 96 * 1024 // serve-side cap for legacy oversized rows
	for rows.Next() {
		var m ConversationMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.UserID, &m.Role, &m.Content, &m.StepType, &m.RoundIndex, &m.CreatedAt, &m.ToolCalls, &m.ToolCallID, &m.TokenUsage); err != nil {
			continue
		}
		if len(m.Content) > maxReadContent {
			m.Content = m.Content[:maxReadContent] + "\n...[truncated by server]"
		}
		result = append(result, m)
	}

	// 从 ai_conversations 表获取 mode
	var mode string
	db.QueryRow(`SELECT COALESCE(mode, '') FROM ai_conversations WHERE id=? AND user_id=?`, sessionID, userID).Scan(&mode)

	// If no messages in conversation_messages, fall back to ai_conversations.messages JSON
	// This handles non-agent modes (chat/generate) that store messages as JSON blob
	if len(result) == 0 {
		var msgJSON string
		err := db.QueryRow(
			`SELECT COALESCE(messages, '[]') FROM ai_conversations WHERE id=? AND user_id=?`,
			sessionID, userID,
		).Scan(&msgJSON)
		if err == nil && msgJSON != "[]" && msgJSON != "" {
			var msgs []Message
			if json.Unmarshal([]byte(msgJSON), &msgs) == nil && len(msgs) > 0 {
				for i, m := range msgs {
					ri := i / 2 // pair user+assistant as same round
					result = append(result, ConversationMessage{
						SessionID:  sessionID,
						UserID:     userID,
						Role:       m.Role,
						Content:    m.Content,
						StepType:   "",
						RoundIndex: ri,
					})
				}
			}
		}
	}

	return result, mode, nil
}

// ListUserSessions returns the user's AI/agent conversations, newest first,
// with pagination (limit/offset). Also returns the total number of sessions.
func ListUserSessions(db *sql.DB, userID string, limit, offset int) ([]map[string]interface{}, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	// Total count (both agent-mode messages and non-agent conversations)
	var total int64
	err := db.QueryRow(
		`SELECT COUNT(DISTINCT sid) FROM (
			SELECT cm.session_id AS sid FROM conversation_messages cm WHERE cm.user_id=?
			UNION
			SELECT ac.id AS sid FROM ai_conversations ac WHERE ac.user_id=?
		)`, userID, userID,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Union both sources: conversation_messages (agent mode) and ai_conversations (chat/generate modes)
	rows, err := db.Query(
		`SELECT session_id, started_at, last_at, msg_count, title, mode, model, token_usage FROM (
			SELECT cm.session_id,
			       MIN(cm.created_at) as started_at,
			       MAX(cm.created_at) as last_at,
			       COUNT(*) as msg_count,
			       COALESCE(ac.title, '') as title,
			       COALESCE(ac.mode, '') as mode,
			       COALESCE(ac.model, '') as model,
			       0 as token_usage
			FROM conversation_messages cm
			LEFT JOIN ai_conversations ac ON cm.session_id = ac.id AND cm.user_id = ac.user_id
			WHERE cm.user_id=?
			GROUP BY cm.session_id
			UNION
			SELECT ac.id as session_id,
			       ac.created_at as started_at,
			       ac.updated_at as last_at,
			       json_array_length(ac.messages) as msg_count,
			       COALESCE(ac.title, '') as title,
			       COALESCE(ac.mode, '') as mode,
			       COALESCE(ac.model, '') as model,
			       COALESCE(ac.token_usage, 0) as token_usage
			FROM ai_conversations ac
			WHERE ac.user_id=?
			  AND NOT EXISTS (
				SELECT 1 FROM conversation_messages cm2
				WHERE cm2.session_id = ac.id AND cm2.user_id = ac.user_id
			  )
		) combined
		ORDER BY last_at DESC LIMIT ? OFFSET ?`,
		userID, userID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var sessionID, startedAt, lastAt, title, mode, model string
		var msgCount int
		var tokenUsage int64
		if err := rows.Scan(&sessionID, &startedAt, &lastAt, &msgCount, &title, &mode, &model, &tokenUsage); err != nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"session_id":  sessionID,
			"started_at":  startedAt,
			"last_at":     lastAt,
			"msg_count":   msgCount,
			"title":       title,
			"mode":        mode,
			"model":       model,
			"token_usage": tokenUsage,
		})
	}
	// Aggregate per-session token usage (agent mode messages persist token_usage).
	if len(result) > 0 {
		trows, err := db.Query(`SELECT session_id, SUM(CAST(json_extract(token_usage, '$.total_tokens') AS INTEGER))
			FROM conversation_messages
			WHERE user_id=? AND token_usage IS NOT NULL AND token_usage != ''
			GROUP BY session_id`, userID)
		if err == nil {
			for trows.Next() {
				var sid string
				var tokens int64
				if err := trows.Scan(&sid, &tokens); err == nil {
					for _, s := range result {
						if s["session_id"] == sid {
							s["token_usage"] = tokens
							break
						}
					}
				}
			}
			trows.Close()
		}
	}
	for _, s := range result {
		if _, ok := s["token_usage"]; !ok {
			s["token_usage"] = int64(0)
		}
	}
	return result, total, nil
}

func DeleteSessionMessages(db *sql.DB, sessionID, userID string) error {
	// Delete from both tables: conversation_messages (agent mode) and ai_conversations (all modes)
	db.Exec(`DELETE FROM conversation_messages WHERE session_id=? AND user_id=?`, sessionID, userID)
	_, err := db.Exec(`DELETE FROM ai_conversations WHERE id=? AND user_id=?`, sessionID, userID)
	return err
}

// SearchSessionMessages searches across all sessions for matching content
func SearchSessionMessages(db *sql.DB, userID, query string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 50
	}
	// Search both conversation_messages (agent mode) and ai_conversations (chat/generate modes)
	rows, err := db.Query(
		`SELECT session_id, role, content, step_type, created_at FROM (
			SELECT cm.session_id, cm.role, cm.content, COALESCE(cm.step_type, '') as step_type, cm.created_at
			FROM conversation_messages cm
			WHERE cm.user_id=? AND cm.content LIKE ?
			UNION ALL
			SELECT ac.id as session_id, 'user' as role, m.value as content, '' as step_type, ac.updated_at as created_at
			FROM ai_conversations ac,
			     json_each(ac.messages) m
			WHERE ac.user_id=?
			  AND json_type(m.value) = 'object'
			  AND json_extract(m.value, '$.content') LIKE ?
		) combined
		ORDER BY created_at DESC LIMIT ?`,
		userID, "%"+query+"%", userID, "%"+query+"%", limit,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var sessionID, role, content, stepType, createdAt string
		if err := rows.Scan(&sessionID, &role, &content, &stepType, &createdAt); err != nil {
			continue
		}
		// Truncate content for search results
		runes := []rune(content)
		if len(runes) > 120 {
			content = string(runes[:120]) + "..."
		}
		result = append(result, map[string]interface{}{
			"session_id":  sessionID,
			"role":        role,
			"content":     content,
			"step_type":   stepType,
			"created_at":  createdAt,
		})
	}
	return result, nil
}

// ExportSessionAsMarkdown exports a session's messages as Markdown
func ExportSessionAsMarkdown(db *sql.DB, sessionID, userID string) (string, error) {
	messages, _, err := GetConversationMessages(db, sessionID, userID)
	if err != nil {
		return "", err
	}
	if len(messages) == 0 {
		return "", fmt.Errorf("session not found or empty")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# 会话 %s\n\n", sessionID))
	for _, m := range messages {
		if m.StepType != "" {
			// Agent step — show as collapsed details
			sb.WriteString(fmt.Sprintf("\n<details><summary>🔧 %s</summary>\n\n%s\n\n</details>\n", m.StepType, m.Content))
		} else if m.Role == "user" {
			sb.WriteString(fmt.Sprintf("## 👤 用户\n\n%s\n\n", m.Content))
		} else {
			sb.WriteString(fmt.Sprintf("## 🤖 助手\n\n%s\n\n---\n\n", m.Content))
		}
	}
	return sb.String(), nil
}

// ExportSessionAsJSON exports a session as JSON
func ExportSessionAsJSON(db *sql.DB, sessionID, userID string) ([]byte, error) {
	messages, _, err := GetConversationMessages(db, sessionID, userID)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(map[string]interface{}{
		"session_id": sessionID,
		"messages":   messages,
	}, "", "  ")
}
