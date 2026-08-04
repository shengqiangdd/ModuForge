package agent

import (
	"github.com/moduforge/backend/internal/agent/registry"
)

// Re-export registry types so existing code doesn't break.
type Skill = registry.Skill
type SkillRegistry = registry.SkillRegistry
type SkillMeta = registry.SkillMeta
type Deps = registry.Deps

// NewSkillRegistry creates a registry from shared dependencies.
// All skills are auto-registered via init() in their respective packages.
func NewSkillRegistry(deps *Deps) *SkillRegistry {
	return registry.NewSkillRegistry(deps)
}
