package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ─── Shared-memory struct (written by C++ monitor + Rust engine) ───────────

type Feature struct {
	CPULoadVar       float64
	TempRateCelsius  float64
	FPSJitter        float64
	RemainingBattery float64
	Temperature      float64
	CurrentRefreshHz uint32
	Reserved         [2]uint32
}

type MonitorPoint struct {
	TimestampMs    uint64
	TemperatureX10 int32
	FPSX100        int32
	PowerMw        int32
	RefreshHz      uint32
	Flags          uint32
}

// ─── Response types (match WebUI expectations exactly) ─────────────────────

// StatusResp wraps everything the WebUI dashboard needs on init / poll.
type StatusResp struct {
	Stats    StatsData     `json:"stats"`
	Policies []interface{} `json:"policies"`
	Version  string        `json:"version"`
	Log      []string      `json:"log"`
}

type StatsData struct {
	CPUUsage     float64 `json:"cpu_usage"`
	CPUFreq      int     `json:"cpu_freq"`
	Temp         float64 `json:"temp"`
	MemUsage     float64 `json:"mem_usage"`
	DiskIO       float64 `json:"disk_io"`
	NetRx        float64 `json:"net_rx"`
	NetTx        float64 `json:"net_tx"`
	FPS          float64 `json:"fps"`
	FPSrc        string  `json:"fps_src,omitempty"`
	Cores        int     `json:"cores"`
	CPUFreqMax   int     `json:"cpu_freq_max"`
	TempThreshold float64 `json:"temp_threshold"`
	MemUsed      int64   `json:"mem_used"`
	MemTotal     int64   `json:"mem_total"`
	RefreshHz    uint32  `json:"refresh_hz"`
	PowerMw      int32   `json:"power_mw"`
	Battery      float64 `json:"battery"`
	Scene        string  `json:"scene"`
	Uptime       string  `json:"uptime"`
	EngineStatus string  `json:"engine_status"`
}

type DeviceResp struct {
	Model            string `json:"model"`
	Brand            string `json:"brand"`
	Soc              string `json:"soc"`
	AndroidVersion   string `json:"android_version"`
	Android          string `json:"android"`
	ABI              string `json:"abi"`
	ModuleVersion    string `json:"module_version"`
	Kernel           string `json:"kernel"`
	MemTotalMB       int64  `json:"mem_total_mb"`
	ScreenResolution string `json:"screen_resolution"`
	ScreenDPI        int    `json:"screen_dpi"`
	Uptime           int64  `json:"uptime"`
	MaxRefreshHz     uint32 `json:"max_refresh_hz"`
}

type EnergyResp struct {
	BatteryLevel    int     `json:"battery_level"`
	BatteryTemp     float64 `json:"battery_temp"`
	BatteryHealth   string  `json:"battery_health"`
	ChargeStatus    string  `json:"charge_status"`
	BatteryVoltage  int     `json:"battery_voltage"`
	BatteryCurrent  int     `json:"battery_current"`
	PowerEstimateMw int32   `json:"power_estimate_mw"`
	CPUmW           float64 `json:"cpu_mw,omitempty"`
	GPUmW           float64 `json:"gpu_mw,omitempty"`
	TotalmW         float64 `json:"total_mw,omitempty"`
	Algorithm       string  `json:"algorithm,omitempty"`
	BatteryPct      float64 `json:"battery_pct,omitempty"`
}

type ThermalZonesResp struct {
	CPU  float64 `json:"cpu"`
	GPU  float64 `json:"gpu"`
	Batt float64 `json:"batt"`
}

type LinUCBResp struct {
	Status  string `json:"status"`
	Arms    int    `json:"arms"`
	Alpha   string `json:"alpha"`
	Dim     int    `json:"dim"`
	Updates string `json:"updates"`
	Version string `json:"version"`
}

type EnergyRankResp struct {
	Entries      []EnergyRankEntry `json:"entries"`
	AvgEEI       float64           `json:"avg_eei"`
	TotalPowerMw float64           `json:"total_power_mw"`
}

type EnergyRankEntry struct {
	AppName    string  `json:"app_name"`
	PackageName string `json:"package_name"`
	PowerMw    float64 `json:"power_mw"`
	Samples    int     `json:"samples"`
}

type AppEnergyHistEntry struct {
	AppName     string  `json:"app_name"`
	PackageName string  `json:"package_name"`
	TotalMWh    float64 `json:"total_mwh"`
	Samples     int     `json:"samples"`
	LastSeen    int64   `json:"last_seen"`
}

type HistoryEntry struct {
	TimestampMs    uint64
	TemperatureX10 int32
	FPSX100        int32
	PowerMw        int32
	RefreshHz      uint32
}

type StreamData struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type SceneOverride struct {
	Scene string `json:"scene"`
}

type PolicyCreateReq struct {
	PackageName string `json:"package_name"`
	AppName     string `json:"app_name"`
	Strategy    string `json:"strategy"`
	CPULimit    int    `json:"cpu_limit"`
	GPULimit    int    `json:"gpu_limit"`
	BigCoreCount int   `json:"big_core_count"`
	BgFreezeMs  int    `json:"bg_freeze_ms"`
	RefreshHz   int    `json:"refresh_hz"`
}

type PolicyUpdateReq struct {
	ID           string `json:"id"`
	PackageName  string `json:"package_name"`
	AppName      string `json:"app_name"`
	Strategy     string `json:"strategy"`
	CPULimit     int    `json:"cpu_limit"`
	GPULimit     int    `json:"gpu_limit"`
	BigCoreCount int    `json:"big_core_count"`
	BgFreezeMs   int    `json:"bg_freeze_ms"`
	RefreshHz    int    `json:"refresh_hz"`
}

// ─── Global state ──────────────────────────────────────────────────────────

var (
	shmData    []byte
	configPath string
	modulePath string
	startTime  = time.Now()
	ringHist   []HistoryEntry
	ringMu     sync.RWMutex
	lastScene  string
	sceneMu    sync.RWMutex
	policyMu   sync.RWMutex
	policies   []map[string]interface{}
)

const VERSION = "2.3.2"

// ─── Enhanced log streaming ────────────────────────────────────────────────

type logBroadcaster struct {
	mu      sync.RWMutex
	clients map[chan string]struct{}
}

var logBroadcast = &logBroadcaster{
	clients: make(map[chan string]struct{}),
}

func (lb *logBroadcaster) subscribe() chan string {
	ch := make(chan string, 64)
	lb.mu.Lock()
	lb.clients[ch] = struct{}{}
	lb.mu.Unlock()
	return ch
}

func (lb *logBroadcaster) unsubscribe(ch chan string) {
	lb.mu.Lock()
	delete(lb.clients, ch)
	lb.mu.Unlock()
	close(ch)
}

func (lb *logBroadcaster) broadcast(msg string) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	for ch := range lb.clients {
		select {
		case ch <- msg:
		default:
			// Drop if client is too slow (backpressure)
		}
	}
}

// ─── System health score ───────────────────────────────────────────────────

type HealthScore struct {
	Score      int     `json:"score"`       // 0-100
	Grade      string  `json:"grade"`       // A/B/C/D/F
	CPU        float64 `json:"cpu"`         // 0-100
	Temp       float64 `json:"temp"`        // 0-100 (inverted: lower=better)
	Memory     float64 `json:"memory"`      // 0-100 (inverted: lower=used=better)
	Battery    float64 `json:"battery"`     // 0-100
	BatteryTmp float64 `json:"battery_temp"` // °C
	UptimeH    float64 `json:"uptime_hours"`
	Procs      int     `json:"processes"`
	LoadAvg1   float64 `json:"load_avg_1m"`
	Details    string  `json:"details"`
}

func computeHealthScore() HealthScore {
	feat := readFeature()
	cpuUsage := math.Min(feat.CPULoadVar*100, 100)
	temp := feat.Temperature
	if t := readFloat("/sys/class/thermal/thermal_zone0/temp"); t > 0 {
		temp = t / 1000.0
	}
	memUsed, memTotal := readMemInfo()
	memPct := memUsagePercent(memUsed, memTotal)
	batteryPct := feat.RemainingBattery * 100
	batteryTemp := readFloat("/sys/class/power_supply/battery/temp") / 10.0
	uptimeSec := time.Since(startTime).Seconds()

	// Count processes
	procCount := 0
	if dirs, err := filepath.Glob("/proc/[0-9]*"); err == nil {
		procCount = len(dirs)
	}

	// Load average
	loadAvg := 0.0
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 {
			loadAvg, _ = strconv.ParseFloat(fields[0], 64)
		}
	}

	// Weighted scoring (lower is better for cpu/temp/mem)
	cpuScore := math.Max(0, 100-cpuUsage)
	tempScore := math.Max(0, 100-math.Max(0, temp-30)*5) // 30°C=100, 50°C=0
	memScore := math.Max(0, 100-memPct)
	batScore := batteryPct

	// Weighted composite
	score := cpuScore*0.30 + tempScore*0.25 + memScore*0.25 + batScore*0.20
	scoreInt := int(math.Round(math.Min(100, math.Max(0, score))))

	grade := "F"
	switch {
	case scoreInt >= 90:
		grade = "A+"
	case scoreInt >= 80:
		grade = "A"
	case scoreInt >= 70:
		grade = "B"
	case scoreInt >= 60:
		grade = "C"
	case scoreInt >= 50:
		grade = "D"
	}

	details := fmt.Sprintf("CPU %.0f%% · 温度 %.1f°C · 内存 %.0f%% · 电量 %.0f%%",
		cpuUsage, temp, memPct, batteryPct)

	return HealthScore{
		Score:      scoreInt,
		Grade:      grade,
		CPU:        cpuScore,
		Temp:       tempScore,
		Memory:     memScore,
		Battery:    batScore,
		BatteryTmp: batteryTemp,
		UptimeH:    uptimeSec / 3600,
		Procs:      procCount,
		LoadAvg1:   loadAvg,
		Details:    details,
	}
}

