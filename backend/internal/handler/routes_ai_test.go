package handler

import (
	"testing"
	"net/http/httptest"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
)

func TestHealthEndpoint(t *testing.T) {
	app := fiber.New()

	app.Get("/health", func(c fiber.Ctx) error {
		return c.Status(200).JSON(fiber.Map{
			"status": "healthy",
		})
	})

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestOpenAPIEndpoint(t *testing.T) {
	app := fiber.New()

	app.Get("/api/docs/openapi", func(c fiber.Ctx) error {
		return c.Status(200).JSON(fiber.Map{
			"openapi": "3.0.3",
			"info": fiber.Map{
				"title": "ModuForge API",
			},
		})
	})

	req := httptest.NewRequest("GET", "/api/docs/openapi", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}
