package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/service"
)

func detectLoop(toolCallHistory map[string]int, uniqueOps map[string]bool, totalCalls int) string {
	// Per-skill threshold: allow parallel batch calls (e.g. 4 read_file in one iteration)
	// Only trigger when truly repetitive (same operation repeated many times)
	for skill, count := range toolCallHistory {
		uniqueCount := 0
		for op := range uniqueOps {
			if strings.HasPrefix(op, skill+":") {
				uniqueCount++
			}
		}
		// If the skill was called many times but on different targets, it's not a loop
		if skill == "read_file" {
			if count >= 6 && uniqueCount <= 3 {
				return fmt.Sprintf("You have called read_file %d times on only %d unique targets. This is a repetitive loop. STOP reading files. Provide your final answer now, or use write_file/edit_file to create/modify files.", count, uniqueCount)
			}
		} else if skill == "create_dir" {
			// Special case: create_dir loops are a common trap — the LLM thinks it's building structure but nothing happens
			if count >= 3 {
				return fmt.Sprintf("You have called create_dir %d times in a loop. This is NOT working — create_dir is not the right approach. STOP calling create_dir. Instead, use write_file to create each file directly (parent directories are auto-created). Do NOT call create_dir before write_file.", count)
			}
		} else {
			// For other skills, trigger at higher threshold
			if count >= 6 {
				return fmt.Sprintf("Skill '%s' called %d times. This indicates a loop. STOP calling this skill. Try a different approach or provide your final answer.", skill, count)
			}
		}
	}
	// Total call budget
	if totalCalls >= 12 {
		return fmt.Sprintf("Made %d tool calls total. You must stop using tools and provide your final answer now. Summarize what you accomplished.", totalCalls)
	}
	// Repeated reads of the same file
	readCount := toolCallHistory["read_file"]
	if readCount >= 3 {
		uniqueReads := 0
		for op := range uniqueOps {
			if strings.HasPrefix(op, "read_file:") {
				uniqueReads++
			}
		}
		if uniqueReads <= 1 && readCount >= 3 {
			return "Reading the same file repeatedly. This is a loop. STOP reading. Use write_file/edit_file to make changes, or provide your final answer."
		}
	}
	return ""
}

// isGarbageOutput detects garbled/meaningless LLM output.
// Adjusted thresholds: code-heavy responses often have special chars and XML tags.
func isGarbageOutput(text string) bool {
	if len(text) == 0 {
		return true
	}
	// Check for excessive special characters (garbled encoding)
	// Code snippets legitimately use special chars, so use a higher threshold
	specialCount := 0
	for _, r := range text {
		if r > 127 && r < 0x4E00 || (r >= 0xFF00 && r <= 0xFFFF) {
			specialCount++
		}
	}
	if len(text) > 200 && float64(specialCount)/float64(len(text)) > 0.40 {
		return true
	}
	// Check for random XML-like tags that aren't real markdown
	// Code often has <, > in generics/templates, so be lenient
	xmlTagCount := 0
	for _, r := range text {
		if r == '<' || r == '>' {
			xmlTagCount++
		}
	}
	if xmlTagCount > 60 {
		return true
	}
	// Check for repeated garbage patterns (e.g. "｜｜", "|||", random chars)
	if strings.Contains(text, "｜｜") || strings.Contains(text, "|||") {
		return true
	}
	// Too short to be meaningful
	if len(strings.TrimSpace(text)) < 10 {
		return true
	}
	return false
}

