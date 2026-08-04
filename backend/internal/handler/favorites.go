package handler

import (
	"database/sql"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
)

type FavoritesHandler struct {
	db *sql.DB
}

func NewFavoritesHandler(db *sql.DB) *FavoritesHandler {
	return &FavoritesHandler{db: db}
}

func (h *FavoritesHandler) List(c fiber.Ctx) error {
	userID := c.Locals("uid")
	if userID == nil {
		return Unauthorized(c, "unauthorized")
	}
	uid := userID.(string)
	itemType := c.Query("type")

	query := "SELECT id, item_type, item_id, created_at FROM favorites WHERE user_id = ?"
	args := []interface{}{uid}
	if itemType != "" {
		query += " AND item_type = ?"
		args = append(args, itemType)
	}
	query += " ORDER BY created_at DESC"

	rows, err := h.db.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type Favorite struct {
		ID        int64  `json:"id"`
		ItemType  string `json:"item_type"`
		ItemID    int64  `json:"item_id"`
		CreatedAt string `json:"created_at"`
	}
	var items []Favorite
	for rows.Next() {
		var f Favorite
		if err := rows.Scan(&f.ID, &f.ItemType, &f.ItemID, &f.CreatedAt); err != nil {
			continue
		}
		items = append(items, f)
	}
	if items == nil {
		items = []Favorite{}
	}
	return c.JSON(fiber.Map{"favorites": items})
}

func (h *FavoritesHandler) Add(c fiber.Ctx) error {
	userID := c.Locals("uid")
	if userID == nil {
		return Unauthorized(c, "unauthorized")
	}
	uid := userID.(string)

	var req struct {
		Type string `json:"type"`
		ID   int64  `json:"id"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Type == "" || req.ID <= 0 {
		return ValidationError(c, "type and id required")
	}

	now := time.Now()
	_, err := h.db.Exec(
		"INSERT OR IGNORE INTO favorites (user_id, item_type, item_id, created_at) VALUES (?, ?, ?, ?)",
		uid, req.Type, req.ID, now,
	)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *FavoritesHandler) Remove(c fiber.Ctx) error {
	userID := c.Locals("uid")
	if userID == nil {
		return Unauthorized(c, "unauthorized")
	}
	uid := userID.(string)

	itemType := c.Params("type")
	itemID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}

	if _, err := h.db.Exec("DELETE FROM favorites WHERE user_id = ? AND item_type = ? AND item_id = ?", uid, itemType, itemID); err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *FavoritesHandler) Check(c fiber.Ctx) error {
	userID := c.Locals("uid")
	if userID == nil {
		return Unauthorized(c, "unauthorized")
	}
	uid := userID.(string)

	itemType := c.Params("type")
	itemID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}

	var count int
	h.db.QueryRow("SELECT COUNT(*) FROM favorites WHERE user_id = ? AND item_type = ? AND item_id = ?", uid, itemType, itemID).Scan(&count)
	return c.JSON(fiber.Map{"favorited": count > 0})
}
