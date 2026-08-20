package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

func (h *AgentHandler) ListSkills(c fiber.Ctx) error {
	var skillsList []map[string]string
	for _, s := range h.runner.ListSkills() {
		skillsList = append(skillsList, map[string]string{
			"name":        s.Name(),
			"description": s.Description(),
		})
	}
	return c.JSON(fiber.Map{"skills": skillsList})
}

// GetSessionState returns session state for a given session ID.
func (h *AgentHandler) GetSessionState(c fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return BadRequest(c, "sessionId required")
	}
	state := h.runner.GetSessionState(sessionID)
	return c.JSON(fiber.Map{"state": state})
}

// ListSessions returns all agent sessions for the current user.
func (h *AgentHandler) ListSessions(c fiber.Ctx) error {
	uid := safeUserID(c)
	if uid == "" {
		return Unauthorized(c, "未授权")
	}

	rows, err := h.db.Conn.Query(
		`SELECT id, title, model, project_id, agent_mode, updated_at
		 FROM ai_conversations
		 WHERE user_id=? AND mode='agent'
		 ORDER BY updated_at DESC LIMIT 100`,
		uid,
	)
	if err != nil {
		return InternalError(c, err.Error())
	}
	defer rows.Close()

	var sessions []map[string]interface{}
	for rows.Next() {
		var id, title, model, projectID, agentMode, updatedAt string
		if err := rows.Scan(&id, &title, &model, &projectID, &agentMode, &updatedAt); err != nil {
			continue
		}
		sessions = append(sessions, map[string]interface{}{
			"id":         id,
			"title":      title,
			"model":      model,
			"project_id": projectID,
			"agent_mode": agentMode,
			"updated_at": updatedAt,
		})
	}
	if sessions == nil {
		sessions = []map[string]interface{}{}
	}
	// Aggregate per-session token usage from persisted conversation messages.
	tokenMap := map[string]int64{}
	if len(sessions) > 0 {
		trows, err := h.db.Conn.Query(`SELECT session_id, SUM(CAST(json_extract(token_usage, '$.total_tokens') AS INTEGER))
			FROM conversation_messages
			WHERE user_id=? AND token_usage IS NOT NULL AND token_usage != ''
			GROUP BY session_id`, uid)
		if err == nil {
			for trows.Next() {
				var sid string
				var tokens int64
				if err := trows.Scan(&sid, &tokens); err == nil {
					tokenMap[sid] = tokens
				}
			}
			trows.Close()
		}
	}
	for _, s := range sessions {
		s["token_usage"] = tokenMap[s["id"].(string)]
	}
	return c.JSON(fiber.Map{"sessions": sessions})
}

// GetSession returns a specific agent session with its messages.
func (h *AgentHandler) GetSession(c fiber.Ctx) error {
	uid := safeUserID(c)
	if uid == "" {
		return Unauthorized(c, "未授权")
	}

	sessionID := c.Params("id")
	if sessionID == "" {
		return BadRequest(c, "session id required")
	}

	// Get session metadata
	var title, model, projectID, agentMode, updatedAt string
	err := h.db.Conn.QueryRow(
		`SELECT title, model, COALESCE(project_id,''), COALESCE(agent_mode,'act'), updated_at
		 FROM ai_conversations
		 WHERE id=? AND user_id=? AND mode='agent'`,
		sessionID, uid,
	).Scan(&title, &model, &projectID, &agentMode, &updatedAt)
	if err != nil {
		return NotFound(c, "session not found")
	}

	// Get messages
	messages, _, err := service.GetConversationMessages(h.db.Conn, sessionID, uid)
	if err != nil {
		return InternalError(c, err.Error())
	}
	if messages == nil {
		messages = []service.ConversationMessage{}
	}

	// Get tool stats from audit log
	toolStats := h.runner.GetToolStats()

	return c.JSON(fiber.Map{
		"id":          sessionID,
		"title":       title,
		"model":       model,
		"project_id":  projectID,
		"agent_mode":  agentMode,
		"updated_at":  updatedAt,
		"messages":    messages,
		"tool_stats":  toolStats,
	})
}