// ─── Preset profiles ──────────────────────────────────────────────────────

type PresetProfile struct {
	Name        string                 `json:"name"`
	Icon        string                 `json:"icon"`
	Description string                 `json:"description"`
	Settings    map[string]interface{} `json:"settings"`
}

var presetProfiles = map[string]PresetProfile{
	"gaming": {
		Name:        "游戏模式",
		Icon:        "🎮",
		Description: "满血调度 · 极致流畅 · 高刷新率",
		Settings: map[string]interface{}{
			"cpu_governor":       "performance",
			"gpu_governor":       "performance",
			"refresh_max":        165,
			"thermal_threshold":  48,
			"io_dirty_ratio":     40,
			"touch_boost":        true,
			"bg_freeze":          true,
			"vm_swappiness":      10,
		},
	},
	"balanced": {
		Name:        "均衡模式",
		Icon:        "⚖️",
		Description: "性能功耗平衡 · 智能调度",
		Settings: map[string]interface{}{
			"cpu_governor":       "schedutil",
			"gpu_governor":       "msm-adreno-tz",
			"refresh_max":        120,
			"thermal_threshold":  43,
			"io_dirty_ratio":     50,
			"touch_boost":        false,
			"bg_freeze":          false,
			"vm_swappiness":      60,
		},
	},
	"powersave": {
		Name:        "省电模式",
		Icon:        "🔋",
		Description: "低功耗 · 长续航 · 降频限帧",
		Settings: map[string]interface{}{
			"cpu_governor":       "powersave",
			"gpu_governor":       "powersave",
			"refresh_max":        60,
			"thermal_threshold":  40,
			"io_dirty_ratio":     60,
			"touch_boost":        false,
			"bg_freeze":          true,
			"vm_swappiness":      80,
		},
	},
	"ultra": {
		Name:        "极致性能",
		Icon:        "🚀",
		Description: "全核心满频 · GPU超频 · 极限散热",
		Settings: map[string]interface{}{
			"cpu_governor":       "performance",
			"gpu_governor":       "performance",
			"refresh_max":        165,
			"thermal_threshold":  52,
			"io_dirty_ratio":     30,
			"touch_boost":        true,
			"bg_freeze":          true,
			"vm_swappiness":      5,
		},
	},
}

func applyPreset(name string) error {
	preset, ok := presetProfiles[name]
	if !ok {
		return fmt.Errorf("unknown preset: %s", name)
	}

	// Apply CPU governor
	if gov, ok := preset.Settings["cpu_governor"].(string); ok {
		cpuPath := detectCPUNode()
		if cpuPath != "" {
			writeSystemValue(cpuPath, gov)
		}
	}

	// Apply GPU governor
	if gov, ok := preset.Settings["gpu_governor"].(string); ok {
		gpuPath := detectGPUNode()
		if gpuPath != "" {
			writeSystemValue(gpuPath, gov)
		}
	}

	// Apply thermal threshold
	if thresh, ok := preset.Settings["thermal_threshold"].(float64); ok {
		cfg := readConfig()
		thermal, _ := cfg["thermal"].([]interface{})
		if len(thermal) > 0 {
			if t, ok := thermal[0].(map[string]interface{}); ok {
				t["temp"] = thresh
			}
		}
		writeConfig(cfg)
	}

	// Apply refresh rate
	if hz, ok := preset.Settings["refresh_max"].(float64); ok {
		cfg := readConfig()
		setNestedValue(cfg, []string{"scaling", "refresh_max"}, hz)
		writeConfig(cfg)
	}

	// Apply VM swappiness
	if sw, ok := preset.Settings["vm_swappiness"].(float64); ok {
		writeSystemValue("/proc/sys/vm/swappiness", strconv.Itoa(int(sw)))
	}

	// Apply IO dirty ratio
	if dr, ok := preset.Settings["io_dirty_ratio"].(float64); ok {
		cfg := readConfig()
		setNestedValue(cfg, []string{"io", "dirty_ratio"}, dr)
		writeConfig(cfg)
	}

	// Store current preset
	cfg := readConfig()
	cfg["current_preset"] = name
	writeConfig(cfg)

	log.Printf("Preset applied: %s (%s)", name, preset.Name)
	return nil
}

// ─── Process info ──────────────────────────────────────────────────────────

type ProcessInfo struct {
	PID    int     `json:"pid"`
	Name   string  `json:"name"`
	CPUPct float64 `json:"cpu_pct"`
	MemPct float64 `json:"mem_pct"`
	RSS    int64   `json:"rss_kb"`
	State  string  `json:"state"`
}

func getTopProcesses(limit int) []ProcessInfo {
	// Read /proc/stat for total CPU time
	totalCPU1, cpuTimes1 := readAllCPUTimes()
	time.Sleep(100 * time.Millisecond)
	totalCPU2, cpuTimes2 := readAllCPUTimes()
	totalDelta := totalCPU2 - totalCPU1
	if totalDelta == 0 {
		totalDelta = 1
	}

	var procs []ProcessInfo
	dirs, _ := filepath.Glob("/proc/[0-9]*")
	numCPU := runtime.NumCPU()

	for _, d := range dirs {
		pidStr := filepath.Base(d)
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		// Read process name
		name := ""
		if data, err := os.ReadFile(filepath.Join(d, "comm")); err == nil {
			name = strings.TrimSpace(string(data))
			// Sanitize: only printable ASCII
			name = sanitizeProcessName(name)
		}
		if name == "" {
			continue
		}

		// Skip kernel threads
		if strings.HasPrefix(name, "[") {
			continue
		}

		// Read process state
		state := "?"
		if data, err := os.ReadFile(filepath.Join(d, "stat")); err == nil {
			fields := strings.Fields(string(data))
			if len(fields) > 2 {
				state = fields[2]
			}
		}

		// Read RSS from /proc/[pid]/stat (field 24, in pages)
		rssKB := int64(0)
		if data, err := os.ReadFile(filepath.Join(d, "stat")); err == nil {
			fields := strings.Fields(string(data))
			if len(fields) > 23 {
				rssPages, _ := strconv.ParseInt(fields[23], 10, 64)
				rssKB = rssPages * 4 // typical page size
			}
		}

		// Calculate CPU% from /proc/[pid]/stat
		cpuPct := 0.0
		if t1, ok := cpuTimes1[pid]; ok {
			if t2, ok := cpuTimes2[pid]; ok {
				delta := (t2 - t1)
				cpuPct = float64(delta) / float64(totalDelta) * float64(numCPU) * 100.0
			}
		}

		// Memory percentage
		var memTotal int64
		_, memTotal = readMemInfo()
		memPct := 0.0
		if memTotal > 0 {
			memPct = float64(rssKB) / float64(memTotal/1024) * 100.0
		}

		if cpuPct > 0.1 || memPct > 0.1 {
			procs = append(procs, ProcessInfo{
				PID:    pid,
				Name:   name,
				CPUPct: math.Round(cpuPct*10) / 10,
				MemPct: math.Round(memPct*10) / 10,
				RSS:    rssKB,
				State:  state,
			})
		}
	}

	// Sort by CPU% descending (simple insertion sort for small N)
	for i := 1; i < len(procs); i++ {
		key := procs[i]
		j := i - 1
		for j >= 0 && procs[j].CPUPct+procs[j].MemPct < key.CPUPct+key.MemPct {
			procs[j+1] = procs[j]
			j--
		}
		procs[j+1] = key
	}

	if len(procs) > limit {
		procs = procs[:limit]
	}
	return procs
}

func sanitizeProcessName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 32 && r < 127 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func readAllCPUTimes() (uint64, map[int]uint64) {
	total := uint64(0)
	procTimes := make(map[int]uint64)

	// Total CPU
	if data, err := os.ReadFile("/proc/stat"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "cpu ") {
				fields := strings.Fields(line)
				for i := 1; i < len(fields); i++ {
					v, _ := strconv.ParseUint(fields[i], 10, 64)
					total += v
				}
				break
			}
		}
	}

	// Per-process CPU time
	dirs, _ := filepath.Glob("/proc/[0-9]*")
	for _, d := range dirs {
		pidStr := filepath.Base(d)
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		if data, err := os.ReadFile(filepath.Join(d, "stat")); err == nil {
			fields := strings.Fields(string(data))
			if len(fields) > 21 {
				utime, _ := strconv.ParseUint(fields[13], 10, 64)
				stime, _ := strconv.ParseUint(fields[14], 10, 64)
				cutime, _ := strconv.ParseUint(fields[15], 10, 64)
				cstime, _ := strconv.ParseUint(fields[16], 10, 64)
				procTimes[pid] = utime + stime + cutime + cstime
			}
		}
	}

	return total, procTimes
}

