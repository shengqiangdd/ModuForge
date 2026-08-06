package agent

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/moduforge/backend/internal/service"
)

// ═══════════════════════════════════════════════════════════════════
// O(1) Pattern Matching — Pre-compiled regex for claimsFileModification
//
// Instead of O(n*k) string contains checks (k=40 patterns),
// we use a single pre-compiled regex that matches any pattern in O(n).
// The regex is compiled once at init time and reused for all calls.
// ═══════════════════════════════════════════════════════════════════

// Pre-compiled regex for strong modification claims (Chinese + English).
// Matches any of the 40+ patterns in a single pass through the text.
var strongClaimsRegex = regexp.MustCompile(
	`(?:已修改|已写入|已保存|已更新|已创建|已完成` +
		`|have modified|have written|have saved|have created` +
		`|i modified|i wrote|i saved|i created|i updated` +
		`|修改了文件|写入了文件|保存了文件|更新了文件` +
		`|我修改了|我写入了|我保存了|我创建了|我更新了` +
		`|修改完成|写入完成|保存完成|更新完成|创建完成` +
		`|successfully modified|successfully wrote|successfully saved` +
		`|files modified|files written|file updated|file created` +
		`|will modify|will write|will create|will update` +
		`|需要修改|需要写入|需要创建|需要更新` +
		`|将要修改|将要写入|将要创建|将要更新` +
		`|计划修改|计划写入|计划创建|计划更新)`,
)

// Pre-compiled regex for file extension/path mentions.
var fileMentionRegex = regexp.MustCompile(
	`\.(?:java|sh|prop|xml|json|kt|cpp|go|h|rs|html|css|js|ts|sql|c|py)` +
		`|makefile|src/|lib/|main\.rs|lib\.rs`,
)

// claimsFileModification checks if the text claims to have modified files.
// O(n) via pre-compiled regex instead of O(n*k) string contains checks.
func claimsFileModification(text string) bool {
	lower := strings.ToLower(text)

	// O(n): Single regex scan for strong claims
	hasStrongClaim := strongClaimsRegex.MatchString(lower)

	// O(n): Single regex scan for file mentions (only if strong claim found)
	if hasStrongClaim {
		return fileMentionRegex.MatchString(lower)
	}

	return false
}

// detectLoop detects repetitive tool call patterns and returns an intervention message.
// O(1) via pre-computed counters in runMetrics instead of iterating through maps.
func detectLoop(toolCallHistory map[string]int, uniqueOps map[string]bool, totalCalls int,
	uniqueTargetsPerSkill map[string]int) string {
	// Total call budget — hard limit (raised to 15 for large projects)
	// O(1): Use pre-computed uniqueOps count
	if totalCalls >= 15 && len(uniqueOps) < totalCalls/2 {
		return fmt.Sprintf("Made %d tool calls total with only %d unique targets. You must stop using tools and provide your final answer now. Summarize what you accomplished.", totalCalls, len(uniqueOps))
	}

	// Per-skill threshold check — O(1) per skill via pre-computed counters
	for skill, count := range toolCallHistory {
		if count < 2 {
			continue
		}

		// O(1): Use pre-computed unique target count instead of iterating uniqueOps
		uniqueCount := uniqueTargetsPerSkill[skill]

		// read_file: special case — allow batch reads of different files, but enforce write after reading
		if skill == "read_file" {
			if uniqueCount <= 1 && count >= 2 {
				return "Reading the same file repeatedly. This is a loop. STOP reading. Use write_file/edit_file to make changes, or provide your final answer."
			}
			if count >= 3 && uniqueCount <= 2 {
				return fmt.Sprintf("You have called read_file %d times on only %d unique targets. This is a repetitive loop. STOP reading files. Provide your final answer now, or use write_file/edit_file to create/modify files.", count, uniqueCount)
			}
			// Excessive reads with low diversity (unique < 50% of total)
			if count >= 6 && uniqueCount < count/2 {
				return fmt.Sprintf("You have called read_file %d times on only %d unique targets. This is excessive. STOP reading files and start writing code immediately.", count, uniqueCount)
			}
			// Absolute cap: no matter how many unique files, 15+ reads without any write is a loop
			if count >= 15 {
				return fmt.Sprintf("You have called read_file %d times. Even with %d unique files, this is excessive reading. STOP reading. You MUST now use edit_file or write_file to make changes, then call build_module.", count, uniqueCount)
			}
		} else {
			// Other skills: 3+ calls = likely loop (reduced from 4)
			if count >= 3 {
				return fmt.Sprintf("Skill '%s' called %d times. This indicates a loop. STOP calling this skill. Try a different approach or provide your final answer.", skill, count)
			}
		}
	}

	return ""
}

