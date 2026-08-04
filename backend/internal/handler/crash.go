package handler

import (
	"database/sql"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

type CrashHandler struct {
	db *sql.DB
}

func NewCrashHandler(db *sql.DB) *CrashHandler {
	return &CrashHandler{db: db}
}

func (h *CrashHandler) Report(c fiber.Ctx) error {
	var req struct {
		DeviceID    string `json:"device_id"`
		ModuleSlug  string `json:"module_slug"`
		ErrorType   string `json:"error_type"`
		StackTrace  string `json:"stack_trace"`
		DeviceInfo  string `json:"device_info"`
		AppVersion  string `json:"app_version"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.DeviceID == "" || req.ErrorType == "" || req.StackTrace == "" {
		return ValidationError(c, "device_id, error_type and stack_trace required")
	}
	if req.DeviceInfo == "" {
		req.DeviceInfo = "{}"
	}
	_, err := h.db.Exec(
		"INSERT INTO crash_logs (device_id, module_slug, error_type, stack_trace, device_info, app_version) VALUES (?, ?, ?, ?, ?, ?)",
		req.DeviceID, req.ModuleSlug, req.ErrorType, req.StackTrace, req.DeviceInfo, req.AppVersion,
	)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.Status(201).JSON(fiber.Map{"ok": true})
}

func (h *CrashHandler) ListLogs(c fiber.Ctx) error {
	query := "SELECT id, device_id, module_slug, error_type, stack_trace, device_info, app_version, created_at FROM crash_logs WHERE 1=1"
	args := []interface{}{}

	if v := c.Query("module"); v != "" {
		query += " AND module_slug = ?"
		args = append(args, v)
	}
	if v := c.Query("type"); v != "" {
		query += " AND error_type = ?"
		args = append(args, v)
	}
	if v := c.Query("device"); v != "" {
		query += " AND device_id = ?"
		args = append(args, v)
	}
	query += " ORDER BY created_at DESC LIMIT 100"

	rows, err := h.db.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type CrashLog struct {
		ID          int64  `json:"id"`
		DeviceID    string `json:"device_id"`
		ModuleSlug  string `json:"module_slug"`
		ErrorType   string `json:"error_type"`
		StackTrace  string `json:"stack_trace"`
		DeviceInfo  string `json:"device_info"`
		AppVersion  string `json:"app_version"`
		CreatedAt   string `json:"created_at"`
	}
	var logs []CrashLog
	for rows.Next() {
		var l CrashLog
		if err := rows.Scan(&l.ID, &l.DeviceID, &l.ModuleSlug, &l.ErrorType, &l.StackTrace, &l.DeviceInfo, &l.AppVersion, &l.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, l)
	}
	if logs == nil {
		logs = []CrashLog{}
	}
	return c.JSON(fiber.Map{"logs": logs})
}

func (h *CrashHandler) Stats(c fiber.Ctx) error {
	var total, today, affectedModules int
	h.db.QueryRow("SELECT COUNT(*) FROM crash_logs").Scan(&total)
	h.db.QueryRow("SELECT COUNT(*) FROM crash_logs WHERE created_at >= datetime('now', '-1 day')").Scan(&today)
	h.db.QueryRow("SELECT COUNT(DISTINCT module_slug) FROM crash_logs WHERE module_slug != ''").Scan(&affectedModules)

	rows, err := h.db.Query("SELECT module_slug, COUNT(*) as cnt FROM crash_logs WHERE module_slug != '' GROUP BY module_slug ORDER BY cnt DESC")
	if err == nil {
		defer rows.Close()
	}
	type ModuleCrash struct {
		Module string `json:"module"`
		Count  int    `json:"count"`
	}
	var byModule []ModuleCrash
	if rows != nil {
		for rows.Next() {
			var m ModuleCrash
			if err := rows.Scan(&m.Module, &m.Count); err == nil {
				byModule = append(byModule, m)
			}
		}
	}
	if byModule == nil {
		byModule = []ModuleCrash{}
	}

	return c.JSON(fiber.Map{
		"total":           total,
		"today":           today,
		"affected_modules": affectedModules,
		"by_module":        byModule,
	})
}

func (h *CrashHandler) Delete(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	_, err = h.db.Exec("DELETE FROM crash_logs WHERE id = ?", id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *CrashHandler) ClearAll(c fiber.Ctx) error {
	if _, err := h.db.Exec("DELETE FROM crash_logs"); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}
