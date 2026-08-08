package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gofiber/fiber/v3"
)

var startTime = time.Now()

type HealthHandler struct {
	db      *sql.DB
	llmURL  string
	adbAddr string
	agentRunner interface{ GetCacheStats() map[string]interface{} }
}

func NewHealthHandler(db *sql.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

// SetAgentRunner configures the agent runner for cache statistics
func (h *HealthHandler) SetAgentRunner(runner interface{ GetCacheStats() map[string]interface{} }) {
	h.agentRunner = runner
}

// SetLLMURL configures the LLM endpoint for connectivity checks
func (h *HealthHandler) SetLLMURL(url string) {
	h.llmURL = url
}

// SetADBAddr configures the ADB address for service checks
func (h *HealthHandler) SetADBAddr(addr string) {
	h.adbAddr = addr
}

func (h *HealthHandler) Check(c fiber.Ctx) error {
	checks := fiber.Map{}
	checkedAt := time.Now().Format(time.RFC3339)

	// Database check
	dbStart := time.Now()
	dbStatus := "ok"
	var dbErr string
	if err := h.db.Ping(); err != nil {
		dbStatus = "error"
		dbErr = err.Error()
	}
	dbDuration := time.Since(dbStart).Milliseconds()
	checks["database"] = fiber.Map{
		"status":      dbStatus,
		"response_ms": dbDuration,
		"error":       dbErr,
	}

	// Disk check with warning threshold
	diskStatus := "ok"
	var freeGB, totalGB float64
	if diskInfo, err := getDiskInfo(); err == nil {
		freeGB = diskInfo.freeGB
		totalGB = diskInfo.totalGB
		if freeGB < 1.0 {
			diskStatus = "warning"
		}
	} else {
		diskStatus = "unknown"
	}
	checks["disk"] = fiber.Map{
		"status":   diskStatus,
		"free_gb":  freeGB,
		"total_gb": totalGB,
	}

	// Memory check with >80% warning
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	memUsedMB := float64(mem.Alloc) / 1024 / 1024
	memSysMB := float64(mem.Sys) / 1024 / 1024
	memStatus := "ok"
	if memSysMB > 0 && (memUsedMB/memSysMB*100) > 80 {
		memStatus = "warning"
	}
	checks["memory"] = fiber.Map{
		"status":    memStatus,
		"used_mb":   int(memUsedMB),
		"sys_mb":    int(memSysMB),
		"gc_cycles": mem.NumGC,
	}

	// LLM API check
	llmStatus := "ok"
	var llmErr string
	if h.llmURL != "" {
		llmStart := time.Now()
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(h.llmURL)
		if err != nil {
			// Connection errors are expected if LLM isn't running, treat as warning
			llmStatus = "warning"
			llmErr = err.Error()
		} else {
			resp.Body.Close()
			if resp.StatusCode >= 500 {
				llmStatus = "error"
				llmErr = fmt.Sprintf("HTTP %d", resp.StatusCode)
			} else if resp.StatusCode >= 400 {
				llmStatus = "warning"
				llmErr = fmt.Sprintf("HTTP %d", resp.StatusCode)
			}
		}
		llmDuration := time.Since(llmStart).Milliseconds()
		// Mask endpoint URL — only show protocol+host, not path or key hints
		maskedEndpoint := maskURL(h.llmURL)
		checks["llm_api"] = fiber.Map{
			"status":      llmStatus,
			"response_ms": llmDuration,
			"endpoint":    maskedEndpoint,
			"error":       llmErr,
		}
	} else {
		checks["llm_api"] = fiber.Map{
			"status": "unknown",
			"error": "no endpoint configured",
		}
	}

	// ADB service check
	adbStatus := "ok"
	adbErr := ""
	adbDevices := 0
	if h.adbAddr != "" {
		// Check if ADB server is reachable by attempting a connection
		adbStart := time.Now()
		// ADB uses its own protocol, but we can check if the port is open
		// For now, just check if adb binary exists
		if _, err := os.Stat("/usr/bin/adb"); err != nil {
			if _, err := os.Stat("/usr/local/bin/adb"); err != nil {
				adbStatus = "warning"
				adbErr = "adb binary not found"
			}
		}
		adbDuration := time.Since(adbStart).Milliseconds()
		checks["adb"] = fiber.Map{
			"status":      adbStatus,
			"response_ms": adbDuration,
			"devices":     adbDevices,
			"error":       adbErr,
		}
	} else {
		checks["adb"] = fiber.Map{
			"status": "unknown",
			"error": "no address configured",
		}
	}

	// Build cache status
	buildCacheStatus := "ok"
	buildCacheDir := "data/builds"
	if info, err := os.Stat(buildCacheDir); err == nil && info.IsDir() {
		// Count files in build cache
		var fileCount int
		filepath.Walk(buildCacheDir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				fileCount++
			}
			return nil
		})
		checks["build_cache"] = fiber.Map{
			"status":     buildCacheStatus,
			"file_count": fileCount,
		}
	} else {
		checks["build_cache"] = fiber.Map{
			"status": "unknown",
			"error":  "cache directory not found",
		}
	}

	uptime := time.Since(startTime)
	uptimeStr := formatDuration(uptime)

	overallStatus := "healthy"
	for _, check := range checks {
		if m, ok := check.(fiber.Map); ok {
			if m["status"] == "error" {
				overallStatus = "degraded"
				break
			}
			if m["status"] == "warning" && overallStatus == "healthy" {
				overallStatus = "warning"
			}
		}
	}

	return c.JSON(fiber.Map{
		"status":    overallStatus,
		"version":   "2.0-lite",
		"uptime":    uptimeStr,
		"checked_at": checkedAt,
		"checks":    checks,
	})
}

