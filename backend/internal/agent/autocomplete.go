package agent

import (
	"fmt"
	"sync"
)

// AutocompleteEngine provides real-time code completions.
type AutocompleteEngine struct {
	mu       sync.Mutex
	cache    map[string][]Completion
	maxCache int
}

// Completion represents a code completion suggestion.
type Completion struct {
	Text        string  `json:"text"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
	Snippet     string  `json:"snippet"`
}

// NewAutocompleteEngine creates a new autocomplete engine.
func NewAutocompleteEngine() *AutocompleteEngine {
	return &AutocompleteEngine{
		cache:    make(map[string][]Completion),
		maxCache: 1000,
	}
}

// GetCompletions returns completions for a given prefix.
func (ae *AutocompleteEngine) GetCompletions(prefix string, context string) []Completion {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	// Check cache
	if cached, ok := ae.cache[prefix]; ok {
		return cached
	}

	// Generate completions based on context
	completions := ae.generateCompletions(prefix, context)

	// Cache results
	if len(ae.cache) < ae.maxCache {
		ae.cache[prefix] = completions
	}

	return completions
}

// generateCompletions generates completions based on context.
func (ae *AutocompleteEngine) generateCompletions(prefix string, context string) []Completion {
	var completions []Completion

	// Simple keyword-based completion
	keywords := []string{
		"func", "if", "else", "for", "range", "switch", "case", "return",
		"import", "package", "type", "struct", "interface", "map", "slice",
	}

	for _, kw := range keywords {
		if len(prefix) > 0 && len(kw) >= len(prefix) && kw[:len(prefix)] == prefix {
			completions = append(completions, Completion{
				Text:        kw,
				Description: fmt.Sprintf("Keyword: %s", kw),
				Score:       1.0,
				Snippet:     kw + " ",
			})
		}
	}

	return completions
}

// ClearCache clears the completion cache.
func (ae *AutocompleteEngine) ClearCache() {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.cache = make(map[string][]Completion)
}
