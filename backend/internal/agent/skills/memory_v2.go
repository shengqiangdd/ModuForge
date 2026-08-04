package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/agent/registry"
)

// MemoryV2Skill enhances memory with semantic search, tiered storage, and synthesis
type MemoryV2Skill struct {
	db *sql.DB
}

func init() {
	registry.RegisterFactory("memory_v2", func(deps *registry.Deps) registry.Skill {
		return &MemoryV2Skill{db: deps.DB}
	})
}

func (s *MemoryV2Skill) Name() string {
	return "memory_v2"
}

func (s *MemoryV2Skill) Description() string {
	return `Enhanced memory system with semantic search, tiered storage, and synthesis. Input: {"action": "store|recall|search|consolidate|stats|forget", "content": "...", "category": "episodic|semantic|procedural", "importance": 1-10, "tags": [...], "query": "..."}`
}

type MemoryEntry struct {
	ID           string   `json:"id"`
	UserID       string   `json:"user_id"`
	SessionID    string   `json:"session_id"`
	Content      string   `json:"content"`
	Category     string   `json:"category"` // episodic, semantic, procedural
	Tier         string   `json:"tier"`     // short_term, long_term, archive
	Importance   int      `json:"importance"`
	Tags         []string `json:"tags"`
	AccessCount  int      `json:"access_count"`
	LastAccessed string   `json:"last_accessed"`
	CreatedAt    string   `json:"created_at"`
	ExpiresAt    string   `json:"expires_at,omitempty"`
}

func (s *MemoryV2Skill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	action, _ := input["action"].(string)
	userID, _ := input["user_id"].(string)
	sessionID, _ := input["session_id"].(string)

	switch action {
	case "store":
		return s.storeMemory(userID, sessionID, input)
	case "recall":
		return s.recallMemory(userID, sessionID, input)
	case "search":
		return s.searchMemory(userID, input)
	case "consolidate":
		return s.consolidateMemory(userID, input)
	case "stats":
		return s.getStats(userID)
	case "forget":
		return s.forgetMemory(input)
	case "promote":
		return s.promoteMemory(input)
	case "demote":
		return s.demoteMemory(input)
	default:
		return "", fmt.Errorf("unknown action: %s (use store|recall|search|consolidate|stats|forget|promote|demote)", action)
	}
}

func (s *MemoryV2Skill) ensureTables() error {
	// Main memory table
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS memory_v2 (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			session_id TEXT,
			content TEXT NOT NULL,
			category TEXT DEFAULT 'episodic',
			tier TEXT DEFAULT 'short_term',
			importance INTEGER DEFAULT 5,
			tags TEXT DEFAULT '[]',
			access_count INTEGER DEFAULT 0,
			last_accessed DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME
		)
	`)
	if err != nil {
		return err
	}

	// FTS5 index for full-text search
	_, err = s.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS memory_v2_fts USING fts5(
			content,
			tags,
			content=memory_v2,
			content_rowid=rowid
		)
	`)
	if err != nil {
		return err
	}

	// Create indexes for common queries
	_, err = s.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_memory_v2_user ON memory_v2(user_id)
	`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_memory_v2_category ON memory_v2(category)
	`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_memory_v2_tier ON memory_v2(tier)
	`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_memory_v2_importance ON memory_v2(importance DESC)
	`)
	if err != nil {
		return err
	}

	return nil
}

