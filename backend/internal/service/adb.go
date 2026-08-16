package service

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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
	"KEYCODE_BACK":            "4",
	"KEYCODE_HOME":            "3",
	"KEYCODE_POWER":           "26",
	"KEYCODE_VOLUME_UP":       "24",
	"KEYCODE_VOLUME_DOWN":     "25",
	"KEYCODE_ENTER":           "66",
	"KEYCODE_TAB":             "61",
	"KEYCODE_DEL":             "67",
	"KEYCODE_MENU":            "82",
	"KEYCODE_CAMERA":          "27",
	"KEYCODE_SEARCH":          "84",
	"KEYCODE_SETTINGS":        "176",
	"KEYCODE_APP_SWITCH":      "187",
	"KEYCODE_DPAD_UP":         "19",
	"KEYCODE_DPAD_DOWN":       "20",
	"KEYCODE_DPAD_LEFT":       "21",
	"KEYCODE_DPAD_RIGHT":      "22",
	"KEYCODE_DPAD_CENTER":     "23",
	"KEYCODE_PAGE_UP":         "92",
	"KEYCODE_PAGE_DOWN":       "93",
	"KEYCODE_MOVE_HOME":       "122",
	"KEYCODE_MOVE_END":        "123",
	"KEYCODE_WAKEUP":          "224",
	"KEYCODE_SLEEP":           "223",
	"KEYCODE_BRIGHTNESS_DOWN": "220",
	"KEYCODE_BRIGHTNESS_UP":   "221",
	// Modifier keys
	"KEYCODE_CTRL_LEFT":   "113",
	"KEYCODE_CTRL_RIGHT":  "114",
	"KEYCODE_ALT_LEFT":    "57",
	"KEYCODE_ALT_RIGHT":   "58",
	"KEYCODE_SHIFT_LEFT":  "59",
	"KEYCODE_SHIFT_RIGHT": "60",
	"KEYCODE_A":           "29", "KEYCODE_B": "30", "KEYCODE_C": "31",
	"KEYCODE_D": "32", "KEYCODE_E": "33", "KEYCODE_F": "34",
	"KEYCODE_G": "35", "KEYCODE_H": "36", "KEYCODE_I": "37",
	"KEYCODE_J": "38", "KEYCODE_K": "39", "KEYCODE_L": "40",
	"KEYCODE_M": "41", "KEYCODE_N": "42", "KEYCODE_O": "43",
	"KEYCODE_P": "44", "KEYCODE_Q": "45", "KEYCODE_R": "46",
	"KEYCODE_S": "47", "KEYCODE_T": "48", "KEYCODE_U": "49",
	"KEYCODE_V": "50", "KEYCODE_W": "51", "KEYCODE_X": "52",
	"KEYCODE_Y": "53", "KEYCODE_Z": "54",
	"KEYCODE_0": "7", "KEYCODE_1": "8", "KEYCODE_2": "9",
	"KEYCODE_3": "10", "KEYCODE_4": "11", "KEYCODE_5": "12",
	"KEYCODE_6": "13", "KEYCODE_7": "14", "KEYCODE_8": "15",
	"KEYCODE_9":             "16",
	"KEYCODE_SPACE":         "62",
	"KEYCODE_COMMA":         "55",
	"KEYCODE_PERIOD":        "56",
	"KEYCODE_SLASH":         "76",
	"KEYCODE_BACKSLASH":     "73",
	"KEYCODE_MINUS":         "69",
	"KEYCODE_EQUALS":        "70",
	"KEYCODE_LEFT_BRACKET":  "71",
	"KEYCODE_RIGHT_BRACKET": "72",
	"KEYCODE_SEMICOLON":     "74",
	"KEYCODE_APOSTROPHE":    "75",
	"KEYCODE_GRAVE":         "68",
}

// ─── Data Types ───

type ADBDevice struct {
	ID        int    `json:"id,omitempty"` // saved-device id (only for saved devices)
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

	// Auto-reconnect throttle: last attempt per saved address.
	// Container/server restarts kill the adb server, which drops every wireless
	// connection. Without auto-reconnect, saved devices stay "offline" forever
	// until the user manually clicks connect — see "设备信息空/root管理器空" bug.
	reconnectMu   sync.Mutex
	lastReconnect map[string]time.Time
}

func NewADBService(db *sql.DB) *ADBService {
	return &ADBService{db: db, lastReconnect: map[string]time.Time{}}
}

func (s *ADBService) SaveDevice(address, name, userID string) error {
	_, err := s.db.Exec(
		`INSERT INTO adb_saved_devices (address, name, user_id, last_connected_at) VALUES (?, ?, ?, datetime('now'))
		 ON CONFLICT(address) DO UPDATE SET name=?, last_connected_at=datetime('now')`,
		address, name, userID, name,
	)
	return err
}

// ReconnectAllSaved reconnects every saved device across all users.
// Called at server startup (container rebuild kills the adb server, which drops
// all wireless connections) and periodically by a background goroutine so saved
// devices stay online without the user having to click connect again.
// Reconnect calls are throttled by reconnectDevice, so calling this frequently
// is cheap.
func (s *ADBService) ReconnectAllSaved(ctx context.Context) {
	rows, err := s.db.Query(`SELECT DISTINCT address FROM adb_saved_devices`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var addr string
		if err := rows.Scan(&addr); err != nil {
			continue
		}
		if addr == "" {
			continue
		}
		s.reconnectDevice(ctx, addr)
	}
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
	cmd := exec.CommandContext(ctx, s.ADBPath(), args...)
	return cmd
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

func (s *ADBService) RunWithTimeout(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
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
		"running":      true,
		"device_count": deviceCount,
		"adb_path":     s.ADBPath(),
	}, nil
}

