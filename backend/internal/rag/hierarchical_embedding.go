package rag

import (
	"fmt"
	"strings"
)

// EmbedLevel represents the granularity level of an embedding.
type EmbedLevel string

const (
	LevelFile     EmbedLevel = "file"
	LevelFunction EmbedLevel = "function"
	LevelSnippet  EmbedLevel = "snippet"
)

// LayeredVector is a vector at a specific embedding level.
type LayeredVector struct {
	Level    EmbedLevel         `json:"level"`
	Vector   map[string]float64 `json:"vector"`
	Source   string             `json:"source"`
	Metadata map[string]string  `json:"metadata,omitempty"`
}

// SearchResult is a scored result from hierarchical search.
type SearchResult struct {
	Chunk CodeChunk  `json:"chunk"`
	Score float64    `json:"score"`
	Level EmbedLevel `json:"level"`
}

// HierarchicalEmbedder creates multi-level embeddings for code.
type HierarchicalEmbedder struct {
	fileIDF     map[string]float64
	functionIDF map[string]float64
}

// NewHierarchicalEmbedder creates a new embedder.
func NewHierarchicalEmbedder() *HierarchicalEmbedder {
	return &HierarchicalEmbedder{}
}

// EmbedAtLevels creates embeddings at file, function, and snippet levels.
func (he *HierarchicalEmbedder) EmbedAtLevels(content string, filePath string) []LayeredVector {
	var vectors []LayeredVector

	// 1. File-level: entire file content
	fileVec := computeTFIDFGlobal(content)
	vectors = append(vectors, LayeredVector{
		Level:  LevelFile,
		Vector: fileVec,
		Source: filePath,
		Metadata: map[string]string{
			"type": "file",
			"file": filePath,
		},
	})

	// 2. Function-level: split by function boundaries
	functions := splitByFunctions(content, filePath)
	for _, fn := range functions {
		fnVec := computeTFIDFGlobal(fn.Content)
		vectors = append(vectors, LayeredVector{
			Level:  LevelFunction,
			Vector: fnVec,
			Source: filePath,
			Metadata: map[string]string{
				"type":     "function",
				"file":     filePath,
				"function": fn.Name,
				"line":     fmt.Sprintf("%d", fn.Line),
			},
		})
	}

	// 3. Snippet-level: fixed-size overlapping chunks
	snippets := chunkText(content, 256, 32)
	for i, snippet := range snippets {
		snippetVec := computeTFIDFGlobal(snippet)
		vectors = append(vectors, LayeredVector{
			Level:  LevelSnippet,
			Vector: snippetVec,
			Source: filePath,
			Metadata: map[string]string{
				"type":  "snippet",
				"file":  filePath,
				"index": fmt.Sprintf("%d", i),
			},
		})
	}

	return vectors
}

// SearchHierarchical performs coarse-to-fine hierarchical search.
// 1. File-level: find top candidates
// 2. Function-level: refine within candidates
func (he *HierarchicalEmbedder) SearchHierarchical(
	query string,
	allVectors []LayeredVector,
	topK int,
) []SearchResult {
	if topK <= 0 {
		topK = 5
	}

	queryVec := computeTFIDFGlobal(query)
	if len(queryVec) == 0 {
		return nil
	}

	// Phase 1: Coarse ranking at file level
	fileScores := make(map[string]float64)
	for _, v := range allVectors {
		if v.Level != LevelFile {
			continue
		}
		sim := CosineSimilarity(queryVec, v.Vector)
		fileScores[v.Source] = sim
	}

	// Sort files by score, take top candidates
	type fileScore struct {
		file  string
		score float64
	}
	var sorted []fileScore
	for f, s := range fileScores {
		sorted = append(sorted, fileScore{f, s})
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].score > sorted[i].score {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	candidateLimit := topK * 2
	if candidateLimit > len(sorted) {
		candidateLimit = len(sorted)
	}
	candidateFiles := make(map[string]bool)
	for i := 0; i < candidateLimit; i++ {
		candidateFiles[sorted[i].file] = true
	}

	// Phase 2: Fine ranking within candidate files
	var results []SearchResult
	for _, v := range allVectors {
		if v.Level == LevelFile {
			continue // Skip file-level in final results
		}
		if !candidateFiles[v.Source] {
			continue
		}

		sim := CosineSimilarity(queryVec, v.Vector)
		if sim < 0.01 {
			continue
		}

		chunk := CodeChunk{
			ID:       fmt.Sprintf("%s:%s:%s", v.Source, v.Level, v.Metadata["function"]),
			Source:   v.Source,
			Content:  fmt.Sprintf("[%s] %s", v.Level, v.Source),
			Vector:   v.Vector,
			Metadata: v.Metadata,
		}

		results = append(results, SearchResult{
			Chunk: chunk,
			Score: sim,
			Level: v.Level,
		})
	}

	// Sort by score descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if topK > len(results) {
		topK = len(results)
	}

	return results[:topK]
}

