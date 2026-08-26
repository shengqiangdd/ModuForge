package agent

import (
	"encoding/json"
	"sync"
	"time"
)

// PerfSnapshot holds a point-in-time performance snapshot.
type PerfSnapshot struct {
	Timestamp       time.Time       `json:"timestamp"`
	ActiveSessions  int             `json:"active_sessions"`
	TotalRequests   int64           `json:"total_requests"`
	RequestsPerMin  float64         `json:"requests_per_min"`
	AvgResponseTime float64         `json:"avg_response_time_ms"`
	ErrorRate       float64         `json:"error_rate"`
	LLMUsage        LLMUsageMetrics `json:"llm_usage"`
	GoroutineCount  int             `json:"goroutine_count"`
	MemoryUsageMB   float64         `json:"memory_usage_mb"`
	ToolCalls       map[string]int  `json:"tool_calls"`
	BuildStats      BuildStats      `json:"build_stats"`
}

// LLMUsageMetrics tracks LLM API usage.
type LLMUsageMetrics struct {
	TotalTokens      int64            `json:"total_tokens"`
	PromptTokens     int64            `json:"prompt_tokens"`
	CompletionTokens int64            `json:"completion_tokens"`
	CostEstimate     float64          `json:"cost_estimate_usd"`
	ByModel          map[string]int64 `json:"by_model"`
}

// BuildStats tracks build performance.
type BuildStats struct {
	TotalBuilds   int     `json:"total_builds"`
	SuccessBuilds int     `json:"success_builds"`
	FailedBuilds  int     `json:"failed_builds"`
	AvgBuildTime  float64 `json:"avg_build_time_sec"`
	SuccessRate   float64 `json:"success_rate"`
}

// PerfMonitor tracks real-time performance metrics.
type PerfMonitor struct {
	mu               sync.RWMutex
	snapshots        []PerfSnapshot
	maxSnapshots     int
	startTime        time.Time
	totalRequests    int64
	totalErrors      int64
	totalTokens      int64
	promptTokens     int64
	completionTokens int64
	buildStats       BuildStats
	toolCalls        map[string]int
	modelUsage       map[string]int64
	activeSessions   int
	subscribers      []chan PerfSnapshot
}

// NewPerfMonitor creates a new performance monitor.
func NewPerfMonitor() *PerfMonitor {
	return &PerfMonitor{
		maxSnapshots: 1440, // 24h at 1-min intervals
		startTime:    time.Now(),
		toolCalls:    make(map[string]int),
		modelUsage:   make(map[string]int64),
		subscribers:  make([]chan PerfSnapshot, 0),
	}
}

// RecordRequest records a completed request.
func (pm *PerfMonitor) RecordRequest(duration time.Duration, err bool, tokens int, model string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.totalRequests++
	if err {
		pm.totalErrors++
	}
	pm.totalTokens += int64(tokens)
	if model != "" {
		pm.modelUsage[model] += int64(tokens)
	}
}

// RecordToolCall records a tool invocation.
func (pm *PerfMonitor) RecordToolCall(toolName string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.toolCalls[toolName]++
}

// RecordPromptTokens records prompt token usage.
func (pm *PerfMonitor) RecordPromptTokens(prompt, completion int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.promptTokens += int64(prompt)
	pm.completionTokens += int64(completion)
}

// RecordBuild records a build outcome.
func (pm *PerfMonitor) RecordBuild(success bool, durationSec float64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.buildStats.TotalBuilds++
	if success {
		pm.buildStats.SuccessBuilds++
	} else {
		pm.buildStats.FailedBuilds++
	}
	pm.buildStats.AvgBuildTime = (pm.buildStats.AvgBuildTime*float64(pm.buildStats.TotalBuilds-1) + durationSec) / float64(pm.buildStats.TotalBuilds)
	if pm.buildStats.TotalBuilds > 0 {
		pm.buildStats.SuccessRate = float64(pm.buildStats.SuccessBuilds) / float64(pm.buildStats.TotalBuilds) * 100
	}
}

