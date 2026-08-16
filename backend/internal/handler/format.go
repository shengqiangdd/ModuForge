package handler

import (
	"database/sql"
	"os/exec"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type FormatHandler struct {
	db *sql.DB
	fr *service.FileContentRepo // S3-first content access (optional)
}

func NewFormatHandler(db *sql.DB) *FormatHandler {
	return &FormatHandler{db: db}
}

// SetFileContentRepo injects the S3-first file content repository.
func (h *FormatHandler) SetFileContentRepo(fr *service.FileContentRepo) {
	h.fr = fr
}

func (h *FormatHandler) formatContent(path, content string) (string, string, bool) {
	// Returns (formatted_content, error_msg, ok)
	ext := getExt(path)
	switch ext {
	case ".json":
		return formatWithCmd("jq", []string{"."}, content)
	case ".go":
		return formatWithCmd("gofmt", nil, content)
	case ".rs":
		if _, err := exec.LookPath("rustfmt"); err == nil {
			return formatWithCmd("rustfmt", nil, content)
		}
		return content, "rustfmt not installed", false
	case ".py":
		if _, err := exec.LookPath("black"); err == nil {
			return formatWithCmd("black", []string{"-"}, content)
		}
		return content, "black not installed", false
	case ".js", ".ts", ".jsx", ".tsx":
		// Try deno fmt first (usually available in containers)
		if _, err := exec.LookPath("deno"); err == nil {
			return formatWithCmd("deno", []string{"fmt", "--stdin-filepath", path}, content)
		}
		// Fallback: basic cleanup
		return basicFormatJS(content), "", true
	case ".css":
		if _, err := exec.LookPath("deno"); err == nil {
			return formatWithCmd("deno", []string{"fmt", "--stdin-filepath", path}, content)
		}
		return content, "", true
	default:
		return content, "", true
	}
}

func formatWithCmd(cmd string, args []string, stdin string) (string, string, bool) {
	c := exec.Command(cmd, args...)
	c.Stdin = strings.NewReader(stdin)
	out, err := c.CombinedOutput()
	if err != nil {
		return stdin, string(out), false
	}
	result := strings.TrimRight(string(out), "\n")
	if result == "" {
		return stdin, "empty output", false
	}
	return result, "", true
}

func basicFormatJS(content string) string {
	// Very basic JS formatting: normalize line endings, trim trailing whitespace
	lines := strings.Split(content, "\n")
	var out []string
	for _, line := range lines {
		out = append(out, strings.TrimRight(line, " \t"))
	}
	return strings.Join(out, "\n")
}

func (h *FormatHandler) FormatProject(c fiber.Ctx) error {
	projectID := c.Params("id")
	if projectID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "project id required"})
	}

	fileMap, err := h.fr.ReadAllContent(c.Context(), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to load project files"})
	}

	type FormatResult struct {
		File   string `json:"file"`
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}
	var results []FormatResult
	success, failed, skipped := 0, 0, 0

	for path, content := range fileMap {
		ext := getExt(path)
		// Skip unsupported extensions
		if ext != ".json" && ext != ".go" && ext != ".rs" && ext != ".py" &&
			ext != ".js" && ext != ".ts" && ext != ".jsx" && ext != ".tsx" && ext != ".css" {
			skipped++
			results = append(results, FormatResult{File: path, Status: "skipped"})
			continue
		}

		formatted, errMsg, ok := h.formatContent(path, content)
		if !ok {
			failed++
			results = append(results, FormatResult{File: path, Status: "failed", Error: errMsg})
			continue
		}

		if formatted != content {
			if err := h.fr.Write(c.Context(), projectID, path, formatted); err != nil {
				failed++
				results = append(results, FormatResult{File: path, Status: "failed", Error: err.Error()})
				continue
			}
		}
		success++
		results = append(results, FormatResult{File: path, Status: "ok"})
	}

	return c.JSON(fiber.Map{
		"results": results,
		"success": success,
		"failed":  failed,
		"skipped": skipped,
	})
}

func (h *FormatHandler) PreviewFormat(c fiber.Ctx) error {
	projectID := c.Params("id")
	if projectID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "project id required"})
	}

	fileMap, err := h.fr.ReadAllContent(c.Context(), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to load project files"})
	}

	type PreviewResult struct {
		File        string `json:"file"`
		NeedsFormat bool   `json:"needs_format"`
		Error       string `json:"error,omitempty"`
	}
	var results []PreviewResult

	for path, content := range fileMap {
		ext := getExt(path)
		if ext != ".json" && ext != ".go" && ext != ".rs" && ext != ".py" &&
			ext != ".js" && ext != ".ts" && ext != ".jsx" && ext != ".tsx" && ext != ".css" {
			results = append(results, PreviewResult{File: path, NeedsFormat: false})
			continue
		}

		formatted, errMsg, ok := h.formatContent(path, content)
		if !ok {
			results = append(results, PreviewResult{File: path, NeedsFormat: false, Error: errMsg})
			continue
		}
		results = append(results, PreviewResult{File: path, NeedsFormat: formatted != content})
	}

	return c.JSON(fiber.Map{"results": results})
}

func getExt(path string) string {
	if i := strings.LastIndex(path, "."); i >= 0 {
		return strings.ToLower(path[i:])
	}
	return ""
}
