package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionMemory tracks successful code generation patterns and fix strategies
// across sessions, enabling the agent to learn from past successes and failures.
type SessionMemory struct {
	mu         sync.Mutex
	successMap map[string][]string // module_type -> [successful_patterns]
	failureMap map[string][]string // error_type -> [fix_strategies]
	filePath   string
}

// MemoryRecord represents a single learning record from a past session.
type MemoryRecord struct {
	Timestamp  time.Time `json:"timestamp"`
	ModuleType string    `json:"module_type"`
	Language   string    `json:"language"`
	Patterns   []string  `json:"patterns"`
	Quality    int       `json:"quality"`
	BuildOK    bool      `json:"build_ok"`
	FixApplied string    `json:"fix_applied,omitempty"`
}

// NewSessionMemory creates a new session memory store.
func NewSessionMemory(dataDir string) *SessionMemory {
	if dataDir == "" {
		dataDir = filepath.Join(".", "data", "memory")
	}
	os.MkdirAll(dataDir, 0755)

	mem := &SessionMemory{
		successMap: make(map[string][]string),
		failureMap: make(map[string][]string),
		filePath:   filepath.Join(dataDir, "session_memory.json"),
	}
	mem.load()
	return mem
}

// RecordSuccess records a successful generation pattern for a module type.
func (sm *SessionMemory) RecordSuccess(moduleType, language string, patterns []string, quality int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.successMap[moduleType] = append(sm.successMap[moduleType], patterns...)
	// Keep max 50 patterns per module type
	if len(sm.successMap[moduleType]) > 50 {
		sm.successMap[moduleType] = sm.successMap[moduleType][len(sm.successMap[moduleType])-50:]
	}
	sm.save()
}

// RecordFailure records a failed error type and the fix strategy that worked.
func (sm *SessionMemory) RecordFailure(errorType, fixStrategy string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.failureMap[errorType] = append(sm.failureMap[errorType], fixStrategy)
	// Keep max 20 strategies per error type
	if len(sm.failureMap[errorType]) > 20 {
		sm.failureMap[errorType] = sm.failureMap[errorType][len(sm.failureMap[errorType])-20:]
	}
	sm.save()
}

// GetRecommendations returns successful patterns for a given module type.
func (sm *SessionMemory) GetRecommendations(moduleType string) []string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.successMap[moduleType]
}

// GetFixStrategy returns the most recent successful fix strategy for an error type.
func (sm *SessionMemory) GetFixStrategy(errorType string) string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if strategies, ok := sm.failureMap[errorType]; ok && len(strategies) > 0 {
		return strategies[len(strategies)-1]
	}
	return ""
}

// GetAllSuccessPatterns returns all recorded success patterns.
func (sm *SessionMemory) GetAllSuccessPatterns() map[string][]string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	result := make(map[string][]string, len(sm.successMap))
	for k, v := range sm.successMap {
		result[k] = v
	}
	return result
}

// GetAllFailureStrategies returns all recorded failure strategies.
func (sm *SessionMemory) GetAllFailureStrategies() map[string][]string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	result := make(map[string][]string, len(sm.failureMap))
	for k, v := range sm.failureMap {
		result[k] = v
	}
	return result
}

func (sm *SessionMemory) save() {
	data, err := json.MarshalIndent(struct {
		SuccessMap map[string][]string `json:"success_map"`
		FailureMap map[string][]string `json:"failure_map"`
	}{
		SuccessMap: sm.successMap,
		FailureMap: sm.failureMap,
	}, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(sm.filePath, data, 0644)
}

func (sm *SessionMemory) load() {
	data, err := os.ReadFile(sm.filePath)
	if err != nil {
		return
	}
	var raw struct {
		SuccessMap map[string][]string `json:"success_map"`
		FailureMap map[string][]string `json:"failure_map"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	if raw.SuccessMap != nil {
		sm.successMap = raw.SuccessMap
	}
	if raw.FailureMap != nil {
		sm.failureMap = raw.FailureMap
	}
}
