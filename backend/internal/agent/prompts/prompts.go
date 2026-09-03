package prompts

import (
	"embed"
	"fmt"
	"strings"
	"sync"
)

//go:embed *.md
var fs embed.FS

// Cache for loaded prompts (avoid repeated file reads)
var promptCache sync.Map

// Prompt holds the assembled system prompt
type Prompt struct {
	Base    string // base.md content
	Mode    string // plan.md, act.md, or free.md content
	Tools   string // tools.md content
	Errors  string // errors.md content
	Full    string // fully assembled prompt
}

// PromptInfo holds metadata about a prompt file
type PromptInfo struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Size    int    `json:"size"`
	IsMD    bool   `json:"is_md"`
}

// Load retrieves and assembles prompts for the given mode
func Load(mode string) (*Prompt, error) {
	// Check cache first
	if cached, ok := promptCache.Load(mode); ok {
		return cached.(*Prompt), nil
	}

	p := &Prompt{}

	// Load base prompt (with override support)
	base, err := loadFileWithOverride("base.md")
	if err != nil {
		return nil, fmt.Errorf("failed to load base.md: %w", err)
	}
	p.Base = base

	// Load mode-specific prompt (with override support)
	// Support both Agent modes (plan/act/free) and AI Handler modes (generate/chat/repair/gather/agent)
	modeFile := "free.md" // default to free mode
	switch strings.ToLower(mode) {
	// Agent modes
	case "plan":
		modeFile = "plan.md"
	case "act":
		modeFile = "act.md"
	case "free":
		modeFile = "free.md"
	// AI Handler modes (previously database-only)
	case "generate":
		modeFile = "generate.md"
	case "chat":
		modeFile = "chat.md"
	case "repair":
		modeFile = "repair.md"
	case "gather":
		modeFile = "gather.md"
	case "agent":
		modeFile = "agent.md"
	}

	modeContent, err := loadFileWithOverride(modeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", modeFile, err)
	}
	p.Mode = modeContent

	// Load tools reference (with override support)
	toolsContent, err := loadFileWithOverride("tools.md")
	if err != nil {
		// Non-fatal, continue without tools reference
		p.Tools = ""
	} else {
		p.Tools = toolsContent
	}

	// Load error reference (with override support)
	errorsContent, err := loadFileWithOverride("errors.md")
	if err != nil {
		// Non-fatal, continue without error reference
		p.Errors = ""
	} else {
		p.Errors = errorsContent
	}

	// Load module spec for module-generation modes
	// Injected into generate/agent/repair modes so LLM always has the spec in context
	moduleSpec := ""
	specModes := map[string]bool{"generate": true, "agent": true, "repair": true}
	if specModes[strings.ToLower(mode)] {
		specContent, err := loadFileWithOverride("module_spec.md")
		if err == nil && specContent != "" {
			moduleSpec = specContent
		}
	}

	// Load Android APP guide for act mode (injected after module spec)
	androidAppGuide := ""
	if strings.ToLower(mode) == "act" {
		guideContent, err := loadFileWithOverride("android_app.md")
		if err == nil && guideContent != "" {
			androidAppGuide = guideContent
		}
	}

	// Assemble full prompt
	var sb strings.Builder
	sb.WriteString(p.Base)
	sb.WriteString("\n\n")
	sb.WriteString(p.Mode)

	if moduleSpec != "" {
		sb.WriteString("\n\n")
		sb.WriteString(moduleSpec)
	}

	if androidAppGuide != "" {
		sb.WriteString("\n\n")
		sb.WriteString(androidAppGuide)
	}

	if p.Tools != "" {
		sb.WriteString("\n\n")
		sb.WriteString(p.Tools)
	}

	if p.Errors != "" {
		sb.WriteString("\n\n")
		sb.WriteString(p.Errors)
	}

	p.Full = sb.String()

	// Cache the result
	promptCache.Store(mode, p)

	return p, nil
}

// Reload clears the cache and reloads all prompts
func Reload() {
	promptCache = sync.Map{}
}

// loadFile reads a file from the embedded filesystem
func loadFile(name string) (string, error) {
	data, err := fs.ReadFile(name)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ListAllPrompts returns all available MD prompt files
func ListAllPrompts() []PromptInfo {
	var prompts []PromptInfo

	entries, err := fs.ReadDir(".")
	if err != nil {
		return prompts
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			content, _ := loadFile(entry.Name())
			prompts = append(prompts, PromptInfo{
				Name:    entry.Name(),
				Content: content,
				Size:    int(info.Size()),
				IsMD:    true,
			})
		}
	}

	return prompts
}

// GetPrompt returns the content of a specific MD file
func GetPrompt(name string) (string, error) {
	if !strings.HasSuffix(name, ".md") {
		name = name + ".md"
	}
	return loadFileWithOverride(name)
}

// UpdatePrompt saves content to a MD file (updates the embedded FS cache)
// Note: This only updates the in-memory cache, not the actual file on disk
// For persistent changes, the file needs to be updated and the binary reloaded
func UpdatePrompt(name, content string) error {
	if !strings.HasSuffix(name, ".md") {
		name = name + ".md"
	}

	// Update the embedded FS cache
	// Note: embed.FS is read-only, so we need to use a different approach
	// For now, we'll store the override in a separate map
	overrideCache.Store(name, content)

	// Clear the prompt cache so it reloads with the override
	Reload()

	return nil
}

// ResetPrompt removes the override for a specific prompt
func ResetPrompt(name string) error {
	if !strings.HasSuffix(name, ".md") {
		name = name + ".md"
	}

	overrideCache.Delete(name)
	Reload()
	return nil
}

// overrideCache stores user overrides for prompts
var overrideCache sync.Map

// loadFileWithOverride loads a file, checking for overrides first
func loadFileWithOverride(name string) (string, error) {
	// Check for override first
	if override, ok := overrideCache.Load(name); ok {
		return override.(string), nil
	}

	// Fall back to embedded file
	return loadFile(name)
}
