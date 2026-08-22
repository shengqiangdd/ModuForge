package rag

import (
	"math"
	"sync"
)

// CodeChunk represents a single chunk of code with its feature vector.
type CodeChunk struct {
	ID       string             `json:"id"`
	Source   string             `json:"source"`
	Content  string             `json:"content"`
	Vector   map[string]float64 `json:"vector"`
	Metadata map[string]string  `json:"metadata,omitempty"`
}

// KnowledgeBase holds all indexed code chunks and the IDF values.
type KnowledgeBase struct {
	Chunks []CodeChunk        `json:"chunks"`
	IDF    map[string]float64 `json:"idf"`
	mu     sync.RWMutex
}

// NewKnowledgeBase creates an empty knowledge base.
func NewKnowledgeBase() *KnowledgeBase {
	return &KnowledgeBase{
		Chunks: make([]CodeChunk, 0),
		IDF:    make(map[string]float64),
	}
}

// CosineSimilarity computes cosine similarity between two vectors.
// Both vectors are sparse (map[string]float64).
func CosineSimilarity(a, b map[string]float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64

	// Iterate over the smaller vector for efficiency
	smaller, larger := a, b
	if len(a) > len(b) {
		smaller, larger = b, a
	}

	for term, valA := range smaller {
		if valB, ok := larger[term]; ok {
			dotProduct += valA * valB
		}
		normA += valA * valA
	}

	for _, valB := range larger {
		normB += valB * valB
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
