package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Module installation, root manager detection, and permission management.

func (s *ADBService) InstallModuleFromURL(ctx context.Context, serial, moduleURL string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	result["url"] = moduleURL

	// 1. Download the zip
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("module_%d.zip", time.Now().UnixMilli()))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, moduleURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create download request failed: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	f, err := os.Create(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("create temp file failed: %w", err)
	}
	written, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpFile)
		return nil, fmt.Errorf("save download failed: %w", err)
	}
	result["downloaded_size"] = written
	result["temp_file"] = tmpFile

	// 2. Push to device
	remotePath := "/data/local/tmp/module_install.zip"
	if _, err := s.PushFile(ctx, serial, tmpFile, remotePath); err != nil {
		os.Remove(tmpFile)
		return nil, fmt.Errorf("push to device failed: %w", err)
	}

	// 3. Install via detected root manager
	mgr, installCmd := s.detectRootManagerAndInstallCmd(ctx, serial)
	if installCmd == "" {
		os.Remove(tmpFile)
		return nil, fmt.Errorf("no supported root manager found (tried APatch/KernelSU/Magisk)")
	}
	log.Printf("[ADB] Installing module from URL via %s", mgr)
	installOut, err := s.RunShell(ctx, serial, installCmd+remotePath+"'")
	if err != nil {
		os.Remove(tmpFile)
		return nil, fmt.Errorf("module install failed (%s): %w", mgr, err)
	}
	result["install_output"] = strings.TrimSpace(installOut)
	result["root_manager"] = mgr

	// 4. Cleanup
	os.Remove(tmpFile)
	s.RunShell(ctx, serial, "rm "+remotePath)

	return result, nil
}

// detectRootManagerAndInstallCmd detects the root manager on the device and returns
// the appropriate install command prefix. The returned cmd already includes the
// trailing space, so callers just append the zip path.
// Priority: APatch > KernelSU > Magisk (each has a unique binary).
func (s *ADBService) detectRootManagerAndInstallCmd(ctx context.Context, serial string) (string, string) {
	// Bound the probe with a timeout so a hung device cannot block installs forever.
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	run := func(args ...string) string {
		cmdArgs := append([]string{"-s", serial, "shell"}, args...)
		out, err := s.run(probeCtx, cmdArgs...)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(out)
	}

	// APatch: apd binary is unique to APatch
	apVer := run("su", "-c", "apd --version")
	if apVer != "" && !strings.Contains(apVer, "not found") && !strings.Contains(apVer, "No such") {
		return "APatch", "su -c 'apd module install "
	}

	// KernelSU: ksud binary is unique to KernelSU
	ksuVer := run("su", "-c", "ksud --version")
	if ksuVer != "" && !strings.Contains(ksuVer, "not found") && !strings.Contains(ksuVer, "No such") {
		return "KernelSU", "su -c 'ksud module install "
	}

	// Magisk: magisk binary is unique to Magisk
	magVer := run("su", "-c", "magisk -v")
	if magVer != "" && !strings.Contains(magVer, "not found") && !strings.Contains(magVer, "No such") {
		return "Magisk", "su -c 'magisk --install-module "
	}

	return "", ""
}

