package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// AgentPresetSkill manages agent presets and styles
type AgentPresetSkill struct {
	db *sql.DB
}

func NewAgentPresetSkill(db *sql.DB) *AgentPresetSkill {
	return &AgentPresetSkill{db: db}
}

func (s *AgentPresetSkill) Name() string {
	return "agent_preset"
}

func (s *AgentPresetSkill) Description() string {
	return "Manage agent presets (personas). Input: {\"action\": \"list|create|apply|delete\", \"preset_id\": \"...\", \"name\": \"...\", \"style\": \"...\"}"
}

func (s *AgentPresetSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	action, _ := input["action"].(string)
	presetID, _ := input["preset_id"].(string)
	name, _ := input["name"].(string)
	style, _ := input["style"].(string)
	userID, _ := input["user_id"].(string)

	switch action {
	case "list":
		return s.listPresets(userID)
	case "create":
		return s.createPreset(userID, name, style, input)
	case "apply":
		return s.applyPreset(userID, presetID)
	case "delete":
		return s.deletePreset(userID, presetID)
	default:
		return s.listPresets(userID)
	}
}

func (s *AgentPresetSkill) listPresets(userID string) (string, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, name, description, style, system_prompt, temperature, max_tokens, is_default, created_at
		FROM agent_presets
		WHERE user_id = ? OR is_default = 1
		ORDER BY is_default DESC, name`, userID)
	if err != nil {
		return "", fmt.Errorf("list presets: %w", err)
	}
	defer rows.Close()

	var presets []map[string]interface{}
	for rows.Next() {
		var id, uid, n, desc, style, prompt, createdAt string
		var temp float64
		var maxTokens int
		var isDefault bool
		if err := rows.Scan(&id, &uid, &n, &desc, &style, &prompt, &temp, &maxTokens, &isDefault, &createdAt); err != nil {
			continue
		}
		presets = append(presets, map[string]interface{}{
			"id": id, "user_id": uid, "name": n, "description": desc,
			"style": style, "system_prompt": prompt, "temperature": temp,
			"max_tokens": maxTokens, "is_default": isDefault, "created_at": createdAt,
		})
	}

	data, _ := json.MarshalIndent(presets, "", "  ")
	return fmt.Sprintf("Found %d presets:\n%s", len(presets), string(data)), nil
}

func (s *AgentPresetSkill) createPreset(userID, name, style string, input map[string]interface{}) (string, error) {
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	prompt, _ := input["system_prompt"].(string)
	temp := 0.7
	if t, ok := input["temperature"].(float64); ok {
		temp = t
	}
	maxTokens := 4096
	if mt, ok := input["max_tokens"].(float64); ok {
		maxTokens = int(mt)
	}

	// Create table if not exists
	s.db.Exec(`CREATE TABLE IF NOT EXISTS agent_presets (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		style TEXT,
		system_prompt TEXT,
		temperature REAL DEFAULT 0.7,
		max_tokens INTEGER DEFAULT 4096,
		is_default BOOLEAN DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_agent_presets_user ON agent_presets(user_id)`)

	id := fmt.Sprintf("preset_%s_%d", userID, len(name))
	_, err := s.db.Exec(`
		INSERT INTO agent_presets (id, user_id, name, description, style, system_prompt, temperature, max_tokens)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, userID, name, input["description"], style, prompt, temp, maxTokens)
	if err != nil {
		return "", fmt.Errorf("create preset: %w", err)
	}

	return fmt.Sprintf("Created preset '%s' (id: %s)", name, id), nil
}

func (s *AgentPresetSkill) applyPreset(userID, presetID string) (string, error) {
	if presetID == "" {
		return "", fmt.Errorf("preset_id is required")
	}

	var prompt, style string
	var temp float64
	var maxTokens int
	err := s.db.QueryRow(`
		SELECT system_prompt, style, temperature, max_tokens
		FROM agent_presets
		WHERE id = ? AND (user_id = ? OR is_default = 1)`,
		presetID, userID).Scan(&prompt, &style, &temp, &maxTokens)
	if err != nil {
		return "", fmt.Errorf("preset not found: %w", err)
	}

	result := map[string]interface{}{
		"preset_id":     presetID,
		"style":         style,
		"temperature":   temp,
		"max_tokens":    maxTokens,
		"system_prompt": prompt,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return fmt.Sprintf("Applied preset:\n%s", string(data)), nil
}

func (s *AgentPresetSkill) deletePreset(userID, presetID string) (string, error) {
	if presetID == "" {
		return "", fmt.Errorf("preset_id is required")
	}

	result, err := s.db.Exec("DELETE FROM agent_presets WHERE id = ? AND user_id = ?", presetID, userID)
	if err != nil {
		return "", fmt.Errorf("delete preset: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return "", fmt.Errorf("preset not found or not owned by user")
	}

	return fmt.Sprintf("Deleted preset %s", presetID), nil
}

func (s *AgentPresetSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  false,
		Essential: false,
	}
}
