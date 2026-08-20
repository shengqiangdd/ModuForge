package service

import "sync"

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// MessageToMap converts a Message to a map suitable for LLM API
func (m Message) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"role":    m.Role,
		"content": m.Content,
	}
	if len(m.ToolCalls) > 0 {
		result["tool_calls"] = m.ToolCalls
	}
	if m.ToolCallID != "" {
		result["tool_call_id"] = m.ToolCallID
	}
	return result
}

type Conversation struct {
	Messages []Message
}

type ConversationStore struct {
	mu          sync.RWMutex
	sessions    map[string]*Conversation
	order       []string
	maxSessions int
	maxMessages int
}
