package rag

import (
	"math"
	"sort"
)

// QualityMetrics holds evaluation metrics for retrieval quality.
type QualityMetrics struct {
	Precision     float64 `json:"precision"`
	Recall        float64 `json:"recall"`
	NDCG          float64 `json:"ndcg"`
	F1            float64 `json:"f1"`
	TotalQueries  int     `json:"total_queries"`
	MarkedResults int     `json:"marked_results"`
}

// QueryRecord stores the results and relevance labels for a single query.
type QueryRecord struct {
	Query     string          `json:"query"`
	Results   []ScoredResult  `json:"results"`
	Relevance map[string]bool `json:"relevance"` // chunkID -> relevant
}

// ScoredResult is a result with its retrieval score.
type ScoredResult struct {
	ChunkID string  `json:"chunk_id"`
	Score   float64 `json:"score"`
}

// QualityEvaluator tracks and evaluates retrieval quality.
type QualityEvaluator struct {
	records    []QueryRecord
	currentIdx int
}

// NewQualityEvaluator creates a new evaluator.
func NewQualityEvaluator() *QualityEvaluator {
	return &QualityEvaluator{}
}

// StartQuery begins a new query evaluation session.
func (qe *QualityEvaluator) StartQuery(query string, results []ScoredResult) {
	qe.records = append(qe.records, QueryRecord{
		Query:     query,
		Results:   results,
		Relevance: make(map[string]bool),
	})
	qe.currentIdx = len(qe.records) - 1
}

// MarkRelevant marks a chunk as relevant or irrelevant for the current query.
func (qe *QualityEvaluator) MarkRelevant(chunkID string, relevant bool) {
	if qe.currentIdx < 0 || qe.currentIdx >= len(qe.records) {
		return
	}
	qe.records[qe.currentIdx].Relevance[chunkID] = relevant
}

// GetMetrics computes aggregate quality metrics across all queries.
func (qe *QualityEvaluator) GetMetrics() QualityMetrics {
	if len(qe.records) == 0 {
		return QualityMetrics{}
	}

	var totalPrecision, totalRecall, totalNDCG float64
	totalMarked := 0

	for _, record := range qe.records {
		if len(record.Relevance) == 0 {
			continue
		}

		prec := computePrecision(record)
		rec := computeRecall(record)
		ndcg := computeNDCG(record)

		totalPrecision += prec
		totalRecall += rec
		totalNDCG += ndcg

		for _, rel := range record.Relevance {
			if rel {
				totalMarked++
			}
		}
	}

	queriesWithLabels := 0
	for _, record := range qe.records {
		if len(record.Relevance) > 0 {
			queriesWithLabels++
		}
	}

	if queriesWithLabels == 0 {
		return QualityMetrics{
			TotalQueries:  len(qe.records),
			MarkedResults: totalMarked,
		}
	}

	avgPrecision := totalPrecision / float64(queriesWithLabels)
	avgRecall := totalRecall / float64(queriesWithLabels)
	avgNDCG := totalNDCG / float64(queriesWithLabels)

	f1 := 0.0
	if avgPrecision+avgRecall > 0 {
		f1 = 2 * avgPrecision * avgRecall / (avgPrecision + avgRecall)
	}

	return QualityMetrics{
		Precision:     math.Round(avgPrecision*1000) / 1000,
		Recall:        math.Round(avgRecall*1000) / 1000,
		NDCG:          math.Round(avgNDCG*1000) / 1000,
		F1:            math.Round(f1*1000) / 1000,
		TotalQueries:  len(qe.records),
		MarkedResults: totalMarked,
	}
}

// GetRecords returns all query records.
func (qe *QualityEvaluator) GetRecords() []QueryRecord {
	return qe.records
}

// ═══════════════════════════════════════════════════════
// Metric computation functions
// ═══════════════════════════════════════════════════════

// computePrecision calculates precision@K for a query.
// precision = relevant retrieved / total retrieved
func computePrecision(record QueryRecord) float64 {
	if len(record.Results) == 0 {
		return 0
	}

	relevant := 0
	for _, r := range record.Results {
		if rel, ok := record.Relevance[r.ChunkID]; ok && rel {
			relevant++
		}
	}

	return float64(relevant) / float64(len(record.Results))
}

// computeRecall calculates recall@K for a query.
// recall = relevant retrieved / total relevant
func computeRecall(record QueryRecord) float64 {
	totalRelevant := 0
	for _, rel := range record.Relevance {
		if rel {
			totalRelevant++
		}
	}
	if totalRelevant == 0 {
		return 0
	}

	retrieved := 0
	for _, r := range record.Results {
		if rel, ok := record.Relevance[r.ChunkID]; ok && rel {
			retrieved++
		}
	}

	return float64(retrieved) / float64(totalRelevant)
}

// computeNDCG calculates Normalized Discounted Cumulative Gain.
// NDCG@K = DCG@K / IDCG@K
func computeNDCG(record QueryRecord) float64 {
	if len(record.Results) == 0 {
		return 0
	}

	// DCG: discounted cumulative gain
	dcg := 0.0
	for i, r := range record.Results {
		rel := 0.0
		if isRel, ok := record.Relevance[r.ChunkID]; ok && isRel {
			rel = 1.0
		}
		// DCG formula: sum(rel_i / log2(i+2))
		dcg += rel / math.Log2(float64(i+2))
	}

	// IDCG: ideal DCG (all relevant items at top)
	totalRelevant := 0
	for _, rel := range record.Relevance {
		if rel {
			totalRelevant++
		}
	}

	idcg := 0.0
	for i := 0; i < totalRelevant && i < len(record.Results); i++ {
		idcg += 1.0 / math.Log2(float64(i+2))
	}

	if idcg == 0 {
		return 0
	}

	return dcg / idcg
}

// SortByScore sorts results by score descending.
func SortByScore(results []ScoredResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
}
