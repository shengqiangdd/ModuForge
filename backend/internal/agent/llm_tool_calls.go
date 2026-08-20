package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// extractTextToolCalls attempts to parse tool calls from LLM text content.
// Some models (especially free ones) output tool calls as text instead of
// using the native function calling format. This function detects and extracts
// them so they can be executed.
func extractTextToolCalls(content string) []LLMToolCall {
	if content == "" {
		return nil
	}

	var toolCalls []LLMToolCall

	// Pattern 1: ```tool_call\n{...}\n``` or ```json\n{...}\n```
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
		// Find closing ```
		endIdx := strings.Index(content[start:], "```")
		if endIdx < 0 {
			continue
		}
		jsonStr := strings.TrimSpace(content[start : start+endIdx])

		// Try to parse as a tool call object
		var tc LLMToolCall
		if err := json.Unmarshal([]byte(jsonStr), &tc); err == nil && tc.Function.Name != "" {
			toolCalls = append(toolCalls, tc)
			continue
		}

		// Try as {"name": "...", "arguments": {...}} format
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

		// Try as array of tool calls
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

	// Pattern 2: Inline text like "I'll call write_file(path=\"foo.rs\", content=\"...\")"
	if len(toolCalls) == 0 {
		toolCalls = extractInlineToolCalls(content)
	}

	return toolCalls
}

// extractInlineToolCalls tries to find tool calls in natural language text.
// Detects patterns like: write_file(path="...", content="...")
func extractInlineToolCalls(content string) []LLMToolCall {
	var toolCalls []LLMToolCall

	// Known tool names to look for
	toolNames := []string{"write_file", "edit_file", "read_file", "bash", "build_module", "test_module", "grep_search", "glob_search"}

	for _, name := range toolNames {
		// Look for tool_name(...) pattern
		searchStr := name + "("
		idx := 0
		for {
			pos := strings.Index(content[idx:], searchStr)
			if pos < 0 {
				break
			}
			start := idx + pos + len(searchStr)
			// Find matching closing paren
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
			// Parse key=value pairs
			args := make(map[string]interface{})
			// Simple parsing: key="value" or key=value
			parts := smartSplitArgs(argsStr)
			for _, part := range parts {
				eqIdx := strings.Index(part, "=")
				if eqIdx < 0 {
					continue
				}
				key := strings.TrimSpace(part[:eqIdx])
				val := strings.TrimSpace(part[eqIdx+1:])
				// Remove quotes
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

// smartSplitArgs splits a string like 'a="hello world", b=42' respecting quotes
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

// repairToolCalls attempts to fix malformed tool call JSON from weak/free models.
// Common issues: truncated JSON, missing quotes, unescaped characters.
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
			// Some models send empty arguments for tools that don't need them
			repaired = append(repaired, tc)
			continue
		}

		// Try to parse as JSON
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(args), &parsed); err != nil {
			fixed, repairErr := repairJSONArguments(args)
			if repairErr != nil {
				log.Printf("[Agent] cannot repair tool call JSON for %s: %v (original: %s)", tc.Function.Name, repairErr, args[:min(len(args), 100)])
				// Skip this tool call - it's unrecoverable
				continue
			}
			tc.Function.Arguments = fixed
		}

		repaired = append(repaired, tc)
	}

	return repaired
}

// repairJSONArguments applies a chain of progressively more aggressive fixes to
// malformed tool-call argument JSON. Returns the fixed JSON or an error if all
// repair attempts fail.
func repairJSONArguments(args string) (string, error) {
	fixed := args

	// Fix 1: Unescaped newlines in strings
	fixed = strings.ReplaceAll(fixed, "\n", "\\n")
	fixed = strings.ReplaceAll(fixed, "\r", "\\r")
	fixed = strings.ReplaceAll(fixed, "\t", "\\t")

	// Fix 2: Try to find JSON object boundaries
	start := strings.Index(fixed, "{")
	end := strings.LastIndex(fixed, "}")
	if start >= 0 && end > start {
		fixed = fixed[start : end+1]
	}

	// Fix 3: Try to fix trailing commas
	fixed = strings.ReplaceAll(fixed, ",}", "}")
	fixed = strings.ReplaceAll(fixed, ",]", "]")

	// Fix 4: Fix unescaped quotes inside strings (common in code content)
	// This is tricky - we need to be careful not to break valid JSON
	// Only apply if the JSON still fails after other fixes

	// Fix 5: Fix missing colons between key and value
	fixed = strings.ReplaceAll(fixed, `"path" "`, `"path": "`)
	fixed = strings.ReplaceAll(fixed, `"content" "`, `"content": "`)
	fixed = strings.ReplaceAll(fixed, `"query" "`, `"query": "`)
	fixed = strings.ReplaceAll(fixed, `"thought" "`, `"thought": "`)
	fixed = strings.ReplaceAll(fixed, `"action" "`, `"action": "`)
	fixed = strings.ReplaceAll(fixed, `"key" "`, `"key": "`)
	fixed = strings.ReplaceAll(fixed, `"value" "`, `"value": "`)
	fixed = strings.ReplaceAll(fixed, `"description" "`, `"description": "`)

	// Fix 6: Fix single quotes instead of double quotes for keys
	fixed = strings.ReplaceAll(fixed, "'path'", `"path"`)
	fixed = strings.ReplaceAll(fixed, "'content'", `"content"`)
	fixed = strings.ReplaceAll(fixed, "'query'", `"query"`)
	fixed = strings.ReplaceAll(fixed, "'thought'", `"thought"`)
	fixed = strings.ReplaceAll(fixed, "'action'", `"action"`)
	fixed = strings.ReplaceAll(fixed, "'key'", `"key"`)
	fixed = strings.ReplaceAll(fixed, "'value'", `"value"`)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(fixed), &parsed); err != nil {
		// Fix 7: Try to extract the first valid JSON object from the string
		// Some models prefix with explanatory text
		idx := strings.Index(fixed, "{")
		if idx > 0 {
			candidate := fixed[idx:]
			// Find matching closing brace
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
