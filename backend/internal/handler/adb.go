package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

func validatePath(p string) error {
	if !strings.HasPrefix(p, "/") {
		return fmt.Errorf("path must be absolute")
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("path traversal not allowed")
	}
	return nil
}

// validateLocalPath ensures a server-side local path is inside an allowed
// directory (temp dir for uploads, or STORAGE_PATH for build artifacts).
// Without this, any authenticated user could point PushFile at arbitrary
// server files (DB, secrets, other users' projects) and leak them to a device.
func validateLocalPath(p string) error {
	if p == "" {
		return fmt.Errorf("local path is required")
	}
	clean := filepath.Clean(p)

	allowed := []string{filepath.Clean(os.TempDir())}
	storagePath := os.Getenv("STORAGE_PATH")
	if storagePath == "" {
		storagePath = "/data/storage"
	}
	allowed = append(allowed, filepath.Clean(storagePath))

	for _, root := range allowed {
		if clean == root || strings.HasPrefix(clean, root+string(os.PathSeparator)) {
			return nil
		}
	}
	return fmt.Errorf("local path %s is outside allowed directories (temp or storage)", p)
}

type ADBHandler struct {
	svc *service.ADBService
}

func NewADBHandler(svc *service.ADBService) *ADBHandler {
	return &ADBHandler{svc: svc}
}

// ─── Request Types ───

type ConnectRequest struct {
	Address string `json:"address"`
}

type DisconnectRequest struct {
	Address string `json:"address"`
}

type PairRequest struct {
	Address string `json:"address"`
	Code    string `json:"code"`
}

type DiagnoseRequest struct {
	Address string `json:"address"`
}

type PushRequest struct {
	Serial     string `json:"serial"`
	LocalPath  string `json:"local_path"`
	RemotePath string `json:"remote_path"`
}

type PullRequest struct {
	Serial     string `json:"serial"`
	RemotePath string `json:"remote_path"`
	LocalPath  string `json:"local_path"`
}

type DeleteRequest struct {
	Serial     string `json:"serial"`
	RemotePath string `json:"remote_path"`
}

type MkdirRequest struct {
	Serial     string `json:"serial"`
	RemotePath string `json:"remote_path"`
}

type RenameRequest struct {
	Serial string `json:"serial"`
	Old    string `json:"old_path"`
	New    string `json:"new_path"`
}

type ShellRequest struct {
	Serial  string `json:"serial"`
	Command string `json:"command"`
}

type ExecRequest struct {
	Serial  string `json:"serial"`
	Command string `json:"command"`
}

type RebootRequest struct {
	Serial string `json:"serial"`
	Mode   string `json:"mode"`
}

type InstallRequest struct {
	Serial  string `json:"serial"`
	ZipPath string `json:"zip_path"`
}

type InstallURLRequest struct {
	Serial string `json:"serial"`
	URL    string `json:"url"`
}

type FileReadRequest struct {
	Serial     string `json:"serial"`
	RemotePath string `json:"remote_path"`
}

type FileWriteRequest struct {
	Serial     string `json:"serial"`
	RemotePath string `json:"remote_path"`
	Content    string `json:"content"`
}

type FileCopyRequest struct {
	Serial string `json:"serial"`
	Src    string `json:"src"`
	Dst    string `json:"dst"`
}

