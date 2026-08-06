package agent

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// ErrorRecoveryStrategy defines how to handle different types of tool failures.
type ErrorRecoveryStrategy struct {
	MaxRetries      int
	RetryDelay      time.Duration
	BackoffMultiplier float64
	FallbackAction  string // "retry", "simplify", "skip", "abort"
}

// DefaultRecoveryStrategies returns recovery strategies for different error types.
func DefaultRecoveryStrategies() map[string]ErrorRecoveryStrategy {
	return map[string]ErrorRecoveryStrategy{
		"network": {
			MaxRetries:       3,
			RetryDelay:       2 * time.Second,
			BackoffMultiplier: 2.0,
			FallbackAction:   "retry",
		},
		"timeout": {
			MaxRetries:       2,
			RetryDelay:       5 * time.Second,
			BackoffMultiplier: 2.0,
			FallbackAction:   "simplify",
		},
		"file_not_found": {
			MaxRetries:       1,
			RetryDelay:       0,
			BackoffMultiplier: 1,
			FallbackAction:   "skip",
		},
		"permission": {
			MaxRetries:       0,
			RetryDelay:       0,
			BackoffMultiplier: 1,
			FallbackAction:   "skip",
		},
		"syntax": {
			MaxRetries:       2,
			RetryDelay:       1 * time.Second,
			BackoffMultiplier: 1,
			FallbackAction:   "retry",
		},
		"build": {
			MaxRetries:       3,
			RetryDelay:       2 * time.Second,
			BackoffMultiplier: 1.5,
			FallbackAction:   "retry",
		},
		"rate_limit": {
			MaxRetries:       3,
			RetryDelay:       10 * time.Second,
			BackoffMultiplier: 2.0,
			FallbackAction:   "retry",
		},
		"unknown": {
			MaxRetries:       1,
			RetryDelay:       1 * time.Second,
			BackoffMultiplier: 1,
			FallbackAction:   "retry",
		},
	}
}

// ClassifyToolError determines the error category from an error message.
func ClassifyToolError(errMsg string) string {
	msg := strings.ToLower(errMsg)

	// Network errors
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "connection refused") || strings.Contains(msg, "dial tcp") ||
		strings.Contains(msg, "network") {
		return "network"
	}

	// Rate limiting
	if strings.Contains(msg, "rate") || strings.Contains(msg, "429") ||
		strings.Contains(msg, "too many requests") {
		return "rate_limit"
	}

	// File not found
	if strings.Contains(msg, "not found") || strings.Contains(msg, "no such file") ||
		strings.Contains(msg, "does not exist") {
		return "file_not_found"
	}

	// Permission errors
	if strings.Contains(msg, "permission denied") || strings.Contains(msg, "access denied") ||
		strings.Contains(msg, "eacces") {
		return "permission"
	}

	// Syntax errors
	if strings.Contains(msg, "syntax error") || strings.Contains(msg, "unexpected token") ||
		strings.Contains(msg, "parse error") {
		return "syntax"
	}

	// Build errors
	if strings.Contains(msg, "build failed") || strings.Contains(msg, "compile error") ||
		strings.Contains(msg, "cannot find package") || strings.Contains(msg, "undefined:") {
		return "build"
	}

	return "unknown"
}

// RecoveryTracker tracks recovery attempts per tool call.
type RecoveryTracker struct {
	attempts map[string]int // toolCallID -> attempt count
	strategies map[string]ErrorRecoveryStrategy
}

// NewRecoveryTracker creates a new recovery tracker.
func NewRecoveryTracker() *RecoveryTracker {
	return &RecoveryTracker{
		attempts:   make(map[string]int),
		strategies: DefaultRecoveryStrategies(),
	}
}

// ShouldRetry determines if a failed tool call should be retried.
func (rt *RecoveryTracker) ShouldRetry(toolCallID string, errorMsg string) (bool, time.Duration) {
	category := ClassifyToolError(errorMsg)
	strategy, ok := rt.strategies[category]
	if !ok {
		strategy = rt.strategies["unknown"]
	}

	attempt := rt.attempts[toolCallID]
	if attempt >= strategy.MaxRetries {
		return false, 0
	}

	rt.attempts[toolCallID]++
	delay := time.Duration(float64(strategy.RetryDelay) * pow(strategy.BackoffMultiplier, float64(attempt)))
	log.Printf("[Recovery] retrying %s (attempt %d/%d, category=%s, delay=%v)",
		toolCallID, attempt+1, strategy.MaxRetries, category, delay)

	return true, delay
}

// RecordSuccess records a successful tool call (clears retry state).
func (rt *RecoveryTracker) RecordSuccess(toolCallID string) {
	delete(rt.attempts, toolCallID)
}

// RecordFailure records a failed tool call.
func (rt *RecoveryTracker) RecordFailure(toolCallID string) {
	rt.attempts[toolCallID]++
}

// GetAttemptCount returns the number of retry attempts for a tool call.
func (rt *RecoveryTracker) GetAttemptCount(toolCallID string) int {
	return rt.attempts[toolCallID]
}

// Reset clears all recovery state.
func (rt *RecoveryTracker) Reset() {
	rt.attempts = make(map[string]int)
}

// pow computes base^exp (simple implementation to avoid math import).
func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

// ToolErrorContext provides additional context for error recovery.
type ToolErrorContext struct {
	ToolName     string
	ToolCallID   string
	ErrorMessage string
	Category     string
	Attempt      int
	MaxAttempts  int
	Suggestion   string
}

// GetRecoverySuggestion returns a human-readable suggestion for the error.
func GetRecoverySuggestion(toolName, errorMsg string) string {
	category := ClassifyToolError(errorMsg)

	switch category {
	case "network":
		return "Network error — will auto-retry with backoff"
	case "timeout":
		return "Timeout — try simplifying the request or using smaller input"
	case "file_not_found":
		return "File not found — check if the path is correct"
	case "permission":
		return "Permission denied — check file permissions"
	case "syntax":
		return "Syntax error — review the file for syntax issues"
	case "build":
		return "Build error — read the error output and fix the code"
	case "rate_limit":
		return "Rate limited — waiting before retry"
	default:
		return fmt.Sprintf("Error in %s — check the output and try again", toolName)
	}
}
