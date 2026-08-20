package agent

import (
	"fmt"
	"strings"
)

func NewToolResultPruner() *ToolResultPruner {
	return &ToolResultPruner{
		threshold:      2000, // Prune results > 2KB
		keepHead:       1500, // Keep first 1.5KB of content
		preserveWrites: true, // Always keep write/edit results
	}
}

// PruneResult compresses a tool result if it's too large.
// Returns (pruned_content, was_pruned).
func (tp *ToolResultPruner) PruneResult(content string, toolName string) (string, bool) {
	// Never prune write/edit results (they show what changed)
	if tp.preserveWrites {
		if toolName == "write_file" || toolName == "edit_file" {
			return content, false
		}
		if toolName == "bash" && len(content) <= 3000 {
			return content, false
		}
		if toolName == "bash" && len(content) > 3000 {
			return tp.pruneBashResult(content), true
		}
	}

	if len(content) <= tp.threshold {
		return content, false
	}

	// For read_file results: keep head + summary
	if toolName == "read_file" || toolName == "file_reader" {
		return tp.pruneReadFileResult(content), true
	}

	// For grep/glob results: keep head + count
	if toolName == "grep_search" || toolName == "glob_search" {
		return tp.pruneSearchResult(content), true
	}

	// Generic pruning: keep head + truncation marker
	pruned := content[:tp.keepHead]
	pruned += fmt.Sprintf("\n... [truncated, %d chars total]", len(content))
	return pruned, true
}

// pruneReadFileResult keeps the first N lines + file metadata.
func (tp *ToolResultPruner) pruneReadFileResult(content string) string {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	keepLines := 40
	if keepLines > totalLines {
		keepLines = totalLines
	}

	pruned := strings.Join(lines[:keepLines], "\n")
	if totalLines > keepLines {
		pruned += fmt.Sprintf("\n... [file has %d lines total, showing first %d for context]", totalLines, keepLines)
	}
	return pruned
}

// pruneSearchResult keeps the first N matches + summary count.
func (tp *ToolResultPruner) pruneSearchResult(content string) string {
	lines := strings.Split(content, "\n")
	totalMatches := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			totalMatches++
		}
	}

	keepLines := 30
	if keepLines > len(lines) {
		keepLines = len(lines)
	}

	pruned := strings.Join(lines[:keepLines], "\n")
	if totalMatches > keepLines {
		pruned += fmt.Sprintf("\n... [%d total matches, showing first %d]", totalMatches, keepLines)
	}
	return pruned
}

// pruneBashResult keeps the command + last N chars of output.
func (tp *ToolResultPruner) pruneBashResult(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}

	cmd := lines[0]
	if len(lines) <= 30 {
		return content
	}

	lastLines := lines[len(lines)-30:]
	return cmd + "\n... [output truncated] ...\n" + strings.Join(lastLines, "\n")
}