// SetActiveSessions updates the active session count.
func (pm *PerfMonitor) SetActiveSessions(count int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.activeSessions = count
}

// TakeSnapshot captures the current performance state.
func (pm *PerfMonitor) TakeSnapshot() PerfSnapshot {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	elapsed := time.Since(pm.startTime).Minutes()
	var reqPerMin float64
	if elapsed > 0 {
		reqPerMin = float64(pm.totalRequests) / elapsed
	}
	var errRate float64
	if pm.totalRequests > 0 {
		errRate = float64(pm.totalErrors) / float64(pm.totalRequests) * 100
	}

	costEstimate := float64(pm.totalTokens) / 1000.0 * 0.002

	tc := make(map[string]int)
	for k, v := range pm.toolCalls {
		tc[k] = v
	}

	mu := make(map[string]int64)
	for k, v := range pm.modelUsage {
		mu[k] = v
	}

	snap := PerfSnapshot{
		Timestamp:      time.Now(),
		ActiveSessions: pm.activeSessions,
		TotalRequests:  pm.totalRequests,
		RequestsPerMin: reqPerMin,
		ErrorRate:      errRate,
		LLMUsage: LLMUsageMetrics{
			TotalTokens:      pm.totalTokens,
			PromptTokens:     pm.promptTokens,
			CompletionTokens: pm.completionTokens,
			CostEstimate:     costEstimate,
			ByModel:          mu,
		},
		ToolCalls: tc,
		BuildStats: BuildStats{
			TotalBuilds:   pm.buildStats.TotalBuilds,
			SuccessBuilds: pm.buildStats.SuccessBuilds,
			FailedBuilds:  pm.buildStats.FailedBuilds,
			AvgBuildTime:  pm.buildStats.AvgBuildTime,
			SuccessRate:   pm.buildStats.SuccessRate,
		},
	}

	pm.snapshots = append(pm.snapshots, snap)
	if len(pm.snapshots) > pm.maxSnapshots {
		pm.snapshots = pm.snapshots[len(pm.snapshots)-pm.maxSnapshots:]
	}

	return snap
}

// Subscribe returns a channel that receives snapshots every interval.
func (pm *PerfMonitor) Subscribe(interval time.Duration) chan PerfSnapshot {
	ch := make(chan PerfSnapshot, 10)
	pm.mu.Lock()
	pm.subscribers = append(pm.subscribers, ch)
	pm.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			snap := pm.TakeSnapshot()
			select {
			case ch <- snap:
			default:
			}
		}
	}()

	return ch
}

// GetHistory returns the last N snapshots.
func (pm *PerfMonitor) GetHistory(n int) []PerfSnapshot {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if n > len(pm.snapshots) {
		n = len(pm.snapshots)
	}
	result := make([]PerfSnapshot, n)
	copy(result, pm.snapshots[len(pm.snapshots)-n:])
	return result
}

// GetSummary returns a JSON-serializable summary.
func (pm *PerfMonitor) GetSummary() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return map[string]interface{}{
		"uptime_seconds":  time.Since(pm.startTime).Seconds(),
		"total_requests":  pm.totalRequests,
		"total_errors":    pm.totalErrors,
		"total_tokens":    pm.totalTokens,
		"active_sessions": pm.activeSessions,
		"build_stats":     pm.buildStats,
		"tool_calls":      pm.toolCalls,
		"model_usage":     pm.modelUsage,
	}
}

// ToJSON serializes the snapshot to JSON.
func (s PerfSnapshot) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

// GetPerfMonitor returns the performance monitor (convenience accessor).
func (r *AgentRunner) GetPerfMonitor() *PerfMonitor {
	return r.perfMonitor
}

// GetCollabManager returns the collaborative session manager (convenience accessor).
func (r *AgentRunner) GetCollabManager() *CollabSessionManager {
	return r.collabManager
}
