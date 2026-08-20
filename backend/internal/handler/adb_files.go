package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v3"
)

// ─── File Management ───

func (h *ADBHandler) ListFiles(c fiber.Ctx) error {
	serial := c.Query("serial")
	path := c.Query("path", "/sdcard/")
	if err := validatePath(path); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	files, err := h.svc.ListFiles(c.Context(), serial, path)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"files": files, "path": path})
}

func (h *ADBHandler) PushFile(c fiber.Ctx) error {
	var req PushRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.LocalPath == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and local_path required"})
	}
	// Security: localPath must live in an allowed directory, otherwise any
	// authenticated user could push arbitrary server files to a device.
	if err := validateLocalPath(req.LocalPath); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if req.RemotePath != "" {
		if err := validatePath(req.RemotePath); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
	}
	result, err := h.svc.PushFile(c.Context(), req.Serial, req.LocalPath, req.RemotePath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) PullFile(c fiber.Ctx) error {
	var req PullRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.RemotePath == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and remote_path required"})
	}
	if err := validatePath(req.RemotePath); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	// Save to temp dir
	localPath := filepath.Join(os.TempDir(), fmt.Sprintf("adb_pull_%d_%s", time.Now().UnixMilli(), filepath.Base(req.RemotePath)))
	result, err := h.svc.PullFile(c.Context(), req.Serial, req.RemotePath, localPath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result, "local_path": localPath})
}

func (h *ADBHandler) DeleteFile(c fiber.Ctx) error {
	var req DeleteRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.RemotePath == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and remote_path required"})
	}
	if err := validatePath(req.RemotePath); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	result, err := h.svc.DeleteFile(c.Context(), req.Serial, req.RemotePath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) MakeDir(c fiber.Ctx) error {
	var req MkdirRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.RemotePath == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and remote_path required"})
	}
	if err := validatePath(req.RemotePath); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	result, err := h.svc.MakeDir(c.Context(), req.Serial, req.RemotePath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) RenameFile(c fiber.Ctx) error {
	var req RenameRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.Old == "" || req.New == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial, old_path and new_path required"})
	}
	if err := validatePath(req.Old); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "old_path: " + err.Error()})
	}
	if err := validatePath(req.New); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "new_path: " + err.Error()})
	}
	result, err := h.svc.RenameFile(c.Context(), req.Serial, req.Old, req.New)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) ReadFile(c fiber.Ctx) error {
	var req FileReadRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.RemotePath == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and remote_path required"})
	}
	if err := validatePath(req.RemotePath); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	content, err := h.svc.ReadFile(c.Context(), req.Serial, req.RemotePath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"content": content, "path": req.RemotePath})
}

func (h *ADBHandler) WriteFile(c fiber.Ctx) error {
	var req FileWriteRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.RemotePath == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and remote_path required"})
	}
	if err := validatePath(req.RemotePath); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	result, err := h.svc.WriteFile(c.Context(), req.Serial, req.RemotePath, req.Content)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) CopyFile(c fiber.Ctx) error {
	var req FileCopyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.Src == "" || req.Dst == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial, src and dst required"})
	}
	if err := validatePath(req.Src); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "src: " + err.Error()})
	}
	if err := validatePath(req.Dst); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "dst: " + err.Error()})
	}
	result, err := h.svc.CopyFile(c.Context(), req.Serial, req.Src, req.Dst)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) GetFileInfo(c fiber.Ctx) error {
	serial := c.Query("serial")
	path := c.Query("path")
	if serial == "" || path == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and path required"})
	}
	if err := validatePath(path); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	info, err := h.svc.GetFileInfo(c.Context(), serial, path)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(info)
}

func (h *ADBHandler) UploadFile(c fiber.Ctx) error {
	serial := c.FormValue("serial")
	remotePath := c.FormValue("remote_path")
	if serial == "" || remotePath == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and remote_path required"})
	}
	if err := validatePath(remotePath); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "file required"})
	}
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("adb_upload_%d_%s", time.Now().UnixMilli(), filepath.Base(file.Filename)))
	if err := c.SaveFile(file, tmpFile); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "save file failed: " + err.Error()})
	}
	result, err := h.svc.PushFile(c.Context(), serial, tmpFile, remotePath)
	os.Remove(tmpFile)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) DownloadFile(c fiber.Ctx) error {
	serial := c.Query("serial")
	path := c.Query("path")
	if serial == "" || path == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and path required"})
	}
	if err := validatePath(path); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	localPath := filepath.Join(os.TempDir(), fmt.Sprintf("adb_download_%d_%s", time.Now().UnixMilli(), filepath.Base(path)))
	_, err := h.svc.PullFile(c.Context(), serial, path, localPath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer os.Remove(localPath)
	c.Response().Header.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(path)))
	return c.SendFile(localPath)
}
