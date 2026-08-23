package handler

import (
	"database/sql"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

type AIAnalyticsHandler struct {
	db *sql.DB
}

func NewAIAnalyticsHandler(db *sql.DB) *AIAnalyticsHandler {
	return &AIAnalyticsHandler{db: db}
}

// resolveUserID returns the effective user_id filter.
// Admin users may query any user (or all if empty).
// Normal users are locked to their own data.
func (h *AIAnalyticsHandler) resolveUserID(c fiber.Ctx) string {
	role, _ := c.Locals("role").(string)
	uid, _ := c.Locals("uid").(string)
	if role == "admin" {
		return c.Query("user_id", "") // admin can filter or see all
	}
	return uid // normal user: locked to self
}

// Overview returns aggregate stats across users
func (h *AIAnalyticsHandler) Overview(c fiber.Ctx) error {
	userID := h.resolveUserID(c)

	var totalUsers, totalConversations, totalMessages int64
	var totalTokens int64
	var totalModels int

	userFilter := ""
	args := []interface{}{}
	if userID != "" {
		userFilter = " WHERE user_id = ?"
		args = append(args, userID)
	}

	h.db.QueryRow("SELECT COUNT(DISTINCT user_id) FROM ai_conversations"+userFilter, args...).Scan(&totalUsers)
	h.db.QueryRow("SELECT COUNT(*) FROM ai_conversations"+userFilter, args...).Scan(&totalConversations)
	h.db.QueryRow("SELECT COUNT(*) FROM conversation_messages"+userFilter, args...).Scan(&totalMessages)
	h.db.QueryRow("SELECT COALESCE(SUM(token_usage), 0) FROM ai_conversations"+userFilter, args...).Scan(&totalTokens)
	h.db.QueryRow("SELECT COUNT(DISTINCT model) FROM ai_conversations WHERE model != ''"+userFilter, args...).Scan(&totalModels)

	// Build stats: success rate
	var totalBuilds, successBuilds int64
	h.db.QueryRow("SELECT COUNT(*) FROM build_tasks").Scan(&totalBuilds)
	h.db.QueryRow("SELECT COUNT(*) FROM build_tasks WHERE status = 'success'").Scan(&successBuilds)

	buildSuccessRate := 0.0
	if totalBuilds > 0 {
		buildSuccessRate = float64(successBuilds) / float64(totalBuilds) * 100
	}

	return c.JSON(fiber.Map{
		"total_users":        totalUsers,
		"total_conversations": totalConversations,
		"total_messages":      totalMessages,
		"total_tokens":        totalTokens,
		"total_models":        totalModels,
		"build_success_rate":  buildSuccessRate,
		"total_builds":        totalBuilds,
	})
}

// Users returns per-user token consumption and usage stats (admin-only sees all, normal user sees only self)
func (h *AIAnalyticsHandler) Users(c fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	uid, _ := c.Locals("uid").(string)
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	// Normal users can only see their own stats
	if role != "admin" {
		type UserStat struct {
			ID                string `json:"id"`
			Username          string `json:"username"`
			TotalTokens       int64  `json:"total_tokens"`
			ConversationCount int64  `json:"conversation_count"`
			FavoriteModel     string `json:"favorite_model"`
		}
		var u UserStat
		err := h.db.QueryRow(`
			SELECT
				COALESCE(u.id, ?) as id,
				COALESCE(u.username, 'unknown') as username,
				COALESCE(SUM(ac.token_usage), 0) as total_tokens,
				COUNT(DISTINCT ac.id) as conversation_count,
				(
					SELECT model FROM ai_conversations ac2
					WHERE ac2.user_id = ? AND ac2.model != ''
					GROUP BY model ORDER BY COUNT(*) DESC LIMIT 1
				) as favorite_model
			FROM ai_conversations ac
			LEFT JOIN users u ON u.id = ac.user_id
			WHERE ac.user_id = ?
			GROUP BY ac.user_id`, uid, uid, uid).Scan(&u.ID, &u.Username, &u.TotalTokens, &u.ConversationCount, &u.FavoriteModel)
		if err != nil {
			// No data yet
			return c.JSON(fiber.Map{"users": []UserStat{}})
		}
		return c.JSON(fiber.Map{"users": []UserStat{u}})
	}

	// Admin: full user list
	rows, err := h.db.Query(`
		SELECT
			u.id,
			u.username,
			COALESCE(SUM(ac.token_usage), 0) as total_tokens,
			COUNT(DISTINCT ac.id) as conversation_count,
			(
				SELECT model FROM ai_conversations ac2
				WHERE ac2.user_id = u.id AND ac2.model != ''
				GROUP BY model ORDER BY COUNT(*) DESC LIMIT 1
			) as favorite_model
		FROM users u
		LEFT JOIN ai_conversations ac ON ac.user_id = u.id
		GROUP BY u.id
		ORDER BY total_tokens DESC
		LIMIT ?`, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type UserStat struct {
		ID                string `json:"id"`
		Username          string `json:"username"`
		TotalTokens       int64  `json:"total_tokens"`
		ConversationCount int64  `json:"conversation_count"`
		FavoriteModel     string `json:"favorite_model"`
	}

	var users []UserStat
	for rows.Next() {
		var u UserStat
		rows.Scan(&u.ID, &u.Username, &u.TotalTokens, &u.ConversationCount, &u.FavoriteModel)
		users = append(users, u)
	}
	if users == nil {
		users = []UserStat{}
	}

	return c.JSON(fiber.Map{"users": users})
}

// Models returns per-model usage stats
func (h *AIAnalyticsHandler) Models(c fiber.Ctx) error {
	userID := h.resolveUserID(c)

	query := `
		SELECT
			COALESCE(model, 'unknown') as model,
			COUNT(*) as call_count,
			COALESCE(SUM(token_usage), 0) as total_tokens,
			COUNT(DISTINCT user_id) as user_count
		FROM ai_conversations
		WHERE model != ''
	`
	args := []interface{}{}
	if userID != "" {
		query += " AND user_id = ?"
		args = append(args, userID)
	}
	query += " GROUP BY model ORDER BY total_tokens DESC"

	rows, err := h.db.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type ModelStat struct {
		Model      string `json:"model"`
		CallCount  int64  `json:"call_count"`
		TotalTokens int64 `json:"total_tokens"`
		UserCount  int64  `json:"user_count"`
	}

	var models []ModelStat
	for rows.Next() {
		var m ModelStat
		rows.Scan(&m.Model, &m.CallCount, &m.TotalTokens, &m.UserCount)
		models = append(models, m)
	}
	if models == nil {
		models = []ModelStat{}
	}

	return c.JSON(fiber.Map{"models": models})
}

// Timeline returns daily usage trends
func (h *AIAnalyticsHandler) Timeline(c fiber.Ctx) error {
	days, _ := strconv.Atoi(c.Query("days", "30"))
	userID := h.resolveUserID(c)
	if days <= 0 || days > 365 {
		days = 30
	}

	query := `
		SELECT
			date,
			SUM(llm_call_count) as call_count,
			SUM(llm_token_usage) as token_usage
		FROM ai_usage_daily
		WHERE date >= date('now', '-' || ? || ' days')
	`
	args := []interface{}{days}
	if userID != "" {
		query += " AND user_id = ?"
		args = append(args, userID)
	}
	query += " GROUP BY date ORDER BY date ASC"

	rows, err := h.db.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type TimelineEntry struct {
		Date        string `json:"date"`
		CallCount   int64  `json:"call_count"`
		TokenUsage  int64  `json:"token_usage"`
	}

	var timeline []TimelineEntry
	for rows.Next() {
		var t TimelineEntry
		rows.Scan(&t.Date, &t.CallCount, &t.TokenUsage)
		timeline = append(timeline, t)
	}
	if timeline == nil {
		timeline = []TimelineEntry{}
	}

	return c.JSON(fiber.Map{"timeline": timeline})
}

// Modes returns per-mode usage stats
func (h *AIAnalyticsHandler) Modes(c fiber.Ctx) error {
	userID := h.resolveUserID(c)

	query := `
		SELECT
			COALESCE(mode, 'unknown') as mode,
			COUNT(*) as conversation_count,
			COALESCE(SUM(token_usage), 0) as total_tokens,
			COUNT(DISTINCT model) as model_count
		FROM ai_conversations
	`
	args := []interface{}{}
	if userID != "" {
		query += " WHERE user_id = ?"
		args = append(args, userID)
	}
	query += " GROUP BY mode ORDER BY total_tokens DESC"

	rows, err := h.db.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type ModeStat struct {
		Mode              string `json:"mode"`
		ConversationCount int64  `json:"conversation_count"`
		TotalTokens       int64  `json:"total_tokens"`
		ModelCount        int64  `json:"model_count"`
	}

	var modes []ModeStat
	for rows.Next() {
		var m ModeStat
		rows.Scan(&m.Mode, &m.ConversationCount, &m.TotalTokens, &m.ModelCount)
		modes = append(modes, m)
	}
	if modes == nil {
		modes = []ModeStat{}
	}

	return c.JSON(fiber.Map{"modes": modes})
}
