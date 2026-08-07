package memory

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// VectorSearch provides simple in-memory vector search for memory entries.
// Uses TF-IDF-like feature extraction and cosine similarity for ranking.
// No external dependencies — pure in-memory implementation suitable for <10K records.
type VectorSearch struct {
	mu        sync.RWMutex
	documents map[string]*Document // id -> document
	dim       int                 // vector dimension (fixed at 128)
	idf       map[string]float64  // inverse document frequency
	docCount  int
}

// Document represents a memory entry with its vector representation.
type Document struct {
	ID       string
	Content  string
	Vector   []float64
	Metadata map[string]interface{}
}

// SearchResult represents a scored search result.
type SearchResult struct {
	ID       string
	Score    float64
	Content  string
	Metadata map[string]interface{}
}

// NewVectorSearch creates a new vector search index.
func NewVectorSearch() *VectorSearch {
	return &VectorSearch{
		documents: make(map[string]*Document),
		dim:       128,
		idf:       make(map[string]float64),
	}
}

// AddDocument adds a document to the index.
func (vs *VectorSearch) AddDocument(id, content string, metadata map[string]interface{}) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	vector := vs.textToVector(content)
	vs.documents[id] = &Document{
		ID:       id,
		Content:  content,
		Vector:   vector,
		Metadata: metadata,
	}
	vs.docCount++
	vs.updateIDF()
}

// RemoveDocument removes a document from the index.
func (vs *VectorSearch) RemoveDocument(id string) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	delete(vs.documents, id)
	vs.docCount = len(vs.documents)
	vs.updateIDF()
}

// Search finds documents similar to the query text.
func (vs *VectorSearch) Search(query string, limit int) []SearchResult {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if len(vs.documents) == 0 {
		return nil
	}

	queryVector := vs.textToVector(query)

	// Score all documents
	results := make([]SearchResult, 0, len(vs.documents))
	for _, doc := range vs.documents {
		score := cosineSimilarity(queryVector, doc.Vector)
		if score > 0.01 { // minimum threshold
			results = append(results, SearchResult{
				ID:       doc.ID,
				Score:    score,
				Content:  doc.Content,
				Metadata: doc.Metadata,
			})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit results
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}

// textToVector converts text to a fixed-dimensional vector using TF-IDF features.
func (vs *VectorSearch) textToVector(text string) []float64 {
	vector := make([]float64, vs.dim)
	tokens := tokenize(text)

	if len(tokens) == 0 {
		return vector
	}

	// Term frequency
	tf := make(map[string]int)
	for _, token := range tokens {
		tf[token]++
	}

	// Convert to vector using hash-based projection
	for token, count := range tf {
		// TF component
		tfScore := float64(count) / float64(len(tokens))

		// IDF component (if available)
		idfScore := 1.0
		if idf, ok := vs.idf[token]; ok {
			idfScore = idf
		}

		// Hash token to vector dimensions
		weight := tfScore * idfScore
		hash := simpleHash(token)
		idx1 := int(hash % uint64(vs.dim))
		idx2 := int((hash >> 8) % uint64(vs.dim))
		idx3 := int((hash >> 16) % uint64(vs.dim))

		vector[idx1] += weight
		vector[idx2] += weight * 0.5
		vector[idx3] += weight * 0.25
	}

	// Normalize
	norm := 0.0
	for _, v := range vector {
		norm += v * v
	}
	if norm > 0 {
		norm = math.Sqrt(norm)
		for i := range vector {
			vector[i] /= norm
		}
	}

	return vector
}

// updateIDF recalculates inverse document frequency.
func (vs *VectorSearch) updateIDF() {
	// Count document frequency for each term
	df := make(map[string]int)
	for _, doc := range vs.documents {
		seen := make(map[string]bool)
		for _, token := range tokenize(doc.Content) {
			if !seen[token] {
				df[token]++
				seen[token] = true
			}
		}
	}

	// Calculate IDF: log(N / df)
	for term, freq := range df {
		if freq > 0 {
			vs.idf[term] = math.Log(float64(vs.docCount) / float64(freq))
		}
	}
}

// tokenize splits text into lowercase tokens.
func tokenize(text string) []string {
	// Remove special characters, keep alphanumeric and CJK
	reg := regexp.MustCompile(`[a-zA-Z0-9\x{4e00}-\x{9fff}]+`)
	words := reg.FindAllString(strings.ToLower(text), -1)

	// Filter short tokens and common stop words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "shall": true, "can": true, "need": true,
		"dare": true, "ought": true, "used": true, "to": true, "of": true,
		"in": true, "for": true, "on": true, "with": true, "at": true,
		"by": true, "from": true, "as": true, "into": true, "through": true,
		"during": true, "before": true, "after": true, "above": true, "below": true,
		"between": true, "out": true, "off": true, "over": true, "under": true,
		"again": true, "further": true, "then": true, "once": true, "here": true,
		"there": true, "when": true, "where": true, "why": true, "how": true,
		"all": true, "both": true, "each": true, "few": true, "more": true,
		"most": true, "other": true, "some": true, "such": true, "no": true,
		"nor": true, "not": true, "only": true, "own": true, "same": true,
		"so": true, "than": true, "too": true, "very": true, "just": true,
		"don": true, "now": true, "的": true, "了": true, "在": true,
		"是": true, "我": true, "有": true, "和": true, "就": true,
		"不": true, "人": true, "都": true, "一": true, "一个": true,
		"上": true, "也": true, "很": true, "到": true, "说": true,
		"要": true, "去": true, "你": true, "会": true, "着": true,
		"没有": true, "看": true, "好": true, "自己": true, "这": true,
	}

	var tokens []string
	for _, word := range words {
		if len(word) > 1 && !stopWords[word] {
			tokens = append(tokens, word)
		}
	}
	return tokens
}

// simpleHash computes a simple hash for a string.
func simpleHash(s string) uint64 {
	var hash uint64 = 5381
	for _, c := range s {
		hash = hash*33 + uint64(c)
	}
	return hash
}

// cosineSimilarity computes cosine similarity between two vectors.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// Size returns the number of documents in the index.
func (vs *VectorSearch) Size() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return len(vs.documents)
}

// Clear removes all documents from the index.
func (vs *VectorSearch) Clear() {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.documents = make(map[string]*Document)
	vs.idf = make(map[string]float64)
	vs.docCount = 0
}
