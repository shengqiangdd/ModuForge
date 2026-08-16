package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)


// ─── Module Management (install, backup, root) ───
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

// ─── Module Backup / Restore / Export / Update (1.1-1.4) ───

func (s *ADBService) BackupModule(ctx context.Context, serial, moduleName, localPath string) (string, error) {
	moduleName = sanitizePath(moduleName)
	basePath := s.getModuleBasePath(ctx, serial)
	// Check existence — try direct, then su
	exists, _ := s.RunShell(ctx, serial, "test -d "+basePath+"/"+moduleName+" && echo yes || echo no")
	if !strings.Contains(exists, "yes") {
		exists, _ = s.RunShell(ctx, serial, "su -c 'test -d "+basePath+"/"+moduleName+"' && echo yes || echo no")
	}
	if !strings.Contains(exists, "yes") {
		return "", fmt.Errorf("模块 %s 不存在", moduleName)
	}
	archivePath := "/data/local/tmp/backup_" + moduleName + ".tar.gz"
	escapedBase := strings.ReplaceAll(basePath, "'", "'\\''")
	escapedName := strings.ReplaceAll(moduleName, "'", "'\\''")
	// Try tar (universally available on Android), fallback to zip
	errMsg := ""
	out, err := s.RunShell(ctx, serial, "cd "+basePath+" && tar czf "+archivePath+" "+moduleName)
	if err != nil || strings.TrimSpace(out) == "" {
		errMsg = fmt.Sprintf("tar: %v %s", err, out)
		// Try with su
		out, err = s.RunShell(ctx, serial, "su -c 'cd "+escapedBase+" && tar czf "+archivePath+" "+escapedName+"'")
		if err != nil || strings.TrimSpace(out) == "" {
			errMsg += " | su-tar: " + err.Error() + " " + out
			// Last resort: try zip
			_, err = s.RunShell(ctx, serial, "cd "+basePath+" && zip -r "+archivePath+" "+moduleName)
			if err != nil {
				_, err = s.RunShell(ctx, serial, "su -c 'cd "+escapedBase+" && zip -r "+archivePath+" "+escapedName+"'")
				if err != nil {
					return "", fmt.Errorf("打包失败 (tar/zip均不可用): %s", errMsg)
				}
			}
		}
	}
	_pullOut, pullErr := s.run(ctx, "-s", serial, "pull", archivePath, localPath)
	s.RunShell(ctx, serial, "rm -f "+archivePath)
	if pullErr != nil {
		return "", fmt.Errorf("拉取失败: %v", pullErr)
	}
	return strings.TrimSpace(_pullOut), nil
}

func (s *ADBService) RestoreModule(ctx context.Context, serial, localPath string) (string, error) {
	remoteTmp := "/data/local/tmp/restore_module.tar.gz"
	_, err := s.run(ctx, "-s", serial, "push", localPath, remoteTmp)
	if err != nil {
		return "", fmt.Errorf("推送失败: %w", err)
	}
	basePath := s.getModuleBasePath(ctx, serial)
	// Try tar first, fallback to unzip
	_, err = s.RunShell(ctx, serial, "cd "+basePath+" && tar xzf "+remoteTmp)
	if err != nil {
		_, err = s.RunShell(ctx, serial, "su -c 'cd "+basePath+" && tar xzf "+remoteTmp+"'")
		if err != nil {
			_, err = s.RunShell(ctx, serial, "cd "+basePath+" && unzip -o "+remoteTmp)
			if err != nil {
				_, err = s.RunShell(ctx, serial, "su -c 'cd "+basePath+" && unzip -o "+remoteTmp+"'")
				if err != nil {
					s.RunShell(ctx, serial, "rm -f "+remoteTmp)
					return "", fmt.Errorf("解压失败: tar/zip均不可用")
				}
			}
		}
	}
	s.RunShell(ctx, serial, "chmod -R 755 "+basePath+"/*")
	s.RunShell(ctx, serial, "rm -f "+remoteTmp)
	return "模块恢复成功", nil
}

