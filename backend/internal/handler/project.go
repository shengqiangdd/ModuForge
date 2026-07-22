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
	userID := c.Locals("user_id")
	if userID == nil {
		userID = "demo"
	}
	projects, err := h.svc.List(c.Context(), userID.(string))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(projects)
}

func (h *ProjectHandler) Create(c fiber.Ctx) error {
	userID := c.Locals("user_id")
	if userID == nil {
		userID = "demo"
	}
	var req domain.CreateProjectInput
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	project, err := h.svc.Create(c.Context(), userID.(string), &req)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(project)
}

func (h *ProjectHandler) Get(c fiber.Ctx) error {
	id := c.Params("id")
	project, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(project)
}

func (h *ProjectHandler) Update(c fiber.Ctx) error {
	id := c.Params("id")
	var req domain.UpdateProjectInput
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	project, err := h.svc.Update(c.Context(), id, &req)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(project)
}

func (h *ProjectHandler) Delete(c fiber.Ctx) error {
	id := c.Params("id")
	if err := h.svc.Delete(c.Context(), id); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(204)
}

func (h *ProjectHandler) ListFiles(c fiber.Ctx) error {
	projectID := c.Params("id")
	files, err := h.svc.ListFiles(c.Context(), projectID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(files)
}

func (h *ProjectHandler) GetFile(c fiber.Ctx) error {
	projectID := c.Params("id")
	path := c.Params("*")
	file, err := h.svc.GetFile(c.Context(), projectID, path)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
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
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	file, err := h.svc.SaveFile(c.Context(), projectID, path, body.Content)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(file)
}

func (h *ProjectHandler) ListTemplates(c fiber.Ctx) error {
	templates := []map[string]string{
		{"name": "Magisk 基础模块", "type": "magisk", "desc": "标准 Magisk 模块骨架"},
		{"name": "KSU WebUI 模块", "type": "ksu", "desc": "带 WebUI 的 KernelSU 模块"},
		{"name": "APatch 动作模块", "type": "apatch", "desc": "APatch action.sh 模块"},
		{"name": "Hybrid 通用模块", "type": "hybrid", "desc": "兼容 Magisk + KSU"},
	}
	return c.JSON(templates)
}
