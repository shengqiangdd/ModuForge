package handler

import (
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type ScreenshotHandler struct {
	adb *service.ADBService
}

func NewScreenshotHandler(adb *service.ADBService) *ScreenshotHandler {
	return &ScreenshotHandler{adb: adb}
}

func (h *ScreenshotHandler) Screenshot(c fiber.Ctx) error {
	serial := c.Query("serial")
	if serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}

	filename := "screenshot_" + time.Now().Format("20060102_150405") + ".png"
	localPath := "data/screenshots/" + filename

	if err := os.MkdirAll("data/screenshots", 0755); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	path, err := h.adb.Screenshot(c.Context(), serial, localPath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"path":     path,
		"filename": filename,
	})
}

func (h *ScreenshotHandler) StreamScreenshots(c fiber.Ctx) error {
	serial := c.Query("serial")
	if serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	for i := 0; i < 10; i++ {
		filename := "screenshot_" + time.Now().Format("20060102_150405") + ".png"
		localPath := "data/screenshots/" + filename

		_, err := h.adb.Screenshot(c.Context(), serial, localPath)
		if err != nil {
			line := fmt.Sprintf("data: {\"error\":\"%s\"}\n\n", err.Error())
			if _, wErr := c.Write([]byte(line)); wErr != nil {
				return wErr
			}
			break
		}

		line := fmt.Sprintf("data: {\"filename\":\"%s\",\"index\":%d}\n\n", filename, i)
		if _, err := c.Write([]byte(line)); err != nil {
			return err
		}

		time.Sleep(2 * time.Second)
	}

	if _, err := c.Write([]byte("data: {\"done\":true}\n\n")); err != nil {
		return err
	}

	return nil
}
