package agent

import (
	"fmt"
	"strings"
	"sync"
)

// Optimization 40: String builder pooling — avoids repeated heap allocation
// in hot summarization paths (read_file, build_module, etc.)
var sbPool = sync.Pool{
	New: func() interface{} {
		sb := new(strings.Builder)
		sb.Grow(2048) // pre-allocate for typical summarize output
		return sb
	},
}

func getSB() *strings.Builder   { return sbPool.Get().(*strings.Builder) }
func putSB(sb *strings.Builder) { sb.Reset(); sbPool.Put(sb) }

// toolNameSet is a pre-built lookup for known tool names, replacing
// the previous O(n×m) slice scan with O(1) map access.
// Mirrors the skills registered in the agent registry.
var toolNameSet = map[string]bool{
	"read_file": true, "write_file": true, "write_file_batch": true,
	"edit_file": true, "grep_search": true, "glob_search": true,
	"list_dir": true, "delete_file": true, "delete_dir": true,
	"move_file": true, "bash": true, "build_module": true,
	"test_module": true, "agent_preset": true, "self_evolve": true,
	"pattern_learn": true, "memory_v2": true, "skill_manager": true,
	"self_reflection": true, "session_summary": true, "skill_registry": true,
	"context_manager": true, "task_delegator": true, "todo_manager": true,
}

func cleanAnswer(text string) string {
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		lowerTrim := strings.ToLower(trim)
		// Skip "function call: xxx(...)" lines
		if strings.HasPrefix(lowerTrim, "function call:") {
			continue
		}
		// Skip standalone tool name lines (e.g. "read_file")
		if toolNameSet[trim] {
			continue
		}
		// Skip "Tool 'xxx' not found" error lines
		if strings.HasPrefix(trim, "Tool '") && strings.Contains(trim, "not found") {
			continue
		}
		// Skip "Executing tool 'xxx'..." lines
		if strings.HasPrefix(trim, "Executing tool '") {
			continue
		}
		// Skip "Successfully wrote to 'xxx'" lines (redundant)
		if strings.HasPrefix(trim, "Successfully wrote to '") {
			continue
		}
		// Skip raw JSON tool call lines like {"type":"function","function":{...}}
		if strings.HasPrefix(trim, `{"type":"function"`) || strings.HasPrefix(trim, `{"type": "function"`) {
			continue
		}
		// Skip lines that look like tool invocation JSON ({"name":"read_file",...)
		if strings.HasPrefix(trim, `{"name":"`) && strings.Contains(trim, `"arguments"`) {
			continue
		}
		// Skip "Using tool xxx..." lines
		if strings.HasPrefix(lowerTrim, "using tool ") || strings.HasPrefix(lowerTrim, "call tool ") {
			continue
		}
		// Skip "I'll use/read/write..." without actual content (tool-talk)
		if lowerTrim == "i'll read the file" || lowerTrim == "let me read the file" ||
			lowerTrim == "i'll write the file" || lowerTrim == "let me write the file" {
			continue
		}
		cleaned = append(cleaned, line)
	}
	result := strings.Join(cleaned, "\n")
	// Collapse 3+ blank lines into 2
	for strings.Contains(result, "\n\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n\n", "\n\n\n")
	}
	return strings.TrimSpace(result)
}

