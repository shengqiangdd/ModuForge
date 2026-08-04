package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditLog records all tool operations for replay and forensics.
type AuditLog struct {
	mu       sync.Mutex
	filePath string
	entries  []AuditEntry
	maxSize  int // max entries before rotation
}

type AuditEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	SessionID   string                 `json:"session_id"`
	ToolName    string                 `json:"tool_name"`
	ToolCallID  string                 `json:"tool_call_id"`
	Parameters  map[string]interface{} `json:"parameters"`
	Result      string                 `json:"result"`
	Success     bool                   `json:"success"`
	Duration    int64                  `json:"duration_ms"`
	Error       string                 `json:"error,omitempty"`
	Iteration   int                    `json:"iteration"`
	UserID      string                 `json:"user_id,omitempty"`
	ProjectID   string                 `json:"project_id,omitempty"`
}

// NewAuditLog creates a new audit log.
func NewAuditLog(logDir string) *AuditLog {
	if logDir == "" {
		logDir = filepath.Join(".", "data", "audit")
	}
	os.MkdirAll(logDir, 0755)

	filePath := filepath.Join(logDir, fmt.Sprintf("audit_%s.jsonl", time.Now().Format("2006-01-02")))

	al := &AuditLog{
		filePath: filePath,
		entries:  make([]AuditEntry, 0, 100),
		maxSize:  10000,
	}

	// Rotate if needed
	al.rotateIfNeeded()

	return al
}

// Record adds an audit entry.
func (al *AuditLog) Record(entry AuditEntry) {
	al.mu.Lock()
	defer al.mu.Unlock()

	al.entries = append(al.entries, entry)

	// Write to file
	f, err := os.OpenFile(al.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[AuditLog] failed to open file: %v", err)
		return
	}
	defer f.Close()

	data, _ := json.Marshal(entry)
	f.Write(data)
	f.WriteString("\n")
}

// RecordToolCall is a convenience method for recording tool execution.
func (al *AuditLog) RecordToolCall(sessionID, toolName, toolCallID string, params map[string]interface{}, result string, success bool, duration time.Duration, iteration int, userID, projectID string) {
	errMsg := ""
	if !success {
		errMsg = result
	}

	al.Record(AuditEntry{
		Timestamp:  time.Now(),
		SessionID:  sessionID,
		ToolName:   toolName,
		ToolCallID: toolCallID,
		Parameters: params,
		Result:     result,
		Success:    success,
		Duration:   duration.Milliseconds(),
		Error:      errMsg,
		Iteration:  iteration,
		UserID:     userID,
		ProjectID:  projectID,
	})
}

// GetHistory returns recent audit entries, optionally filtered.
func (al *AuditLog) GetHistory(toolName string, limit int) []AuditEntry {
	al.mu.Lock()
	defer al.mu.Unlock()

	if limit <= 0 {
		limit = 100
	}

	var result []AuditEntry
	for i := len(al.entries) - 1; i >= 0 && len(result) < limit; i-- {
		if toolName == "" || al.entries[i].ToolName == toolName {
			result = append(result, al.entries[i])
		}
	}
	return result
}

// GetToolStats returns statistics about tool usage.
func (al *AuditLog) GetToolStats() map[string]ToolStats {
	al.mu.Lock()
	defer al.mu.Unlock()

	stats := make(map[string]ToolStats)
	for _, entry := range al.entries {
		s := stats[entry.ToolName]
		s.TotalCalls++
		if entry.Success {
			s.SuccessCalls++
		} else {
			s.FailureCalls++
		}
		s.TotalDuration += entry.Duration
		if entry.Duration > 0 {
			s.Count++
			s.AvgDuration = s.TotalDuration / int64(s.Count)
		}
		stats[entry.ToolName] = s
	}
	return stats
}

type ToolStats struct {
	TotalCalls   int   `json:"total_calls"`
	SuccessCalls int   `json:"success_calls"`
	FailureCalls int   `json:"failure_calls"`
	TotalDuration int64 `json:"total_duration_ms"`
	AvgDuration  int64 `json:"avg_duration_ms"`
	Count        int   `json:"count"`
}

// rotateIfNeeded rotates the log file if it's too large.
func (al *AuditLog) rotateIfNeeded() {
	info, err := os.Stat(al.filePath)
	if err != nil {
		return
	}
	if info.Size() > 10*1024*1024 { // 10MB
		backup := al.filePath + ".old"
		os.Rename(al.filePath, backup)
		log.Printf("[AuditLog] rotated log file to %s", backup)
	}
}

// Close flushes any pending writes.
func (al *AuditLog) Close() {
	al.mu.Lock()
	defer al.mu.Unlock()
	// All writes are immediate, nothing to flush
}
