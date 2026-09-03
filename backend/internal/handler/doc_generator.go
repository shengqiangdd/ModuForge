package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type DocGeneratorHandler struct {
	svc *service.DocGenerator
}

func NewDocGeneratorHandler() *DocGeneratorHandler {
	return &DocGeneratorHandler{svc: service.NewDocGenerator()}
}

type DocGenerateRequest struct {
	ProjectID     string   `json:"project_id"`
	ProjectName   string   `json:"project_name"`
	Description   string   `json:"description"`
	Author        string   `json:"author"`
	Version       string   `json:"version"`
	ModuleType    string   `json:"module_type"`
	License       string   `json:"license"`
	Tags          []string `json:"tags"`
	Dependencies  []string `json:"dependencies"`
	HasDaemon     bool     `json:"has_daemon"`
	HasWebUI      bool     `json:"has_webui"`
	HasService    bool     `json:"has_service"`
	MinAPI        int      `json:"min_api"`
	Architectures []string `json:"architectures"`
}

func (h *DocGeneratorHandler) Generate(c fiber.Ctx) error {
	var req DocGenerateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.ProjectName == "" {
		return c.Status(400).JSON(fiber.Map{"error": "project_name is required"})
	}

	opts := service.DocOptions{
		ProjectName:   req.ProjectName,
		Description:   req.Description,
		Author:        req.Author,
		Version:       req.Version,
		ModuleType:    req.ModuleType,
		License:       req.License,
		Tags:          req.Tags,
		Dependencies:  req.Dependencies,
		HasDaemon:     req.HasDaemon,
		HasWebUI:      req.HasWebUI,
		HasService:    req.HasService,
		MinAPI:        req.MinAPI,
		Architectures: req.Architectures,
	}

	if opts.ModuleType == "" {
		opts.ModuleType = "magisk"
	}
	if opts.MinAPI == 0 {
		opts.MinAPI = 26
	}

	docs := h.svc.GenerateAll(opts)
	return c.JSON(fiber.Map{"docs": docs})
}