// CacheStats returns cache statistics for the agent
func (h *HealthHandler) CacheStats(c fiber.Ctx) error {
	if h.agentRunner == nil {
		return c.JSON(fiber.Map{
			"status":  "unavailable",
			"message": "Agent runner not configured",
		})
	}

	stats := h.agentRunner.GetCacheStats()
	return c.JSON(fiber.Map{
		"status": "ok",
		"caches": stats,
	})
}

// DetailedHealth returns comprehensive health information
func (h *HealthHandler) DetailedHealth(c fiber.Ctx) error {
	checkStart := time.Now()
	checks := fiber.Map{}
	checkedAt := time.Now().Format(time.RFC3339)

	// Database check
	dbStart := time.Now()
	dbStatus := "ok"
	var dbErr string
	dbLatency := int64(0)
	if err := h.db.Ping(); err != nil {
		dbStatus = "error"
		dbErr = err.Error()
		checks["database"] = fiber.Map{
			"status":      dbStatus,
			"response_ms": dbLatency,
			"error":       dbErr,
		}
	} else {
		// Get database stats
		stats := h.db.Stats()
		dbLatency = time.Since(dbStart).Milliseconds()
		checks["database"] = fiber.Map{
			"status":       dbStatus,
			"response_ms":  dbLatency,
			"error":        dbErr,
			"open_conns":   stats.OpenConnections,
			"in_use":       stats.InUse,
			"wait_count":   stats.WaitCount,
			"wait_duration": stats.WaitDuration.Milliseconds(),
		}
	}

	// Disk check with detailed info
	diskStatus := "ok"
	var freeGB, totalGB, usedGB float64
	var diskErr string
	if diskInfo, err := getDiskInfo(); err == nil {
		freeGB = diskInfo.freeGB
		totalGB = diskInfo.totalGB
		usedGB = totalGB - freeGB
		if freeGB < 1.0 {
			diskStatus = "warning"
		}
	} else {
		diskStatus = "error"
		diskErr = err.Error()
	}
	checks["disk"] = fiber.Map{
		"status":   diskStatus,
		"free_gb":  freeGB,
		"used_gb":  usedGB,
		"total_gb": totalGB,
		"error":    diskErr,
	}

	// Memory check with detailed stats
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	memUsedMB := float64(mem.Alloc) / 1024 / 1024
	memSysMB := float64(mem.Sys) / 1024 / 1024
	memStatus := "ok"
	if memSysMB > 0 && (memUsedMB/memSysMB*100) > 80 {
		memStatus = "warning"
	}
	checks["memory"] = fiber.Map{
		"status":       memStatus,
		"used_mb":      int(memUsedMB),
		"sys_mb":       int(memSysMB),
		"gc_cycles":    mem.NumGC,
		"gc_pause_ns":  mem.PauseTotalNs,
		"heap_objects": mem.HeapObjects,
	}

	// LLM API check
	llmStatus := "ok"
	var llmErr string
	llmLatency := int64(0)
	if h.llmURL != "" {
		llmStart := time.Now()
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(h.llmURL)
		if err != nil {
			llmStatus = "warning"
			llmErr = err.Error()
		} else {
			resp.Body.Close()
			if resp.StatusCode >= 500 {
				llmStatus = "error"
				llmErr = fmt.Sprintf("HTTP %d", resp.StatusCode)
			} else if resp.StatusCode >= 400 {
				llmStatus = "warning"
				llmErr = fmt.Sprintf("HTTP %d", resp.StatusCode)
			}
		}
		llmLatency = time.Since(llmStart).Milliseconds()
		checks["llm_api"] = fiber.Map{
			"status":      llmStatus,
			"response_ms": llmLatency,
			"endpoint":    maskURL(h.llmURL),
			"error":       llmErr,
		}
	} else {
		checks["llm_api"] = fiber.Map{
			"status": "unknown",
			"error": "no endpoint configured",
		}
	}

	// ADB service check
	adbStatus := "ok"
	adbErr := ""
	adbDevices := 0
	adbStart := time.Now()
	if _, err := os.Stat("/usr/bin/adb"); err != nil {
		if _, err := os.Stat("/usr/local/bin/adb"); err != nil {
			adbStatus = "warning"
			adbErr = "adb binary not found"
		}
	}
	adbLatency := time.Since(adbStart).Milliseconds()
	checks["adb"] = fiber.Map{
		"status":      adbStatus,
		"response_ms": adbLatency,
		"devices":     adbDevices,
		"error":       adbErr,
	}

	// Build cache status
	buildCacheStatus := "ok"
	buildCacheDir := "data/builds"
	if info, err := os.Stat(buildCacheDir); err == nil && info.IsDir() {
		var fileCount int
		var totalSize int64
		filepath.Walk(buildCacheDir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				fileCount++
				totalSize += info.Size()
			}
			return nil
		})
		checks["build_cache"] = fiber.Map{
			"status":     buildCacheStatus,
			"file_count": fileCount,
			"total_size_mb": float64(totalSize) / 1024 / 1024,
		}
	} else {
		checks["build_cache"] = fiber.Map{
			"status": "unknown",
			"error":  "cache directory not found",
		}
	}

	// Runtime info
	checks["runtime"] = fiber.Map{
		"go_version":   runtime.Version(),
		"os":           runtime.GOOS,
		"arch":         runtime.GOARCH,
		"num_cpu":      runtime.NumCPU(),
		"num_goroutines": runtime.NumGoroutine(),
	}

	uptime := time.Since(startTime)
	totalCheckMs := time.Since(checkStart).Milliseconds()

	overallStatus := "healthy"
	for _, check := range checks {
		if m, ok := check.(fiber.Map); ok {
			if m["status"] == "error" {
				overallStatus = "degraded"
				break
			}
			if m["status"] == "warning" && overallStatus == "healthy" {
				overallStatus = "warning"
			}
		}
	}

	return c.JSON(fiber.Map{
		"status":      overallStatus,
		"version":     "2.0-lite",
		"uptime":      formatDuration(uptime),
		"checked_at":  checkedAt,
		"check_ms":    totalCheckMs,
		"checks":      checks,
	})
}

type diskInfo struct {
	freeGB  float64
	totalGB float64
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return itoa(days) + "d " + itoa(hours) + "h"
	}
	if hours > 0 {
		return itoa(hours) + "h " + itoa(mins) + "m"
	}
	return itoa(mins) + "m"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// maskURL masks sensitive parts of a URL, showing only scheme + host.
// e.g. "https://api.openai.com/v1/chat/completions" → "https://api.openai.com"
func maskURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "***"
	}
	return u.Scheme + "://" + u.Host
}
