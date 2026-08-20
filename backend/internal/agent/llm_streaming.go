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

// streamChunk is a single SSE "data:" payload from a chat-completions stream.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Role             string                `json:"role"`
			Content          string                `json:"content"`
			ReasoningContent string                `json:"reasoning_content"`
			ToolCalls        []streamToolCallDelta `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	// Usage is present on the final chunk of streaming responses (OpenAI-style).
	// It is sent in a chunk with empty choices, which the old code skipped.
	Usage *TokenUsage `json:"usage"`
}

// TokenUsage holds per-call token accounting from the LLM API.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// forEachSSEChunk iterates over the SSE "data:" payloads in a streaming
// chat-completions body, invoking fn for each non-"[DONE]" payload. It stops on
// "[DONE]" or when fn returns false, and returns the scanner error (nil on a
// clean end). Shared by parseStreamingResponse and streamSSEContent.
func forEachSSEChunk(r io.Reader, bufSize int, fn func(data []byte) bool) error {
	scanner := bufio.NewScanner(r)
	// Use a large buffer to avoid "bufio.Scanner: token too long" on long tool
	// call JSONs. Default 64KB is too small; 256KB handles most LLM responses.
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

// parseStreamingResponse reads an SSE stream and extracts content + tool calls.
// Uses a 256KB scanner buffer to handle large tool call JSON without truncation.
func (r *AgentRunner) parseStreamingResponse(ctx context.Context, resp *http.Response, w SSEWriter) (*LLMResponse, error) {
	var fullContent strings.Builder
	var toolCalls []LLMToolCall
	toolCallMap := make(map[int]*LLMToolCall, 4) // Optimization 36: pre-allocate for typical 1-3 tool calls
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
			// Log failed parse at debug level (some LLMs send non-standard chunks)
			debugLog("stream parse failed (len=%d): %v", len(data), err)
			return true
		}
		// Capture usage from the final chunk (empty choices + usage field).
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
		// If scanner failed and we have no data at all, propagate the error
		// so callers know the LLM stream was interrupted (network/proxy issue).
		if fullContent.Len() == 0 && len(toolCallMap) == 0 {
			return nil, fmt.Errorf("LLM stream interrupted: %w", err)
		}
	}

	toolCalls = mergeToolCalls(toolCallMap)

	// Validate and repair tool calls from weak models
	toolCalls = repairToolCalls(toolCalls)

	// P0: If no native tool calls, try to extract text-format tool calls from content
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

// streamToolCallDelta is the tool_calls portion of a streaming SSE delta chunk.
type streamToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// accumulateStreamToolCall merges a streaming tool-call delta into the map
// keyed by call index, appending argument fragments across chunks.
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

// mergeToolCalls flattens the per-index tool call map into an ordered slice.
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
