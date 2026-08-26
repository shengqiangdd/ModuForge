package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
)

// GetGitStatus returns the git working tree status.
func (h *AgentHandler) GetGitStatus(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	status, err := h.runner.GetGitStatus()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(status)
}

// GetGitLog returns recent git commits.
func (h *AgentHandler) GetGitLog(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	n, _ := strconv.Atoi(c.Query("n", "20"))
	logs, err := h.runner.GetGitLog(n)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(logs)
}

// GitCommit creates a new git commit.
func (h *AgentHandler) GitCommit(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	var req struct {
		Message string   `json:"message"`
		Files   []string `json:"files"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	hash, err := h.runner.GitCommit(req.Message, req.Files)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"hash": hash})
}

// GitRollback reverts to a specific commit.
func (h *AgentHandler) GitRollback(c fiber.Ctx) error {
	if h.runner == nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent not ready"})
	}
	var req struct {
		Commit string `json:"commit"`
		Hard   bool   `json:"hard"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	err := h.runner.GitRollback(req.Commit, req.Hard)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}
