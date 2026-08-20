package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════════════
// SSE Stream Processing
// ═══════════════════════════════════════════════════════════════════

func forEachSSEChunk(r io.Reader, bufSize int, fn func(data []byte) bool) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, bufSize), bufSize)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		if !fn([]byte(data)) {
			break
		}
	}
	return scanner.Err()
}

func (r *AgentRunner) parseStreamingResponse(ctx context.Context, resp *http.Response, w SSEWriter) (*LLMResponse, error) {
	var fullContent strings.Builder
	var toolCalls []LLMToolCall
	toolCallMap := make(map[int]*LLMToolCall, 4)
	var finishReason string
	var usage *TokenUsage
	keepAliveDone := make(chan struct{})
	startKeepalive(ctx, w, keepAliveDone, 10*time.Second)
	err := forEachSSEChunk(resp.Body, 256*1024, func(data []byte) bool {
		if w.IsDisconnected() {
			return false
		}
		var parsed streamChunk
		if err := json.Unmarshal(data, &parsed); err != nil {
			debugLog("stream parse failed (len=%d): %v", len(data), err)
			return true
		}
		if parsed.Usage != nil && parsed.Usage.TotalTokens > 0 {
			usage = parsed.Usage
		}
		if len(parsed.Choices) == 0 {
			return true
		}
		delta := parsed.Choices[0].Delta
		if parsed.Choices[0].FinishReason != "" {
			finishReason = parsed.Choices[0].FinishReason
		}
		if delta.ReasoningContent != "" {
			cleaned := sanitizeReasoning(delta.ReasoningContent)
			if cleaned != "" {
				w.WriteSSE(map[string]interface{}{"type": "reasoning", "content": cleaned})
			}
		}
		if delta.Content != "" {
			fullContent.WriteString(delta.Content)
			w.WriteSSE(map[string]interface{}{"type": "stream_delta", "content": delta.Content})
		}
		for _, tc := range delta.ToolCalls {
			accumulateStreamToolCall(tc, toolCallMap)
		}
		return true
	})
	close(keepAliveDone)
	if err != nil {
		log.Printf("[Agent] scanner error: %v", err)
		if fullContent.Len() == 0 && len(toolCallMap) == 0 {
			return nil, fmt.Errorf("LLM stream interrupted: %w", err)
		}
	}
	toolCalls = mergeToolCalls(toolCallMap)
	toolCalls = repairToolCalls(toolCalls)
	if len(toolCalls) == 0 && fullContent.Len() > 0 {
		toolCalls = extractTextToolCalls(fullContent.String())
		if len(toolCalls) > 0 {
			log.Printf("[Agent] extracted %d text-format tool calls from content", len(toolCalls))
		}
	}
	return &LLMResponse{
		Role:         "assistant",
		Content:      fullContent.String(),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		TokenUsage:   usage,
	}, nil
}

// ═══════════════════════════════════════════════════════════════════
// Stream Tool Call Accumulation
// ═══════════════════════════════════════════════════════════════════

func accumulateStreamToolCall(tc streamToolCallDelta, toolCallMap map[int]*LLMToolCall) {
	idx := tc.Index
	if idx < 0 {
		idx = 0
	}
	existing, ok := toolCallMap[idx]
	if !ok {
		toolCallMap[idx] = &LLMToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
		return
	}
	if tc.ID != "" {
		existing.ID = tc.ID
	}
	if tc.Function.Name != "" {
		existing.Function.Name = tc.Function.Name
	}
	existing.Function.Arguments += tc.Function.Arguments
}

func mergeToolCalls(toolCallMap map[int]*LLMToolCall) []LLMToolCall {
	if len(toolCallMap) == 0 {
		return nil
	}
	toolCalls := make([]LLMToolCall, 0, len(toolCallMap))
	for i := 0; i < len(toolCallMap); i++ {
		if tc, ok := toolCallMap[i]; ok {
			toolCalls = append(toolCalls, *tc)
		}
	}
	return toolCalls
}

// ═══════════════════════════════════════════════════════════════════
// Text Tool Call Extraction
// ═══════════════════════════════════════════════════════════════════

func extractTextToolCalls(content string) []LLMToolCall {
	if content == "" {
		return nil
	}
	var toolCalls []LLMToolCall
	codeBlockRegex := []string{
		"```tool_call\n",
		"```tool_call\r\n",
		"```json\n",
		"```json\r\n",
		"```\n{\"name\":",
		"```\n{\"function\":",
	}
	for _, prefix := range codeBlockRegex {
		idx := strings.Index(content, prefix)
		if idx < 0 {
			continue
		}
		start := idx + len(prefix)
		endIdx := strings.Index(content[start:], "```")
		if endIdx < 0 {
			continue
		}
		jsonStr := strings.TrimSpace(content[start : start+endIdx])
		var tc LLMToolCall
		if err := json.Unmarshal([]byte(jsonStr), &tc); err == nil && tc.Function.Name != "" {
			toolCalls = append(toolCalls, tc)
			continue
		}
		var named struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &named); err == nil && named.Name != "" {
			argsBytes, _ := json.Marshal(named.Arguments)
			toolCalls = append(toolCalls, LLMToolCall{
				ID:   fmt.Sprintf("text_%d", len(toolCalls)),
				Type: "function",
				Function: ToolCallFunction{
					Name:      named.Name,
					Arguments: string(argsBytes),
				},
			})
			continue
		}
		var arr []struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal([]byte("["+jsonStr+"]"), &arr); err == nil {
			for _, item := range arr {
				if item.Name != "" {
					argsBytes, _ := json.Marshal(item.Arguments)
					toolCalls = append(toolCalls, LLMToolCall{
						ID:   fmt.Sprintf("text_%d", len(toolCalls)),
						Type: "function",
						Function: ToolCallFunction{
							Name:      item.Name,
							Arguments: string(argsBytes),
						},
					})
				}
			}
		}
	}
	if len(toolCalls) == 0 {
		toolCalls = extractInlineToolCalls(content)
	}
	return toolCalls
}

