package service

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─── ADB Path Resolution ───

var (
	adbPathCache string
	adbPathOnce  sync.Once
	adbVersion   string
)

// resolveADBPath finds the adb binary. Priority:
// 1. exec.LookPath (PATH search — fastest)
// 2. Common install locations
// 3. Fallback to "adb" (will fail gracefully with clear error)
func resolveADBPath() string {
	adbPathOnce.Do(func() {
		// Fast path: check PATH first
		if p, err := exec.LookPath("adb"); err == nil {
			adbPathCache = p
			return
		}
		// Slow path: check known locations
		candidates := []string{}
		if runtime.GOOS == "windows" {
			candidates = append(candidates,
				"C:\\platform-tools\\adb.exe",
				"adb.exe",
			)
		} else {
			candidates = append(candidates,
				"/usr/bin/adb",
				"/usr/local/bin/adb",
				"/opt/homebrew/bin/adb",
				"/usr/lib/android-sdk/platform-tools/adb",
				"/opt/platform-tools/adb",
			)
			if h := os.Getenv("ANDROID_HOME"); h != "" {
				candidates = append(candidates, filepath.Join(h, "platform-tools", "adb"))
			}
			if r := os.Getenv("ANDROID_SDK_ROOT"); r != "" {
				candidates = append(candidates, filepath.Join(r, "platform-tools", "adb"))
			}
		}
		for _, p := range candidates {
			if _, err := exec.LookPath(p); err == nil {
				adbPathCache = p
				return
			}
			// Also check if the file exists directly (absolute path)
			if info, err := os.Stat(p); err == nil && info.Mode()&0111 != 0 {
				adbPathCache = p
				return
			}
		}
		adbPathCache = "adb"
	})
	return adbPathCache
}

// ADBVersion returns the adb version string, or an error message if adb is not found.
func ADBVersion() string {
	p := resolveADBPath()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, p, "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	adbVersion = strings.TrimSpace(string(out))
	return adbVersion
}

// ADBAvailable reports whether adb is installed and runnable.
func ADBAvailable() bool {
	p := resolveADBPath()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, p, "version")
	out, err := cmd.CombinedOutput()
	return err == nil && strings.Contains(string(out), "Android Debug Bridge")
}

// ADBInstallHint returns a human-readable install instruction for the current OS.
func ADBInstallHint() string {
	switch runtime.GOOS {
	case "linux":
		return "apt-get install -y adb  OR  download from https://developer.android.com/tools/releases/platform-tools"
	case "darwin":
		return "brew install android-platform-tools  OR  download from https://developer.android.com/tools/releases/platform-tools"
	case "windows":
		return "download from https://developer.android.com/tools/releases/platform-tools and add to PATH"
	default:
		return "download from https://developer.android.com/tools/releases/platform-tools"
	}
}

// formatBytes converts a raw KB string (e.g. "3874560 kB") to a human-friendly form.
func formatBytes(kbStr string) string {
	kbStr = strings.TrimSpace(kbStr)
	re := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*kB$`)
	matches := re.FindStringSubmatch(kbStr)
	if len(matches) < 2 {
		re2 := regexp.MustCompile(`^(\d+(?:\.\d+)?)`)
		m2 := re2.FindStringSubmatch(kbStr)
		if len(m2) < 2 {
			return kbStr
		}
		matches = m2
	}
	kb, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return kbStr
	}
	bytes := kb * 1024
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	unitIdx := 0
	size := bytes
	for size >= 1024 && unitIdx < len(units)-1 {
		size /= 1024
		unitIdx++
	}
	if unitIdx == 0 {
		return fmt.Sprintf("%.0f %s", size, units[unitIdx])
	}
	if size >= 10 {
		return fmt.Sprintf("%.0f %s", size, units[unitIdx])
	}
	return fmt.Sprintf("%.1f %s", size, units[unitIdx])
}

// keyMap maps common key names to Android keyevent codes.
// Supports both short names (home) and KEYCODE_* format (KEYCODE_HOME).
var keyMap = map[string]string{
	// Short names
	"back":        "4",
	"home":        "3",
	"power":       "26",
	"volume_up":   "24",
	"volume_down": "25",
	"enter":       "66",
	"tab":         "61",
	"delete":      "67",
	"menu":        "82",
	"camera":      "27",
	"search":      "84",
	"settings":    "176",
	"app_switch":  "187",
	// KEYCODE_* format (frontend uses these)
	"KEYCODE_BACK":        "4",
	"KEYCODE_HOME":        "3",
	"KEYCODE_POWER":       "26",
	"KEYCODE_VOLUME_UP":   "24",
	"KEYCODE_VOLUME_DOWN": "25",
	"KEYCODE_ENTER":       "66",
	"KEYCODE_TAB":         "61",
	"KEYCODE_DEL":         "67",
	"KEYCODE_MENU":        "82",
	"KEYCODE_CAMERA":      "27",
	"KEYCODE_SEARCH":      "84",
	"KEYCODE_SETTINGS":    "176",
	"KEYCODE_APP_SWITCH":  "187",
	"KEYCODE_DPAD_UP":     "19",
	"KEYCODE_DPAD_DOWN":   "20",
	"KEYCODE_DPAD_LEFT":   "21",
	"KEYCODE_DPAD_RIGHT":  "22",
	"KEYCODE_DPAD_CENTER": "23",
	"KEYCODE_PAGE_UP":     "92",
	"KEYCODE_PAGE_DOWN":   "93",
	"KEYCODE_MOVE_HOME":   "122",
	"KEYCODE_MOVE_END":    "123",
	"KEYCODE_WAKEUP":      "224",
	"KEYCODE_SLEEP":       "223",
	"KEYCODE_BRIGHTNESS_DOWN": "220",
	"KEYCODE_BRIGHTNESS_UP":   "221",
	// Modifier keys
	"KEYCODE_CTRL_LEFT":  "113",
	"KEYCODE_CTRL_RIGHT": "114",
	"KEYCODE_ALT_LEFT":   "57",
	"KEYCODE_ALT_RIGHT":  "58",
	"KEYCODE_SHIFT_LEFT": "59",
	"KEYCODE_SHIFT_RIGHT": "60",
	"KEYCODE_A":  "29", "KEYCODE_B":  "30", "KEYCODE_C":  "31",
	"KEYCODE_D":  "32", "KEYCODE_E":  "33", "KEYCODE_F":  "34",
	"KEYCODE_G":  "35", "KEYCODE_H":  "36", "KEYCODE_I":  "37",
	"KEYCODE_J":  "38", "KEYCODE_K":  "39", "KEYCODE_L":  "40",
	"KEYCODE_M":  "41", "KEYCODE_N":  "42", "KEYCODE_O":  "43",
	"KEYCODE_P":  "44", "KEYCODE_Q":  "45", "KEYCODE_R":  "46",
	"KEYCODE_S":  "47", "KEYCODE_T":  "48", "KEYCODE_U":  "49",
	"KEYCODE_V":  "50", "KEYCODE_W":  "51", "KEYCODE_X":  "52",
	"KEYCODE_Y":  "53", "KEYCODE_Z":  "54",
	"KEYCODE_0":  "7",  "KEYCODE_1":  "8",  "KEYCODE_2":  "9",
	"KEYCODE_3":  "10", "KEYCODE_4":  "11", "KEYCODE_5":  "12",
	"KEYCODE_6":  "13", "KEYCODE_7":  "14", "KEYCODE_8":  "15",
	"KEYCODE_9":  "16",
	"KEYCODE_SPACE":      "62",
	"KEYCODE_COMMA":      "55",
	"KEYCODE_PERIOD":     "56",
	"KEYCODE_SLASH":      "76",
	"KEYCODE_BACKSLASH":  "73",
	"KEYCODE_MINUS":      "69",
	"KEYCODE_EQUALS":     "70",
	"KEYCODE_LEFT_BRACKET":  "71",
	"KEYCODE_RIGHT_BRACKET": "72",
	"KEYCODE_SEMICOLON":  "74",
	"KEYCODE_APOSTROPHE": "75",
	"KEYCODE_GRAVE":      "68",
}

// ─── Data Types ───

type ADBDevice struct {
	Serial    string `json:"serial"`
	Model     string `json:"model,omitempty"`
	Brand     string `json:"brand,omitempty"`
	State     string `json:"state"`
	Android   string `json:"android_version,omitempty"`
	Transport string `json:"transport,omitempty"`
}

type DeviceInfo struct {
	Serial        string `json:"serial"`
	Model         string `json:"model"`
	Brand         string `json:"brand"`
	Manufacturer  string `json:"manufacturer"`
	AndroidVer    string `json:"android_version"`
	SDKVer        string `json:"sdk_version"`
	BuildID       string `json:"build_id"`
	SecurityPatch string `json:"security_patch"`
	RootStatus    string `json:"root_status"`
	RootManager   string `json:"root_manager"`
	RootPath      string `json:"root_path"`
	MagiskVer     string `json:"magisk_version,omitempty"`
	KSUVer        string `json:"ksu_version,omitempty"`
	APatchVer     string `json:"apatch_version,omitempty"`
	BatteryLevel  int    `json:"battery_level"`
	BatteryStatus string `json:"battery_status"`
	StorageTotal  string `json:"storage_total"`
	StorageUsed   string `json:"storage_used"`
	StorageFree   string `json:"storage_free"`
	RAMTotal      string `json:"ram_total"`
	RAMFree       string `json:"ram_free"`
	RAMUsed       string `json:"ram_used"`
	Uptime        string `json:"uptime"`
	Kernel        string `json:"kernel"`
	ABI           string `json:"abi"`
}

type InstalledModule struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Author      string `json:"author,omitempty"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
	Size        string `json:"size,omitempty"`
	UpdateDate  string `json:"update_date,omitempty"`
	HasUpdate   bool   `json:"has_update"`
	Source      string `json:"source,omitempty"`
}

