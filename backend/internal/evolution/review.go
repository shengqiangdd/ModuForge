package evolution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ReviewReport is a post-task retrospective.
type ReviewReport struct {
	TaskID                 string        `json:"task_id"`
	Requirement            string        `json:"requirement"`
	Success                bool          `json:"success"`
	Duration               time.Duration `json:"duration"`
	TokensUsed             int           `json:"tokens_used"`
	Issues                 []string      `json:"issues,omitempty"`
	ImprovementSuggestions []string      `json:"improvement_suggestions,omitempty"`
	Timestamp              time.Time     `json:"timestamp"`
}

// CapabilityScores rates different agent capabilities (0-100).
type CapabilityScores struct {
	CodeGeneration float64 `json:"code_generation"`
	FixCapability  float64 `json:"fix_capability"`
	SpecCompliance float64 `json:"spec_compliance"`
}

// WeeklySummary aggregates review data for a week.
type WeeklySummary struct {
	TotalTasks      int              `json:"total_tasks"`
	SuccessRate     float64          `json:"success_rate"`
	AvgDuration     float64          `json:"avg_duration_seconds"`
	TopIssues       []string         `json:"top_issues"`
	CapabilityRadar CapabilityScores `json:"capability_radar"`
}

// ReviewStore manages review reports.
type ReviewStore struct {
	mu      sync.RWMutex
	dir     string
	reviews []ReviewReport
}

// NewReviewStore creates a review store.
func NewReviewStore(dataDir string) *ReviewStore {
	return &ReviewStore{dir: dataDir}
}

// GenerateReview creates a review report from task results.
func (rs *ReviewStore) GenerateReview(taskID, requirement string, success bool, duration time.Duration, tokensUsed int, issues []string) ReviewReport {
	suggestions := generateSuggestions(issues, success)

	report := ReviewReport{
		TaskID:                 taskID,
		Requirement:            requirement,
		Success:                success,
		Duration:               duration,
		TokensUsed:             tokensUsed,
		Issues:                 issues,
		ImprovementSuggestions: suggestions,
		Timestamp:              time.Now(),
	}

	return report
}

// SaveReview persists a review report.
func (rs *ReviewStore) SaveReview(report ReviewReport) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if err := rs.load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load: %w", err)
	}

	rs.reviews = append(rs.reviews, report)
	return rs.save()
}

// GetWeeklySummary aggregates reviews from the past 7 days.
func (rs *ReviewStore) GetWeeklySummary() WeeklySummary {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	rs.load()

	cutoff := time.Now().AddDate(0, 0, -7)
	var recent []ReviewReport
	for _, r := range rs.reviews {
		if r.Timestamp.After(cutoff) {
			recent = append(recent, r)
		}
	}

	if len(recent) == 0 {
		return WeeklySummary{}
	}

	successCount := 0
	totalDuration := 0.0
	issueCounts := make(map[string]int)

	for _, r := range recent {
		if r.Success {
			successCount++
		}
		totalDuration += r.Duration.Seconds()
		for _, issue := range r.Issues {
			issueCounts[issue]++
		}
	}

	total := float64(len(recent))
	successRate := float64(successCount) / total * 100
	avgDuration := totalDuration / total

	// Top issues
	type ic struct {
		issue string
		count int
	}
	var sorted []ic
	for issue, count := range issueCounts {
		sorted = append(sorted, ic{issue, count})
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	var topIssues []string
	limit := 5
	if len(sorted) < limit {
		limit = len(sorted)
	}
	for i := 0; i < limit; i++ {
		topIssues = append(topIssues, sorted[i].issue)
	}

	// Capability scores
	caps := CapabilityScores{
		CodeGeneration: calcCapability(recent, "code_generation"),
		FixCapability:  calcCapability(recent, "fix_capability"),
		SpecCompliance: calcCapability(recent, "spec_compliance"),
	}

	return WeeklySummary{
		TotalTasks:      len(recent),
		SuccessRate:     successRate,
		AvgDuration:     avgDuration,
		TopIssues:       topIssues,
		CapabilityRadar: caps,
	}
}

// GetAll returns all reviews.
func (rs *ReviewStore) GetAll() []ReviewReport {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	rs.load()
	out := make([]ReviewReport, len(rs.reviews))
	copy(out, rs.reviews)
	return out
}

// ═══════════════════════════════════════════════════════
// Internal helpers
// ═══════════════════════════════════════════════════════

func (rs *ReviewStore) load() error {
	path := filepath.Join(rs.dir, "reviews.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &rs.reviews)
}

func (rs *ReviewStore) save() error {
	if err := os.MkdirAll(rs.dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rs.reviews, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(rs.dir, "reviews.json"), data, 0644)
}

func generateSuggestions(issues []string, success bool) []string {
	var suggestions []string

	if !success {
		suggestions = append(suggestions, "Review failed task for root cause analysis")
	}

	for _, issue := range issues {
		lower := issue
		switch {
		case containsAny(lower, []string{"syntax", "parse"}):
			suggestions = append(suggestions, "Add syntax validation step before code generation")
		case containsAny(lower, []string{"timeout", "slow"}):
			suggestions = append(suggestions, "Consider increasing timeout or optimizing execution")
		case containsAny(lower, []string{"memory", "oom"}):
			suggestions = append(suggestions, "Reduce memory footprint or add resource limits")
		case containsAny(lower, []string{"permission", "access"}):
			suggestions = append(suggestions, "Add permission check before file operations")
		default:
			suggestions = append(suggestions, fmt.Sprintf("Investigate issue: %s", issue))
		}
	}

	return suggestions
}

func calcCapability(reviews []ReviewReport, category string) float64 {
	if len(reviews) == 0 {
		return 50.0
	}

	successCount := 0
	for _, r := range reviews {
		if r.Success {
			successCount++
		}
	}

	base := float64(successCount) / float64(len(reviews)) * 100

	// Adjust based on category-specific signals
	switch category {
	case "fix_capability":
		fixCount := 0
		for _, r := range reviews {
			for _, issue := range r.Issues {
				if containsAny(issue, []string{"fix", "repair", "error"}) {
					fixCount++
				}
			}
		}
		if fixCount > 0 {
			base = base * 0.9 // Slightly lower if many fixes needed
		}
	case "spec_compliance":
		specIssues := 0
		for _, r := range reviews {
			for _, issue := range r.Issues {
				if containsAny(issue, []string{"spec", "convention", "format"}) {
					specIssues++
				}
			}
		}
		if specIssues > 0 {
			base = base * 0.85
		}
	}

	if base > 100 {
		base = 100
	}
	if base < 0 {
		base = 0
	}

	return base
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
