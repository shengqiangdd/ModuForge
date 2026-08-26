package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FineTuneLoop collects training data for model fine-tuning.
type FineTuneLoop struct {
	mu         sync.Mutex
	dataDir    string
	samples    []TrainingSample
	maxSamples int
}

// TrainingSample represents a training sample for fine-tuning.
type TrainingSample struct {
	ID         string    `json:"id"`
	Prompt     string    `json:"prompt"`
	Completion string    `json:"completion"`
	Rating     int       `json:"rating"` // 1-5
	Timestamp  time.Time `json:"timestamp"`
	Source     string    `json:"source"` // "user_feedback", "implicit"
}

// NewFineTuneLoop creates a new fine-tuning loop.
func NewFineTuneLoop(dataDir string) *FineTuneLoop {
	return &FineTuneLoop{
		dataDir:    dataDir,
		samples:    make([]TrainingSample, 0),
		maxSamples: 10000,
	}
}

// RecordSample records a training sample.
func (ftl *FineTuneLoop) RecordSample(sample TrainingSample) {
	ftl.mu.Lock()
	defer ftl.mu.Unlock()

	ftl.samples = append(ftl.samples, sample)

	// Keep only recent samples
	if len(ftl.samples) > ftl.maxSamples {
		ftl.samples = ftl.samples[len(ftl.samples)-ftl.maxSamples:]
	}

	// Persist to disk
	ftl.persistSamples()
}

// GetSamples returns training samples for export.
func (ftl *FineTuneLoop) GetSamples(limit int) []TrainingSample {
	ftl.mu.Lock()
	defer ftl.mu.Unlock()

	if limit > len(ftl.samples) {
		limit = len(ftl.samples)
	}
	return ftl.samples[len(ftl.samples)-limit:]
}

// ExportForTraining exports samples in training format.
func (ftl *FineTuneLoop) ExportForTraining(outputPath string) error {
	ftl.mu.Lock()
	defer ftl.mu.Unlock()

	data, err := json.MarshalIndent(ftl.samples, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, data, 0644)
}

// persistSamples persists samples to disk.
func (ftl *FineTuneLoop) persistSamples() {
	path := filepath.Join(ftl.dataDir, "training_samples.json")
	data, err := json.MarshalIndent(ftl.samples, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0644)
}

// GetMetrics returns fine-tuning loop metrics.
func (ftl *FineTuneLoop) GetMetrics() map[string]interface{} {
	ftl.mu.Lock()
	defer ftl.mu.Unlock()

	return map[string]interface{}{
		"total_samples": len(ftl.samples),
		"max_samples":   ftl.maxSamples,
	}
}
