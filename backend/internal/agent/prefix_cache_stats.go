package agent

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════
// Stats — GetStats methods for PrefixCache and SemanticCache
// ═══════════════════════════════════════════════════════════════════

// GetStats returns PrefixCache statistics.
func (pc *PrefixCache) GetStats() map[string]interface{} {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	total := pc.hits + pc.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(pc.hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"entries":  len(pc.entries),
		"hits":     pc.hits,
		"misses":   pc.misses,
		"hit_rate": fmt.Sprintf("%.1f%%", hitRate),
	}
}

// GetStats returns SemanticCache statistics.
func (sc *SemanticCache) GetStats() map[string]interface{} {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	total := sc.hits + sc.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(sc.hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"entries":   len(sc.entries),
		"hits":      sc.hits,
		"misses":    sc.misses,
		"hit_rate":  fmt.Sprintf("%.1f%%", hitRate),
		"threshold": sc.similarityThreshold,
	}
}

// ═══════════════════════════════════════════════════════════════════
// Context Condenser — LLM-based context summarization
//
// Based on OpenHands LLMSummarizingCondenser:
// 1. Keep recent messages intact
// 2. Summarize older messages with LLM
// 3. Preserve key information (file paths, decisions, errors)
// 4. Maintain continuity across summarizations
//
// Reference: https://docs.openhands.dev/sdk/guides/context-condenser
// ═══════════════════════════════════════════════════════════════════

// ContextCondenser manages intelligent context compression.
type ContextCondenser struct {
	// Configuration
	maxContextLength int // Max messages before condensing
	keepRecent       int // Number of recent messages to keep intact
	keepFirst        int // Number of first messages to keep (system prompt)

	// State
	summaryHistory []string // Previous summaries for continuity
	mu             sync.RWMutex
}

// NewContextCondenser creates a new context condenser.
func NewContextCondenser(maxContextLength, keepRecent, keepFirst int) *ContextCondenser {
	if maxContextLength <= 0 {
		maxContextLength = 30
	}
	if keepRecent <= 0 {
		keepRecent = 6
	}
	if keepFirst <= 0 {
		keepFirst = 1 // Always keep system prompt
	}
	return &ContextCondenser{
		maxContextLength: maxContextLength,
		keepRecent:       keepRecent,
		keepFirst:        keepFirst,
		summaryHistory:   make([]string, 0),
	}
}

// ShouldCondense checks if the conversation needs condensing.
func (cc *ContextCondenser) ShouldCondense(conversation []map[string]interface{}) bool {
	return len(conversation) > cc.maxContextLength
}

