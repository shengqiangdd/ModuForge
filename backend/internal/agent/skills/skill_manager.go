package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moduforge/backend/internal/agent/registry"
)

// SkillManagerSkill manages skill versions, dependencies, and lifecycle
type SkillManagerSkill struct {
	db *sql.DB
}

func init() {
	registry.RegisterFactory("skill_manager", func(deps *registry.Deps) registry.Skill {
		return &SkillManagerSkill{db: deps.DB}
	})
}

func (s *SkillManagerSkill) Name() string {
	return "skill_manager"
}

func (s *SkillManagerSkill) Description() string {
	return `Advanced skill management with versions, dependencies, and lifecycle. Input: {"action": "version|dependencies|history|rollback|activate|deactivate|clone|export|import", "skill_name": "...", "version": "...", "config": {...}}`
}

type SkillVersion struct {
	ID          string                 `json:"id"`
	SkillName   string                 `json:"skill_name"`
	Version     string                 `json:"version"`
	Changelog   string                 `json:"changelog"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Dependencies []string              `json:"dependencies,omitempty"`
	Status      string                 `json:"status"` // active, deprecated, archived
	CreatedBy   string                 `json:"created_by"`
	CreatedAt   string                 `json:"created_at"`
}

type SkillDependency struct {
	SkillName    string `json:"skill_name"`
	DependsOn    string `json:"depends_on"`
	MinVersion   string `json:"min_version"`
	MaxVersion   string `json:"max_version"`
	Optional     bool   `json:"optional"`
}

func (s *SkillManagerSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	action, _ := input["action"].(string)
	skillName, _ := input["skill_name"].(string)

	switch action {
	case "version":
		return s.createVersion(skillName, input)
	case "versions":
		return s.listVersions(skillName)
	case "dependencies":
		return s.manageDependencies(skillName, input)
	case "history":
		return s.getVersionHistory(skillName)
	case "rollback":
		return s.rollbackVersion(skillName, input)
	case "activate":
		return s.activateSkill(skillName)
	case "deactivate":
		return s.deactivateSkill(skillName)
	case "clone":
		return s.cloneSkill(skillName, input)
	case "export":
		return s.exportSkill(skillName)
	case "import":
		return s.importSkill(input)
	case "check_compatibility":
		return s.checkCompatibility(skillName, input)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (s *SkillManagerSkill) ensureTables() error {
	// Skill versions table
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS skill_versions (
			id TEXT PRIMARY KEY,
			skill_name TEXT NOT NULL,
			version TEXT NOT NULL,
			changelog TEXT DEFAULT '',
			config TEXT DEFAULT '{}',
			dependencies TEXT DEFAULT '[]',
			status TEXT DEFAULT 'active',
			created_by TEXT DEFAULT 'system',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(skill_name, version)
		)
	`)
	if err != nil {
		return err
	}

	// Skill dependencies table
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS skill_dependencies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			skill_name TEXT NOT NULL,
			depends_on TEXT NOT NULL,
			min_version TEXT DEFAULT '*',
			max_version TEXT DEFAULT '*',
			optional INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(skill_name, depends_on)
		)
	`)
	if err != nil {
		return err
	}

	// Skill snapshots for rollback
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS skill_snapshots (
			id TEXT PRIMARY KEY,
			skill_name TEXT NOT NULL,
			version TEXT NOT NULL,
			config TEXT NOT NULL,
			system_prompt TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	return nil
}

func (s *SkillManagerSkill) createVersion(skillName string, input map[string]interface{}) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	version, _ := input["version"].(string)
	changelog, _ := input["changelog"].(string)
	config, _ := input["config"].(map[string]interface{})
	deps, _ := input["dependencies"].([]interface{})
	userID, _ := input["user_id"].(string)

	if version == "" {
		return "", fmt.Errorf("version is required")
	}

	// Generate version ID
	versionID := fmt.Sprintf("ver_%s_%s_%d", skillName, version, time.Now().UnixMilli())

	// Convert config and dependencies to JSON
	configJSON := "{}"
	if config != nil {
		b, _ := json.Marshal(config)
		configJSON = string(b)
	}

	depsJSON := "[]"
	if deps != nil {
		b, _ := json.Marshal(deps)
		depsJSON = string(b)
	}

	// Insert version
	_, err := s.db.Exec(`
		INSERT INTO skill_versions (id, skill_name, version, changelog, config, dependencies, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, versionID, skillName, version, changelog, configJSON, depsJSON, userID)
	if err != nil {
		return "", fmt.Errorf("create version: %w", err)
	}

	// Create snapshot for rollback
	_, err = s.db.Exec(`
		INSERT INTO skill_snapshots (id, skill_name, version, config)
		VALUES (?, ?, ?, ?)
	`, versionID, skillName, version, configJSON)
	if err != nil {
		return "", fmt.Errorf("create snapshot: %w", err)
	}

	result := map[string]interface{}{
		"action":     "version",
		"success":    true,
		"version_id": versionID,
		"skill_name": skillName,
		"version":    version,
		"message":    fmt.Sprintf("Version %s created for %s", version, skillName),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SkillManagerSkill) listVersions(skillName string) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	rows, err := s.db.Query(`
		SELECT id, skill_name, version, changelog, status, created_by, created_at
		FROM skill_versions
		WHERE skill_name = ?
		ORDER BY created_at DESC
	`, skillName)
	if err != nil {
		return "", fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	var versions []SkillVersion
	for rows.Next() {
		var v SkillVersion
		if err := rows.Scan(&v.ID, &v.SkillName, &v.Version, &v.Changelog, &v.Status, &v.CreatedBy, &v.CreatedAt); err == nil {
			versions = append(versions, v)
		}
	}

	result := map[string]interface{}{
		"action":    "versions",
		"success":   true,
		"skill_name": skillName,
		"versions":  versions,
		"count":     len(versions),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SkillManagerSkill) manageDependencies(skillName string, input map[string]interface{}) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	action, _ := input["dep_action"].(string)
	dependsOn, _ := input["depends_on"].(string)
	minVersion, _ := input["min_version"].(string)
	maxVersion, _ := input["max_version"].(string)
	optional, _ := input["optional"].(bool)

	switch action {
	case "add":
		if dependsOn == "" {
			return "", fmt.Errorf("depends_on is required")
		}
		if minVersion == "" {
			minVersion = "*"
		}
		if maxVersion == "" {
			maxVersion = "*"
		}

		_, err := s.db.Exec(`
			INSERT INTO skill_dependencies (skill_name, depends_on, min_version, max_version, optional)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(skill_name, depends_on) DO UPDATE SET
				min_version = excluded.min_version,
				max_version = excluded.max_version,
				optional = excluded.optional
		`, skillName, dependsOn, minVersion, maxVersion, optional)
		if err != nil {
			return "", fmt.Errorf("add dependency: %w", err)
		}

		return fmt.Sprintf(`{"action":"dependencies","success":true,"message":"Added dependency: %s"}`, dependsOn), nil

	case "remove":
		if dependsOn == "" {
			return "", fmt.Errorf("depends_on is required")
		}

		_, err := s.db.Exec("DELETE FROM skill_dependencies WHERE skill_name = ? AND depends_on = ?", skillName, dependsOn)
		if err != nil {
			return "", fmt.Errorf("remove dependency: %w", err)
		}

		return fmt.Sprintf(`{"action":"dependencies","success":true,"message":"Removed dependency: %s"}`, dependsOn), nil

	case "list":
		rows, err := s.db.Query(`
			SELECT skill_name, depends_on, min_version, max_version, optional
			FROM skill_dependencies
			WHERE skill_name = ?
		`, skillName)
		if err != nil {
			return "", fmt.Errorf("list dependencies: %w", err)
		}
		defer rows.Close()

		var deps []SkillDependency
		for rows.Next() {
			var d SkillDependency
			var optional int
			if err := rows.Scan(&d.SkillName, &d.DependsOn, &d.MinVersion, &d.MaxVersion, &optional); err == nil {
				d.Optional = optional == 1
				deps = append(deps, d)
			}
		}

		result := map[string]interface{}{
			"action":       "dependencies",
			"success":      true,
			"skill_name":   skillName,
			"dependencies": deps,
			"count":        len(deps),
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return string(b), nil

	default:
		return "", fmt.Errorf("unknown dep_action: %s (use add|remove|list)", action)
	}
}

func (s *SkillManagerSkill) getVersionHistory(skillName string) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	rows, err := s.db.Query(`
		SELECT id, version, changelog, status, created_at
		FROM skill_versions
		WHERE skill_name = ?
		ORDER BY created_at DESC
		LIMIT 20
	`, skillName)
	if err != nil {
		return "", fmt.Errorf("get history: %w", err)
	}
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var id, version, changelog, status, createdAt string
		if err := rows.Scan(&id, &version, &changelog, &status, &createdAt); err == nil {
			history = append(history, map[string]interface{}{
				"id":         id,
				"version":    version,
				"changelog":  changelog,
				"status":     status,
				"created_at": createdAt,
			})
		}
	}

	result := map[string]interface{}{
		"action":     "history",
		"success":    true,
		"skill_name": skillName,
		"history":    history,
		"count":      len(history),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SkillManagerSkill) rollbackVersion(skillName string, input map[string]interface{}) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	targetVersion, _ := input["target_version"].(string)
	if targetVersion == "" {
		return "", fmt.Errorf("target_version is required")
	}

	// Get the snapshot for the target version
	var configJSON string
	err := s.db.QueryRow(`
		SELECT config FROM skill_snapshots
		WHERE skill_name = ? AND version = ?
	`, skillName, targetVersion).Scan(&configJSON)
	if err != nil {
		return "", fmt.Errorf("version not found: %w", err)
	}

	// Create a new version with the old config
	newVersion := fmt.Sprintf("%s-rollback-%d", targetVersion, time.Now().UnixMilli())
	_, err = s.createVersion(skillName, map[string]interface{}{
		"version":   newVersion,
		"changelog": fmt.Sprintf("Rollback to version %s", targetVersion),
		"config":    configJSON,
	})
	if err != nil {
		return "", fmt.Errorf("create rollback version: %w", err)
	}

	result := map[string]interface{}{
		"action":          "rollback",
		"success":         true,
		"skill_name":      skillName,
		"rollback_to":     targetVersion,
		"new_version":     newVersion,
		"message":         fmt.Sprintf("Rolled back to version %s", targetVersion),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SkillManagerSkill) activateSkill(skillName string) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	_, err := s.db.Exec(`
		UPDATE skill_versions SET status = 'active'
		WHERE skill_name = ? AND status != 'archived'
	`, skillName)
	if err != nil {
		return "", fmt.Errorf("activate skill: %w", err)
	}

	result := map[string]interface{}{
		"action":     "activate",
		"success":    true,
		"skill_name": skillName,
		"message":    fmt.Sprintf("Skill %s activated", skillName),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SkillManagerSkill) deactivateSkill(skillName string) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	_, err := s.db.Exec(`
		UPDATE skill_versions SET status = 'deprecated'
		WHERE skill_name = ? AND status = 'active'
	`, skillName)
	if err != nil {
		return "", fmt.Errorf("deactivate skill: %w", err)
	}

	result := map[string]interface{}{
		"action":     "deactivate",
		"success":    true,
		"skill_name": skillName,
		"message":    fmt.Sprintf("Skill %s deactivated", skillName),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SkillManagerSkill) cloneSkill(skillName string, input map[string]interface{}) (string, error) {
	newName, _ := input["new_name"].(string)
	if newName == "" {
		return "", fmt.Errorf("new_name is required")
	}

	// Get the latest version of the source skill
	var configJSON, changelog string
	err := s.db.QueryRow(`
		SELECT config, changelog FROM skill_versions
		WHERE skill_name = ? AND status = 'active'
		ORDER BY created_at DESC LIMIT 1
	`, skillName).Scan(&configJSON, &changelog)
	if err != nil {
		return "", fmt.Errorf("source skill not found: %w", err)
	}

	// Create new skill with cloned config
	_, err = s.createVersion(newName, map[string]interface{}{
		"version":   "1.0.0",
		"changelog": fmt.Sprintf("Cloned from %s", skillName),
		"config":    configJSON,
	})
	if err != nil {
		return "", fmt.Errorf("clone skill: %w", err)
	}

	result := map[string]interface{}{
		"action":     "clone",
		"success":    true,
		"source":     skillName,
		"new_name":   newName,
		"message":    fmt.Sprintf("Cloned %s to %s", skillName, newName),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SkillManagerSkill) exportSkill(skillName string) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	// Get latest version
	var version, configJSON, changelog string
	err := s.db.QueryRow(`
		SELECT version, config, changelog FROM skill_versions
		WHERE skill_name = ? AND status = 'active'
		ORDER BY created_at DESC LIMIT 1
	`, skillName).Scan(&version, &configJSON, &changelog)
	if err != nil {
		return "", fmt.Errorf("skill not found: %w", err)
	}

	// Get dependencies
	var deps []string
	rows, err := s.db.Query(`
		SELECT depends_on FROM skill_dependencies WHERE skill_name = ?
	`, skillName)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var dep string
			if err := rows.Scan(&dep); err == nil {
				deps = append(deps, dep)
			}
		}
	}

	export := map[string]interface{}{
		"skill_name":  skillName,
		"version":     version,
		"changelog":   changelog,
		"config":      configJSON,
		"dependencies": deps,
		"exported_at": time.Now().Format(time.RFC3339),
	}

	b, _ := json.MarshalIndent(export, "", "  ")
	return string(b), nil
}