type TapRequest struct {
	Serial string `json:"serial"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
}

type SwipeRequest struct {
	Serial   string `json:"serial"`
	X1       int    `json:"x1"`
	Y1       int    `json:"y1"`
	X2       int    `json:"x2"`
	Y2       int    `json:"y2"`
	Duration int    `json:"duration"`
}

type InputTextRequest struct {
	Serial string `json:"serial"`
	Text   string `json:"text"`
}

type KeyEventRequest struct {
	Serial string `json:"serial"`
	Key    string `json:"key"`
}

type InstallAppRequest struct {
	Serial string `json:"serial"`
	APK    string `json:"apk_path"`
}

type UninstallAppRequest struct {
	Serial   string `json:"serial"`
	Package  string `json:"package"`
	KeepData bool   `json:"keep_data"`
}

type AppActionRequest struct {
	Serial  string `json:"serial"`
	Package string `json:"package"`
}

type ToggleModuleRequest struct {
	Serial string `json:"serial"`
	Enable bool   `json:"enable"`
}

type ToggleAppRequest struct {
	Serial  string `json:"serial"`
	Package string `json:"package"`
	Enable  bool   `json:"enable"`
}

type PropRequest struct {
	Serial string `json:"serial"`
	Prop   string `json:"prop"`
	Value  string `json:"value,omitempty"`
}

type BackupModuleRequest struct {
	Serial     string `json:"serial"`
	ModuleName string `json:"module_name"`
	LocalPath  string `json:"local_path,omitempty"`
}

type RestoreModuleRequest struct {
	Serial    string `json:"serial"`
	LocalPath string `json:"local_path"`
}

type CheckUpdateRequest struct {
	Serial     string `json:"serial"`
	ModuleName string `json:"module_name"`
}

type ExportModuleRequest struct {
	Serial     string `json:"serial"`
	ModuleName string `json:"module_name"`
}

// ─── Enhanced Module Push Request Types ───

type PushModuleFolderRequest struct {
	Serial     string `json:"serial"`
	LocalDir   string `json:"local_dir"`
	ModuleName string `json:"module_name"`
	Install    bool   `json:"install"`
	Reboot     bool   `json:"reboot"`
}

type PushBuildModuleRequest struct {
	Serial     string `json:"serial"`
	BuildID    string `json:"build_id"`
	ModuleName string `json:"module_name,omitempty"`
	Install    bool   `json:"install"`
	Reboot     bool   `json:"reboot"`
}

type PushBuildZipRequest struct {
	Serial     string `json:"serial"`
	ZipPath    string `json:"zip_path"`
	ModuleName string `json:"module_name,omitempty"`
	Install    bool   `json:"install"`
	Reboot     bool   `json:"reboot"`
}

type RootPermissionRequest struct {
	Serial      string `json:"serial"`
	PackageName string `json:"package_name"`
	Grant       bool   `json:"grant"`
}

// ─── ADB Server ───

func (h *ADBHandler) CheckADB(c fiber.Ctx) error {
	available := service.ADBAvailable()
	result := fiber.Map{"available": available}
	if available {
		result["adb_path"] = h.svc.ADBPath()
		result["version"] = service.ADBVersion()
		result["install_hint"] = ""
	} else {
		result["adb_path"] = ""
		result["version"] = ""
		result["error"] = "adb not found in PATH"
		result["install_hint"] = service.ADBInstallHint()
	}
	return c.JSON(result)
}

func (h *ADBHandler) StartServer(c fiber.Ctx) error {
	out, err := h.svc.StartServer(c.Context())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": out})
}

func (h *ADBHandler) KillServer(c fiber.Ctx) error {
	out, err := h.svc.KillServer(c.Context())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": out})
}

func (h *ADBHandler) GetServerStatus(c fiber.Ctx) error {
	status, err := h.svc.GetServerStatus(c.Context())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(status)
}

// ─── Device Management ───

func (h *ADBHandler) ListDevices(c fiber.Ctx) error {
	uid, _ := c.Locals("user_id").(string)
	devices, err := h.svc.ListDevices(c.Context(), uid)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"devices": devices})
}

func (h *ADBHandler) GetDeviceInfo(c fiber.Ctx) error {
	serial := c.Query("serial")
	if serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}
	info, err := h.svc.GetDeviceInfo(c.Context(), serial)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(info)
}

func (h *ADBHandler) ConnectDevice(c fiber.Ctx) error {
	var req ConnectRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Address == "" {
		return c.Status(400).JSON(fiber.Map{"error": "address required"})
	}
	result, err := h.svc.ConnectDevice(c.Context(), req.Address)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	statusCode := 200
	if result["status"] == "error" {
		statusCode = 500
	} else if result["status"] == "unauthorized" {
		statusCode = 403
	}
	if result["status"] == "connected" || (result["status"] == "device" && result["state"] == "device") {
		uid, _ := c.Locals("user_id").(string)
		h.svc.SaveDevice(req.Address, "", uid)
	}
	return c.Status(statusCode).JSON(result)
}

func (h *ADBHandler) PairDevice(c fiber.Ctx) error {
	var req PairRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Address == "" || req.Code == "" {
		return c.Status(400).JSON(fiber.Map{"error": "address and code required"})
	}
	result, err := h.svc.PairDevice(c.Context(), req.Address, req.Code)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	statusCode := 200
	if result["status"] == "error" {
		statusCode = 500
	}
	return c.Status(statusCode).JSON(result)
}

func (h *ADBHandler) DiagnoseDevice(c fiber.Ctx) error {
	address := c.Query("address")
	if address == "" {
		// Try body
		var req DiagnoseRequest
		if err := c.Bind().JSON(&req); err == nil {
			address = req.Address
		}
	}
	if address == "" {
		return c.Status(400).JSON(fiber.Map{"error": "address required"})
	}
	result, err := h.svc.DiagnoseDevice(c.Context(), address)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func (h *ADBHandler) DisconnectDevice(c fiber.Ctx) error {
	var req DisconnectRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	out, err := h.svc.DisconnectDevice(c.Context(), req.Address)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": out})
}

func (h *ADBHandler) DisconnectAll(c fiber.Ctx) error {
	out, err := h.svc.DisconnectAll(c.Context())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": out})
}

// ─── Shell & Exec ───

func (h *ADBHandler) RunShell(c fiber.Ctx) error {
	var req ShellRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Command == "" {
		return c.Status(400).JSON(fiber.Map{"error": "command required"})
	}
	result, err := h.svc.RunShell(c.Context(), req.Serial, req.Command)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) RunExec(c fiber.Ctx) error {
	var req ExecRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Command == "" {
		return c.Status(400).JSON(fiber.Map{"error": "command required"})
	}
	result, err := h.svc.RunExec(c.Context(), req.Serial, req.Command)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

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

// ─── App Management ───

func (h *ADBHandler) ListApps(c fiber.Ctx) error {
	serial := c.Query("serial")
	filter := c.Query("filter")
	apps, err := h.svc.ListApps(c.Context(), serial, filter)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"apps": apps, "count": len(apps)})
}

func (h *ADBHandler) InstallApp(c fiber.Ctx) error {
	var req InstallAppRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.APK == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and apk_path required"})
	}
	result, err := h.svc.InstallApp(c.Context(), req.Serial, req.APK)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) UninstallApp(c fiber.Ctx) error {
	var req UninstallAppRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.Package == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and package required"})
	}
	result, err := h.svc.UninstallApp(c.Context(), req.Serial, req.Package, req.KeepData)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) ClearAppData(c fiber.Ctx) error {
	var req AppActionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.Package == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and package required"})
	}
	result, err := h.svc.ClearAppData(c.Context(), req.Serial, req.Package)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) ForceStopApp(c fiber.Ctx) error {
	var req AppActionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.Package == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and package required"})
	}
	result, err := h.svc.ForceStopApp(c.Context(), req.Serial, req.Package)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) LaunchApp(c fiber.Ctx) error {
	var req AppActionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.Package == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and package required"})
	}
	result, err := h.svc.LaunchApp(c.Context(), req.Serial, req.Package)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) ToggleApp(c fiber.Ctx) error {
	var req ToggleAppRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.Package == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and package required"})
	}
	result, err := h.svc.ToggleApp(c.Context(), req.Serial, req.Package, req.Enable)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

// ─── Module Management ───

func (h *ADBHandler) ListInstalledModules(c fiber.Ctx) error {
	serial := c.Query("serial")
	if serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}
	modules, err := h.svc.ListInstalledModules(c.Context(), serial)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"modules": modules})
}

func (h *ADBHandler) GetModuleInfo(c fiber.Ctx) error {
	serial := c.Query("serial")
	name := c.Params("name")
	if serial == "" || name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and name required"})
	}
	mod, err := h.svc.GetModuleInfo(c.Context(), serial, name)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(mod)
}

func (h *ADBHandler) InstallModule(c fiber.Ctx) error {
	var req InstallRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.ZipPath == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and zip_path required"})
	}
	result, err := h.svc.InstallModule(c.Context(), req.Serial, req.ZipPath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) InstallModuleFromURL(c fiber.Ctx) error {
	var req InstallURLRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.URL == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and url required"})
	}
	result, err := h.svc.InstallModuleFromURL(c.Context(), req.Serial, req.URL)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func (h *ADBHandler) UploadAndInstallModule(c fiber.Ctx) error {
	serial := c.FormValue("serial")
	if serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "file required"})
	}
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("upload_module_%d.zip", time.Now().UnixMilli()))
	if err := c.SaveFile(file, tmpFile); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "save file failed: " + err.Error()})
	}
	result, err := h.svc.InstallModule(c.Context(), serial, tmpFile)
	os.Remove(tmpFile)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) ToggleModule(c fiber.Ctx) error {
	var req ToggleModuleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	name := c.Params("name")
	if req.Serial == "" || name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and name required"})
	}
	result, err := h.svc.ToggleModule(c.Context(), req.Serial, name, req.Enable)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) UninstallModule(c fiber.Ctx) error {
	var req struct {
		Serial string `json:"serial"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	name := c.Params("name")
	if req.Serial == "" || name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and name required"})
	}
	result, err := h.svc.UninstallModule(c.Context(), req.Serial, name)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

