package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type BuildScheduleHandler struct {
	svc *service.BuildScheduleService
}

func NewBuildScheduleHandler(svc *service.BuildScheduleService) *BuildScheduleHandler {
	return &BuildScheduleHandler{svc: svc}
}

func (h *BuildScheduleHandler) Create(c fiber.Ctx) error {
	projectID := c.Params("id")
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "missing user")
	}
	var req struct {
		CronExpr string `json:"cron_expr"`
		Target   string `json:"target"`
		Arch     string `json:"arch"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.CronExpr == "" {
		return c.Status(400).JSON(fiber.Map{"error": "cron_expr is required"})
	}
	if req.Target == "" {
		req.Target = "universal"
	}
	if req.Arch == "" {
		req.Arch = "arm64"
	}
	schedule, err := h.svc.Create(c.Context(), projectID, uid, req.CronExpr, req.Target, req.Arch)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(schedule)
}

func (h *BuildScheduleHandler) List(c fiber.Ctx) error {
	projectID := c.Params("id")
	list, err := h.svc.List(c.Context(), projectID)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"schedules": list})
}

func (h *BuildScheduleHandler) Toggle(c fiber.Ctx) error {
	id := c.Params("scheduleId")
	var req struct {
		Active bool `json:"active"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if err := h.svc.Toggle(c.Context(), id, req.Active); err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *BuildScheduleHandler) Delete(c fiber.Ctx) error {
	id := c.Params("scheduleId")
	if err := h.svc.Delete(c.Context(), id); err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}
