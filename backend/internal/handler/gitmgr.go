package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type GitHandler struct {
	svc *service.GitManagerService
}

func NewGitHandler(svc *service.GitManagerService) *GitHandler {
	return &GitHandler{svc: svc}
}

type CommitRequest struct {
	ProjectID string `json:"project_id"`
	Message   string `json:"message"`
}

type CheckoutRequest struct {
	ProjectID string `json:"project_id"`
	Hash      string `json:"hash"`
}

func (h *GitHandler) Commit(c fiber.Ctx) error {
	var req CommitRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	info, err := h.svc.AddAndCommit(c.Context(), req.ProjectID, req.Message)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(info)
}

func (h *GitHandler) ListCommits(c fiber.Ctx) error {
	projectID := c.Query("project_id", "")
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	commits, err := h.svc.ListCommits(c.Context(), projectID, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(commits)
}

func (h *GitHandler) GetDiff(c fiber.Ctx) error {
	projectID := c.Query("project_id", "")
	hash := c.Query("hash", "")
	diff, err := h.svc.GetDiff(c.Context(), projectID, hash)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"diff": diff})
}

func (h *GitHandler) Checkout(c fiber.Ctx) error {
	var req CheckoutRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if err := h.svc.CheckoutVersion(c.Context(), req.ProjectID, req.Hash); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "checked out"})
}

func (h *GitHandler) GetCurrentHash(c fiber.Ctx) error {
	projectID := c.Query("project_id", "")
	info, err := h.svc.GetCurrentHash(c.Context(), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(info)
}
