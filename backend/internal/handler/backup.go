package handler

import (
	"path/filepath"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

// userIDFromLocals extracts user_id as a string from JWT context.
func userIDFromLocals(c fiber.Ctx) string {
	if uid := c.Locals("user_id"); uid != nil {
		if s, ok := uid.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

type BackupHandler struct {
	svc *service.BackupService
}

func NewBackupHandler(svc *service.BackupService) *BackupHandler {
	return &BackupHandler{svc: svc}
}

func (h *BackupHandler) ExportDatabase(c fiber.Ctx) error {
	// Only admin can export entire database
	role, _ := c.Locals("role").(string)
	if role != "admin" {
		return c.Status(403).JSON(fiber.Map{"error": "admin access required"})
	}
	path, err := h.svc.ExportDatabase(c.Context())
	if err != nil {
		return InternalError(c, "导出数据库失败: "+err.Error())
	}

	c.Attachment()
	return c.SendFile(path)
}

func (h *BackupHandler) ImportDatabase(c fiber.Ctx) error {
	// Only admin can import entire database
	role, _ := c.Locals("role").(string)
	if role != "admin" {
		return c.Status(403).JSON(fiber.Map{"error": "admin access required"})
	}

	form, err := c.MultipartForm()
	if err != nil {
		return BadRequest(c, "请上传 SQL 文件")
	}

	files := form.File["file"]
	if len(files) == 0 {
		return BadRequest(c, "请选择要导入的 SQL 文件")
	}

	f, err := files[0].Open()
	if err != nil {
		return InternalError(c, "读取上传文件失败: "+err.Error())
	}
	defer f.Close()

	// Use random filename to prevent path traversal via uploaded filename
	tmpPath := "data/tmp_import_backup.sql"
	if err := c.SaveFile(files[0], tmpPath); err != nil {
		return InternalError(c, "保存文件失败: "+err.Error())
	}

	if err := h.svc.ImportDatabase(c.Context(), tmpPath); err != nil {
		return InternalError(c, "导入数据库失败: "+err.Error())
	}

	return c.JSON(fiber.Map{"ok": true, "message": "数据库导入成功"})
}

func (h *BackupHandler) ExportProject(c fiber.Ctx) error {
	projectID := filepath.Base(c.Params("id"))
	if projectID == "." || projectID == ".." {
		return BadRequest(c, "invalid project id")
	}

	var req struct {
		Files map[string]string `json:"files"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Files == nil {
		req.Files = make(map[string]string)
	}

	path, err := h.svc.ExportProject(c.Context(), projectID, req.Files)
	if err != nil {
		return InternalError(c, "导出项目失败: "+err.Error())
	}

	c.Attachment()
	return c.SendFile(path)
}

func (h *BackupHandler) ImportProject(c fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		return BadRequest(c, "请上传 ZIP 文件")
	}

	files := form.File["file"]
	if len(files) == 0 {
		return BadRequest(c, "请选择要导入的 ZIP 文件")
	}

	tmpPath := filepath.Join("data", "tmp_import_"+filepath.Base(files[0].Filename))
	if err := c.SaveFile(files[0], tmpPath); err != nil {
		return InternalError(c, "保存文件失败: "+err.Error())
	}

	projectFiles, err := h.svc.ImportProject(c.Context(), tmpPath)
	if err != nil {
		return InternalError(c, "导入项目失败: "+err.Error())
	}

	return c.JSON(fiber.Map{"ok": true, "files": projectFiles, "message": "项目导入成功"})
}

// ===== Backup Schedules =====

func (h *BackupHandler) ListSchedules(c fiber.Ctx) error {
	uid := userIDFromLocals(c)
	if uid == "" {
		return Unauthorized(c, "unauthorized")
	}
	schedules, err := h.svc.ListSchedules(uid)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"schedules": schedules})
}

func (h *BackupHandler) CreateSchedule(c fiber.Ctx) error {
	uid := userIDFromLocals(c)
	if uid == "" {
		return Unauthorized(c, "unauthorized")
	}
	var req struct {
		Name      string `json:"name"`
		Frequency string `json:"frequency"`
		KeepCount int    `json:"keep_count"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Name == "" {
		return ValidationError(c, "name required")
	}
	if req.Frequency == "" {
		req.Frequency = "daily"
	}
	if req.KeepCount < 1 {
		req.KeepCount = 7
	}
	sc, err := h.svc.CreateSchedule(uid, req.Name, req.Frequency, req.KeepCount)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.Status(201).JSON(sc)
}

func (h *BackupHandler) UpdateSchedule(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	uid := userIDFromLocals(c)
	var req struct {
		Name      *string `json:"name"`
		Frequency *string `json:"frequency"`
		KeepCount *int    `json:"keep_count"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Name != nil {
		h.svc.GetDB().Exec("UPDATE backup_schedules SET name = ? WHERE id = ? AND user_id = ?", *req.Name, id, uid)
	}
	if req.Frequency != nil {
		h.svc.GetDB().Exec("UPDATE backup_schedules SET frequency = ? WHERE id = ? AND user_id = ?", *req.Frequency, id, uid)
	}
	if req.KeepCount != nil {
		h.svc.GetDB().Exec("UPDATE backup_schedules SET keep_count = ? WHERE id = ? AND user_id = ?", *req.KeepCount, id, uid)
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *BackupHandler) DeleteSchedule(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	uid := userIDFromLocals(c)
	if err := h.svc.DeleteScheduleByUser(id, uid); err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *BackupHandler) ListHistory(c fiber.Ctx) error {
	uid := userIDFromLocals(c)
	if uid == "" {
		return Unauthorized(c, "unauthorized")
	}
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	history, err := h.svc.ListHistory(uid, limit, offset)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"history": history})
}

func (h *BackupHandler) RunSchedule(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.svc.RunScheduledBackup(id); err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}
