package monitoring

import (
	"strings"
	"sync"
	"time"
)

// LogAggregator 日志聚合器
type LogAggregator struct {
	mu       sync.RWMutex
	entries  []LogEntry
	maxSize  int
	stopChan chan struct{}
}

// LogEntry 日志条目
type LogEntry struct {
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	RequestID string                 `json:"request_id,omitempty"`
	UserID    string                 `json:"user_id,omitempty"`
	Extra     map[string]interface{} `json:"extra,omitempty"`
}

// LogStats 日志统计
type LogStats struct {
	Total    int            `json:"total"`
	ByLevel  map[string]int `json:"by_level"`
	BySource map[string]int `json:"by_source"`
	Recent   []LogEntry     `json:"recent"`
}

// NewLogAggregator 创建日志聚合器
func NewLogAggregator(maxSize int) *LogAggregator {
	la := &LogAggregator{
		entries:  make([]LogEntry, 0),
		maxSize:  maxSize,
		stopChan: make(chan struct{}),
	}

	go la.cleanupLoop()

	return la
}

// Log 记录日志
func (la *LogAggregator) Log(entry LogEntry) {
	la.mu.Lock()
	defer la.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	la.entries = append(la.entries, entry)

	// 检查是否需要清理
	if len(la.entries) > la.maxSize {
		la.entries = la.entries[len(la.entries)-la.maxSize:]
	}
}

// Info 记录信息日志
func (la *LogAggregator) Info(source, message string) {
	la.Log(LogEntry{Level: "info", Source: source, Message: message})
}

// Warn 记录警告日志
func (la *LogAggregator) Warn(source, message string) {
	la.Log(LogEntry{Level: "warn", Source: source, Message: message})
}

// Error 记录错误日志
func (la *LogAggregator) Error(source, message string) {
	la.Log(LogEntry{Level: "error", Source: source, Message: message})
}

// GetStats 获取日志统计
func (la *LogAggregator) GetStats() LogStats {
	la.mu.RLock()
	defer la.mu.RUnlock()

	stats := LogStats{
		Total:    len(la.entries),
		ByLevel:  make(map[string]int),
		BySource: make(map[string]int),
		Recent:   make([]LogEntry, 0),
	}

	for _, entry := range la.entries {
		stats.ByLevel[entry.Level]++
		stats.BySource[entry.Source]++
	}

	// 最近100条
	limit := 100
	if limit > len(la.entries) {
		limit = len(la.entries)
	}
	stats.Recent = make([]LogEntry, limit)
	copy(stats.Recent, la.entries[len(la.entries)-limit:])

	return stats
}

// Search 搜索日志
func (la *LogAggregator) Search(query string, level string, limit int) []LogEntry {
	la.mu.RLock()
	defer la.mu.RUnlock()

	result := make([]LogEntry, 0)

	for i := len(la.entries) - 1; i >= 0; i-- {
		entry := la.entries[i]

		// 级别过滤
		if level != "" && entry.Level != level {
			continue
		}

		// 关键词搜索
		if query != "" && !strings.Contains(entry.Message, query) && !strings.Contains(entry.Source, query) {
			continue
		}

		result = append(result, entry)

		if len(result) >= limit {
			break
		}
	}

	return result
}

func (la *LogAggregator) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 定期清理已完成（内存中自动管理）
		case <-la.stopChan:
			return
		}
	}
}

// Stop 停止清理循环
func (la *LogAggregator) Stop() {
	close(la.stopChan)
}