func (s *SkillManagerSkill) importSkill(input map[string]interface{}) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	skillName, _ := input["skill_name"].(string)
	version, _ := input["version"].(string)
	config, _ := input["config"].(string)
	changelog, _ := input["changelog"].(string)

	if skillName == "" || version == "" {
		return "", fmt.Errorf("skill_name and version are required")
	}

	_, err := s.createVersion(skillName, map[string]interface{}{
		"version":   version,
		"changelog": changelog,
		"config":    config,
	})
	if err != nil {
		return "", fmt.Errorf("import skill: %w", err)
	}

	result := map[string]interface{}{
		"action":     "import",
		"success":    true,
		"skill_name": skillName,
		"version":    version,
		"message":    fmt.Sprintf("Imported %s v%s", skillName, version),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SkillManagerSkill) checkCompatibility(skillName string, input map[string]interface{}) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	// Get dependencies
	rows, err := s.db.Query(`
		SELECT depends_on, min_version, max_version, optional
		FROM skill_dependencies
		WHERE skill_name = ?
	`, skillName)
	if err != nil {
		return "", fmt.Errorf("check compatibility: %w", err)
	}
	defer rows.Close()

	var issues []map[string]interface{}
	var compatible []string

	for rows.Next() {
		var dep SkillDependency
		var optional int
		if err := rows.Scan(&dep.DependsOn, &dep.MinVersion, &dep.MaxVersion, &optional); err == nil {
			dep.Optional = optional == 1

			// Check if dependency exists
			var exists int
			s.db.QueryRow("SELECT COUNT(*) FROM skill_versions WHERE skill_name = ? AND status = 'active'", dep.DependsOn).Scan(&exists)

			if exists == 0 {
				if !dep.Optional {
					issues = append(issues, map[string]interface{}{
						"dependency": dep.DependsOn,
						"issue":      "not found",
						"severity":   "critical",
					})
				}
			} else {
				compatible = append(compatible, dep.DependsOn)
			}
		}
	}

	result := map[string]interface{}{
		"action":      "check_compatibility",
		"success":     len(issues) == 0,
		"skill_name":  skillName,
		"compatible":  compatible,
		"issues":      issues,
		"issue_count": len(issues),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SkillManagerSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}