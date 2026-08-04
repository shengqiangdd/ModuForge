package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// AgentPresetSkill manages agent presets and styles (QwenPaw-style)
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
	return "Manage agent presets (personas) and prompt versions. Input: {\"action\": \"list|create|apply|delete|defaults|update_prompt|version_history|rollback\", \"preset_id\": \"...\", \"skill_id\": \"...\", \"prompt\": \"...\"}"
}

type AgentPreset struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Style       string `json:"style"`
	SystemPrompt string `json:"system_prompt"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int    `json:"max_tokens"`
	IsDefault   bool   `json:"is_default"`
	CreatedAt   string `json:"created_at"`
}

// Default presets following QwenPaw style
var defaultPresets = []AgentPreset{
	{
		ID:          "codepilot",
		Name:        "CodePilot",
		Description: "全能编程助手，精通多语言开发和系统架构",
		Style:       "professional",
		SystemPrompt: `You are CodePilot, an expert programming assistant with deep knowledge of:
- Multi-language development (Shell, Rust, Go, Python, C/C++, Java, JavaScript)
- Android system programming (Magisk, KernelSU, APatch modules)
- System architecture and design patterns
- Performance optimization and security

Communication style:
- Precise and concise, avoid unnecessary words
- Provide code examples when relevant
- Explain technical decisions clearly
- Proactively suggest improvements and alternatives
- Use technical analogies to explain complex concepts

Always think step by step. Use available skills when needed for specific tasks.`,
		Temperature: 0.7,
		MaxTokens:   4096,
	},
	{
		ID:          "module_master",
		Name:        "Module Master",
		Description: "Android模块开发专家，专注于Magisk/KernelSU/APatch模块",
		Style:       "expert",
		SystemPrompt: `You are Module Master, a specialist in Android module development.

Expertise:
- Magisk module development and best practices
- KernelSU and APatch module creation
- System property modifications
- SELinux policy management
- Android framework hooking techniques

Development philosophy:
- Security first: Always validate inputs and use proper permissions
- Compatibility: Support multiple Android versions (8-14+)
- Performance: Minimize resource usage and battery impact
- Documentation: Clear install instructions and changelogs

When generating modules:
1. Always include module.prop with proper metadata
2. Use post-fs-data.sh for early system modifications
3. Use service.sh for late boot optimizations
4. Include proper error handling and logging
5. Test on multiple devices/ROMs when possible`,
		Temperature: 0.6,
		MaxTokens:   4096,
	},
	{
		ID:          "debugger",
		Name:        "Debugger",
		Description: "调试专家，专注于问题诊断和修复",
		Style:       "analytical",
		SystemPrompt: `You are Debugger, a systematic problem-solver focused on diagnosis and repair.

Approach:
1. REPRODUCE: Understand the exact issue and conditions
2. ISOLATE: Narrow down the root cause systematically
3. ANALYZE: Examine logs, code, and system state
4. FIX: Implement minimal, targeted solutions
5. VERIFY: Confirm the fix works and doesn't introduce regressions

Tools and techniques:
- Log analysis (logcat, kernel logs, dmesg)
- Strace/ltrace for system call tracing
- GDB for native debugging
- Android Studio profiler for app-level issues
- ADB shell for device-side debugging

Communication style:
- Ask clarifying questions when needed
- Present findings with evidence
- Provide step-by-step debugging guides
- Suggest preventive measures for the future`,
		Temperature: 0.5,
		MaxTokens:   4096,
	},
	{
		ID:          "architect",
		Name:        "Architect",
		Description: "系统架构师，专注于设计和规划",
		Style:       "strategic",
		SystemPrompt: `You are Architect, a strategic thinker focused on system design and planning.

Core competencies:
- System architecture design and documentation
- Technology selection and evaluation
- Scalability and performance planning
- Security architecture and threat modeling
- API design and integration patterns

Design principles:
1. SOLID principles and clean architecture
2. Separation of concerns
3. Loose coupling, high cohesion
4. Defense in depth for security
5. Fail-safe defaults and graceful degradation

When designing:
- Start with requirements and constraints
- Consider trade-offs explicitly
- Document decisions and rationale
- Plan for evolution and change
- Validate against real-world scenarios

Output format:
- High-level architecture diagrams (text-based)
- Component interaction flows
- Technology recommendations with rationale
- Risk assessment and mitigation strategies`,
		Temperature: 0.6,
		MaxTokens:   4096,
	},
	{
		ID:          "teacher",
		Name:        "Teacher",
		Description: "教学模式，详细解释概念和原理",
		Style:       "educational",
		SystemPrompt: `You are Teacher, a patient educator who explains concepts clearly.