// ─── Module Backup / Restore / Export / Update ───

func (h *ADBHandler) BackupModule(c fiber.Ctx) error {
	var req BackupModuleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.ModuleName == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and module_name required"})
	}
	// Stream backup directly to client — no server-side storage
	tmpPath := filepath.Join(os.TempDir(), "adb_backup_"+req.ModuleName+".tar.gz")
	_, err := h.svc.BackupModule(c.Context(), req.Serial, req.ModuleName, tmpPath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	// Send file then clean up
	defer os.Remove(tmpPath)
	return c.Download(tmpPath, req.ModuleName+"_backup.tar.gz")
}

func (h *ADBHandler) RestoreModule(c fiber.Ctx) error {
	var req RestoreModuleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.LocalPath == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and local_path required"})
	}
	result, err := h.svc.RestoreModule(c.Context(), req.Serial, req.LocalPath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) CheckModuleUpdate(c fiber.Ctx) error {
	serial := c.Query("serial")
	moduleName := c.Query("module_name")
	if serial == "" || moduleName == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and module_name required"})
	}
	result, err := h.svc.CheckModuleUpdate(c.Context(), serial, moduleName)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// ListBackups returns all ADB module backups stored on the server.
func (h *ADBHandler) ExportModule(c fiber.Ctx) error {
	var req ExportModuleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.ModuleName == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and module_name required"})
	}
	result, err := h.svc.ExportModule(c.Context(), req.Serial, req.ModuleName)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"export_path": result})
}