type AppInfo struct {
	PackageName string `json:"package_name"`
	VersionName string `json:"version_name"`
	VersionCode int    `json:"version_code"`
	TargetSDK   int    `json:"target_sdk"`
	MinSDK      int    `json:"min_sdk"`
	Enabled     bool   `json:"enabled"`
	System      bool   `json:"system"`
	Size        int64  `json:"size"`
	InstallTime string `json:"install_time,omitempty"`
}

type FileInfo struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Mode  string `json:"mode"`
	IsDir bool   `json:"is_dir"`
}

type SavedDevice struct {
	ID              int       `json:"id"`
	Address         string    `json:"address"`
	Name            string    `json:"name"`
	LastConnectedAt time.Time `json:"last_connected_at"`
	CreatedAt       time.Time `json:"created_at"`
}

type ADBService struct {
	db *sql.DB
}

func NewADBService(db *sql.DB) *ADBService {
	return &ADBService{db: db}
}

func (s *ADBService) SaveDevice(address, name, userID string) error {
	_, err := s.db.Exec(
		`INSERT INTO adb_saved_devices (address, name, user_id, last_connected_at) VALUES (?, ?, ?, datetime('now'))
		 ON CONFLICT(address) DO UPDATE SET name=?, last_connected_at=datetime('now')`,
		address, name, userID, name,
	)
	return err
}

