package builder

import (
	"strings"
)

// CodePattern represents a reusable code pattern in the catalog.
type CodePattern struct {
	ID          string   `json:"id"`           // e.g. "go_daemon"
	Name        string   `json:"name"`         // e.g. "Go Daemon"
	Language    string   `json:"language"`     // "go", "c", "sh"
	Category    string   `json:"category"`     // "daemon", "system_call", "monitor", etc.
	Description string   `json:"description"`  // When to use this pattern
	Tags        []string `json:"tags"`         // Searchable tags
	Imports     []string `json:"imports"`      // Required Go imports
	Code        string   `json:"code"`         // Template code with {{SLOT}} placeholders
}

// PatternCatalog holds all available code patterns.
type PatternCatalog struct {
	patterns []CodePattern
}

// NewPatternCatalog creates the default pattern catalog.
func NewPatternCatalog() *PatternCatalog {
	pc := &PatternCatalog{}
	pc.loadBuiltinPatterns()
	return pc
}

// FindMatchingPatterns finds patterns that match given tags/type.
func (pc *PatternCatalog) FindMatchingPatterns(category string, tags []string) []CodePattern {
	var matches []CodePattern
	tagSet := make(map[string]bool)
	for _, t := range tags {
		tagSet[strings.ToLower(t)] = true
	}

	for _, p := range pc.patterns {
		if category != "" && p.Category != category {
			continue
		}
		// Check tag overlap
		for _, t := range p.Tags {
			if tagSet[strings.ToLower(t)] {
				matches = append(matches, p)
				break
			}
		}
	}
	return matches
}

// GetPattern returns a pattern by ID.
func (pc *PatternCatalog) GetPattern(id string) *CodePattern {
	for i := range pc.patterns {
		if pc.patterns[i].ID == id {
			return &pc.patterns[i]
		}
	}
	return nil
}

// AddPattern adds a new pattern to the catalog (for self-evolution).
func (pc *PatternCatalog) AddPattern(p CodePattern) {
	pc.patterns = append(pc.patterns, p)
}

// loadBuiltinPatterns loads the built-in pattern library.
func (pc *PatternCatalog) loadBuiltinPatterns() {
	pc.patterns = append(pc.patterns,
		// ═══════════════════════════════════════
		// GO PATTERNS
		// ═══════════════════════════════════════
		goDaemonPattern(),
		goSignalHandlerPattern(),
		goConfigLoaderPattern(),
		goPeriodicCheckPattern(),
		goSysfsReaderPattern(),
		goFileManagerPattern(),

		// ═══════════════════════════════════════
		// C PATTERNS
		// ═══════════════════════════════════════
		cWatchdogPattern(),
		cSysrqPattern(),
		cProcReaderPattern(),
	)
}

// ═══════════════════════════════════════════════════════
// GO PATTERNS
// ═══════════════════════════════════════════════════════

func goDaemonPattern() CodePattern {
	return CodePattern{
		ID:       "go_daemon",
		Name:     "Go Daemon",
		Language: "go",
		Category: "daemon",
		Description: "A complete Go daemon with signal handling, config loading, " +
			"periodic tasks, and logging. Use for any background service.",
		Tags:    []string{"daemon", "service", "background", "守护进程"},
		Imports: []string{"context", "encoding/json", "log", "os", "os/signal", "syscall", "time"},
		Code: `package main

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
	{{CONFIG_FIELDS}}
}

var defaultConfig = Config{
	{{CONFIG_DEFAULTS}}
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

	log.Println("Daemon running, interval:", cfg.{{INTERVAL_FIELD}})

	// Main loop
	ticker := time.NewTicker(cfg.{{INTERVAL_FIELD}})
	defer ticker.Stop()

	{{MAIN_LOOP_BODY}}

	log.Println("{{MODULE_NAME}} stopped")
}
`,
	}
}

func goSignalHandlerPattern() CodePattern {
	return CodePattern{
		ID:          "go_signal_handler",
		Name:        "Go Signal Handler",
		Language:    "go",
		Category:    "daemon",
		Description: "Graceful signal handling with context. Always use this in daemons.",
		Tags:        []string{"signal", "graceful", "shutdown", "context", "退出"},
		Imports:     []string{"context", "os/signal", "syscall"},
		Code: `// Graceful shutdown with signal handling
ctx, stop := signal.NotifyContext(context.Background(),
	syscall.SIGTERM, syscall.SIGINT)
defer stop()

// Wait for signal or context cancellation
<-ctx.Done()
log.Println("Received shutdown signal, cleaning up...")
// Cleanup resources here
`,
	}
}

func goConfigLoaderPattern() CodePattern {
	return CodePattern{
		ID:          "go_config_loader",
		Name:        "Go JSON Config Loader",
		Language:    "go",
		Category:    "config",
		Description: "Load JSON config with defaults. Use json.Unmarshal + os.ReadFile.",
		Tags:        []string{"config", "json", "configuration", "配置"},
		Imports:     []string{"encoding/json", "os"},
		Code: `type Config struct {
	{{CONFIG_FIELDS}}
}

var defaultConfig = Config{
	{{CONFIG_DEFAULTS}}
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
`,
	}
}

