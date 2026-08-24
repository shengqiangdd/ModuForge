package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ═══ Config ═══
type Config struct {
	CheckInterval int    `json:"check_interval"` // seconds
	LogPath       string `json:"log_path"`
	Verbose       bool   `json:"verbose"`
}

var defaultConfig = Config{
	CheckInterval: 30,
	LogPath:       "/data/local/tmp/{{MODULE_ID}}.log",
	Verbose:       false,
}

func loadConfig(path string) (Config, error) {
	cfg := defaultConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// ═══ Main Daemon ═══
func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("{{MODULE_NAME}} starting...")

	// Load config
	cfgPath := "{{CONFIG_PATH}}"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		log.Printf("Warning: using default config (%v)", err)
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	log.Printf("Daemon running, interval: %ds", cfg.CheckInterval)

	// Main loop
	ticker := time.NewTicker(time.Duration(cfg.CheckInterval) * time.Second)
	defer ticker.Stop()

	// Run immediately on start
	checkOnce(cfg)

	for {
		select {
		case <-ctx.Done():
			log.Println("Received shutdown signal, cleaning up...")
			log.Println("{{MODULE_NAME}} stopped")
			return
		case <-ticker.C:
			checkOnce(cfg)
		}
	}
}

func checkOnce(cfg Config) {
	if cfg.Verbose {
		log.Println("Checking...")
	}
	// TODO: Implement main logic
}
