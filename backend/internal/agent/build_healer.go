package agent

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════
// Build Healer — Automatic build error recovery
// Inspired by: Claude Code's auto-fix loop, Cursor's error context
//
// When build_module fails, the healer:
// 1. Parses compiler/error output to extract structured diagnostics
// 2. Groups errors by file (most errors first = highest impact fix)
// 3. Injects targeted error context into conversation so the LLM can fix
// 4. Tracks healing attempts to prevent infinite fix loops
// ═══════════════════════════════════════════════════════════════════

// BuildDiagnostic represents a single parsed build error.
type BuildDiagnostic struct {
	File    string // file path (absolute or relative)
	Line    int    // line number (0 if unknown)
	Column  int    // column number (0 if unknown)
	Level   string // "error", "warning", "note"
	Message string // error message
	Raw     string // original error line
}

// BuildHealAttempt tracks healing progress for a single build failure.
type BuildHealAttempt struct {
	Iteration      int
	Diagnostics    []BuildDiagnostic
	FilesAffected  []string
	FilesFixed     []string
	FailedFiles    []string
	MaxAttempts    int
	CurrentAttempt int
}

// BuildHealer manages automatic build error recovery.
type BuildHealer struct {
	// Per-session state
	sessions map[string]*BuildHealAttempt
}

// NewBuildHealer creates a new build healer.
func NewBuildHealer() *BuildHealer {
	return &BuildHealer{
		sessions: make(map[string]*BuildHealAttempt),
	}
}

// ═══════════════════════════════════════════════════════════════════
// Error Parsing — Extract structured diagnostics from build output
// ═══════════════════════════════════════════════════════════════════

// Common compiler error patterns (order matters: most specific first)
var (
	// Go compiler: file.go:42:10: undefined: Foo
	goErrorPattern = regexp.MustCompile(`^(.+?):(\d+):(\d+)?:\s*(error|warning|note|panic):\s*(.+)$`)
	// Go vet / staticcheck
	goVetPattern = regexp.MustCompile(`^(.+?):(\d+):(\d+)?:\s*(.+)$`)
	// Rust compiler: error[E0412]: cannot find type
	rustErrorPattern = regexp.MustCompile(`^(error\[E\d+\]|error|warning\[w\d+\]|warning):?\s*(.+)$`)
	rustFilePattern  = regexp.MustCompile(`^\s*-->\s*(.+?):(\d+):(\d+)?`)
	// Generic: file.ext:line:col: error: message
	genericErrorPattern = regexp.MustCompile(`^(.+?):(\d+):?(\d+)?:?\s*(error|Error|ERROR|warning|Warning|WARNING):\s*(.+)$`)
	// Java/Kotlin
	javaPattern = regexp.MustCompile(`^(.+?):(\d+):\s*(error|warning):\s*(.+)$`)
)

