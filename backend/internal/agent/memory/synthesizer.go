package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"
)

// Synthesizer handles automatic memory consolidation, deduplication, and promotion.
type Synthesizer struct {
	db           *sql.DB
	vectorSearch *VectorSearch
}

// NewSynthesizer creates a new memory synthesizer.
func NewSynthesizer(db *sql.DB) *Synthesizer {
	return &Synthesizer{
		db:           db,
		vectorSearch: NewVectorSearch(),
	}
}

// SynthesisResult represents the result of a synthesis operation.
type SynthesisResult struct {
	DuplicatesFound int      `json:"_duplicates_found"`
	DuplicatesMerged int     `json:"_duplicates_merged"`
	Contradictions   []string `json:"contradictions"`
	Promoted         int      `json:"promoted"`
	Archived         int      `json:"archived"`
}

// RunSynthesis performs a full synthesis cycle on user memories.
func (s *Synthesizer) RunSynthesis(userID string) (*SynthesisResult, error) {
	result := &SynthesisResult{}

	// Load all user memories into vector index
	memories, err := s.loadMemories(userID)
	if err != nil {
		return nil, fmt.Errorf("load memories: %w", err)
	}

	// Build vector index
	for _, m := range memories {
		s.vectorSearch.AddDocument(m.ID, m.Content, map[string]interface{}{
			"category": m.Category,
			"tier":     m.Tier,
			"importance": m.Importance,
		})
	}

	// 1. Deduplication
	duplicates := s.findDuplicates(memories)
	result.DuplicatesFound = len(duplicates)
	result.DuplicatesMerged = s.mergeDuplicates(duplicates)

	// 2. Contradiction detection
	contradictions := s.findContradictions(memories)
	result.Contradictions = contradictions

	// 3. Auto-promotion (short_term -> long_term)
	promoted, err := s.autoPromote(userID)
	if err != nil {
		log.Printf("[Synthesizer] auto-promote error: %v", err)
	}
	result.Promoted = promoted

	// 4. Archive old memories
	archived, err := s.archiveOldMemories(userID)
	if err != nil {
		log.Printf("[Synthesizer] archive error: %v", err)
	}
	result.Archived = archived

	log.Printf("[Synthesizer] synthesis complete for user %s: %+v", userID, result)
	return result, nil
}

// MemoryEntry represents a memory for synthesis.
type MemoryEntry struct {
	ID         string
	Content    string
	Category   string
	Tier       string
	Importance int
	CreatedAt  time.Time
}