// ─── Network stats ─────────────────────────────────────────────────────────

type NetIfaceStats struct {
	Name    string  `json:"name"`
	RxBytes int64   `json:"rx_bytes"`
	TxBytes int64   `json:"tx_bytes"`
	RxPkts  int64   `json:"rx_packets"`
	TxPkts  int64   `json:"tx_packets"`
	RxErrs  int64   `json:"rx_errors"`
	TxErrs  int64   `json:"tx_errors"`
	RxDrop  int64   `json:"rx_drops"`
	TxDrop  int64   `json:"tx_drops"`
	RxRate  float64 `json:"rx_rate_kbs"` // KB/s
	TxRate  float64 `json:"tx_rate_kbs"`
}

var prevNetStats map[string]NetIfaceStats
var prevNetTime time.Time
var netMu sync.Mutex

func getNetworkStats() []NetIfaceStats {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return nil
	}

	now := time.Now()
	netMu.Lock()
	defer netMu.Unlock()

	var result []NetIfaceStats
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		iface := strings.TrimSpace(parts[0])
		if iface == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 10 {
			continue
		}

		rxBytes, _ := strconv.ParseInt(fields[0], 10, 64)
		rxPkts, _ := strconv.ParseInt(fields[1], 10, 64)
		rxErrs, _ := strconv.ParseInt(fields[2], 10, 64)
		rxDrop, _ := strconv.ParseInt(fields[3], 10, 64)
		txBytes, _ := strconv.ParseInt(fields[8], 10, 64)
		txPkts, _ := strconv.ParseInt(fields[9], 10, 64)
		txErrs, _ := strconv.ParseInt(fields[10], 10, 64)
		txDrop, _ := strconv.ParseInt(fields[11], 10, 64)

		st := NetIfaceStats{
			Name:    iface,
			RxBytes: rxBytes,
			TxBytes: txBytes,
			RxPkts:  rxPkts,
			TxPkts:  txPkts,
			RxErrs:  rxErrs,
			TxErrs:  txErrs,
			RxDrop:  rxDrop,
			TxDrop:  txDrop,
		}

		// Calculate rate
		if prev, ok := prevNetStats[iface]; ok && !prevNetTime.IsZero() {
			elapsed := now.Sub(prevNetTime).Seconds()
			if elapsed > 0 {
				st.RxRate = math.Round(float64(rxBytes-prev.RxBytes)/elapsed/1024*100) / 100
				st.TxRate = math.Round(float64(txBytes-prev.TxBytes)/elapsed/1024*100) / 100
				if st.RxRate < 0 {
					st.RxRate = 0
				}
				if st.TxRate < 0 {
					st.TxRate = 0
				}
			}
		}

		result = append(result, st)
	}

	// Store for next calculation
	if prevNetStats == nil {
		prevNetStats = make(map[string]NetIfaceStats)
	}
	for _, st := range result {
		prevNetStats[st.Name] = st
	}
	prevNetTime = now

	return result
}

// ─── /api/logs/stream (SSE) ────────────────────────────────────────────────

func handleLogStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := logBroadcast.subscribe()
	defer logBroadcast.unsubscribe(ch)

	// Send last 50 lines as history
	level := r.URL.Query().Get("level")
	sendRecentLogs(w, flusher, 50, level)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			// Level filter
			if level != "" && level != "all" {
				lineUpper := strings.ToUpper(line)
				switch level {
				case "error":
					if !strings.Contains(lineUpper, "ERROR") && !strings.Contains(lineUpper, "CRITICAL") && !strings.Contains(lineUpper, "PANIC") {
						continue
					}
				case "warn":
					if !strings.Contains(lineUpper, "WARN") && !strings.Contains(lineUpper, "WARNING") {
						continue
					}
				}
			}
			fmt.Fprintf(w, "data: %s\n\n", line)
			flusher.Flush()
		}
	}
}

func sendRecentLogs(w http.ResponseWriter, flusher http.Flusher, count int, level string) {
	ld := filepath.Join(modulePath, "logs")
	files, _ := filepath.Glob(filepath.Join(ld, "*.log"))
	sort.Strings(files)

	var allLines []string
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line == "" {
				continue
			}
			if level != "" && level != "all" {
				lineUpper := strings.ToUpper(line)
				switch level {
				case "error":
					if !strings.Contains(lineUpper, "ERROR") && !strings.Contains(lineUpper, "CRITICAL") && !strings.Contains(lineUpper, "PANIC") {
						continue
					}
				case "warn":
					if !strings.Contains(lineUpper, "WARN") && !strings.Contains(lineUpper, "WARNING") {
						continue
					}
				}
			}
			allLines = append(allLines, line)
		}
	}

	start := 0
	if len(allLines) > count {
		start = len(allLines) - count
	}

	for _, line := range allLines[start:] {
		fmt.Fprintf(w, "data: %s\n\n", line)
	}
	flusher.Flush()
}

func watchLogFiles() {
	ld := filepath.Join(modulePath, "logs")
	lastModTimes := make(map[string]time.Time)

	for {
		files, _ := filepath.Glob(filepath.Join(ld, "*.log"))
		for _, f := range files {
			info, err := os.Stat(f)
			if err != nil {
				continue
			}
			lastMod, exists := lastModTimes[f]
			if !exists || info.ModTime().After(lastMod) {
				lastModTimes[f] = info.ModTime()
				if exists {
					// Read new content
					data, err := os.ReadFile(f)
					if err != nil {
						continue
					}
					lines := strings.Split(string(data), "\n")
					for _, line := range lines {
						if line != "" {
							logBroadcast.broadcast(line)
						}
					}
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
}

// ─── /api/logs/search ──────────────────────────────────────────────────────

func handleLogSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	level := r.URL.Query().Get("level")
	limitStr := r.URL.Query().Get("limit")
	limit := 200
	if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 1000 {
		limit = n
	}

	// Input validation: limit query length
	if len(query) > 200 {
		query = query[:200]
	}

	if query == "" && level == "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[],"total":0}`))
		return
	}

	ld := filepath.Join(modulePath, "logs")
	files, _ := filepath.Glob(filepath.Join(ld, "*.log"))
	sort.Strings(files)

	queryLower := strings.ToLower(query)
	var results []string

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line == "" {
				continue
			}
			// Level filter
			if level != "" && level != "all" {
				lineUpper := strings.ToUpper(line)
				switch level {
				case "error":
					if !strings.Contains(lineUpper, "ERROR") && !strings.Contains(lineUpper, "CRITICAL") && !strings.Contains(lineUpper, "PANIC") {
						continue
					}
				case "warn":
					if !strings.Contains(lineUpper, "WARN") && !strings.Contains(lineUpper, "WARNING") {
						continue
					}
				case "info":
					// Show all
				}
			}
			// Keyword search
			if query != "" && !strings.Contains(strings.ToLower(line), queryLower) {
				continue
			}
			results = append(results, line)
			if len(results) >= limit {
				break
			}
		}
		if len(results) >= limit {
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
		"total":   len(results),
	})
}

// ─── /api/system/health ────────────────────────────────────────────────────

