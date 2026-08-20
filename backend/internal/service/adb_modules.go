package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Core module management: listing, info, toggle, uninstall, and size helpers.

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
