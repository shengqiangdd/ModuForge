package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)


// ─── Device Management (devices, connect, info) ───
// ─── Device Management ───

func (s *ADBService) ListDevices(ctx context.Context, userID string) ([]ADBDevice, error) {
	out, err := s.run(ctx, "devices", "-l")
	if err != nil {
		return nil, fmt.Errorf("adb devices failed: %w", err)
	}
	live := map[string]ADBDevice{}
	var order []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of") || strings.HasPrefix(line, "*") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		dev := ADBDevice{
			Serial: parts[0],
			State:  parts[1],
		}
		for _, p := range parts[2:] {
			if strings.HasPrefix(p, "model:") {
				dev.Model = strings.TrimPrefix(p, "model:")
			} else if strings.HasPrefix(p, "brand:") {
				dev.Brand = strings.TrimPrefix(p, "brand:")
			} else if strings.HasPrefix(p, "transport_id:") {
				dev.Transport = strings.TrimPrefix(p, "transport_id:")
			}
		}
		if dev.State == "device" {
			if vOut, err := s.run(ctx, "-s", dev.Serial, "shell", "getprop", "ro.build.version.release"); err == nil {
				dev.Android = strings.TrimSpace(vOut)
			}
		}
		live[dev.Serial] = dev
		order = append(order, dev.Serial)
	}

	// No authenticated user (e.g. public mirror flow): return the global list.
	if userID == "" {
		devices := make([]ADBDevice, 0, len(order))
		for _, serial := range order {
			if dev, ok := live[serial]; ok {
				devices = append(devices, dev)
			}
		}
		return devices, nil
	}

	// Per-user isolation: only devices this user has connected/saved are shown,
	// with live online state from adb devices.
	saved, err := s.GetSavedDevices(userID)
	if err != nil {
		return nil, err
	}

	// Auto-reconnect offline saved devices (throttled, background).
	// The adb server loses all wireless connections whenever the container is
	// rebuilt / adb server restarts. Without this, saved devices remain
	// "offline" until the user manually clicks connect.
	for _, sv := range saved {
		if _, ok := live[sv.Address]; !ok {
			s.reconnectDevice(ctx, sv.Address)
		}
	}

	// Give reconnects a moment to register, then re-read live state so a
	// successful reconnect is reflected in THIS response (next poll would pick
	// it up anyway, but immediate feedback is nicer).
	if len(live) < len(saved) {
		if out2, err2 := s.run(ctx, "devices", "-l"); err2 == nil {
			for _, line := range strings.Split(out2, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "List of") || strings.HasPrefix(line, "*") {
					continue
				}
				parts := strings.Fields(line)
				if len(parts) < 2 {
					continue
				}
				dev := ADBDevice{Serial: parts[0], State: parts[1]}
				for _, p := range parts[2:] {
					if strings.HasPrefix(p, "model:") {
						dev.Model = strings.TrimPrefix(p, "model:")
					} else if strings.HasPrefix(p, "brand:") {
						dev.Brand = strings.TrimPrefix(p, "brand:")
					} else if strings.HasPrefix(p, "transport_id:") {
						dev.Transport = strings.TrimPrefix(p, "transport_id:")
					}
				}
				if dev.State == "device" {
					if vOut, errV := s.run(ctx, "-s", dev.Serial, "shell", "getprop", "ro.build.version.release"); errV == nil {
						dev.Android = strings.TrimSpace(vOut)
					}
				}
				if existing, ok := live[dev.Serial]; ok {
					// Keep already-known model/brand details
					if dev.Model == "" {
						dev.Model = existing.Model
					}
					if dev.Brand == "" {
						dev.Brand = existing.Brand
					}
				}
				live[dev.Serial] = dev
			}
		}
	}

	devices := make([]ADBDevice, 0, len(saved))
	for _, sv := range saved {
		if dev, ok := live[sv.Address]; ok {
			dev.ID = sv.ID
			devices = append(devices, dev)
		} else {
			devices = append(devices, ADBDevice{ID: sv.ID, Serial: sv.Address, State: "offline"})
		}
	}
	return devices, nil
}