func handleSystemHealth(w http.ResponseWriter, r *http.Request) {
	health := computeHealthScore()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// ─── /api/processes ────────────────────────────────────────────────────────

func handleProcesses(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 25
	if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 100 {
		limit = n
	}
	procs := getTopProcesses(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(procs)
}

// ─── /api/network/stats ────────────────────────────────────────────────────

func handleNetworkStats(w http.ResponseWriter, r *http.Request) {
	stats := getNetworkStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// ─── /api/presets ──────────────────────────────────────────────────────────

func handlePresets(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		cfg := readConfig()
		currentPreset := ""
		if p, ok := cfg["current_preset"].(string); ok {
			currentPreset = p
		}
		type presetResp struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Icon    string `json:"icon"`
			Desc    string `json:"description"`
			Active  bool   `json:"active"`
		}
		var list []presetResp
		for id, p := range presetProfiles {
			list = append(list, presetResp{
				ID:     id,
				Name:   p.Name,
				Icon:   p.Icon,
				Desc:   p.Description,
				Active: id == currentPreset,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"presets": list,
			"current": currentPreset,
		})
		return
	}
}

func handlePresetApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Preset string `json:"preset"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Validate preset name (alphanumeric + underscore only)
	validName := true
	for _, c := range req.Preset {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			validName = false
			break
		}
	}
	if !validName || len(req.Preset) > 20 {
		http.Error(w, `{"error":"invalid preset name"}`, 400)
		return
	}

	if err := applyPreset(req.Preset); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	shm := flag.String("shm", "", "shared memory file")
	flag.Parse()

	exe, _ := os.Executable()
	modulePath = filepath.Dir(filepath.Dir(exe))
	configPath = filepath.Join(modulePath, "data", "config.json")

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Printf("WebUI daemon %s starting on %s", VERSION, *addr)

	if *shm != "" {
		if err := openSHM(*shm); err != nil {
			log.Printf("SHM open failed: %v", err)
		} else {
			go pollSHM()
		}
	}

	// Load policies from config
	loadPoliciesFromConfig()

	// Start log file watcher for real-time streaming
	go watchLogFiles()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		os.Exit(0)
	}()

	mux := http.NewServeMux()

	// Core endpoints
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/stream", handleStream)
	mux.HandleFunc("/api/thermal", handleThermal)
	mux.HandleFunc("/api/thermal/zones", handleThermalZones)
	mux.HandleFunc("/api/energy", handleEnergy)
	mux.HandleFunc("/api/energy/rank", handleEnergyRank)
	mux.HandleFunc("/api/energy/app_history", handleAppEnergyHist)
	mux.HandleFunc("/api/scene", handleScene)
	mux.HandleFunc("/api/scene/override", handleSceneOverride)
	mux.HandleFunc("/api/linucb", handleLinucb)
	mux.HandleFunc("/api/device", handleDevice)
	mux.HandleFunc("/api/apps", handleApps)
	mux.HandleFunc("/api/history", handleHistory)
	mux.HandleFunc("/api/logs", handleLogs)
	mux.HandleFunc("/api/logs/stream", handleLogStream)
	mux.HandleFunc("/api/logs/search", handleLogSearch)
	mux.HandleFunc("/api/restart", handleRestart)
	mux.HandleFunc("/api/apply", handleApply)

	// System health & processes
	mux.HandleFunc("/api/system/health", handleSystemHealth)
	mux.HandleFunc("/api/processes", handleProcesses)
	mux.HandleFunc("/api/network/stats", handleNetworkStats)

	// Presets
	mux.HandleFunc("/api/presets", handlePresets)
	mux.HandleFunc("/api/presets/apply", handlePresetApply)

	// Policies
	mux.HandleFunc("/api/policies", handlePolicies)
	mux.HandleFunc("/api/policies/create", handlePolicyCreate)
	mux.HandleFunc("/api/policies/update", handlePolicyUpdate)
	mux.HandleFunc("/api/policies/delete/", handlePolicyDelete)

	// Config sub-endpoints (WebUI calls each separately)
	mux.HandleFunc("/api/config", handleConfig)
	mux.HandleFunc("/api/config/temperature", handleConfigTemperature)
	mux.HandleFunc("/api/config/refresh", handleConfigRefresh)
	mux.HandleFunc("/api/config/resolution", handleConfigResolution)
	mux.HandleFunc("/api/config/io", handleConfigIO)
	mux.HandleFunc("/api/config/gpu", handleConfigGPU)
	mux.HandleFunc("/api/config/touch", handleConfigTouch)
	mux.HandleFunc("/api/config/vm", handleConfigVM)
	mux.HandleFunc("/api/config/thermal_guard", handleConfigThermalGuard)
	mux.HandleFunc("/api/config/game", handleConfigGame)
	mux.HandleFunc("/api/config/cpu", handleConfigCPU)
	mux.HandleFunc("/api/config/refresh_rate", handleConfigRefreshRate)
	mux.HandleFunc("/api/config/reset", handleConfigReset)
	mux.HandleFunc("/api/config/export", handleConfigExport)
	mux.HandleFunc("/api/config/import", handleConfigImport)

	// Security headers middleware
	handler := securityHeaders(mux)

	// Static files
	wr := filepath.Join(modulePath, "webroot")
	if _, err := os.Stat(wr); err == nil {
		mux.Handle("/", http.FileServer(http.Dir(wr)))
	}
	log.Printf("Listening on %s (WebUI at %s)", *addr, wr)
	log.Fatal(http.ListenAndServe(*addr, handler))
}

// ─── Security + CORS middleware ──────────────────────────────────────────────

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS: root manager webview serves HTML from its own origin;
		// allow all origins so cross-origin fetch to 127.0.0.1:8080 works.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}

		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// ─── SHM helpers ───────────────────────────────────────────────────────────

func openSHM(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	info, _ := f.Stat()
	sz := int64(262336)
	if info.Size() < sz {
		f.Truncate(sz)
	}
	shmData, err = syscall.Mmap(int(f.Fd()), 0, int(sz), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	return err
}

func readFeature() Feature {
	var f Feature
	if len(shmData) < 56 {
		return f
	}
	f.CPULoadVar = math.Float64frombits(binary.LittleEndian.Uint64(shmData[0:8]))
	f.TempRateCelsius = math.Float64frombits(binary.LittleEndian.Uint64(shmData[8:16]))
	f.FPSJitter = math.Float64frombits(binary.LittleEndian.Uint64(shmData[16:24]))
	f.RemainingBattery = math.Float64frombits(binary.LittleEndian.Uint64(shmData[24:32]))
	f.Temperature = math.Float64frombits(binary.LittleEndian.Uint64(shmData[32:40]))
	f.CurrentRefreshHz = binary.LittleEndian.Uint32(shmData[40:44])
	return f
}

func pollSHM() {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for range t.C {
		if len(shmData) < 4 {
			continue
		}
		wIdx := binary.LittleEndian.Uint32(shmData[len(shmData)-4:])
		ringMu.Lock()
		lastTs := uint64(0)
		if len(ringHist) > 0 {
			lastTs = ringHist[len(ringHist)-1].TimestampMs
		}
		for i := uint32(0); i < wIdx; i++ {
			base := 120 + int(i%8192)*32
			if base+32 > len(shmData) {
				break
			}
			buf := shmData[base : base+32]
			ts := binary.LittleEndian.Uint64(buf[0:8])
			if ts > lastTs && ts > 0 {
				ringHist = append(ringHist, HistoryEntry{
					TimestampMs:    ts,
					TemperatureX10: int32(binary.LittleEndian.Uint32(buf[8:12])),
					FPSX100:        int32(binary.LittleEndian.Uint32(buf[12:16])),
					PowerMw:        int32(binary.LittleEndian.Uint32(buf[16:20])),
					RefreshHz:      binary.LittleEndian.Uint32(buf[20:24]),
				})
			}
		}
		if len(ringHist) > 2000 {
			ringHist = ringHist[len(ringHist)-2000:]
		}
		ringMu.Unlock()
	}
}

// ─── Scene detection (cached) ──────────────────────────────────────────────

func detectScene() string {
	sceneMu.RLock()
	s := lastScene
	sceneMu.RUnlock()
	if s != "" {
		return s
	}
	f := readFeature()
	s = "normal"
	if f.Temperature > 40 {
		s = "gaming"
	} else if f.CPULoadVar > 0.7 {
		s = "heavy_load"
	} else if f.CurrentRefreshHz >= 120 {
		s = "smooth"
	}
	sceneMu.Lock()
	lastScene = s
	sceneMu.Unlock()
	return s
}

// ─── Config helpers ────────────────────────────────────────────────────────

func getThermalTemp(cfg map[string]interface{}) float64 {
	thermal, ok := cfg["thermal"].([]interface{})
	if !ok || len(thermal) == 0 {
		return 43 // default
	}
	if first, ok := thermal[0].(map[string]interface{}); ok {
		if v, ok := first["temp"].(float64); ok {
			return v
		}
	}
	return 43
}

func readConfig() map[string]interface{} {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return map[string]interface{}{}
	}
	var cfg map[string]interface{}
	json.Unmarshal(data, &cfg)
	return cfg
}

func writeConfig(cfg map[string]interface{}) error {
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(configPath, data, 0644)
}

func getNestedFloat(cfg map[string]interface{}, keys ...string) float64 {
	val := interface{}(cfg)
	for _, k := range keys {
		m, ok := val.(map[string]interface{})
		if !ok {
			return 0
		}
		val = m[k]
	}
	if f, ok := val.(float64); ok {
		return f
	}
	return 0
}

func getNestedString(cfg map[string]interface{}, keys ...string) string {
	val := interface{}(cfg)
	for _, k := range keys {
		m, ok := val.(map[string]interface{})
		if !ok {
			return ""
		}
		val = m[k]
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

func setNestedValue(cfg map[string]interface{}, keys []string, value interface{}) {
	if len(keys) == 1 {
		cfg[keys[0]] = value
		return
	}
	m, ok := cfg[keys[0]].(map[string]interface{})
	if !ok {
		m = map[string]interface{}{}
		cfg[keys[0]] = m
	}
	setNestedValue(m, keys[1:], value)
}

func readSystemValue(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func writeSystemValue(path, value string) error {
	return os.WriteFile(path, []byte(value), 0644)
}

// ─── Policy helpers ────────────────────────────────────────────────────────

func loadPoliciesFromConfig() {
	cfg := readConfig()
	if ap, ok := cfg["app_policies"].([]interface{}); ok {
		policyMu.Lock()
		policies = make([]map[string]interface{}, 0, len(ap))
		for _, item := range ap {
			if m, ok := item.(map[string]interface{}); ok {
				policies = append(policies, m)
			}
		}
		policyMu.Unlock()
	}
}

func savePoliciesToConfig() {
	cfg := readConfig()
	policyMu.RLock()
	ap := make([]interface{}, len(policies))
	for i, p := range policies {
		ap[i] = p
	}
	policyMu.RUnlock()
	cfg["app_policies"] = ap
	writeConfig(cfg)
}

// ─── Handlers ──────────────────────────────────────────────────────────────

func handleRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	// Try multiple restart script locations
	restartScript := ""
	candidates := []string{
		filepath.Join(modulePath, "restart.sh"),
		filepath.Join(modulePath, "customize.sh"),
		"/data/adb/modules/androboost-smarttune/restart.sh",
		"/data/adb/modules/androboost-smarttune/customize.sh",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			restartScript = c
			break
		}
	}
	if restartScript == "" {
		http.Error(w, `{"ok":false,"error":"restart.sh not found"}`, 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true,"message":"Services restarting..."}`))
	// Force flush the response before restarting
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() {
		time.Sleep(1 * time.Second)
		exec.Command("sh", restartScript).Start()
		time.Sleep(3 * time.Second)
		os.Exit(0)
	}()
}

