package handler

import (
	"database/sql"
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

type TagsHandler struct {
	db *sql.DB
}

func NewTagsHandler(db *sql.DB) *TagsHandler {
	return &TagsHandler{db: db}
}

func (h *TagsHandler) List(c fiber.Ctx) error {
	// 确保表存在
	h.db.Exec(`CREATE TABLE IF NOT EXISTS module_tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		color TEXT DEFAULT '#6366f1',
		usage_count INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	rows, err := h.db.Query("SELECT id, name, color, usage_count FROM module_tags ORDER BY usage_count DESC")
	if err != nil {
		return c.JSON(fiber.Map{"tags": []interface{}{}})
	}
	defer rows.Close()

	type Tag struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		Color      string `json:"color"`
		UsageCount int    `json:"usage_count"`
	}
	tags := []Tag{}
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.UsageCount); err != nil {
			continue
		}
		tags = append(tags, t)
	}
	return c.JSON(fiber.Map{"tags": tags})
}

func (h *TagsHandler) Create(c fiber.Ctx) error {
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Name == "" {
		return ValidationError(c, "name required")
	}
	if req.Color == "" {
		req.Color = "#6366f1"
	}
	_, err := h.db.Exec("INSERT INTO module_tags (name, color) VALUES (?, ?)", req.Name, req.Color)
	if err != nil {
		return c.Status(409).JSON(fiber.Map{"error": "tag already exists"})
	}
	return c.Status(201).JSON(fiber.Map{"ok": true})
}

func (h *TagsHandler) Delete(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	h.db.Exec("DELETE FROM module_tag_relations WHERE tag_id = ?", id)
	h.db.Exec("DELETE FROM module_tags WHERE id = ?", id)
	return c.JSON(fiber.Map{"ok": true})
}

func (h *TagsHandler) SetModuleTags(c fiber.Ctx) error {
	slug := c.Params("slug")
	var req struct {
		TagIDs []int `json:"tag_ids"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}

	tx, err := h.db.Begin()
	if err != nil {
		return InternalError(c, err.Error())
	}
	defer tx.Rollback()

	tx.Exec("DELETE FROM module_tag_relations WHERE module_slug = ?", slug)
	for _, tagID := range req.TagIDs {
		tx.Exec("INSERT OR IGNORE INTO module_tag_relations (module_slug, tag_id) VALUES (?, ?)", slug, tagID)
		tx.Exec("UPDATE module_tags SET usage_count = (SELECT COUNT(*) FROM module_tag_relations WHERE tag_id = ?) WHERE id = ?", tagID, tagID)
	}
	tx.Commit()
	return c.JSON(fiber.Map{"ok": true})
}

func (h *TagsHandler) GetModuleTags(c fiber.Ctx) error {
	slug := c.Params("slug")
	rows, err := h.db.Query(
		"SELECT t.id, t.name, t.color, t.usage_count FROM module_tags t INNER JOIN module_tag_relations r ON t.id = r.tag_id WHERE r.module_slug = ?",
		slug,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type Tag struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		Color      string `json:"color"`
		UsageCount int    `json:"usage_count"`
	}
	tags := []Tag{}
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.UsageCount); err != nil {
			continue
		}
		tags = append(tags, t)
	}
	return c.JSON(fiber.Map{"tags": tags})
}

// ParseMarketTags returns tags string from market_modules field as a slice
func ParseMarketTags(tagsStr string) []string {
	if tagsStr == "" {
		return nil
	}
	var tags []string
	json.Unmarshal([]byte(tagsStr), &tags)
	return tags
}
