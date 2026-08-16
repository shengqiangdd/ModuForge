package service

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"github.com/moduforge/backend/internal/builder"
)


// ─── Build & Push ───
func (s *ADBService) PushModuleFolder(ctx context.Context, serial, localDir, moduleName string, install bool) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// 1. Zip the directory locally (excluding source code, build cache, etc.)
	tmpZip := filepath.Join(os.TempDir(), fmt.Sprintf("module_push_%d.zip", time.Now().UnixMilli()))
	if err := builder.ZipDirExcluding(localDir, tmpZip, builder.ModuleExcludePatterns); err != nil {
		return nil, fmt.Errorf("zip module directory failed: %w", err)
	}
	defer os.Remove(tmpZip)

	zipInfo, _ := os.Stat(tmpZip)
	result["zip_size"] = zipInfo.Size()

	// 2. Push zip to device (unique path so concurrent installs can't clash)
	remotePath := fmt.Sprintf("/data/local/tmp/module_push_%d.zip", time.Now().UnixMilli())
	if _, err := s.PushFile(ctx, serial, tmpZip, remotePath); err != nil {
		return nil, fmt.Errorf("push zip to device failed: %w", err)
	}

	// 3. If install=true, detect root manager and install
	if install {
		mgr, installCmd := s.detectRootManagerAndInstallCmd(ctx, serial)
		if installCmd == "" {
			return nil, fmt.Errorf("no supported root manager found (tried APatch/KernelSU/Magisk)")
		}
		log.Printf("[ADB] Installing module via %s", mgr)
		out, err := s.RunShell(ctx, serial, installCmd+remotePath+"'")
		if err != nil {
			return nil, fmt.Errorf("install failed (%s): %w", mgr, err)
		}
		result["install_output"] = strings.TrimSpace(out)
		result["root_manager"] = mgr
	}

	// Cleanup
	s.RunShell(ctx, serial, "rm -f "+remotePath)

	result["module_name"] = moduleName
	result["source_dir"] = localDir
	result["message"] = "Module zip pushed" + map[bool]string{true: " and installed", false: ""}[install] + "."
	return result, nil
}

// PushBuildModule pushes a build artifact zip to the device and installs it
// via the detected root manager.
func (s *ADBService) PushBuildModule(ctx context.Context, serial, zipPath, moduleName string, install bool) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	zipInfo, err := os.Stat(zipPath)
	if err != nil {
		return nil, fmt.Errorf("zip file not found: %w", err)
	}
	result["zip_size"] = zipInfo.Size()

	// 1. Push zip to device (unique path so concurrent installs can't clash)
	remotePath := fmt.Sprintf("/data/local/tmp/module_push_%d.zip", time.Now().UnixMilli())
	if _, err := s.PushFile(ctx, serial, zipPath, remotePath); err != nil {
		return nil, fmt.Errorf("push zip to device failed: %w", err)
	}

	// 2. If install=true, detect root manager and install
	if install {
		mgr, installCmd := s.detectRootManagerAndInstallCmd(ctx, serial)
		if installCmd == "" {
			return nil, fmt.Errorf("no supported root manager found (tried APatch/KernelSU/Magisk)")
		}
		log.Printf("[ADB] Installing module via %s", mgr)
		out, err := s.RunShell(ctx, serial, installCmd+remotePath+"'")
		if err != nil {
			return nil, fmt.Errorf("install failed (%s): %w", mgr, err)
		}
		result["install_output"] = strings.TrimSpace(out)
		result["root_manager"] = mgr
	}

	// Cleanup
	s.RunShell(ctx, serial, "rm -f "+remotePath)

	result["module_name"] = moduleName
	result["source_zip"] = zipPath
	result["message"] = "Module zip pushed" + map[bool]string{true: " and installed", false: ""}[install] + "."
	return result, nil
}

// PushBuildByID pushes a build artifact by build ID to the device and installs it.
func (s *ADBService) PushBuildByID(ctx context.Context, serial, buildID, moduleName string, install bool) (map[string]interface{}, error) {
	// Query database for artifact path
	var artifactPath string
	err := s.db.QueryRowContext(ctx, "SELECT artifact_path FROM build_tasks WHERE id = ?", buildID).Scan(&artifactPath)
	if err != nil {
		return nil, fmt.Errorf("build not found: %w", err)
	}

	// Resolve full path
	fullPath := artifactPath
	if !filepath.IsAbs(fullPath) {
		storagePath := os.Getenv("STORAGE_PATH")
		if storagePath == "" {
			storagePath = "/data/storage"
		}
		fullPath = filepath.Join(storagePath, artifactPath)
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("artifact file not found: %s", fullPath)
	}

	return s.PushBuildModule(ctx, serial, fullPath, moduleName, install)
}

// extractZip extracts a zip file to a destination directory.
func extractZip(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		path := filepath.Join(dst, f.Name)

		// Security: prevent path traversal
		if !strings.HasPrefix(path, filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, 0755)
			continue
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
