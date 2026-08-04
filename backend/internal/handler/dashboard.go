package handler

import (
	"database/sql"
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

type DashboardHandler struct {
	db *sql.DB
}

func NewDashboardHandler(db *sql.DB) *DashboardHandler {
	return &DashboardHandler{db: db}
}

type Widget struct {
	ID         int64  `json:"id"`
	UserID     string `json:"user_id"`
	WidgetType string `json:"widget_type"`
	Title      string `json:"title"`
	Config     string `json:"config"`
	PositionX  int    `json:"position_x"`
	PositionY  int    `json:"position_y"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	IsVisible  bool   `json:"is_visible"`
	CreatedAt  string `json:"created_at"`
}

// userID returns the user_id as a string (UUID).
// Users.id is TEXT (UUID), so dashboard_widgets.user_id must match.
func (h *DashboardHandler) userID(c fiber.Ctx) string {
	return currentUserID(c)
}

func (h *DashboardHandler) ListWidgets(c fiber.Ctx) error {
	uid := h.userID(c)
	if uid == "" {
		return Unauthorized(c, "unauthorized")
	}
	rows, err := h.db.Query("SELECT id, widget_type, title, config, position_x, position_y, width, height, is_visible, created_at FROM dashboard_widgets WHERE user_id = ? ORDER BY position_y, position_x", uid)
	if err != nil {
		return InternalError(c, err.Error())
	}
	defer rows.Close()
	var widgets []Widget
	for rows.Next() {
		var w Widget
		var vis int
		if err := rows.Scan(&w.ID, &w.WidgetType, &w.Title, &w.Config, &w.PositionX, &w.PositionY, &w.Width, &w.Height, &vis, &w.CreatedAt); err != nil {
			continue
		}
		w.IsVisible = vis == 1
		widgets = append(widgets, w)
	}
	if widgets == nil {
		widgets = []Widget{}
	}
	return c.JSON(fiber.Map{"widgets": widgets})
}

func (h *DashboardHandler) AddWidget(c fiber.Ctx) error {
	uid := h.userID(c)
	if uid == "" {
		return Unauthorized(c, "unauthorized")
	}
	var req struct {
		WidgetType string `json:"widget_type"`
		Title      string `json:"title"`
		Config     string `json:"config"`
		PositionX  int    `json:"position_x"`
		PositionY  int    `json:"position_y"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.WidgetType == "" {
		return ValidationError(c, "widget_type required")
	}
	if req.Title == "" {
		req.Title = req.WidgetType
	}
	if req.Config == "" {
		req.Config = "{}"
	}
	if req.Width < 1 {
		req.Width = 1
	}
	if req.Height < 1 {
		req.Height = 1
	}
	// Prevent duplicate widget types per user
	var existingID int64
	err := h.db.QueryRow("SELECT id FROM dashboard_widgets WHERE user_id = ? AND widget_type = ?", uid, req.WidgetType).Scan(&existingID)
	if err == nil {
		// Already exists — return existing ID instead of creating duplicate
		return c.Status(200).JSON(fiber.Map{"id": existingID, "existed": true})
	}
	res, err := h.db.Exec("INSERT INTO dashboard_widgets (user_id, widget_type, title, config, position_x, position_y, width, height) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", uid, req.WidgetType, req.Title, req.Config, req.PositionX, req.PositionY, req.Width, req.Height)
	if err != nil {
		return InternalError(c, err.Error())
	}
	id, _ := res.LastInsertId()
	return c.Status(201).JSON(fiber.Map{"id": id})
}

func (h *DashboardHandler) UpdateWidget(c fiber.Ctx) error {
	uid := h.userID(c)
	if uid == "" {
		return Unauthorized(c, "unauthorized")
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid widget id"})
	}
	var req struct {
		Title     *string `json:"title"`
		Config    *string `json:"config"`
		PositionX *int    `json:"position_x"`
		PositionY *int    `json:"position_y"`
		Width     *int    `json:"width"`
		Height    *int    `json:"height"`
		IsVisible *bool   `json:"is_visible"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	var sets []string
	var args []interface{}
	if req.Title != nil {
		sets = append(sets, "title = ?")
		args = append(args, *req.Title)
	}
	if req.Config != nil {
		sets = append(sets, "config = ?")
		args = append(args, *req.Config)
	}
	if req.PositionX != nil {
		sets = append(sets, "position_x = ?")
		args = append(args, *req.PositionX)
	}
	if req.PositionY != nil {
		sets = append(sets, "position_y = ?")
		args = append(args, *req.PositionY)
	}
	if req.Width != nil {
		sets = append(sets, "width = ?")
		args = append(args, *req.Width)
	}
	if req.Height != nil {
		sets = append(sets, "height = ?")
		args = append(args, *req.Height)
	}
	if req.IsVisible != nil {
		v := 0
		if *req.IsVisible {
			v = 1
		}
		sets = append(sets, "is_visible = ?")
		args = append(args, v)
	}
	if len(sets) == 0 {
		return BadRequest(c, "no fields to update")
	}
	args = append(args, uid, id)
	q := "UPDATE dashboard_widgets SET "
	for i, s := range sets {
		if i > 0 {
			q += ", "
		}
		q += s
	}
	q += " WHERE user_id = ? AND id = ?"
	_, err = h.db.Exec(q, args...)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *DashboardHandler) DeleteWidget(c fiber.Ctx) error {
	uid := h.userID(c)
	if uid == "" {
		return Unauthorized(c, "unauthorized")
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid widget id"})
	}
	if _, err := h.db.Exec("DELETE FROM dashboard_widgets WHERE user_id = ? AND id = ?", uid, id); err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *DashboardHandler) ReorderWidgets(c fiber.Ctx) error {
	uid := h.userID(c)
	if uid == "" {
		return Unauthorized(c, "unauthorized")
	}
	var req struct {
		Items []struct {
			ID        int64 `json:"id"`
			PositionX int   `json:"position_x"`
			PositionY int   `json:"position_y"`
		} `json:"items"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	tx, err := h.db.Begin()
	if err != nil {
		return InternalError(c, err.Error())
	}
	defer tx.Rollback()
	for _, item := range req.Items {
		_, err := tx.Exec("UPDATE dashboard_widgets SET position_x = ?, position_y = ? WHERE user_id = ? AND id = ?", item.PositionX, item.PositionY, uid, item.ID)
		if err != nil {
			return InternalError(c, err.Error())
		}
	}
	if err := tx.Commit(); err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *DashboardHandler) GetWidgetTypes(c fiber.Ctx) error {
	types := []struct {
		Type string `json:"type"`
		Name string `json:"name"`
		Desc string `json:"desc"`
	}{
		{"system_overview", "系统概览", "查看项目、用户、构建总数等系统级指标"},
		{"build_stats", "构建统计", "构建成功率、总构建数、平均耗时"},
		{"build_trends", "构建趋势", "按日构建成功/失败柱状图"},
		{"market_stats", "市场统计", "模块总数、安装数、星标数、热门分类"},
		{"system_info", "系统信息", "运行时间、数据库大小等信息"},
		{"recent_activity", "最近活动", "最近的操作动态列表"},
		{"trending_modules", "热门趋势", "Top 5 热门模块"},
		{"health_check", "系统健康", "系统服务健康状态监控"},
	}
	data, _ := json.Marshal(types)
	return c.JSON(fiber.Map{"types": json.RawMessage(string(data))})
}
