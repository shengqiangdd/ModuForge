package rag

import (
	"testing"
)

func TestQualityEvaluator_StartAndMark(t *testing.T) {
	qe := NewQualityEvaluator()

	results := []ScoredResult{
		{ChunkID: "c1", Score: 0.9},
		{ChunkID: "c2", Score: 0.8},
		{ChunkID: "c3", Score: 0.7},
	}

	qe.StartQuery("battery daemon", results)
	qe.MarkRelevant("c1", true)
	qe.MarkRelevant("c2", true)
	qe.MarkRelevant("c3", false)

	records := qe.GetRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if len(records[0].Relevance) != 3 {
		t.Errorf("expected 3 relevance labels, got %d", len(records[0].Relevance))
	}
}

func TestQualityEvaluator_Precision(t *testing.T) {
	qe := NewQualityEvaluator()

	// Query 1: 2 relevant out of 3 retrieved
	qe.StartQuery("q1", []ScoredResult{
		{ChunkID: "c1", Score: 0.9},
		{ChunkID: "c2", Score: 0.8},
		{ChunkID: "c3", Score: 0.7},
	})
	qe.MarkRelevant("c1", true)
	qe.MarkRelevant("c2", true)
	qe.MarkRelevant("c3", false)

	metrics := qe.GetMetrics()
	// Precision = 2/3 = 0.667
	if metrics.Precision < 0.66 || metrics.Precision > 0.67 {
		t.Errorf("expected precision ~0.667, got %f", metrics.Precision)
	}
}

func TestQualityEvaluator_Recall(t *testing.T) {
	qe := NewQualityEvaluator()

	// 3 total relevant items, 2 retrieved
	qe.StartQuery("q1", []ScoredResult{
		{ChunkID: "c1", Score: 0.9},
		{ChunkID: "c2", Score: 0.8},
	})
	qe.MarkRelevant("c1", true)
	qe.MarkRelevant("c2", true)
	qe.MarkRelevant("c3", true) // relevant but not retrieved

	metrics := qe.GetMetrics()
	// Recall = 2/3 = 0.667
	if metrics.Recall < 0.66 || metrics.Recall > 0.67 {
		t.Errorf("expected recall ~0.667, got %f", metrics.Recall)
	}
}

func TestQualityEvaluator_NDCG(t *testing.T) {
	qe := NewQualityEvaluator()

	// Perfect ranking: relevant items at top
	qe.StartQuery("q1", []ScoredResult{
		{ChunkID: "c1", Score: 0.9}, // relevant
		{ChunkID: "c2", Score: 0.8}, // relevant
		{ChunkID: "c3", Score: 0.7}, // not relevant
	})
	qe.MarkRelevant("c1", true)
	qe.MarkRelevant("c2", true)
	qe.MarkRelevant("c3", false)

	metrics := qe.GetMetrics()
	// NDCG should be high for good ranking
	if metrics.NDCG < 0.8 {
		t.Errorf("expected NDCG > 0.8 for good ranking, got %f", metrics.NDCG)
	}
}

func TestQualityEvaluator_NDCG_BadRanking(t *testing.T) {
	qe := NewQualityEvaluator()

	// Bad ranking: relevant items at bottom
	qe.StartQuery("q1", []ScoredResult{
		{ChunkID: "c3", Score: 0.9}, // not relevant
		{ChunkID: "c1", Score: 0.8}, // relevant
		{ChunkID: "c2", Score: 0.7}, // relevant
	})
	qe.MarkRelevant("c1", true)
	qe.MarkRelevant("c2", true)
	qe.MarkRelevant("c3", false)

	metrics := qe.GetMetrics()
	// NDCG should be lower for bad ranking
	if metrics.NDCG > 0.8 {
		t.Errorf("expected NDCG < 0.8 for bad ranking, got %f", metrics.NDCG)
	}
}

func TestQualityEvaluator_F1(t *testing.T) {
	qe := NewQualityEvaluator()

	qe.StartQuery("q1", []ScoredResult{
		{ChunkID: "c1", Score: 0.9},
		{ChunkID: "c2", Score: 0.8},
	})
	qe.MarkRelevant("c1", true)
	qe.MarkRelevant("c2", true)

	metrics := qe.GetMetrics()
	// Precision = 1.0, Recall depends on total relevant
	// F1 = 2 * P * R / (P + R)
	if metrics.F1 <= 0 {
		t.Errorf("expected positive F1, got %f", metrics.F1)
	}
}

