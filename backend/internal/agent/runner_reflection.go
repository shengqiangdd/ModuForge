package agent

import (
	"fmt"
	"sync"
	"time"
)

// ReflectionEvent records an agent's self-reflection about its actions.
type ReflectionEvent struct {
	Timestamp time.Time
	ToolName  string
	Action    string // "success", "failure", "retry", "skip"
	Reason    string
	Iteration int
}

// ReflectionLog tracks agent reflections for debugging and improvement.
type ReflectionLog struct {
	events []ReflectionEvent
	mu     sync.Mutex
}

// NewReflectionLog creates a new reflection log.
func NewReflectionLog() *ReflectionLog {
	return &ReflectionLog{
		events: make([]ReflectionEvent, 0, 100),
	}
}

// Record adds a reflection event.
func (rl *ReflectionLog) Record(toolName, action, reason string, iteration int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.events = append(rl.events, ReflectionEvent{
		Timestamp: time.Now(),
		ToolName:  toolName,
		Action:    action,
		Reason:    reason,
		Iteration: iteration,
	})
}

// GetSummary returns a summary of reflections.
func (rl *ReflectionLog) GetSummary() string {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if len(rl.events) == 0 {
		return "无反思记录"
	}

	successCount := 0
	failureCount := 0
	retryCount := 0
	for _, e := range rl.events {
		switch e.Action {
		case "success":
			successCount++
		case "failure":
			failureCount++
		case "retry":
			retryCount++
		}
	}

	return fmt.Sprintf("反思统计: 成功=%d, 失败=%d, 重试=%d, 总计=%d",
		successCount, failureCount, retryCount, len(rl.events))
}

