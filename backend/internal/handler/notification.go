package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type NotificationHandler struct {
	svc *service.NotificationService
}

func NewNotificationHandler(svc *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

func (h *NotificationHandler) List(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "missing user")
	}
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	list, err := h.svc.List(userID, limit, offset)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"notifications": list})
}

func (h *NotificationHandler) UnreadCount(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "missing user")
	}
	count, err := h.svc.UnreadCount(userID)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"count": count})
}

func (h *NotificationHandler) MarkRead(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "missing user")
	}
	notifID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return BadRequest(c, "invalid notification id")
	}
	if err := h.svc.MarkRead(userID, notifID); err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *NotificationHandler) MarkAllRead(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "missing user")
	}
	if err := h.svc.MarkAllRead(userID); err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *NotificationHandler) Delete(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "missing user")
	}
	notifID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return BadRequest(c, "invalid notification id")
	}
	if err := h.svc.Delete(userID, notifID); err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}
