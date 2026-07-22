package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type ZipperHandler struct {
	zipper *service.ZipperService
}

func NewZipperHandler(zipper *service.ZipperService) *ZipperHandler {
	return &ZipperHandler{zipper: zipper}
}

type BuildZipRequest struct {
	ProjectID string            `json:"project_id"`
	Files     []service.ModuleFile `json:"files"`
}

func (h *ZipperHandler) BuildZip(c fiber.Ctx) error {
	var req BuildZipRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	zipPath, err := h.zipper.BuildModuleZip(c.Context(), req.ProjectID, req.Files)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Download(zipPath)
}

func (h *ZipperHandler) ListDownloads(c fiber.Ctx) error {
	zips := h.zipper.GetAvailableDownloads()
	return c.JSON(fiber.Map{"zips": zips})
}