// Condense performs intelligent context compression.
// Returns the condensed conversation and whether compression was applied.
func (cc *ContextCondenser) Condense(
	conversation []map[string]interface{},
	llmCaller func(messages []map[string]string) (string, error),
) ([]map[string]interface{}, bool) {
	if !cc.ShouldCondense(conversation) {
		return conversation, false
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Split conversation into three parts:
	// 1. First messages (system prompt, etc.) - always keep
	// 2. Middle messages (to be summarized)
	// 3. Recent messages (to keep in full)

	if len(conversation) <= cc.keepFirst+cc.keepRecent {
		return conversation, false
	}

	firstMessages := conversation[:cc.keepFirst]
	middleMessages := conversation[cc.keepFirst : len(conversation)-cc.keepRecent]
	recentMessages := conversation[len(conversation)-cc.keepRecent:]

	// Build summary of middle messages
	summary, err := cc.buildSummary(middleMessages, llmCaller)
	if err != nil {
		log.Printf("[ContextCondenser] summary failed: %v, using heuristic", err)
		summary = cc.heuristicSummary(middleMessages)
	}

	// Build condensed conversation
	condensed := make([]map[string]interface{}, 0, len(firstMessages)+1+len(recentMessages))
	condensed = append(condensed, firstMessages...)

	// Add summary as system message
	condensed = append(condensed, map[string]interface{}{
		"role":    "system",
		"content": fmt.Sprintf("[Context Summary]\n%s", summary),
	})

	condensed = append(condensed, recentMessages...)

	log.Printf("[ContextCondenser] condensed %d messages to %d (kept %d first, %d recent, summary=%d chars)",
		len(conversation), len(condensed), len(firstMessages), len(recentMessages), len(summary))

	return condensed, true
}

// buildSummary creates an LLM-based summary of messages.
func (cc *ContextCondenser) buildSummary(
	messages []map[string]interface{},
	llmCaller func(messages []map[string]string) (string, error),
) (string, error) {
	if llmCaller == nil {
		return "", fmt.Errorf("no LLM caller provided")
	}

	// Build the conversation text
	var convText strings.Builder
	for _, msg := range messages {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		if content == "" {
			continue
		}

		label := "User"
		switch role {
		case "assistant":
			label = "Agent"
		case "system":
			label = "System"
		case "tool":
			label = "Tool Result"
		}

		// Truncate very long messages
		if len(content) > 2000 {
			content = content[:2000] + "...[truncated]"
		}

		convText.WriteString(fmt.Sprintf("%s: %s\n\n", label, content))
	}

	// Include previous summary for continuity
	var prompt []map[string]string
	if len(cc.summaryHistory) > 0 {
		lastSummary := cc.summaryHistory[len(cc.summaryHistory)-1]
		prompt = append(prompt, map[string]string{
			"role":    "system",
			"content": fmt.Sprintf("You are a conversation summarizer. You have a previous summary:\n\n%s\n\nNow summarize the following NEW messages, incorporating key points from the previous summary.", lastSummary),
		})
	} else {
		prompt = append(prompt, map[string]string{
			"role":    "system",
			"content": `You are a conversation summarizer for a coding agent. Summarize the conversation, preserving:
- File paths mentioned or modified
- Key decisions and their reasons
- Errors encountered and how they were resolved
- Current work in progress
- User constraints and requirements

Be concise but complete. Output ONLY the summary text, no labels.`,
		})
	}

	prompt = append(prompt, map[string]string{
		"role":    "user",
		"content": convText.String(),
	})

	summary, err := llmCaller(prompt)
	if err != nil {
		return "", err
	}

	// Store summary for future continuity
	cc.summaryHistory = append(cc.summaryHistory, summary)
	if len(cc.summaryHistory) > 5 {
		cc.summaryHistory = cc.summaryHistory[len(cc.summaryHistory)-5:]
	}

	return summary, nil
}

// heuristicSummary creates a zero-LLM-cost summary.
func (cc *ContextCondenser) heuristicSummary(messages []map[string]interface{}) string {
	var fileChanges []string
	var decisions []string
	var errors []string
	var rounds int

	for _, msg := range messages {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)

		if role == "user" {
			rounds++
		}

		// Extract file changes
		if isFileChangeResult(content) {
			if fc := extractFileChange(content); fc != "" {
				fileChanges = append(fileChanges, fc)
			}
		}

		// Extract decisions
		if containsDecision(content) {
			if len(content) > 200 {
				content = content[:200]
			}
			decisions = append(decisions, content)
		}

		// Extract errors
		if strings.HasPrefix(content, "Error:") || strings.HasPrefix(content, "❌") {
			if len(content) > 150 {
				content = content[:150]
			}
			errors = append(errors, content)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Summary of %d messages (%d rounds):\n\n", len(messages), rounds))

	if len(fileChanges) > 0 {
		sb.WriteString(fmt.Sprintf("Files modified (%d):\n", len(fileChanges)))
		for _, fc := range fileChanges {
			if len(fileChanges) > 10 && sb.Len() > 500 {
				sb.WriteString(fmt.Sprintf("  ... and %d more files\n", len(fileChanges)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("  - %s\n", fc))
		}
	}

	if len(decisions) > 0 {
		sb.WriteString(fmt.Sprintf("\nKey decisions (%d):\n", len(decisions)))
		for _, d := range decisions {
			sb.WriteString(fmt.Sprintf("  - %s\n", d))
		}
	}

	if len(errors) > 0 {
		sb.WriteString(fmt.Sprintf("\nErrors (%d):\n", len(errors)))
		for _, e := range errors {
			sb.WriteString(fmt.Sprintf("  - %s\n", e))
		}
	}

	return sb.String()
}

// GetStats returns condenser statistics.
func (cc *ContextCondenser) GetStats() map[string]interface{} {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	return map[string]interface{}{
		"max_context_length": cc.maxContextLength,
		"keep_recent":        cc.keepRecent,
		"keep_first":         cc.keepFirst,
		"summary_count":      len(cc.summaryHistory),
	}
}

// ═══════════════════════════════════════════════════════════════════
// Session Learning — Learn from successful patterns
//
// Based on pattern learning research:
// 1. Track successful tool call sequences
// 2. Cache successful patterns for reuse
// 3. Suggest patterns for similar tasks
// 4. Reduce redundant exploration
// ═══════════════════════════════════════════════════════════════════

// SessionLearner tracks and learns from successful patterns.
type SessionLearner struct {
	patterns []LearnedPattern
	mu       sync.RWMutex

	// Configuration
	maxPatterns int
}

// LearnedPattern represents a learned successful pattern.
type LearnedPattern struct {
	TaskType     string    // e.g., "fix_rust_error", "add_feature"
	ToolSequence []string  // Sequence of tools that worked
	FileTypes    []string  // File types involved
	SuccessRate  float64   // Historical success rate
	LastUsed     time.Time
}

// NewSessionLearner creates a new session learner.
func NewSessionLearner(maxPatterns int) *SessionLearner {
	if maxPatterns <= 0 {
		maxPatterns = 100
	}
	return &SessionLearner{
		patterns:    make([]LearnedPattern, 0, maxPatterns),
		maxPatterns: maxPatterns,
	}
}

// RecordPattern records a successful tool call sequence.
func (sl *SessionLearner) RecordPattern(taskType string, toolSequence []string, fileTypes []string) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	// Find existing pattern or create new
	for i := range sl.patterns {
		if sl.patterns[i].TaskType == taskType {
			sl.patterns[i].ToolSequence = toolSequence
			sl.patterns[i].FileTypes = fileTypes
			sl.patterns[i].SuccessRate = (sl.patterns[i].SuccessRate*0.9 + 1.0*0.1) // Exponential moving average
			sl.patterns[i].LastUsed = time.Now()
			return
		}
	}

	// New pattern
	if len(sl.patterns) >= sl.maxPatterns {
		sl.patterns = sl.patterns[1:] // Remove oldest
	}

	sl.patterns = append(sl.patterns, LearnedPattern{
		TaskType:     taskType,
		ToolSequence: toolSequence,
		FileTypes:    fileTypes,
		SuccessRate:  1.0,
		LastUsed:     time.Now(),
	})
}

// SuggestPattern suggests a pattern for a given task type.
func (sl *SessionLearner) SuggestPattern(taskType string) *LearnedPattern {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	for i := range sl.patterns {
		if sl.patterns[i].TaskType == taskType && sl.patterns[i].SuccessRate > 0.5 {
			return &sl.patterns[i]
		}
	}
	return nil
}

// GetStats returns learner statistics.
func (sl *SessionLearner) GetStats() map[string]interface{} {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	return map[string]interface{}{
		"patterns":     len(sl.patterns),
		"max_patterns": sl.maxPatterns,
	}
}