func (s *ADBService) GetSavedDevices(userID string) ([]SavedDevice, error) {
	rows, err := s.db.Query(`SELECT id, address, COALESCE(name,''), last_connected_at, created_at FROM adb_saved_devices WHERE user_id=? ORDER BY last_connected_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var devices []SavedDevice
	for rows.Next() {
		var d SavedDevice
		if err := rows.Scan(&d.ID, &d.Address, &d.Name, &d.LastConnectedAt, &d.CreatedAt); err != nil {
			continue
		}
		devices = append(devices, d)
	}
	if devices == nil {
		devices = []SavedDevice{}
	}
	return devices, nil
}

func (s *ADBService) DeleteSavedDevice(id int, userID string) error {
	_, err := s.db.Exec(`DELETE FROM adb_saved_devices WHERE id=? AND user_id=?`, id, userID)
	return err
}

func (s *ADBService) GetSavedDevice(id int, userID string) (*SavedDevice, error) {
	var d SavedDevice
	err := s.db.QueryRow(`SELECT id, address, COALESCE(name,''), last_connected_at, created_at FROM adb_saved_devices WHERE id=? AND user_id=?`, id, userID).Scan(&d.ID, &d.Address, &d.Name, &d.LastConnectedAt, &d.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *ADBService) ADBPath() string {
	return resolveADBPath()
}

// CreateCommand creates an exec.Cmd for ADB with the given args, suitable for streaming stdout.
func (s *ADBService) CreateCommand(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, s.ADBPath(), args...)
}

func (s *ADBService) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, s.ADBPath(), args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("%s", errMsg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// ExecADBRaw executes ADB and returns raw bytes (no trimming, no string conversion)
func (s *ADBService) ExecADBRaw(ctx context.Context, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, s.ADBPath(), args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, stderr.Bytes(), fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

func (s *ADBService)RunWithTimeout(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return s.run(ctx, args...)
}

// ─── ADB Server Management ───

func (s *ADBService) CheckADBAvailable(ctx context.Context) bool {
	return ADBAvailable()
}

func (s *ADBService) StartServer(ctx context.Context) (string, error) {
	return s.run(ctx, "start-server")
}

func (s *ADBService) KillServer(ctx context.Context) (string, error) {
	return s.run(ctx, "kill-server")
}

func (s *ADBService) GetServerStatus(ctx context.Context) (map[string]interface{}, error) {
	out, err := s.run(ctx, "devices")
	if err != nil {
		return map[string]interface{}{
			"running": false,
			"error":   err.Error(),
		}, nil
	}
	lines := strings.Split(out, "\n")
	deviceCount := 0
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "*") {
			deviceCount++
		}
	}
	return map[string]interface{}{
		"running":       true,
		"device_count":  deviceCount,
		"adb_path":      s.ADBPath(),
	}, nil
}

// ─── Device Management ───

func (s *ADBService) ListDevices(ctx context.Context) ([]ADBDevice, error) {
	out, err := s.run(ctx, "devices", "-l")
	if err != nil {
		return nil, fmt.Errorf("adb devices failed: %w", err)
	}
	var devices []ADBDevice
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
		devices = append(devices, dev)
	}
	return devices, nil
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
			if strings.HasPrefix(line, address) || strings.Contains(line, address) {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					state = fields[1]
					serial = fields[0]
				}
				break
			}
		}
	}

	lower := strings.ToLower(out)
	result["serial"] = serial
	result["state"] = state

	switch {
	case strings.Contains(lower, "unauthorized"):
		result["status"] = "unauthorized"
		result["message"] = "设备未授权。请在手机上开启：设置 → 开发者选项 → USB调试 → 允许调试"
		result["suggestions"] = []string{
			"检查设备是否已开启USB调试",
			"如果通过TCP/IP连接，设备之前必须通过USB授权过",
			"Android 11+：前往 设置 → 开发者选项 → 无线调试，使用配对码连接",
			"尝试重启ADB服务: adb kill-server && adb start-server",
			"检查本机ADB密钥 (~/.android/adbkey) 是否被设备授权",
		}
	case strings.Contains(lower, "connected") || strings.Contains(lower, "already connected"):
		result["status"] = "connected"
		result["message"] = "连接成功 ✅"
		result["suggestions"] = []string{}
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
		if state == "device" {
			result["status"] = "connected"
			result["message"] = "设备已连接并授权 ✅"
			result["suggestions"] = []string{}
		} else if state == "offline" {
			result["status"] = "offline"
			result["message"] = "设备离线。请检查手机网络连接，或重启USB调试"
			result["suggestions"] = []string{
				"尝试重新连接设备",
				"检查设备的网络连接",
				"重启ADB服务: adb kill-server && adb start-server",
			}
		} else if state == "unauthorized" {
			result["status"] = "unauthorized"
			result["message"] = "设备未授权。请在手机上开启：设置 → 开发者选项 → USB调试 → 允许调试"
			result["suggestions"] = []string{
				"检查设备是否已开启USB调试",
				"如果通过TCP/IP连接，设备之前必须通过USB授权过",
				"Android 11+：前往 设置 → 开发者选项 → 无线调试，使用配对码连接",
				"尝试重启ADB服务: adb kill-server && adb start-server",
			}
		} else {
			result["status"] = state
			result["message"] = fmt.Sprintf("设备状态: %s", state)
			result["suggestions"] = []string{}
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
	serverRunning := !strings.Contains(serverOut, "adb server" ) && serverOut != ""
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

// ─── Shell & Exec ───

func sanitizePath(p string) string {
	p = strings.ReplaceAll(p, "'", "")
	p = strings.ReplaceAll(p, "\"", "")
	p = strings.ReplaceAll(p, ";", "")
	p = strings.ReplaceAll(p, "|", "")
	p = strings.ReplaceAll(p, "&", "")
	p = strings.ReplaceAll(p, "$", "")
	p = strings.ReplaceAll(p, "`", "")
	p = strings.ReplaceAll(p, "(", "")
	p = strings.ReplaceAll(p, ")", "")
	p = strings.ReplaceAll(p, "{", "")
	p = strings.ReplaceAll(p, "}", "")
	p = strings.ReplaceAll(p, "\n", "")
	p = strings.ReplaceAll(p, "\r", "")
	// Remove path traversal components
	var cleaned []string
	for _, part := range strings.Split(p, "/") {
		if part == ".." || part == "." {
			continue
		}
		cleaned = append(cleaned, part)
	}
	return strings.Join(cleaned, "/")
}

func (s *ADBService) RunShell(ctx context.Context, serial, shellCmd string) (string, error) {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "shell", shellCmd)
	out, err := s.run(ctx, args...)
	if err != nil {
		return "", err
	}
	// Truncate very long output
	if len(out) > 50000 {
		out = out[:50000] + "\n... (truncated)"
	}
	return out, nil
}

func (s *ADBService) RunExec(ctx context.Context, serial, shellCmd string) (string, error) {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	// Parse the command to determine if it's a host-level or shell command
	trimmed := strings.TrimSpace(shellCmd)
	hostCmds := []string{"connect", "disconnect", "devices", "start-server", "kill-server", "forward", "reverse", "usb", "tcpip", "wait-for-device", "get-state", "get-serialno", "remount", "unlock", "oem"}
	isHostCmd := false
	for _, hc := range hostCmds {
		if strings.HasPrefix(trimmed, hc+" ") || trimmed == hc {
			isHostCmd = true
			break
		}
	}
	if isHostCmd {
		args = append(args, strings.Fields(trimmed)...)
	} else {
		args = append(args, "shell", shellCmd)
	}
	out, err := s.RunWithTimeout(ctx, 30*time.Second, args...)
	if err != nil {
		return "", err
	}
	return out, nil
}

// ─── File Management ───

type cacheEntry struct {
	files     []FileInfo
	expiresAt time.Time
}

var (
	fileCache   = make(map[string]cacheEntry)
	fileCacheMu sync.RWMutex
)

