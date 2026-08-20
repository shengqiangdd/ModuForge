package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/moduforge/backend/internal/service"
)

// isFileChangeResult reports whether a tool result records a file modification
// (write_file/edit_file/move_file) so it can be preserved during compaction.
// P0-1: edit_file results ("File edited:") were previously missed, so edits were
// silently dropped from compacted summaries even though writes were kept.
func isFileChangeResult(content string) bool {
	return strings.Contains(content, "Successfully wrote") ||
		strings.Contains(content, "File edited:") ||
		strings.Contains(content, "File moved:") ||
		strings.Contains(content, "write_file")
}

// extractFileChange extracts the first file-change line from a tool result for
// use in compaction key-facts summaries.
func extractFileChange(content string) string {
	for _, marker := range []string{"Successfully wrote", "File edited:", "File moved:"} {
		if idx := strings.Index(content, marker); idx >= 0 {
			end := strings.Index(content[idx:], "\n")
			if end < 0 {
				end = len(content[idx:])
			}
			return content[idx : idx+end]
		}
	}
	return ""
}

// containsDecision reports whether a message records a decision, conclusion, or
// plan that must be preserved in full during compaction. P1-6: mirrors the
// heuristic-compaction decision extraction so incremental compaction does not
// truncate critical user constraints and agent choices to 200 chars.
func containsDecision(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "decided") || strings.Contains(lower, "decision") ||
		strings.Contains(lower, "conclusion") || strings.Contains(lower, "chose") ||
		strings.Contains(lower, "approach") || strings.Contains(lower, "plan") ||
		strings.Contains(lower, "agree") || strings.Contains(lower, "disagree")
}

// heuristicCompactHistory provides zero-LLM-cost summarization for free models.
// Optimization 28: Extracts key facts (file paths, decisions, errors) and keeps
// the most recent messages, discarding the rest. Much cheaper than calling an LLM
// which itself consumes precious tokens from the free model's small context window.
func (r *AgentRunner) heuristicCompactHistory(history []service.Message) []service.Message {
	var fileChanges []string
	var decisions []string
	var errors []string

	for _, msg := range history {
		content := msg.Content
		// Extract file paths from write/edit results
		if isFileChangeResult(content) {
			if fc := extractFileChange(content); fc != "" {
				fileChanges = append(fileChanges, fc)
			}
		}
		// Extract decisions (assistant messages with key phrases)
		if msg.Role == "assistant" {
			lower := strings.ToLower(content)
			if strings.Contains(lower, "decided") || strings.Contains(lower, "decision") ||
				strings.Contains(lower, "chose") || strings.Contains(lower, "approach") {
				if len(content) > 200 {
					content = content[:200] + "..."
				}
				decisions = append(decisions, content)
			}
		}
		// Extract errors
		if strings.HasPrefix(content, "Error:") || strings.HasPrefix(content, "❌") ||
			strings.HasPrefix(content, "⚠️") {
			if len(content) > 150 {
				content = content[:150] + "..."
			}
			errors = append(errors, content)
		}
	}

	// Build compact summary
	var sb strings.Builder
	sb.WriteString("[上下文压缩 - 节省模式]\n\n")
	if len(fileChanges) > 0 {
		sb.WriteString(fmt.Sprintf("已修改文件 (%d):\n", len(fileChanges)))
		limit := len(fileChanges)
		if limit > 10 {
			limit = 10
		}
		for _, fc := range fileChanges[:limit] {
			sb.WriteString(fmt.Sprintf("  - %s\n", fc))
		}
	}
	if len(decisions) > 0 {
		sb.WriteString(fmt.Sprintf("\n关键决策 (%d):\n", len(decisions)))
		limit := len(decisions)
		if limit > 5 {
			limit = 5
		}
		for _, d := range decisions[:limit] {
			sb.WriteString(fmt.Sprintf("  - %s\n", d))
		}
	}
	if len(errors) > 0 {
		sb.WriteString(fmt.Sprintf("\n遇到的错误 (%d):\n", len(errors)))
		limit := len(errors)
		if limit > 5 {
			limit = 5
		}
		for _, e := range errors[:limit] {
			sb.WriteString(fmt.Sprintf("  - %s\n", e))
		}
	}

	// Keep the last 4 messages for immediate context
	keepCount := 4
	if len(history) < keepCount {
		keepCount = len(history)
	}
	result := []service.Message{
		{Role: "system", Content: sb.String()},
	}
	result = append(result, history[len(history)-keepCount:]...)

	return result
}