// ─── Root Manager ───

func (h *ADBHandler) GetAvailableRootManagers(c fiber.Ctx) error {
	serial := c.Query("serial")
	if serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}
	managers, err := h.svc.GetAvailableRootManagers(c.Context(), serial)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"managers": managers})
}

func (h *ADBHandler) ManageRootPermission(c fiber.Ctx) error {
	var req RootPermissionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.PackageName == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and package_name required"})
	}
	result, err := h.svc.ManageRootPermission(c.Context(), req.Serial, req.PackageName, req.Grant)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

func (h *ADBHandler) ListRootPermissions(c fiber.Ctx) error {
	serial := c.Query("serial")
	if serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}
	permissions, err := h.svc.ListRootPermissions(c.Context(), serial)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"permissions": permissions})
}

func (h *ADBHandler) GetRootModules(c fiber.Ctx) error {
	serial := c.Query("serial")
	if serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}
	modules, err := h.svc.GetRootModules(c.Context(), serial)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"modules": modules})
}

// ─── Log Viewer ───

func (h *ADBHandler) GetLogcat(c fiber.Ctx) error {
	serial := c.Query("serial")
	filter := c.Query("filter")
	level := c.Query("level")
	lines := 500
	if l := c.Query("lines"); l != "" {
		fmt.Sscanf(l, "%d", &lines)
	}
	out, err := h.svc.GetLogcat(c.Context(), serial, filter, level, lines)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"logs": out})
}

