package handler

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
)

type TemplateMarketHandler struct {
	db *sql.DB
}

func NewTemplateMarketHandler(db *sql.DB) *TemplateMarketHandler {
	return &TemplateMarketHandler{db: db}
}

type ModuleTemplate struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Author      string  `json:"author"`
	Downloads   int     `json:"downloads"`
	Rating      float64 `json:"rating"`
	ModuleData  string  `json:"module_data"`
	CreatedAt   string  `json:"created_at"`
}

// GET /templates/market — List templates with filtering, search, pagination
func (h *TemplateMarketHandler) ListTemplates(c fiber.Ctx) error {
	query := c.Query("query")
	category := c.Query("category")
	sortBy := c.Query("sort", "downloads")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage
	where := "WHERE 1=1"
	args := []interface{}{}

	if query != "" {
		where += " AND (name LIKE ? OR description LIKE ?)"
		q := "%" + query + "%"
		args = append(args, q, q)
	}
	if category != "" {
		where += " AND category = ?"
		args = append(args, category)
	}

	orderClause := "ORDER BY downloads DESC"
	switch sortBy {
	case "rating":
		orderClause = "ORDER BY rating DESC"
	case "newest":
		orderClause = "ORDER BY created_at DESC"
	case "name":
		orderClause = "ORDER BY name ASC"
	}

	// Count total
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM module_templates %s", where)
	h.db.QueryRow(countQuery, args...).Scan(&total)

	// Fetch templates
	querySQL := fmt.Sprintf(
		"SELECT id, name, description, category, author, downloads, rating, module_data, created_at FROM module_templates %s %s LIMIT ? OFFSET ?",
		where, orderClause,
	)
	args = append(args, perPage, offset)

	rows, err := h.db.Query(querySQL, args...)
	if err != nil {
		return InternalError(c, "查询模板失败")
	}
	defer rows.Close()

	var templates []ModuleTemplate
	for rows.Next() {
		var t ModuleTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Category, &t.Author, &t.Downloads, &t.Rating, &t.ModuleData, &t.CreatedAt); err == nil {
			templates = append(templates, t)
		}
	}
	if templates == nil {
		templates = []ModuleTemplate{}
	}

	return c.JSON(fiber.Map{
		"templates": templates,
		"total":     total,
		"page":      page,
		"per_page":  perPage,
	})
}

// POST /templates/market — Publish a new template
func (h *TemplateMarketHandler) PublishTemplate(c fiber.Ctx) error {
	uid := c.Locals("uid")
	if uid == nil {
		return Unauthorized(c, "未授权")
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    string `json:"category"`
		ModuleData  string `json:"module_data"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "请求格式无效")
	}
	if req.Name == "" {
		return ValidationError(c, "模板名称不能为空")
	}

	// Get username from uid
	var username string
	h.db.QueryRow("SELECT username FROM users WHERE id=?", uid).Scan(&username)
	if username == "" {
		username = "anonymous"
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := h.db.Exec(
		`INSERT INTO module_templates (name, description, category, author, author_uid, module_data, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		req.Name, req.Description, req.Category, username, uid, req.ModuleData, now,
	)
	if err != nil {
		return InternalError(c, "发布模板失败")
	}

	id, _ := result.LastInsertId()
	return c.Status(201).JSON(fiber.Map{
		"id":      id,
		"message": "模板发布成功",
	})
}

// POST /templates/market/:id/use — Use a template (increment downloads, return data)
func (h *TemplateMarketHandler) UseTemplate(c fiber.Ctx) error {
	id := c.Params("id")

	var moduleData string
	err := h.db.QueryRow("SELECT module_data FROM module_templates WHERE id=?", id).Scan(&moduleData)
	if err != nil {
		return NotFound(c, "模板不存在")
	}

	// Increment download count
	h.db.Exec("UPDATE module_templates SET downloads = downloads + 1 WHERE id=?", id)

	return c.JSON(fiber.Map{
		"module_data": moduleData,
	})
}

// POST /templates/market/:id/rate — Rate a template
func (h *TemplateMarketHandler) RateTemplate(c fiber.Ctx) error {
	id := c.Params("id")
	uid := c.Locals("uid")
	if uid == nil {
		return Unauthorized(c, "未授权")
	}

	var req struct {
		Rating float64 `json:"rating"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "请求格式无效")
	}
	if req.Rating < 1 || req.Rating > 5 {
		return ValidationError(c, "评分必须在1-5之间")
	}

	// Upsert rating
	_, err := h.db.Exec(
		`INSERT INTO template_ratings (template_id, user_id, rating, created_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(template_id, user_id) DO UPDATE SET rating=?, updated_at=?`,
		id, uid, req.Rating, time.Now().UTC().Format(time.RFC3339),
		req.Rating, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return InternalError(c, "评分失败")
	}

	// Recalculate average rating
	var avgRating float64
	h.db.QueryRow("SELECT COALESCE(AVG(rating), 0) FROM template_ratings WHERE template_id=?", id).Scan(&avgRating)
	h.db.Exec("UPDATE module_templates SET rating=? WHERE id=?", avgRating, id)

	return c.JSON(fiber.Map{
		"rating": avgRating,
		"message": "评分成功",
	})
}

// GET /templates/market/trending — Get trending templates
func (h *TemplateMarketHandler) GetTrending(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	rows, err := h.db.Query(
		`SELECT id, name, description, category, author, downloads, rating, module_data, created_at
		 FROM module_templates ORDER BY downloads DESC, rating DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return InternalError(c, "查询热门模板失败")
	}
	defer rows.Close()

	var templates []ModuleTemplate
	for rows.Next() {
		var t ModuleTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Category, &t.Author, &t.Downloads, &t.Rating, &t.ModuleData, &t.CreatedAt); err == nil {
			templates = append(templates, t)
		}
	}
	if templates == nil {
		templates = []ModuleTemplate{}
	}

	return c.JSON(fiber.Map{"templates": templates})
}

// GET /templates/market/categories — List all categories
func (h *TemplateMarketHandler) GetCategories(c fiber.Ctx) error {
	rows, err := h.db.Query(
		"SELECT DISTINCT category, COUNT(*) as count FROM module_templates WHERE category != '' GROUP BY category ORDER BY count DESC",
	)
	if err != nil {
		return InternalError(c, "查询分类失败")
	}
	defer rows.Close()

	type CategoryInfo struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	var categories []CategoryInfo
	for rows.Next() {
		var cat CategoryInfo
		if err := rows.Scan(&cat.Name, &cat.Count); err == nil {
			categories = append(categories, cat)
		}
	}
	if categories == nil {
		categories = []CategoryInfo{}
	}

	return c.JSON(fiber.Map{"categories": categories})
}

// DELETE /templates/market/:id — Delete a template (owner only)
func (h *TemplateMarketHandler) DeleteTemplate(c fiber.Ctx) error {
	id := c.Params("id")
	uid := c.Locals("uid")
	if uid == nil {
		return Unauthorized(c, "未授权")
	}

	var authorUID string
	h.db.QueryRow("SELECT author_uid FROM module_templates WHERE id=?", id).Scan(&authorUID)
	if authorUID != "" && authorUID != uid.(string) {
		return Forbidden(c, "只能删除自己发布的模板")
	}

	result, err := h.db.Exec("DELETE FROM module_templates WHERE id=?", id)
	if err != nil {
		return InternalError(c, "删除模板失败")
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return NotFound(c, "模板不存在")
	}

	return c.JSON(fiber.Map{"message": "删除成功"})
}
