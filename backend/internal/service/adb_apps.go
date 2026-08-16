package service

import (
	"context"
	"fmt"
	"log"
	"strings"
)


// ─── App Management ───

// ─── App Management ───

func (s *ADBService) ListApps(ctx context.Context, serial, filter string) ([]AppInfo, error) {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "shell", "pm", "list", "packages", "-f")
	if filter == "thirdparty" {
		args = append(args, "-3")
	} else if filter == "system" {
		args = append(args, "-s")
	}
	out, err := s.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("list apps failed: %w", err)
	}
	var apps []AppInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "package:") {
			continue
		}
		pkgPath := strings.TrimPrefix(line, "package:")
		parts := strings.Split(pkgPath, "=")
		if len(parts) < 2 {
			continue
		}
		app := AppInfo{
			PackageName: parts[1],
			System:      strings.Contains(parts[0], "/system/"),
		}
		// Get version info
		dumpOut, err := s.RunShell(ctx, serial, "dumpsys package "+app.PackageName+" 2>/dev/null | grep -E '(versionName|versionCode|targetSdk|minSdk|firstInstallTime)' | head -6")
		if err == nil {
			for _, dline := range strings.Split(dumpOut, "\n") {
				dline = strings.TrimSpace(dline)
				if strings.Contains(dline, "versionName=") {
					app.VersionName = strings.TrimPrefix(dline, "versionName=")
				} else if strings.Contains(dline, "versionCode=") {
					fmt.Sscanf(strings.TrimPrefix(dline, "versionCode="), "%d", &app.VersionCode)
				} else if strings.Contains(dline, "targetSdk=") {
					fmt.Sscanf(strings.TrimPrefix(dline, "targetSdk="), "%d", &app.TargetSDK)
				} else if strings.Contains(dline, "minSdk=") {
					fmt.Sscanf(strings.TrimPrefix(dline, "minSdk="), "%d", &app.MinSDK)
				} else if strings.Contains(dline, "firstInstallTime=") {
					app.InstallTime = strings.TrimPrefix(dline, "firstInstallTime=")
				}
			}
		}
		apps = append(apps, app)
	}
	return apps, nil
}

func (s *ADBService) UninstallApp(ctx context.Context, serial, packageName string, keepData bool) (string, error) {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "shell", "pm", "uninstall")
	if keepData {
		args = append(args, "-k")
	}
	args = append(args, packageName)
	out, err := s.run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("uninstall failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) ClearAppData(ctx context.Context, serial, packageName string) (string, error) {
	out, err := s.RunShell(ctx, serial, "pm clear "+packageName)
	if err != nil {
		return "", fmt.Errorf("clear data failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) ForceStopApp(ctx context.Context, serial, packageName string) (string, error) {
	_, err := s.RunShell(ctx, serial, "am force-stop "+packageName)
	if err != nil {
		return "", fmt.Errorf("force stop failed: %w", err)
	}
	return "stopped", nil
}

func (s *ADBService) LaunchApp(ctx context.Context, serial, packageName string) (string, error) {
	// Try to get launcher activity
	launchOut, err := s.RunShell(ctx, serial, "cmd package resolve-activity --brief "+packageName+" 2>/dev/null | tail -1")
	if err == nil && strings.Contains(launchOut, "/") {
		out, err := s.RunShell(ctx, serial, "am start -n "+strings.TrimSpace(launchOut))
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(out), nil
	}
	// Fallback: launch with monkey
	out, err := s.RunShell(ctx, serial, "monkey -p "+packageName+" -c android.intent.category.LAUNCHER 1 2>/dev/null")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) ToggleApp(ctx context.Context, serial, packageName string, enable bool) (string, error) {
	action := "enable"
	if !enable {
		action = "disable"
	}
	out, err := s.RunShell(ctx, serial, "pm "+action+" "+packageName)
	if err != nil {
		return "", fmt.Errorf("%s failed: %v", action, err)
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) InstallApp(ctx context.Context, serial, apkPath string) (string, error) {
	out, err := s.run(ctx, "-s", serial, "install", "-r", "-g", apkPath)
	if err != nil {
		return "", fmt.Errorf("install failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) InstallModule(ctx context.Context, serial, zipPath string) (string, error) {
	zipPath = sanitizePath(zipPath)
	remotePath := "/data/local/tmp/module.zip"
	if _, err := s.PushFile(ctx, serial, zipPath, remotePath); err != nil {
		return "", err
	}

	// Detect root manager and use the correct install command
	mgr, installCmd := s.detectRootManagerAndInstallCmd(ctx, serial)
	if installCmd == "" {
		return "", fmt.Errorf("no supported root manager found (tried APatch/KernelSU/Magisk)")
	}

	log.Printf("[ADB] Installing module via %s: %s", mgr, installCmd+remotePath+"'")
	out, err := s.RunShell(ctx, serial, installCmd+remotePath+"'")
	if err != nil {
		return "", fmt.Errorf("install failed (%s): %w", mgr, err)
	}

	// Cleanup temp zip
	s.RunShell(ctx, serial, "rm -f "+remotePath)

	return strings.TrimSpace(out), nil
}

