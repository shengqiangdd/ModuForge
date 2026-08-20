package service

import (
	"database/sql"
	"fmt"
	"strings"
)

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
		return nil, err
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
			"session_id": sessionID,
			"role":       role,
			"content":    content,
			"step_type":  stepType,
			"created_at": createdAt,
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