// isGarbageOutput detects garbled/meaningless LLM output.
// Heuristics are lenient about code-heavy text (generics, templates, unicode).
func isGarbageOutput(text string) bool {
	if len(text) == 0 {
		return true
	}
	if len(strings.TrimSpace(text)) < 10 {
		return true // Too short to be meaningful
	}

	// Repeated full-width garbage patterns (e.g. ｜｜, |||)
	if strings.Contains(text, "｜｜") || strings.Contains(text, "|||") {
		return true
	}

	// Excessive special characters (garbled encoding)
	specialCount := 0
	xmlTagCount := 0
	for _, r := range text {
		switch {
		case r == '<' || r == '>':
			xmlTagCount++
		case r > 127 && r < 0x4E00, r >= 0xFF00 && r <= 0xFFFF:
			specialCount++
		}
	}
	if len(text) > 200 && float64(specialCount)/float64(len(text)) > 0.40 {
		return true
	}
	if xmlTagCount > 60 {
		return true
	}

	// Check for repetitive patterns (same line repeated)
	lines := strings.Split(text, "\n")
	if len(lines) > 10 {
		uniqueLines := make(map[string]bool)
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if len(trimmed) > 20 {
				uniqueLines[trimmed] = true
			}
		}
		if len(uniqueLines) < len(lines)/4 {
			return true // Too many repeated lines
		}
	}

	return false
}

// buildDiagnosticSummary builds a diagnostic summary of the conversation for forceAnswer.
func buildDiagnosticSummary(conversation []map[string]interface{}) string {
	var sb strings.Builder
	sb.WriteString("\n\n## Diagnostic Summary\n")

	// Count tool calls by type
	toolCounts := make(map[string]int)
	writeCalls := 0
	readCalls := 0
	for _, msg := range conversation {
		if toolCalls, ok := msg["tool_calls"].([]LLMToolCall); ok {
			for _, tc := range toolCalls {
				toolCounts[tc.Function.Name]++
				switch tc.Function.Name {
				case "write_file":
					writeCalls++
				case "read_file":
					readCalls++
				}
			}
		}
	}

	if len(toolCounts) > 0 {
		sb.WriteString(fmt.Sprintf("- Total tool calls: %d (read: %d, write: %d)\n", toolCountTotal(toolCounts), readCalls, writeCalls))
		sb.WriteString("Tool calls made:\n")
		for tool, count := range toolCounts {
			sb.WriteString(fmt.Sprintf("- %s: %d times\n", tool, count))
		}
	}

	return sb.String()
}

func toolCountTotal(counts map[string]int) int {
	total := 0
	for _, c := range counts {
		total += c
	}
	return total
}

// forceAnswer forces the LLM to produce a final answer when it's stuck in a loop.
func (r *AgentRunner) forceAnswer(ctx context.Context, conversation []map[string]interface{}, w SSEWriter, sessionID string, cfg RunConfig, reqProviderID, reqModel, reason string) error {
	log.Printf("[Agent] forceAnswer triggered: %s", reason)
	w.WriteSSE(map[string]interface{}{
		"type":    "step",
		"step":    "force_answer",
		"content": reason,
	})

	diagnostic := buildDiagnosticSummary(conversation)

	conversation = append(conversation, map[string]interface{}{
		"role":    "user",
		"content": reason + diagnostic + "\n\nIMPORTANT: Use write_file directly to create/modify files — it auto-creates parent directories.\n\nProvide your final answer now using clean Markdown formatting (## headings, - bullet lists, **bold**, `code`). Do NOT use any tools. Do NOT output raw tool syntax or XML tags.",
	})

	llmResp, err := r.callLLMWithTools(ctx, conversation, nil, w, cfg.UserID, reqProviderID, reqModel, cfg)
	if err != nil {
		log.Printf("[Agent] forceAnswer LLM call failed: %v", err)
		errMsg := fmt.Sprintf("生成最终回答时出错: %s", err.Error())
		w.WriteSSE(map[string]interface{}{"type": "step", "step": "answer", "content": errMsg})
		w.WriteSSEPlain("[DONE]")
		return nil
	}

	answer := cleanAnswer(llmResp.Content)
	if isGarbageOutput(answer) {
		log.Printf("[Agent] garbage answer from forceAnswer (len=%d), model may be unavailable", len(answer))
		answer = fmt.Sprintf("⚠️ Model returned garbled output. Based on the conversation, here is what was attempted:\n\n%s", reason)
	}

	w.WriteSSE(map[string]interface{}{
		"type":    "step",
		"step":    "answer",
		"content": answer,
	})

	// Save to conversation history
	if sessionID != "" {
		r.convStore.Append(sessionID, service.Message{Role: "assistant", Content: answer})
	}

	w.WriteSSEPlain("[DONE]")
	return nil
}