func goPeriodicCheckPattern() CodePattern {
	return CodePattern{
		ID:          "go_periodic_check",
		Name:        "Go Periodic Check Loop",
		Language:    "go",
		Category:    "loop",
		Description: "Periodic check with context support and graceful exit.",
		Tags:        []string{"loop", "periodic", "ticker", "interval", "定时"},
		Imports:     []string{"context", "time"},
		Code: `// Periodic check loop with context support
func runPeriodicCheck(ctx context.Context, interval time.Duration, checkFunc func() error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start
	if err := checkFunc(); err != nil {
		log.Printf("Check error: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Periodic check stopped")
			return
		case <-ticker.C:
			if err := checkFunc(); err != nil {
				log.Printf("Check error: %v", err)
			}
		}
	}
}
`,
	}
}

func goSysfsReaderPattern() CodePattern {
	return CodePattern{
		ID:          "go_sysfs_reader",
		Name:        "Go Sysfs Reader",
		Language:    "go",
		Category:    "system_call",
		Description: "Read values from /sys/ filesystem (thermal, battery, CPU, etc.).",
		Tags:        []string{"sysfs", "thermal", "battery", "cpu", "温度", "电池", "系统"},
		Imports:     []string{"os", "strconv", "strings"},
		Code: `// Read integer value from sysfs
func readSysfsInt(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	val, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return val, nil
}

// Read string value from sysfs
func readSysfsString(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
`,
	}
}

func goFileManagerPattern() CodePattern {
	return CodePattern{
		ID:          "go_file_manager",
		Name:        "Go File Manager",
		Language:    "go",
		Category:    "file_io",
		Description: "Safe file operations with proper error handling and cleanup.",
		Tags:        []string{"file", "io", "write", "read", "文件"},
		Imports:     []string{"fmt", "os", "path/filepath"},
		Code: `// Ensure directory exists
func ensureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0755)
}

// Write file atomically (write to temp then rename)
func writeAtomic(path string, data []byte) error {
	if err := ensureDir(path); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Append line to log file
func appendLog(path, line string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}
`,
	}
}

// ═══════════════════════════════════════════════════════
// C PATTERNS
// ═══════════════════════════════════════════════════════

func cWatchdogPattern() CodePattern {
	return CodePattern{
		ID:       "c_watchdog",
		Name:     "C Watchdog",
		Language: "c",
		Category: "daemon",
		Description: "A C watchdog daemon that monitors a condition and takes action.",
		Tags:    []string{"watchdog", "monitor", "看门狗", "守护"},
		Code: `#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <signal.h>
#include <time.h>

static volatile int running = 1;

void signal_handler(int sig) {
	(void)sig;
	running = 0;
}

int read_file_int(const char *path) {
	FILE *f = fopen(path, "r");
	if (!f) return -1;
	int val = 0;
	fscanf(f, "%d", &val);
	fclose(f);
	return val;
}

int main(int argc, char *argv[]) {
	int interval = {{INTERVAL}};
	(void)argc;
	(void)argv;

	signal(SIGTERM, signal_handler);
	signal(SIGINT, signal_handler);

	printf("Watchdog started, interval=%d\\n", interval);

	while (running) {
		{{WATCHDOG_BODY}}
		sleep(interval);
	}

	printf("Watchdog stopped\\n");
	return 0;
}
`,
	}
}

func cSysrqPattern() CodePattern {
	return CodePattern{
		ID:          "c_sysrq",
		Name:        "C Sysrq Trigger",
		Language:    "c",
		Category:    "system_call",
		Description: "Write to /proc/sysrq-trigger for system-level operations.",
		Tags:        []string{"sysrq", "proc", "系统调用", "系统请求"},
		Code: `#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>

int write_sysrq(char trigger) {
	int fd = open("/proc/sysrq-trigger", O_WRONLY);
	if (fd < 0) {
		perror("open sysrq");
		return -1;
	}
	char buf[2] = {trigger, 0};
	write(fd, buf, 1);
	close(fd);
	return 0;
}
`,
	}
}

func cProcReaderPattern() CodePattern {
	return CodePattern{
		ID:          "c_proc_reader",
		Name:        "C /proc Reader",
		Language:    "c",
		Category:    "system_call",
		Description: "Read values from /proc filesystem.",
		Tags:        []string{"proc", "read", "系统", "读取"},
		Code: `#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int read_proc_int(const char *path) {
	FILE *f = fopen(path, "r");
	if (!f) return -1;
	int val = 0;
	fscanf(f, "%d", &val);
	fclose(f);
	return val;
}

int read_proc_string(const char *path, char *buf, int bufsize) {
	FILE *f = fopen(path, "r");
	if (!f) return -1;
	if (fgets(buf, bufsize, f) == NULL) {
		fclose(f);
		return -1;
	}
	buf[strcspn(buf, "\\n")] = 0;
	fclose(f);
	return 0;
}
`,
	}
}