func (s *MemoryV2Skill) storeMemory(userID, sessionID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	content, _ := input["content"].(string)
	category, _ := input["category"].(string)
	importance := 5
	if imp, ok := input["importance"].(float64); ok {
		importance = int(imp)
	}
	tags, _ := input["tags"].([]interface{})
	tier, _ := input["tier"].(string)

	if content == "" {
		return "", fmt.Errorf("content is required")
	}
	if category == "" {
		category = "episodic"
	}
	if tier == "" {
		tier = "short_term"
	}

	// Generate ID
	entryID := fmt.Sprintf("mem_%s_%d", userID, time.Now().UnixMilli())

	// Convert tags to JSON
	tagsJSON := "[]"
	if tags != nil {
		b, _ := json.Marshal(tags)
		tagsJSON = string(b)
	}

	// Set expiry based on tier
	var expiresAt sql.NullString
	switch tier {
	case "short_term":
		// Short-term: 7 days
		expiresAt = sql.NullString{String: time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339), Valid: true}
	case "long_term":
		// Long-term: 90 days
		expiresAt = sql.NullString{String: time.Now().Add(90 * 24 * time.Hour).Format(time.RFC3339), Valid: true}
	case "archive":
		// Archive: no expiry
		expiresAt = sql.NullString{Valid: false}
	}

	// Insert memory
	_, err := s.db.Exec(`
		INSERT INTO memory_v2 (id, user_id, session_id, content, category, tier, importance, tags, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, entryID, userID, sessionID, content, category, tier, importance, tagsJSON, expiresAt)
	if err != nil {
		return "", fmt.Errorf("store memory: %w", err)
	}

	// Update FTS index
	_, err = s.db.Exec(`
		INSERT INTO memory_v2_fts (rowid, content, tags)
		SELECT rowid, content, tags FROM memory_v2 WHERE id = ?
	`, entryID)
	if err != nil {
		return "", fmt.Errorf("update FTS index: %w", err)
	}

	result := map[string]interface{}{
		"action":     "store",
		"success":    true,
		"entry_id":   entryID,
		"category":   category,
		"tier":       tier,
		"importance": importance,
		"message":    fmt.Sprintf("Memory stored: %s (%s)", category, tier),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *MemoryV2Skill) recallMemory(userID, sessionID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	category, _ := input["category"].(string)
	tier, _ := input["tier"].(string)
	limit := 10
	if l, ok := input["limit"].(float64); ok {
		limit = int(l)
	}

	// Build query
	var conditions []string
	var args []interface{}

	conditions = append(conditions, "(user_id = ? OR user_id IS NULL)")
	args = append(args, userID)

	if sessionID != "" {
		conditions = append(conditions, "(session_id = ? OR session_id IS NULL)")
		args = append(args, sessionID)
	}

	if category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, category)
	}

	if tier != "" {
		conditions = append(conditions, "tier = ?")
		args = append(args, tier)
	}

	// Filter expired memories
	conditions = append(conditions, "(expires_at IS NULL OR expires_at > datetime('now'))")

	whereClause := strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT id, user_id, session_id, content, category, tier, importance, tags, access_count, last_accessed, created_at
		FROM memory_v2
		WHERE %s
		ORDER BY importance DESC, last_accessed DESC NULLS LAST, created_at DESC
		LIMIT ?
	`, whereClause)
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return "", fmt.Errorf("recall memory: %w", err)
	}
	defer rows.Close()

	var memories []MemoryEntry
	for rows.Next() {
		var m MemoryEntry
		var tagsJSON string
		var lastAccessed sql.NullString
		if err := rows.Scan(&m.ID, &m.UserID, &m.SessionID, &m.Content, &m.Category, &m.Tier, &m.Importance, &tagsJSON, &m.AccessCount, &lastAccessed, &m.CreatedAt); err == nil {
			json.Unmarshal([]byte(tagsJSON), &m.Tags)
			if lastAccessed.Valid {
				m.LastAccessed = lastAccessed.String
			}
			memories = append(memories, m)

			// Update access count and last accessed
			s.db.Exec(`
				UPDATE memory_v2 SET access_count = access_count + 1, last_accessed = datetime('now')
				WHERE id = ?
			`, m.ID)
		}
	}

	result := map[string]interface{}{
		"action":   "recall",
		"success":  true,
		"memories": memories,
		"count":    len(memories),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *MemoryV2Skill) searchMemory(userID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	query, _ := input["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	limit := 10
	if l, ok := input["limit"].(float64); ok {
		limit = int(l)
	}

	// Use FTS5 for full-text search
	rows, err := s.db.Query(`
		SELECT m.id, m.user_id, m.session_id, m.content, m.category, m.tier, m.importance, m.tags, m.access_count, m.last_accessed, m.created_at
		FROM memory_v2 m
		JOIN memory_v2_fts fts ON m.rowid = fts.rowid
		WHERE memory_v2_fts MATCH ? AND m.user_id = ?
		ORDER BY rank
		LIMIT ?
	`, query, userID, limit)
	if err != nil {
		// Fallback to LIKE search if FTS fails
		rows, err = s.db.Query(`
			SELECT id, user_id, session_id, content, category, tier, importance, tags, access_count, last_accessed, created_at
			FROM memory_v2
			WHERE user_id = ? AND (content LIKE ? OR tags LIKE ?)
			ORDER BY importance DESC, created_at DESC
			LIMIT ?
		`, userID, "%"+query+"%", "%"+query+"%", limit)
		if err != nil {
			return "", fmt.Errorf("search memory: %w", err)
		}
	}
	defer rows.Close()

	var memories []MemoryEntry
	for rows.Next() {
		var m MemoryEntry
		var tagsJSON string
		var lastAccessed sql.NullString
		if err := rows.Scan(&m.ID, &m.UserID, &m.SessionID, &m.Content, &m.Category, &m.Tier, &m.Importance, &tagsJSON, &m.AccessCount, &lastAccessed, &m.CreatedAt); err == nil {
			json.Unmarshal([]byte(tagsJSON), &m.Tags)
			if lastAccessed.Valid {
				m.LastAccessed = lastAccessed.String
			}
			memories = append(memories, m)
		}
	}

	result := map[string]interface{}{
		"action":  "search",
		"success": true,
		"query":   query,
		"results": memories,
		"count":   len(memories),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *MemoryV2Skill) consolidateMemory(userID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	// Consolidate short-term memories to long-term
	// Move memories accessed more than 3 times or with importance >= 7 to long_term
	_, err := s.db.Exec(`
		UPDATE memory_v2
		SET tier = 'long_term', expires_at = datetime('now', '+90 days')
		WHERE user_id = ?
		AND tier = 'short_term'
		AND (access_count >= 3 OR importance >= 7)
		AND expires_at > datetime('now')
	`, userID)
	if err != nil {
		return "", fmt.Errorf("consolidate: %w", err)
	}

	// Archive old long-term memories that haven't been accessed in 30 days
	_, err = s.db.Exec(`
		UPDATE memory_v2
		SET tier = 'archive', expires_at = NULL
		WHERE user_id = ?
		AND tier = 'long_term'
		AND (last_accessed IS NULL OR last_accessed < datetime('now', '-30 days'))
	`, userID)
	if err != nil {
		return "", fmt.Errorf("consolidate archive: %w", err)
	}

	// Delete expired short-term memories
	_, err = s.db.Exec(`
		DELETE FROM memory_v2
		WHERE user_id = ?
		AND tier = 'short_term'
		AND expires_at IS NOT NULL
		AND expires_at < datetime('now')
	`, userID)
	if err != nil {
		return "", fmt.Errorf("cleanup expired: %w", err)
	}

	// Get statistics
	var shortTerm, longTerm, archive int
	s.db.QueryRow("SELECT COUNT(*) FROM memory_v2 WHERE user_id = ? AND tier = 'short_term'", userID).Scan(&shortTerm)
	s.db.QueryRow("SELECT COUNT(*) FROM memory_v2 WHERE user_id = ? AND tier = 'long_term'", userID).Scan(&longTerm)
	s.db.QueryRow("SELECT COUNT(*) FROM memory_v2 WHERE user_id = ? AND tier = 'archive'", userID).Scan(&archive)

	result := map[string]interface{}{
		"action":     "consolidate",
		"success":    true,
		"short_term": shortTerm,
		"long_term":  longTerm,
		"archive":    archive,
		"message":    fmt.Sprintf("Consolidated: %d short-term, %d long-term, %d archive", shortTerm, longTerm, archive),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *MemoryV2Skill) getStats(userID string) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	var total, shortTerm, longTerm, archive int
	var totalSize int
	s.db.QueryRow("SELECT COUNT(*) FROM memory_v2 WHERE user_id = ?", userID).Scan(&total)
	s.db.QueryRow("SELECT COUNT(*) FROM memory_v2 WHERE user_id = ? AND tier = 'short_term'", userID).Scan(&shortTerm)
	s.db.QueryRow("SELECT COUNT(*) FROM memory_v2 WHERE user_id = ? AND tier = 'long_term'", userID).Scan(&longTerm)
	s.db.QueryRow("SELECT COUNT(*) FROM memory_v2 WHERE user_id = ? AND tier = 'archive'", userID).Scan(&archive)
	s.db.QueryRow("SELECT COALESCE(SUM(LENGTH(content)), 0) FROM memory_v2 WHERE user_id = ?", userID).Scan(&totalSize)

	// Get category breakdown
	var episodic, semantic, procedural int
	s.db.QueryRow("SELECT COUNT(*) FROM memory_v2 WHERE user_id = ? AND category = 'episodic'", userID).Scan(&episodic)
	s.db.QueryRow("SELECT COUNT(*) FROM memory_v2 WHERE user_id = ? AND category = 'semantic'", userID).Scan(&semantic)
	s.db.QueryRow("SELECT COUNT(*) FROM memory_v2 WHERE user_id = ? AND category = 'procedural'", userID).Scan(&procedural)

	// Get most accessed memories
	rows, err := s.db.Query(`
		SELECT content, access_count, importance
		FROM memory_v2
		WHERE user_id = ?
		ORDER BY access_count DESC
		LIMIT 5
	`, userID)
	if err != nil {
		return "", fmt.Errorf("get stats: %w", err)
	}
	defer rows.Close()

	var topMemories []map[string]interface{}
	for rows.Next() {
		var content string
		var accessCount, importance int
		if err := rows.Scan(&content, &accessCount, &importance); err == nil {
			// Truncate content
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			topMemories = append(topMemories, map[string]interface{}{
				"content":      content,
				"access_count": accessCount,
				"importance":   importance,
			})
		}
	}

	result := map[string]interface{}{
		"action":     "stats",
		"success":    true,
		"total":      total,
		"short_term": shortTerm,
		"long_term":  longTerm,
		"archive":    archive,
		"total_size": totalSize,
		"by_category": map[string]int{
			"episodic":   episodic,
			"semantic":   semantic,
			"procedural": procedural,
		},
		"top_memories": topMemories,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *MemoryV2Skill) forgetMemory(input map[string]interface{}) (string, error) {
	entryID, _ := input["entry_id"].(string)
	if entryID == "" {
		return "", fmt.Errorf("entry_id is required")
	}

	// Delete from FTS first
	_, err := s.db.Exec(`
		DELETE FROM memory_v2_fts WHERE rowid IN (
			SELECT rowid FROM memory_v2 WHERE id = ?
		)
	`, entryID)
	if err != nil {
		return "", fmt.Errorf("delete from FTS: %w", err)
	}

	// Delete from main table
	_, err = s.db.Exec("DELETE FROM memory_v2 WHERE id = ?", entryID)
	if err != nil {
		return "", fmt.Errorf("delete memory: %w", err)
	}

	result := map[string]interface{}{
		"action":   "forget",
		"success":  true,
		"entry_id": entryID,
		"message":  "Memory forgotten",
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *MemoryV2Skill) promoteMemory(input map[string]interface{}) (string, error) {
	entryID, _ := input["entry_id"].(string)
	if entryID == "" {
		return "", fmt.Errorf("entry_id is required")
	}

	// Promote from short_term to long_term
	_, err := s.db.Exec(`
		UPDATE memory_v2
		SET tier = 'long_term', expires_at = datetime('now', '+90 days')
		WHERE id = ? AND tier = 'short_term'
	`, entryID)
	if err != nil {
		return "", fmt.Errorf("promote memory: %w", err)
	}

	result := map[string]interface{}{
		"action":   "promote",
		"success":  true,
		"entry_id": entryID,
		"message":  "Memory promoted to long_term",
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *MemoryV2Skill) demoteMemory(input map[string]interface{}) (string, error) {
	entryID, _ := input["entry_id"].(string)
	if entryID == "" {
		return "", fmt.Errorf("entry_id is required")
	}

	// Demote from long_term to short_term
	_, err := s.db.Exec(`
		UPDATE memory_v2
		SET tier = 'short_term', expires_at = datetime('now', '+7 days')
		WHERE id = ? AND tier = 'long_term'
	`, entryID)
	if err != nil {
		return "", fmt.Errorf("demote memory: %w", err)
	}

	result := map[string]interface{}{
		"action":   "demote",
		"success":  true,
		"entry_id": entryID,
		"message":  "Memory demoted to short_term",
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *MemoryV2Skill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