// ParseBuildOutput parses raw build/error output into structured diagnostics.
// Uses a single-pass O(n) algorithm where n = number of lines.
func ParseBuildOutput(output string) []BuildDiagnostic {
	if output == "" {
		return nil
	}

	lines := strings.Split(output, "\n")
	diagnostics := make([]BuildDiagnostic, 0, len(lines)/2) // estimate ~50% are errors

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var diag BuildDiagnostic

		// Try Go compiler pattern first (most common in ModuForge)
		if m := goErrorPattern.FindStringSubmatch(line); m != nil {
			diag.File = m[1]
			diag.Line = parseIntSafe(m[2])
			diag.Column = parseIntSafe(m[3])
			diag.Level = m[4]
			diag.Message = m[5]
			diag.Raw = line
			diagnostics = append(diagnostics, diag)
			continue
		}

		// Rust compiler error with file location on next line
		if m := rustErrorPattern.FindStringSubmatch(line); m != nil {
			diag.Level = "error"
			diag.Message = m[2]
			diag.Raw = line
			// Check next line for file location (handled by caller if needed)
			diagnostics = append(diagnostics, diag)
			continue
		}

		// Rust file location: --> src/main.rs:42:10
		if m := rustFilePattern.FindStringSubmatch(line); m != nil {
			// Update the last diagnostic with file info
			if len(diagnostics) > 0 && diagnostics[len(diagnostics)-1].File == "" {
				diagnostics[len(diagnostics)-1].File = m[1]
				diagnostics[len(diagnostics)-1].Line = parseIntSafe(m[2])
				diagnostics[len(diagnostics)-1].Column = parseIntSafe(m[3])
			}
			continue
		}

		// Generic pattern
		if m := genericErrorPattern.FindStringSubmatch(line); m != nil {
			diag.File = m[1]
			diag.Line = parseIntSafe(m[2])
			diag.Column = parseIntSafe(m[3])
			diag.Level = strings.ToLower(m[4])
			diag.Message = m[5]
			diag.Raw = line
			diagnostics = append(diagnostics, diag)
			continue
		}

		// Java/Kotlin pattern
		if m := javaPattern.FindStringSubmatch(line); m != nil {
			diag.File = m[1]
			diag.Line = parseIntSafe(m[2])
			diag.Level = m[3]
			diag.Message = m[4]
			diag.Raw = line
			diagnostics = append(diagnostics, diag)
			continue
		}

		// Go vet pattern (less specific, try last)
		if m := goVetPattern.FindStringSubmatch(line); m != nil {
			// Only if it looks like an error/warning (has file extension)
			ext := filepath.Ext(m[1])
			if ext != "" && (strings.Contains(ext, ".go") || strings.Contains(ext, ".rs") ||
				strings.Contains(ext, ".js") || strings.Contains(ext, ".ts") ||
				strings.Contains(ext, ".java") || strings.Contains(ext, ".kt")) {
				diag.File = m[1]
				diag.Line = parseIntSafe(m[2])
				diag.Column = parseIntSafe(m[3])
				diag.Level = "error" // assume error for vet patterns
				diag.Message = m[4]
				diag.Raw = line
				diagnostics = append(diagnostics, diag)
				continue
			}
		}
	}

	return diagnostics
}

// parseIntSafe converts a string to int, returning 0 on failure.
func parseIntSafe(s string) int {
	if s == "" {
		return 0
	}
	n := 0
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		} else {
			return 0
		}
	}
	return n
}

// ═══════════════════════════════════════════════════════════════════
// Error Grouping — Group diagnostics by file for targeted fixing
// ═══════════════════════════════════════════════════════════════════

// FileErrors groups all diagnostics for a single file.
type FileErrors struct {
	FilePath    string
	Diagnostics []BuildDiagnostic
	ErrorCount  int
	WarningCount int
}

// GroupByFile groups diagnostics by file path, sorted by error count (descending).
// This ensures the most broken files are fixed first.
func GroupByFile(diagnostics []BuildDiagnostic) []FileErrors {
	groupMap := make(map[string]*FileErrors)

	for _, d := range diagnostics {
		if d.File == "" {
			continue
		}
		fe, ok := groupMap[d.File]
		if !ok {
			fe = &FileErrors{FilePath: d.File}
			groupMap[d.File] = fe
		}
		fe.Diagnostics = append(fe.Diagnostics, d)
		if d.Level == "error" {
			fe.ErrorCount++
		} else {
			fe.WarningCount++
		}
	}

	// Convert to slice and sort by error count (most errors first)
	result := make([]FileErrors, 0, len(groupMap))
	for _, fe := range groupMap {
		result = append(result, *fe)
	}

	// Sort by error count descending (simple insertion sort for small N)
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j].ErrorCount > result[j-1].ErrorCount; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}

	return result
}

// ═══════════════════════════════════════════════════════════════════
// Healing Strategy — Determine how to fix build errors
// ═══════════════════════════════════════════════════════════════════

// HealStrategy represents the approach to fix build errors.
type HealStrategy int

