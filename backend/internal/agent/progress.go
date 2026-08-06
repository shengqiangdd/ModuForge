package agent

import (
	"fmt"
	"strings"
	"sync"
)

// ProgressTracker tracks task completion progress by analyzing agent actions.
// It parses LLM output for TODO/progress markers and maintains completion percentage.
type ProgressTracker struct {
	mu              sync.Mutex
	totalSteps      int
	completedSteps  int
	currentPhase    string
	fileOperations  map[string]bool // path -> modified
	buildStatus     string          // "pending", "pass", "fail"
	toolCallCount   int
	writeCallCount  int
	lastUpdate      string
	checkpoints     []ProgressCheckpoint
}

type ProgressCheckpoint struct {
	Phase    string
	Progress int
	Summary  string
}

// NewProgressTracker creates a new progress tracker.
func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		fileOperations: make(map[string]bool),
		buildStatus:    "pending",
		checkpoints:    make([]ProgressCheckpoint, 0),
	}
}

// RecordToolCall records a tool call and updates progress.
func (pt *ProgressTracker) RecordToolCall(toolName string, input map[string]interface{}, result string, success bool) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.toolCallCount++

	switch toolName {
	case "write_file", "write_file_batch":
		pt.writeCallCount++
		if path, ok := input["path"].(string); ok {
			pt.fileOperations[path] = true
		}
		// Writing files is progress
		pt.completedSteps++
		pt.lastUpdate = fmt.Sprintf("Written %s", pt.getFileName(input))

	case "edit_file":
		pt.writeCallCount++
		if path, ok := input["path"].(string); ok {
			pt.fileOperations[path] = true
		}
		pt.completedSteps++
		pt.lastUpdate = fmt.Sprintf("Edited %s", pt.getFileName(input))

	case "build_module":
		if success {
			pt.buildStatus = "pass"
			pt.lastUpdate = "Build passed"
		} else {
			pt.buildStatus = "fail"
			pt.lastUpdate = "Build failed"
		}

	case "read_file":
		// Reading is preparation, not completion
		pt.lastUpdate = fmt.Sprintf("Read %s", pt.getFileName(input))

	case "test_module":
		if success {
			pt.lastUpdate = "Tests passed"
		} else {
			pt.lastUpdate = "Tests failed"
		}
	}
}

// getFileName extracts filename from tool input.
func (pt *ProgressTracker) getFileName(input map[string]interface{}) string {
	if path, ok := input["path"].(string); ok {
		parts := strings.Split(path, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
		return path
	}
	return "unknown"
}

// SetTotalSteps sets the expected total steps (from task decomposition).
func (pt *ProgressTracker) SetTotalSteps(n int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.totalSteps = n
}

// SetPhase sets the current phase of execution.
func (pt *ProgressTracker) SetPhase(phase string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.currentPhase = phase
	pt.checkpoints = append(pt.checkpoints, ProgressCheckpoint{
		Phase:    phase,
		Progress: pt.getProgressPercent(),
		Summary:  pt.lastUpdate,
	})
}

// GetProgressPercent returns completion percentage (0-100).
func (pt *ProgressTracker) GetProgressPercent() int {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return pt.getProgressPercent()
}

func (pt *ProgressTracker) getProgressPercent() int {
	if pt.totalSteps > 0 {
		return min(100, pt.completedSteps*100/pt.totalSteps)
	}
	// Estimate based on activity
	if pt.writeCallCount == 0 {
		return 0
	}
	if pt.buildStatus == "pass" {
		return 90
	}
	// Rough estimate: each write is ~15% progress
	progress := pt.writeCallCount * 15
	if progress > 85 {
		progress = 85
	}
	return progress
}

// GetSummary returns a human-readable progress summary.
func (pt *ProgressTracker) GetSummary() string {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Progress: %d%%", pt.getProgressPercent()))
	sb.WriteString(fmt.Sprintf(" | Files: %d modified", len(pt.fileOperations)))
	sb.WriteString(fmt.Sprintf(" | Writes: %d", pt.writeCallCount))
	sb.WriteString(fmt.Sprintf(" | Tools: %d calls", pt.toolCallCount))

	if pt.buildStatus != "pending" {
		sb.WriteString(fmt.Sprintf(" | Build: %s", pt.buildStatus))
	}

	if pt.currentPhase != "" {
		sb.WriteString(fmt.Sprintf(" | Phase: %s", pt.currentPhase))
	}

	if pt.lastUpdate != "" {
		sb.WriteString(fmt.Sprintf(" | Last: %s", pt.lastUpdate))
	}

	return sb.String()
}

// GetProgressContext returns a string to inject into the system prompt.
func (pt *ProgressTracker) GetProgressContext() string {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.toolCallCount == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## CURRENT PROGRESS\n")
	sb.WriteString(fmt.Sprintf("- Completion: %d%%\n", pt.getProgressPercent()))
	sb.WriteString(fmt.Sprintf("- Files modified: %d\n", len(pt.fileOperations)))
	sb.WriteString(fmt.Sprintf("- Write operations: %d\n", pt.writeCallCount))

	if len(pt.fileOperations) > 0 {
		sb.WriteString("- Modified files:\n")
		count := 0
		for path := range pt.fileOperations {
			if count >= 10 {
				sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(pt.fileOperations)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("  - %s\n", path))
			count++
		}
	}

	if pt.buildStatus == "pass" {
		sb.WriteString("- Build: PASSED\n")
	} else if pt.buildStatus == "fail" {
		sb.WriteString("- Build: FAILED - needs fixing\n")
	}

	return sb.String()
}

// Reset resets the tracker for a new task.
func (pt *ProgressTracker) Reset() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.totalSteps = 0
	pt.completedSteps = 0
	pt.currentPhase = ""
	pt.fileOperations = make(map[string]bool)
	pt.buildStatus = "pending"
	pt.toolCallCount = 0
	pt.writeCallCount = 0
	pt.lastUpdate = ""
	pt.checkpoints = pt.checkpoints[:0]
}

// AnalyzeLLMOutput parses LLM output for progress markers.
func (pt *ProgressTracker) AnalyzeLLMOutput(content string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	lower := strings.ToLower(content)

	// Detect phase transitions
	if strings.Contains(lower, "reading") || strings.Contains(lower, "analyzing") {
		pt.currentPhase = "analysis"
	} else if strings.Contains(lower, "implementing") || strings.Contains(lower, "creating") || strings.Contains(lower, "writing") {
		pt.currentPhase = "implementation"
	} else if strings.Contains(lower, "testing") || strings.Contains(lower, "verifying") {
		pt.currentPhase = "verification"
	} else if strings.Contains(lower, "done") || strings.Contains(lower, "complete") || strings.Contains(lower, "finished") {
		pt.currentPhase = "completion"
	}

	// Parse TODO markers for progress hints
	if strings.Contains(content, "TODO:") || strings.Contains(content, "todo:") {
		pt.lastUpdate = "Working on TODO items"
	}
}
