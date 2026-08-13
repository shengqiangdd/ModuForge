// Package skills provides device_test — push, install, and verify a module
// on a real Android device via ADB.
package skills

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/agent/registry"
	"github.com/moduforge/backend/internal/service"
)

// Register in register.go via registry.RegisterFactory("device_test", ...)

// ──────────────────────────────────────────────────────────
// Types
// ──────────────────────────────────────────────────────────

type DeviceTestSkill struct {
	db          *sql.DB
	storagePath string
}

func NewDeviceTestSkill(db *sql.DB, storagePath string) *DeviceTestSkill {
	return &DeviceTestSkill{db: db, storagePath: storagePath}
}

func (s *DeviceTestSkill) Name() string        { return "device_test" }
func (s *DeviceTestSkill) Description() string {
	return "Push a built module to an Android device via ADB, install it through the detected root manager (Magisk/KernelSU/APatch), and verify installation and service status. Use after build_module succeeds to test on real hardware."
}

// Metadata declares this skill has side effects (installs on device).
func (s *DeviceTestSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{ReadOnly: false, Essential: false, Core: false, NeedsDB: true}
}

// ──────────────────────────────────────────────────────────
// Execute
// ──────────────────────────────────────────────────────────

func (s *DeviceTestSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	// ── Parse inputs ──
	projectID := fmt.Sprintf("%v", input["project_id"])
	if projectID == "" {
		return "", fmt.Errorf("project_id is required")
	}
	deviceSerial := fmt.Sprintf("%v", input["device_serial"]) // optional
	action := fmt.Sprintf("%v", input["action"])               // install|verify|logs|full (default: full)
	if action == "" {
		action = "full"
	}
	logLines := 100
	if v, ok := input["log_lines"]; ok {
		if n, ok := v.(float64); ok {
			logLines = int(n)
		}
	}

	log.Printf("[device_test] project=%s device=%s action=%s", projectID, deviceSerial, action)

	// ── Create ADB service ──
	adb := service.NewADBService(s.db)

	// ── Auto-detect device if not specified ──
	if deviceSerial == "" {
		devices, err := adb.ListDevices(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list ADB devices: %w", err)
		}
		connected := []service.ADBDevice{}
		for _, d := range devices {
			if d.State == "device" {
				connected = append(connected, d)
			}
		}
		if len(connected) == 0 {
			return "", fmt.Errorf("no authorized ADB device connected. Connect a device first via the ADB panel or specify device_serial")
		}
		deviceSerial = connected[0].Serial
		log.Printf("[device_test] auto-detected device: %s (%s)", deviceSerial, connected[0].Model)
	}

	// ── Check device is authorized ──
	deviceInfo, err := adb.GetDeviceInfo(ctx, deviceSerial)
	if err != nil {
		return "", fmt.Errorf("cannot get device info for %s: %w (is the device authorized?)", deviceSerial, err)
	}
	_ = deviceInfo

	// ── Find output.zip ──
	projectDir := filepath.Join(s.storagePath, projectID)
	outputZIP := filepath.Join(projectDir, "output.zip")

	if _, err := os.Stat(outputZIP); os.IsNotExist(err) {
		// Try to find any .zip in the project dir
		zips, _ := filepath.Glob(filepath.Join(projectDir, "*.zip"))
		if len(zips) > 0 {
			outputZIP = zips[0]
			log.Printf("[device_test] output.zip not found, using %s", filepath.Base(outputZIP))
		} else {
			return "", fmt.Errorf("no build artifact found in %s. Run build_module first", projectDir)
		}
	}

	// ── Read module.prop to get module name ──
	moduleName := readModulePropField(filepath.Join(projectDir, "module.prop"), "id")
	if moduleName == "" {
		moduleName = projectID // fallback to project ID
	}
	moduleVersion := readModulePropField(filepath.Join(projectDir, "module.prop"), "version")
	moduleDesc := readModulePropField(filepath.Join(projectDir, "module.prop"), "description")

	var result strings.Builder
	result.WriteString(fmt.Sprintf("📱 **Device Test: %s**\n", moduleName))
	result.WriteString(fmt.Sprintf("• Version: %s\n", moduleVersion))
	result.WriteString(fmt.Sprintf("• Description: %s\n", moduleDesc))
	result.WriteString(fmt.Sprintf("• Device: %s\n", deviceSerial))
	result.WriteString(fmt.Sprintf("• Artifact: %s (%.1f KB)\n", filepath.Base(outputZIP), float64(fileSize(outputZIP))/1024))
	result.WriteString("\n")

	// ── Phase 1: Push & Install ──
	if action == "full" || action == "install" {
		result.WriteString("## Phase 1: Push & Install\n\n")

		// Detect root manager first
		managers, _ := adb.GetAvailableRootManagers(ctx, deviceSerial)
		mgr := ""
		if len(managers) > 0 {
			mgr = managers[0]["name"]
		}
		if mgr == "" {
			result.WriteString("⚠️ **No root manager detected** (tried Magisk/KernelSU/APatch)\n")
			result.WriteString("Cannot install module without root. Options:\n")
			result.WriteString("1. Install Magisk/KernelSU/APatch on the device\n")
			result.WriteString("2. Use `action=verify` to check files only\n")
			if action == "install" {
				return result.String(), nil
			}
		} else {
			mgrVer := managers[0]["version"]
			result.WriteString(fmt.Sprintf("Root manager: **%s** %s ✓\n\n", mgr, mgrVer))
		}

		// Push and install
		pushResult, err := adb.PushBuildModule(ctx, deviceSerial, outputZIP, moduleName, mgr != "")
		if err != nil {
			result.WriteString(fmt.Sprintf("❌ Push/install failed: %v\n", err))
			return result.String(), nil
		}

		if rootMgr, ok := pushResult["root_manager"]; ok {
			result.WriteString(fmt.Sprintf("✅ Installed via **%s**\n", rootMgr))
		}
		if installOut, ok := pushResult["install_output"]; ok {
			out := fmt.Sprintf("%v", installOut)
			if out != "" {
				result.WriteString(fmt.Sprintf("```\n%s\n```\n", out))
			}
		}
		result.WriteString("\n")
	}

	// ── Phase 2: Verify files ──
	if action == "full" || action == "verify" {
		result.WriteString("## Phase 2: Verify Installation\n\n")

		modulePaths := []string{
			"/data/adb/modules/" + moduleName,
			"/data/adb/ksu/modules/" + moduleName,
			"/data/adb/apatch/modules/" + moduleName,
		}

		installedPath := ""
		for _, p := range modulePaths {
			out, err := adb.RunShell(ctx, deviceSerial, fmt.Sprintf("ls -la %s/module.prop 2>/dev/null", p))
			if err == nil && strings.Contains(out, "module.prop") {
				installedPath = p
				break
			}
		}

		if installedPath == "" {
			result.WriteString("❌ Module not found in any module directory\n")
			result.WriteString("Checked:\n")
			for _, p := range modulePaths {
				result.WriteString(fmt.Sprintf("  • `%s`\n", p))
			}
		} else {
			result.WriteString(fmt.Sprintf("✅ Module installed at `%s`\n\n", installedPath))

			// List module files
			fileList, err := adb.RunShell(ctx, deviceSerial, fmt.Sprintf("find %s -type f | head -30", installedPath))
			if err == nil && fileList != "" {
				result.WriteString("**Module files:**\n")
				for _, line := range strings.Split(strings.TrimSpace(fileList), "\n") {
					line = strings.TrimPrefix(line, installedPath+"/")
					if line != "" {
						result.WriteString(fmt.Sprintf("  • `%s`\n", line))
					}
				}
				result.WriteString("\n")
			}

			// Check service.sh exists
			serviceCheck, _ := adb.RunShell(ctx, deviceSerial, fmt.Sprintf("cat %s/service.sh 2>/dev/null | head -5", installedPath))
			if serviceCheck != "" {
				result.WriteString("**service.sh** (first 5 lines):\n")
				result.WriteString(fmt.Sprintf("```\n%s\n```\n\n", serviceCheck))
			}
		}
	}

	// ── Phase 3: Service status ──
	if action == "full" {
		result.WriteString("## Phase 3: Service Status\n\n")

		// Try to find the daemon binary in common locations
		binaryPaths := []string{
			"/data/adb/modules/" + moduleName + "/system/bin/",
			"/data/adb/modules/" + moduleName + "/system/xbin/",
			"/data/adb/modules/" + moduleName + "/bin/",
		}

		daemonFound := false
		for _, binDir := range binaryPaths {
			bins, err := adb.RunShell(ctx, deviceSerial, fmt.Sprintf("ls %s 2>/dev/null", binDir))
			if err == nil && strings.TrimSpace(bins) != "" {
				result.WriteString(fmt.Sprintf("**Binaries in `%s`:**\n", binDir))
				for _, bin := range strings.Split(strings.TrimSpace(bins), "\n") {
					bin = strings.TrimSpace(bin)
					if bin == "" {
						continue
					}
					fullPath := binDir + bin

					// Check if running
					psOut, _ := adb.RunShell(ctx, deviceSerial, fmt.Sprintf("pgrep -f %s 2>/dev/null || echo not_running", bin))
					psOut = strings.TrimSpace(psOut)
					if psOut == "not_running" || psOut == "" {
						result.WriteString(fmt.Sprintf("  • `%s` — ⚠️ not running\n", bin))
					} else {
						result.WriteString(fmt.Sprintf("  • `%s` — ✅ running (PID: %s)\n", bin, psOut))
						daemonFound = true
					}
				}
				result.WriteString("\n")
			}
		}

		if !daemonFound {
			result.WriteString("⚠️ No daemon process detected. The service may need a reboot to start.\n")
			result.WriteString("To reboot: call `device_test` with `action=reboot`, or manually `adb reboot`.\n\n")

			// Try to manually start service.sh
			serviceScript := "/data/adb/modules/" + moduleName + "/service.sh"
			startCheck, _ := adb.RunShell(ctx, deviceSerial, fmt.Sprintf("test -f %s && echo exists || echo missing", serviceScript))
			if strings.TrimSpace(startCheck) == "exists" {
				result.WriteString("**Attempting to start service.sh manually...**\n")
				startOut, err := adb.RunShell(ctx, deviceSerial, fmt.Sprintf("su -c 'sh %s &'", serviceScript))
				if err != nil {
					result.WriteString(fmt.Sprintf("Start failed: %v\n", err))
				} else {
					if startOut != "" {
						result.WriteString(fmt.Sprintf("```\n%s\n```\n", strings.TrimSpace(startOut)))
					}
					time.Sleep(2 * time.Second)
					// Re-check
					for _, binDir := range binaryPaths {
						bins, _ := adb.RunShell(ctx, deviceSerial, fmt.Sprintf("ls %s 2>/dev/null", binDir))
						for _, bin := range strings.Split(strings.TrimSpace(bins), "\n") {
							bin = strings.TrimSpace(bin)
							if bin == "" {
								continue
							}
							psOut, _ := adb.RunShell(ctx, deviceSerial, fmt.Sprintf("pgrep -f %s 2>/dev/null || echo not_running", bin))
							psOut = strings.TrimSpace(psOut)
							if psOut != "not_running" && psOut != "" {
								result.WriteString(fmt.Sprintf("✅ `%s` is now running (PID: %s)\n", bin, psOut))
							}
						}
					}
				}
				result.WriteString("\n")
			}
		}
	}

	// ── Phase 4: Logs ──
	if action == "full" || action == "logs" {
		result.WriteString("## Phase 4: Device Logs\n\n")

		// Get logcat filtered for the module
		logcat, err := adb.GetLogcat(ctx, deviceSerial, moduleName, "", logLines)
		if err != nil {
			result.WriteString(fmt.Sprintf("Failed to get logcat: %v\n", err))
		} else if logcat != "" {
			// Truncate if too long
			lines := strings.Split(logcat, "\n")
			if len(lines) > 50 {
				logcat = strings.Join(lines[len(lines)-50:], "\n")
				result.WriteString(fmt.Sprintf("(showing last 50 of %d lines)\n", len(lines)))
			}
			result.WriteString(fmt.Sprintf("```\n%s\n```\n", logcat))
		} else {
			result.WriteString("No logcat entries for this module tag.\n")
			result.WriteString("Tip: Your daemon should log via `logcat -s ModuleName:V` to appear here.\n")
		}
		result.WriteString("\n")
	}

	// ── Phase 5: Reboot (if requested) ──
	if action == "reboot" {
		result.WriteString("## Rebooting Device\n\n")
		err := adb.RebootDevice(ctx, deviceSerial, "")
		if err != nil {
			result.WriteString(fmt.Sprintf("❌ Reboot failed: %v\n", err))
		} else {
			result.WriteString("✅ Device is rebooting. Wait ~30 seconds, then call `device_test` with `action=verify` to check after boot.\n")
		}
	}

	// ── Summary ──
	result.WriteString("---\n")
	result.WriteString(fmt.Sprintf("Test completed at %s\n", time.Now().Format("15:04:05")))

	return result.String(), nil
}

// ──────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────

func readModulePropField(path, field string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, field+"=") {
			return strings.Trim(strings.TrimPrefix(line, field+"="), "\"'")
		}
	}
	return ""
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