func extractInlineToolCalls(content string) []LLMToolCall {
	var toolCalls []LLMToolCall
	toolNames := []string{"write_file", "edit_file", "read_file", "bash", "build_module", "test_module", "grep_search", "glob_search"}
	for _, name := range toolNames {
		searchStr := name + "("
		idx := 0
		for {
			pos := strings.Index(content[idx:], searchStr)
			if pos < 0 {
				break
			}
			start := idx + pos + len(searchStr)
			depth := 1
			end := start
			for end < len(content) && depth > 0 {
				if content[end] == '(' {
					depth++
				} else if content[end] == ')' {
					depth--
				}
				end++
			}
			if depth != 0 {
				idx = start
				continue
			}
			argsStr := content[start : end-1]
			args := make(map[string]interface{})
			parts := smartSplitArgs(argsStr)
			for _, part := range parts {
				eqIdx := strings.Index(part, "=")
				if eqIdx < 0 {
					continue
				}
				key := strings.TrimSpace(part[:eqIdx])
				val := strings.TrimSpace(part[eqIdx+1:])
				if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
					(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
					val = val[1 : len(val)-1]
				}
				args[key] = val
			}
			if len(args) > 0 {
				argsBytes, _ := json.Marshal(args)
				toolCalls = append(toolCalls, LLMToolCall{
					ID:   fmt.Sprintf("inline_%d", len(toolCalls)),
					Type: "function",
					Function: ToolCallFunction{
						Name:      name,
						Arguments: string(argsBytes),
					},
				})
			}
			idx = end
		}
	}
	return toolCalls
}

func smartSplitArgs(s string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inQuote {
			current.WriteByte(ch)
			if ch == quoteChar && (i == 0 || s[i-1] != '\\') {
				inQuote = false
			}
		} else {
			if ch == '"' || ch == '\'' {
				inQuote = true
				quoteChar = ch
				current.WriteByte(ch)
			} else if ch == ',' {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteByte(ch)
			}
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// ═══════════════════════════════════════════════════════════════════
// Tool Call Repair
// ═══════════════════════════════════════════════════════════════════

func repairToolCalls(toolCalls []LLMToolCall) []LLMToolCall {
	if len(toolCalls) == 0 {
		return toolCalls
	}
	repaired := make([]LLMToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		if tc.Function.Name == "" {
			log.Printf("[Agent] skipping tool call with empty name")
			continue
		}
		args := tc.Function.Arguments
		if args == "" {
			repaired = append(repaired, tc)
			continue
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(args), &parsed); err != nil {
			fixed, repairErr := repairJSONArguments(args)
			if repairErr != nil {
				log.Printf("[Agent] cannot repair tool call JSON for %s: %v (original: %s)", tc.Function.Name, repairErr, args[:min(len(args), 100)])
				continue
			}
			tc.Function.Arguments = fixed
		}
		repaired = append(repaired, tc)
	}
	return repaired
}

func repairJSONArguments(args string) (string, error) {
	fixed := args
	fixed = strings.ReplaceAll(fixed, "\n", "\\n")
	fixed = strings.ReplaceAll(fixed, "\r", "\\r")
	fixed = strings.ReplaceAll(fixed, "\t", "\\t")
	start := strings.Index(fixed, "{")
	end := strings.LastIndex(fixed, "}")
	if start >= 0 && end > start {
		fixed = fixed[start : end+1]
	}
	fixed = strings.ReplaceAll(fixed, ",}", "}")
	fixed = strings.ReplaceAll(fixed, ",]", "]")
	fixed = strings.ReplaceAll(fixed, `"path" "`, `"path": "`)
	fixed = strings.ReplaceAll(fixed, `"content" "`, `"content": "`)
	fixed = strings.ReplaceAll(fixed, `"query" "`, `"query": "`)
	fixed = strings.ReplaceAll(fixed, `"thought" "`, `"thought": "`)
	fixed = strings.ReplaceAll(fixed, `"action" "`, `"action": "`)
	fixed = strings.ReplaceAll(fixed, `"key" "`, `"key": "`)
	fixed = strings.ReplaceAll(fixed, `"value" "`, `"value": "`)
	fixed = strings.ReplaceAll(fixed, `"description" "`, `"description": "`)
	fixed = strings.ReplaceAll(fixed, "'path'", `"path"`)
	fixed = strings.ReplaceAll(fixed, "'content'", `"content"`)
	fixed = strings.ReplaceAll(fixed, "'query'", `"query"`)
	fixed = strings.ReplaceAll(fixed, "'thought'", `"thought"`)
	fixed = strings.ReplaceAll(fixed, "'action'", `"action"`)
	fixed = strings.ReplaceAll(fixed, "'key'", `"key"`)
	fixed = strings.ReplaceAll(fixed, "'value'", `"value"`)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(fixed), &parsed); err != nil {
		idx := strings.Index(fixed, "{")
		if idx > 0 {
			candidate := fixed[idx:]
			depth := 0
			endIdx := -1
			for i, ch := range candidate {
				if ch == '{' {
					depth++
				} else if ch == '}' {
					depth--
					if depth == 0 {
						endIdx = i + 1
						break
					}
				}
			}
			if endIdx > 0 {
				candidate = candidate[:endIdx]
				if err3 := json.Unmarshal([]byte(candidate), &parsed); err3 == nil {
					return candidate, nil
				}
			}
		}
		return "", err
	}
	return fixed, nil
}
