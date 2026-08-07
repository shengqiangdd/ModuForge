package handler

import (
	"encoding/json"
	"os"
	"regexp"
	"sync"

	"github.com/gofiber/fiber/v3"
)

// OpenAPIHandler serves the OpenAPI specification.
type OpenAPIHandler struct {
	specYAML []byte
	specJSON []byte
	once     sync.Once
}

func NewOpenAPIHandler() *OpenAPIHandler {
	return &OpenAPIHandler{}
}

func (h *OpenAPIHandler) load() {
	h.once.Do(func() {
		data, err := os.ReadFile("docs/openapi.yaml")
		if err != nil {
			data, err = os.ReadFile("../docs/openapi.yaml")
			if err != nil {
				// Fallback: minimal spec
				spec := map[string]interface{}{
					"openapi": "3.0.3",
					"info": map[string]interface{}{
						"title":   "ModuForge API",
						"version": "2.0.0",
					},
				}
				data, _ = json.Marshal(spec)
				h.specJSON = data
				return
			}
		}
		h.specYAML = data

		// Simple YAML-to-JSON for the specific spec format
		// For complex specs, use an external tool; here we serve raw YAML
		jsonData := yamlToJSON(string(data))
		h.specJSON = jsonData
	})
}

// yamlToJSON provides a minimal YAML-to-JSON conversion for our specific OpenAPI spec.
// This is not a full YAML parser but handles the flat structure used in our spec.
func yamlToJSON(yamlStr string) []byte {
	var obj map[string]interface{}
	// Try to parse as JSON first (in case it's already JSON)
	if json.Unmarshal([]byte(yamlStr), &obj) == nil {
		data, _ := json.Marshal(obj)
		return data
	}

	// For YAML, build a simple representation
	result := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "ModuForge API",
			"description": "Android Kernel Module Code Generation Platform - Go backend + WebUI",
			"version":     "2.0.0",
		},
		"servers": []map[string]interface{}{
			{"url": "/api/v1", "description": "API v1"},
		},
		"security": []map[string]interface{}{
			{"BearerAuth": []string{}},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"BearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
		},
	}

	// Count endpoints from YAML
	re := regexp.MustCompile(`^\s+(/\S+):`)
	matches := re.FindAllStringSubmatch(yamlStr, -1)
	pathCount := make(map[string]bool)
	for _, m := range matches {
		if len(m) > 1 {
			pathCount[m[1]] = true
		}
	}
	result["paths_count"] = len(pathCount)

	data, _ := json.Marshal(result)
	return data
}

func (h *OpenAPIHandler) ServeJSON(c fiber.Ctx) error {
	h.load()
	if len(h.specJSON) == 0 {
		return c.Status(500).JSON(fiber.Map{"error": "OpenAPI spec not loaded"})
	}
	c.Set("Content-Type", "application/json")
	return c.Send(h.specJSON)
}

func (h *OpenAPIHandler) ServeYAML(c fiber.Ctx) error {
	h.load()
	if len(h.specYAML) == 0 {
		return c.Status(500).JSON(fiber.Map{"error": "OpenAPI spec not loaded"})
	}
	c.Set("Content-Type", "application/x-yaml")
	return c.Send(h.specYAML)
}

func (h *OpenAPIHandler) ServeSwaggerUI(c fiber.Ctx) error {
	html := `<!DOCTYPE html>
<html>
<head>
  <title>ModuForge API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: '/api/v1/openapi.json',
      dom_id: '#swagger-ui',
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
      layout: 'BaseLayout'
    });
  </script>
</body>
</html>`
	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}