// loadMemories loads all user memories for synthesis.
func (s *Synthesizer) loadMemories(userID string) ([]MemoryEntry, error) {
	rows, err := s.db.Query(`
		SELECT id, content, category, tier, importance, created_at
		FROM memory_v2
		WHERE user_id = ? AND (expires_at IS NULL OR expires_at > datetime('now'))
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []MemoryEntry
	for rows.Next() {
		var m MemoryEntry
		var createdAt string
		if err := rows.Scan(&m.ID, &m.Content, &m.Category, &m.Tier, &m.Importance, &createdAt); err == nil {
			m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
			memories = append(memories, m)
		}
	}
	return memories, nil
}

// findDuplicates finds memory pairs with similarity > 0.8.
func (s *Synthesizer) findDuplicates(memories []MemoryEntry) [][2]string {
	var duplicates [][2]string
	checked := make(map[string]bool)

	for _, m1 := range memories {
		if checked[m1.ID] {
			continue
		}

		results := s.vectorSearch.Search(m1.Content, 5)
		for _, r := range results {
			if r.ID == m1.ID || checked[r.ID] {
				continue
			}
			if r.Score > 0.8 {
				duplicates = append(duplicates, [2]string{m1.ID, r.ID})
				checked[r.ID] = true
			}
		}
		checked[m1.ID] = true
	}

	return duplicates
}

// mergeDuplicates merges duplicate memories (keeps the newer one, deletes the older).
func (s *Synthesizer) mergeDuplicates(duplicates [][2]string) int {
	merged := 0
	for _, pair := range duplicates {
		// Keep the first (newer) one, delete the second
		_, err := s.db.Exec(`DELETE FROM memory_v2 WHERE id = ?`, pair[1])
		if err == nil {
			merged++
			// Update FTS index
			s.db.Exec(`DELETE FROM memory_v2_fts WHERE rowid IN (SELECT rowid FROM memory_v2 WHERE id = ?)`, pair[1])
		}
	}
	return merged
}

// findContradictions detects conflicting memories about the same topic.
func (s *Synthesizer) findContradictions(memories []MemoryEntry) []string {
	var contradictions []string

	// Group by category
	byCategory := make(map[string][]MemoryEntry)
	for _, m := range memories {
		byCategory[m.Category] = append(byCategory[m.Category], m)
	}

	// Check for contradictions within each category
	for _, categoryMemories := range byCategory {
		if len(categoryMemories) < 2 {
			continue
		}

		// Simple contradiction detection: check for negation patterns
		for i := 0; i < len(categoryMemories); i++ {
			for j := i + 1; j < len(categoryMemories); j++ {
				if s.areContradictory(categoryMemories[i].Content, categoryMemories[j].Content) {
					contradictions = append(contradictions, fmt.Sprintf(
						"Contradiction between memories %s and %s: both discuss same topic with conflicting information",
						categoryMemories[i].ID, categoryMemories[j].ID))
				}
			}
		}
	}

	return contradictions
}

// areContradictory checks if two texts contain contradictory information.
func (s *Synthesizer) areContradictory(text1, text2 string) bool {
	lower1 := strings.ToLower(text1)
	lower2 := strings.ToLower(text2)

	// Check for negation patterns
	negations := []string{"not", "never", "no", "don't", "doesn't", "didn't", "won't", "can't", "shouldn't",
		"不", "没", "未", "别", "无", "非"}

	hasNeg1 := false
	hasNeg2 := false
	for _, neg := range negations {
		if strings.Contains(lower1, neg) {
			hasNeg1 = true
		}
		if strings.Contains(lower2, neg) {
			hasNeg2 = true
		}
	}

	// If one has negation and the other doesn't, check for common topics
	if hasNeg1 != hasNeg2 {
		// Check if they discuss similar topics
		sim := cosineSimilarity(
			s.textToSimpleVector(lower1),
			s.textToSimpleVector(lower2))
		if sim > 0.6 {
			return true
		}
	}

	return false
}

// textToSimpleVector creates a simple vector for comparison.
func (s *Synthesizer) textToSimpleVector(text string) []float64 {
	vector := make([]float64, 64)
	tokens := tokenize(text)
	for _, token := range tokens {
		hash := simpleHash(token)
		idx := int(hash % 64)
		vector[idx] += 1.0
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

// autoPromote promotes short-term memories accessed frequently to long-term.
func (s *Synthesizer) autoPromote(userID string) (int, error) {
	result, err := s.db.Exec(`
		UPDATE memory_v2
		SET tier = 'long_term', expires_at = datetime('now', '+90 days')
		WHERE user_id = ?
		AND tier = 'short_term'
		AND (access_count >= 5 OR importance >= 7)
		AND expires_at > datetime('now')
	`, userID)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// archiveOldMemories archives long-term memories not accessed in 30 days.
func (s *Synthesizer) archiveOldMemories(userID string) (int, error) {
	result, err := s.db.Exec(`
		UPDATE memory_v2
		SET tier = 'archive', expires_at = NULL
		WHERE user_id = ?
		AND tier = 'long_term'
		AND (last_accessed IS NULL OR last_accessed < datetime('now', '-30 days'))
		AND importance < 7
	`, userID)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// GetMemoryWeight calculates the current weight of a memory based on decay.
// Weight = importance × e^(-λ × days_since_access)
func GetMemoryWeight(importance int, lastAccessed string, decayRate float64) float64 {
	if decayRate <= 0 {
		decayRate = 0.01 // default decay rate
	}

	if lastAccessed == "" {
		return float64(importance)
	}

	t, err := time.Parse(time.RFC3339, lastAccessed)
	if err != nil {
		return float64(importance)
	}

	daysSinceAccess := time.Since(t).Hours() / 24
	weight := float64(importance) * math.Exp(-decayRate*daysSinceAccess)

	return weight
}

// CleanupDecayed removes memories with weight < 0.1.
func (s *Synthesizer) CleanupDecayed(userID string) (int, error) {
	rows, err := s.db.Query(`
		SELECT id, importance, COALESCE(last_accessed, created_at) as last_access
		FROM memory_v2
		WHERE user_id = ? AND tier != 'archive'
	`, userID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var toArchive []string
	for rows.Next() {
		var id string
		var importance int
		var lastAccess string
		if err := rows.Scan(&id, &importance, &lastAccess); err == nil {
			weight := GetMemoryWeight(importance, lastAccess, 0.01)
			if weight < 0.1 {
				toArchive = append(toArchive, id)
			}
		}
	}

	archived := 0
	for _, id := range toArchive {
		_, err := s.db.Exec(`
			UPDATE memory_v2 SET tier = 'archive', expires_at = NULL
			WHERE id = ?
		`, id)
		if err == nil {
			archived++
		}
	}

	return archived, nil
}

// ExtractLessons extracts reusable lessons from failed attempts.
func ExtractLessons(content string) []string {
	var lessons []string
	lower := strings.ToLower(content)

	// Pattern: "failed because..." or "error:..."
	patterns := []string{
		"failed because",
		"error:",
		"mistake:",
		"lesson:",
		"learned:",
		"don't ",
		"avoid ",
		"should not",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(lower, pattern); idx >= 0 {
			end := strings.Index(content[idx:], "\n")
			if end < 0 {
				end = len(content) - idx
			}
			lesson := strings.TrimSpace(content[idx : idx+end])
			if len(lesson) > 10 && len(lesson) < 200 {
				lessons = append(lessons, lesson)
			}
		}
	}

	return lessons
}

// EnhanceSummary enhances a session summary with lessons and best practices.
func EnhanceSummary(summary string, decisions, files, errors []string) string {
	var enhanced strings.Builder
	enhanced.WriteString(summary)

	if len(errors) > 0 {
		enhanced.WriteString("\n\n## Lessons Learned\n")
		for _, err := range errors {
			lessons := ExtractLessons(err)
			for _, lesson := range lessons {
				enhanced.WriteString(fmt.Sprintf("- %s\n", lesson))
			}
		}
	}

	if len(decisions) > 0 {
		enhanced.WriteString("\n\n## Best Practices\n")
		for _, d := range decisions {
			if strings.Contains(strings.ToLower(d), "chose") || strings.Contains(strings.ToLower(d), "approach") {
				enhanced.WriteString(fmt.Sprintf("- %s\n", d))
			}
		}
	}

	return enhanced.String()
}

// StoreEnhancedSummary stores an enhanced session summary.
func StoreEnhancedSummary(db *sql.DB, userID, sessionID, projectID, summary string, decisions, files, errors []string) error {
	enhanced := EnhanceSummary(summary, decisions, files, errors)

	// Store the enhanced summary
	decJSON, _ := json.Marshal(decisions)
	filesJSON, _ := json.Marshal(files)

	_, err := db.Exec(`
		INSERT INTO session_summaries (user_id, session_id, project_id, summary, key_decisions, files_changed)
		VALUES (?, ?, ?, ?, ?, ?)
	`, userID, sessionID, projectID, enhanced, string(decJSON), string(filesJSON))

	return err
}
