package feedback

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Action represents what the user did with the generated code.
type Action string

const (
	ActionAccept Action = "accept"
	ActionReject Action = "reject"
	ActionEdit   Action = "edit"
)

// Feedback stores a single user feedback record.
type Feedback struct {
	ID            string    `json:"id"`
	Prompt        string    `json:"prompt"`
	GeneratedCode string    `json:"generated_code"`
	FinalCode     string    `json:"final_code,omitempty"`
	Action        Action    `json:"action"`
	Timestamp     time.Time `json:"timestamp"`
	ErrorSummary  string    `json:"error_summary,omitempty"`
}

// Store manages persistent feedback storage.
type Store struct {
	dir       string
	feedbacks []Feedback
	mu        sync.RWMutex
}

// NewStore creates a feedback store backed by JSON files in dataDir.
func NewStore(dataDir string) *Store {
	return &Store{dir: dataDir}
}

// SaveFeedback persists a feedback record.
func (s *Store) SaveFeedback(fb Feedback) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if fb.ID == "" {
		fb.ID = fmt.Sprintf("fb_%d", time.Now().UnixNano())
	}
	if fb.Timestamp.IsZero() {
		fb.Timestamp = time.Now()
	}

	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load existing feedback: %w", err)
	}

	s.feedbacks = append(s.feedbacks, fb)
	return s.save()
}

// GetRecentFeedback returns the most recent N feedback records.
func (s *Store) GetRecentFeedback(limit int) ([]Feedback, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.load(); err != nil {
		return nil, fmt.Errorf("load feedback: %w", err)
	}

	if limit <= 0 || limit > len(s.feedbacks) {
		limit = len(s.feedbacks)
	}

	// Sort by timestamp descending
	sorted := make([]Feedback, len(s.feedbacks))
	copy(sorted, s.feedbacks)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.After(sorted[j].Timestamp)
	})

	return sorted[:limit], nil
}

// AnalyzePatterns analyzes feedback to find高频 error patterns.
func (s *Store) AnalyzePatterns() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.load(); err != nil {
		if os.IsNotExist(err) {
			return "No feedback data available.", nil
		}
		return "", fmt.Errorf("load feedback: %w", err)
	}

	if len(s.feedbacks) == 0 {
		return "No feedback data available.", nil
	}

	var sb strings.Builder

	// Count actions
	accept, reject, edit := 0, 0, 0
	for _, fb := range s.feedbacks {
		switch fb.Action {
		case ActionAccept:
			accept++
		case ActionReject:
			reject++
		case ActionEdit:
			edit++
		}
	}

	total := len(s.feedbacks)
	acceptRate := float64(accept) / float64(total) * 100

	sb.WriteString(fmt.Sprintf("## Feedback Summary (%d records)\n\n", total))
	sb.WriteString(fmt.Sprintf("- Accept: %d (%.1f%%)\n", accept, acceptRate))
	sb.WriteString(fmt.Sprintf("- Reject: %d (%.1f%%)\n", reject, float64(reject)/float64(total)*100))
	sb.WriteString(fmt.Sprintf("- Edit:   %d (%.1f%%)\n\n", edit, float64(edit)/float64(total)*100))

	// Analyze error patterns in rejected/edited feedback
	errorCounts := make(map[string]int)
	for _, fb := range s.feedbacks {
		if fb.Action == ActionReject || fb.Action == ActionEdit {
			if fb.ErrorSummary != "" {
				// Normalize error message (take first line, lowercase)
				errLine := strings.ToLower(strings.Split(fb.ErrorSummary, "\n")[0])
				errLine = strings.TrimSpace(errLine)
				if len(errLine) > 100 {
					errLine = errLine[:100]
				}
				errorCounts[errLine]++
			}
		}
	}

	if len(errorCounts) > 0 {
		sb.WriteString("## Top Error Patterns\n\n")

		type errCount struct {
			err   string
			count int
		}
		var errs []errCount
		for e, c := range errorCounts {
			errs = append(errs, errCount{e, c})
		}
		sort.Slice(errs, func(i, j int) bool {
			return errs[i].count > errs[j].count
		})

		limit := 10
		if len(errs) < limit {
			limit = len(errs)
		}
		for i := 0; i < limit; i++ {
			sb.WriteString(fmt.Sprintf("%d. [%d times] %s\n", i+1, errs[i].count, errs[i].err))
		}
		sb.WriteString("\n")
	}

	// Analyze prompt keywords in rejected feedback
	if reject+edit > 0 {
		wordCounts := make(map[string]int)
		for _, fb := range s.feedbacks {
			if fb.Action == ActionReject || fb.Action == ActionEdit {
				words := strings.Fields(strings.ToLower(fb.Prompt))
				for _, w := range words {
					if len(w) > 3 { // skip short words
						wordCounts[w]++
					}
				}
			}
		}

		if len(wordCounts) > 0 {
			sb.WriteString("## Common Keywords in Rejected/Edited Prompts\n\n")

			type wordCount struct {
				word  string
				count int
			}
			var words []wordCount
			for w, c := range wordCounts {
				words = append(words, wordCount{w, c})
			}
			sort.Slice(words, func(i, j int) bool {
				return words[i].count > words[j].count
			})

			limit := 10
			if len(words) < limit {
				limit = len(words)
			}
			for i := 0; i < limit; i++ {
				sb.WriteString(fmt.Sprintf("- %s (%d)\n", words[i].word, words[i].count))
			}
		}
	}

	return sb.String(), nil
}

// load reads feedback from disk (must hold lock).
func (s *Store) load() error {
	path := filepath.Join(s.dir, "feedback.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.feedbacks)
}

// save writes feedback to disk (must hold lock).
func (s *Store) save() error {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.feedbacks, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, "feedback.json")
	return os.WriteFile(path, data, 0644)
}
