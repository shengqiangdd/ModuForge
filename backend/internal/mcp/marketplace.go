package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ParamDef describes a parameter for a tool.
type ParamDef struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
}

// ToolDefinition describes an MCP tool available in the marketplace.
type ToolDefinition struct {
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	Parameters         []ParamDef `json:"parameters,omitempty"`
	PermissionRequired string     `json:"permission_required,omitempty"`
	Version            string     `json:"version"`
	Author             string     `json:"author,omitempty"`
	RegisteredAt       time.Time  `json:"registered_at"`
}

// Marketplace manages tool definitions.
type Marketplace struct {
	mu    sync.RWMutex
	dir   string
	tools map[string]ToolDefinition
}

// NewMarketplace creates a marketplace backed by JSON in dataDir.
func NewMarketplace(dataDir string) *Marketplace {
	return &Marketplace{
		dir:   dataDir,
		tools: make(map[string]ToolDefinition),
	}
}

// RegisterTool adds a tool definition to the marketplace.
func (m *Marketplace) RegisterTool(tool ToolDefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load: %w", err)
	}

	if tool.Name == "" {
		return fmt.Errorf("tool name is required")
	}

	if tool.RegisteredAt.IsZero() {
		tool.RegisteredAt = time.Now()
	}

	m.tools[tool.Name] = tool
	return m.save()
}

// UnregisterTool removes a tool from the marketplace.
func (m *Marketplace) UnregisterTool(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.load(); err != nil {
		return fmt.Errorf("load: %w", err)
	}

	if _, ok := m.tools[name]; !ok {
		return fmt.Errorf("tool not found: %s", name)
	}

	delete(m.tools, name)
	return m.save()
}

// GetTool returns a tool definition by name.
func (m *Marketplace) GetTool(name string) (ToolDefinition, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.load()
	t, ok := m.tools[name]
	return t, ok
}

// ListTools returns all registered tools.
func (m *Marketplace) ListTools() []ToolDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.load()
	var tools []ToolDefinition
	for _, t := range m.tools {
		tools = append(tools, t)
	}
	return tools
}

// SearchTools finds tools matching a query (keyword overlap).
func (m *Marketplace) SearchTools(query string) []ToolDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.load()

	keywords := strings.Fields(strings.ToLower(query))
	if len(keywords) == 0 {
		return m.ListTools()
	}

	type scored struct {
		tool  ToolDefinition
		score float64
	}

	var results []scored
	for _, t := range m.tools {
		text := strings.ToLower(t.Name + " " + t.Description)
		score := 0.0
		for _, kw := range keywords {
			if strings.Contains(text, kw) {
				score += 1.0
			}
		}
		if score > 0 {
			results = append(results, scored{t, score})
		}
	}

	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	var out []ToolDefinition
	for _, r := range results {
		out = append(out, r.tool)
	}
	return out
}

// Count returns the number of registered tools.
func (m *Marketplace) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.load()
	return len(m.tools)
}

// ═══════════════════════════════════════════════════════
// Internal helpers
// ═══════════════════════════════════════════════════════

func (m *Marketplace) load() error {
	path := filepath.Join(m.dir, "mcp_tools.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var tools []ToolDefinition
	if err := json.Unmarshal(data, &tools); err != nil {
		return err
	}
	m.tools = make(map[string]ToolDefinition, len(tools))
	for _, t := range tools {
		m.tools[t.Name] = t
	}
	return nil
}

func (m *Marketplace) save() error {
	if err := os.MkdirAll(m.dir, 0755); err != nil {
		return err
	}
	var tools []ToolDefinition
	for _, t := range m.tools {
		tools = append(tools, t)
	}
	data, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.dir, "mcp_tools.json"), data, 0644)
}
