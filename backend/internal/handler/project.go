package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
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
	userID := c.Locals("uid")
	uid := ""
	if userID != nil {
		if s, ok := userID.(string); ok {
			uid = s
		}
	}
	if uid == "" {
		return Unauthorized(c, "未授权")
	}
	files, err := h.svc.ListFiles(c.Context(), projectID, uid)
	if err != nil {
		return ErrorResponse(c, 400, err.Error(), ErrCodeInternal)
	}
	return c.JSON(files)
}

func (h *ProjectHandler) GetFile(c fiber.Ctx) error {
	projectID := c.Params("id")
	path := c.Params("*")
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	userID := c.Locals("uid")
	uid := ""
	if userID != nil {
		if s, ok := userID.(string); ok {
			uid = s
		}
	}
	if uid == "" {
		return Unauthorized(c, "未授权")
	}
	file, err := h.svc.GetFile(c.Context(), projectID, path, uid)
	if err != nil {
		return NotFound(c, "文件不存在")
	}
	return c.JSON(file)
}

func (h *ProjectHandler) SaveFile(c fiber.Ctx) error {
	projectID := c.Params("id")
	path := c.Params("*")
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	userID := c.Locals("uid")
	uid := ""
	if userID != nil {
		if s, ok := userID.(string); ok {
			uid = s
		}
	}
	if uid == "" {
		return Unauthorized(c, "未授权")
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return BadRequest(c, "请求格式无效")
	}
	if path == "" {
		return ValidationError(c, "文件路径不能为空")
	}
	file, err := h.svc.SaveFile(c.Context(), projectID, path, body.Content, uid)
	if err != nil {
		return ErrorResponse(c, 400, err.Error(), ErrCodeInternal)
	}

	// After saving a build config file, run integrity check
	resp := fiber.Map{"file": file}
	if isBuildConfig(path) {
		if validation := h.validateProject(c.Context(), projectID); validation != nil {
			resp["validation"] = validation
		}
	}
	return c.JSON(resp)
}

// ValidateProject runs project integrity checks and returns missing file warnings.
func (h *ProjectHandler) ValidateProject(c fiber.Ctx) error {
	projectID := c.Params("id")
	validation := h.validateProject(c.Context(), projectID)
	if validation == nil {
		return c.JSON(fiber.Map{"valid": true, "warnings": []interface{}{}, "errors": []interface{}{}})
	}
	return c.JSON(validation)
}

// validateProject collects all files from S3 (or DB fallback), writes them to
// a temp dir, runs validation.
func (h *ProjectHandler) validateProject(ctx context.Context, projectID string) *builder.ValidationResult {
	tmpDir, err := h.svc.ExportToTempDir(ctx, projectID)
	if err != nil {
		return nil
	}
	defer os.RemoveAll(tmpDir)

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
	userID := c.Locals("uid")
	uid := ""
	if userID != nil {
		if s, ok := userID.(string); ok {
			uid = s
		}
	}
	if uid == "" {
		return Unauthorized(c, "未授权")
	}
	form, err := c.MultipartForm()
	if err != nil {
		return BadRequest(c, "invalid multipart form")
	}
	files := form.File["files"]
	if len(files) == 0 {
		return BadRequest(c, "no files provided")
	}

	// paths field: optional comma-separated list of full paths matching each file.
	// When provided, these paths are used instead of the multipart filename,
	// preserving subdirectory structure (e.g., "config/tool.py").
	pathsField := form.Value["paths"]
	var overridePaths []string
	if len(pathsField) > 0 {
		overridePaths = strings.Split(pathsField[0], ",")
	}

	var results []string
	for i, f := range files {
		// Determine the file path: prefer explicit paths field, fall back to filename
		var path string
		if i < len(overridePaths) && overridePaths[i] != "" {
			path = strings.TrimSpace(overridePaths[i])
			path = strings.TrimLeft(path, "/")
		} else {
			path = strings.TrimLeft(f.Filename, "/")
			// Some HTTP clients strip directory from filename —
			// if the name has no slash but the content looks like
			// it belongs in a subdirectory, leave it at root.
		}

		src, err := f.Open()
		if err != nil {
			results = append(results, fmt.Sprintf("%s: open error", f.Filename))
			continue
		}
		buf := make([]byte, f.Size)
		src.Read(buf)
		src.Close()
		content := string(buf)
		if _, err := h.svc.SaveFile(c.Context(), projectID, path, content, uid); err != nil {
			results = append(results, fmt.Sprintf("%s: save error", f.Filename))
			continue
		}
		results = append(results, fmt.Sprintf("%s: ok", path))
	}
	return c.JSON(fiber.Map{"results": results})
}

