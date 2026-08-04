package handler

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

type GlossaryHandler struct {
	db *sql.DB
}

func NewGlossaryHandler(db *sql.DB) *GlossaryHandler {
	return &GlossaryHandler{db: db}
}

func (h *GlossaryHandler) List(c fiber.Ctx) error {
	category := c.Query("category")
	search := c.Query("search")
	query := "SELECT id, term, definition, category, related_terms, created_at FROM glossary WHERE 1=1"
	args := []interface{}{}
	if category != "" {
		query += " AND category = ?"
		args = append(args, category)
	}
	if search != "" {
		query += " AND (term LIKE ? OR definition LIKE ?)"
		s := "%" + search + "%"
		args = append(args, s, s)
	}
	query += " ORDER BY term ASC"
	rows, err := h.db.Query(query, args...)
	if err != nil {
		return InternalError(c, err.Error())
	}
	defer rows.Close()
	type GlossaryItem struct {
		ID          int64  `json:"id"`
		Term        string `json:"term"`
		Definition  string `json:"definition"`
		Category    string `json:"category"`
		RelatedTerms string `json:"related_terms"`
		CreatedAt   string `json:"created_at"`
	}
	var items []GlossaryItem
	for rows.Next() {
		var item GlossaryItem
		if err := rows.Scan(&item.ID, &item.Term, &item.Definition, &item.Category, &item.RelatedTerms, &item.CreatedAt); err != nil {
			continue
		}
		items = append(items, item)
	}
	if items == nil {
		items = []GlossaryItem{}
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *GlossaryHandler) Get(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	var item struct {
		ID          int64  `json:"id"`
		Term        string `json:"term"`
		Definition  string `json:"definition"`
		Category    string `json:"category"`
		RelatedTerms string `json:"related_terms"`
		CreatedAt   string `json:"created_at"`
	}
	err = h.db.QueryRow("SELECT id, term, definition, category, related_terms, created_at FROM glossary WHERE id = ?", id).
		Scan(&item.ID, &item.Term, &item.Definition, &item.Category, &item.RelatedTerms, &item.CreatedAt)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	}
	return c.JSON(item)
}

func (h *GlossaryHandler) Create(c fiber.Ctx) error {
	var req struct {
		Term        string `json:"term"`
		Definition  string `json:"definition"`
		Category    string `json:"category"`
		RelatedTerms string `json:"related_terms"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Term == "" || req.Definition == "" {
		return ValidationError(c, "term and definition required")
	}
	if req.Category == "" {
		req.Category = "general"
	}
	_, err := h.db.Exec("INSERT INTO glossary (term, definition, category, related_terms) VALUES (?, ?, ?, ?)",
		req.Term, req.Definition, req.Category, req.RelatedTerms)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return c.Status(409).JSON(fiber.Map{"error": "term already exists"})
		}
		return InternalError(c, err.Error())
	}
	return c.Status(201).JSON(fiber.Map{"ok": true})
}

func (h *GlossaryHandler) Update(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	var req struct {
		Term        *string `json:"term"`
		Definition  *string `json:"definition"`
		Category    *string `json:"category"`
		RelatedTerms *string `json:"related_terms"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	var sets []string
	var args []interface{}
	if req.Term != nil { sets = append(sets, "term = ?"); args = append(args, *req.Term) }
	if req.Definition != nil { sets = append(sets, "definition = ?"); args = append(args, *req.Definition) }
	if req.Category != nil { sets = append(sets, "category = ?"); args = append(args, *req.Category) }
	if req.RelatedTerms != nil { sets = append(sets, "related_terms = ?"); args = append(args, *req.RelatedTerms) }
	if len(sets) == 0 {
		return BadRequest(c, "no fields to update")
	}
	args = append(args, id)
	q := "UPDATE glossary SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	_, err = h.db.Exec(q, args...)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *GlossaryHandler) Delete(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	h.db.Exec("DELETE FROM glossary WHERE id = ?", id)
	return c.JSON(fiber.Map{"ok": true})
}

func (h *GlossaryHandler) Popular(c fiber.Ctx) error {
	rows, err := h.db.Query("SELECT id, term, definition, category, related_terms, created_at FROM glossary ORDER BY id LIMIT 10")
	if err != nil {
		return InternalError(c, err.Error())
	}
	defer rows.Close()
	type GlossaryItem struct {
		ID          int64  `json:"id"`
		Term        string `json:"term"`
		Definition  string `json:"definition"`
		Category    string `json:"category"`
		RelatedTerms string `json:"related_terms"`
		CreatedAt   string `json:"created_at"`
	}
	var items []GlossaryItem
	for rows.Next() {
		var item GlossaryItem
		if err := rows.Scan(&item.ID, &item.Term, &item.Definition, &item.Category, &item.RelatedTerms, &item.CreatedAt); err != nil {
			continue
		}
		items = append(items, item)
	}
	if items == nil {
		items = []GlossaryItem{}
	}
	return c.JSON(fiber.Map{"items": items})
}
