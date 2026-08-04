package agent

import (
	"github.com/moduforge/backend/internal/service"
)

// appendRoleMessage appends a simple role/content message to the conversation.
// Returns the updated slice since append may reallocate the backing array.
func appendRoleMessage(conversation []map[string]interface{}, role, content string) []map[string]interface{} {
	return append(conversation, map[string]interface{}{
		"role":    role,
		"content": content,
	})
}

// appendToolResult appends a tool result message to the conversation and
// persists it to the session conversation store. Returns the updated slice
// since append may reallocate the backing array.
func (r *AgentRunner) appendToolResult(conversation []map[string]interface{}, sessionID, toolCallID, content string) []map[string]interface{} {
	conversation = append(conversation, map[string]interface{}{
		"role":         "tool",
		"tool_call_id": toolCallID,
		"content":      content,
	})
	if sessionID != "" {
		r.convStore.Append(sessionID, service.Message{
			Role:       "tool",
			Content:    content,
			ToolCallID: toolCallID,
		})
	}
	return conversation
}
