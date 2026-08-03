package agent

import (
	"github.com/moduforge/backend/internal/service"
)

// fixToolCallsInHistory removes tool_calls from assistant messages in history
// when their corresponding tool response messages have been removed (e.g., during compression).
// It also removes orphaned tool messages (tool responses without preceding tool_calls).
// This prevents LLM API errors about missing/mismatched tool messages.
func fixToolCallsInHistory(history []service.Message) []service.Message {
	// Build set of tool_call_ids that have responses
	toolResponseIDs := make(map[string]bool)
	for _, msg := range history {
		if msg.Role == "tool" && msg.ToolCallID != "" {
			toolResponseIDs[msg.ToolCallID] = true
		}
	}

	// Collect all tool_call_ids from assistant messages
	assistantToolCallIDs := make(map[string]bool)
	for _, msg := range history {
		if msg.Role == "assistant" {
			for _, tc := range msg.ToolCalls {
				if tc.ID != "" {
					assistantToolCallIDs[tc.ID] = true
				}
			}
		}
	}

	// Remove tool_calls from assistant messages whose tool responses are missing
	// AND remove orphaned tool messages
	result := make([]service.Message, 0, len(history))
	for _, msg := range history {
		// Skip orphaned tool messages
		if msg.Role == "tool" && msg.ToolCallID != "" {
			if !assistantToolCallIDs[msg.ToolCallID] {
				continue
			}
		}
		// Fix assistant messages with missing tool responses
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			var validCalls []service.ToolCall
			for _, tc := range msg.ToolCalls {
				if tc.ID == "" || toolResponseIDs[tc.ID] {
					validCalls = append(validCalls, tc)
				}
			}
			if len(validCalls) != len(msg.ToolCalls) {
				msg.ToolCalls = validCalls
			}
		}
		result = append(result, msg)
	}
	return result
}