func (s *ADBService) GetAvailableRootManagers(ctx context.Context, serial string) ([]map[string]string, error) {
	var managers []map[string]string

	// Helper: run command via ADB shell (with the caller's context for timeouts)
	run := func(args ...string) string {
		cmdArgs := append([]string{"-s", serial, "shell"}, args...)
		out, err := s.run(ctx, cmdArgs...)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(out)
	}

	// Detect which managers exist with a single directory listing, then probe
	// versions only for the ones present (previously this was up to 9 su calls).
	dirOut := run("su", "-c", "ls -1 /data/adb/ 2>/dev/null")
	hasDir := func(name string) bool {
		for _, l := range strings.Split(dirOut, "\n") {
			if strings.TrimSpace(l) == name {
				return true
			}
		}
		return false
	}

	if hasDir("ap") {
		ver := run("su", "-c", "/data/adb/ap/bin/apd --version 2>/dev/null || /data/adb/ap/bin/apd -v 2>/dev/null || apd --version 2>/dev/null")
		if ver != "" && (strings.Contains(ver, "not found") || strings.Contains(ver, "No such")) {
			ver = ""
		}
		managers = append(managers, map[string]string{
			"name":    "APatch",
			"version": ver,
			"path":    "/data/adb/ap/bin/apd",
		})
	}

	if hasDir("ksu") {
		ver := run("su", "-c", "/data/adb/ksu/bin/ksud --version 2>/dev/null || ksud --version 2>/dev/null")
		if ver != "" && (strings.Contains(ver, "not found") || strings.Contains(ver, "No such")) {
			ver = ""
		}
		managers = append(managers, map[string]string{
			"name":    "KernelSU",
			"version": ver,
			"path":    "/data/adb/ksu/bin/ksud",
		})
	}

	if hasDir("magisk") {
		ver := run("su", "-c", "magisk -v 2>/dev/null || magisk --version 2>/dev/null")
		if ver != "" && (strings.Contains(ver, "not found") || strings.Contains(ver, "No such")) {
			ver = ""
		}
		managers = append(managers, map[string]string{
			"name":    "Magisk",
			"version": ver,
			"path":    "/data/adb/magisk",
		})
	}

	// Fallback: if no directories matched, probe each binary directly
	if len(managers) == 0 {
		apVer := run("su", "-c", "apd --version 2>/dev/null")
		if apVer != "" && !strings.Contains(apVer, "not found") && !strings.Contains(apVer, "No such") {
			managers = append(managers, map[string]string{"name": "APatch", "version": apVer, "path": "/data/adb/ap/bin/apd"})
		}
		ksuVer := run("su", "-c", "ksud --version 2>/dev/null")
		if ksuVer != "" && !strings.Contains(ksuVer, "not found") && !strings.Contains(ksuVer, "No such") {
			managers = append(managers, map[string]string{"name": "KernelSU", "version": ksuVer, "path": "/data/adb/ksu/bin/ksud"})
		}
		magVer := run("su", "-c", "magisk -v 2>/dev/null")
		if magVer != "" && !strings.Contains(magVer, "not found") && !strings.Contains(magVer, "No such") {
			managers = append(managers, map[string]string{"name": "Magisk", "version": magVer, "path": "/data/adb/magisk"})
		}
	}

	log.Printf("[ADB] Root manager detection: found %d managers", len(managers))
	return managers, nil
}

func (s *ADBService) ManageRootPermission(ctx context.Context, serial, packageName string, grant bool) (string, error) {
	action := "grant"
	if !grant {
		action = "revoke"
	}
	_, err := s.RunShell(ctx, serial, "su -c 'pm "+action+" "+packageName+" android.permission.ROOT' 2>/dev/null")
	if err != nil {
		return "", fmt.Errorf("%s root permission failed: %w", action, err)
	}
	if grant {
		return "已授予 Root 权限", nil
	}
	return "已撤销 Root 权限", nil
}

func (s *ADBService) ListRootPermissions(ctx context.Context, serial string) ([]map[string]string, error) {
	var permissions []map[string]string
	// Parse dumpsys output by package block: track the current package, then
	// emit it when its android.permission.ROOT line carries granted=true.
	// The old grep -A5 approach could associate granted=true with the wrong
	// package because dumpsys groups permissions per package.
	out, err := s.RunShell(ctx, serial,
		`su -c 'dumpsys package 2>/dev/null' | awk '/^  Package \[/ {pkg=$2; gsub(/\[|\]/, "", pkg)} /android.permission.ROOT/ && /granted=true/ {print pkg}'`)
	if err != nil || out == "" {
		return permissions, nil
	}
	seen := make(map[string]bool)
	for _, line := range strings.Split(out, "\n") {
		pkg := strings.TrimSpace(line)
		if pkg != "" && !seen[pkg] {
			seen[pkg] = true
			permissions = append(permissions, map[string]string{"package": pkg, "status": "granted"})
		}
	}
	return permissions, nil
}

func (s *ADBService) getModuleBasePath(ctx context.Context, serial string) string {
	return "/data/adb/modules"
}
