package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/moduforge/backend/internal/agent/registry"
)

// ToolSkill adapts a remote MCP tool to the ModuForge registry.Skill
// interface. Once registered, the Agent can call it like any native skill:
//
//	{
//	  "name": "mcp__github__get_issue",
//	  "arguments": {"owner": "shengqiangdd", "repo": "ModuForge", "number": 12}
//	}
type ToolSkill struct {
	client  *Client
	tool    Tool
	meta    registry.SkillMeta
}

// NewToolSkill wraps a remote MCP tool into a registry.Skill.
func NewToolSkill(client *Client, tool Tool) *ToolSkill {
	// MCP tools are side-effect-unknown until called: keep them out of Plan
	// mode (ReadOnly=false) and out of the free-model essential set, but mark
	// them Core so they are always visible to paying models. MCP tools are
	// never "essential" — they are optional external integrations.
	return &ToolSkill{
		client: client,
		tool:   tool,
		meta: registry.SkillMeta{
			ReadOnly:  false,
			Essential: false,
			Core:      true,
			NeedsDB:   false,
			NeedsLLM:  false,
			MinTier:   0,
		},
	}
}

// Name returns the namespaced tool name: mcp__<server>__<tool>.
// The "__" separator avoids collisions with native skill names.
func (s *ToolSkill) Name() string {
	return "mcp__" + sanitizeName(s.client.Name) + "__" + sanitizeName(s.tool.Name)
}

// Description returns a human-readable tool description for the LLM.
func (s *ToolSkill) Description() string {
	desc := strings.TrimSpace(s.tool.Description)
	if desc == "" {
		desc = "MCP tool " + s.tool.Name
	}
	return fmt.Sprintf("[MCP %s] %s (remote tool from %s MCP server)", s.tool.Name, desc, s.client.Name)
}

// Metadata implements registry.MetadataProvider.
func (s *ToolSkill) Metadata() registry.SkillMeta { return s.meta }

// Writes reports whether this tool mutates state (used for permission
// enforcement — write tools require explicit user approval before the Agent
// may call them automatically).
func (s *ToolSkill) Writes() bool { return IsWriteTool(s.tool) }

// Parameters implements registry.ParameterProvider — returns the MCP tool's
// native JSON Schema so the LLM sees real argument fields instead of a
// generic {"input": string} wrapper.
func (s *ToolSkill) Parameters() map[string]interface{} {
	if s.tool.InputSchema != nil {
		// MCP inputSchema is already JSON Schema (type: object, properties,
		// required). OpenAI-style tool definitions accept this shape directly.
		return s.tool.InputSchema
	}
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

// Execute forwards the call to the remote MCP server.
func (s *ToolSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	// MCP arguments are the raw JSON-schema object; pass through untouched.
	args := make(map[string]interface{}, len(input))
	for k, v := range input {
		args[k] = v
	}
	return s.client.CallTool(ctx, s.tool.Name, args)
}

// sanitizeName makes a name safe for the LLM tool namespace.
func sanitizeName(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			sb.WriteRune(r)
		default:
			sb.WriteByte('_')
		}
	}
	out := sb.String()
	if out == "" {
		out = "tool"
	}
	return out
}
