package rag

import (
	"fmt"
	"sort"
)

var globalKB *KnowledgeBase

// Init loads or builds the knowledge base.
// Call this once at startup. If the persisted KB exists, load it;
// otherwise ingest from scratch and persist.
func Init(baseDir string) error {
	kb, err := LoadKnowledge(baseDir + "/" + VectordbDir)
	if err != nil {
		// No persisted KB — ingest and save
		kb, err = IngestKnowledge(baseDir)
		if err != nil {
			return fmt.Errorf("ingest knowledge: %w", err)
		}
		if err := kb.Save(baseDir + "/" + VectordbDir); err != nil {
			return fmt.Errorf("save knowledge base: %w", err)
		}
	}

	globalKB = kb
	return nil
}

// SearchRelevant finds the top-K most similar code chunks for a query.
func SearchRelevant(query string, topK int) ([]CodeChunk, error) {
	if globalKB == nil {
		return nil, fmt.Errorf("knowledge base not initialized, call Init() first")
	}
	if topK <= 0 {
		topK = 3
	}

	globalKB.mu.RLock()
	defer globalKB.mu.RUnlock()

	// Vectorize the query using the KB's IDF
	queryVec := computeTFIDF(query, globalKB.IDF)
	if len(queryVec) == 0 {
		return nil, nil
	}

	// Score all chunks
	type scored struct {
		chunk CodeChunk
		score float64
	}

	results := make([]scored, 0, len(globalKB.Chunks))
	for _, chunk := range globalKB.Chunks {
		sim := CosineSimilarity(queryVec, chunk.Vector)
		if sim > 0.01 { // minimum threshold
			results = append(results, scored{chunk: chunk, score: sim})
		}
	}

	// Sort by descending similarity
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// Return top-K
	if topK > len(results) {
		topK = len(results)
	}

	out := make([]CodeChunk, topK)
	for i := 0; i < topK; i++ {
		out[i] = results[i].chunk
	}

	return out, nil
}

// SearchWithThreshold finds chunks above a minimum similarity threshold.
func SearchWithThreshold(query string, minScore float64, maxResults int) ([]CodeChunk, error) {
	if globalKB == nil {
		return nil, fmt.Errorf("knowledge base not initialized")
	}
	if maxResults <= 0 {
		maxResults = 10
	}

	globalKB.mu.RLock()
	defer globalKB.mu.RUnlock()

	queryVec := computeTFIDF(query, globalKB.IDF)
	if len(queryVec) == 0 {
		return nil, nil
	}

	type scored struct {
		chunk CodeChunk
		score float64
	}

	var results []scored
	for _, chunk := range globalKB.Chunks {
		sim := CosineSimilarity(queryVec, chunk.Vector)
		if sim >= minScore {
			results = append(results, scored{chunk: chunk, score: sim})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if maxResults > len(results) {
		maxResults = len(results)
	}

	out := make([]CodeChunk, maxResults)
	for i := 0; i < maxResults; i++ {
		out[i] = results[i].chunk
	}

	return out, nil
}

// GetKB returns the current knowledge base (for testing/inspection).
func GetKB() *KnowledgeBase {
	return globalKB
}

// SetKB sets the global knowledge base (for testing).
func SetKB(kb *KnowledgeBase) {
	globalKB = kb
}