// funcInfo holds parsed function boundaries.
type funcInfo struct {
	Name    string
	Content string
	Line    int
}

// splitByFunctions splits code into function-level chunks.
func splitByFunctions(content string, filePath string) []funcInfo {
	lines := strings.Split(content, "\n")
	var functions []funcInfo

	ext := ""
	if idx := strings.LastIndex(filePath, "."); idx >= 0 {
		ext = filePath[idx:]
	}

	switch ext {
	case ".go":
		functions = splitGoFunctions(lines, filePath)
	case ".sh":
		functions = splitShellFunctions(lines, filePath)
	}

	return functions
}

// splitGoFunctions extracts Go functions.
func splitGoFunctions(lines []string, filePath string) []funcInfo {
	var funcs []funcInfo
	inFunc := false
	funcName := ""
	funcStart := 0
	braceDepth := 0
	var funcLines []string

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !inFunc {
			// Match func declaration
			if strings.HasPrefix(trimmed, "func ") {
				inFunc = true
				funcStart = i + 1
				braceDepth = 0
				funcLines = nil

				// Extract function name
				trimmed2 := strings.TrimPrefix(trimmed, "func ")
				if idx := strings.Index(trimmed2, "("); idx > 0 {
					funcName = strings.TrimSpace(trimmed2[:idx])
				} else if idx := strings.Index(trimmed2, "{"); idx > 0 {
					funcName = strings.TrimSpace(trimmed2[:idx])
				}
			}
		}

		if inFunc {
			funcLines = append(funcLines, line)
			braceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")

			if braceDepth <= 0 && strings.Contains(trimmed, "}") {
				funcs = append(funcs, funcInfo{
					Name:    funcName,
					Content: strings.Join(funcLines, "\n"),
					Line:    funcStart,
				})
				inFunc = false
				funcName = ""
			}
		}
	}

	return funcs
}

// splitShellFunctions extracts Shell functions.
func splitShellFunctions(lines []string, filePath string) []funcInfo {
	var funcs []funcInfo
	inFunc := false
	funcName := ""
	funcStart := 0
	braceDepth := 0
	var funcLines []string

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !inFunc {
			// Match function definition: name() {
			if strings.HasSuffix(trimmed, "() {") || strings.HasSuffix(trimmed, "(){") {
				inFunc = true
				funcStart = i + 1
				braceDepth = 0
				funcLines = nil
				funcName = strings.TrimSuffix(strings.TrimSuffix(trimmed, " {"), "()")
			}
		}

		if inFunc {
			funcLines = append(funcLines, line)
			braceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")

			if braceDepth <= 0 && strings.Contains(trimmed, "}") {
				funcs = append(funcs, funcInfo{
					Name:    funcName,
					Content: strings.Join(funcLines, "\n"),
					Line:    funcStart,
				})
				inFunc = false
				funcName = ""
			}
		}
	}

	return funcs
}

// computeTFIDFGlobal computes TF-IDF using a simple global IDF.
// This is a simplified version for hierarchical search.
func computeTFIDFGlobal(text string) map[string]float64 {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return nil
	}

	tf := make(map[string]float64)
	for _, t := range tokens {
		tf[t]++
	}

	maxTF := 0.0
	for _, v := range tf {
		if v > maxTF {
			maxTF = v
		}
	}

	// Simple TF normalization without IDF (since we don't have corpus stats)
	vector := make(map[string]float64)
	for term, freq := range tf {
		vector[term] = 0.5 + 0.5*(freq/maxTF)
	}

	return vector
}
