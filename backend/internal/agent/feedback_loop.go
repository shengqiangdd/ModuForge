package agent

import (
	"sync"
	"time"

	"github.com/moduforge/backend/internal/evolution"
)

// FeedbackLoop tracks build outcomes and feeds them back to improve future generations.
type FeedbackLoop struct {
	mu              sync.Mutex
	experienceStore *evolution.ExperienceStore
	sessionMemory   *SessionMemory
	metrics         FeedbackMetrics
}

// FeedbackMetrics tracks feedback loop performance.
type FeedbackMetrics struct {
	TotalBuilds     int     `json:"total_builds"`
	SuccessBuilds   int     `json:"success_builds"`
	FailureBuilds   int     `json:"failure_builds"`
	AverageFixTime  float64 `json:"average_fix_time_seconds"`
	PatternAccuracy float64 `json:"pattern_accuracy"`
}

// BuildOutcome represents the result of a single build attempt.
type BuildOutcome struct {
	TaskID        string
	Language      string
	Success       bool
	Duration      time.Duration
	ErrorType     string
	FixStrategy   string
	FilesModified []string
	PatternsUsed  []string
}

// NewFeedbackLoop creates a new feedback loop.
func NewFeedbackLoop(dataDir string) *FeedbackLoop {
	return &FeedbackLoop{
		experienceStore: evolution.NewExperienceStore(dataDir),
		sessionMemory:   NewSessionMemory(dataDir),
	}
}

// RecordOutcome records a build outcome for learning.
func (fl *FeedbackLoop) RecordOutcome(outcome BuildOutcome) {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	fl.metrics.TotalBuilds++
	if outcome.Success {
		fl.metrics.SuccessBuilds++
		fl.sessionMemory.RecordSuccess(outcome.Language, outcome.Language, outcome.PatternsUsed, 80)
	} else {
		fl.metrics.FailureBuilds++
		if outcome.FixStrategy != "" {
			fl.sessionMemory.RecordFailure(outcome.ErrorType, outcome.FixStrategy)
		}
	}

	// Update average fix time
	if fl.metrics.TotalBuilds > 0 {
		fl.metrics.AverageFixTime = (fl.metrics.AverageFixTime*float64(fl.metrics.TotalBuilds-1) + outcome.Duration.Seconds()) / float64(fl.metrics.TotalBuilds)
	}

	// Update pattern accuracy
	if fl.metrics.TotalBuilds > 0 {
		fl.metrics.PatternAccuracy = float64(fl.metrics.SuccessBuilds) / float64(fl.metrics.TotalBuilds)
	}

	// Record to experience store
	fl.experienceStore.SaveExperience(evolution.Experience{
		ErrorPattern: outcome.ErrorType,
		FixSolution:  outcome.FixStrategy,
		SuccessRate:  boolToFloat(outcome.Success),
		Timestamp:    time.Now(),
		Source:       "feedback_loop",
	})
}

// GetRecommendations returns learning-based recommendations for a task.
func (fl *FeedbackLoop) GetRecommendations(language string) []string {
	return fl.sessionMemory.GetRecommendations(language)
}

// GetFixStrategy returns the best known fix for an error type.
func (fl *FeedbackLoop) GetFixStrategy(errorType string) string {
	return fl.sessionMemory.GetFixStrategy(errorType)
}

// GetMetrics returns current feedback loop metrics.
func (fl *FeedbackLoop) GetMetrics() FeedbackMetrics {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	return fl.metrics
}