// fixConcatenatedEnglish inserts spaces between English words that got
// merged by LLM output. Detects transitions like:
//   - lowercase→Uppercase: "theUser" → "the User"
//   - letter→digit: "v123" stays (version strings), but "file123abc" → "file123 abc"
//   - lowercase letter→opening paren with preceding uppercase: "createNewProject(" stays
//
// Only processes runs of ASCII letters; leaves CJK, URLs intact.
// Skips code blocks (``` fenced regions) to preserve code identifiers.
func fixConcatenatedEnglish(text string) string {
	// Fast path: no code block markers, process entire text
	if !strings.Contains(text, "```") {
		return fixConcatenatedEnglishPlain(text)
	}

	// Process line by line, tracking code block state
	var result strings.Builder
	inCodeBlock := false
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Toggle code block state on ``` markers
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
		}

		if inCodeBlock {
			// Preserve code blocks untouched
			result.WriteString(line)
		} else {
			result.WriteString(fixConcatenatedEnglishPlain(line))
		}

		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// fixConcatenatedEnglishPlain applies camelCase splitting to a single line
// of natural language text. No code-block awareness — the caller handles that.
func fixConcatenatedEnglishPlain(text string) string {
	runes := []rune(text)
	var out []rune
	n := len(runes)
	i := 0
	for i < n {
		r := runes[i]
		// Only operate on ASCII letter sequences (potential English words)
		if isASCIILetter(r) {
			// Collect the full run of ASCII letters
			start := i
			for i < n && isASCIILetter(runes[i]) {
				i++
			}
			word := string(runes[start:i])
			// Split camelCase within this run: "theUserAsked" → "the User Asked"
			split := splitCamelCase(word)
			for j, w := range split {
				if j > 0 {
					out = append(out, ' ')
				}
				out = append(out, []rune(w)...)
			}
			continue
		}
		// Keep everything else as-is
		out = append(out, r)
		i++
	}
	return string(out)
}

// isASCIILetter returns true for a-z A-Z.
func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// splitCamelCase breaks "theUserAsked" into ["the", "User", "Asked"].
func splitCamelCase(word string) []string {
	var parts []string
	var current strings.Builder
	for i, r := range word {
		if i > 0 && r >= 'A' && r <= 'Z' {
			// Upper case after lower case → word boundary
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// sanitizeReasoning cleans up reasoning/thinking content from LLM output.
// Some models produce garbled text: control characters, mojibake, repeated
// junk patterns, or half-rendered Unicode. This strips those artifacts.
func sanitizeReasoning(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	prevWasSpace := false
	for _, r := range text {
		// Skip ASCII control characters (except \n and \t)
		if r < 32 && r != '\n' && r != '\t' {
			continue
		}
		// Skip Unicode control/format characters (C0/C1 range)
		if (r >= 0x200B && r <= 0x200F) || // zero-width chars
			(r >= 0x2028 && r <= 0x202E) || // line/paragraph separators, bidi
			(r >= 0x2060 && r <= 0x2069) || // word joiners, invisible operators
			(r >= 0xFFF0 && r <= 0xFFFF) || // specials
			(r >= 0xE000 && r <= 0xF8FF) { // private use
			continue
		}
		// Collapse multiple spaces/newlines
		if r == ' ' || r == '\t' {
			if prevWasSpace {
				continue
			}
			prevWasSpace = true
			b.WriteRune(' ')
			continue
		}
		if r == '\n' {
			if prevWasSpace {
				continue
			}
			prevWasSpace = true
			b.WriteRune('\n')
			continue
		}
		prevWasSpace = false
		b.WriteRune(r)
	}
	result := b.String()
	// Trim leading/trailing whitespace
	result = strings.TrimSpace(result)
	// Fix concatenated English words (LLM sometimes merges words without spaces)
	result = fixConcatenatedEnglish(result)
	return result
}

func summarizeResult(result string, maxLen int) string {
	if len(result) <= maxLen {
		return result
	}
	return result[:maxLen] + "\n...(truncated)"
}

// isRetryableError determines if an error is worth retrying.
// Returns true for transient errors (network, upstream, rate limit, context too long).
// Returns false for permanent errors (auth, quota, unsupported region).
func isRetryableError(err error) bool {
	errStr := err.Error()
	// Always retry these transient patterns
	if strings.Contains(errStr, "Upstream request failed") ||
		strings.Contains(errStr, "Bad Gateway") ||
		strings.Contains(errStr, "Service Unavailable") ||
		strings.Contains(errStr, "Gateway Timeout") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "LLM stream interrupted") {
		return true
	}
	// Rate limit — retryable with backoff
	if strings.Contains(errStr, "HTTP 429") || strings.Contains(errStr, "quota_exceeded") {
		return true
	}
	// Context too long — retryable (compaction will shrink it)
	if strings.Contains(errStr, "context_length_exceeded") ||
		strings.Contains(errStr, "max_tokens") ||
		strings.Contains(errStr, "maximum context length") {
		return true
	}
	// 400 with "invalid_request" but transient upstream — retryable
	if strings.Contains(errStr, "HTTP 400") && strings.Contains(errStr, "Upstream") {
		return true
	}
	// Permanent errors — do NOT retry
	if strings.Contains(errStr, "unsupported_country_region_territory") ||
		strings.Contains(errStr, "invalid_api_key") ||
		strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "permission_denied") {
		return false
	}
	// Other 4xx (401, 403, 404 etc.) — generally non-retryable
	if strings.Contains(errStr, "HTTP 4") && !strings.Contains(errStr, "HTTP 429") && !strings.Contains(errStr, "HTTP 400") {
		return false
	}
	// 5xx — retryable
	if strings.Contains(errStr, "HTTP 5") {
		return true
	}
	return true // default: retry
}

// userFriendlyError returns a concise, user-friendly error message
func userFriendlyError(err error) string {
	errStr := err.Error()
	if strings.Contains(errStr, "unsupported_country_region_territory") {
		return "🚫 当前地区不支持此 LLM 提供商，请切换模型或使用代理"
	}
	if strings.Contains(errStr, "invalid_api_key") || strings.Contains(errStr, "401") {
		return "🔑 API Key 无效，请检查配置"
	}
	if strings.Contains(errStr, "quota_exceeded") || strings.Contains(errStr, "429") {
		return "⏳ 请求频率超限，请稍后再试"
	}
	if strings.Contains(errStr, "Upstream request failed") {
		return "🔧 上游 LLM 服务异常，已自动重试失败，请稍后再试或切换模型"
	}
	if strings.Contains(errStr, "context_length_exceeded") || strings.Contains(errStr, "maximum context length") {
		return "📏 对话上下文过长，已尝试压缩但仍超限，请开启新对话或切换支持长上下文的模型"
	}
	if strings.Contains(errStr, "HTTP 5") {
		return "🔧 LLM 服务暂时不可用，请稍后再试"
	}
	if strings.Contains(errStr, "HTTP 4") {
		return "⚠️ LLM 请求被拒绝（HTTP 4xx），请检查模型配置"
	}
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "context deadline exceeded") {
		return "⏱️ LLM 响应超时，请稍后再试"
	}
	if strings.Contains(errStr, "LLM stream interrupted") {
		return "🔌 LLM 连接中断，请检查网络或稍后重试"
	}
	// Truncate generic errors
	if len(errStr) > 100 {
		errStr = errStr[:100] + "..."
	}
	return fmt.Sprintf("❌ LLM 错误: %s", errStr)
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// smartSummarizeResult creates a structured summary for oversized tool results.
func smartSummarizeResult(result string, skillName string, maxLen int) string {
	if len(result) <= maxLen {
		return result
	}

	lines := strings.Split(result, "\n")

	switch skillName {
	case "read_file":
		return summarizeReadFile(lines, maxLen)
	case "build_module":
		return summarizeBuildOutput(lines, maxLen)
	default:
		return summarizeResult(result, maxLen)
	}
}

// summarizeReadFile extracts key structural info from source files.
func summarizeReadFile(lines []string, maxLen int) string {
	var sigs []string     // function/type signatures
	var keyLines []string // important lines (imports, structs, exports)
	totalLines := len(lines)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Collect function signatures (Go, Rust, Python, JS/TS)
		if strings.HasPrefix(trimmed, "func ") || strings.HasPrefix(trimmed, "fn ") ||
			strings.HasPrefix(trimmed, "def ") || strings.HasPrefix(trimmed, "function ") ||
			strings.HasPrefix(trimmed, "pub fn ") || strings.HasPrefix(trimmed, "pub struct ") ||
			strings.HasPrefix(trimmed, "struct ") || strings.HasPrefix(trimmed, "class ") ||
			strings.HasPrefix(trimmed, "export ") || strings.HasPrefix(trimmed, "export default ") {
			sigs = append(sigs, fmt.Sprintf("L%d: %s", i+1, trimmed))
		}
		// Collect imports, module declarations, const definitions
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "use ") ||
			strings.HasPrefix(trimmed, "package ") || strings.HasPrefix(trimmed, "module ") ||
			strings.HasPrefix(trimmed, "const ") || strings.HasPrefix(trimmed, "type ") ||
			strings.HasPrefix(trimmed, "pub type ") || strings.HasPrefix(trimmed, "pub const ") {
			keyLines = append(keyLines, fmt.Sprintf("L%d: %s", i+1, trimmed))
		}
	}

	sb := getSB()
	defer putSB(sb)
	sb.WriteString(fmt.Sprintf("=== File Summary (%d lines total) ===\n\n", totalLines))

	// Include first 30 lines (header/imports)
	headEnd := 30
	if headEnd > totalLines {
		headEnd = totalLines
	}
	sb.WriteString("## Header (lines 1-")
	sb.WriteString(fmt.Sprintf("%d):\n", headEnd))
	sb.WriteString(strings.Join(lines[:headEnd], "\n"))
	sb.WriteString("\n\n")

	// Include function signatures
	if len(sigs) > 0 {
		sb.WriteString("## Function/Type Signatures:\n")
		limit := 40
		if len(sigs) > limit {
			sb.WriteString(strings.Join(sigs[:limit], "\n"))
			sb.WriteString(fmt.Sprintf("\n... and %d more signatures\n", len(sigs)-limit))
		} else {
			sb.WriteString(strings.Join(sigs, "\n"))
		}
		sb.WriteString("\n\n")
	}

	// Include key definitions
	if len(keyLines) > 0 {
		sb.WriteString("## Key Definitions:\n")
		limit := 30
		if len(keyLines) > limit {
			sb.WriteString(strings.Join(keyLines[:limit], "\n"))
			sb.WriteString(fmt.Sprintf("\n... and %d more\n", len(keyLines)-limit))
		} else {
			sb.WriteString(strings.Join(keyLines, "\n"))
		}
		sb.WriteString("\n\n")
	}

	// Include last 20 lines (typically closing braces, tests, etc.)
	tailStart := totalLines - 20
	if tailStart < headEnd {
		tailStart = headEnd
	}
	if tailStart < totalLines {
		sb.WriteString(fmt.Sprintf("## Tail (lines %d-%d):\n", tailStart+1, totalLines))
		sb.WriteString(strings.Join(lines[tailStart:], "\n"))
	}

	result := sb.String()
	if len(result) > maxLen {
		result = result[:maxLen]
	}
	return result
}

// summarizeBuildOutput extracts error/warning lines from build output.
func summarizeBuildOutput(lines []string, maxLen int) string {
	var errors []string
	var warnings []string
	var status []string
	totalLines := len(lines)

	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "panicked") ||
			// P0-3: Go compiler diagnostics often omit the literal words "error"/"failed"
			// (e.g. "./main.go:5:2: undefined: foo"), so they were dropped from the
			// Errors section and the LLM never saw the real build failure.
			strings.Contains(lower, "undefined:") || strings.Contains(lower, "cannot ") ||
			strings.Contains(lower, "not enough arguments") || strings.Contains(lower, "too many arguments") ||
			strings.Contains(lower, "exit status") || strings.Contains(lower, "imported and not used") ||
			strings.Contains(lower, "implicit assignment to") || strings.Contains(lower, " undeclared") {
			errors = append(errors, line)
		} else if strings.Contains(lower, "warning") {
			warnings = append(warnings, line)
		} else if strings.Contains(lower, "finished") || strings.Contains(lower, "success") || strings.Contains(lower, "completed") {
			status = append(status, line)
		}
	}

	sb := getSB()
	defer putSB(sb)
	sb.WriteString(fmt.Sprintf("=== Build Output Summary (%d lines) ===\n\n", totalLines))

	if len(status) > 0 {
		sb.WriteString("## Status:\n")
		sb.WriteString(strings.Join(status, "\n"))
		sb.WriteString("\n\n")
	}

	if len(errors) > 0 {
		sb.WriteString(fmt.Sprintf("## Errors (%d):\n", len(errors)))
		limit := 30
		if len(errors) > limit {
			sb.WriteString(strings.Join(errors[:limit], "\n"))
			sb.WriteString(fmt.Sprintf("\n... and %d more errors\n", len(errors)-limit))
		} else {
			sb.WriteString(strings.Join(errors, "\n"))
		}
		sb.WriteString("\n\n")
	}

	if len(warnings) > 0 {
		sb.WriteString(fmt.Sprintf("## Warnings (%d, showing first 10):\n", len(warnings)))
		limit := 10
		if len(warnings) > limit {
			sb.WriteString(strings.Join(warnings[:limit], "\n"))
			sb.WriteString(fmt.Sprintf("\n... and %d more warnings\n", len(warnings)-limit))
		} else {
			sb.WriteString(strings.Join(warnings, "\n"))
		}
		sb.WriteString("\n")
	}

	if len(errors) == 0 && len(warnings) == 0 {
		// No errors/warnings found, keep tail
		tailStart := totalLines - 20
		if tailStart < 0 {
			tailStart = 0
		}
		sb.WriteString("## Output (last 20 lines):\n")
		sb.WriteString(strings.Join(lines[tailStart:], "\n"))
	}

	result := sb.String()
	if len(result) > maxLen {
		result = result[:maxLen]
	}
	return result
}
