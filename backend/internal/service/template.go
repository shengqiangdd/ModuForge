package service

import (
	"fmt"
	"strings"
)

// ModuleTemplate 表示一个模板
type ModuleTemplate struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"` // system, module, ui
	Tags        []string       `json:"tags"`
	Files       []TemplateFile `json:"files"`
}

type TemplateFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type TemplateService struct {
	templates map[string]*ModuleTemplate
}

func NewTemplateService() *TemplateService {
	ts := &TemplateService{
		templates: make(map[string]*ModuleTemplate),
	}
	ts.registerDefaults()
	return ts
}

func (s *TemplateService) registerDefaults() {
	s.templates["system.prop"] = &ModuleTemplate{
		Name:        "System.prop 模块",
		Description: "通过 system.prop 修改系统属性的 Magisk/KSU 模块",
		Category:    "system",
		Tags:        []string{"magisk", "ksu", "system", "prop"},
		Files: []TemplateFile{
			{Path: "module.prop", Content: "id=system_prop_mod\nname=System Prop Mod\nversion=v1.0\nversionCode=1\nauthor=ModuForge\ndescription=Custom system property modification module"},
			{Path: "system.prop", Content: "# System Properties\n# Add your properties below\n# Example:\n# debug.hwui.renderer=opengl\n"},
			{Path: "customize.sh", Content: "#!/system/bin/sh\n\nui_print \"- Installing system properties...\"\n"},
		},
	}
	s.templates["boot_animation"] = &ModuleTemplate{
		Name:        "开机动画模块",
		Description: "自定义开机动画的 Magisk 模块",
		Category:    "ui",
		Tags:        []string{"magisk", "boot", "animation", "ui"},
		Files: []TemplateFile{
			{Path: "module.prop", Content: "id=boot_animation\nname=Boot Animation\nversion=v1.0\nversionCode=1\nauthor=ModuForge\ndescription=Custom boot animation"},
			{Path: "customize.sh", Content: "#!/system/bin/sh\n\nui_print \"- Installing boot animation...\"\n"},
			{Path: "post-fs-data.sh", Content: "#!/system/bin/sh\n# Boot animation customization\n"},
		},
	}
	s.templates["audio_tweaks"] = &ModuleTemplate{
		Name:        "音频优化模块",
		Description: "音频参数优化的 Magisk/KSU 模块",
		Category:    "module",
		Tags:        []string{"magisk", "ksu", "audio", "tweaks"},
		Files: []TemplateFile{
			{Path: "module.prop", Content: "id=audio_tweaks\nname=Audio Tweaks\nversion=v1.0\nversionCode=1\nauthor=ModuForge\ndescription=Audio parameter optimization module"},
			{Path: "system/etc/audio_parameters.conf", Content: "# Audio Parameters\n"},
		},
	}
	s.templates["go_daemon"] = &ModuleTemplate{
		Name:        "Go 守护进程模块",
		Description: "Go 后台守护进程服务模块（带信号处理、配置管理、日志）",
		Category:    "module",
		Tags:        []string{"magisk", "ksu", "go", "daemon", "service", "background"},
		Files: []TemplateFile{
			{Path: "module.prop", Content: "id=go_daemon_module\nname=Go Daemon Service\nversion=v1.0\nversionCode=1\nauthor=ModuForge\ndescription=Go-based background daemon service module"},
			{Path: "customize.sh", Content: `#!/system/bin/sh
# Magisk/KSU/APatch install script
set -euo pipefail

MODDIR="${0%/*}"

ui_print "- Installing Go Daemon Service..."

# Check Android version
API=$(getprop ro.build.version.sdk)
if [ "$API" -lt 26 ]; then
    ui_print "! Minimum Android API 26 required"
    abort "! Aborting"
fi

# Set permissions
set_perm_recursive $MODPATH 0 0 0755 0644
set_perm $MODPATH/system/bin/daemon 0 0 0755

ui_print "- Installation complete"
`},
			{Path: "service.sh", Content: `#!/system/bin/sh
# Service script - runs on boot
MODDIR="${0%/*}"
LOGFILE="$MODDIR/logs/daemon.log"

# Create log directory
mkdir -p "$MODDIR/logs"

# Start daemon
if [ -x "$MODDIR/system/bin/daemon" ]; then
    nohup "$MODDIR/system/bin/daemon" \
        -config "$MODDIR/config.json" \
        -log "$LOGFILE" \
        >> "$LOGFILE" 2>&1 &
    echo $! > "$MODDIR/daemon.pid"
fi
`},
			{Path: "src/main.go", Content: `package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Config struct {
	Interval   int
	MaxRetries int
	LogLevel   string
}

func loadConfig(path string) (*Config, error) {
	cfg := &Config{
		Interval:   60,
		MaxRetries: 3,
		LogLevel:   "info",
	}
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func setupLogger(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)
}

func run(ctx context.Context, cfg *Config) error {
	slog.Info("daemon starting", "interval", cfg.Interval, "max_retries", cfg.MaxRetries)
	ticker := time.NewTicker(time.Duration(cfg.Interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("daemon shutting down")
			return nil
		case <-ticker.C:
			if err := doWork(); err != nil {
				slog.Error("work failed", "error", err)
			}
		}
	}
}

func doWork() error {
	slog.Info("performing scheduled work")
	return nil
}

func main() {
	configPath := flag.String("config", "", "path to config file")
	logPath := flag.String("log", "", "path to log file")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	setupLogger(cfg.LogLevel)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, cfg); err != nil {
		slog.Error("daemon failed", "error", err)
		os.Exit(1)
	}
}
`},
			{Path: "go.mod", Content: "module go-daemon\n\ngo 1.21\n"},
			{Path: "build.sh", Content: `#!/bin/bash
# Cross-compile for Android ARM64
set -euo pipefail

cd "$(dirname "$0")"
mkdir -p bin

GOOS=android GOARCH=arm64 CGO_ENABLED=0 \\
    go build -trimpath -ldflags="-s -w" -o ./bin/daemon ./src

echo "Build complete: bin/daemon"
`},
		},
	}
	s.templates["rest_api"] = &ModuleTemplate{
		Name:        "Go REST API 模块",
		Description: "Go HTTP REST API 服务模块（带路由、JSON处理、健康检查）",
		Category:    "module",
		Tags:        []string{"magisk", "ksu", "go", "rest", "api", "http", "webui"},
		Files: []TemplateFile{
			{Path: "module.prop", Content: "id=rest_api_module\nname=REST API Service\nversion=v1.0\nversionCode=1\nauthor=ModuForge\ndescription=Go REST API service module with WebUI"},
			{Path: "customize.sh", Content: `#!/system/bin/sh
set -euo pipefail
MODDIR="${0%/*}"
ui_print "- Installing REST API Service..."
set_perm_recursive $MODPATH 0 0 0755 0644
set_perm $MODPATH/system/bin/api-server 0 0 0755
ui_print "- Installation complete"
`},
			{Path: "service.sh", Content: `#!/system/bin/sh
MODDIR="${0%/*}"
mkdir -p "$MODDIR/logs"
if [ -x "$MODDIR/system/bin/api-server" ]; then
    nohup "$MODDIR/system/bin/api-server" -port 8080 >> "$MODDIR/logs/api.log" 2>&1 &
    echo $! > "$MODDIR/api.pid"
fi
`},
			{Path: "src/main.go", Content: `package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Response struct {
	Status  string
	Message string
	Data    interface{}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{Status: "ok"})
}

func main() {
	port := flag.Int("port", 8080, "server port")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Status:  "ok",
			Message: "service running",
			Data:    map[string]interface{}{"uptime": time.Since(startTime).String()},
		})
	})

	server := &http.Server{Addr: fmt.Sprintf(":%d", *port), Handler: mux}
	startTime := time.Now()

	go func() {
		slog.Info("API server starting", "port", *port)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("server shutting down")
	server.Close()
}
`},
			{Path: "go.mod", Content: "module rest-api\n\ngo 1.21\n"},
			{Path: "build.sh", Content: `#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p bin
GOOS=android GOARCH=arm64 CGO_ENABLED=0 \\
    go build -trimpath -ldflags="-s -w" -o ./bin/api-server ./src
echo "Build complete: bin/api-server"
`},
		},
	}
	s.templates["rust_daemon"] = &ModuleTemplate{
		Name:        "Rust 守护进程模块",
		Description: "Rust 后台守护进程模块（内存安全、tokio异步）",
		Category:    "module",
		Tags:        []string{"magisk", "ksu", "rust", "daemon", "async", "memory-safe"},
		Files: []TemplateFile{
			{Path: "module.prop", Content: "id=rust_daemon_module\nname=Rust Daemon\nversion=v1.0\nversionCode=1\nauthor=ModuForge\ndescription=Rust-based memory-safe daemon service module"},
			{Path: "customize.sh", Content: `#!/system/bin/sh
set -euo pipefail
MODDIR="${0%/*}"
ui_print "- Installing Rust Daemon..."
set_perm_recursive $MODPATH 0 0 0755 0644
set_perm $MODPATH/system/bin/rust-daemon 0 0 0755
ui_print "- Installation complete"
`},
			{Path: "src/main.rs", Content: `use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use tokio::signal;
use tracing::{info, error};

#[derive(serde::Deserialize)]
struct Config {
    #[serde(default = "default_interval")]
    interval: u64,
}

fn default_interval() -> u64 { 60 }

async fn run(config: Config) -> Result<(), Box<dyn std::error::Error>> {
    let running = Arc::new(AtomicBool::new(true));
    let r = running.clone();

    tokio::spawn(async move {
        signal::ctrl_c().await.ok();
        info!("shutdown signal received");
        r.store(false, Ordering::SeqCst);
    });

    info!("daemon starting, interval={}s", config.interval);

    while running.load(Ordering::SeqCst) {
        tokio::select! {
            _ = tokio::time::sleep(std::time::Duration::from_secs(config.interval)) => {
                if let Err(e) = do_work().await {
                    error!("work failed: {}", e);
                }
            }
            _ = signal::ctrl_c() => {
                break;
            }
        }
    }

    info!("daemon shutting down");
    Ok(())
}

async fn do_work() -> Result<(), Box<dyn std::error::Error>> {
    info!("performing scheduled work");
    Ok(())
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt::init();

    let config = Config { interval: 60 };
    run(config).await
}
`},
			{Path: "Cargo.toml", Content: `[package]
name = "rust-daemon"
version = "0.1.0"
edition = "2021"

[dependencies]
tokio = { version = "1", features = ["full"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
tracing = "0.1"
tracing-subscriber = "0.3"

[profile.release]
opt-level = "s"
lto = true
strip = true
`},
			{Path: "build.sh", Content: `#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p bin
cargo build --release --target aarch64-linux-android
cp target/aarch64-linux-android/release/rust-daemon bin/
echo "Build complete: bin/rust-daemon"
`},
		},
	}
	s.templates["performance_tuner"] = &ModuleTemplate{
		Name:        "性能调优模块",
		Description: "CPU/GPU/内存性能调优守护进程模块（通过 sysfs 控制 CPU 调度器、GPU 频率、虚拟内存参数）",
		Category:    "performance",
		Tags:        []string{"magisk", "ksu", "performance", "cpu", "gpu", "memory"},
		Files: []TemplateFile{
			{Path: "module.prop", Content: "id=performance_tuner\nname=Performance Tuner\nversion=v1.0\nversionCode=1\nauthor=ModuForge\ndescription=CPU/GPU/memory performance tuning daemon"},
			{Path: "service.sh", Content: `#!/system/bin/sh
# Service script - runs on boot
MODDIR="${0%/*}"
LOGFILE="$MODDIR/logs/perf_tuner.log"

mkdir -p "$MODDIR/logs"

# Start performance tuner daemon
if [ -x "$MODDIR/system/bin/perf-tuner" ]; then
    nohup "$MODDIR/system/bin/perf-tuner" \
        -config "$MODDIR/config.json" \
        -log "$LOGFILE" \
        >> "$LOGFILE" 2>&1 &
    echo $! > "$MODDIR/perf_tuner.pid"
fi
`},
			{Path: "src/main.go", Content: `package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

type PerfConfig struct {
	CPUGovernor    string
	CPUFreqMin     int
	CPUFreqMax     int
	GPUFreqMin     int
	GPUFreqMax     int
	VmSwappiness   int
	UpdateInterval int
}

func defaultConfig() *PerfConfig {
	return &PerfConfig{
		CPUGovernor:    "schedutil",
		CPUFreqMin:     0,
		CPUFreqMax:     0,
		GPUFreqMin:     0,
		GPUFreqMax:     0,
		VmSwappiness:   100,
		UpdateInterval: 30,
	}
}

func readSysfs(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(data[:len(data)-1]), nil // strip trailing newline
}

func writeSysfs(path, value string) error {
	return os.WriteFile(path, []byte(value), 0644)
}

func setCPUGovernor(governor string) error {
	paths, _ := filepath.Glob("/sys/devices/system/cpu/cpu*/cpufreq/scaling_governor")
	for _, p := range paths {
		if err := writeSysfs(p, governor); err != nil {
			slog.Warn("failed to set governor", "path", p, "error", err)
			continue
		}
		slog.Info("set cpu governor", "path", p, "governor", governor)
	}
	return nil
}

func setCPUFreqMinMax(minFreq, maxFreq int) error {
	if minFreq > 0 {
		paths, _ := filepath.Glob("/sys/devices/system/cpu/cpu*/cpufreq/scaling_min_freq")
		for _, p := range paths {
			if err := writeSysfs(p, fmt.Sprintf("%d", minFreq)); err != nil {
				slog.Warn("failed to set min freq", "path", p, "error", err)
			}
		}
	}
	if maxFreq > 0 {
		paths, _ := filepath.Glob("/sys/devices/system/cpu/cpu*/cpufreq/scaling_max_freq")
		for _, p := range paths {
			if err := writeSysfs(p, fmt.Sprintf("%d", maxFreq)); err != nil {
				slog.Warn("failed to set max freq", "path", p, "error", err)
			}
		}
	}
	return nil
}

func setGPUFreq(gpuPath string, minFreq, maxFreq int) error {
	if minFreq > 0 {
		minPath := filepath.Join(gpuPath, "min_freq")
		if err := writeSysfs(minPath, fmt.Sprintf("%d", minFreq)); err != nil {
			slog.Warn("failed to set gpu min freq", "path", minPath, "error", err)
		}
	}
	if maxFreq > 0 {
		maxPath := filepath.Join(gpuPath, "max_freq")
		if err := writeSysfs(maxPath, fmt.Sprintf("%d", maxFreq)); err != nil {
			slog.Warn("failed to set gpu max freq", "path", maxPath, "error", err)
		}
	}
	return nil
}

func setSwappiness(val int) error {
	return writeSysfs("/proc/sys/vm/swappiness", fmt.Sprintf("%d", val))
}

func applyConfig(cfg *PerfConfig) error {
	if err := setCPUGovernor(cfg.CPUGovernor); err != nil {
		return err
	}
	if err := setCPUFreqMinMax(cfg.CPUFreqMin, cfg.CPUFreqMax); err != nil {
		return err
	}
	if cfg.VmSwappiness > 0 {
		if err := setSwappiness(cfg.VmSwappiness); err != nil {
			slog.Warn("failed to set swappiness", "error", err)
		}
	}
	// Try Adreno GPU path
	adrenoPath := "/sys/class/kgsl/kgsl-3d0"
	if _, err := os.Stat(adrenoPath); err == nil {
		setGPUFreq(adrenoPath, cfg.GPUFreqMin, cfg.GPUFreqMax)
	}
	// Try Mali GPU path (common platform paths)
	for _, p := range []string{
		"/sys/devices/platform/mali.0",
		"/sys/devices/platform/gpu",
	} {
		if _, err := os.Stat(p); err == nil {
			setGPUFreq(p, cfg.GPUFreqMin, cfg.GPUFreqMax)
			break
		}
	}
	slog.Info("performance config applied",
		"gov", cfg.CPUGovernor,
		"swappiness", cfg.VmSwappiness,
	)
	return nil
}

func run(ctx context.Context, cfg *PerfConfig) error {
	slog.Info("performance tuner starting", "governor", cfg.CPUGovernor, "interval", cfg.UpdateInterval)
	// Apply immediately on start
	applyConfig(cfg)

	ticker := time.NewTicker(time.Duration(cfg.UpdateInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("performance tuner shutting down")
			return nil
		case <-ticker.C:
			applyConfig(cfg)
		}
	}
}

func main() {
	configPath := flag.String("config", "", "path to config file")
	logPath := flag.String("log", "", "path to log file")
	flag.Parse()

	cfg := defaultConfig()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if *logPath != "" {
		f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			defer f.Close()
			logger = slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo}))
			slog.SetDefault(logger)
		}
	}

	_ = configPath // TODO: parse JSON config from file

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, cfg); err != nil {
		slog.Error("performance tuner failed", "error", err)
		os.Exit(1)
	}
}
`},
			{Path: "go.mod", Content: "module performance-tuner\n\ngo 1.21\n"},
			{Path: "build.sh", Content: `#!/bin/bash
# Cross-compile for Android ARM64
set -euo pipefail

cd "$(dirname "$0")"
mkdir -p bin

GOOS=android GOARCH=arm64 CGO_ENABLED=0 \
    go build -trimpath -ldflags="-s -w" -o ./bin/perf-tuner ./src

echo "Build complete: bin/perf-tuner"
`},
		},
	}
	s.templates["battery_manager"] = &ModuleTemplate{
		Name:        "电池管理模块",
		Description: "电池监控与管理守护进程模块（监控电池状态、管理 wakelock、调整灭屏超时）",
		Category:    "battery",
		Tags:        []string{"magisk", "ksu", "battery", "power", "wakelock"},
		Files: []TemplateFile{
			{Path: "module.prop", Content: "id=battery_manager\nname=Battery Manager\nversion=v1.0\nversionCode=1\nauthor=ModuForge\ndescription=Battery monitoring and power management daemon"},
			{Path: "service.sh", Content: `#!/system/bin/sh
# Service script - runs on boot
MODDIR="${0%/*}"
LOGFILE="$MODDIR/logs/battery.log"

mkdir -p "$MODDIR/logs"

if [ -x "$MODDIR/system/bin/battery-mgr" ]; then
    nohup "$MODDIR/system/bin/battery-mgr" \
        -config "$MODDIR/config.json" \
        -log "$LOGFILE" \
        >> "$LOGFILE" 2>&1 &
    echo $! > "$MODDIR/battery.pid"
fi
`},
			{Path: "src/main.go", Content: `package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type BatteryConfig struct {
	CheckInterval      int
	ScreenOffTimeout   int
	EnableWakelockMgmt bool
	HighBatThreshold   int
	LowBatThreshold    int
}

type BatteryInfo struct {
	Status   string
	Level    int
	Health   string
	Temp     int
	Voltage  int
	Capacity int
}

func readSysfs(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func readBatteryInfo() (*BatteryInfo, error) {
	basePath := "/sys/class/power_supply"
	powerSupplies, err := filepath.Glob(filepath.Join(basePath, "battery*"))
	if err != nil || len(powerSupplies) == 0 {
		return nil, fmt.Errorf("no battery found under %s", basePath)
	}

	batPath := powerSupplies[0]
	info := &BatteryInfo{}

	if v, err := readSysfs(filepath.Join(batPath, "status")); err == nil {
		info.Status = v
	}
	if v, err := readSysfs(filepath.Join(batPath, "capacity")); err == nil {
		info.Capacity, _ = strconv.Atoi(v)
		info.Level = info.Capacity
	}
	if v, err := readSysfs(filepath.Join(batPath, "health")); err == nil {
		info.Health = v
	}
	if v, err := readSysfs(filepath.Join(batPath, "temp")); err == nil {
		info.Temp, _ = strconv.Atoi(v)
	}
	if v, err := readSysfs(filepath.Join(batPath, "voltage_now")); err == nil {
		volt, _ := strconv.Atoi(v)
		info.Voltage = volt / 1000 // convert uV to mV
	}

	return info, nil
}

func getActiveWakelocks() ([]string, error) {
	data, err := readSysfs("/sys/kernel/wakelocks")
	if err != nil {
		return nil, err
	}
	var active []string
	for _, line := range strings.Split(data, "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 3 && parts[2] != "" {
			active = append(active, parts[0])
		}
	}
	return active, nil
}

func setScreenOffTimeout(ms int) error {
	path := "/sys/class/android_charger/device/timeout"
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d", ms)), 0644); err != nil {
		// Fallback to settings put
		return fmt.Errorf("set timeout: %w", err)
	}
	return nil
}

func defaultConfig() *BatteryConfig {
	return &BatteryConfig{
		CheckInterval:      60,
		ScreenOffTimeout:   30000,
		EnableWakelockMgmt: true,
		HighBatThreshold:   90,
		LowBatThreshold:    15,
	}
}

func run(ctx context.Context, cfg *BatteryConfig) error {
	slog.Info("battery manager starting", "interval", cfg.CheckInterval)

	ticker := time.NewTicker(time.Duration(cfg.CheckInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("battery manager shutting down")
			return nil
		case <-ticker.C:
			bat, err := readBatteryInfo()
			if err != nil {
				slog.Error("failed to read battery", "error", err)
				continue
			}
			slog.Info("battery status",
				"level", bat.Level,
				"status", bat.Status,
				"temp", bat.Temp,
				"voltage_mv", bat.Voltage,
			)

			if cfg.EnableWakelockMgmt {
				wls, err := getActiveWakelocks()
				if err != nil {
					slog.Warn("failed to read wakelocks", "error", err)
				} else if len(wls) > 0 {
					slog.Info("active wakelocks", "count", len(wls))
				}
			}

			if bat.Level <= cfg.LowBatThreshold {
				slog.Warn("low battery", "level", bat.Level)
			}
			if bat.Level >= cfg.HighBatThreshold {
				slog.Info("battery nearly full", "level", bat.Level)
			}
		}
	}
}

func main() {
	configPath := flag.String("config", "", "path to config file")
	logPath := flag.String("log", "", "path to log file")
	flag.Parse()

	cfg := defaultConfig()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if *logPath != "" {
		f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			defer f.Close()
			logger = slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo}))
			slog.SetDefault(logger)
		}
	}

	_ = configPath

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, cfg); err != nil {
		slog.Error("battery manager failed", "error", err)
		os.Exit(1)
	}
}
`},
			{Path: "go.mod", Content: "module battery-manager\n\ngo 1.21\n"},
			{Path: "build.sh", Content: `#!/bin/bash
# Cross-compile for Android ARM64
set -euo pipefail

cd "$(dirname "$0")"
mkdir -p bin

GOOS=android GOARCH=arm64 CGO_ENABLED=0 \
    go build -trimpath -ldflags="-s -w" -o ./bin/battery-mgr ./src

echo "Build complete: bin/battery-mgr"
`},
		},
	}
	s.templates["network_tool"] = &ModuleTemplate{
		Name:        "网络优化模块",
		Description: "网络参数优化模块（TCP 缓冲区、WiFi 省电模式、DNS 设置）",
		Category:    "network",
		Tags:        []string{"magisk", "ksu", "network", "tcp", "wifi", "dns"},
		Files: []TemplateFile{
			{Path: "module.prop", Content: "id=network_tool\nname=Network Tool\nversion=v1.0\nversionCode=1\nauthor=ModuForge\ndescription=Network optimization module with TCP/WiFi/DNS tuning"},
			{Path: "service.sh", Content: `#!/system/bin/sh
# Network optimization service - runs on boot
MODDIR="${0%/*}"
LOGFILE="$MODDIR/logs/network.log"

mkdir -p "$MODDIR/logs"

# Apply TCP buffer sizes
echo "4096 16384 131072" > /proc/sys/net/ipv4/tcp_rmem
echo "4096 65536 131072" > /proc/sys/net/ipv4/tcp_wmem
echo 131072 > /proc/sys/net/core/wmem_max
echo 131072 > /proc/sys/net/core/rmem_max

# Enable TCP window scaling and timestamps
echo 1 > /proc/sys/net/ipv4/tcp_window_scaling
echo 1 > /proc/sys/net/ipv4/tcp_timestamps
echo 1 > /proc/sys/net/ipv4/tcp_sack

# Optimize TCP keepalive
echo 600 > /proc/sys/net/ipv4/tcp_keepalive_time
echo 60 > /proc/sys/net/ipv4/tcp_keepalive_intvl
echo 5 > /proc/sys/net/ipv4/tcp_keepalive_probes

# Disable WiFi power save for better latency
for iface in /sys/class/net/wlan*/; do
    if [ -d "$iface" ]; then
        iwconfig "$(basename $iface)" power off 2>/dev/null || true
    fi
done

# Set DNS servers
setprop net.dns1 8.8.8.8
setprop net.dns2 8.8.4.4
setprop net.dns3 1.1.1.1

echo "$(date): Network optimizations applied" >> "$LOGFILE"
`},
			{Path: "customize.sh", Content: `#!/system/bin/sh
set -euo pipefail

MODDIR="${0%/*}"

ui_print "- Installing Network Tool..."

# Set permissions
set_perm_recursive $MODPATH 0 0 0755 0644
set_perm $MODPATH/service.sh 0 0 0755
set_perm $MODPATH/system.prop 0 0 0644

ui_print "- Network Tool installed"
ui_print "- Settings will apply on next boot"
`},
			{Path: "system.prop", Content: `# TCP Optimization
net.ipv4.tcp_rmem=4096 16384 131072
net.ipv4.tcp_wmem=4096 65536 131072
net.core.wmem_max=131072
net.core.rmem_max=131072
net.ipv4.tcp_window_scaling=1
net.ipv4.tcp_timestamps=1
net.ipv4.tcp_sack=1

# WiFi Power Save (disable for lower latency)
wifi.supplicant_scan_interval=180
net.wifi.sleep.policy=2

# DNS Settings
net.dns1=8.8.8.8
net.dns2=8.8.4.4
net.dns3=1.1.1.1
`},
		},
	}
	s.templates["system_cleaner"] = &ModuleTemplate{
		Name:        "系统清理模块",
		Description: "系统清理模块（开机自动清理缓存、dalvik-cache 等垃圾文件）",
		Category:    "utility",
		Tags:        []string{"magisk", "ksu", "cleanup", "cache", "storage"},
		Files: []TemplateFile{
			{Path: "module.prop", Content: "id=system_cleaner\nname=System Cleaner\nversion=v1.0\nversionCode=1\nauthor=ModuForge\ndescription=System cleanup module for cache and dalvik-cache"},
			{Path: "service.sh", Content: `#!/system/bin/sh
# System cleaner - runs on boot
MODDIR="${0%/*}"
LOGFILE="$MODDIR/logs/cleaner.log"

mkdir -p "$MODDIR/logs"

echo "$(date): Starting system cleanup" >> "$LOGFILE"

# Clean app caches
CACHE_CLEANED=0
for cache_dir in /data/cache/*; do
    if [ -d "$cache_dir" ]; then
        size=$(du -sk "$cache_dir" 2>/dev/null | awk '{print $1}')
        rm -rf "$cache_dir"/* 2>/dev/null
        CACHE_CLEANED=$((CACHE_CLEANED + size))
    fi
done

# Clean dalvik-cache (requires reboot to take effect)
DALVIK_CLEANED=0
if [ -d "/data/dalvik-cache" ]; then
    DALVIK_CLEANED=$(du -sk /data/dalvik-cache 2>/dev/null | awk '{print $1}')
    rm -rf /data/dalvik-cache/* 2>/dev/null
fi

# Clean unused APK files
APK_CLEANED=0
for apk_dir in /data/app/*/; do
    if [ -d "$apk_dir" ]; then
        find "$apk_dir" -name "*.tmp" -delete 2>/dev/null
    fi
done

# Clean log files older than 7 days
find /data/log -name "*.log" -mtime +7 -delete 2>/dev/null

# Clean /data/tombstones (old crash dumps)
if [ -d "/data/tombstones" ]; then
    find /data/tombstones -name "*.pb" -mtime +30 -delete 2>/dev/null
fi

# Clean temp files
rm -rf /data/local/tmp/*.tmp 2>/dev/null
rm -rf /data/local/tmp/*.log 2>/dev/null

TOTAL_CLEANED=$((CACHE_CLEANED + DALVIK_CLEANED))
echo "$(date): Cleanup complete - cache: ${CACHE_CLEANED}KB, dalvik: ${DALVIK_CLEANED}KB, total: ${TOTAL_CLEANED}KB" >> "$LOGFILE"
`},
			{Path: "customize.sh", Content: `#!/system/bin/sh
set -euo pipefail

MODDIR="${0%/*}"

ui_print "- Installing System Cleaner..."

# Set permissions
set_perm_recursive $MODPATH 0 0 0755 0644
set_perm $MODPATH/service.sh 0 0 0755

# Create log directory
mkdir -p "$MODDIR/logs"

ui_print "- System Cleaner installed"
ui_print "- Cleanup will run on next boot"
`},
		},
	}
	s.templates["gpu_optimizer"] = &ModuleTemplate{
		Name:        "GPU 优化模块",
		Description: "GPU 性能优化模块（支持 Adreno 和 Mali GPU 频率控制）",
		Category:    "performance",
		Tags:        []string{"magisk", "ksu", "gpu", "mali", "adreno", "graphics"},
		Files: []TemplateFile{
			{Path: "module.prop", Content: "id=gpu_optimizer\nname=GPU Optimizer\nversion=v1.0\nversionCode=1\nauthor=ModuForge\ndescription=GPU optimization module for Adreno and Mali GPUs"},
			{Path: "service.sh", Content: `#!/system/bin/sh
# GPU optimization service - runs on boot
MODDIR="${0%/*}"
LOGFILE="$MODDIR/logs/gpu.log"

mkdir -p "$MODDIR/logs"

# Detect GPU type and apply optimizations
apply_adreno() {
    local gpu_path="/sys/class/kgsl/kgsl-3d0"
    if [ ! -d "$gpu_path" ]; then
        return 1
    fi

    echo "Adreno GPU detected" >> "$LOGFILE"

    # Read available frequencies
    if [ -f "$gpu_path/gpu_available_frequencies" ]; then
        local avail_freqs=$(cat "$gpu_path/gpu_available_frequencies")
        echo "Available frequencies: $avail_freqs" >> "$LOGFILE"
    fi

    # Read current GPU clock
    if [ -f "$gpu_path/gpuclk" ]; then
        echo "Current GPU clock: $(cat $gpu_path/gpuclk)" >> "$LOGFILE"
    fi

    # Set GPU governor to performance
    if [ -f "$gpu_path/devfreq/governor" ]; then
        echo "msm-adreno-tz" > "$gpu_path/devfreq/governor" 2>/dev/null
    fi

    # Enable GPU bus scaling
    if [ -f "$gpu_path/force_clk_on" ]; then
        echo 1 > "$gpu_path/force_clk_on" 2>/dev/null
    fi

    # Set max GPU frequency
    if [ -f "$gpu_path/gpu_available_frequencies" ]; then
        local max_freq=$(cat "$gpu_path/gpu_available_frequencies" | tr ' ' '\n' | sort -n | tail -1)
        if [ ! -z "$max_freq" ]; then
            echo "$max_freq" > "$gpu_path/max_gpuclk" 2>/dev/null
            echo "Set max GPU frequency to $max_freq" >> "$LOGFILE"
        fi
    fi
}

apply_mali() {
    local mali_paths="/sys/devices/platform/mali.0 /sys/devices/platform/gpu"
    local found=0

    for mali_path in $mali_paths; do
        if [ -d "$mali_path" ]; then
            found=1
            echo "Mali GPU detected at $mali_path" >> "$LOGFILE"

            # Set performance governor
            if [ -f "$mali_path/devfreq/governor" ]; then
                echo "performance" > "$mali_path/devfreq/governor" 2>/dev/null
            fi

            # Set max frequency
            if [ -f "$mali_path/gpu_available_frequencies" ]; then
                local max_freq=$(cat "$mali_path/gpu_available_frequencies" | tr ' ' '\n' | sort -n | tail -1)
                if [ ! -z "$max_freq" ]; then
                    echo "$max_freq" > "$mali_path/gpu_max_frequency" 2>/dev/null
                    echo "Set max Mali frequency to $max_freq" >> "$LOGFILE"
                fi
            fi

            break
        fi
    done

    return $((1 - found))
}

# Try Adreno first, then Mali
if apply_adreno; then
    echo "$(date): Adreno GPU optimized" >> "$LOGFILE"
elif apply_mali; then
    echo "$(date): Mali GPU optimized" >> "$LOGFILE"
else
    echo "$(date): No compatible GPU found" >> "$LOGFILE"
fi
`},
			{Path: "customize.sh", Content: `#!/system/bin/sh
set -euo pipefail

MODDIR="${0%/*}"

ui_print "- Installing GPU Optimizer..."

# Detect GPU type for user info
if [ -d "/sys/class/kgsl/kgsl-3d0" ]; then
    ui_print "- Detected: Qualcomm Adreno GPU"
elif [ -d "/sys/devices/platform/mali.0" ] || [ -d "/sys/devices/platform/gpu" ]; then
    ui_print "- Detected: ARM Mali GPU"
else
    ui_print "- Warning: Could not detect GPU type"
    ui_print "- Module will attempt detection at boot"
fi

# Set permissions
set_perm_recursive $MODPATH 0 0 0755 0644
set_perm $MODPATH/service.sh 0 0 0755

# Create log directory
mkdir -p "$MODDIR/logs"

ui_print "- GPU Optimizer installed"
ui_print "- GPU optimizations will apply on next boot"
`},
		},
	}
}

