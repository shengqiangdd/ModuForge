package handler

import (
	"database/sql"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

type SearchHistoryHandler struct {
	db *sql.DB
}

func NewSearchHistoryHandler(db *sql.DB) *SearchHistoryHandler {
	return &SearchHistoryHandler{db: db}
}

func (h *SearchHistoryHandler) GetHistory(c fiber.Ctx) error {
	userID := c.Locals("uid")
	if userID == nil {
		return Unauthorized(c, "unauthorized")
	}
	uid := userID.(string)

	rows, err := h.db.Query(
		`SELECT id, query, result_count, searched_at FROM search_history WHERE user_id = ? GROUP BY query ORDER BY MAX(searched_at) DESC LIMIT 20`, uid,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type HistoryItem struct {
		ID          int64  `json:"id"`
		Query       string `json:"query"`
		ResultCount int    `json:"result_count"`
		SearchedAt  string `json:"searched_at"`
	}
	var items []HistoryItem
	for rows.Next() {
		var item HistoryItem
		if err := rows.Scan(&item.ID, &item.Query, &item.ResultCount, &item.SearchedAt); err != nil {
			continue
		}
		items = append(items, item)
	}
	if items == nil {
		items = []HistoryItem{}
	}
	return c.JSON(fiber.Map{"history": items})
}

func (h *SearchHistoryHandler) DeleteHistory(c fiber.Ctx) error {
	userID := c.Locals("uid")
	if userID == nil {
		return Unauthorized(c, "unauthorized")
	}
	uid := userID.(string)

	idStr := c.Params("id")
	if idStr == "" || idStr == "all" {
		if _, err := h.db.Exec("DELETE FROM search_history WHERE user_id = ?", uid); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
	} else {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
		}
		if _, err := h.db.Exec("DELETE FROM search_history WHERE id = ? AND user_id = ?", id, uid); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *SearchHistoryHandler) ClearHistory(c fiber.Ctx) error {
	userID := c.Locals("uid")
	if userID == nil {
		return Unauthorized(c, "unauthorized")
	}
	uid := userID.(string)
	if _, err := h.db.Exec("DELETE FROM search_history WHERE user_id = ?", uid); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func LogSearchHistory(db *sql.DB, userID, query string, resultCount int) {
	if userID == "" || query == "" {
		return
	}
	db.Exec("INSERT INTO search_history (user_id, query, result_count) VALUES (?, ?, ?)", userID, query, resultCount)
}