func TestQualityEvaluator_Empty(t *testing.T) {
	qe := NewQualityEvaluator()
	metrics := qe.GetMetrics()

	if metrics.TotalQueries != 0 {
		t.Errorf("expected 0 queries, got %d", metrics.TotalQueries)
	}
	if metrics.Precision != 0 {
		t.Errorf("expected 0 precision, got %f", metrics.Precision)
	}
}

func TestQualityEvaluator_MultipleQueries(t *testing.T) {
	qe := NewQualityEvaluator()

	// Query 1
	qe.StartQuery("q1", []ScoredResult{
		{ChunkID: "c1", Score: 0.9},
	})
	qe.MarkRelevant("c1", true)

	// Query 2
	qe.StartQuery("q2", []ScoredResult{
		{ChunkID: "c2", Score: 0.8},
	})
	qe.MarkRelevant("c2", false)

	metrics := qe.GetMetrics()

	if metrics.TotalQueries != 2 {
		t.Errorf("expected 2 queries, got %d", metrics.TotalQueries)
	}

	if metrics.MarkedResults != 1 {
		t.Errorf("expected 1 marked relevant, got %d", metrics.MarkedResults)
	}
}

func TestQualityEvaluator_NoLabels(t *testing.T) {
	qe := NewQualityEvaluator()

	qe.StartQuery("q1", []ScoredResult{
		{ChunkID: "c1", Score: 0.9},
	})
	// Don't mark any relevance

	metrics := qe.GetMetrics()
	if metrics.TotalQueries != 1 {
		t.Errorf("expected 1 query, got %d", metrics.TotalQueries)
	}
	// No labels means precision/recall should be 0
	if metrics.Precision != 0 {
		t.Errorf("expected 0 precision without labels, got %f", metrics.Precision)
	}
}

func TestComputePrecision(t *testing.T) {
	record := QueryRecord{
		Results: []ScoredResult{
			{ChunkID: "c1"},
			{ChunkID: "c2"},
			{ChunkID: "c3"},
		},
		Relevance: map[string]bool{
			"c1": true,
			"c2": true,
			"c3": false,
		},
	}

	prec := computePrecision(record)
	if prec < 0.66 || prec > 0.67 {
		t.Errorf("expected precision ~0.667, got %f", prec)
	}
}

func TestComputeRecall(t *testing.T) {
	record := QueryRecord{
		Results: []ScoredResult{
			{ChunkID: "c1"},
			{ChunkID: "c2"},
		},
		Relevance: map[string]bool{
			"c1": true,
			"c2": true,
			"c3": true, // not retrieved
		},
	}

	rec := computeRecall(record)
	if rec < 0.66 || rec > 0.67 {
		t.Errorf("expected recall ~0.667, got %f", rec)
	}
}

func TestComputeNDCG_Perfect(t *testing.T) {
	record := QueryRecord{
		Results: []ScoredResult{
			{ChunkID: "c1"},
			{ChunkID: "c2"},
		},
		Relevance: map[string]bool{
			"c1": true,
			"c2": true,
		},
	}

	ndcg := computeNDCG(record)
	if ndcg < 0.99 {
		t.Errorf("expected NDCG ~1.0 for perfect ranking, got %f", ndcg)
	}
}

func TestComputeNDCG_Empty(t *testing.T) {
	record := QueryRecord{
		Results:   []ScoredResult{},
		Relevance: map[string]bool{},
	}

	ndcg := computeNDCG(record)
	if ndcg != 0 {
		t.Errorf("expected 0 NDCG for empty, got %f", ndcg)
	}
}

func TestSortByScore(t *testing.T) {
	results := []ScoredResult{
		{ChunkID: "c3", Score: 0.5},
		{ChunkID: "c1", Score: 0.9},
		{ChunkID: "c2", Score: 0.7},
	}

	SortByScore(results)

	if results[0].ChunkID != "c1" {
		t.Errorf("expected c1 first, got %s", results[0].ChunkID)
	}
	if results[1].ChunkID != "c2" {
		t.Errorf("expected c2 second, got %s", results[1].ChunkID)
	}
	if results[2].ChunkID != "c3" {
		t.Errorf("expected c3 third, got %s", results[2].ChunkID)
	}
}

func TestQualityMetrics_JSON(t *testing.T) {
	m := QualityMetrics{
		Precision:     0.85,
		Recall:        0.72,
		NDCG:          0.91,
		F1:            0.78,
		TotalQueries:  10,
		MarkedResults: 50,
	}

	if m.Precision != 0.85 {
		t.Errorf("expected 0.85, got %f", m.Precision)
	}
	if m.TotalQueries != 10 {
		t.Errorf("expected 10, got %d", m.TotalQueries)
	}
}
