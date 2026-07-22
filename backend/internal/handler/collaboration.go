package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type CollaborationHandler struct {
	collab *service.CollaborationService
}

func NewCollaborationHandler(collab *service.CollaborationService) *CollaborationHandler {
	return &CollaborationHandler{collab: collab}
}

func (h *CollaborationHandler) AddCollaborator(c fiber.Ctx) error {
	projectID := c.Params("id")
	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Role == "" {
		req.Role = "editor"
	}

	collab, err := h.collab.AddCollaborator(c.Context(), projectID, req.UserID, req.Role)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(collab)
}

func (h *CollaborationHandler) ListCollaborators(c fiber.Ctx) error {
	projectID := c.Params("id")
	list, err := h.collab.ListCollaborators(c.Context(), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"collaborators": list})
}

func (h *CollaborationHandler) RemoveCollaborator(c fiber.Ctx) error {
	projectID := c.Params("id")
	userID := c.Params("userId")
	if err := h.collab.RemoveCollaborator(c.Context(), projectID, userID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *CollaborationHandler) AddComment(c fiber.Ctx) error {
	projectID := c.Params("id")
	var req struct {
		FilePath   string `json:"file_path"`
		Content    string `json:"content"`
		LineNumber int    `json:"line_number"`
		UserID     string `json:"user_id"`
		Username   string `json:"username"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	comment, err := h.collab.AddComment(c.Context(), projectID, req.UserID, req.Username, req.FilePath, req.Content, req.LineNumber)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(comment)
}

func (h *CollaborationHandler) ListComments(c fiber.Ctx) error {
	projectID := c.Params("id")
	filePath := c.Query("file_path")
	list, err := h.collab.ListComments(c.Context(), projectID, filePath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"comments": list})
}

func (h *CollaborationHandler) ResolveComment(c fiber.Ctx) error {
	commentID := c.Params("commentId")
	if err := h.collab.ResolveComment(c.Context(), commentID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *CollaborationHandler) UpsertEditSession(c fiber.Ctx) error {
	projectID := c.Params("id")
	var session service.EditSession
	if err := c.Bind().JSON(&session); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	session.ProjectID = projectID

	if err := h.collab.UpsertEditSession(c.Context(), &session); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(session)
}

func (h *CollaborationHandler) ListEditSessions(c fiber.Ctx) error {
	projectID := c.Params("id")
	sessions, err := h.collab.ListEditSessions(c.Context(), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"sessions": sessions})
}

func (h *CollaborationHandler) RemoveEditSession(c fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if err := h.collab.RemoveEditSession(c.Context(), sessionID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}