func (s *ADBService) ExportModule(ctx context.Context, serial, moduleName string) (string, error) {
	moduleName = sanitizePath(moduleName)
	basePath := s.getModuleBasePath(ctx, serial)
	// Check existence — try direct, then su
	exists, _ := s.RunShell(ctx, serial, "test -d "+basePath+"/"+moduleName+" && echo yes || echo no")
	if !strings.Contains(exists, "yes") {
		exists, _ = s.RunShell(ctx, serial, "su -c 'test -d "+basePath+"/"+moduleName+"' && echo yes || echo no")
	}
	if !strings.Contains(exists, "yes") {
		return "", fmt.Errorf("模块 %s 不存在", moduleName)
	}
	archivePath := "/data/local/tmp/" + moduleName + ".tar.gz"
	escapedBase := strings.ReplaceAll(basePath, "'", "'\\''")
	escapedName := strings.ReplaceAll(moduleName, "'", "'\\''")
	// Try tar first, fallback to zip
	out, err := s.RunShell(ctx, serial, "cd "+basePath+" && tar czf "+archivePath+" "+moduleName)
	if err != nil || strings.TrimSpace(out) == "" {
		_, err = s.RunShell(ctx, serial, "su -c 'cd "+escapedBase+" && tar czf "+archivePath+" "+escapedName+"'")
		if err != nil {
			_, err = s.RunShell(ctx, serial, "cd "+basePath+" && zip -r "+archivePath+" "+moduleName)
			if err != nil {
				_, err = s.RunShell(ctx, serial, "su -c 'cd "+escapedBase+" && zip -r "+archivePath+" "+escapedName+"'")
				if err != nil {
					return "", fmt.Errorf("打包失败: tar/zip均不可用")
				}
			}
		}
	}
	return archivePath, nil
}

func (s *ADBService) CheckModuleUpdate(ctx context.Context, serial, moduleName string) (map[string]interface{}, error) {
	moduleName = sanitizePath(moduleName)
	// Universal module path
	basePath := "/data/adb/modules"
	propOut, err := s.RunShell(ctx, serial, "cat "+basePath+"/"+moduleName+"/module.prop 2>/dev/null")
	if err != nil || strings.TrimSpace(propOut) == "" {
		// Fallback: try with su (some devices need root to read /data/adb/modules)
		escaped := strings.ReplaceAll(basePath+"/"+moduleName+"/module.prop", "'", "'\\''")
		propOut, err = s.RunShell(ctx, serial, "su -c 'cat "+escaped+"' 2>/dev/null")
		if err != nil || strings.TrimSpace(propOut) == "" {
			return nil, fmt.Errorf("模块 %s 未找到或无权读取", moduleName)
		}
	}
	currentVersion := parsePropValue(propOut, "version")
	updateOut, _ := s.RunShell(ctx, serial, "cat "+basePath+"/"+moduleName+"/update.json 2>/dev/null")
	if updateOut == "" {
		return map[string]interface{}{"has_update": false, "current_version": currentVersion}, nil
	}
	var updateInfo struct {
		Version   string `json:"version"`
		Changelog string `json:"changelog"`
		URL       string `json:"url"`
	}
	if json.Unmarshal([]byte(updateOut), &updateInfo) == nil && updateInfo.Version != "" {
		result := map[string]interface{}{
			"has_update":      updateInfo.Version != currentVersion,
			"current_version": currentVersion,
			"latest_version":  updateInfo.Version,
			"changelog":       updateInfo.Changelog,
			"download_url":    updateInfo.URL,
		}
		// If the module advertises a remote update URL, fetch it to see whether
		// a newer version is available than the one recorded locally.
		if updateInfo.URL != "" {
			if remote, rerr := s.fetchRemoteModuleUpdate(ctx, updateInfo.URL); rerr == nil {
				if remote.Version != "" {
					result["latest_version"] = remote.Version
					result["has_update"] = remote.Version != currentVersion
				}
				if remote.Changelog != "" {
					result["changelog"] = remote.Changelog
				}
				if remote.URL != "" {
					result["download_url"] = remote.URL
				}
			} else {
				result["check_error"] = rerr.Error()
			}
		}
		return result, nil
	}
	return map[string]interface{}{"has_update": false, "current_version": currentVersion}, nil
}

type remoteModuleUpdate struct {
	Version   string `json:"version"`
	Changelog string `json:"changelog"`
	URL       string `json:"url"`
}

