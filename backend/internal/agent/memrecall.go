package agent

import (
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/agent/memory"
)

// autoRecallMemory performs a cheap, local semantic recall of the user's
// persisted memories (memory_v2 table) against the current task text, and
// returns a formatted prompt section with the top-k most relevant entries.
//
// Key insight: without this, memories are ONLY available when the LLM
// proactively calls the memory_v2 skill — free/small models rarely do that,
// so cross-session knowledge silently rots. Injecting the top matches into
// the system prompt makes memory useful for every model tier at near-zero
// per-call cost (TF-IDF hashing, no embedding API).
func (r *AgentRunner) autoRecallMemory(cfg RunConfig, taskText string, k int) string {
	if r.db == nil || cfg.UserID == "" || taskText == "" {
		return ""
	}
	if k <= 0 {
		k = 3
	}
	if k > 8 {
		k = 8
	}

	// Load this user's non-archived, non-expired memories.
	rows, err := r.db.Query(`
		SELECT id, content, category, importance, tags, created_at
		FROM memory_v2
		WHERE user_id = ?
		  AND (tier != 'archive' OR tier IS NULL)
		  AND (expires_at IS NULL OR expires_at > datetime('now'))
		LIMIT 200
	`, cfg.UserID)
	if err != nil {
		return ""
	}
	defer rows.Close()

	vs := memory.NewVectorSearch()
	meta := map[string]interface{}{}
	var ids []string
	for rows.Next() {
		var id, content, category string
		var importance int
		var tagsJSON, createdAt string
		if err := rows.Scan(&id, &content, &category, &importance, &tagsJSON, &createdAt); err != nil {
			continue
		}
		if strings.TrimSpace(content) == "" {
			continue
		}
		// Normalize negative importance to 0 so cosine similarity stays usable.
		if importance < 0 {
			importance = 0
		}
		meta["category"] = category
		meta["importance"] = importance
		meta["tags"] = tagsJSON
		meta["created_at"] = createdAt
		vs.AddDocument(id, content, meta)
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return ""
	}

	results := vs.Search(taskText, len(ids))
	if len(results) == 0 {
		return ""
	}

	// Recency-aware re-rank: score * exp(-ageDays/halfLife) * (1 + importance/20).
	// Older memories fade smoothly instead of being ranked purely by keyword
	// similarity, so recent context wins ties while still allowing important
	// old facts (importance >= 7) to surface.
	const halfLifeDays = 30.0
	for i := range results {
		ageDays := 0.0
		if created, ok := results[i].Metadata["created_at"].(string); ok && created != "" {
			if t, err := time.Parse("2006-01-02 15:04:05", created); err == nil {
				ageDays = time.Since(t).Hours() / 24.0
			} else if t, err := time.Parse(time.RFC3339, created); err == nil {
				ageDays = time.Since(t).Hours() / 24.0
			} else if t, err := time.Parse("2006-01-02T15:04:05Z", created); err == nil {
				ageDays = time.Since(t).Hours() / 24.0
			}
		}
		decay := math.Exp(-ageDays / halfLifeDays)
		imp, _ := results[i].Metadata["importance"].(float64)
		results[i].Score = results[i].Score * decay * (1 + imp/20.0)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > k {
		results = results[:k]
	}

	var sb strings.Builder
	sb.WriteString("\n## RELEVANT PAST MEMORIES (auto-recalled)\n")
	sb.WriteString("These are memories from previous sessions that may help the current task:\n")
	for i, r := range results {
		content := strings.TrimSpace(r.Content)
		if len(content) > 400 {
			content = content[:400] + "…"
		}
		cat, _ := r.Metadata["category"].(string)
		imp, _ := r.Metadata["importance"].(float64)
		line := content
		if cat != "" {
			line = "[" + cat + "] " + line
		}
		if imp >= 7 {
			line = "⭐ " + line
		}
		sb.WriteString(line + "\n")
		if i >= k-1 {
			break
		}
	}
	sb.WriteString("\nUse these only if relevant; ignore irrelevant ones.\n")
	return sb.String()
}// autoStoreMemory persists a concise memory entry after a task completes so
// future sessions can recall it. It is intentionally conservative:
//   - only stores when the final answer is non-trivial (>= 40 chars)
//   - caps content length (200 chars) to keep the memory table lean
//   - stores at most one entry per run (idempotency via fixed id derived from
//     user+session+created timestamp is impractical here, so we allow a couple);
//     a simple dedupe window keeps rapid repeats from flooding the table.
//
// This closes the loop: without it, memory only exists when the LLM
// *remembers* to call the memory_v2 skill, which free/small models never do.
func (r *AgentRunner) autoStoreMemory(userID, sessionID, answer string) {
	if r.db == nil || userID == "" || answer == "" {
		return
	}
	answer = strings.TrimSpace(answer)
	if len(answer) < 40 {
		return // too short to be useful
	}
	if len(answer) > 500 {
		answer = answer[:500] + "…"
	}

	// Dedupe: skip if we stored a near-identical entry in the last 24h.
	var recent int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM memory_v2
		WHERE user_id = ? AND content = ? AND created_at > datetime('now', '-1 day')
	`, userID, answer).Scan(&recent)
	if err == nil && recent > 0 {
		return
	}

	id := newMemoryID(userID, answer)
	_, err = r.db.Exec(`
		INSERT INTO memory_v2 (id, user_id, session_id, content, category, tier, importance, tags, created_at, expires_at)
		VALUES (?, ?, ?, ?, 'episodic', 'short_term', 5, '["auto"]', datetime('now'), datetime('now', '+30 days'))
		ON CONFLICT(id) DO NOTHING
	`, id, userID, sessionID, answer)
	if err != nil {
		// Table may not exist if the memory_v2 skill was never invoked.
		if r.ensureMemoryTables() {
			_, err = r.db.Exec(`
				INSERT INTO memory_v2 (id, user_id, session_id, content, category, tier, importance, tags, created_at, expires_at)
				VALUES (?, ?, ?, ?, 'episodic', 'short_term', 5, '["auto"]', datetime('now'), datetime('now', '+30 days'))
				ON CONFLICT(id) DO NOTHING
			`, id, userID, sessionID, answer)
		}
		if err != nil {
			return
		}
	}
	log.Printf("[Memory] auto-stored episodic memory id=%s len=%d", id, len(answer))
}

// ensureMemoryTables creates memory_v2 and its FTS index if missing (mirrors
// the memory_v2 skill's ensureTables so auto-store works without the skill).
func (r *AgentRunner) ensureMemoryTables() bool {
	if r.db == nil {
		return false
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS memory_v2 (
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
		)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_v2_user ON memory_v2(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_v2_category ON memory_v2(category)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_v2_tier ON memory_v2(tier)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_v2_importance ON memory_v2(importance DESC)`,
	}
	ok := true
	for _, s := range stmts {
		if _, err := r.db.Exec(s); err != nil {
			ok = false
		}
	}
	return ok
}

// newMemoryID derives a stable id from user + content (SHA1-like hex).
func newMemoryID(userID, content string) string {
	h := fnv.New64a()
	h.Write([]byte(userID))
	h.Write([]byte(":"))
	h.Write([]byte(content))
	return fmt.Sprintf("auto-%x", h.Sum64())
}