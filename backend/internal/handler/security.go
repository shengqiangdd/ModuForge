package handler

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type SecurityHandler struct {
	scanner *service.SecurityScanner
	db      *sql.DB
}

func NewSecurityHandler(scanner *service.SecurityScanner, db *sql.DB) *SecurityHandler {
	return &SecurityHandler{scanner: scanner, db: db}
}

type ScanRequest struct {
	Files map[string]string `json:"files"`
}

func (h *SecurityHandler) ScanFiles(c fiber.Ctx) error {
	var req ScanRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if len(req.Files) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "no files provided"})
	}

	result := h.scanner.ScanFiles(req.Files)
	return c.JSON(result)
}

func (h *SecurityHandler) ScanProject(c fiber.Ctx) error {
	projectID := c.Params("id")
	if projectID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "project id required"})
	}

	rows, err := h.db.Query(
		`SELECT path, content FROM project_files WHERE project_id=?`, projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to read project files"})
	}
	defer rows.Close()

	files := make(map[string]string)
	for rows.Next() {
		var path, content string
		if err := rows.Scan(&path, &content); err != nil {
			continue
		}
		files[path] = content
	}

	if len(files) == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "no files found in project"})
	}

	result := h.scanner.ScanFiles(files)
	return c.JSON(result)
}
