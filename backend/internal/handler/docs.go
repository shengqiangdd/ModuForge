package handler

import (
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v3"
)

func RegisterDocs(api fiber.Router, docsDir string) {
	api.Get("/docs/openapi.yaml", func(c fiber.Ctx) error {
		path := filepath.Join(docsDir, "openapi.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "openapi.yaml not found"})
		}
		c.Set("Content-Type", "text/yaml; charset=utf-8")
		return c.Send(data)
	})
}

func DocsRedirect(c fiber.Ctx) error {
	html := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>ModuForge API 文档</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: '/api/v1/docs/openapi.yaml',
      dom_id: '#swagger-ui',
      deepLinking: true,
      docExpansion: 'list',
      defaultModelsExpandDepth: -1,
    });
  </script>
  <style>
    body { margin: 0; background: #1a1a2e; }
    .swagger-ui { color: #e0e0e0; }
    .swagger-ui .topbar { display: none; }
  </style>
</body>
</html>`
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Send([]byte(html))
}
