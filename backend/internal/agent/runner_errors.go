package agent

import (
	"database/sql"
	"fmt"
	"strings"
)

// ToolRetryFallback provides fallback strategies when tool calls fail.
type ToolRetryFallback struct {
	db           *sql.DB
	currentModel string
}

// FallbackStrategy represents a fallback action.
type FallbackStrategy int

const (
	FallbackRetrySame FallbackStrategy = iota
	FallbackSimplifyTask
	FallbackSwitchModel
	FallbackForceAnswer
)

// GetFallback determines the best fallback strategy for a failed tool call.
func (trf *ToolRetryFallback) GetFallback(toolName string, err error, consecutiveFailures int) FallbackStrategy {
	errStr := err.Error()

	// If tool not found, try alternative
	if strings.Contains(errStr, "not found") || strings.Contains(errStr, "unknown skill") {
		return FallbackSimplifyTask
	}

	// If timeout, try with simpler input
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		if consecutiveFailures >= 2 {
			return FallbackSimplifyTask
		}
		return FallbackRetrySame
	}

	// If rate limited, switch model
	if strings.Contains(errStr, "rate") || strings.Contains(errStr, "429") {
		return FallbackSwitchModel
	}

	// If context too long, force answer
	if strings.Contains(errStr, "context_length") || strings.Contains(errStr, "max_tokens") {
		return FallbackForceAnswer
	}

	// Default: after 3 failures, simplify
	if consecutiveFailures >= 3 {
		return FallbackSimplifyTask
	}

	return FallbackRetrySame
}

// SimplifyTaskInput creates a simplified version of the tool input.
func (trf *ToolRetryFallback) SimplifyTaskInput(toolName string, input map[string]interface{}) map[string]interface{} {
	simplified := make(map[string]interface{})
	for k, v := range input {
		simplified[k] = v
	}

	switch toolName {
	case "write_file":
		// If content is too long, truncate and add marker
		if content, ok := simplified["content"].(string); ok && len(content) > 5000 {
			simplified["content"] = content[:5000] + "\n// ... (truncated due to size limit)"
		}
	case "bash":
		// If command is complex, simplify
		if cmd, ok := simplified["command"].(string); ok {
			if strings.Contains(cmd, "&&") {
				// Take only first command
				parts := strings.SplitN(cmd, "&&", 2)
				simplified["command"] = strings.TrimSpace(parts[0])
			}
		}
	case "build_module":
		// No simplification needed
	}

	return simplified
}

// ═══════════════════════════════════════════════════════════════════
// P0-2: ErrorClassifier — Classify errors and determine recovery strategy
// ═══════════════════════════════════════════════════════════════════

// ErrorCategory represents the type of error encountered.
type ErrorCategory int

const (
	ErrorUnknown      ErrorCategory = iota
	ErrorNetwork                    // Network timeout, connection refused
	ErrorAuth                       // Authentication failed, permission denied
	ErrorRateLimit                  // Rate limit exceeded (429)
	ErrorContext                    // Context too long
	ErrorToolNotFound               // Tool/skill not found
	ErrorPermission                 // File permission denied
	ErrorDiskSpace                  // Disk full
	ErrorSyntax                     // Code syntax error
	ErrorBuild                      // Build/compile error
)

// ClassifyError determines the error category from an error message.
func ClassifyError(errMsg string) ErrorCategory {
	msg := strings.ToLower(errMsg)

	// Network errors
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "connection refused") || strings.Contains(msg, "dial tcp") {
		return ErrorNetwork
	}

	// Auth errors
	if strings.Contains(msg, "unauthorized") || strings.Contains(msg, "401") ||
		strings.Contains(msg, "authentication") || strings.Contains(msg, "api key") {
		return ErrorAuth
	}

	// Rate limit
	if strings.Contains(msg, "rate") || strings.Contains(msg, "429") ||
		strings.Contains(msg, "too many requests") {
		return ErrorRateLimit
	}

	// Context too long
	if strings.Contains(msg, "context_length") || strings.Contains(msg, "maximum context") ||
		strings.Contains(msg, "max_tokens") || strings.Contains(msg, "token limit") {
		return ErrorContext
	}

	// Tool not found
	if strings.Contains(msg, "not found") || strings.Contains(msg, "unknown skill") ||
		strings.Contains(msg, "no such skill") {
		return ErrorToolNotFound
	}

	// Permission
	if strings.Contains(msg, "permission denied") || strings.Contains(msg, "access denied") ||
		strings.Contains(msg, "eacces") {
		return ErrorPermission
	}

	// Disk space
	if strings.Contains(msg, "no space") || strings.Contains(msg, "disk full") ||
		strings.Contains(msg, "enospc") {
		return ErrorDiskSpace
	}

	// Syntax errors
	if strings.Contains(msg, "syntax error") || strings.Contains(msg, "unexpected token") ||
		strings.Contains(msg, "parse error") {
		return ErrorSyntax
	}

	// Build errors
	if strings.Contains(msg, "build failed") || strings.Contains(msg, "compile error") ||
		strings.Contains(msg, "cannot find package") || strings.Contains(msg, "undefined:") {
		return ErrorBuild
	}

	return ErrorUnknown
}

