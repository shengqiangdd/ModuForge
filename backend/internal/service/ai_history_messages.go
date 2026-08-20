package service

import (
	"database/sql"
	"encoding/json"
)

type ConversationMessage struct {
	ID         int64  `json:"id"`
	SessionID  string `json:"session_id"`
	UserID     string `json:"user_id"`
	Role       string `json:"role"`
	Content    string `json:"content"`
	StepType   string `json:"step_type,omitempty"` // think, skill_call, skill_result, answer, ""
	RoundIndex int    `json:"round_index"`         // which Q&A round this message/step belongs to
	CreatedAt  string `json:"created_at"`
	ToolCalls  string `json:"tool_calls,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	TokenUsage string `json:"token_usage,omitempty"` // JSON: {"prompt_tokens":N,"completion_tokens":N,"total_tokens":N}
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
	msgs, _, mode, err := getConversationMessagesPage(db, sessionID, userID, 0, "", "")
	return msgs, mode, err
}

// GetConversationMessagesPage returns up to `limit` messages ending at or
// before the given cursor (created_at, id), oldest first. `limit` must be > 0.
// The second return value reports whether older messages exist (has_more).
// The composite cursor avoids dropping messages that share the same second
// (SQLite created_at has second precision).
func GetConversationMessagesPage(db *sql.DB, sessionID, userID string, limit int, before, beforeID string) ([]ConversationMessage, bool, string, error) {
	if limit <= 0 {
		limit = 50
	}
	return getConversationMessagesPage(db, sessionID, userID, limit, before, beforeID)
}

// getConversationMessagesPage is the shared implementation. limit<=0 means all.
func getConversationMessagesPage(db *sql.DB, sessionID, userID string, limit int, before, beforeID string) ([]ConversationMessage, bool, string, error) {
	query := `SELECT id, session_id, user_id, role, content, COALESCE(step_type, ''), COALESCE(round_index, 0), created_at, COALESCE(tool_calls, ''), COALESCE(tool_call_id, ''), COALESCE(token_usage, '')
		 FROM conversation_messages WHERE session_id=? AND user_id=?`
	args := []interface{}{sessionID, userID}
	if beforeID != "" {
		// id is an auto-increment PK ordered by insertion time, so it is the
		// reliable pagination cursor (avoids SQLite created_at format/textual
		// comparison pitfalls and second-precision duplicates).
		query += ` AND id < ?`
		args = append(args, beforeID)
	} else if before != "" {
		query += ` AND created_at < ?`
		args = append(args, before)
	}

	// Fetch newest-first, limit+1 rows to detect has_more, then reverse to
	// chronological order for rendering. id breaks ties for rows sharing the
	// same created_at second, keeping pagination deterministic.
	if limit > 0 {
		query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
		args = append(args, limit+1)
	} else {
		query += ` ORDER BY created_at ASC, id ASC`
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, false, "", err
	}
	defer rows.Close()

	var newest []ConversationMessage
	const maxReadContent = 96 * 1024 // serve-side cap for legacy oversized rows
	for rows.Next() {
		var m ConversationMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.UserID, &m.Role, &m.Content, &m.StepType, &m.RoundIndex, &m.CreatedAt, &m.ToolCalls, &m.ToolCallID, &m.TokenUsage); err != nil {
			continue
		}
		if len(m.Content) > maxReadContent {
			m.Content = m.Content[:maxReadContent] + "\n...[truncated by server]"
		}
		newest = append(newest, m)
	}

	hasMore := false
	if limit > 0 && len(newest) > limit {
		hasMore = true
		newest = newest[:limit]
	}
	// reverse newest-first -> chronological
	result := make([]ConversationMessage, 0, len(newest))
	for i := len(newest) - 1; i >= 0; i-- {
		result = append(result, newest[i])
	}

	// From ai_conversations table get mode
	var mode string
	db.QueryRow(`SELECT COALESCE(mode, '') FROM ai_conversations WHERE id=? AND user_id=?`, sessionID, userID).Scan(&mode)

	// Fallback for non-agent modes (chat/generate) that store messages as JSON blob.
	// Pagination only applies to conversation_messages; the JSON-blob path is
	// small and always returns everything (limit=0 path only).
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

	return result, hasMore, mode, nil
}