// buildDiagnosticSummary analyzes the conversation to produce a compact diagnostic
// that helps the LLM understand what happened when it's forced to answer.
func buildDiagnosticSummary(conversation []map[string]interface{}) string {
	toolCalls := 0
	writeCalls := 0
	readCalls := 0
	errors := 0
	var writtenFiles []string
	var failedTools []string

	for _, msg := range conversation {
		role, _ := msg["role"].(string)

		// Count tool calls from assistant messages
		if toolCallsList, ok := msg["tool_calls"].([]LLMToolCall); ok {
			for _, tc := range toolCallsList {
				toolCalls++
				switch tc.Function.Name {
				case "write_file":
					writeCalls++
					var args map[string]interface{}
					if json.Unmarshal([]byte(tc.Function.Arguments), &args) == nil {
						if p, ok := args["path"].(string); ok {
							writtenFiles = append(writtenFiles, p)
						}
					}
				case "read_file":
					readCalls++
				}
			}
		}

		// Count errors from tool results
		if role == "tool" {
			content, _ := msg["content"].(string)
			if strings.HasPrefix(content, "Error:") || strings.HasPrefix(content, "❌") ||
				strings.HasPrefix(content, "⚠️") || strings.Contains(content, "failed") {
				errors++
				// Extract tool name from nearby assistant message (best effort)
				if len(failedTools) < 5 {
					failedTools = append(failedTools, content)
				}
			}
		}
	}

	if toolCalls == 0 {
		return "" // no diagnostic needed
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n\n## DIAGNOSTIC (auto-generated)\n"))
	sb.WriteString(fmt.Sprintf("- Total tool calls: %d (read: %d, write: %d, errors: %d)\n", toolCalls, readCalls, writeCalls, errors))
	if len(writtenFiles) > 0 {
		sb.WriteString(fmt.Sprintf("- Files written: %s\n", strings.Join(writtenFiles, ", ")))
	}
	if errors > 0 {
		sb.WriteString(fmt.Sprintf("- Errors encountered: %d\n", errors))
		for i, err := range failedTools {
			if i >= 3 {
				sb.WriteString(fmt.Sprintf("  ... and %d more errors\n", len(failedTools)-3))
				break
			}
			// Truncate long error messages
			if len(err) > 150 {
				err = err[:150] + "..."
			}
			sb.WriteString(fmt.Sprintf("  - %s\n", err))
		}
	}
	sb.WriteString("\nUse this context to provide an accurate summary of what was accomplished and what failed.")

	return sb.String()
}

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
	// This prevents false positives like "The config file references X"
	hasFile := strings.Contains(lower, ".java") || strings.Contains(lower, ".sh") ||
		strings.Contains(lower, ".prop") || strings.Contains(lower, ".xml") ||
		strings.Contains(lower, ".json") || strings.Contains(lower, ".kt") ||
		strings.Contains(lower, ".cpp") || strings.Contains(lower, ".go") ||
		strings.Contains(lower, ".h") || strings.Contains(lower, ".rs") ||
		strings.Contains(lower, ".html") || strings.Contains(lower, ".css") ||
		strings.Contains(lower, ".js") || strings.Contains(lower, ".ts") ||
		strings.Contains(lower, ".sql") || strings.Contains(lower, ".c") ||
		strings.Contains(lower, "makefile") || strings.Contains(lower, ".py")

	// Only trigger on strong claims + file mention
	// Previously also triggered on "缺少"/"需要创建" which caused too many false positives
	return hasStrongClaim && hasFile
}

func (r *AgentRunner) forceAnswer(ctx context.Context, conversation []map[string]interface{}, w SSEWriter, sessionID string, cfg RunConfig, reqProviderID, reqModel, reason string) error {
	// Send keepalive heartbeat while LLM processes
	keepaliveCtx, keepaliveCancel := context.WithCancel(ctx)
	defer keepaliveCancel()
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-keepaliveCtx.Done():
				return
			case <-ticker.C:
				// Send empty step to keep connection alive
				_ = w.WriteSSE(map[string]interface{}{"type": "step", "step": "think", "content": ""})
			}
		}
	}()

	// Optimization 19: Inject diagnostic context into forceAnswer
	// Summarize what tools were called, what succeeded, what failed
	diagnostic := buildDiagnosticSummary(conversation)

	conversation = append(conversation, map[string]interface{}{
		"role":    "user",
		"content": reason + diagnostic + "\n\nIMPORTANT: If you were trying to create files, use write_file directly — it auto-creates parent directories. Do NOT use create_dir.\n\nProvide your final answer now using clean Markdown formatting (## headings, - bullet lists, **bold**, `code`). Do NOT use any tools. Do NOT output raw tool syntax or XML tags.",
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
	// If answer is garbled, don't retry with same model (it'll likely return garbage again)
	// Instead, return a specific error message
	if isGarbageOutput(answer) {
		log.Printf("[Agent] garbage answer from forceAnswer (len=%d), model may be unavailable", len(answer))
		answer = "⚠️ AI 返回了无法解析的内容，可能是模型服务暂时不可用。请稍后重试或切换到其他模型。"
	}
	if answer == "" || isGarbageOutput(answer) {
		answer = "Agent 未能生成有效回复。请尝试简化问题后重试。"
	}

	w.WriteSSE(map[string]interface{}{"type": "step", "step": "answer", "content": answer})
	if sessionID != "" {
		r.convStore.Append(sessionID, service.Message{Role: "assistant", Content: answer})
	}
	w.WriteSSEPlain("[DONE]")
	return nil
}
