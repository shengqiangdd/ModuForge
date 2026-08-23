package rag

import (
	"fmt"
	"strings"
)

// SessionMemory stores key-value pairs for conversational context.
type SessionMemory struct {
	Pairs map[string]string `json:"pairs"`
}

// NewSessionMemory creates an empty session memory.
func NewSessionMemory() *SessionMemory {
	return &SessionMemory{
		Pairs: make(map[string]string),
	}
}

// UpdateMemory sets or updates a key-value pair.
func (m *SessionMemory) UpdateMemory(key, value string) {
	if m.Pairs == nil {
		m.Pairs = make(map[string]string)
	}
	m.Pairs[key] = value
}

// GetMemory retrieves a value by key.
func (m *SessionMemory) GetMemory(key string) (string, bool) {
	if m.Pairs == nil {
		return "", false
	}
	v, ok := m.Pairs[key]
	return v, ok
}

// ClearMemory resets all stored pairs.
func (m *SessionMemory) ClearMemory() {
	m.Pairs = make(map[string]string)
}

// ContextString builds a context string from all memory pairs.
func (m *SessionMemory) ContextString() string {
	if len(m.Pairs) == 0 {
		return ""
	}
	var sb strings.Builder
	for k, v := range m.Pairs {
		sb.WriteString(fmt.Sprintf("%s: %s; ", k, v))
	}
	return sb.String()
}

// AllKeys returns all stored keys.
func (m *SessionMemory) AllKeys() []string {
	keys := make([]string, 0, len(m.Pairs))
	for k := range m.Pairs {
		keys = append(keys, k)
	}
	return keys
}

// StatefulRAG extends the base RAG with session-aware retrieval.
type StatefulRAG struct {
	memory *SessionMemory
	kb     *KnowledgeBase
}

// NewStatefulRAG creates a StatefulRAG with a knowledge base.
func NewStatefulRAG(kb *KnowledgeBase) *StatefulRAG {
	return &StatefulRAG{
		memory: NewSessionMemory(),
		kb:     kb,
	}
}

// UpdateMemory updates a session memory key-value pair.
func (sr *StatefulRAG) UpdateMemory(key, value string) {
	sr.memory.UpdateMemory(key, value)
}

// GetMemory retrieves a value from session memory.
func (sr *StatefulRAG) GetMemory(key string) (string, bool) {
	return sr.memory.GetMemory(key)
}

// ClearMemory resets session memory.
func (sr *StatefulRAG) ClearMemory() {
	sr.memory.ClearMemory()
}

// GetMemoryContext returns the current memory as a context string.
func (sr *StatefulRAG) GetMemoryContext() string {
	return sr.memory.ContextString()
}

// SearchWithContext performs retrieval combining the query with session memory.
// Memory context is appended to the query to influence ranking.
func (sr *StatefulRAG) SearchWithContext(query string, topK int) ([]SearchResult, error) {
	if sr.kb == nil {
		return nil, fmt.Errorf("knowledge base not set")
	}
	if topK <= 0 {
		topK = 5
	}

	// Build enhanced query with memory context
	enhancedQuery := query
	if ctx := sr.memory.ContextString(); ctx != "" {
		enhancedQuery = query + " " + ctx
	}

	// Vectorize enhanced query
	queryVec := computeTFIDF(enhancedQuery, sr.kb.IDF)
	if len(queryVec) == 0 {
		return nil, nil
	}

	// Also compute raw query vector for boosting
	rawVec := computeTFIDF(query, sr.kb.IDF)

	// Score all chunks with memory boost
	type scored struct {
		chunk CodeChunk
		score float64
	}

	var results []scored
	for _, chunk := range sr.kb.Chunks {
		// Base similarity with enhanced query
		sim := CosineSimilarity(queryVec, chunk.Vector)

		// Boost if also similar to raw query (reinforcement)
		if len(rawVec) > 0 {
			rawSim := CosineSimilarity(rawVec, chunk.Vector)
			sim = 0.7*sim + 0.3*rawSim
		}

		// Boost chunks that contain memory keywords
		boost := memoryBoost(chunk, sr.memory)
		sim += boost

		if sim > 0.01 {
			results = append(results, scored{chunk: chunk, score: sim})
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

	out := make([]SearchResult, topK)
	for i := 0; i < topK; i++ {
		out[i] = SearchResult{
			Chunk: results[i].chunk,
			Score: results[i].score,
			Level: LevelSnippet,
		}
	}

	return out, nil
}

// memoryBoost computes a score boost based on memory keyword overlap.
func memoryBoost(chunk CodeChunk, memory *SessionMemory) float64 {
	if len(memory.Pairs) == 0 {
		return 0
	}

	content := strings.ToLower(chunk.Content)
	boost := 0.0
	count := 0

	for _, value := range memory.Pairs {
		words := strings.Fields(strings.ToLower(value))
		for _, word := range words {
			if len(word) > 2 && strings.Contains(content, word) {
				boost += 0.05
				count++
			}
		}
	}

	// Cap boost at 0.2 to avoid overwhelming base similarity
	if boost > 0.2 {
		boost = 0.2
	}

	return boost
}