const (
	// HealAutoFix: LLM can fix automatically (most common)
	HealAutoFix HealStrategy = iota
	// HealSkipWarnings: Only warnings, skip and continue
	HealSkipWarnings
	// HealForceAnswer: Too many errors, stop and answer
	HealForceAnswer
	// HealAbort: Critical error, abort
	HealAbort
)

// DetermineHealStrategy decides the best healing approach based on diagnostics.
func DetermineHealStrategy(diagnostics []BuildDiagnostic, attempt int, projectPath string) HealStrategy {
	if len(diagnostics) == 0 {
		return HealSkipWarnings
	}

	errorCount := 0
	warningCount := 0
	hasPanic := false

	for _, d := range diagnostics {
		if d.Level == "error" {
			errorCount++
		} else {
			warningCount++
		}
		msgLower := strings.ToLower(d.Message)
		if strings.Contains(msgLower, "panic") || strings.Contains(msgLower, "segmentation") {
			hasPanic = true
		}
	}

	// Only warnings: skip
	if errorCount == 0 {
		return HealSkipWarnings
	}

	// Too many attempts: force answer
	if attempt >= 5 {
		return HealForceAnswer
	}

	// Panic: abort (needs careful manual fix)
	if hasPanic && attempt >= 2 {
		return HealAbort
	}

	// Default: auto-fix
	return HealAutoFix
}

// ═══════════════════════════════════════════════════════════════════
// Heal — Main entry point for build error recovery
// ═══════════════════════════════════════════════════════════════════

// HealResult contains the outcome of a healing attempt.
type HealResult struct {
	Strategy     HealStrategy
	Diagnostics  []BuildDiagnostic
	FileGroups   []FileErrors
	Attempt      int
	MaxAttempts  int
	ContextForLLM string // context to inject into conversation
	UserMessage   string // message to show user via SSE
	ShouldRetry   bool   // whether to retry build after LLM fixes
}

// Heal analyzes build output and produces a healing plan.
func (bh *BuildHealer) Heal(sessionID string, buildOutput string, projectPath string) *HealResult {
	// Get or create attempt tracker
	attempt, ok := bh.sessions[sessionID]
	if !ok {
		attempt = &BuildHealAttempt{
			MaxAttempts: 5,
		}
		bh.sessions[sessionID] = attempt
	}
	attempt.CurrentAttempt++

	// Parse diagnostics
	diagnostics := ParseBuildOutput(buildOutput)
	attempt.Diagnostics = diagnostics

	// Group by file
	fileGroups := GroupByFile(diagnostics)
	attempt.FilesAffected = make([]string, 0, len(fileGroups))
	for _, fg := range fileGroups {
		attempt.FilesAffected = append(attempt.FilesAffected, fg.FilePath)
	}

	// Determine strategy
	strategy := DetermineHealStrategy(diagnostics, attempt.CurrentAttempt, projectPath)

	// Build context for LLM
	contextForLLM := bh.buildHealingContext(diagnostics, fileGroups, strategy, attempt.CurrentAttempt)

	// Build user message
	userMessage := bh.buildUserMessage(diagnostics, fileGroups, strategy, attempt.CurrentAttempt, attempt.MaxAttempts)

	return &HealResult{
		Strategy:      strategy,
		Diagnostics:   diagnostics,
		FileGroups:    fileGroups,
		Attempt:       attempt.CurrentAttempt,
		MaxAttempts:   attempt.MaxAttempts,
		ContextForLLM: contextForLLM,
		UserMessage:    userMessage,
		ShouldRetry:    strategy == HealAutoFix || strategy == HealSkipWarnings,
	}
}

// Reset clears the healing state for a session (called when build succeeds).
func (bh *BuildHealer) Reset(sessionID string) {
	delete(bh.sessions, sessionID)
}

// ═══════════════════════════════════════════════════════════════════
// Context Generation — Build LLM context from build errors
// ═══════════════════════════════════════════════════════════════════

