package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/moduforge/backend/internal/service"
)

// claimsFileModification checks if the text claims to have modified files.
func claimsFileModification(text string) bool {
	lower := strings.ToLower(text)

	// Strong signal: explicitly claims to have written/saved/modified files
	strongClaims := []string{
		"已修改", "已写入", "已保存", "已更新", "已创建",
		"have modified", "have written", "have saved", "have created",
		"i modified", "i wrote", "i saved", "i created", "i updated",
		"修改了文件", "写入了文件", "保存了文件",
	}
	hasStrongClaim := false
	for _, s := range strongClaims {
		if strings.Contains(lower, s) {
			hasStrongClaim = true
			break
		}
	}

	// Weak signal: mentions files that need changes (only triggers with strong claim)
	hasFile := strings.Contains(lower, ".java") || strings.Contains(lower, ".sh") ||
		strings.Contains(lower, ".prop") || strings.Contains(lower, ".xml") ||
		strings.Contains(lower, ".json") || strings.Contains(lower, ".kt") ||
		strings.Contains(lower, ".cpp") || strings.Contains(lower, ".go") ||
		strings.Contains(lower, ".h") || strings.Contains(lower, ".rs") ||
		strings.Contains(lower, ".html") || strings.Contains(lower, ".css") ||
		strings.Contains(lower, ".js") || strings.Contains(lower, ".ts") ||
		strings.Contains(lower, ".sql") || strings.Contains(lower, ".c") ||
		strings.Contains(lower, "makefile") || strings.Contains(lower, ".py")

	return hasStrongClaim && hasFile
}

// detectLoop detects repetitive tool call patterns and returns an intervention message.
func detectLoop(toolCallHistory map[string]int, uniqueOps map[string]bool, totalCalls int) string {
	// Total call budget — hard limit
	if totalCalls >= 12 {
		return fmt.Sprintf("Made %d tool calls total. You must stop using tools and provide your final answer now. Summarize what you accomplished.", totalCalls)
	}

	// Per-skill threshold check
	for skill, count := range toolCallHistory {
		if count < 3 {
			continue // No need to check skills called fewer than 3 times
		}

		// Count unique targets for this skill
		uniqueCount := 0
		for op := range uniqueOps {
			if strings.HasPrefix(op, skill+":") {
				uniqueCount++
			}
		}

		// read_file: special case — allow batch reads of different files
		if skill == "read_file" {
			// Same file read 3+ times = loop
			if uniqueCount <= 1 && count >= 3 {
				return "Reading the same file repeatedly. This is a loop. STOP reading. Use write_file/edit_file to make changes, or provide your final answer."
			}
			// Many reads but few unique targets = loop
			if count >= 6 && uniqueCount <= 3 {
				return fmt.Sprintf("You have called read_file %d times on only %d unique targets. This is a repetitive loop. STOP reading files. Provide your final answer now, or use write_file/edit_file to create/modify files.", count, uniqueCount)
			}
		} else {
			// Other skills: 6+ calls = likely loop
			if count >= 6 {
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
