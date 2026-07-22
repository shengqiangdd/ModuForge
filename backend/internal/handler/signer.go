package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type SignerHandler struct {
	signer *service.SignerService
}

func NewSignerHandler(signer *service.SignerService) *SignerHandler {
	return &SignerHandler{signer: signer}
}

type SignRequest struct {
	ZipPath string `json:"zip_path"`
}

func (h *SignerHandler) Sign(c fiber.Ctx) error {
	var req SignRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	info, err := h.signer.SignModule(req.ZipPath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(info)
}

func (h *SignerHandler) Verify(c fiber.Ctx) error {
	var req struct {
		ZipPath      string `json:"zip_path"`
		ExpectedHash string `json:"expected_hash"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	valid, err := h.signer.VerifyModule(req.ZipPath, req.ExpectedHash)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"valid": valid, "zip_path": req.ZipPath})
}