// RecoveryStrategy represents the recommended recovery action.
type RecoveryStrategy int

const (
	RecoveryRetrySame      RecoveryStrategy = iota // Retry with same parameters
	RecoverySimplifyInput                          // Simplify task input
	RecoverySwitchModel                            // Switch to different model
	RecoveryForceAnswer                            // Force agent to provide answer
	RecoverySkipTool                               // Skip this tool and continue
	RecoveryCompactContext                         // Compact context and retry
	RecoveryAbort                                  // Abort immediately
)

// GetRecoveryStrategy determines the best recovery strategy for an error.
func GetRecoveryStrategy(category ErrorCategory, consecutiveFailures int) RecoveryStrategy {
	switch category {
	case ErrorNetwork:
		// Network: retry with backoff, after 3 failures abort
		if consecutiveFailures >= 3 {
			return RecoveryAbort
		}
		return RecoveryRetrySame

	case ErrorAuth:
		// Auth: try different provider, after 2 attempts abort
		if consecutiveFailures >= 2 {
			return RecoveryAbort
		}
		return RecoverySwitchModel

	case ErrorRateLimit:
		// Rate limit: switch model immediately
		return RecoverySwitchModel

	case ErrorContext:
		// Context too long: compact first, then force answer
		if consecutiveFailures >= 2 {
			return RecoveryForceAnswer
		}
		return RecoveryCompactContext

	case ErrorToolNotFound:
		// Tool not found: skip and continue
		return RecoverySkipTool

	case ErrorPermission:
		// Permission: skip and inform user
		return RecoverySkipTool

	case ErrorDiskSpace:
		// Disk full: abort immediately
		return RecoveryAbort

	case ErrorSyntax:
		// Syntax error: simplify input (maybe truncation caused it)
		if consecutiveFailures >= 2 {
			return RecoveryForceAnswer
		}
		return RecoverySimplifyInput

	case ErrorBuild:
		// Build error: let agent fix it
		return RecoverySkipTool

	default:
		// Unknown: retry once, then force answer
		if consecutiveFailures >= 2 {
			return RecoveryForceAnswer
		}
		return RecoveryRetrySame
	}
}

// GetRecoveryMessage returns a user-friendly message for the recovery strategy.
func GetRecoveryMessage(strategy RecoveryStrategy, toolName string) string {
	switch strategy {
	case RecoveryRetrySame:
		return fmt.Sprintf("工具 '%s' 执行失败，正在重试...", toolName)
	case RecoverySimplifyInput:
		return fmt.Sprintf("工具 '%s' 输入过复杂，正在简化...", toolName)
	case RecoverySwitchModel:
		return "当前模型限流，正在切换备用模型..."
	case RecoveryForceAnswer:
		return "多次重试失败，请基于已有信息给出答案"
	case RecoverySkipTool:
		return fmt.Sprintf("跳过工具 '%s'，继续执行...", toolName)
	case RecoveryCompactContext:
		return "上下文过长，正在压缩..."
	case RecoveryAbort:
		return "多次失败，终止执行"
	default:
		return fmt.Sprintf("工具 '%s' 执行异常", toolName)
	}
}

// getSessionCache returns (or creates) a session-scoped tool result cache.
// This cache persists across multiple Run() calls in the same session,
// avoiding redundant I/O when the LLM re-reads the same file in later rounds.
