package skills

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moduforge/backend/internal/service"
)

var memoryStore *service.MemoryStore

func SetMemoryStore(ms *service.MemoryStore) {
	memoryStore = ms
}

type MemoryManagerSkill struct{}

func NewMemoryManagerSkill() *MemoryManagerSkill {
	return &MemoryManagerSkill{}
}

func (s *MemoryManagerSkill) Name() string {
	return "memory_manager"
}

func (s *MemoryManagerSkill) Description() string {
	return "Manage persistent memory for user preferences. Input: {\"user_id\": \"...\", \"action\": \"get|list\", \"type\": \"user_preference|project_context\", \"key\": \"...\"}. Read-only by default."
}

type memoryManagerResult struct {
	Action  string `json:"action"`
	Type    string `json:"type"`
	Key     string `json:"key,omitempty"`
	Value   string `json:"value,omitempty"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (s *MemoryManagerSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	if memoryStore == nil {
		return "", fmt.Errorf("memory store not initialized")
	}

	userID, _ := input["user_id"].(string)
	if userID == "" {
		return "", fmt.Errorf("user_id is required")
	}

	action, _ := input["action"].(string)
	if action == "" {
		return "", fmt.Errorf("action is required (save|get|list|delete)")
	}

	memType, _ := input["type"].(string)
	key, _ := input["key"].(string)
	value, _ := input["value"].(string)

	switch action {
	case "save":
		if memType == "" || key == "" {
			return "", fmt.Errorf("type and key are required for save")
		}
		if err := memoryStore.SaveMemory(userID, memType, key, value); err != nil {
			return "", fmt.Errorf("save memory: %w", err)
		}
		result := memoryManagerResult{
			Action:  "save",
			Type:    memType,
			Key:     key,
			Value:   value,
			Success: true,
			Message: fmt.Sprintf("记忆已保存: [%s] %s = %s", memType, key, value),
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return string(b), nil

	case "get":
		if memType == "" || key == "" {
			return "", fmt.Errorf("type and key are required for get")
		}
		val, err := memoryStore.GetMemory(userID, memType, key)
		if err != nil {
			return "", fmt.Errorf("get memory: %w", err)
		}
		result := memoryManagerResult{
			Action:  "get",
			Type:    memType,
			Key:     key,
			Value:   val,
			Success: val != "",
			Message: fmt.Sprintf("记忆值: %s", val),
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return string(b), nil

	case "list":
		if memType == "" {
			return "", fmt.Errorf("type is required for list")
		}
		entries, err := memoryStore.ListMemory(userID, memType)
		if err != nil {
			return "", fmt.Errorf("list memory: %w", err)
		}
		type listResult struct {
			Action  string               `json:"action"`
			Type    string               `json:"type"`
			Entries []service.MemoryEntry `json:"entries"`
			Count   int                  `json:"count"`
		}
		result := listResult{
			Action:  "list",
			Type:    memType,
			Entries: entries,
			Count:   len(entries),
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return string(b), nil

	case "delete":
		if memType == "" || key == "" {
			return "", fmt.Errorf("type and key are required for delete")
		}
		if err := memoryStore.DeleteMemory(userID, memType, key); err != nil {
			return "", fmt.Errorf("delete memory: %w", err)
		}
		result := memoryManagerResult{
			Action:  "delete",
			Type:    memType,
			Key:     key,
			Success: true,
			Message: fmt.Sprintf("记忆已删除: [%s] %s", memType, key),
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return string(b), nil

	default:
		return "", fmt.Errorf("unsupported action: %s (use save|get|list|delete)", action)
	}
}

func (s *MemoryManagerSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  true,
		Essential: true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