// buildHealingContext creates a targeted system message for the LLM to fix errors.
func (bh *BuildHealer) buildHealingContext(diagnostics []BuildDiagnostic, fileGroups []FileErrors, strategy HealStrategy, attempt int) string {
	if strategy == HealSkipWarnings {
		return "[System: Build completed with warnings only. No action needed.]"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[BUILD ERROR RECOVERY — Attempt %d/5]\n", attempt))
	sb.WriteString("The build failed with the following errors. You MUST fix ALL errors listed below.\n")
	sb.WriteString("Priority: Fix errors in the file with the MOST errors first.\n\n")

	// Group diagnostics by file
	for _, fg := range fileGroups {
		if fg.ErrorCount == 0 {
			continue // skip warning-only files
		}
		sb.WriteString(fmt.Sprintf("## File: %s (%d errors)\n", fg.FilePath, fg.ErrorCount))
		for _, d := range fg.Diagnostics {
			if d.Level != "error" {
				continue
			}
			loc := fmt.Sprintf("  Line %d", d.Line)
			if d.Column > 0 {
				loc += fmt.Sprintf(", Col %d", d.Column)
			}
			sb.WriteString(fmt.Sprintf("%s: %s\n", loc, d.Message))
			sb.WriteString(fmt.Sprintf("  Raw: %s\n", d.Raw))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Instructions\n")
	sb.WriteString("1. Read each affected file using read_file\n")
	sb.WriteString("2. Fix ALL errors in each file\n")
	sb.WriteString("3. Write the corrected file using write_file (COMPLETE content, not snippets)\n")
	sb.WriteString("4. After fixing all files, call build_module to verify\n")
	sb.WriteString("5. If new errors appear, fix them in the next iteration\n")

	return sb.String()
}

// buildUserMessage creates a user-facing message about the build failure.
func (bh *BuildHealer) buildUserMessage(diagnostics []BuildDiagnostic, fileGroups []FileErrors, strategy HealStrategy, attempt, maxAttempts int) string {
	errorCount := 0
	for _, d := range diagnostics {
		if d.Level == "error" {
			errorCount++
		}
	}

	switch strategy {
	case HealAutoFix:
		return fmt.Sprintf("🔨 构建失败 (%d 个错误)，Agent 正在自动修复... (尝试 %d/%d)",
			errorCount, attempt, maxAttempts)
	case HealSkipWarnings:
		return "🔨 构建完成（仅有警告，可忽略）"
	case HealForceAnswer:
		return fmt.Sprintf("⚠️ 构建仍有 %d 个错误，已尝试 %d 次，Agent 将基于当前进度给出答案", errorCount, attempt)
	case HealAbort:
		return "❌ 构建出现严重错误，已终止自动修复"
	default:
		return fmt.Sprintf("🔨 构建失败: %d 个错误", errorCount)
	}
}

// ═══════════════════════════════════════════════════════════════════
// Healing Integration — Connect to the agent runner
// ═══════════════════════════════════════════════════════════════════

// HandleBuildFailure processes a build failure and returns healing instructions.
// This is the main integration point called by the agent runner after build_module fails.
func (bh *BuildHealer) HandleBuildFailure(ctx context.Context, sessionID string, buildOutput string, projectPath string, w SSEWriter) (*HealResult, bool) {
	result := bh.Heal(sessionID, buildOutput, projectPath)

	log.Printf("[BuildHealer] session=%s attempt=%d strategy=%d errors=%d files=%d",
		sessionID, result.Attempt, result.Strategy, len(result.Diagnostics), len(result.FileGroups))

	// Send user notification
	w.WriteSSE(map[string]interface{}{
		"type":    "step",
		"step":    "build_heal",
		"content": result.UserMessage,
		"attempt": result.Attempt,
		"total":   result.MaxAttempts,
	})

	switch result.Strategy {
	case HealAutoFix:
		// Return true = inject context and let LLM fix
		return result, true

	case HealSkipWarnings:
		// Build succeeded (warnings only), reset and continue
		bh.Reset(sessionID)
		return result, false

	case HealForceAnswer:
		// Too many attempts, force final answer
		return result, false

	case HealAbort:
		// Critical error, abort
		return result, false

	default:
		return result, false
	}
}