// fetchRemoteModuleUpdate fetches a module's update.json from a remote URL,
// with a 10s timeout and a 1 MiB body cap. Failures are returned to the caller
// (surfaced as check_error) rather than failing the whole update check.
func (s *ADBService) fetchRemoteModuleUpdate(ctx context.Context, url string) (*remoteModuleUpdate, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ModuForge/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update fetch failed: HTTP %d", resp.StatusCode)
	}
	var u remoteModuleUpdate
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *ADBService) getModuleBasePath(ctx context.Context, serial string) string {
	return "/data/adb/modules"
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

func (s *ADBService) GetRootModules(ctx context.Context, serial string) ([]map[string]interface{}, error) {
	var modules []map[string]interface{}
	basePath := s.getModuleBasePath(ctx, serial)
	out, err := s.RunShell(ctx, serial, "ls -1 "+basePath+" 2>/dev/null")
	if err != nil || out == "" {
		return modules, nil
	}
	for _, name := range strings.Split(strings.TrimSpace(out), "\n") {
		name = strings.TrimSpace(name)
		if name == "" || name == "lost+found" {
			continue
		}
		propOut, _ := s.RunShell(ctx, serial, "cat "+basePath+"/"+name+"/module.prop 2>/dev/null")
		disableOut, _ := s.RunShell(ctx, serial, "test -f "+basePath+"/"+name+"/disable && echo disabled || echo enabled")
		enabled := !strings.Contains(disableOut, "disabled")
		modules = append(modules, map[string]interface{}{
			"name":        name,
			"version":     parsePropValue(propOut, "version"),
			"description": parsePropValue(propOut, "description"),
			"author":      parsePropValue(propOut, "author"),
			"enabled":     enabled,
		})
	}
	return modules, nil
}

// getModuleSize attempts to get the size of a module directory.
// Tries multiple approaches for Android compatibility.
func (s *ADBService) getModuleSize(ctx context.Context, serial, path string) string {
	cmds := []string{
		"du -sh '" + path + "' 2>/dev/null",
		"du -sk '" + path + "' 2>/dev/null",
		"ls -la '" + path + "' 2>/dev/null | awk '{print $5}'",
		"wc -c < '" + path + "' 2>/dev/null",
	}
	for _, cmd := range cmds {
		if out, err := s.RunShell(ctx, serial, cmd); err == nil {
			out = strings.TrimSpace(out)
			if out == "" || out == "0" {
				continue
			}
			// Human-readable format from du -sh
			if strings.Contains(out, "M") || strings.Contains(out, "K") || strings.Contains(out, "G") {
				parts := strings.Fields(out)
				if len(parts) > 0 {
					return parts[0]
				}
			}
			// Numeric size (bytes or KB)
			if sz, err := strconv.ParseInt(out, 10, 64); err == nil && sz > 0 {
				if sz >= 1024*1024 {
					return fmt.Sprintf("%.1fM", float64(sz)/(1024*1024))
				} else if sz >= 1024 {
					return fmt.Sprintf("%.1fK", float64(sz)/1024)
				}
				return fmt.Sprintf("%dB", sz)
			}
		}
	}
	return ""
}

func parsePropValue(content, key string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+"=") {
			return strings.TrimPrefix(line, key+"=")
		}
	}
	return ""
}

func (s *ADBService) ListInstalledModules(ctx context.Context, serial string) ([]InstalledModule, error) {
	// Universal module path — all root managers (Magisk/KernelSU/APatch) use this.
	// One shell script collects every module in a single `adb shell` call
	// (previously this was ~5 calls per module, which was very slow with many
	// modules installed). Records use the ASCII record separator (0x1E), fields
	// use the unit separator (0x1F) — both are unlikely to appear in module.prop.
	basePath := "/data/adb/modules"
	log.Printf("[ADB] ListInstalledModules: serial=%s, path=%s", serial, basePath)

	script := `sep=$(printf '\037'); rec=$(printf '\036');
for d in ` + basePath + `/*/; do
  [ -d "$d" ] || continue
  n=$(basename "$d")
  [ "$n" = "lost+found" ] && continue
  v=$(grep -m1 '^version=' "$d/module.prop" 2>/dev/null | cut -d= -f2-)
  a=$(grep -m1 '^author=' "$d/module.prop" 2>/dev/null | cut -d= -f2-)
  ds=$(grep -m1 '^description=' "$d/module.prop" 2>/dev/null | cut -d= -f2-)
  en=1; [ -f "$d/disable" ] && en=0
  sz=$(du -sk "$d" 2>/dev/null | awk '{print $1}')
  dt=$(stat -c %y "$d" 2>/dev/null | cut -d. -f1)
  up=0; [ -f "$d/update.json" ] && up=1
  echo "MOD${rec}$n${sep}$v${sep}$a${sep}$ds${sep}$en${sep}$sz${sep}$dt${sep}$up"
done`

	out, err := s.RunShell(ctx, serial, script)
	if err != nil || !strings.Contains(out, "MOD") {
		// Fallback: some devices need root to read /data/adb/modules
		log.Printf("[ADB] Normal shell failed, trying su fallback (err=%v)", err)
		escapedScript := strings.ReplaceAll(script, "'", "'\\''")
		out, err = s.RunShell(ctx, serial, "su -c '"+escapedScript+"' 2>/dev/null")
	}
	if err != nil || strings.TrimSpace(out) == "" {
		log.Printf("[ADB] No modules found on device %s", serial)
		return nil, nil
	}

	const recSep = "\x1e"
	const fieldSep = "\x1f"
	var modules []InstalledModule
	seen := make(map[string]bool)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "MOD"+recSep) {
			continue
		}
		f := strings.Split(strings.TrimPrefix(line, "MOD"+recSep), fieldSep)
		if len(f) < 8 {
			continue
		}
		name := sanitizePath(f[0])
		if name == "" || name == "lost+found" || seen[name] {
			continue
		}
		seen[name] = true

		mod := InstalledModule{
			Name:        name,
			Version:     strings.TrimSpace(f[1]),
			Author:      strings.TrimSpace(f[2]),
			Description: strings.TrimSpace(f[3]),
			Source:      "Magisk",
			Enabled:     strings.TrimSpace(f[4]) != "0",
			UpdateDate:  strings.TrimSpace(f[6]),
			HasUpdate:   strings.TrimSpace(f[7]) == "1",
		}
		// du -sk reports KB
		if sz, err := strconv.ParseInt(strings.TrimSpace(f[5]), 10, 64); err == nil && sz > 0 {
			if sz >= 1024 {
				mod.Size = fmt.Sprintf("%.1fM", float64(sz)/1024)
			} else {
				mod.Size = fmt.Sprintf("%dK", sz)
			}
		}
		modules = append(modules, mod)
	}

	log.Printf("[ADB] Found %d modules on device %s", len(modules), serial)
	return modules, nil
}

