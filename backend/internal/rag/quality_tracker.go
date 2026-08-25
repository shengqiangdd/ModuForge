package rag

import (
	"sync"
	"time"
)

// QualityTracker tracks RAG retrieval quality over time.
type QualityTracker struct {
	mu         sync.Mutex
	queries    []QueryRecord
	maxRecords int
}

// QueryRecord represents a single RAG query and its outcome.
type QueryRecord struct {
	Query        string
	Retrieved    []string // file paths retrieved
	UsedFiles    []string // files actually used in generated code
	Timestamp    time.Time
	PrecisionAtK float64 // how many retrieved files were actually used
}

// NewQualityTracker creates a new quality tracker.
func NewQualityTracker() *QualityTracker {
	return &QualityTracker{
		maxRecords: 1000,
	}
}

// RecordQuery records a RAG query and its outcome.
func (qt *QualityTracker) RecordQuery(query string, retrieved []string, usedFiles []string) {
	qt.mu.Lock()
	defer qt.mu.Unlock()

	// Calculate precision@K
	usedSet := make(map[string]bool)
	for _, f := range usedFiles {
		usedSet[f] = true
	}

	hits := 0
	for _, r := range retrieved {
		if usedSet[r] {
			hits++
		}
	}

	precision := 0.0
	if len(retrieved) > 0 {
		precision = float64(hits) / float64(len(retrieved))
	}

	qt.queries = append(qt.queries, QueryRecord{
		Query:        query,
		Retrieved:    retrieved,
		UsedFiles:    usedFiles,
		Timestamp:    time.Now(),
		PrecisionAtK: precision,
	})

	// Keep only recent records
	if len(qt.queries) > qt.maxRecords {
		qt.queries = qt.queries[len(qt.queries)-qt.maxRecords:]
	}
}

// GetAveragePrecision returns the average precision across all queries.
func (qt *QualityTracker) GetAveragePrecision() float64 {
	qt.mu.Lock()
	defer qt.mu.Unlock()

	if len(qt.queries) == 0 {
		return 0
	}

	total := 0.0
	for _, q := range qt.queries {
		total += q.PrecisionAtK
	}
	return total / float64(len(qt.queries))
}

// GetRecentQueries returns the most recent queries.
func (qt *QualityTracker) GetRecentQueries(n int) []QueryRecord {
	qt.mu.Lock()
	defer qt.mu.Unlock()

	if n > len(qt.queries) {
		n = len(qt.queries)
	}
	return qt.queries[len(qt.queries)-n:]
}

// QueryCount returns the total number of recorded queries.
func (qt *QualityTracker) QueryCount() int {
	qt.mu.Lock()
	defer qt.mu.Unlock()
	return len(qt.queries)
}
