package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/moduforge/backend/internal/agent/registry"
)

// SkillRegistrySkill manages dynamic skill loading and configuration
type SkillRegistrySkill struct {
	db *sql.DB
}

func init() {
	registry.RegisterFactory("skill_registry", func(deps *registry.Deps) registry.Skill {
		return &SkillRegistrySkill{db: deps.DB}
	})
}

func (s *SkillRegistrySkill) Name() string {
	return "skill_registry"
}

func (s *SkillRegistrySkill) Description() string {
	return `Manage skill registry: list, enable, disable, configure skills. Input: {"action": "list|enable|disable|configure|info|search", "skill_name": "...", "config": {...}}`
}

type SkillConfig struct {
	Name        string                 `json:"name"`
	Enabled     bool                   `json:"enabled"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Dependencies []string              `json:"dependencies,omitempty"`
	LastUpdated string                 `json:"last_updated"`
}

func (s *SkillRegistrySkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	action, _ := input["action"].(string)
	skillName, _ := input["skill_name"].(string)

	switch action {
	case "list":
		return s.listSkills(input)
	case "enable":
		return s.enableSkill(skillName)
	case "disable":
		return s.disableSkill(skillName)
	case "configure":
		return s.configureSkill(skillName, input)
	case "info":
		return s.getSkillInfo(skillName)
	case "search":
		return s.searchSkills(input)
	case "stats":
		return s.getStats()
	default:
		return "", fmt.Errorf("unknown action: %s (use list|enable|disable|configure|info|search|stats)", action)
	}
}

func (s *SkillRegistrySkill) ensureTable() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS skill_configs (
			name TEXT PRIMARY KEY,
			enabled INTEGER DEFAULT 1,
			version TEXT DEFAULT '1.0.0',
			description TEXT DEFAULT '',
			config TEXT DEFAULT '{}',
			dependencies TEXT DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func (s *SkillRegistrySkill) listSkills(input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	showDisabled, _ := input["show_disabled"].(bool)

	query := "SELECT name, enabled, version, description, config, dependencies, updated_at FROM skill_configs"
	if !showDisabled {
		query += " WHERE enabled = 1"
	}
	query += " ORDER BY name"

	rows, err := s.db.Query(query)
	if err != nil {
		return "", fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close()

	var skills []SkillConfig
	for rows.Next() {
		var sc SkillConfig
		var enabled int
		var configJSON, depsJSON string
		if err := rows.Scan(&sc.Name, &enabled, &sc.Version, &sc.Description, &configJSON, &depsJSON, &sc.LastUpdated); err == nil {
			sc.Enabled = enabled == 1
			json.Unmarshal([]byte(configJSON), &sc.Config)
			json.Unmarshal([]byte(depsJSON), &sc.Dependencies)
			skills = append(skills, sc)
		}
	}

	// Add built-in skills that aren't in the config table
	builtinSkills := []string{
		"read_file", "write_file", "edit_file", "grep_search", "glob_search",
		"bash", "build_module", "web_search", "memory_manager", "agent_preset",
		"todo_manager", "task_delegator", "context_manager", "skill_registry",
	}

	existing := make(map[string]bool)
	for _, sk := range skills {
		existing[sk.Name] = true
	}

	for _, name := range builtinSkills {
		if !existing[name] {
			skills = append(skills, SkillConfig{
				Name:    name,
				Enabled: true,
				Version: "builtin",
			})
		}
	}

	result := map[string]interface{}{
		"action":  "list",
		"success": true,
		"skills":  skills,
		"count":   len(skills),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SkillRegistrySkill) enableSkill(skillName string) (string, error) {
	if skillName == "" {
		return "", fmt.Errorf("skill_name is required")
	}

	if err := s.ensureTable(); err != nil {
		return "", err
	}

	// Upsert
	_, err := s.db.Exec(`
		INSERT INTO skill_configs (name, enabled, updated_at)
		VALUES (?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET enabled = 1, updated_at = CURRENT_TIMESTAMP
	`, skillName)
	if err != nil {
		return "", fmt.Errorf("enable skill: %w", err)
	}

	result := map[string]interface{}{
		"action":  "enable",
		"success": true,
		"skill":   skillName,
		"message": fmt.Sprintf("Skill '%s' enabled", skillName),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SkillRegistrySkill) disableSkill(skillName string) (string, error) {
	if skillName == "" {
		return "", fmt.Errorf("skill_name is required")
	}

	if err := s.ensureTable(); err != nil {
		return "", err
	}

	_, err := s.db.Exec(`
		INSERT INTO skill_configs (name, enabled, updated_at)
		VALUES (?, 0, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET enabled = 0, updated_at = CURRENT_TIMESTAMP
	`, skillName)
	if err != nil {
		return "", fmt.Errorf("disable skill: %w", err)
	}

	result := map[string]interface{}{
		"action":  "disable",
		"success": true,
		"skill":   skillName,
		"message": fmt.Sprintf("Skill '%s' disabled", skillName),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SkillRegistrySkill) configureSkill(skillName string, input map[string]interface{}) (string, error) {
	if skillName == "" {
		return "", fmt.Errorf("skill_name is required")
	}

	if err := s.ensureTable(); err != nil {
		return "", err
	}

	config, _ := input["config"].(map[string]interface{})
	description, _ := input["description"].(string)
	version, _ := input["version"].(string)
	deps, _ := input["dependencies"].([]interface{})

	var configJSON, depsJSON string
	if config != nil {
		b, _ := json.Marshal(config)
		configJSON = string(b)
	}
	if deps != nil {
		b, _ := json.Marshal(deps)
		depsJSON = string(b)
	}

	// Upsert with all fields
	query := `
		INSERT INTO skill_configs (name, enabled, version, description, config, dependencies, updated_at)
		VALUES (?, 1, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET
			version = COALESCE(NULLIF(excluded.version, ''), version),
			description = COALESCE(NULLIF(excluded.description, ''), description),
			config = CASE WHEN excluded.config != '{}' THEN excluded.config ELSE config END,
			dependencies = CASE WHEN excluded.dependencies != '[]' THEN excluded.dependencies ELSE dependencies END,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(query, skillName, version, description, configJSON, depsJSON)
	if err != nil {
		return "", fmt.Errorf("configure skill: %w", err)
	}

	result := map[string]interface{}{
		"action":  "configure",
		"success": true,
		"skill":   skillName,
		"message": fmt.Sprintf("Skill '%s' configured", skillName),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SkillRegistrySkill) getSkillInfo(skillName string) (string, error) {
	if skillName == "" {
		return "", fmt.Errorf("skill_name is required")
	}

	if err := s.ensureTable(); err != nil {
		return "", err
	}

	var sc SkillConfig
	var enabled int
	var configJSON, depsJSON string

	err := s.db.QueryRow(`
		SELECT name, enabled, version, description, config, dependencies, updated_at
		FROM skill_configs WHERE name = ?
	`, skillName).Scan(&sc.Name, &enabled, &sc.Version, &sc.Description, &configJSON, &depsJSON, &sc.LastUpdated)
	if err != nil {
		// Not in config table, check if it's a builtin
		builtinSkills := map[string]string{
			"read_file":       "Read file content",
			"write_file":      "Write file content",
			"edit_file":       "Edit file with find-and-replace",
			"grep_search":     "Search file contents",
			"glob_search":     "Find files by pattern",
			"bash":            "Execute shell commands",
			"build_module":    "Build and package module",
			"web_search":      "Search the web",
			"memory_manager":  "Manage memory entries",
			"agent_preset":    "Manage agent presets",
			"todo_manager":    "Manage todo lists",
			"task_delegator":  "Delegate tasks to sub-agents",
			"context_manager": "Manage conversation context",
			"skill_registry":  "Manage skill registry",
		}

		if desc, ok := builtinSkills[skillName]; ok {
			sc = SkillConfig{
				Name:        skillName,
				Enabled:     true,
				Version:     "builtin",
				Description: desc,
			}
		} else {
			return "", fmt.Errorf("skill not found: %s", skillName)
		}
	} else {
		sc.Enabled = enabled == 1
		json.Unmarshal([]byte(configJSON), &sc.Config)
		json.Unmarshal([]byte(depsJSON), &sc.Dependencies)
	}

	result := map[string]interface{}{
		"action": "info",
		"skill":  sc,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SkillRegistrySkill) searchSkills(input map[string]interface{}) (string, error) {
	query, _ := input["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	if err := s.ensureTable(); err != nil {
		return "", err
	}

	// Search in name and description
	rows, err := s.db.Query(`
		SELECT name, enabled, version, description
		FROM skill_configs
		WHERE name LIKE ? OR description LIKE ?
		ORDER BY name
		LIMIT 20
	`, "%"+query+"%", "%"+query+"%")
	if err != nil {
		return "", fmt.Errorf("search skills: %w", err)
	}
	defer rows.Close()

	var skills []map[string]interface{}
	for rows.Next() {
		var name, version, description string
		var enabled int
		if err := rows.Scan(&name, &enabled, &version, &description); err == nil {
			skills = append(skills, map[string]interface{}{
				"name":        name,
				"enabled":     enabled == 1,
				"version":     version,
				"description": description,
			})
		}
	}

	result := map[string]interface{}{
		"action":  "search",
		"success": true,
		"query":   query,
		"results": skills,
		"count":   len(skills),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SkillRegistrySkill) getStats() (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	var total, enabled, disabled int
	s.db.QueryRow("SELECT COUNT(*) FROM skill_configs").Scan(&total)
	s.db.QueryRow("SELECT COUNT(*) FROM skill_configs WHERE enabled = 1").Scan(&enabled)
	s.db.QueryRow("SELECT COUNT(*) FROM skill_configs WHERE enabled = 0").Scan(&disabled)

	// Count builtin skills
	builtinCount := 14 // read_file, write_file, edit_file, grep_search, glob_search, bash, build_module, web_search, memory_manager, agent_preset, todo_manager, task_delegator, context_manager, skill_registry

	result := map[string]interface{}{
		"action":         "stats",
		"success":        true,
		"total_config":   total,
		"enabled":        enabled,
		"disabled":       disabled,
		"builtin_count":  builtinCount,
		"total_available": total + builtinCount,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SkillRegistrySkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}