func (s *ADBService) ListFiles(ctx context.Context, serial, remotePath string) ([]FileInfo, error) {
	if remotePath == "" {
		remotePath = "/sdcard/"
	}
	remotePath = sanitizePath(remotePath)
	if remotePath != "/" && !strings.HasSuffix(remotePath, "/") {
		remotePath += "/"
	}

	cacheKey := serial + ":" + remotePath
	fileCacheMu.RLock()
	if entry, ok := fileCache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		fileCacheMu.RUnlock()
		return entry.files, nil
	}
	fileCacheMu.RUnlock()

	out, err := s.RunShell(ctx, serial, "ls -la "+remotePath+" 2>/dev/null")
	if err != nil || strings.TrimSpace(out) == "" {
		out, err = s.RunShell(ctx, serial, "su -c 'ls -la "+remotePath+"' 2>/dev/null")
	}
	if err != nil {
		return nil, fmt.Errorf("list files failed: %w", err)
	}
	var files []FileInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 7 {
			continue
		}
		fi := FileInfo{
			Mode:  parts[0],
			Name:  sanitizePath(parts[len(parts)-1]),
			Path:  strings.TrimRight(remotePath, "/") + "/" + sanitizePath(parts[len(parts)-1]),
			IsDir: strings.HasPrefix(parts[0], "d"),
		}
		if !fi.IsDir && len(parts) >= 5 {
			fmt.Sscanf(parts[4], "%d", &fi.Size)
		}
		if fi.Name == "." || fi.Name == ".." {
			continue
		}
		files = append(files, fi)
	}

	// Sort: directories first, then by name (case-insensitive)
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	fileCacheMu.Lock()
	fileCache[cacheKey] = cacheEntry{files: files, expiresAt: time.Now().Add(30 * time.Second)}
	fileCacheMu.Unlock()

	return files, nil
}

