package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type ADBHandler struct {
	svc *service.ADBService
}

func NewADBHandler(svc *service.ADBService) *ADBHandler {
	return &ADBHandler{svc: svc}
}

type PushRequest struct {
	Serial     string `json:"serial"`
	LocalPath  string `json:"local_path"`
	RemotePath string `json:"remote_path"`
}

type InstallRequest struct {
	Serial string `json:"serial"`
	ZipPath string `json:"zip_path"`
}

type ShellRequest struct {
	Serial  string `json:"serial"`
	Command string `json:"command"`
}

type RebootRequest struct {
	Serial string `json:"serial"`
}

func (h *ADBHandler) ListDevices(c fiber.Ctx) error {
	devices, err := h.svc.ListDevices(c.Context())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(devices)
}

func (h *ADBHandler) PushFile(c fiber.Ctx) error {
	var req PushRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	result, err := h.svc.PushFile(c.Context(), req.Serial, req.LocalPath, req.RemotePath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) InstallModule(c fiber.Ctx) error {
	var req InstallRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	result, err := h.svc.InstallModule(c.Context(), req.Serial, req.ZipPath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) RunShell(c fiber.Ctx) error {
	var req ShellRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	result, err := h.svc.RunShell(c.Context(), req.Serial, req.Command)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) RebootDevice(c fiber.Ctx) error {
	var req RebootRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if err := h.svc.RebootDevice(c.Context(), req.Serial); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "rebooting"})
}

func (h *ADBHandler) CheckADB(c fiber.Ctx) error {
	available := h.svc.CheckADBAvailable(c.Context())
	return c.JSON(fiber.Map{"available": available})
}