func (h *ProjectHandler) DeleteFile(c fiber.Ctx) error {
	projectID := c.Params("id")
	path := c.Params("*")
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	if path == "" {
		return ValidationError(c, "文件路径不能为空")
	}
	userID := c.Locals("uid")
	uid := ""
	if userID != nil {
		if s, ok := userID.(string); ok {
			uid = s
		}
	}
	if uid == "" {
		return Unauthorized(c, "未授权")
	}
	if err := h.svc.DeleteFile(c.Context(), projectID, path, uid); err != nil {
		if err.Error() == "file not found" {
			return NotFound(c, "文件不存在")
		}
		if strings.Contains(err.Error(), "permission denied") {
			return Forbidden(c, "无权访问此项目")
		}
		if strings.Contains(err.Error(), "项目不存在") {
			return NotFound(c, "项目不存在")
		}
		return InternalError(c, err.Error())
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

// FileTreeNode represents a node in the file tree
type FileTreeNode struct {
	Name     string         `json:"name"`
	Path     string         `json:"path"`
	Type     string         `json:"type"` // "file" or "directory"
	Children []*FileTreeNode `json:"children,omitempty"`
	Size     int64          `json:"size,omitempty"`
	Modified string         `json:"modified,omitempty"`
}

// GetFileTree returns a hierarchical file tree for a project
func (h *ProjectHandler) GetFileTree(c fiber.Ctx) error {
	projectID := c.Params("id")
	if projectID == "" {
		return c.JSON(&FileTreeNode{})
	}
	
	userID := c.Locals("uid")
	uid := ""
	if userID != nil {
		if s, ok := userID.(string); ok {
			uid = s
		}
	}
	if uid == "" {
		return Unauthorized(c, "未授权")
	}
	
	// Get all files for the project
	files, err := h.svc.ListFiles(c.Context(), projectID, uid)
	if err != nil {
		return ErrorResponse(c, 400, err.Error(), ErrCodeInternal)
	}
	
	// Build tree structure
	root := &FileTreeNode{
		Name:     "root",
		Path:     "",
		Type:     "directory",
		Children: []*FileTreeNode{},
	}
	
	for _, file := range files {
		path := file.Path
		if path == "" {
			continue
		}
		
		// Split path into components
		parts := strings.Split(path, "/")
		current := root
		
		// Traverse or create directory structure
		for i, part := range parts {
			if i == len(parts)-1 {
				// This is a file
				size := file.FileSize
				if size == 0 {
					size = int64(len(file.Content))
				}
				current.Children = append(current.Children, &FileTreeNode{
					Name:     part,
					Path:     path,
					Type:     "file",
					Size:     size,
					Modified: file.UpdatedAt,
				})
			} else {
				// This is a directory - find or create it
				found := false
				for _, child := range current.Children {
					if child.Name == part && child.Type == "directory" {
						current = child
						found = true
						break
					}
				}
				if !found {
					newDir := &FileTreeNode{
						Name:     part,
						Path:     strings.Join(parts[:i+1], "/"),
						Type:     "directory",
						Children: []*FileTreeNode{},
					}
					current.Children = append(current.Children, newDir)
					current = newDir
				}
			}
		}
	}
	
	return c.JSON(root)
}
