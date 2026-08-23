package evolution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Experience represents a learned error-fix pattern.
type Experience struct {
	ID            string    `json:"id"`
	ErrorPattern  string    `json:"error_pattern"`
	FixSolution   string    `json:"fix_solution"`
	SuccessRate   float64   `json:"success_rate"`
	Timestamp     time.Time `json:"timestamp"`
	Source        string    `json:"source,omitempty"`
	HumanVerified bool      `json:"human_verified"`
	ApplyCount    int       `json:"apply_count"`
	SuccessCount  int       `json:"success_count"`
}

// ExperienceStore manages persistent experience storage.
type ExperienceStore struct {
	mu        sync.RWMutex
	dir       string
	exps      []Experience
	byPattern map[string][]int // pattern keywords -> indices
}

// NewExperienceStore creates a store backed by JSON in dataDir.
func NewExperienceStore(dataDir string) *ExperienceStore {
	return &ExperienceStore{
		dir:       dataDir,
		byPattern: make(map[string][]int),
	}
}

// LoadFromReport parses a repair report and extracts experience entries.
func (s *ExperienceStore) LoadFromReport(reportPath string) ([]Experience, error) {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("read report: %w", err)
	}

	var report struct {
		Errors []struct {
			Pattern string `json:"pattern"`
			Fix     string `json:"fix"`
			Success bool   `json:"success"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("parse report: %w", err)
	}

	var exps []Experience
	for _, e := range report.Errors {
		exp := Experience{
			ID:           fmt.Sprintf("exp_%d", time.Now().UnixNano()),
			ErrorPattern: e.Pattern,
			FixSolution:  e.Fix,
			SuccessRate:  boolToFloat(e.Success),
			Timestamp:    time.Now(),
			Source:       reportPath,
		}
		exps = append(exps, exp)
	}

	return exps, nil
}

// SaveExperience persists an experience entry.
func (s *ExperienceStore) SaveExperience(exp Experience) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if exp.ID == "" {
		exp.ID = fmt.Sprintf("exp_%d", time.Now().UnixNano())
	}
	if exp.Timestamp.IsZero() {
		exp.Timestamp = time.Now()
	}

	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load existing: %w", err)
	}

	s.exps = append(s.exps, exp)
	s.rebuildIndex()

	return s.save()
}

// SearchByPattern finds experiences matching a pattern (keyword overlap).
func (s *ExperienceStore) SearchByPattern(pattern string, topK int) []Experience {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.load(); err != nil {
		return nil
	}

	if topK <= 0 {
		topK = 5
	}

	keywords := tokenize(pattern)
	if len(keywords) == 0 {
		return nil
	}

	type scored struct {
		exp   Experience
		score float64
	}

	var results []scored
	for _, exp := range s.exps {
		expLower := strings.ToLower(exp.ErrorPattern + " " + exp.FixSolution)
		score := 0.0
		for _, kw := range keywords {
			if strings.Contains(expLower, kw) {
				score += 1.0
			}
		}
		if score > 0 {
			// Boost verified entries
			if exp.HumanVerified {
				score *= 1.5
			}
			results = append(results, scored{exp: exp, score: score})
		}
	}

	// Sort by score descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if topK > len(results) {
		topK = len(results)
	}

	out := make([]Experience, topK)
	for i := 0; i < topK; i++ {
		out[i] = results[i].exp
	}

	return out
}

// MarkVerified sets the human-verified flag on an experience.
func (s *ExperienceStore) MarkVerified(expID string, verified bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.load(); err != nil {
		return fmt.Errorf("load: %w", err)
	}

	for i := range s.exps {
		if s.exps[i].ID == expID {
			s.exps[i].HumanVerified = verified
			return s.save()
		}
	}

	return fmt.Errorf("experience not found: %s", expID)
}

// GetAll returns all experiences.
func (s *ExperienceStore) GetAll() []Experience {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.load()
	out := make([]Experience, len(s.exps))
	copy(out, s.exps)
	return out
}

// Count returns the total number of experiences.
func (s *ExperienceStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.load()
	return len(s.exps)
}

// ═══════════════════════════════════════════════════════
// Internal helpers
// ═══════════════════════════════════════════════════════

func (s *ExperienceStore) load() error {
	path := filepath.Join(s.dir, "experience.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.exps)
}

func (s *ExperienceStore) save() error {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.exps, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, "experience.json")
	return os.WriteFile(path, data, 0644)
}

func (s *ExperienceStore) rebuildIndex() {
	s.byPattern = make(map[string][]int)
	for i, exp := range s.exps {
		keywords := tokenize(exp.ErrorPattern)
		for _, kw := range keywords {
			s.byPattern[kw] = append(s.byPattern[kw], i)
		}
	}
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	words := strings.Fields(s)
	var tokens []string
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?\"'()-")
		if len(w) >= 3 {
			tokens = append(tokens, w)
		}
	}
	return tokens
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}
