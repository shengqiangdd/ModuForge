package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
