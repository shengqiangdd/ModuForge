package handler

import (
	"bufio"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
	"github.com/valyala/fasthttp"
)

type MirrorHandler struct {
	adb *service.ADBService
}

func NewMirrorHandler(adb *service.ADBService) *MirrorHandler {
	return &MirrorHandler{adb: adb}
}

// GET /api/v1/adb/mirror?serial=XXX&fps=3
func (h *MirrorHandler) Mirror(c fiber.Ctx) error {
	serial := c.Query("serial")
	if serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}
	fps, _ := strconv.Atoi(c.Query("fps", "3"))
	if fps < 1 {
		fps = 1
	}
	if fps > 10 {
		fps = 10
	}

	devices, err := h.adb.ListDevices(c.Context())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	found := false
	for _, d := range devices {
		if d.Serial == serial && d.State == "device" {
			found = true
			break
		}
	}
	if !found {
		return c.Status(404).JSON(fiber.Map{"error": "device not found or not online"})
	}

	c.Set("Content-Type", "multipart/x-mixed-replace; boundary=MJPEG_BOUNDARY")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Access-Control-Allow-Origin", "*")

	fctx, ok := c.Context().(*fasthttp.RequestCtx)
	if !ok {
		return c.Status(500).JSON(fiber.Map{"error": "streaming not supported"})
	}

	fctx.SetBodyStreamWriter(func(w *bufio.Writer) {
		h.adb.StreamScreen(c.Context(), serial, fps, w)
	})

	return nil
}
