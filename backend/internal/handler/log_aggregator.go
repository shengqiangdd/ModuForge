package handler

import (
	"database/sql"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
)

type LogAggregatorHandler struct {
	db *sql.DB
}

func NewLogAggregatorHandler(db *sql.DB) *LogAggregatorHandler {
	return &LogAggregatorHandler{db: db}
}

// ListLogs returns logs with optional filtering
func (h *LogAggregatorHandler) ListLogs(c fiber.Ctx) error {
	level := c.Query("level")
	module := c.Query("module")
	startTime := c.Query("start")
	endTime := c.Query("end")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 50
	}
	offset := (page - 1) * limit

	query := "SELECT id, level, module, message, details, created_at FROM app_logs WHERE 1=1"
	countQuery := "SELECT COUNT(*) FROM app_logs WHERE 1=1"
	var args []interface{}

	if level != "" {
		query += " AND level = ?"
		countQuery += " AND level = ?"
		args = append(args, level)
	}
	if module != "" {
		query += " AND module = ?"
		countQuery += " AND module = ?"
		args = append(args, module)
	}
	if startTime != "" {
		query += " AND created_at >= ?"
		countQuery += " AND created_at >= ?"
		args = append(args, startTime)
	}
	if endTime != "" {
		query += " AND created_at <= ?"
		countQuery += " AND created_at <= ?"
		args = append(args, endTime)
	}

	// Get total count
	var total int
	if err := h.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Get logs
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := h.db.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type LogEntry struct {
		ID        int64  `json:"id"`
		Level     string `json:"level"`
		Module    string `json:"module"`
		Message   string `json:"message"`
		Details   string `json:"details"`
		CreatedAt string `json:"created_at"`
	}

	var logs []LogEntry
	for rows.Next() {
		var log LogEntry
		if err := rows.Scan(&log.ID, &log.Level, &log.Module, &log.Message, &log.Details, &log.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, log)
	}
	if logs == nil {
		logs = []LogEntry{}
	}

	return c.JSON(fiber.Map{
		"logs":  logs,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GetLogStats returns log statistics
func (h *LogAggregatorHandler) GetLogStats(c fiber.Ctx) error {
	// Count by level
	levelQuery := `SELECT level, COUNT(*) as count FROM app_logs GROUP BY level`
	levelRows, err := h.db.Query(levelQuery)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer levelRows.Close()

	levelCounts := fiber.Map{}
	for levelRows.Next() {
		var level string
		var count int
		if err := levelRows.Scan(&level, &count); err == nil {
			levelCounts[level] = count
		}
	}

	// Count by module
	moduleQuery := `SELECT module, COUNT(*) as count FROM app_logs GROUP BY module ORDER BY count DESC LIMIT 10`
	moduleRows, err := h.db.Query(moduleQuery)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer moduleRows.Close()

	type ModuleStat struct {
		Module string `json:"module"`
		Count  int    `json:"count"`
	}
	var moduleStats []ModuleStat
	for moduleRows.Next() {
		var stat ModuleStat
		if err := moduleRows.Scan(&stat.Module, &stat.Count); err == nil {
			moduleStats = append(moduleStats, stat)
		}
	}
	if moduleStats == nil {
		moduleStats = []ModuleStat{}
	}

	// Recent trend (last 7 days)
	trendQuery := `SELECT DATE(created_at) as day, COUNT(*) as count FROM app_logs 
		WHERE created_at >= datetime('now', '-7 days') GROUP BY DATE(created_at) ORDER BY day`
	trendRows, err := h.db.Query(trendQuery)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer trendRows.Close()

	type TrendEntry struct {
		Date  string `json:"date"`
		Count int    `json:"count"`
	}
	var trend []TrendEntry
	for trendRows.Next() {
		var entry TrendEntry
		if err := trendRows.Scan(&entry.Date, &entry.Count); err == nil {
			trend = append(trend, entry)
		}
	}
	if trend == nil {
		trend = []TrendEntry{}
	}

	// Total count
	var total int
	h.db.QueryRow("SELECT COUNT(*) FROM app_logs").Scan(&total)

	return c.JSON(fiber.Map{
		"levels":      levelCounts,
		"modules":     moduleStats,
		"trend":       trend,
		"total_logs":  total,
	})
}

// CleanupLogs deletes old logs (default 30 days)
func (h *LogAggregatorHandler) CleanupLogs(c fiber.Ctx) error {
	days, _ := strconv.Atoi(c.Query("days", "30"))
	if days < 1 {
		days = 30
	}

	result, err := h.db.Exec(
		"DELETE FROM app_logs WHERE created_at < datetime('now', '-"+strconv.Itoa(days)+" days')",
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	affected, _ := result.RowsAffected()

	return c.JSON(fiber.Map{
		"deleted": affected,
		"days":    days,
		"message": "已清理 " + strconv.FormatInt(affected, 10) + " 条超过 " + strconv.Itoa(days) + " 天的日志",
	})
}

// SaveLog saves a log entry (internal use)
func (h *LogAggregatorHandler) SaveLog(level, module, message, details string) error {
	_, err := h.db.Exec(
		`INSERT INTO app_logs (level, module, message, details, created_at) VALUES (?, ?, ?, ?, ?)`,
		level, module, message, details, time.Now().Format(time.RFC3339),
	)
	return err
}
