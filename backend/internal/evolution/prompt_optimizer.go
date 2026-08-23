package evolution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PromptSuggestion represents a suggested improvement to a prompt template.
type PromptSuggestion struct {
	ID              string    `json:"id"`
	OriginalPrompt  string    `json:"original_prompt"`
	SuggestedChange string    `json:"suggested_change"`
	Reason          string    `json:"reason"`
	Confidence      float64   `json:"confidence"`
	CreatedAt       time.Time `json:"created_at"`
	Applied         bool      `json:"applied"`
	ExperienceIDs   []string  `json:"experience_ids,omitempty"`
}

// PromptOptimizer analyzes experiences and generates prompt improvements.
type PromptOptimizer struct {
	mu          sync.RWMutex
	dir         string
	suggestions []PromptSuggestion
}

// NewPromptOptimizer creates a new optimizer.
func NewPromptOptimizer(dataDir string) *PromptOptimizer {
	return &PromptOptimizer{dir: dataDir}
}

// AnalyzeAndSuggest examines experiences and generates prompt optimization suggestions.
func (po *PromptOptimizer) AnalyzeAndSuggest(experiences []Experience) []PromptSuggestion {
	if len(experiences) == 0 {
		return nil
	}

	// Group experiences by error pattern similarity
	patternGroups := groupByPattern(experiences)

	var suggestions []PromptSuggestion

	for pattern, exps := range patternGroups {
		if len(exps) < 2 {
			continue // Need at least 2 occurrences for a pattern
		}

		// Calculate success rate for this pattern
		successCount := 0
		for _, exp := range exps {
			if exp.SuccessRate > 0.5 {
				successCount++
			}
		}
		successRate := float64(successCount) / float64(len(exps))

		// Find the most successful fix solution
		bestFix := findBestFix(exps)

		// Generate suggestion
		confidence := calculateConfidence(len(exps), successRate)

		suggestion := PromptSuggestion{
			ID:              fmt.Sprintf("sug_%d", time.Now().UnixNano()),
			OriginalPrompt:  pattern,
			SuggestedChange: bestFix,
			Reason:          fmt.Sprintf("Pattern '%s' appeared %d times with %.0f%% success rate", pattern, len(exps), successRate*100),
			Confidence:      confidence,
			CreatedAt:       time.Now(),
			ExperienceIDs:   extractIDs(exps),
		}

		suggestions = append(suggestions, suggestion)
	}

	// Sort by confidence descending
	for i := 0; i < len(suggestions); i++ {
		for j := i + 1; j < len(suggestions); j++ {
			if suggestions[j].Confidence > suggestions[i].Confidence {
				suggestions[i], suggestions[j] = suggestions[j], suggestions[i]
			}
		}
	}

	return suggestions
}

// ApplySuggestion marks a suggestion as applied.
func (po *PromptOptimizer) ApplySuggestion(suggestionID string) error {
	po.mu.Lock()
	defer po.mu.Unlock()

	if err := po.load(); err != nil {
		return fmt.Errorf("load: %w", err)
	}

	for i := range po.suggestions {
		if po.suggestions[i].ID == suggestionID {
			po.suggestions[i].Applied = true
			return po.save()
		}
	}

	return fmt.Errorf("suggestion not found: %s", suggestionID)
}

// GetPendingSuggestions returns unapplied suggestions.
func (po *PromptOptimizer) GetPendingSuggestions() []PromptSuggestion {
	po.mu.RLock()
	defer po.mu.RUnlock()

	po.load()

	var pending []PromptSuggestion
	for _, s := range po.suggestions {
		if !s.Applied {
			pending = append(pending, s)
		}
	}
	return pending
}

// SaveSuggestion persists a suggestion.
func (po *PromptOptimizer) SaveSuggestion(sug PromptSuggestion) error {
	po.mu.Lock()
	defer po.mu.Unlock()

	if sug.ID == "" {
		sug.ID = fmt.Sprintf("sug_%d", time.Now().UnixNano())
	}
	if sug.CreatedAt.IsZero() {
		sug.CreatedAt = time.Now()
	}

	if err := po.load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load: %w", err)
	}

	po.suggestions = append(po.suggestions, sug)
	return po.save()
}

// GetAll returns all suggestions.
func (po *PromptOptimizer) GetAll() []PromptSuggestion {
	po.mu.RLock()
	defer po.mu.RUnlock()
	po.load()
	out := make([]PromptSuggestion, len(po.suggestions))
	copy(out, po.suggestions)
	return out
}

// ═══════════════════════════════════════════════════════
// Internal helpers
// ═══════════════════════════════════════════════════════

func (po *PromptOptimizer) load() error {
	path := filepath.Join(po.dir, "suggestions.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &po.suggestions)
}

func (po *PromptOptimizer) save() error {
	if err := os.MkdirAll(po.dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(po.suggestions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(po.dir, "suggestions.json"), data, 0644)
}

func groupByPattern(experiences []Experience) map[string][]Experience {
	groups := make(map[string][]Experience)

	for _, exp := range experiences {
		// Normalize pattern: lowercase, first 50 chars
		pattern := exp.ErrorPattern
		if len(pattern) > 50 {
			pattern = pattern[:50]
		}
		groups[pattern] = append(groups[pattern], exp)
	}

	return groups
}

func findBestFix(exps []Experience) string {
	bestRate := -1.0
	bestFix := ""

	for _, exp := range exps {
		if exp.SuccessRate > bestRate {
			bestRate = exp.SuccessRate
			bestFix = exp.FixSolution
		}
	}

	return bestFix
}

func calculateConfidence(count int, successRate float64) float64 {
	// More occurrences + higher success = higher confidence
	countScore := float64(count) / 10.0
	if countScore > 1.0 {
		countScore = 1.0
	}

	confidence := (countScore*0.4 + successRate*0.6) * 100
	if confidence > 95 {
		confidence = 95
	}
	return confidence
}

func extractIDs(exps []Experience) []string {
	var ids []string
	for _, exp := range exps {
		ids = append(ids, exp.ID)
	}
	return ids
}
