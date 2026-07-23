package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/domain"
	"github.com/moduforge/backend/internal/service"
)

type ProjectHandler struct {
	svc *service.ProjectService
}

func NewProjectHandler(svc *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{svc: svc}
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
	project, err := h.svc.Get(c.Context(), id)
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
	project, err := h.svc.Update(c.Context(), id, &req)
	if err != nil {
		return ErrorResponse(c, 400, err.Error(), ErrCodeConflict)
	}
	return c.JSON(project)
}

func (h *ProjectHandler) Delete(c fiber.Ctx) error {
	id := c.Params("id")
	if err := h.svc.Delete(c.Context(), id); err != nil {
		return ErrorResponse(c, 400, err.Error(), ErrCodeInternal)
	}
	return c.SendStatus(204)
}

func (h *ProjectHandler) ListFiles(c fiber.Ctx) error {
	projectID := c.Params("id")
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
	return c.JSON(file)
}

func (h *ProjectHandler) ListTemplates(c fiber.Ctx) error {
	templates := []map[string]string{
		{"name": "通用模块模板", "type": "universal", "desc": "兼容 Magisk / KernelSU / APatch 的通用模块"},
	}
	return c.JSON(templates)
}