Teaching approach:
1. Start with the big picture, then dive into details
2. Use analogies and real-world examples
3. Provide hands-on exercises when possible
4. Encourage questions and exploration
5. Adapt to the learner's level

Communication style:
- Clear, step-by-step explanations
- Use diagrams and visual aids (text-based)
- Provide practical examples
- Highlight common pitfalls and how to avoid them
- Suggest further reading and resources

When explaining:
- Break complex topics into manageable chunks
- Build on existing knowledge
- Use multiple explanations for different learning styles
- Provide "try it yourself" suggestions
- Summarize key takeaways`,
		Temperature: 0.7,
		MaxTokens:   4096,
	},
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
	case "defaults":
		return s.getDefaultPresets()
	case "update_prompt":
		return s.updatePrompt(input)
	case "version_history":
		return s.versionHistory(input)
	case "rollback":
		return s.rollbackVersion(input)
	case "list_versions":
		return s.listVersions(input)
	default:
		return "", fmt.Errorf("unknown action: %s (use list|create|apply|delete|defaults|update_prompt|version_history|rollback|list_versions)", action)
	}
}

func (s *AgentPresetSkill) listPresets(userID string) (string, error) {
	// Get user's custom presets
	rows, err := s.db.Query(`
		SELECT id, name, description, style, system_prompt, temperature, max_tokens, is_default, created_at
		FROM agent_presets
		WHERE user_id = ?
		ORDER BY is_default DESC, created_at DESC
	`, userID)
	if err != nil {
		// Table might not exist, return defaults
		return s.getDefaultPresets()
	}
	defer rows.Close()

	var presets []map[string]interface{}
	for rows.Next() {
		var p AgentPreset
		var isDefault int
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Style, &p.SystemPrompt, &p.Temperature, &p.MaxTokens, &isDefault, &p.CreatedAt); err == nil {
			p.IsDefault = isDefault == 1
			presets = append(presets, map[string]interface{}{
				"id":          p.ID,
				"name":        p.Name,
				"description": p.Description,
				"style":       p.Style,
				"is_default":  p.IsDefault,
				"created_at":  p.CreatedAt,
			})
		}
	}

	// Add defaults if not already present
	defaultIDs := make(map[string]bool)
	for _, p := range presets {
		defaultIDs[p["id"].(string)] = true
	}
	for _, dp := range defaultPresets {
		if !defaultIDs[dp.ID] {
			presets = append(presets, map[string]interface{}{
				"id":          dp.ID,
				"name":        dp.Name,
				"description": dp.Description,
				"style":       dp.Style,
				"is_default":  true,
				"built_in":    true,
			})
		}
	}

	result := map[string]interface{}{
		"action":  "list",
		"success": true,
		"presets": presets,
		"count":   len(presets),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *AgentPresetSkill) createPreset(userID string, name string, style string, input map[string]interface{}) (string, error) {
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	description, _ := input["description"].(string)
	systemPrompt, _ := input["system_prompt"].(string)
	temperature := 0.7
	if t, ok := input["temperature"].(float64); ok {
		temperature = t
	}
	maxTokens := 4096
	if mt, ok := input["max_tokens"].(float64); ok {
		maxTokens = int(mt)
	}

	// Generate ID
	presetID := fmt.Sprintf("custom_%s_%d", userID, len(name))

	// Ensure table exists
	s.db.Exec(`
		CREATE TABLE IF NOT EXISTS agent_presets (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			style TEXT DEFAULT 'professional',
			system_prompt TEXT DEFAULT '',
			temperature REAL DEFAULT 0.7,
			max_tokens INTEGER DEFAULT 4096,
			is_default INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)

	// Insert preset
	_, err := s.db.Exec(`
		INSERT INTO agent_presets (id, user_id, name, description, style, system_prompt, temperature, max_tokens)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, presetID, userID, name, description, style, systemPrompt, temperature, maxTokens)
	if err != nil {
		return "", fmt.Errorf("create preset: %w", err)
	}

	result := map[string]interface{}{
		"action":    "create",
		"success":   true,
		"preset_id": presetID,
		"name":      name,
		"style":     style,
		"message":   fmt.Sprintf("预设 '%s' 创建成功", name),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *AgentPresetSkill) applyPreset(userID string, presetID string) (string, error) {
	if presetID == "" {
		return "", fmt.Errorf("preset_id is required")
	}

	// Check if it's a built-in preset
	for _, p := range defaultPresets {
		if p.ID == presetID {
			result := map[string]interface{}{
				"action":       "apply",
				"success":      true,
				"preset_id":    presetID,
				"name":         p.Name,
				"style":        p.Style,
				"system_prompt": p.SystemPrompt,
				"temperature":  p.Temperature,
				"max_tokens":   p.MaxTokens,
				"message":      fmt.Sprintf("已应用预设: %s", p.Name),
			}
			b, _ := json.MarshalIndent(result, "", "  ")
			return string(b), nil
		}
	}

	// Check user's custom presets
	var p AgentPreset
	var isDefault int
	err := s.db.QueryRow(`
		SELECT id, name, style, system_prompt, temperature, max_tokens, is_default
		FROM agent_presets
		WHERE id = ? AND user_id = ?
	`, presetID, userID).Scan(&p.ID, &p.Name, &p.Style, &p.SystemPrompt, &p.Temperature, &p.MaxTokens, &isDefault)
	if err != nil {
		return "", fmt.Errorf("preset not found: %w", err)
	}

	result := map[string]interface{}{
		"action":       "apply",
		"success":      true,
		"preset_id":    p.ID,
		"name":         p.Name,
		"style":        p.Style,
		"system_prompt": p.SystemPrompt,
		"temperature":  p.Temperature,
		"max_tokens":   p.MaxTokens,
		"message":      fmt.Sprintf("已应用预设: %s", p.Name),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *AgentPresetSkill) deletePreset(userID string, presetID string) (string, error) {
	if presetID == "" {
		return "", fmt.Errorf("preset_id is required")
	}

	// Check if it's a built-in preset
	for _, p := range defaultPresets {
		if p.ID == presetID {
			return "", fmt.Errorf("cannot delete built-in preset")
		}
	}

	_, err := s.db.Exec("DELETE FROM agent_presets WHERE id = ? AND user_id = ?", presetID, userID)
	if err != nil {
		return "", fmt.Errorf("delete preset: %w", err)
	}

	result := map[string]interface{}{
		"action":    "delete",
		"success":   true,
		"preset_id": presetID,
		"message":   "预设已删除",
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *AgentPresetSkill) updatePrompt(input map[string]interface{}) (string, error) {
	skillID, _ := input["skill_id"].(string)
	prompt, _ := input["prompt"].(string)
	changeReason, _ := input["change_reason"].(string)

	if skillID == "" || prompt == "" {
		return "", fmt.Errorf("skill_id and prompt are required")
	}

	var currentPrompt string
	err := s.db.QueryRow("SELECT prompt FROM custom_skills WHERE id = ?", skillID).Scan(&currentPrompt)
	if err != nil {
		return "", fmt.Errorf("skill not found: %w", err)
	}

	var maxVersion int
	s.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM prompt_versions WHERE skill_id = ?", skillID).Scan(&maxVersion)
	newVersion := maxVersion + 1

	if changeReason == "" {
		changeReason = "Manual prompt update"
	}

	_, err = s.db.Exec(
		"INSERT INTO prompt_versions (skill_id, prompt, version, change_reason) VALUES (?, ?, ?, ?)",
		skillID, currentPrompt, newVersion, changeReason,
	)
	if err != nil {
		return "", fmt.Errorf("record version: %w", err)
	}

	_, err = s.db.Exec("UPDATE custom_skills SET prompt = ? WHERE id = ?", prompt, skillID)
	if err != nil {
		return "", fmt.Errorf("update prompt: %w", err)
	}

	result := map[string]interface{}{
		"action":       "update_prompt",
		"success":      true,
		"skill_id":     skillID,
		"version":      newVersion,
		"old_version":  newVersion - 1,
		"change_reason": changeReason,
		"message":      fmt.Sprintf("Prompt 已更新至版本 %d", newVersion),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *AgentPresetSkill) versionHistory(input map[string]interface{}) (string, error) {
	skillID, _ := input["skill_id"].(string)
	if skillID == "" {
		return "", fmt.Errorf("skill_id is required")
	}

	rows, err := s.db.Query(`
		SELECT id, version, prompt, change_reason, created_at
		FROM prompt_versions
		WHERE skill_id = ?
		ORDER BY version DESC
		LIMIT 20
	`, skillID)
	if err != nil {
		return "", fmt.Errorf("query versions: %w", err)
	}
	defer rows.Close()

	type VersionRecord struct {
		ID           int64  `json:"id"`
		Version      int    `json:"version"`
		Prompt       string `json:"prompt"`
		ChangeReason string `json:"change_reason"`
		CreatedAt    string `json:"created_at"`
	}

	var versions []VersionRecord
	for rows.Next() {
		var v VersionRecord
		if err := rows.Scan(&v.ID, &v.Version, &v.Prompt, &v.ChangeReason, &v.CreatedAt); err == nil {
			versions = append(versions, v)
		}
	}

	result := map[string]interface{}{
		"action":    "version_history",
		"success":   true,
		"skill_id":  skillID,
		"versions":  versions,
		"count":     len(versions),
		"message":   fmt.Sprintf("找到 %d 个历史版本", len(versions)),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *AgentPresetSkill) rollbackVersion(input map[string]interface{}) (string, error) {
	skillID, _ := input["skill_id"].(string)
	version := 0
	if v, ok := input["version"].(float64); ok {
		version = int(v)
	}

	if skillID == "" || version <= 0 {
		return "", fmt.Errorf("skill_id and version (> 0) are required")
	}

	var targetPrompt string
	err := s.db.QueryRow(
		"SELECT prompt FROM prompt_versions WHERE skill_id = ? AND version = ?",
		skillID, version,
	).Scan(&targetPrompt)
	if err != nil {
		return "", fmt.Errorf("version not found: %w", err)
	}

	var currentPrompt string
	s.db.QueryRow("SELECT prompt FROM custom_skills WHERE id = ?", skillID).Scan(&currentPrompt)

	var maxVersion int
	s.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM prompt_versions WHERE skill_id = ?", skillID).Scan(&maxVersion)
	newVersion := maxVersion + 1

	_, err = s.db.Exec(
		"INSERT INTO prompt_versions (skill_id, prompt, version, change_reason) VALUES (?, ?, ?, ?)",
		skillID, currentPrompt, newVersion,
		fmt.Sprintf("Rollback to version %d", version),
	)
	if err != nil {
		return "", fmt.Errorf("record rollback version: %w", err)
	}

	_, err = s.db.Exec("UPDATE custom_skills SET prompt = ? WHERE id = ?", targetPrompt, skillID)
	if err != nil {
		return "", fmt.Errorf("rollback prompt: %w", err)
	}

	result := map[string]interface{}{
		"action":           "rollback",
		"success":          true,
		"skill_id":         skillID,
		"rollback_to":      version,
		"new_version":      newVersion,
		"restored_prompt_length": len(targetPrompt),
		"message":          fmt.Sprintf("已回滚至版本 %d，当前版本 %d", version, newVersion),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *AgentPresetSkill) listVersions(input map[string]interface{}) (string, error) {
	skillID, _ := input["skill_id"].(string)
	if skillID == "" {
		return "", fmt.Errorf("skill_id is required")
	}

	var totalVersions int
	s.db.QueryRow("SELECT COUNT(*) FROM prompt_versions WHERE skill_id = ?", skillID).Scan(&totalVersions)

	var latestVersion int
	s.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM prompt_versions WHERE skill_id = ?", skillID).Scan(&latestVersion)

	var currentPromptLength int
	s.db.QueryRow("SELECT COALESCE(LENGTH(prompt), 0) FROM custom_skills WHERE id = ?", skillID).Scan(&currentPromptLength)

	rows, err := s.db.Query(`
		SELECT version, change_reason, created_at
		FROM prompt_versions
		WHERE skill_id = ?
		ORDER BY version DESC
	`, skillID)
	if err != nil {
		return "", fmt.Errorf("query versions: %w", err)
	}
	defer rows.Close()

	var versionList []map[string]interface{}
	for rows.Next() {
		var version int
		var changeReason, createdAt string
		if err := rows.Scan(&version, &changeReason, &createdAt); err == nil {
			versionList = append(versionList, map[string]interface{}{
				"version":       version,
				"change_reason": changeReason,
				"created_at":    createdAt,
			})
		}
	}

	result := map[string]interface{}{
		"action":              "list_versions",
		"success":             true,
		"skill_id":            skillID,
		"total_versions":      totalVersions,
		"latest_version":      latestVersion,
		"current_prompt_length": currentPromptLength,
		"versions":            versionList,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *AgentPresetSkill) getDefaultPresets() (string, error) {
	var presets []map[string]interface{}
	for _, p := range defaultPresets {
		presets = append(presets, map[string]interface{}{
			"id":          p.ID,
			"name":        p.Name,
			"description": p.Description,
			"style":       p.Style,
			"built_in":    true,
		})
	}

	result := map[string]interface{}{
		"action":  "defaults",
		"success": true,
		"presets": presets,
		"count":   len(presets),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *AgentPresetSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  false,
		Essential: false,
		NeedsDB:   true,
		NeedsLLM:  true,
	}
}
