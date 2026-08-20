package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
)

// ─── Screen Control ───

func (h *ADBHandler) ScreenshotBase64(c fiber.Ctx) error {
	serial := c.Query("serial")
	if serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}
	encoded, err := h.svc.ScreenshotBase64(c.Context(), serial)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"image_base64": encoded, "format": "png"})
}

func (h *ADBHandler) GetScreenSize(c fiber.Ctx) error {
	serial := c.Query("serial")
	if serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}
	w, height, err := h.svc.GetScreenSize(c.Context(), serial)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"width": w, "height": height})
}

func (h *ADBHandler) TapScreen(c fiber.Ctx) error {
	var req TapRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}
	if err := h.svc.TapScreen(c.Context(), req.Serial, req.X, req.Y); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok", "x": req.X, "y": req.Y})
}

func (h *ADBHandler) SwipeScreen(c fiber.Ctx) error {
	var req SwipeRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}
	if err := h.svc.SwipeScreen(c.Context(), req.Serial, req.X1, req.Y1, req.X2, req.Y2, req.Duration); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *ADBHandler) InputText(c fiber.Ctx) error {
	var req InputTextRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}
	if err := h.svc.InputText(c.Context(), req.Serial, req.Text); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *ADBHandler) KeyEvent(c fiber.Ctx) error {
	var req KeyEventRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.Key == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and key required"})
	}
	if err := h.svc.KeyEvent(c.Context(), req.Serial, req.Key); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok", "key": req.Key})
}

// ─── Screen Record ───

func (h *ADBHandler) ScreenRecord(c fiber.Ctx) error {
	var req struct {
		Serial   string `json:"serial"`
		Action   string `json:"action"`
		Duration string `json:"duration,omitempty"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}
	if req.Action == "" {
		req.Action = "record"
	}
	// Validate duration is numeric only (prevents command injection)
	if req.Duration != "" {
		if _, err := strconv.Atoi(req.Duration); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "duration must be a number (seconds)"})
		}
	}
	switch req.Action {
	case "start":
		remotePath := "/data/local/tmp/record.mp4"
		cmd := fmt.Sprintf("screenrecord %s &", remotePath)
		if req.Duration != "" {
			cmd = fmt.Sprintf("screenrecord --time-limit %s %s &", req.Duration, remotePath)
		}
		if _, err := h.svc.RunShell(c.Context(), req.Serial, "rm -f /data/local/tmp/record.mp4 2>/dev/null"); err != nil {
			// ignore cleanup error
		}
		if _, err := h.svc.RunShell(c.Context(), req.Serial, cmd); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("start recording failed: %v", err)})
		}
		return c.JSON(fiber.Map{"status": "recording", "path": remotePath})
	case "stop":
		h.svc.RunShell(c.Context(), req.Serial, "pkill -2 screenrecord 2>/dev/null")
		time.Sleep(500 * time.Millisecond)
		localPath := filepath.Join(os.TempDir(), fmt.Sprintf("record_%d.mp4", time.Now().UnixMilli()))
		_, err := h.svc.PullFile(c.Context(), req.Serial, "/data/local/tmp/record.mp4", localPath)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("pull recording failed: %v", err)})
		}
		h.svc.RunShell(c.Context(), req.Serial, "rm -f /data/local/tmp/record.mp4 2>/dev/null")
		return c.JSON(fiber.Map{"status": "ok", "path": localPath})
	default:
		// record with duration
		localPath := filepath.Join(os.TempDir(), fmt.Sprintf("record_%d.mp4", time.Now().UnixMilli()))
		path, err := h.svc.ScreenRecord(c.Context(), req.Serial, localPath, req.Duration)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok", "path": path})
	}
}

// ─── Screenshot ───

func (h *ADBHandler) Screenshot(c fiber.Ctx) error {
	serial := c.Query("serial")
	if serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}
	localPath := filepath.Join(os.TempDir(), fmt.Sprintf("screenshot_%d.png", time.Now().UnixMilli()))
	path, err := h.svc.Screenshot(c.Context(), serial, localPath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"path": path})
}