// ─── /api/status ───────────────────────────────────────────────────────────

func handleStatus(w http.ResponseWriter, r *http.Request) {
	feat := readFeature()

	// Read real system stats from /proc
	cpuFreqNow := readInt("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq")
	cpuFreqMax := readInt("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq")
	cores := runtime.NumCPU()
	memUsed, memTotal := readMemInfo()
	diskIO := readDiskIO()
	netRx, netTx := readNetIO()

	// Temperature from sysfs (prefer thermal_zone0, fallback to SHM)
	temp := feat.Temperature
	if sysTemp := readFloat("/sys/class/thermal/thermal_zone0/temp"); sysTemp > 0 {
		temp = sysTemp / 1000.0
	}

	// FPS from ring buffer
	var fps float64
	var powerMw int32
	var refreshHz = feat.CurrentRefreshHz
	ringMu.RLock()
	if len(ringHist) > 0 {
		last := ringHist[len(ringHist)-1]
		fps = float64(last.FPSX100) / 100.0
		powerMw = last.PowerMw
		if last.RefreshHz > 0 {
			refreshHz = last.RefreshHz
		}
	}
	ringMu.RUnlock()

	// CPU usage: prefer SHM feature, fallback to /proc/stat
	cpuUsage := math.Min(feat.CPULoadVar*100, 100)
	if cpuUsage <= 0 {
		cpuUsage = readCPUUsage()
	}

	stats := StatsData{
		CPUUsage:      cpuUsage,
		CPUFreq:       cpuFreqNow,
		Temp:          temp,
		MemUsage:      memUsagePercent(memUsed, memTotal),
		DiskIO:        diskIO,
		NetRx:         netRx,
		NetTx:         netTx,
		FPS:           fps,
		Cores:         cores,
		CPUFreqMax:    cpuFreqMax,
		TempThreshold: getThermalTemp(readConfig()),
		MemUsed:       memUsed,
		MemTotal:      memTotal,
		RefreshHz:     refreshHz,
		PowerMw:       powerMw,
		Battery:       feat.RemainingBattery * 100,
		Scene:         detectScene(),
		Uptime:        time.Since(startTime).Truncate(time.Second).String(),
		EngineStatus:  "running",
	}

	resp := StatusResp{
		Stats:   stats,
		Version: VERSION,
		Log:     []string{"系统初始化完成 · AndroBoost WebUI"},
	}

	// Attach policies
	policyMu.RLock()
	resp.Policies = make([]interface{}, len(policies))
	for i, p := range policies {
		resp.Policies[i] = p
	}
	policyMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ─── /api/stream (SSE) ─────────────────────────────────────────────────────

func handleStream(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-t.C:
			feat := readFeature()
			temp := feat.Temperature
			if sysTemp := readFloat("/sys/class/thermal/thermal_zone0/temp"); sysTemp > 0 {
				temp = sysTemp / 1000.0
			}
			// CPU usage: prefer SHM, fallback to /proc/stat
			cpuU := math.Min(feat.CPULoadVar*100, 100)
			if cpuU <= 0 {
				cpuU = readCPUUsage()
			}
			d := StreamData{Type: "stats", Payload: map[string]interface{}{
				"cpu_usage":  cpuU,
				"cpu_freq":   readInt("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq"),
				"temp":       temp,
				"mem_usage":  memUsagePercentFunc(),
				"disk_io":    readDiskIO(),
				"net_rx":     readNetIORx(),
				"net_tx":     readNetIOTx(),
				"battery":    feat.RemainingBattery * 100,
				"refresh_hz": feat.CurrentRefreshHz,
				"fps":        readFPS(),
				"cores":      runtime.NumCPU(),
				"cpu_freq_max": readInt("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq"),
			}}
			j, _ := json.Marshal(d)
			fmt.Fprintf(w, "data: %s\n\n", j)
			f.Flush()
		}
	}
}

// ─── /api/thermal ──────────────────────────────────────────────────────────