func (s *ADBService) PullFile(ctx context.Context, serial, remotePath, localPath string) (string, error) {
	if localPath == "" {
		localPath = filepath.Base(remotePath)
	}
	out, err := s.run(ctx, "-s", serial, "pull", remotePath, localPath)
	if err != nil {
		return "", fmt.Errorf("pull failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) PushFile(ctx context.Context, serial, localPath, remotePath string) (string, error) {
	if remotePath == "" {
		remotePath = "/sdcard/"
	}
	out, err := s.run(ctx, "-s", serial, "push", localPath, remotePath)
	if err != nil {
		return "", fmt.Errorf("push failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) DeleteFile(ctx context.Context, serial, remotePath string) (string, error) {
	remotePath = sanitizePath(remotePath)
	out, err := s.RunShell(ctx, serial, "rm -rf "+remotePath+" && echo 'deleted'")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) MakeDir(ctx context.Context, serial, remotePath string) (string, error) {
	remotePath = sanitizePath(remotePath)
	out, err := s.RunShell(ctx, serial, "mkdir -p "+remotePath+" && echo 'created'")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) RenameFile(ctx context.Context, serial, oldPath, newPath string) (string, error) {
	oldPath = sanitizePath(oldPath)
	newPath = sanitizePath(newPath)
	out, err := s.RunShell(ctx, serial, fmt.Sprintf("mv %q %q && echo 'renamed'", oldPath, newPath))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) GetFileSize(ctx context.Context, serial, remotePath string) (string, error) {
	remotePath = sanitizePath(remotePath)
	out, err := s.RunShell(ctx, serial, "du -sh "+remotePath+" 2>/dev/null | awk '{print $1}'")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) ReadFile(ctx context.Context, serial, remotePath string) (string, error) {
	remotePath = sanitizePath(remotePath)

	// Check file size first (avoids pulling huge files)
	sizeOut, _ := s.RunShell(ctx, serial, "wc -c < '"+remotePath+"' 2>/dev/null")
	fileSize := int64(0)
	if sizeStr := strings.TrimSpace(sizeOut); sizeStr != "" {
		if sz, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
			fileSize = sz
			if fileSize > 1024*100 {
				return "(file too large to preview: " + fmt.Sprintf("%.1f", float64(fileSize)/1024) + " KB)", nil
			}
		}
	}

	// Detect binary files via `file` command (broad pattern matching)
	fileOut, _ := s.RunShell(ctx, serial, "file '"+remotePath+"' 2>/dev/null")
	fileLower := strings.ToLower(fileOut)
	binaryKeywords := []string{
		"binary", "image", "executable", "archive", "compressed",
		"data", "media", "audio", "video", "font", "pdf",
		"elf", "mach-o", "java archive", "zip", "gzip",
	}
	for _, kw := range binaryKeywords {
		if strings.Contains(fileLower, kw) {
			return "(binary file, can't preview)", nil
		}
	}

	// Fallback: check for null bytes in first 8KB (definitive binary indicator)
	// Uses od instead of grep -P for Android compatibility
	if fileSize > 0 {
		nullCheck, _ := s.RunShell(ctx, serial, "dd if='"+remotePath+"' bs=4096 count=2 2>/dev/null | od -An -tx1 2>/dev/null | grep 00 | wc -l")
		nullCount := strings.TrimSpace(nullCheck)
		if nullCount != "" && nullCount != "0" {
			return "(binary file, can't preview)", nil
		}
	}

	// Read as text
	out, err := s.RunShell(ctx, serial, "cat '"+remotePath+"' 2>/dev/null")
	if err != nil {
		return "", fmt.Errorf("read file failed: %w", err)
	}

	// Strip common non-printable control characters that cause garbled display
	// Keep newlines (\n), tabs (\t), and carriage returns (\r)
	out = strings.ReplaceAll(out, "\x00", "")
	out = strings.ReplaceAll(out, "\x1b", "") // ESC sequences
	return out, nil
}

func (s *ADBService) WriteFile(ctx context.Context, serial, remotePath, content string) (string, error) {
	remotePath = sanitizePath(remotePath)
	// Use base64 to avoid shell escaping issues
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	cmd := fmt.Sprintf("echo '%s' | base64 -d > '%s' && echo 'written'", encoded, remotePath)
	out, err := s.RunShell(ctx, serial, cmd)
	if err != nil {
		return "", fmt.Errorf("write file failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) CopyFile(ctx context.Context, serial, src, dst string) (string, error) {
	src = sanitizePath(src)
	dst = sanitizePath(dst)
	out, err := s.RunShell(ctx, serial, fmt.Sprintf("cp -r %q %q && echo 'copied'", src, dst))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) GetFileInfo(ctx context.Context, serial, remotePath string) (map[string]interface{}, error) {
	remotePath = sanitizePath(remotePath)
	statOut, err := s.RunShell(ctx, serial, "stat "+remotePath+" 2>/dev/null")
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}
	info := make(map[string]interface{})
	info["path"] = remotePath
	for _, line := range strings.Split(statOut, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Size:") {
			var sz int
			fmt.Sscanf(line, "Size: %d", &sz)
			info["size"] = sz
		} else if strings.Contains(line, "File:") {
			parts := strings.SplitN(line, "->", 2)
			if len(parts) > 1 {
				info["link_target"] = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(line, "Access:") && !strings.HasPrefix(line, "Access: (") {
			info["access_time"] = strings.TrimPrefix(line, "Access: ")
		} else if strings.HasPrefix(line, "Modify:") {
			info["modify_time"] = strings.TrimPrefix(line, "Modify: ")
		} else if strings.HasPrefix(line, "Change:") {
			info["change_time"] = strings.TrimPrefix(line, "Change: ")
		}
	}
	// Get file size human-readable
	sizeStr, _ := s.GetFileSize(ctx, serial, remotePath)
	info["size_human"] = sizeStr
	// Get mode
	modeOut, _ := s.RunShell(ctx, serial, "stat -c '%A %a' "+remotePath+" 2>/dev/null")
	if modeOut != "" {
		parts := strings.Fields(modeOut)
		if len(parts) >= 2 {
			info["mode"] = parts[0]
			info["mode_numeric"] = parts[1]
		}
	}
	return info, nil
}

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
	out, err := s.RunShell(ctx, serial, "su -c 'magisk --install-module "+remotePath+"'")
	if err != nil {
		return "", fmt.Errorf("install failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

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

	// 3. Install
	installOut, err := s.RunShell(ctx, serial, "su -c 'magisk --install-module "+remotePath+"'")
	if err != nil {
		os.Remove(tmpFile)
		return nil, fmt.Errorf("module install failed: %w", err)
	}
	result["install_output"] = strings.TrimSpace(installOut)

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
		return map[string]interface{}{
			"has_update":      updateInfo.Version != currentVersion,
			"current_version": currentVersion,
			"latest_version":  updateInfo.Version,
			"changelog":       updateInfo.Changelog,
			"download_url":    updateInfo.URL,
		}, nil
	}
	return map[string]interface{}{"has_update": false, "current_version": currentVersion}, nil
}

func (s *ADBService) getModuleBasePath(ctx context.Context, serial string) string {
	return "/data/adb/modules"
}

// ─── Root Manager (2.1-2.3) ───

func (s *ADBService) GetAvailableRootManagers(ctx context.Context, serial string) ([]map[string]string, error) {
	var managers []map[string]string

	// Helper: run command via ADB shell
	run := func(args ...string) string {
		cmdArgs := append([]string{"-s", serial, "shell"}, args...)
		out, err := s.run(context.Background(), cmdArgs...)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(out)
	}

	// ── APatch detection (apd binary is unique to APatch) ──
	// Try full path first (most reliable), then short name
	apVer := run("su", "-c", "/data/adb/ap/bin/apd --version")
	if apVer == "" {
		apVer = run("su", "-c", "apd --version")
	}
	if apVer == "" {
		apVer = run("su", "-c", "apd -v")
	}
	if apVer != "" && !strings.Contains(apVer, "not found") && !strings.Contains(apVer, "No such") {
		managers = append(managers, map[string]string{
			"name":    "APatch",
			"version": apVer,
			"path":    "/data/adb/ap/bin/apd",
		})
	} else {
		// Directory fallback only if no other manager found yet
		if len(managers) == 0 {
			if out := run("su", "-c", "ls /data/adb/ap/"); out != "" && !strings.Contains(out, "No such") {
				managers = append(managers, map[string]string{
					"name":    "APatch",
					"version": "",
					"path":    "/data/adb/ap/bin/apd",
				})
			}
		}
	}

	// ── KernelSU detection (ksud binary is unique to KernelSU) ──
	ksuVer := run("su", "-c", "ksud --version")
	if ksuVer == "" {
		ksuVer = run("su", "-c", "ksud -v")
	}
	if ksuVer != "" && !strings.Contains(ksuVer, "not found") && !strings.Contains(ksuVer, "No such") {
		managers = append(managers, map[string]string{
			"name":    "KernelSU",
			"version": ksuVer,
			"path":    "/data/adb/ksu/bin/ksud",
		})
	} else {
		// Check directory fallback
		if out := run("su", "-c", "ls /data/adb/ksu/"); out != "" && !strings.Contains(out, "No such") {
			managers = append(managers, map[string]string{
				"name":    "KernelSU",
				"version": "",
				"path":    "/data/adb/ksu/bin/ksud",
			})
		}
	}

	// ── Magisk detection (magisk binary is unique to Magisk) ──
	// Only check if NOT already detected as APatch or KernelSU
	hasApOrKsu := len(managers) > 0
	magVer := ""
	if !hasApOrKsu {
		// No APatch or KernelSU detected, check Magisk
		magVer = run("su", "-c", "magisk -v")
		if magVer == "" {
			magVer = run("su", "-c", "magisk --version")
		}
	}
	if magVer != "" && !strings.Contains(magVer, "not found") && !strings.Contains(magVer, "No such") {
		managers = append(managers, map[string]string{
			"name":    "Magisk",
			"version": magVer,
			"path":    "/data/adb/magisk",
		})
	} else if !hasApOrKsu {
		// Check directory fallback only if no other manager found
		if out := run("su", "-c", "ls /data/adb/magisk/"); out != "" && !strings.Contains(out, "No such") {
			managers = append(managers, map[string]string{
				"name":    "Magisk",
				"version": "",
				"path":    "/data/adb/magisk",
			})
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
	out, err := s.RunShell(ctx, serial, "su -c 'dumpsys package 2>/dev/null | grep -A5 \"android.permission.ROOT\" | grep \"granted=true\"'")
	if err != nil || out == "" {
		return permissions, nil
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) > 0 {
			permissions = append(permissions, map[string]string{"package": parts[0], "status": "granted"})
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
	// Universal module path — all root managers (Magisk/KernelSU/APatch) use this
	modulePaths := []string{
		"/data/adb/modules",
	}
	log.Printf("[ADB] ListInstalledModules: serial=%s, checking %d paths", serial, len(modulePaths))
	seen := make(map[string]bool)
	var modules []InstalledModule

	for _, basePath := range modulePaths {
		source := ""
		switch {
		case strings.Contains(basePath, "ksu"):
			source = "KernelSU"
		case strings.Contains(basePath, "apatch") || strings.Contains(basePath, "/ap/"):
			source = "APatch"
		default:
			source = "Magisk"
		}
		out, err := s.RunShell(ctx, serial, "ls -1 '"+basePath+"/' 2>/dev/null")
		if err != nil {
			log.Printf("[ADB] Path %s not found (err=%v), skipping", basePath, err)
			continue
		}
		names := strings.Fields(out)
		if len(names) == 0 {
			log.Printf("[ADB] Path %s found but empty, skipping", basePath)
			continue
		}
		log.Printf("[ADB] Path %s found %d entries: %v", basePath, len(names), names)
		for _, name := range names {
			name = strings.TrimSpace(name)
			name = sanitizePath(name)
			if name == "" || name == "lost+found" || seen[name] {
				continue
			}
			seen[name] = true
			mod := InstalledModule{Name: name, Source: source}

			propOut, err := s.RunShell(ctx, serial, "cat '"+basePath+"/"+name+"/module.prop' 2>/dev/null")
			if err == nil {
				log.Printf("[ADB] module.prop found for %s/%s", basePath, name)
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
			} else {
				log.Printf("[ADB] No module.prop for %s/%s (err=%v), using defaults", basePath, name, err)
			}
			disableOut, _ := s.RunShell(ctx, serial, "test -f '"+basePath+"/"+name+"/disable' && echo disabled || echo enabled")
			mod.Enabled = !strings.Contains(disableOut, "disabled")
			mod.Size = s.getModuleSize(ctx, serial, basePath+"/"+name)

			// Modification time
			dateOut, _ := s.RunShell(ctx, serial, "stat -c %y '"+basePath+"/"+name+"' 2>/dev/null | cut -d'.' -f1")
			if dateOut == "" {
				dateOut, _ = s.RunShell(ctx, serial, "ls -ld '"+basePath+"/"+name+"' 2>/dev/null | awk '{print $6, $7, $8}'")
			}
			mod.UpdateDate = strings.TrimSpace(dateOut)

			// Check for update.json
			updateCheck, _ := s.RunShell(ctx, serial, "test -f '"+basePath+"/"+name+"/update.json' && echo yes || echo no")
			mod.HasUpdate = strings.Contains(updateCheck, "yes")

			modules = append(modules, mod)
		}
	}

	// Fallback: try with su if no modules found via normal shell
	if len(modules) == 0 {
		log.Printf("[ADB] No modules found via normal shell, trying su fallback")
		for _, basePath := range modulePaths {
			escapedPath := strings.ReplaceAll(basePath, "'", "'\\''")
			out, err := s.RunShell(ctx, serial, "su -c 'ls -1 "+escapedPath+"/' 2>/dev/null")
			if err != nil || strings.TrimSpace(out) == "" {
				continue
			}
			source := "Magisk"
			switch {
			case strings.Contains(basePath, "ksu"):
				source = "KernelSU"
			case strings.Contains(basePath, "apatch") || strings.Contains(basePath, "/ap/"):
				source = "APatch"
			}
			for _, name := range strings.Split(strings.TrimSpace(out), "\n") {
				name = strings.TrimSpace(name)
				name = sanitizePath(name)
				if name == "" || name == "lost+found" || seen[name] {
					continue
				}
				seen[name] = true
				mod := InstalledModule{Name: name, Source: source}
				escapedName := strings.ReplaceAll(name, "'", "'\\''")
				propOut, _ := s.RunShell(ctx, serial, "su -c 'cat "+escapedPath+"/"+escapedName+"/module.prop' 2>/dev/null")
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
				disableOut, _ := s.RunShell(ctx, serial, "su -c 'test -f "+escapedPath+"/"+escapedName+"/disable' 2>/dev/null && echo disabled || echo enabled")
				mod.Enabled = !strings.Contains(disableOut, "disabled")
				mod.Size = s.getModuleSize(ctx, serial, basePath+"/"+name)
				modules = append(modules, mod)
			}
		}
	}

	if len(modules) == 0 {
		log.Printf("[ADB] No modules found on device %s", serial)
		return nil, nil
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

// ─── Screenshot & Screen Record ───

func (s *ADBService) Screenshot(ctx context.Context, serial, localPath string) (string, error) {
	remotePath := "/data/local/tmp/screenshot.png"
	if _, err := s.RunShell(ctx, serial, "screencap -p "+remotePath); err != nil {
		return "", fmt.Errorf("screencap failed: %w", err)
	}
	if _, err := s.run(ctx, "-s", serial, "pull", remotePath, localPath); err != nil {
		return "", fmt.Errorf("pull screenshot failed: %w", err)
	}
	s.RunShell(ctx, serial, "rm "+remotePath)
	return localPath, nil
}

// ScreenshotBase64 captures the screen and returns the PNG data as base64.
func (s *ADBService) ScreenshotBase64(ctx context.Context, serial string) (string, error) {
	cmd := exec.CommandContext(ctx, s.ADBPath(), "-s", serial, "exec-out", "screencap", "-p")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("screencap failed: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(stdout.Bytes())
	return encoded, nil
}

// GetScreenSize returns the device screen width and height.
func (s *ADBService) GetScreenSize(ctx context.Context, serial string) (int, int, error) {
	out, err := s.RunShell(ctx, serial, "wm size")
	if err != nil {
		return 0, 0, fmt.Errorf("get screen size failed: %w", err)
	}
	// Output format: "Physical size: 1080x2340" or "Override size: 1080x2340"
	// Find the last line with "x" in it
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "x") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				sizeStr := strings.TrimSpace(parts[len(parts)-1])
				dim := strings.Split(sizeStr, "x")
				if len(dim) == 2 {
					w, _ := strconv.Atoi(strings.TrimSpace(dim[0]))
					h, _ := strconv.Atoi(strings.TrimSpace(dim[1]))
					if w > 0 && h > 0 {
						return w, h, nil
					}
				}
			}
		}
	}
	return 0, 0, fmt.Errorf("cannot parse screen size from: %s", out)
}

// CaptureScreenJPEG captures the screen as raw PPM via `screencap`, resizes
// by the given scale factor, and returns JPEG bytes. This avoids the slow PNG
// compression on the device side, giving 5-10× higher throughput.
func (s *ADBService) CaptureScreenJPEG(ctx context.Context, serial string, quality, scale int) (int, int, []byte, error) {
	if quality <= 0 {
		quality = 70
	}
	if scale <= 0 {
		scale = 4
	}

	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "exec-out", "screencap")

	out, _, err := s.ExecADBRaw(ctx, args...)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("screencap failed: %w", err)
	}
	if len(out) < 20 {
		return 0, 0, nil, fmt.Errorf("screencap output too short (%d bytes)", len(out))
	}

	// Parse PPM P6 header
	width, height, headerLen, err := parsePPMHeader(out)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("PPM parse error: %w", err)
	}

	pixelData := out[headerLen:]
	expected := width * height * 3
	if len(pixelData) < expected {
		return 0, 0, nil, fmt.Errorf("PPM pixel data short: got %d, need %d", len(pixelData), expected)
	}

	// Resize by scale factor
	newW, newH := width/scale, height/scale
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	resized := make([]byte, newW*newH*3)
	scaleX := float64(width) / float64(newW)
	scaleY := float64(height) / float64(newH)

	for ny := 0; ny < newH; ny++ {
		sy := int(float64(ny) * scaleY)
		for nx := 0; nx < newW; nx++ {
			sx := int(float64(nx) * scaleX)
			srcIdx := (sy*width+sx)*3
			dstIdx := (ny*newW+nx)*3
			resized[dstIdx] = pixelData[srcIdx]
			resized[dstIdx+1] = pixelData[srcIdx+1]
			resized[dstIdx+2] = pixelData[srcIdx+2]
		}
	}

	// Build image.RGBA for jpeg encoder
	img := image.NewRGBA(image.Rect(0, 0, newW, newH))
	pix := img.Pix
	for i := 0; i < newW*newH; i++ {
		pix[i*4] = resized[i*3]
		pix[i*4+1] = resized[i*3+1]
		pix[i*4+2] = resized[i*3+2]
		pix[i*4+3] = 0xFF
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return 0, 0, nil, fmt.Errorf("JPEG encode failed: %w", err)
	}

	return newW, newH, buf.Bytes(), nil
}

// parsePPMHeader parses a P6 PPM header and returns width, height, and the
// byte offset where pixel data begins.
func parsePPMHeader(data []byte) (int, int, int, error) {
	s := bufio.NewScanner(bytes.NewReader(data))
	// Magic number
	if !s.Scan() {
		return 0, 0, 0, fmt.Errorf("empty PPM")
	}
	magic := strings.TrimSpace(s.Text())
	if magic != "P6" {
		return 0, 0, 0, fmt.Errorf("not P6: %s", magic)
	}

	// Dimensions (skip comment lines starting with #)
	width, height := 0, 0
	found := false
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			width, _ = strconv.Atoi(parts[0])
			height, _ = strconv.Atoi(parts[1])
			found = true
			break
		}
	}
	if !found || width <= 0 || height <= 0 {
		return 0, 0, 0, fmt.Errorf("cannot parse dimensions")
	}

	// Max value line
	if s.Scan() {
		// typically "255" — we just skip it
	}

	// Calculate header end offset: everything after the scanner consumed
	// the header text. We re-find the position in the raw bytes.
	headerEnd := 0
	lines := 0
	for i, b := range data {
		if b == '\n' {
			lines++
			if lines >= 3 {
				// After the 3rd newline (magic, dims, maxval) the pixel data starts
				headerEnd = i + 1
				break
			}
		}
	}

	return width, height, headerEnd, nil
}

// getTouchDevice finds the input device path for touch events.
func (s *ADBService) getTouchDevice(ctx context.Context, serial string) (string, error) {
	out, err := s.RunShell(ctx, serial, "getevent -pl 2>/dev/null | grep -B5 'ABS_MT_POSITION' | grep 'add device' | head -1 | awk '{print $NF}'")
	if err != nil || strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("no touch device found")
	}
	return strings.TrimSpace(out), nil
}

// TapScreen sends a tap event at (x, y). Uses sendevent for speed, falls back to input tap.
func (s *ADBService) TapScreen(ctx context.Context, serial string, x, y int) error {
	devicePath, err := s.getTouchDevice(ctx, serial)
	if err != nil {
		// Fallback to input tap (slower, ~700ms)
		_, err = s.RunShell(ctx, serial, fmt.Sprintf("input tap %d %d", x, y))
		return err
	}
	// Use sendevent for fast tap (~50ms)
	cmds := []string{
		fmt.Sprintf("sendevent %s 3 57 0", devicePath),
		fmt.Sprintf("sendevent %s 3 53 %d", devicePath, x),
		fmt.Sprintf("sendevent %s 3 54 %d", devicePath, y),
		fmt.Sprintf("sendevent %s 3 48 5", devicePath),
		fmt.Sprintf("sendevent %s 3 58 50", devicePath),
		fmt.Sprintf("sendevent %s 0 2 0", devicePath),
		fmt.Sprintf("sendevent %s 0 0 0", devicePath),
		fmt.Sprintf("sendevent %s 3 57 -1", devicePath),
		fmt.Sprintf("sendevent %s 0 2 0", devicePath),
		fmt.Sprintf("sendevent %s 0 0 0", devicePath),
	}
	fullCmd := strings.Join(cmds, " && ")
	_, err = s.RunShell(ctx, serial, fullCmd)
	return err
}

// SwipeScreen sends a swipe/drag event.
func (s *ADBService) SwipeScreen(ctx context.Context, serial string, x1, y1, x2, y2, duration int) error {
	if duration <= 0 {
		duration = 300
	}
	_, err := s.RunShell(ctx, serial, fmt.Sprintf("input swipe %d %d %d %d %d", x1, y1, x2, y2, duration))
	return err
}

// SendTap uses input tap for reliable touch response
func (s *ADBService) SendTap(ctx context.Context, serial string, x, y int) error {
	// Try `input tap` first (works on most devices)
	_, err := s.RunShell(ctx, serial, fmt.Sprintf("input tap %d %d", x, y))
	if err != nil {
		log.Printf("[ADB] input tap failed (%v), retrying with input touchscreen tap", err)
		_, err = s.RunShell(ctx, serial, fmt.Sprintf("input touchscreen tap %d %d", x, y))
	}
	return err
}

// SendLongPress uses input swipe (same point) for long press
func (s *ADBService) SendLongPress(ctx context.Context, serial string, x, y, durationMs int) error {
	if durationMs <= 0 {
		durationMs = 800
	}
	_, err := s.RunShell(ctx, serial, fmt.Sprintf("input swipe %d %d %d %d %d", x, y, x, y, durationMs))
	return err
}

// SendSwipe uses input swipe for smooth gesture
func (s *ADBService) SendSwipe(ctx context.Context, serial string, x1, y1, x2, y2, durationMs int) error {
	if durationMs <= 0 {
		durationMs = 300
	}
	_, err := s.RunShell(ctx, serial, fmt.Sprintf("input swipe %d %d %d %d %d", x1, y1, x2, y2, durationMs))
	return err
}

// SendPinch approximates pinch with two sequential swipes
func (s *ADBService) SendPinch(ctx context.Context, serial string, x1, y1, x2, y2, durationMs int) error {
	if durationMs <= 0 {
		durationMs = 300
	}
	// Approximate pinch as two swipes from center outward
	midX := (x1 + x2) / 2
	midY := (y1 + y2) / 2
	// First swipe (finger 1)
	_, err := s.RunShell(ctx, serial, fmt.Sprintf("input swipe %d %d %d %d %d", midX, midY, x1, y1, durationMs))
	if err != nil {
		return err
	}
	// Second swipe (finger 2)
	_, err = s.RunShell(ctx, serial, fmt.Sprintf("input swipe %d %d %d %d %d", midX, midY, x2, y2, durationMs))
	return err
}

// InputText sends text input.
func (s *ADBService) InputText(ctx context.Context, serial, text string) error {
	// Android's input text command: spaces must be encoded as %s
	// and single quotes / special chars need shell escaping
	var buf strings.Builder
	for _, ch := range text {
		switch ch {
		case ' ':
			buf.WriteString("%s")
		case '\'':
			buf.WriteString("'\\''")
		case '&', '<', '>', '|', ';', '(', ')', '$', '`', '"', '\\':
			buf.WriteByte('\\')
			buf.WriteRune(ch)
		default:
			buf.WriteRune(ch)
		}
	}
	escaped := buf.String()
	_, err := s.RunShell(ctx, serial, "input text '"+escaped+"'")
	return err
}

// KeyEvent sends an Android key event.
func (s *ADBService) KeyEvent(ctx context.Context, serial, key string) error {
	code, ok := keyMap[key]
	if !ok {
		return fmt.Errorf("unknown key: %s", key)
	}
	_, err := s.RunShell(ctx, serial, "input keyevent "+code)
	return err
}

func (s *ADBService) ScreenRecord(ctx context.Context, serial, localPath, duration string) (string, error) {
	remotePath := "/data/local/tmp/record.mp4"
	if duration == "" {
		duration = "10"
	}
	if _, err := s.RunShell(ctx, serial, "screenrecord --time-limit "+duration+" "+remotePath); err != nil {
		return "", fmt.Errorf("screenrecord failed: %w", err)
	}
	if _, err := s.run(ctx, "-s", serial, "pull", remotePath, localPath); err != nil {
		return "", fmt.Errorf("pull recording failed: %w", err)
	}
	s.RunShell(ctx, serial, "rm "+remotePath)
	return localPath, nil
}

type flusher interface {
	Flush() error
}

func (s *ADBService) StreamScreen(ctx context.Context, serial string, fps int, writer io.Writer) error {
	if fps <= 0 {
		fps = 2
	}
	if fps > 10 {
		fps = 10
	}
	interval := time.Duration(1000/fps) * time.Millisecond
	boundary := "MJPEG_BOUNDARY"
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		cmd := exec.CommandContext(ctx, s.ADBPath(), "-s", serial, "exec-out", "screencap", "-p")
		imgData, err := cmd.Output()
		if err != nil {
			time.Sleep(interval)
			continue
		}
		header := fmt.Sprintf("--%s\r\nContent-Type: image/png\r\nContent-Length: %d\r\n\r\n", boundary, len(imgData))
		writer.Write([]byte(header))
		writer.Write(imgData)
		writer.Write([]byte("\r\n"))
		if f, ok := writer.(flusher); ok {
			f.Flush()
		}
		time.Sleep(interval)
	}
}