// reconnectDevice attempts `adb connect` for a saved device that is currently
// offline. Throttled to once per address per minute to avoid hammering the adb
// server on every poll; runs in the background so it never blocks the caller.
func (s *ADBService) reconnectDevice(ctx context.Context, address string) {
	s.reconnectMu.Lock()
	if last, ok := s.lastReconnect[address]; ok && time.Since(last) < time.Minute {
		s.reconnectMu.Unlock()
		return
	}
	s.lastReconnect[address] = time.Now()
	s.reconnectMu.Unlock()

	go func(addr string) {
		// 8s cap: wireless adb connect to a missing/unreachable host would
		// otherwise block for the full TCP timeout.
		cctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		s.run(cctx, "connect", addr)
	}(address)
}

func (s *ADBService) ConnectDevice(ctx context.Context, address string) (map[string]interface{}, error) {
	out, err := s.run(ctx, "connect", address)
	out = strings.TrimSpace(out)

	result := map[string]interface{}{
		"serial": address,
		"raw":    out,
	}

	if err != nil {
		result["status"] = "error"
		result["message"] = fmt.Sprintf("连接失败: %v", err)
		result["suggestions"] = []string{
			"请确认设备在同一网络",
			"检查IP地址和端口是否正确",
			"请确保设备已启用ADB over TCP/IP",
			"尝试: adb kill-server && adb start-server",
		}
		return result, nil
	}

	// Check device state via adb devices
	devOut, devErr := s.run(ctx, "devices", "-l")
	state := "unknown"
	serial := address
	if devErr == nil {
		for _, line := range strings.Split(devOut, "\n") {
			line = strings.TrimSpace(line)
			fields := strings.Fields(line)
			// Match the serial exactly (adb may print trailing tabs/spaces).
			// A prefix match like HasPrefix("192.168.2.2", "192.168.2.20") would
			// incorrectly pair a short address with a different device.
			if len(fields) >= 2 && fields[0] == address {
				state = fields[1]
				serial = fields[0]
				break
			}
		}
	}

	lower := strings.ToLower(out)
	result["serial"] = serial
	result["state"] = state

	// IMPORTANT: `adb connect` may print "connected to ..." even when the device
	// is actually unauthorized or offline. The authoritative source of truth is
	// the device state from `adb devices` — only state == "device" means the
	// device is really connected and usable.
	switch {
	case state == "device":
		result["status"] = "connected"
		result["message"] = "连接成功 ✅"
		result["suggestions"] = []string{}
	case state == "unauthorized" || strings.Contains(lower, "unauthorized"):
		result["status"] = "unauthorized"
		result["message"] = "设备未授权。请在手机上开启：设置 → 开发者选项 → USB调试 → 允许调试"
		result["suggestions"] = []string{
			"检查设备是否已开启USB调试",
			"如果通过TCP/IP连接，设备之前必须通过USB授权过",
			"Android 11+：前往 设置 → 开发者选项 → 无线调试，使用配对码连接",
			"尝试重启ADB服务: adb kill-server && adb start-server",
			"检查本机ADB密钥 (~/.android/adbkey) 是否被设备授权",
		}
	case state == "offline":
		result["status"] = "offline"
		result["message"] = "设备离线。请检查手机网络连接，或重启USB调试"
		result["suggestions"] = []string{
			"尝试重新连接设备",
			"检查设备的网络连接",
			"重启ADB服务: adb kill-server && adb start-server",
		}
	case strings.Contains(lower, "connected") || strings.Contains(lower, "already connected"):
		// adb claims connected, but the device list says otherwise — not usable
		result["status"] = "error"
		result["message"] = fmt.Sprintf("ADB 报告已连接，但设备实际状态为 %s，未真正连上", state)
		result["suggestions"] = []string{
			"设备可能未授权：请在手机上允许 USB 调试",
			"尝试重启ADB服务: adb kill-server && adb start-server",
			"Android 11+ 可使用配对：设置 → 开发者选项 → 无线调试",
		}
	case strings.Contains(lower, "failed") || strings.Contains(lower, "refused"):
		result["status"] = "error"
		result["message"] = out
		result["suggestions"] = []string{
			"请确认设备在同一网络",
			"检查IP地址和端口是否正确",
			"确保设备已启用ADB over TCP/IP (adb tcpip 5555)",
			"尝试: adb kill-server && adb start-server",
		}
	default:
		result["status"] = "error"
		result["message"] = fmt.Sprintf("连接失败: %s (设备状态: %s)", out, state)
		result["suggestions"] = []string{
			"请确认设备在同一网络",
			"检查IP地址和端口是否正确",
			"尝试: adb kill-server && adb start-server",
		}
	}

	return result, nil
}

