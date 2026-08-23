package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ToolCallLog records a single tool invocation.
type ToolCallLog struct {
	ToolName  string        `json:"tool_name"`
	Input     string        `json:"input,omitempty"`
	Output    string        `json:"output,omitempty"`
	Duration  time.Duration `json:"duration"`
	Success   bool          `json:"success"`
	ErrorCode string        `json:"error_code,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// ToolStats holds aggregated statistics for a tool.
type ToolStats struct {
	TotalCalls  int      `json:"total_calls"`
	SuccessRate float64  `json:"success_rate"`
	AvgLatency  float64  `json:"avg_latency_ms"`
	ErrorRate   float64  `json:"error_rate"`
	LastErrors  []string `json:"last_errors,omitempty"`
}

// ToolRanking is a tool with its stats for dashboard display.
type ToolRanking struct {
	Name  string    `json:"name"`
	Stats ToolStats `json:"stats"`
}

// DashboardData is the monitoring dashboard snapshot.
type DashboardData struct {
	ToolRankings       []ToolRanking `json:"tool_rankings"`
	OverallSuccessRate float64       `json:"overall_success_rate"`
	TotalCalls         int           `json:"total_calls"`
}

// ToolMonitor tracks tool call statistics and provides circuit breaker.
type ToolMonitor struct {
	mu   sync.RWMutex
	dir  string
	logs []ToolCallLog
}

// NewToolMonitor creates a monitor backed by JSON in dataDir.
func NewToolMonitor(dataDir string) *ToolMonitor {
	return &ToolMonitor{dir: dataDir}
}

// RecordCall logs a tool invocation.
func (tm *ToolMonitor) RecordCall(log ToolCallLog) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if err := tm.load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load: %w", err)
	}

	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	tm.logs = append(tm.logs, log)

	// Keep only last 10000 logs
	if len(tm.logs) > 10000 {
		tm.logs = tm.logs[len(tm.logs)-10000:]
	}

	return tm.save()
}

// GetStats returns aggregated statistics for a tool.
func (tm *ToolMonitor) GetStats(toolName string) ToolStats {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tm.load()

	var total, successes int
	var totalLatency float64
	var lastErrors []string

	for _, log := range tm.logs {
		if log.ToolName != toolName {
			continue
		}
		total++
		if log.Success {
			successes++
		}
		totalLatency += float64(log.Duration.Milliseconds())
		if !log.Success && log.ErrorCode != "" {
			if len(lastErrors) < 5 {
				lastErrors = append(lastErrors, log.ErrorCode)
			}
		}
	}

	stats := ToolStats{
		TotalCalls: total,
		LastErrors: lastErrors,
	}

	if total > 0 {
		stats.SuccessRate = float64(successes) / float64(total) * 100
		stats.AvgLatency = totalLatency / float64(total)
		stats.ErrorRate = float64(total-successes) / float64(total) * 100
	}

	return stats
}

// CheckCircuitBreaker returns true if the tool should be paused (error rate > threshold).
func (tm *ToolMonitor) CheckCircuitBreaker(toolName string, threshold float64) bool {
	stats := tm.GetStats(toolName)
	if stats.TotalCalls < 10 {
		return false // Not enough data
	}
	return stats.ErrorRate > threshold
}

// GetDashboard returns dashboard data for all tools.
func (tm *ToolMonitor) GetDashboard() DashboardData {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tm.load()

	toolNames := make(map[string]bool)
	for _, log := range tm.logs {
		toolNames[log.ToolName] = true
	}

	var rankings []ToolRanking
	var totalCalls int
	var totalSuccesses int

	for name := range toolNames {
		var total, successes int
		var totalLatency float64

		for _, log := range tm.logs {
			if log.ToolName != name {
				continue
			}
			total++
			if log.Success {
				successes++
			}
			totalLatency += float64(log.Duration.Milliseconds())
		}

		stats := ToolStats{TotalCalls: total}
		if total > 0 {
			stats.SuccessRate = float64(successes) / float64(total) * 100
			stats.AvgLatency = totalLatency / float64(total)
			stats.ErrorRate = float64(total-successes) / float64(total) * 100
		}

		rankings = append(rankings, ToolRanking{Name: name, Stats: stats})
		totalCalls += total
		totalSuccesses += successes
	}

	// Sort by total calls descending
	for i := 0; i < len(rankings); i++ {
		for j := i + 1; j < len(rankings); j++ {
			if rankings[j].Stats.TotalCalls > rankings[i].Stats.TotalCalls {
				rankings[i], rankings[j] = rankings[j], rankings[i]
			}
		}
	}

	overallSuccessRate := 0.0
	if totalCalls > 0 {
		overallSuccessRate = float64(totalSuccesses) / float64(totalCalls) * 100
	}

	return DashboardData{
		ToolRankings:       rankings,
		OverallSuccessRate: overallSuccessRate,
		TotalCalls:         totalCalls,
	}
}

// GetRecentErrors returns recent error logs for a tool.
func (tm *ToolMonitor) GetRecentErrors(toolName string, limit int) []ToolCallLog {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tm.load()

	if limit <= 0 {
		limit = 10
	}

	var errors []ToolCallLog
	for i := len(tm.logs) - 1; i >= 0 && len(errors) < limit; i-- {
		if tm.logs[i].ToolName == toolName && !tm.logs[i].Success {
			errors = append(errors, tm.logs[i])
		}
	}

	return errors
}

// ═══════════════════════════════════════════════════════
// Internal helpers
// ═══════════════════════════════════════════════════════

func (tm *ToolMonitor) load() error {
	path := filepath.Join(tm.dir, "tool_calls.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &tm.logs)
}

func (tm *ToolMonitor) save() error {
	if err := os.MkdirAll(tm.dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tm.logs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(tm.dir, "tool_calls.json"), data, 0644)
}