// ─── Log Viewer ───

func (s *ADBService) GetLogcat(ctx context.Context, serial, filter, level string, lines int) (string, error) {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "logcat", "-d", "-v", "threadtime")
	if lines > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", lines))
	}
	if level != "" {
		args = append(args, "*:"+strings.ToUpper(level))
	}
	if filter != "" {
		args = append(args, filter)
	}
	out, err := s.run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("logcat failed: %w", err)
	}
	return out, nil
}

func (s *ADBService) ClearLogcat(ctx context.Context, serial string) (string, error) {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "logcat", "-c")
	_, err := s.run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("clear logcat failed: %w", err)
	}
	return "Log cleared", nil
}

// ─── Device Operations ───

func (s *ADBService) RebootDevice(ctx context.Context, serial, mode string) error {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "reboot")
	if mode != "" {
		args = append(args, mode)
	}
	_, err := s.run(ctx, args...)
	return err
}

func (s *ADBService) GetProp(ctx context.Context, serial, prop string) (string, error) {
	return s.RunShell(ctx, serial, "getprop "+prop)
}

func (s *ADBService) SetProp(ctx context.Context, serial, prop, value string) (string, error) {
	return s.RunShell(ctx, serial, fmt.Sprintf("setprop %s %q", prop, value))
}