func (s *ADBService) PairDevice(ctx context.Context, address, code string) (map[string]interface{}, error) {
	out, err := s.run(ctx, "pair", address, code)
	out = strings.TrimSpace(out)

	result := map[string]interface{}{
		"serial": address,
		"raw":    out,
	}

	if err != nil {
		result["status"] = "error"
		result["message"] = fmt.Sprintf("配对失败: %v", err)
		result["suggestions"] = []string{
			"确保设备运行Android 11+",
			"验证配对码是否正确",
			"前往 设置 → 开发者选项 → 无线调试，检查配对码和端口",
			"尝试重启ADB服务: adb kill-server && adb start-server",
		}
		return result, nil
	}

	lower := strings.ToLower(out)
	if strings.Contains(lower, "successfully paired") {
		result["status"] = "paired"
		result["message"] = "设备配对成功！现在可以使用 adb connect 连接"
		result["suggestions"] = []string{
			fmt.Sprintf("运行: adb connect %s", address),
		}
	} else if strings.Contains(lower, "failed") || strings.Contains(lower, "error") {
		result["status"] = "error"
		result["message"] = out
		result["suggestions"] = []string{
			"验证配对码是否正确",
			"确保设备运行Android 11+",
			"前往 设置 → 开发者选项 → 无线调试，检查配对码和端口",
		}
	} else {
		result["status"] = "unknown"
		result["message"] = out
		result["suggestions"] = []string{}
	}

	return result, nil
}

func (s *ADBService) DiagnoseDevice(ctx context.Context, address string) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"serial": address,
	}

	// 1. Check ADB availability
	if !ADBAvailable() {
		result["status"] = "error"
		result["message"] = "ADB is not available on this system"
		result["adb_available"] = false
		result["install_hint"] = ADBInstallHint()
		result["suggestions"] = []string{ADBInstallHint()}
		return result, nil
	}
	result["adb_available"] = true
	result["adb_path"] = s.ADBPath()
	result["adb_version"] = ADBVersion()

	// 2. Check ADB server status
	serverOut, _ := s.run(ctx, "devices")
	serverRunning := !strings.Contains(serverOut, "adb server") && serverOut != ""
	result["server_running"] = serverRunning
	if !serverRunning {
		result["server_status"] = "not running"
	} else {
		result["server_status"] = "running"
	}

	// 3. Try to connect
	connectResult, _ := s.ConnectDevice(ctx, address)
	result["connect_result"] = connectResult

	// 4. Check device in device list
	devOut, _ := s.run(ctx, "devices", "-l")
	deviceFound := false
	deviceState := "not found"
	deviceSerial := address
	for _, line := range strings.Split(devOut, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, address) {
			deviceFound = true
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				deviceState = fields[1]
				deviceSerial = fields[0]
			}
			break
		}
	}
	result["device_found"] = deviceFound
	result["device_state"] = deviceState
	result["serial"] = deviceSerial

	// 5. Check ADB key files
	homeDir, _ := os.UserHomeDir()
	adbKeyPath := filepath.Join(homeDir, ".android", "adbkey")
	adbKeyPubPath := filepath.Join(homeDir, ".android", "adbkey.pub")
	keyExists := false
	if info, err := os.Stat(adbKeyPath); err == nil && !info.IsDir() {
		keyExists = true
	}
	pubKeyExists := false
	if info, err := os.Stat(adbKeyPubPath); err == nil && !info.IsDir() {
		pubKeyExists = true
	}
	result["adb_key_exists"] = keyExists
	result["adb_pub_key_exists"] = pubKeyExists

	// 6. Generate diagnosis and suggestions
	var suggestions []string
	status := "ok"
	message := "No issues detected"

	if !deviceFound {
		status = "disconnected"
		message = "Device not found in ADB device list"
		suggestions = append(suggestions,
			"Verify the device is on the same network",
			fmt.Sprintf("Try: adb connect %s", address),
			"Check if the IP address and port are correct",
		)
	} else if deviceState == "unauthorized" {
		status = "unauthorized"
		message = "Device is unauthorized"
		if !keyExists {
			suggestions = append(suggestions, "ADB RSA key not found. Connect via USB and approve USB debugging first.")
		} else {
			suggestions = append(suggestions, "ADB RSA key exists but may not be authorized on the device")
		}
		suggestions = append(suggestions,
			"Connect via USB and approve USB debugging on the device",
			"For Android 11+: Use 'adb pair' with the pairing code from Settings > Developer Options > Wireless debugging",
			"Try: adb kill-server && adb start-server",
		)
	} else if deviceState == "offline" {
		status = "offline"
		message = "Device is offline"
		suggestions = append(suggestions,
			"Check the device's network connection",
			"Try reconnecting: adb disconnect "+address+" && adb connect "+address,
			"Restart ADB server: adb kill-server && adb start-server",
		)
	} else if deviceState == "device" {
		status = "connected"
		message = "Device is connected and authorized"
	}

	result["status"] = status
	result["message"] = message
	result["suggestions"] = suggestions

	return result, nil
}

