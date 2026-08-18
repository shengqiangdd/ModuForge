package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type RepoHandler struct {
	svc *service.RepoService
}

func NewRepoHandler(svc *service.RepoService) *RepoHandler {
	return &RepoHandler{svc: svc}
}

func (h *RepoHandler) Fetch(c fiber.Ctx) error {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.URL == "" {
		return c.Status(400).JSON(fiber.Map{"error": "url required"})
	}

	info, err := h.svc.FetchRepoInfo(c.Context(), req.URL)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(info)
}

func (h *RepoHandler) FetchFiles(c fiber.Ctx) error {
	var req struct {
		URL  string `json:"url"`
		Path string `json:"path"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.URL == "" {
		return c.Status(400).JSON(fiber.Map{"error": "url required"})
	}

	files, err := h.svc.FetchRepoFiles(c.Context(), req.URL, req.Path)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(files)
}

// FetchFileContent 拉取仓库中单个文件并解码为文本（智能参考用）。
func (h *RepoHandler) FetchFileContent(c fiber.Ctx) error {
	var req struct {
		URL  string `json:"url"`
		Path string `json:"path"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.URL == "" || req.Path == "" {
		return c.Status(400).JSON(fiber.Map{"error": "url and path required"})
	}

	content, err := h.svc.FetchFileContent(c.Context(), req.URL, req.Path)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(content)
}

// SmartSelect 从给定的仓库文件列表中智能挑选对 AI 改造最有价值的关键文件。
func (h *RepoHandler) SmartSelect(c fiber.Ctx) error {
	var req struct {
		Files []map[string]interface{} `json:"files"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	selected, err := h.svc.SmartSelectFiles(req.Files)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"selected": selected})
}
