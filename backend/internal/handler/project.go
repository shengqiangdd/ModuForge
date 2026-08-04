package handler

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/builder"
	"github.com/moduforge/backend/internal/domain"
	"github.com/moduforge/backend/internal/service"
)

type ProjectHandler struct {
	svc *service.ProjectService
	db  *sql.DB
}

func NewProjectHandler(svc *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{svc: svc}
}

func NewProjectHandlerWithDB(svc *service.ProjectService, db *sql.DB) *ProjectHandler {
	return &ProjectHandler{svc: svc, db: db}
}

func (h *ProjectHandler) List(c fiber.Ctx) error {
	userID := c.Locals("uid")
	if userID == nil {
		return Unauthorized(c, "未授权")
	}
	projects, err := h.svc.List(c.Context(), userID.(string))
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(projects)
}

func (h *ProjectHandler) Create(c fiber.Ctx) error {
	userID := c.Locals("uid")
	if userID == nil {
		return Unauthorized(c, "未授权")
	}
	var req domain.CreateProjectInput
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "请求格式无效")
	}
	if msg := ValidateProjectName(req.Name); msg != "" {
		return ValidationError(c, msg)
	}
	if req.Description != "" && len(req.Description) > 500 {
		return ValidationError(c, "描述不能超过500个字符")
	}
	project, err := h.svc.Create(c.Context(), userID.(string), &req)
	if err != nil {
		return ErrorResponse(c, 400, err.Error(), ErrCodeConflict)
	}
	return c.Status(201).JSON(project)
}

func (h *ProjectHandler) Get(c fiber.Ctx) error {
	id := c.Params("id")
	// Use ownership-checked query when user is authenticated
	userID := ""
	if u := c.Locals("uid"); u != nil {
		if s, ok := u.(string); ok {
			userID = s
		}
	}
	project, err := h.svc.GetByUser(c.Context(), id, userID)
	if err != nil {
		return NotFound(c, "项目不存在")
	}
	return c.JSON(project)
}

func (h *ProjectHandler) Update(c fiber.Ctx) error {
	id := c.Params("id")
	var req domain.UpdateProjectInput
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "请求格式无效")
	}
	if req.Name != nil {
		if msg := ValidateProjectName(*req.Name); msg != "" {
			return ValidationError(c, msg)
		}
	}
	// Use ownership-checked update
	userID := ""
	if u := c.Locals("uid"); u != nil {
		if s, ok := u.(string); ok {
			userID = s
		}
	}
	project, err := h.svc.UpdateByUser(c.Context(), id, userID, &req)
	if err != nil {
		return ErrorResponse(c, 400, err.Error(), ErrCodeConflict)
	}
	return c.JSON(project)
}

func (h *ProjectHandler) Delete(c fiber.Ctx) error {
	id := c.Params("id")

	// Extract user ID as string (matches JWT Claims.UID type)
	userID := ""
	if u := c.Locals("user_id"); u != nil {
		if s, ok := u.(string); ok {
			userID = s
		}
	}
	if userID == "" {
		if u := c.Locals("uid"); u != nil {
			if s, ok := u.(string); ok {
				userID = s
			}
		}
	}

	// Verify project exists and belongs to this user
	var name string
	var ownerID string
	err := h.db.QueryRow("SELECT name, user_id FROM projects WHERE id = ? AND deleted_at IS NULL", id).Scan(&name, &ownerID)
	if err != nil {
		return ErrorResponse(c, 404, "project not found", ErrCodeNotFound)
	}
	if userID != "" && ownerID != "" && ownerID != userID {
		return ErrorResponse(c, 403, "not your project", ErrCodeForbidden)
	}

	// Delete everything in a single transaction (including recycle_bin insert)
	if err := h.svc.DeleteWithRecycle(c.Context(), id, userID, name); err != nil {
		slog.Error("Delete project failed", "id", id, "error", err)
		return ErrorResponse(c, 500, "delete failed: "+err.Error(), ErrCodeInternal)
	}
	return c.SendStatus(204)
}

func (h *ProjectHandler) ListFiles(c fiber.Ctx) error {
	projectID := c.Params("id")
	if projectID == "" {
		return c.JSON([]interface{}{}) // 空 ID 返回空数组，不报错
	}
	files, err := h.svc.ListFiles(c.Context(), projectID)
	if err != nil {
		return ErrorResponse(c, 400, err.Error(), ErrCodeInternal)
	}
	return c.JSON(files)
}

func (h *ProjectHandler) GetFile(c fiber.Ctx) error {
	projectID := c.Params("id")
	path := c.Params("*")
	file, err := h.svc.GetFile(c.Context(), projectID, path)
	if err != nil {
		return NotFound(c, "文件不存在")
	}
	return c.JSON(file)
}

