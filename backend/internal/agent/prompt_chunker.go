package agent

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func NewPromptChunker(promptsDir string) *PromptChunker {
	return &PromptChunker{
		promptsDir: promptsDir,
		cache:      make(map[string]string),
	}
}

// ModeModules defines which prompt modules each mode needs.
// Instead of loading all 11 files (~15KB), load only 3-5 needed for the mode.
var ModeModules = map[string][]string{
	"code": {"base", "agent", "tools", "act", "errors"},
	"plan": {"base", "agent", "tools", "plan", "errors"},
	"free": {"base", "chat"},
	"chat": {"base", "chat"},
	"edit": {"base", "agent", "tools", "act", "errors"},
}

// BuildModePrompt constructs a prompt with only the modules needed for the mode.
func (pc *PromptChunker) BuildModePrompt(mode string) string {
	pc.mu.RLock()
	if cached, ok := pc.cache[mode]; ok {
		pc.mu.RUnlock()
		return cached
	}
	pc.mu.RUnlock()

	modules, ok := ModeModules[mode]
	if !ok {
		modules = ModeModules["code"]
	}

	var sb strings.Builder
	for _, mod := range modules {
		path := filepath.Join(pc.promptsDir, mod+".md")
		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[PromptChunker] Warning: failed to load %s.md: %v", mod, err)
			continue
		}
		sb.WriteString(string(content))
		sb.WriteString("\n\n")
	}

	result := sb.String()

	pc.mu.Lock()
	pc.cache[mode] = result
	pc.mu.Unlock()

	return result
}

// InvalidateCache clears the cache.
func (pc *PromptChunker) InvalidateCache() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.cache = make(map[string]string)
}

