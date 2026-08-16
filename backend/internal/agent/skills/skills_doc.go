package skills

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/moduforge/backend/internal/agent/registry"
)

// SkillsDocSkill lists every registered skill with its description and
// parameters — the ModuForge equivalent of a SKILLS.md reference doc.
// It is registered dynamically (after MCP tools are loaded) so the output
// always reflects the live skill catalog, including MCP-backed tools.
type SkillsDocSkill struct {
	reg *registry.SkillRegistry
}

// NewSkillsDocSkill creates the catalog skill bound to the live registry.
func NewSkillsDocSkill(reg *registry.SkillRegistry) *SkillsDocSkill {
	return &SkillsDocSkill{reg: reg}
}

func (s *SkillsDocSkill) Name() string {
	return "skills_doc"
}

func (s *SkillsDocSkill) Description() string {
	return "List all available skills with descriptions and parameters. Call this when you need to discover what tools exist or check a skill's expected input. Input: {\"filter\": \"optional substring filter\", \"format\": \"compact|detailed\" (default detailed)}"
}

func (s *SkillsDocSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	filter, _ := input["filter"].(string)
	format, _ := input["format"].(string)

	skills := s.reg.List()
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name() < skills[j].Name() })

	var sb strings.Builder
	sb.WriteString("# ModuForge Skill Catalog\n\n")
	sb.WriteString(fmt.Sprintf("Total: %d skills\n\n", len(skills)))

	for _, skill := range skills {
		name := skill.Name()
		if filter != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
			continue
		}

		desc := skill.Description()
		sb.WriteString(fmt.Sprintf("## %s\n", name))
		sb.WriteString(fmt.Sprintf("%s\n", desc))

		if format == "detailed" {
			if pp, ok := skill.(registry.ParameterProvider); ok {
				if params := pp.Parameters(); params != nil {
					sb.WriteString(fmt.Sprintf("Parameters: %v\n", params))
				}
			}
			// Mark MCP tools for clarity
			if strings.HasPrefix(name, "mcp__") {
				sb.WriteString("(remote MCP tool)\n")
			}
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}