func handleThermal(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"current":     readFeature().Temperature,
		"throttled":   false,
		"max_temp":    45.0,
		"thermal_zone": "thermal_zone0",
	}
	if t := readFloat("/sys/class/thermal/thermal_zone0/temp"); t > 0 {
		resp["current"] = t / 1000.0
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ─── /api/thermal/zones ────────────────────────────────────────────────────

func handleThermalZones(w http.ResponseWriter, r *http.Request) {
	cpu := readFloat("/sys/class/thermal/thermal_zone0/temp") / 1000.0
	gpu := readFloat("/sys/class/thermal/thermal_zone1/temp") / 1000.0
	batt := readFloat("/sys/class/power_supply/battery/temp")
	if batt > 100 {
		batt = batt / 10.0 // some report in 0.1°C
	}
	resp := ThermalZonesResp{CPU: cpu, GPU: gpu, Batt: batt}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ─── /api/energy ───────────────────────────────────────────────────────────

func handleEnergy(w http.ResponseWriter, r *http.Request) {
	feat := readFeature()
	batteryPct := feat.RemainingBattery * 100
	batteryTemp := readFloat("/sys/class/power_supply/battery/temp") / 10.0
	batteryVoltage := readInt("/sys/class/power_supply/battery/voltage_now") / 1000 // µV → mV
	batteryCurrent := readInt("/sys/class/power_supply/battery/current_now")        // µA

	// Estimate power from SHM ring buffer
	var powerMw int32
	ringMu.RLock()
	if len(ringHist) > 0 {
		powerMw = ringHist[len(ringHist)-1].PowerMw
	}
	ringMu.RUnlock()

	resp := EnergyResp{
		BatteryLevel:    int(batteryPct),
		BatteryTemp:     batteryTemp,
		BatteryHealth:   "good",
		ChargeStatus:    readString("/sys/class/power_supply/battery/status"),
		BatteryVoltage:  batteryVoltage,
		BatteryCurrent:  batteryCurrent,
		PowerEstimateMw: powerMw,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ─── /api/energy/rank ──────────────────────────────────────────────────────

func handleEnergyRank(w http.ResponseWriter, r *http.Request) {
	// Build per-process power ranking from ring buffer + /proc
	// Aggregate power by process from ring buffer recent entries
	ringMu.RLock()
	totalPower := float64(0)
	totalSamples := 0
	for _, pt := range ringHist {
		pw := float64(pt.PowerMw)
		if pw <= 0 {
			continue
		}
		// For now, attribute all power to the current foreground process
		totalPower += pw
		totalSamples++
	}
	ringMu.RUnlock()

	// Find the foreground app
	fgPkg := ""
	if data, err := exec.Command("dumpsys", "activity", "activities").Output(); err == nil {
		s := string(data)
		// Look for "mResumedActivity" or "topResumedActivity"
		for _, line := range strings.Split(s, "\n") {
			if strings.Contains(line, "mResumedActivity") || strings.Contains(line, "topResumedActivity") {
				// Extract package name from: com.example.app/.MainActivity t123
				re := regexp.MustCompile(`(\S+)/\S+\s+t\d+`)
				if m := re.FindStringSubmatch(line); len(m) > 1 {
					fgPkg = m[1]
				}
				break
			}
		}
	}

	var entries []map[string]interface{}
	if totalSamples > 0 && fgPkg != "" {
		avgPower := totalPower / float64(totalSamples)
		fps := 0.0
		ringMu.RLock()
		if len(ringHist) > 0 {
			fps = float64(ringHist[len(ringHist)-1].FPSX100) / 100.0
		}
		ringMu.RUnlock()
		// EEI = power / fps (lower is better)
		eei := 0.0
		if fps > 0 {
			eei = avgPower / fps
			if eei > 2000 {
				eei = 2000 // clamp
			}
		}
		entries = append(entries, map[string]interface{}{
			"package_name": fgPkg,
			"app_name":     fgPkg,
			"power_mw":     math.Round(avgPower*10) / 10,
			"fps":          math.Round(fps*10) / 10,
			"eei":          math.Round(eei*100) / 100,
			"samples":      totalSamples,
		})
	}

	avgEEI := 0.0
	if len(entries) > 0 {
		if v, ok := entries[0]["eei"].(float64); ok {
			avgEEI = v
		}
	}

	resp := map[string]interface{}{
		"entries":        entries,
		"avg_eei":        math.Round(avgEEI*100) / 100,
		"total_power_mw": math.Round(totalPower/float64(totalSamples)*10) / 10,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ─── /api/energy/app_history ───────────────────────────────────────────────

func handleAppEnergyHist(w http.ResponseWriter, r *http.Request) {
	// Build cumulative per-app energy from ring buffer
	type appHist struct {
		PackageName string  `json:"package_name"`
		AppName     string  `json:"app_name"`
		TotalMwh    float64 `json:"total_mwh"`
		AvgPowerMw  float64 `json:"avg_power_mw"`
		Samples     int     `json:"samples"`
		LastSeen    int64   `json:"last_seen"`
	}

	// Find the foreground app
	fgPkg := ""
	if data, err := exec.Command("dumpsys", "activity", "activities").Output(); err == nil {
		s := string(data)
		for _, line := range strings.Split(s, "\n") {
			if strings.Contains(line, "mResumedActivity") || strings.Contains(line, "topResumedActivity") {
				re := regexp.MustCompile(`(\S+)/\S+\s+t\d+`)
				if m := re.FindStringSubmatch(line); len(m) > 1 {
					fgPkg = m[1]
				}
				break
			}
		}
	}

	var entries []appHist
	if fgPkg != "" {
		ringMu.RLock()
		totalPower := float64(0)
		totalSamples := 0
		var lastTs uint64
		for _, pt := range ringHist {
			pw := float64(pt.PowerMw)
			if pw > 0 {
				totalPower += pw
				totalSamples++
			}
			if pt.TimestampMs > lastTs {
				lastTs = pt.TimestampMs
			}
		}
		ringMu.RUnlock()

		if totalSamples > 0 {
			// Convert mW samples (100ms interval) to mWh
			totalMwh := totalPower * 0.1 / 3600.0 // mW * 0.1s / 3600 = mWh
			avgPower := totalPower / float64(totalSamples)
			entries = append(entries, appHist{
				PackageName: fgPkg,
				AppName:     fgPkg,
				TotalMwh:    math.Round(totalMwh*100) / 100,
				AvgPowerMw:  math.Round(avgPower*10) / 10,
				Samples:     totalSamples,
				LastSeen:    int64(lastTs / 1000),
			})
		}
	}

	resp := map[string]interface{}{
		"entries": entries,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ─── /api/scene ────────────────────────────────────────────────────────────

func handleScene(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"scene":      detectScene(),
		"confidence": 0.85,
	})
}

func handleSceneOverride(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req SceneOverride
	json.NewDecoder(r.Body).Decode(&req)
	sceneMu.Lock()
	lastScene = req.Scene
	sceneMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// ─── /api/linucb ───────────────────────────────────────────────────────────

func handleLinucb(w http.ResponseWriter, r *http.Request) {
	cfg := readConfig()
	linucb := cfg["linucb"]
	arms := 5
	alpha := "0.1"
	dim := 7
	if m, ok := linucb.(map[string]interface{}); ok {
		if v, ok := m["arms"].(float64); ok {
			arms = int(v)
		}
		if v, ok := m["alpha"].(float64); ok {
			alpha = fmt.Sprintf("%.2f", v)
		}
		if v, ok := m["dim"].(float64); ok {
			dim = int(v)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LinUCBResp{
		Status:  "running",
		Arms:    arms,
		Alpha:   alpha,
		Dim:     dim,
		Updates: "0",
		Version: VERSION,
	})
}

// ─── /api/device ───────────────────────────────────────────────────────────

func handleDevice(w http.ResponseWriter, r *http.Request) {
	info := DeviceResp{
		Brand:          "Android",
		AndroidVersion: "unknown",
		MaxRefreshHz:   120,
		Uptime:         int64(time.Since(startTime).Seconds()),
		ModuleVersion:  VERSION,
	}
	if data, err := os.ReadFile("/system/build.prop"); err == nil {
		c := string(data)
		info.Model = extractProp(c, "ro.product.model")
		info.Brand = extractProp(c, "ro.product.brand")
		info.AndroidVersion = extractProp(c, "ro.build.version.release")
		info.Soc = extractProp(c, "ro.hardware")
		if info.Soc == "" {
			info.Soc = extractProp(c, "ro.board.platform")
		}
		if dpi := extractProp(c, "ro.sf.lcd_density"); dpi != "" {
			info.ScreenDPI, _ = strconv.Atoi(dpi)
		}
	}
	// Aliases for WebUI compatibility
	info.Android = info.AndroidVersion
	info.ABI = runtime.GOARCH
	if abi := extractProp(readBuildProp(), "ro.product.cpu.abi"); abi != "" {
		info.ABI = abi
	} else if abi := extractProp(readBuildProp(), "ro.product.cpu.abilist"); abi != "" {
		info.ABI = strings.Split(abi, ",")[0]
	}
	if res := extractProp(readBuildProp(), "ro.sf.lcd_density"); res != "" {
		// Try to get resolution from dumpsys or /sys
	}
	if data, err := os.ReadFile("/proc/version"); err == nil {
		info.Kernel = strings.TrimSpace(string(data))
	}
	// Memory
	if _, memTotal := readMemInfo(); memTotal > 0 {
		info.MemTotalMB = memTotal / (1024 * 1024)
	}
	// Screen resolution from sysfs
	if res := readString("/sys/class/graphics/fb0/virtual_size"); res != "" {
		info.ScreenResolution = res
	} else {
		// Fallback: try wm size
		if out, err := exec.Command("wm", "size").Output(); err == nil {
			info.ScreenResolution = strings.TrimSpace(strings.TrimPrefix(string(out), "Physical size: "))
		}
	}
	// Max refresh rate
	if hz := readInt("/sys/class/graphics/fb0/msm_fb_vsync_mode"); hz > 0 {
		info.MaxRefreshHz = uint32(hz)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func readBuildProp() string {
	data, _ := os.ReadFile("/system/build.prop")
	return string(data)
}

// ─── /api/apps ─────────────────────────────────────────────────────────────

func handleApps(w http.ResponseWriter, r *http.Request) {
	seen := make(map[string]bool)
	var apps []map[string]interface{}
	dirs, _ := filepath.Glob("/proc/[0-9]*")
	for _, d := range dirs {
		data, err := os.ReadFile(filepath.Join(d, "cmdline"))
		if err != nil {
			continue
		}
		cmd := strings.ReplaceAll(string(data), "\x00", " ")
		if cmd == "" {
			continue
		}
		parts := strings.Fields(cmd)
		if len(parts) == 0 {
			continue
		}
		binPath := parts[0]
		pkgName := filepath.Base(binPath)
		// Filter out kernel threads
		if strings.HasPrefix(pkgName, "[") {
			continue
		}
		// Deduplicate by package name
		if seen[pkgName] {
			continue
		}
		seen[pkgName] = true
		isSystem := strings.HasPrefix(binPath, "/system/") || strings.HasPrefix(binPath, "/vendor/") || strings.HasPrefix(binPath, "/product/")
		// Try to get a friendly name from the APK's AndroidManifest via aapt2 or pm
		appName := pkgName
		if !isSystem {
			if out, err := exec.Command("pm", "list", "packages", "-f", pkgName).Output(); err == nil {
				// Output: package:/data/app/.../base.apk=com.example.app
				outStr := strings.TrimSpace(string(out))
				if idx := strings.LastIndex(outStr, "="); idx >= 0 {
					// Try to get label via dumpsys
				}
			}
		}
		apps = append(apps, map[string]interface{}{
			"package_name": pkgName,
			"app_name":     appName,
			"path":         binPath,
			"system":       isSystem,
		})
		if len(apps) >= 200 {
			break
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apps)
}

// ─── /api/policies ─────────────────────────────────────────────────────────

func handlePolicies(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		policyMu.RLock()
		list := make([]interface{}, len(policies))
		for i, p := range policies {
			list[i] = p
		}
		policyMu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
		return
	}
}

func handlePolicyCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req PolicyCreateReq
	json.NewDecoder(r.Body).Decode(&req)
	id := fmt.Sprintf("policy_%d", time.Now().UnixMilli())
	policy := map[string]interface{}{
		"id":            id,
		"package_name":  req.PackageName,
		"app_name":      req.AppName,
		"strategy":      req.Strategy,
		"cpu_limit":     req.CPULimit,
		"gpu_limit":     req.GPULimit,
		"big_core_count": req.BigCoreCount,
		"bg_freeze_ms":  req.BgFreezeMs,
		"refresh_hz":    req.RefreshHz,
		"explicit":      true,
		"active":        true,
	}
	policyMu.Lock()
	policies = append(policies, policy)
	policyMu.Unlock()
	savePoliciesToConfig()
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func handlePolicyUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req PolicyUpdateReq
	json.NewDecoder(r.Body).Decode(&req)
	policyMu.Lock()
	for i, p := range policies {
		if pid, _ := p["id"].(string); pid == req.ID || pid == req.PackageName {
			policies[i]["strategy"] = req.Strategy
			policies[i]["cpu_limit"] = req.CPULimit
			policies[i]["gpu_limit"] = req.GPULimit
			policies[i]["big_core_count"] = req.BigCoreCount
			policies[i]["bg_freeze_ms"] = req.BgFreezeMs
			policies[i]["refresh_hz"] = req.RefreshHz
			if req.AppName != "" {
				policies[i]["app_name"] = req.AppName
			}
			break
		}
	}
	policyMu.Unlock()
	savePoliciesToConfig()
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func handlePolicyDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "method not allowed", 405)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/policies/delete/")
	if id == "" {
		http.Error(w, "missing id", 400)
		return
	}
	policyMu.Lock()
	for i, p := range policies {
		if pid, _ := p["id"].(string); pid == id {
			policies = append(policies[:i], policies[i+1:]...)
			break
		}
	}
	policyMu.Unlock()
	savePoliciesToConfig()
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// ─── /api/history ──────────────────────────────────────────────────────────

func handleHistory(w http.ResponseWriter, r *http.Request) {
	// Return action history from ring buffer (mapped to WebUI format)
	ringMu.RLock()
	list := make([]map[string]interface{}, 0, len(ringHist))
	for _, h := range ringHist {
		scene := "normal"
		temp := float64(h.TemperatureX10) / 10.0
		if temp > 40 {
			scene = "gaming"
		}
		list = append(list, map[string]interface{}{
			"time":   h.TimestampMs / 1000,
			"action": fmt.Sprintf("温度 %.1f°C · 功耗 %dmW", temp, h.PowerMw),
			"detail": fmt.Sprintf("场景: %s · 刷新率 %dHz", scene, h.RefreshHz),
		})
	}
	ringMu.RUnlock()
	// Keep only last 200
	if len(list) > 200 {
		list = list[len(list)-200:]
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

// ─── /api/logs ─────────────────────────────────────────────────────────────

func handleLogs(w http.ResponseWriter, r *http.Request) {
	ld := filepath.Join(modulePath, "logs")
	level := r.URL.Query().Get("level")
	linesParam := r.URL.Query().Get("lines")
	maxLines := 500
	if n, err := strconv.Atoi(linesParam); err == nil && n > 0 {
		maxLines = n
	}

	var allLines []string
	files, _ := filepath.Glob(filepath.Join(ld, "*.log"))
	sort.Strings(files) // alphabetical order
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		fileLines := strings.Split(string(data), "\n")
		for _, line := range fileLines {
			if line == "" {
				continue
			}
			// Level filter
			if level != "" && level != "all" {
				lineUpper := strings.ToUpper(line)
				switch level {
				case "error":
					if !strings.Contains(lineUpper, "ERROR") && !strings.Contains(lineUpper, "CRITICAL") && !strings.Contains(lineUpper, "PANIC") {
						continue
					}
				case "warn":
					if !strings.Contains(lineUpper, "WARN") && !strings.Contains(lineUpper, "WARNING") {
						continue
					}
				case "info":
					// show all for info
				}
			}
			allLines = append(allLines, line)
		}
	}
	// Keep last N lines
	if len(allLines) > maxLines {
		allLines = allLines[len(allLines)-maxLines:]
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs": allLines,
	})
}

// ─── /api/apply ────────────────────────────────────────────────────────────

func handleApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Mode string `json:"mode"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	// Store current mode in config
	cfg := readConfig()
	cfg["current_mode"] = req.Mode
	writeConfig(cfg)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// ─── /api/config (generic) ─────────────────────────────────────────────────

func handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return
	}
	if r.Method == "POST" {
		var cfg map[string]interface{}
		json.NewDecoder(r.Body).Decode(&cfg)
		writeConfig(cfg)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
		return
	}
}

// ─── /api/config/temperature ───────────────────────────────────────────────

func handleConfigTemperature(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		cfg := readConfig()
		threshold := getThermalTemp(cfg)
		if threshold == 0 {
			threshold = 43
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"threshold": threshold,
		})
		return
	}
	if r.Method == "POST" {
		var req struct {
			Threshold float64 `json:"threshold"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		cfg := readConfig()
		// Update thermal[0].temp
		thermal, _ := cfg["thermal"].([]interface{})
		if len(thermal) > 0 {
			if t, ok := thermal[0].(map[string]interface{}); ok {
				t["temp"] = req.Threshold
			}
		}
		writeConfig(cfg)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

// ─── /api/config/refresh ───────────────────────────────────────────────────

func handleConfigRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		cfg := readConfig()
		scaling := cfg["scaling"]
		maxHz := 120
		deviceMaxHz := 120
		if m, ok := scaling.(map[string]interface{}); ok {
			if v, ok := m["refresh_max"].(float64); ok {
				maxHz = int(v)
			}
		}
		// Try to detect device max refresh rate
		if detected := detectMaxRefreshHz(); detected > 0 {
			deviceMaxHz = detected
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"max_hz":        maxHz,
			"device_max_hz": deviceMaxHz,
		})
		return
	}
	if r.Method == "POST" {
		var req struct {
			MaxHz int `json:"max_hz"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		cfg := readConfig()
		setNestedValue(cfg, []string{"scaling", "refresh_max"}, float64(req.MaxHz))
		writeConfig(cfg)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

// ─── /api/config/resolution ────────────────────────────────────────────────

func handleConfigResolution(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		cfg := readConfig()
		scale := getNestedFloat(cfg, "scaling", "resolution")
		if scale == 0 {
			scale = 100
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"scale": scale,
		})
		return
	}
	if r.Method == "POST" {
		var req struct {
			Scale float64 `json:"scale"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		cfg := readConfig()
		setNestedValue(cfg, []string{"scaling", "resolution"}, req.Scale)
		writeConfig(cfg)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

// ─── /api/config/io ────────────────────────────────────────────────────────

func handleConfigIO(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		cfg := readConfig()
		ioCfg := cfg["io"]
		dirtyRatio := 50
		if m, ok := ioCfg.(map[string]interface{}); ok {
			if v, ok := m["dirty_ratio"].(float64); ok {
				dirtyRatio = int(v)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"dirty_ratio": dirtyRatio,
		})
		return
	}
	if r.Method == "POST" {
		var req struct {
			DirtyRatio int    `json:"dirty_ratio"`
			Scheduler  string `json:"scheduler"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		cfg := readConfig()
		if req.DirtyRatio > 0 {
			setNestedValue(cfg, []string{"io", "dirty_ratio"}, float64(req.DirtyRatio))
		}
		writeConfig(cfg)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

// ─── /api/config/gpu ───────────────────────────────────────────────────────

func handleConfigGPU(w http.ResponseWriter, r *http.Request) {
	gpuPath := detectGPUNode()
	gpuCurrent := ""
	gpuUserSet := ""
	gameMode := false
	if gpuPath != "" {
		gpuCurrent = readString(gpuPath)
	}
	cfg := readConfig()
	if m, ok := cfg["gpu"].(map[string]interface{}); ok {
		if v, ok := m["user_governor"].(string); ok {
			gpuUserSet = v
		}
		if v, ok := m["game_mode"].(bool); ok {
			gameMode = v
		}
	}
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path":       gpuPath,
			"current":    gpuCurrent,
			"user_set":   gpuUserSet,
			"game_mode":  gameMode,
		})
		return
	}
	if r.Method == "POST" {
		var req struct {
			Governor string `json:"governor"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		setNestedValue(cfg, []string{"gpu", "user_governor"}, req.Governor)
		if gpuPath != "" && req.Governor != "" && req.Governor != "auto" {
			writeSystemValue(gpuPath, req.Governor)
		}
		writeConfig(cfg)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

// ─── /api/config/touch ─────────────────────────────────────────────────────

func handleConfigTouch(w http.ResponseWriter, r *http.Request) {
	touchPath := detectTouchNode()
	enabled := false
	if touchPath != "" {
		val := readString(touchPath)
		enabled = val != "" && val != "0"
	}
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path":    touchPath,
			"enabled": enabled,
		})
		return
	}
	if r.Method == "POST" {
		var req struct {
			Enabled bool `json:"enabled"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if touchPath != "" {
			if req.Enabled {
				writeSystemValue(touchPath, "1")
			} else {
				writeSystemValue(touchPath, "0")
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

// ─── /api/config/vm ────────────────────────────────────────────────────────

func handleConfigVM(w http.ResponseWriter, r *http.Request) {
	swappiness := readInt("/proc/sys/vm/swappiness")
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"swappiness": swappiness,
		})
		return
	}
	if r.Method == "POST" {
		var req struct {
			Swappiness int `json:"swappiness"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Swappiness >= 0 && req.Swappiness <= 100 {
			writeSystemValue("/proc/sys/vm/swappiness", strconv.Itoa(req.Swappiness))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

// ─── /api/config/thermal_guard ─────────────────────────────────────────────

func handleConfigThermalGuard(w http.ResponseWriter, r *http.Request) {
	cfg := readConfig()
	ratio := 80
	enabled := true
	if m, ok := cfg["thermal_guard"].(map[string]interface{}); ok {
		if v, ok := m["ratio"].(float64); ok {
			ratio = int(v)
		}
		if v, ok := m["enabled"].(bool); ok {
			enabled = v
		}
	}
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ratio":   ratio,
			"enabled": enabled,
		})
		return
	}
	if r.Method == "POST" {
		var req struct {
			Ratio int `json:"ratio"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		cfg["thermal_guard"] = map[string]interface{}{
			"ratio":   req.Ratio,
			"enabled": req.Ratio > 0,
		}
		writeConfig(cfg)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

// ─── /api/config/game ──────────────────────────────────────────────────────

func handleConfigGame(w http.ResponseWriter, r *http.Request) {
	cfg := readConfig()
	game := cfg["game"]
	if game == nil {
		game = map[string]interface{}{}
	}
	gameMap, _ := game.(map[string]interface{})
	if r.Method == "GET" {
		bypassPath := detectBypassChargingPath()
		ioScheduler := detectIOScheduler()
		ioAvailable := detectIOSchedulers()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"bypass_charging":  gameMap["bypass_charging"],
			"bypass_path":      bypassPath,
			"bypass_threshold": gameMap["bypass_threshold"],
			"battery_level":    featBatteryLevel(),
			"gpu_min_lock":     gameMap["gpu_min_lock"],
			"io_scheduler":     ioScheduler,
			"io_current":       ioScheduler,
			"io_available":     ioAvailable,
			"dnd_level":        gameMap["dnd_level"],
			"anim_scale":       gameMap["anim_scale"],
		})
		return
	}
	if r.Method == "POST" {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		for k, v := range req {
			gameMap[k] = v
		}
		cfg["game"] = gameMap
		writeConfig(cfg)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

// ─── /api/config/cpu ───────────────────────────────────────────────────────

func handleConfigCPU(w http.ResponseWriter, r *http.Request) {
	cpuPath := detectCPUNode()
	cpuCurrent := ""
	cpuUserSet := ""
	if cpuPath != "" {
		cpuCurrent = readString(cpuPath)
	}
	cfg := readConfig()
	if m, ok := cfg["cpu"].(map[string]interface{}); ok {
		if v, ok := m["user_governor"].(string); ok {
			cpuUserSet = v
		}
	}
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path":     cpuPath,
			"current":  cpuCurrent,
			"user_set": cpuUserSet,
		})
		return
	}
	if r.Method == "POST" {
		var req struct {
			Governor string `json:"governor"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		setNestedValue(cfg, []string{"cpu", "user_governor"}, req.Governor)
		writeConfig(cfg)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

// ─── /api/config/refresh_rate ──────────────────────────────────────────────

func handleConfigRefreshRate(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled": true,
		})
		return
	}
	if r.Method == "POST" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

// ─── /api/config/reset ─────────────────────────────────────────────────────

func handleConfigReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	defaultCfg := map[string]interface{}{
		"version": "1.0",
		"monitoring": map[string]interface{}{
			"interval_ms": 100,
			"enable_temp": true,
			"enable_fps":  true,
			"enable_power": true,
		},
		"scaling": map[string]interface{}{
			"resolution":  100,
			"refresh_min": 60,
			"refresh_max": 165,
			"ltpo":        true,
		},
		"thermal": []interface{}{
			map[string]interface{}{"temp": 43, "action": "light"},
			map[string]interface{}{"temp": 46, "action": "moderate"},
			map[string]interface{}{"temp": 48, "action": "big_core_off"},
			map[string]interface{}{"temp": 52, "action": "force30hz"},
		},
		"io": map[string]interface{}{
			"foreground":   "mq-deadline",
			"idle":         "bfq",
			"dirty_ratio":  50,
			"dirty_expire": 6000,
		},
		"linucb": map[string]interface{}{
			"alpha":   0.1,
			"dim":     7,
			"arms":    5,
			"explore": 0.3,
		},
	}
	writeConfig(defaultCfg)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// ─── /api/config/export ────────────────────────────────────────────────────

func handleConfigExport(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// ─── /api/config/import ────────────────────────────────────────────────────

func handleConfigImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var cfg map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	writeConfig(cfg)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// ─── System helpers ────────────────────────────────────────────────────────

func readInt(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	s := strings.TrimSpace(string(data))
	v, _ := strconv.Atoi(s)
	return v
}

func readFloat(path string) float64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	s := strings.TrimSpace(string(data))
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func readString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func extractProp(content, key string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `=(.*)$`)
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func readMemInfo() (used int64, total int64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseInt(fields[1], 10, 64)
		val *= 1024 // kB → B
		switch fields[0] {
		case "MemTotal:":
			total = val
		case "MemAvailable:":
			used = total - val
		}
	}
	return
}

// readCPUUsage reads CPU usage from /proc/stat (fallback when SHM unavailable)
var prevIdle, prevTotal uint64

func readCPUUsage() float64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0
		}
		// fields[1]=user [2]=nice [3]=system [4]=idle [5]=iowait [6]=irq [7]=softirq
		var vals [8]uint64
		for i := 1; i <= 8 && i < len(fields); i++ {
			vals[i-1], _ = strconv.ParseUint(fields[i], 10, 64)
		}
		idle := vals[3] + vals[4] // idle + iowait
		total := uint64(0)
		for _, v := range vals {
			total += v
		}
		dIdle := idle - prevIdle
		dTotal := total - prevTotal
		prevIdle = idle
		prevTotal = total
		if dTotal == 0 {
			return 0
		}
		return float64(dTotal-dIdle) / float64(dTotal) * 100.0
	}
	return 0
}

func memUsagePercent(used, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}

func memUsagePercentFunc() float64 {
	used, total := readMemInfo()
	return memUsagePercent(used, total)
}

func readDiskIO() float64 {
	data, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return 0
	}
	var totalSectors int64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 14 {
			continue
		}
		// Only count whole disks (sda, mmcblk0, etc.)
		name := fields[2]
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") {
			continue
		}
		reads, _ := strconv.ParseInt(fields[5], 10, 64)
		writes, _ := strconv.ParseInt(fields[9], 10, 64)
		totalSectors += reads + writes
	}
	return float64(totalSectors) / 2.0 / 1024.0 // sectors → KB
}

func readNetIO() (rx float64, tx float64) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		iface := strings.TrimSpace(parts[0])
		if iface == "lo" || strings.HasPrefix(iface, "wlan") {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) >= 10 {
			r, _ := strconv.ParseFloat(fields[0], 64)
			t, _ := strconv.ParseFloat(fields[8], 64)
			rx += r
			tx += t
		}
	}
	return rx / 1024.0, tx / 1024.0 // B → KB
}

func readNetIORx() float64 {
	rx, _ := readNetIO()
	return rx
}

func readNetIOTx() float64 {
	_, tx := readNetIO()
	return tx
}

func readFPS() float64 {
	ringMu.RLock()
	defer ringMu.RUnlock()
	if len(ringHist) > 0 {
		last := ringHist[len(ringHist)-1]
		return float64(last.FPSX100) / 100.0
	}
	return 0
}

func detectGPUNode() string {
	// Adreno (Qualcomm)
	paths := []string{
		"/sys/class/kgsl/kgsl-3d0/devfreq/governor",
		"/sys/class/kgsl/kgsl-3d0/gpuclk",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Mali
	maliPaths, _ := filepath.Glob("/sys/devices/platform/*.gpu/devfreq/*/governor")
	if len(maliPaths) > 0 {
		return maliPaths[0]
	}
	return ""
}

func detectTouchNode() string {
	paths := []string{
		"/sys/class/input/input0/properties",
		"/sys/devices/virtual/input/input0/properties",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func detectCPUNode() string {
	p := "/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor"
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

func detectMaxRefreshHz() int {
	// Try fb0 mode
	if v := readInt("/sys/class/graphics/fb0/msm_fb_vsync_mode"); v > 0 {
		return v
	}
	// Try to parse from panel info
	data, _ := os.ReadFile("/sys/class/graphics/fb0/modes")
	if data != nil {
		modes := strings.Split(string(data), "\n")
		maxHz := 60
		for _, m := range modes {
			m = strings.TrimSpace(m)
			if strings.Contains(m, "60") {
				maxHz = 60
			}
			if strings.Contains(m, "120") {
				maxHz = 120
			}
			if strings.Contains(m, "144") {
				maxHz = 144
			}
			if strings.Contains(m, "165") {
				maxHz = 165
			}
		}
		return maxHz
	}
	return 120
}

func detectBypassChargingPath() string {
	paths := []string{
		"/sys/class/power_supply/battery/bypass_charging",
		"/sys/class/power_supply/charger/bypass",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func detectIOScheduler() string {
	data, _ := os.ReadFile("/sys/block/mmcblk0/queue/scheduler")
	if data == nil {
		data, _ = os.ReadFile("/sys/block/sda/queue/scheduler")
	}
	if data != nil {
		s := string(data)
		// Current scheduler is in brackets: [mq-deadline] bfq
		if idx := strings.Index(s, "["); idx >= 0 {
			if end := strings.Index(s[idx:], "]"); end >= 0 {
				return s[idx+1 : idx+end]
			}
		}
	}
	return ""
}

func detectIOSchedulers() []string {
	data, _ := os.ReadFile("/sys/block/mmcblk0/queue/scheduler")
	if data == nil {
		data, _ = os.ReadFile("/sys/block/sda/queue/scheduler")
	}
	if data != nil {
		s := strings.ReplaceAll(string(data), "[", "")
		s = strings.ReplaceAll(s, "]", "")
		return strings.Fields(s)
	}
	return nil
}

func featBatteryLevel() int {
	feat := readFeature()
	return int(feat.RemainingBattery * 100)
}