func (h *ProjectHandler) SaveFile(c fiber.Ctx) error {
	projectID := c.Params("id")
	path := c.Params("*")
	var body struct {
		Content string `json:"content"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return BadRequest(c, "请求格式无效")
	}
	if path == "" {
		return ValidationError(c, "文件路径不能为空")
	}
	file, err := h.svc.SaveFile(c.Context(), projectID, path, body.Content)
	if err != nil {
		return ErrorResponse(c, 400, err.Error(), ErrCodeInternal)
	}

	// After saving a build config file, run integrity check
	resp := fiber.Map{"file": file}
	if isBuildConfig(path) {
		if validation := h.validateProject(projectID); validation != nil {
			resp["validation"] = validation
		}
	}
	return c.JSON(resp)
}

// ValidateProject runs project integrity checks and returns missing file warnings.
func (h *ProjectHandler) ValidateProject(c fiber.Ctx) error {
	projectID := c.Params("id")
	validation := h.validateProject(projectID)
	if validation == nil {
		return c.JSON(fiber.Map{"valid": true, "warnings": []interface{}{}, "errors": []interface{}{}})
	}
	return c.JSON(validation)
}

// validateProject collects all files from DB, writes them to a temp dir, runs validation.
func (h *ProjectHandler) validateProject(projectID string) *builder.ValidationResult {
	ctx := h.db // use raw db for temp dir approach
	_ = ctx

	// Collect files from DB
	rows, err := h.db.Query(
		`SELECT path, content FROM project_files WHERE project_id=?`, projectID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	// Create temp dir and write all files
	tmpDir, err := os.MkdirTemp("", "moduforge-validate-*")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(tmpDir)

	fileCount := 0
	for rows.Next() {
		var path, content string
		if err := rows.Scan(&path, &content); err != nil {
			continue
		}
		fullPath := filepath.Join(tmpDir, filepath.Clean(path))
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
		fileCount++
	}

	if fileCount == 0 {
		return nil
	}

	// Run validation
	return builder.ValidateProjectIntegrity(tmpDir)
}

// isBuildConfig checks if the file path is a build configuration file.
func isBuildConfig(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "cmakelists.txt", "android.mk", "application.mk", "makefile", "cargo.toml":
		return true
	}
	return strings.HasSuffix(base, ".mk")
}

func (h *ProjectHandler) UploadFiles(c fiber.Ctx) error {
	projectID := c.Params("id")
	form, err := c.MultipartForm()
	if err != nil {
		return BadRequest(c, "invalid multipart form")
	}
	files := form.File["files"]
	if len(files) == 0 {
		return BadRequest(c, "no files provided")
	}
	var results []string
	for _, f := range files {
		path := strings.TrimLeft(f.Filename, "/")
		src, err := f.Open()
		if err != nil {
			results = append(results, fmt.Sprintf("%s: open error", f.Filename))
			continue
		}
		buf := make([]byte, f.Size)
		src.Read(buf)
		src.Close()
		content := string(buf)
		if _, err := h.svc.SaveFile(c.Context(), projectID, path, content); err != nil {
			results = append(results, fmt.Sprintf("%s: save error", f.Filename))
			continue
		}
		results = append(results, fmt.Sprintf("%s: ok", f.Filename))
	}
	return c.JSON(fiber.Map{"results": results})
}

func (h *ProjectHandler) DeleteFile(c fiber.Ctx) error {
	projectID := c.Params("id")
	path := c.Params("*")
	if path == "" {
		return ValidationError(c, "文件路径不能为空")
	}
	result, err := h.db.Exec(`DELETE FROM project_files WHERE project_id=? AND path=?`, projectID, path)
	if err != nil {
		return InternalError(c, err.Error())
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return NotFound(c, "文件不存在")
	}
	return c.JSON(fiber.Map{"ok": true, "deleted": path})
}

func (h *ProjectHandler) Search(c fiber.Ctx) error {
	userID := c.Locals("uid")
	if userID == nil {
		return Unauthorized(c, "未授权")
	}
	query := c.Query("q")
	if query == "" {
		return c.JSON(fiber.Map{"results": []service.SearchResult{}})
	}
	results, err := h.svc.SearchAll(c.Context(), userID.(string), query)
	if err != nil {
		return InternalError(c, err.Error())
	}
	go LogSearchHistory(h.db, userID.(string), query, len(results))
	return c.JSON(fiber.Map{"results": results})
}

func (h *ProjectHandler) ListTemplates(c fiber.Ctx) error {
	templates := []map[string]string{
		{"name": "通用模块模板", "type": "universal", "desc": "兼容 Magisk / KernelSU / APatch 的通用模块"},
	}
	return c.JSON(templates)
}
