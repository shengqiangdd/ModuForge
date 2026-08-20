package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Module backup, restore, export, update checking, and listing operations.

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
