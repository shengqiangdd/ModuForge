package service

import (
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
)

type ConversationSummary struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Mode         string `json:"mode"`
	Model        string `json:"model"`
	ProjectID    string `json:"project_id"`
	MessageCount int    `json:"message_count"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type ConversationData struct {
	Messages  []Message `json:"messages"`
	Mode      string    `json:"mode"`
	ProjectID string    `json:"project_id"`
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

// RenameSession updates a saved conversation's title, creating a stub record
// when the session only exists in conversation_messages (no ai_conversations row).
func RenameSession(db *sql.DB, sessionID, userID, title string) error {
	res, err := db.Exec(
		`UPDATE ai_conversations SET title=?, updated_at=datetime('now') WHERE id=? AND user_id=?`,
		title, sessionID, userID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	_, err = db.Exec(
		`INSERT OR IGNORE INTO ai_conversations (id, user_id, title, mode, messages, updated_at)
		 VALUES (?, ?, ?, '', '[]', datetime('now'))`,
		sessionID, userID, title,
	)
	return err
}

func ListConversations(db *sql.DB, userID string) ([]ConversationSummary, error) {
	rows, err := db.Query(
		`SELECT id, title, mode, model, COALESCE(project_id, ''), json_array_length(messages), created_at, updated_at
		 FROM ai_conversations WHERE user_id=? ORDER BY updated_at DESC LIMIT 100`,
		userID,
	)
	if err != nil {
		return nil, err
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
