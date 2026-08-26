package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ModelCandidate holds a generation result from one model.
type ModelCandidate struct {
	Model      string
	Provider   string
	Content    string
	Duration   time.Duration
	TokensUsed int
	Quality    float64 // 0.0 - 1.0 estimated quality score
	Error      error
}

// EnsembleGenerator runs multiple models in parallel and picks the best result.
type EnsembleGenerator struct {
	mu      sync.Mutex
	caller  llmCaller
	latency map[string]time.Duration // model -> avg latency
	success map[string]int           // model -> success count
	failure map[string]int           // model -> failure count
}

// NewEnsembleGenerator creates a new ensemble generator.
func NewEnsembleGenerator(caller llmCaller) *EnsembleGenerator {
	return &EnsembleGenerator{
		caller:  caller,
		latency: make(map[string]time.Duration),
		success: make(map[string]int),
		failure: make(map[string]int),
	}
}

// SetCaller updates the LLM caller (e.g. after runner resolves the real endpoint).
func (eg *EnsembleGenerator) SetCaller(caller llmCaller) {
	eg.mu.Lock()
	defer eg.mu.Unlock()
	eg.caller = caller
}

// GenerateWithEnsemble runs the same prompt across multiple models in parallel
// and returns the best candidate based on quality scoring.
func (eg *EnsembleGenerator) GenerateWithEnsemble(
	ctx context.Context,
	prompt string,
	models []string,
	maxTokens int,
) *ModelCandidate {
	if len(models) == 0 {
		return &ModelCandidate{Error: fmt.Errorf("no models provided")}
	}
	if len(models) == 1 {
		c := eg.generateSingle(ctx, prompt, models[0], maxTokens)
		return &c
	}

	// Run all models in parallel
	results := make(chan ModelCandidate, len(models))
	for _, model := range models {
		go func(m string) {
			results <- eg.generateSingle(ctx, prompt, m, maxTokens)
		}(model)
	}

	// Collect results
	var candidates []ModelCandidate
	for i := 0; i < len(models); i++ {
		candidates = append(candidates, <-results)
	}

	// Score and pick the best
	best := eg.selectBest(candidates)
	return best
}

// generateSingle calls one model and returns a candidate.
func (eg *EnsembleGenerator) generateSingle(
	ctx context.Context,
	prompt string,
	model string,
	maxTokens int,
) ModelCandidate {
	start := time.Now()

	candidate := ModelCandidate{
		Model: model,
	}

	eg.mu.Lock()
	caller := eg.caller
	eg.mu.Unlock()

	if caller == nil {
		candidate.Error = fmt.Errorf("no LLM caller configured")
		eg.recordFailure(model)
		return candidate
	}

	content, err := caller.CallLLM(ctx, prompt)
	duration := time.Since(start)
	candidate.Duration = duration

	if err != nil {
		eg.recordFailure(model)
		candidate.Error = err
		return candidate
	}

	candidate.Content = content
	candidate.TokensUsed = estimateTokenCount(content)
	candidate.Quality = eg.estimateQuality(content, duration)
	eg.recordSuccess(model, duration)

	return candidate
}

// estimateTokenCount gives a rough token estimate (words / 0.75).
func estimateTokenCount(s string) int {
	words := strings.Fields(s)
	return int(float64(len(words)) / 0.75)
}

// estimateQuality estimates the quality of generated content.
func (eg *EnsembleGenerator) estimateQuality(content string, duration time.Duration) float64 {
	score := 0.5 // base score

	// Longer content is generally better (up to a point)
	length := len(content)
	if length > 100 {
		score += 0.1
	}
	if length > 500 {
		score += 0.1
	}
	if length > 2000 {
		score += 0.05
	}

	// Penalize very short responses
	if length < 20 {
		score -= 0.3
	}

	// Check for code blocks (indicates structured output)
	if strings.Contains(content, "```") {
		score += 0.1
	}

	// Check for Chinese (indicates proper language handling)
	for _, r := range content {
		if r >= 0x4e00 && r <= 0x9fff {
			score += 0.05
			break
		}
	}

	// Penalize if took too long
	if duration > 30*time.Second {
		score -= 0.1
	}

	// Clamp to [0, 1]
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// selectBest picks the best candidate from a list.
func (eg *EnsembleGenerator) selectBest(candidates []ModelCandidate) *ModelCandidate {
	if len(candidates) == 0 {
		return &ModelCandidate{Error: fmt.Errorf("no candidates")}
	}

	var best *ModelCandidate
	for i := range candidates {
		c := &candidates[i]
		if c.Error != nil {
			continue
		}
		if best == nil || c.Quality > best.Quality {
			best = c
		}
	}

	// If all failed, return the first error
	if best == nil {
		for i := range candidates {
			if candidates[i].Error != nil {
				return &candidates[i]
			}
		}
		return &ModelCandidate{Error: fmt.Errorf("all models failed")}
	}

	return best
}

// recordSuccess records a successful call.
func (eg *EnsembleGenerator) recordSuccess(model string, latency time.Duration) {
	eg.mu.Lock()
	defer eg.mu.Unlock()
	eg.success[model]++
	// Update average latency
	n := eg.success[model] + eg.failure[model]
	if n > 0 {
		eg.latency[model] = (eg.latency[model]*time.Duration(n-1) + latency) / time.Duration(n)
	}
}

// recordFailure records a failed call.
func (eg *EnsembleGenerator) recordFailure(model string) {
	eg.mu.Lock()
	defer eg.mu.Unlock()
	eg.failure[model]++
}

// GetModelStats returns performance stats for all models.
func (eg *EnsembleGenerator) GetModelStats() map[string]map[string]interface{} {
	eg.mu.Lock()
	defer eg.mu.Unlock()

	stats := make(map[string]map[string]interface{})
	models := make(map[string]bool)
	for m := range eg.success {
		models[m] = true
	}
	for m := range eg.failure {
		models[m] = true
	}

	for m := range models {
		total := eg.success[m] + eg.failure[m]
		var successRate float64
		if total > 0 {
			successRate = float64(eg.success[m]) / float64(total) * 100
		}
		stats[m] = map[string]interface{}{
			"success":      eg.success[m],
			"failure":      eg.failure[m],
			"total":        total,
			"success_rate": successRate,
			"avg_latency":  eg.latency[m].String(),
		}
	}
	return stats
}

// RankModels returns models sorted by success rate (best first).
func (eg *EnsembleGenerator) RankModels() []string {
	stats := eg.GetModelStats()
	type modelScore struct {
		model string
		score float64
	}
	var scores []modelScore
	for m, s := range stats {
		scores = append(scores, modelScore{
			model: m,
			score: s["success_rate"].(float64),
		})
	}
	// Sort by score descending
	for i := 0; i < len(scores); i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].score > scores[i].score {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}
	var result []string
	for _, s := range scores {
		result = append(result, s.model)
	}
	return result
}