func (s *ADBService) BenchmarkDevice(ctx context.Context, serial string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	if out, err := s.RunShell(ctx, serial, "cat /proc/cpuinfo | grep 'model name' | head -1"); err == nil {
		result["cpu"] = strings.TrimSpace(out)
	}
	if out, err := s.RunShell(ctx, serial, "cat /proc/meminfo | head -3"); err == nil {
		result["memory"] = strings.TrimSpace(out)
	}
	if out, err := s.RunShell(ctx, serial, "dd if=/dev/zero of=/data/local/tmp/bench bs=1M count=10 2>&1 && rm /data/local/tmp/bench"); err == nil {
		result["storage_write"] = strings.TrimSpace(out)
	}
	if out, err := s.RunShell(ctx, serial, "getprop ro.hardware.chipname || getprop ro.board.platform"); err == nil {
		result["chipset"] = strings.TrimSpace(out)
	}
	if out, err := s.RunShell(ctx, serial, "cat /proc/uptime"); err == nil {
		result["uptime"] = strings.TrimSpace(out)
	}
	return result, nil
}

// ScreenshotJPEG captures screen and returns raw PNG bytes (screencap -p output).
// The caller (screen_ws.go) sends these bytes directly as binary WebSocket frames.
// Android screencap outputs PNG natively — we skip JPEG conversion to avoid CPU overhead.
func (s *ADBService) ScreenshotJPEG(ctx context.Context, serial string, quality int) ([]byte, error) {
	return s.ScreenshotRaw(ctx, serial)
}

// ScreenshotRaw captures screen and returns raw image bytes (PNG from screencap -p).
func (s *ADBService) ScreenshotRaw(ctx context.Context, serial string) ([]byte, error) {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "exec-out", "screencap", "-p")

	// Execute ADB and capture raw bytes (not string)
	cmd := exec.CommandContext(ctx, s.ADBPath(), args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}