func (s *ADBService) DisconnectDevice(ctx context.Context, address string) (string, error) {
	args := []string{"disconnect"}
	if address != "" {
		args = append(args, address)
	}
	out, err := s.run(ctx, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) DisconnectAll(ctx context.Context) (string, error) {
	out, err := s.run(ctx, "disconnect")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) GetDeviceInfo(ctx context.Context, serial string) (*DeviceInfo, error) {
	info := &DeviceInfo{Serial: serial}
	shell := func(cmd string) string {
		out, err := s.run(ctx, "-s", serial, "shell", cmd)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(out)
	}

	info.Model = shell("getprop ro.product.model")
	info.Brand = shell("getprop ro.product.brand")
	info.Manufacturer = shell("getprop ro.product.manufacturer")
	info.AndroidVer = shell("getprop ro.build.version.release")
	info.SDKVer = shell("getprop ro.build.version.sdk")
	info.BuildID = shell("getprop ro.build.display.id")
	info.SecurityPatch = shell("getprop ro.build.version.security_patch")
	info.Kernel = shell("uname -r")
	info.ABI = shell("getprop ro.product.cpu.abi")

	// Root detection: each root manager has its own unique binary
	// Magisk: `magisk` command
	// KernelSU: `ksud` command
	// APatch: `apd` command
	// These are independent checks — detect whichever is present
	suExec := func(args ...string) string {
		cmdArgs := []string{"-s", serial, "shell", "su", "-c"}
		cmdArgs = append(cmdArgs, args...)
		out, err := s.run(ctx, cmdArgs...)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(out)
	}

	// Check each root manager independently (they have unique binaries)
	apatchVer := ""
	kernelsuVer := ""
	magiskVer := ""

	// APatch: `apd` binary is unique to APatch
	// Try full path first (most reliable), then short name
	out := suExec("/data/adb/ap/bin/apd --version")
	if out == "" {
		out = suExec("apd --version")
	}
	if out == "" {
		out = suExec("apd -v")
	}
	if out != "" && !strings.Contains(out, "not found") && !strings.Contains(out, "No such") {
		apatchVer = out
	}

	// KernelSU: `ksud` binary is unique to KernelSU
	out = suExec("/data/adb/ksu/bin/ksud --version")
	if out == "" {
		out = suExec("ksud --version")
	}
	if out == "" {
		out = suExec("ksud -v")
	}
	if out != "" && !strings.Contains(out, "not found") && !strings.Contains(out, "No such") {
		kernelsuVer = out
	}

	// Magisk: `magisk` command is unique to Magisk
	out = suExec("magisk -v")
	if out == "" {
		out = suExec("magisk --version")
	}
	if out != "" && !strings.Contains(out, "not found") && !strings.Contains(out, "No such") {
		magiskVer = out
	}

	// Report detected root managers (a device may have multiple)
	if apatchVer != "" {
		info.RootStatus = "rooted"
		info.RootManager = "APatch " + apatchVer
		info.APatchVer = apatchVer
		info.RootPath = "/data/adb/ap/bin/apd"
	}
	if kernelsuVer != "" {
		info.RootStatus = "rooted"
		if info.RootManager != "" {
			info.RootManager += " + KernelSU " + kernelsuVer
		} else {
			info.RootManager = "KernelSU " + kernelsuVer
		}
		info.KSUVer = kernelsuVer
		if info.RootPath == "" {
			info.RootPath = "/data/adb/ksu/bin/ksud"
		}
	}
	if magiskVer != "" {
		info.RootStatus = "rooted"
		if info.RootManager != "" {
			info.RootManager += " + Magisk " + magiskVer
		} else {
			info.RootManager = "Magisk " + magiskVer
		}
		info.MagiskVer = magiskVer
		if info.RootPath == "" {
			info.RootPath = "/data/adb/magisk"
		}
	}

	// Fallback: if no specific manager detected, check if su works at all
	if info.RootStatus == "unknown" {
		suOut := suExec("id")
		if strings.Contains(suOut, "uid=0") {
			info.RootStatus = "rooted"
			info.RootManager = "su (unknown manager)"
		} else {
			info.RootStatus = "unrooted"
		}
	}

	// Battery
	batteryOut := shell("dumpsys battery 2>/dev/null")
	for _, line := range strings.Split(batteryOut, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "level:") {
			fmt.Sscanf(strings.TrimPrefix(line, "level:"), "%d", &info.BatteryLevel)
		} else if strings.HasPrefix(line, "status:") {
			statusCode := strings.TrimSpace(strings.TrimPrefix(line, "status:"))
			switch statusCode {
			case "2":
				info.BatteryStatus = "Charging"
			case "3":
				info.BatteryStatus = "Discharging"
			case "4":
				info.BatteryStatus = "Not charging"
			case "5":
				info.BatteryStatus = "Full"
			default:
				info.BatteryStatus = "Unknown"
			}
		}
	}

	// Storage (df -k gives KB, use formatBytes for consistency)
	dfOut := shell("df -k /data 2>/dev/null | tail -1")
	parts := strings.Fields(dfOut)
	if len(parts) >= 4 {
		info.StorageTotal = formatBytes(parts[1] + " kB")
		info.StorageUsed = formatBytes(parts[2] + " kB")
		info.StorageFree = formatBytes(parts[3] + " kB")
	} else {
		// Fallback to human-readable df
		dfOut2 := shell("df -h /data 2>/dev/null | tail -1")
		parts2 := strings.Fields(dfOut2)
		if len(parts2) >= 4 {
			info.StorageTotal = parts2[1]
			info.StorageUsed = parts2[2]
			info.StorageFree = parts2[3]
		}
	}

	// Memory
	memOut := shell("cat /proc/meminfo 2>/dev/null | head -2")
	var memTotalKB, memAvailKB int64
	for _, line := range strings.Split(memOut, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "MemTotal:") {
			info.RAMTotal = formatBytes(strings.TrimSpace(strings.TrimPrefix(line, "MemTotal:")))
			val := strings.TrimSpace(strings.TrimPrefix(line, "MemTotal:"))
			val = strings.TrimSuffix(val, "kB")
			val = strings.TrimSpace(val)
			if v, err := strconv.ParseInt(val, 10, 64); err == nil {
				memTotalKB = v
			}
		} else if strings.HasPrefix(line, "MemAvailable:") {
			info.RAMFree = formatBytes(strings.TrimSpace(strings.TrimPrefix(line, "MemAvailable:")))
			val := strings.TrimSpace(strings.TrimPrefix(line, "MemAvailable:"))
			val = strings.TrimSuffix(val, "kB")
			val = strings.TrimSpace(val)
			if v, err := strconv.ParseInt(val, 10, 64); err == nil {
				memAvailKB = v
			}
		}
	}
	if memTotalKB > 0 && memAvailKB > 0 {
		info.RAMUsed = formatBytes(fmt.Sprintf("%d kB", memTotalKB-memAvailKB))
	} else {
		info.RAMUsed = info.RAMTotal
	}

	// Uptime
	uptimeOut := shell("cat /proc/uptime 2>/dev/null | awk '{print $1}'")
	info.Uptime = uptimeOut

	return info, nil
}