// heuristicCompactConversation provides zero-LLM-cost summarization for free models.
// Optimization 28: Extracts key facts and keeps recent messages, no LLM call needed.
// P1-1: Enhanced with information density scoring to preserve critical context.
func (r *AgentRunner) heuristicCompactConversation(conversation []map[string]interface{}) []map[string]interface{} {
	// P1-1: Score messages by importance
	type scoredMsg struct {
		msg      map[string]interface{}
		score    float64
		position int
	}

	scored := make([]scoredMsg, 0, len(conversation))
	for i, msg := range conversation {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		score := 0.0

		// System messages are always important
		if role == "system" {
			score = 100.0
		}

		// User messages are important
		if role == "user" {
			score = 80.0
		}

		// Tool results with file changes are critical
		if role == "tool" && strings.Contains(content, "Successfully wrote") {
			score = 90.0
		}

		// Errors are important for debugging
		if role == "tool" && (strings.HasPrefix(content, "Error:") || strings.HasPrefix(content, "❌")) {
			score = 70.0
		}

		// Recent messages get position bonus
		positionBonus := float64(i) / float64(len(conversation)) * 20.0
		score += positionBonus

		// Penalize very long messages (they waste context window)
		if len(content) > 5000 {
			score -= 10.0
		}

		scored = append(scored, scoredMsg{msg: msg, score: score, position: i})
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Keep top messages by score, but maintain order
	keepCount := len(conversation) / 3 // Keep 1/3 of messages
	if keepCount < 5 {
		keepCount = 5
	}
	if keepCount > len(scored) {
		keepCount = len(scored)
	}

	// Select top scored messages
	keepSet := make(map[int]bool)
	for i := 0; i < keepCount; i++ {
		keepSet[scored[i].position] = true
	}

	// Rebuild conversation in original order
	var fileChanges []string
	var errors []string
	newConv := make([]map[string]interface{}, 0)

	// Always keep system prompt
	for _, msg := range conversation {
		if msg["role"] == "system" {
			newConv = append(newConv, msg)
			break
		}
	}

	// Add summary of what happened
	for _, msg := range conversation {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		if role == "tool" && isFileChangeResult(content) {
			if fc := extractFileChange(content); fc != "" {
				fileChanges = append(fileChanges, fc)
			}
		}
		if role == "tool" && (strings.HasPrefix(content, "Error:") || strings.HasPrefix(content, "❌") || strings.HasPrefix(content, "⚠️")) {
			if len(content) > 150 {
				content = content[:150] + "..."
			}
			errors = append(errors, content)
		}
	}

	// Build summary
	var summary strings.Builder
	summary.WriteString("[上下文压缩 - 智能保留重要信息]\n\n")
	if len(fileChanges) > 0 {
		summary.WriteString(fmt.Sprintf("已修改文件 (%d):\n", len(fileChanges)))
		for _, fc := range fileChanges {
			if len(fileChanges) > 10 && summary.Len() > 500 {
				summary.WriteString(fmt.Sprintf("  ... 还有 %d 个文件\n", len(fileChanges)-10))
				break
			}
			summary.WriteString(fmt.Sprintf("  - %s\n", fc))
		}
	}
	if len(errors) > 0 {
		summary.WriteString(fmt.Sprintf("\n错误 (%d):\n", len(errors)))
		limit := len(errors)
		if limit > 5 {
			limit = 5
		}
		for _, e := range errors[:limit] {
			summary.WriteString(fmt.Sprintf("  - %s\n", e))
		}
	}

	newConv = append(newConv, map[string]interface{}{
		"role":    "system",
		"content": summary.String(),
	})

	// Add kept messages in order
	for i, msg := range conversation {
		if keepSet[i] && msg["role"] != "system" {
			newConv = append(newConv, msg)
		}
	}

	// Always keep last user message
	for i := len(conversation) - 1; i >= 0; i-- {
		if conversation[i]["role"] == "user" {
			if !keepSet[i] {
				newConv = append(newConv, conversation[i])
			}
			break
		}
	}

	return newConv
}

// callLLMSummary sends a one-shot summary request to the LLM and returns the
// streamed content. Shared by compactConversation and compactHistoryViaLLM.
func (r *AgentRunner) callLLMSummary(ctx context.Context, cfg RunConfig, summaryPrompt []map[string]string) (string, error) {
	// P0-Optimization: Use cached resolved config when available.
	endpoint := cfg.resolvedEndpoint
	apiKey := cfg.resolvedAPIKey
	model := cfg.resolvedModel

	// Fallback: if not cached (e.g., called outside Run()), resolve now
	if endpoint == "" {
		endpoint, apiKey, model = r.resolveLLMConfig(cfg.UserID, cfg.ProviderID, "", cfg)
	}

	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint = endpoint + "/chat/completions"
	}

	log.Printf("[Agent] callLLMSummary: endpoint=%s model=%s apiKeyLen=%d providerID=%s", endpoint, model, len(apiKey), cfg.ProviderID)

	body := map[string]interface{}{
		"model":    model,
		"messages": summaryPrompt,
		"stream":   true, // Optimization 28: streaming reduces latency and connection hold time
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		log.Printf("[Agent] callLLMSummary: HTTP error: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	log.Printf("[Agent] callLLMSummary: response status=%d", resp.StatusCode)
	result := streamSSEContent(resp)
	log.Printf("[Agent] callLLMSummary: result length=%d", len(result))
	return result, nil
}

// streamSSEContent parses a streaming chat-completions SSE body and concatenates
// the delta content chunks.
func streamSSEContent(resp *http.Response) string {
	var summary strings.Builder
	forEachSSEChunk(resp.Body, 64*1024, func(data []byte) bool {
		var chunk streamChunk
		if err := json.Unmarshal(data, &chunk); err != nil {
			return true
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			summary.WriteString(chunk.Choices[0].Delta.Content)
		}
		return true
	})
	return summary.String()
}
