package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type BadgeHandler struct {
	svc *service.BadgeService
}

func NewBadgeHandler(svc *service.BadgeService) *BadgeHandler {
	return &BadgeHandler{svc: svc}
}

func (h *BadgeHandler) Definitions(c fiber.Ctx) error {
	defs := h.svc.GetDefinitions()
	return c.JSON(fiber.Map{"definitions": defs})
}

func (h *BadgeHandler) MyBadges(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "unauthorized")
	}
	badges, err := h.svc.GetUserBadges(userID)
	if err != nil {
		return InternalError(c, err.Error())
	}
	if badges == nil {
		badges = []service.UserBadge{}
	}
	return c.JSON(fiber.Map{"badges": badges})
}

func (h *BadgeHandler) UserBadges(c fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid user id"})
	}
	badges, err := h.svc.GetUserBadges(userID)
	if err != nil {
		return InternalError(c, err.Error())
	}
	if badges == nil {
		badges = []service.UserBadge{}
	}
	return c.JSON(fiber.Map{"badges": badges})
}