func (s *ADBService) GetModuleInfo(ctx context.Context, serial, moduleName string) (*InstalledModule, error) {
	moduleName = sanitizePath(moduleName)
	modulePaths := []string{
		"/data/adb/modules",
		"/data/adb/ksu/modules",
		"/data/adb/ap/modules",
	}
	var mod *InstalledModule
	for _, basePath := range modulePaths {
		propOut, err := s.RunShell(ctx, serial, "cat "+basePath+"/"+moduleName+"/module.prop 2>/dev/null")
		if err != nil {
			continue
		}
		source := "Magisk"
		if strings.Contains(basePath, "ksu") {
			source = "KernelSU"
		} else if strings.Contains(basePath, "apatch") {
			source = "APatch"
		}
		mod = &InstalledModule{Name: moduleName, Source: source}
		for _, line := range strings.Split(propOut, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "version=") {
				mod.Version = strings.TrimPrefix(line, "version=")
			} else if strings.HasPrefix(line, "author=") {
				mod.Author = strings.TrimPrefix(line, "author=")
			} else if strings.HasPrefix(line, "description=") {
				mod.Description = strings.TrimPrefix(line, "description=")
			}
		}
		disableOut, _ := s.RunShell(ctx, serial, "test -f "+basePath+"/"+moduleName+"/disable && echo disabled || echo enabled")
		mod.Enabled = !strings.Contains(disableOut, "disabled")
		mod.Size = s.getModuleSize(ctx, serial, basePath+"/"+moduleName)
		dateOut, _ := s.RunShell(ctx, serial, "stat -c %y "+basePath+"/"+moduleName+" 2>/dev/null | cut -d'.' -f1")
		mod.UpdateDate = strings.TrimSpace(dateOut)
		updateCheck, _ := s.RunShell(ctx, serial, "test -f "+basePath+"/"+moduleName+"/update.json && echo yes || echo no")
		mod.HasUpdate = strings.Contains(updateCheck, "yes")
		break
	}
	if mod == nil {
		return nil, fmt.Errorf("module %s not found", moduleName)
	}
	return mod, nil
}

func (s *ADBService) ToggleModule(ctx context.Context, serial, moduleName string, enable bool) (string, error) {
	moduleName = sanitizePath(moduleName)
	modulePaths := []string{
		"/data/adb/modules",
		"/data/adb/ksu/modules",
		"/data/adb/ap/modules",
	}
	found := false
	for _, basePath := range modulePaths {
		checkOut, _ := s.RunShell(ctx, serial, "test -d "+basePath+"/"+moduleName+" && echo yes || echo no")
		if strings.Contains(checkOut, "yes") {
			var cmd string
			if enable {
				cmd = "rm -f " + basePath + "/" + moduleName + "/disable && echo '" + moduleName + " enabled'"
			} else {
				cmd = "touch " + basePath + "/" + moduleName + "/disable && echo '" + moduleName + " disabled'"
			}
			out, err := s.RunShell(ctx, serial, cmd)
			if err != nil {
				return "", err
			}
			found = true
			return strings.TrimSpace(out), nil
		}
	}
	if !found {
		return "", fmt.Errorf("module %s not found", moduleName)
	}
	return "", nil
}

func (s *ADBService) UninstallModule(ctx context.Context, serial, moduleName string) (string, error) {
	moduleName = sanitizePath(moduleName)
	modulePaths := []string{
		"/data/adb/modules",
		"/data/adb/ksu/modules",
		"/data/adb/ap/modules",
	}
	for _, basePath := range modulePaths {
		checkOut, _ := s.RunShell(ctx, serial, "test -d "+basePath+"/"+moduleName+" && echo yes || echo no")
		if strings.Contains(checkOut, "yes") {
			return s.RunShell(ctx, serial, "rm -rf "+basePath+"/"+moduleName+" && echo 'Module uninstalled, reboot to complete'")
		}
	}
	return "", fmt.Errorf("module %s not found", moduleName)
}

