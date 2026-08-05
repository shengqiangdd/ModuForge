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
