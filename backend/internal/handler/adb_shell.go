package handler

import (
	"github.com/gofiber/fiber/v3"
)

// ─── Shell & Exec ───

func (h *ADBHandler) RunShell(c fiber.Ctx) error {
	var req ShellRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Command == "" {
		return c.Status(400).JSON(fiber.Map{"error": "command required"})
	}
	result, err := h.svc.RunShell(c.Context(), req.Serial, req.Command)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) RunExec(c fiber.Ctx) error {
	var req ExecRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Command == "" {
		return c.Status(400).JSON(fiber.Map{"error": "command required"})
	}
	result, err := h.svc.RunExec(c.Context(), req.Serial, req.Command)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}