// ListTemplates 返回所有模板
func (s *TemplateService) ListTemplates() []*ModuleTemplate {
	result := make([]*ModuleTemplate, 0, len(s.templates))
	for _, t := range s.templates {
		result = append(result, t)
	}
	return result
}

// RecommendByDescription 根据用户描述推荐模板
func (s *TemplateService) RecommendByDescription(description string) []*ModuleTemplate {
	desc := strings.ToLower(description)
	var scored []struct {
		template *ModuleTemplate
		score    int
	}

	for _, t := range s.templates {
		score := 0
		// Check name
		if strings.Contains(strings.ToLower(t.Name), desc) {
			score += 3
		}
		// Check tags
		for _, tag := range t.Tags {
			if strings.Contains(desc, tag) {
				score += 2
			}
		}
		// Check description
		if strings.Contains(strings.ToLower(t.Description), desc) {
			score += 1
		}
		if score > 0 {
			scored = append(scored, struct {
				template *ModuleTemplate
				score    int
			}{t, score})
		}
	}

	// Sort by score desc
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	result := make([]*ModuleTemplate, 0, len(scored))
	for _, s := range scored {
		result = append(result, s.template)
	}
	return result
}

// GetTemplate 按名称获取模板
func (s *TemplateService) GetTemplate(name string) (*ModuleTemplate, error) {
	if t, ok := s.templates[name]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("template not found: %s", name)
}

// GetTemplatesByCategory returns templates filtered by category
func (s *TemplateService) GetTemplatesByCategory(category string) []*ModuleTemplate {
	var result []*ModuleTemplate
	for _, t := range s.templates {
		if t.Category == category {
			result = append(result, t)
		}
	}
	return result
}

// GetCategories returns all unique categories
func (s *TemplateService) GetCategories() []string {
	seen := make(map[string]bool)
	var cats []string
	for _, t := range s.templates {
		if !seen[t.Category] {
			seen[t.Category] = true
			cats = append(cats, t.Category)
		}
	}
	return cats
}