func (h *ADBHandler) ClearLogcat(c fiber.Ctx) error {
	serial := c.Query("serial")
	out, err := h.svc.ClearLogcat(c.Context(), serial)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": out})
}

// ─── Saved Devices ───

func (h *ADBHandler) GetSavedDevices(c fiber.Ctx) error {
	uid, _ := c.Locals("user_id").(string)
	devices, err := h.svc.GetSavedDevices(uid)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"devices": devices})
}

func (h *ADBHandler) SaveDevice(c fiber.Ctx) error {
	uid, _ := c.Locals("user_id").(string)
	var req struct {
		Address string `json:"address"`
		Name    string `json:"name"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Address == "" {
		return c.Status(400).JSON(fiber.Map{"error": "address required"})
	}
	if err := h.svc.SaveDevice(req.Address, req.Name, uid); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"id": 0})
}

func (h *ADBHandler) DeleteSavedDevice(c fiber.Ctx) error {
	uid, _ := c.Locals("user_id").(string)
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.svc.DeleteSavedDevice(id, uid); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

// ─── Device Operations ───

func (h *ADBHandler) RebootDevice(c fiber.Ctx) error {
	var req RebootRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial required"})
	}
	if err := h.svc.RebootDevice(c.Context(), req.Serial, req.Mode); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "rebooting"})
}

func (h *ADBHandler) GetProp(c fiber.Ctx) error {
	serial := c.Query("serial")
	prop := c.Query("prop")
	if serial == "" || prop == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and prop required"})
	}
	value, err := h.svc.GetProp(c.Context(), serial, prop)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"prop": prop, "value": value})
}

func (h *ADBHandler) SetProp(c fiber.Ctx) error {
	var req PropRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.Prop == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and prop required"})
	}
	result, err := h.svc.SetProp(c.Context(), req.Serial, req.Prop, req.Value)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": result})
}

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

// ─── Enhanced Module Push Handlers ───

// PushModuleFolder pushes a complete module folder structure to the device.
func (h *ADBHandler) PushModuleFolder(c fiber.Ctx) error {
	var req PushModuleFolderRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.LocalDir == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and local_dir required"})
	}

	result, err := h.svc.PushModuleFolder(c.Context(), req.Serial, req.LocalDir, req.ModuleName, req.Install)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if req.Reboot {
		h.svc.RebootDevice(c.Context(), req.Serial, "normal")
		result["rebooting"] = true
	}
	return c.JSON(result)
}

// PushBuildModule pushes a build artifact by build ID to the device as a complete folder.
func (h *ADBHandler) PushBuildModule(c fiber.Ctx) error {
	var req PushBuildModuleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.BuildID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and build_id required"})
	}

	result, err := h.svc.PushBuildByID(c.Context(), req.Serial, req.BuildID, req.ModuleName, req.Install)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if req.Reboot {
		h.svc.RebootDevice(c.Context(), req.Serial, "normal")
		result["rebooting"] = true
	}
	return c.JSON(result)
}

// PushBuildZip pushes a local module.zip to the device as a complete folder.
func (h *ADBHandler) PushBuildZip(c fiber.Ctx) error {
	var req PushBuildZipRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Serial == "" || req.ZipPath == "" {
		return c.Status(400).JSON(fiber.Map{"error": "serial and zip_path required"})
	}

	result, err := h.svc.PushBuildModule(c.Context(), req.Serial, req.ZipPath, req.ModuleName, req.Install)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if req.Reboot {
		h.svc.RebootDevice(c.Context(), req.Serial, "normal")
		result["rebooting"] = true
	}
	return c.JSON(result)
}
