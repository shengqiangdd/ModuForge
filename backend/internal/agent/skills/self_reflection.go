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

// SelfReflectionSkill enables the agent to diagnose failures, extract lessons, and adapt strategy.
// Inspired by Reflexion (Shinn et al.) and OpenCode's self-critique patterns.
type SelfReflectionSkill struct {
	db *sql.DB
}

func init() {
	registry.RegisterFactory("self_reflection", func(deps *registry.Deps) registry.Skill {
		return &SelfReflectionSkill{db: deps.DB}
	})
}

func (s *SelfReflectionSkill) Name() string {
	return "self_reflection"
}

func (s *SelfReflectionSkill) Description() string {
	return `Self-reflection and failure diagnosis. Input: {"action": "diagnose|adapt|history|pattern|reset", "task_id": "...", "error": "...", "attempt": N, "context": "...", "lesson": "..."}`
}

type ReflectionEntry struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	Error     string `json:"error"`
	Attempt   int    `json:"attempt"`
	Context   string `json:"context"`
	Lesson    string `json:"lesson"`
	Strategy  string `json:"strategy"`
	Resolved  bool   `json:"resolved"`
	CreatedAt string `json:"created_at"`
}

func (s *SelfReflectionSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	action, _ := input["action"].(string)

	switch action {
	case "diagnose":
		return s.diagnose(input)
	case "adapt":
		return s.adapt(input)
	case "history":
		return s.history(input)
	case "pattern":
		return s.detectPatterns(input)
	case "reset":
		return s.reset(input)
	case "lessons":
		return s.getLessons(input)
	default:
		return "", fmt.Errorf("unknown action: %s (use diagnose|adapt|history|pattern|reset|lessons)", action)
	}
}

func (s *SelfReflectionSkill) ensureTable() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS self_reflections (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			error TEXT NOT NULL,
			attempt INTEGER NOT NULL,
			context TEXT DEFAULT '',
			lesson TEXT DEFAULT '',
			strategy TEXT DEFAULT '',
			resolved INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_reflections_task ON self_reflections(task_id)
	`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_reflections_error ON self_reflections(error)
	`)
	if err != nil {
		return err
	}

	return nil
}

// diagnose records a failure and provides initial diagnosis
func (s *SelfReflectionSkill) diagnose(input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	taskID, _ := input["task_id"].(string)
	errMsg, _ := input["error"].(string)
	attempt := 1
	if a, ok := input["attempt"].(float64); ok {
		attempt = int(a)
	}
	context, _ := input["context"].(string)

	if errMsg == "" {
		return "", fmt.Errorf("error is required")
	}
	if taskID == "" {
		taskID = "general"
	}

	id := fmt.Sprintf("refl_%s_%d", taskID, time.Now().UnixMilli())

	// Check for similar past errors
	var similarLessons []string
	rows, err := s.db.Query(`
		SELECT lesson, strategy FROM self_reflections
		WHERE error LIKE ? AND lesson != ''
		ORDER BY created_at DESC LIMIT 5
	`, "%"+truncateError(errMsg, 50)+"%")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var lesson, strategy string
			if err := rows.Scan(&lesson, &strategy); err == nil && lesson != "" {
				similarLessons = append(similarLessons, fmt.Sprintf("- %s → %s", lesson, strategy))
			}
		}
	}

	// Insert reflection
	_, err = s.db.Exec(`
		INSERT INTO self_reflections (id, task_id, error, attempt, context)
		VALUES (?, ?, ?, ?, ?)
	`, id, taskID, errMsg, attempt, context)
	if err != nil {
		return "", fmt.Errorf("diagnose: %w", err)
	}

	// Build diagnosis
	var diagnosis []string
	diagnosis = append(diagnosis, fmt.Sprintf("Attempt %d failed: %s", attempt, errMsg))

	if attempt >= 3 {
		diagnosis = append(diagnosis, "⚠️ 3+ consecutive failures. Consider changing approach entirely.")
	}

	if len(similarLessons) > 0 {
		diagnosis = append(diagnosis, "\n📚 Similar past errors and solutions:")
		diagnosis = append(diagnosis, strings.Join(similarLessons, "\n"))
		diagnosis = append(diagnosis, "\n💡 Try applying one of these solutions.")
	}

	// Suggest next steps
	diagnosis = append(diagnosis, "\n🔧 Recommended actions:")
	diagnosis = append(diagnosis, "1. Use self_reflection({action: 'adapt', task_id: '"+taskID+"', error: '"+errMsg+"'}) to get adapted strategy")
	diagnosis = append(diagnosis, "2. Use self_reflection({action: 'pattern', task_id: '"+taskID+"'}) to check for repeated patterns")
	diagnosis = append(diagnosis, "3. After resolving, record: self_reflection({action: 'adapt', task_id: '"+taskID+"', lesson: 'what worked', strategy: 'the fix'})")

	result := map[string]interface{}{
		"action":  "diagnose",
		"success": true,
		"id":      id,
		"attempt": attempt,
		"diagnosis": strings.Join(diagnosis, "\n"),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

// adapt provides adaptive strategy based on failure patterns
func (s *SelfReflectionSkill) adapt(input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	taskID, _ := input["task_id"].(string)
	errMsg, _ := input["error"].(string)
	lesson, _ := input["lesson"].(string)
	strategy, _ := input["strategy"].(string)

	if taskID == "" {
		taskID = "general"
	}

	// If lesson/strategy provided, record the resolution
	if lesson != "" || strategy != "" {
		// Find the most recent unresolved reflection for this task
		var id string
		err := s.db.QueryRow(`
			SELECT id FROM self_reflections
			WHERE task_id = ? AND resolved = 0
			ORDER BY created_at DESC LIMIT 1
		`, taskID).Scan(&id)
		if err == nil && id != "" {
			s.db.Exec(`
				UPDATE self_reflections SET lesson = ?, strategy = ?, resolved = 1
				WHERE id = ?
			`, lesson, strategy, id)
		} else {
			// Create a new resolved entry
			newID := fmt.Sprintf("refl_%s_%d", taskID, time.Now().UnixMilli())
			s.db.Exec(`
				INSERT INTO self_reflections (id, task_id, error, attempt, lesson, strategy, resolved)
				VALUES (?, ?, ?, 0, ?, ?, 1)
			`, newID, taskID, "resolved", lesson, strategy)
		}
	}

	// Get recent failures for this task
	rows, err := s.db.Query(`
		SELECT error, attempt, context, lesson, strategy, resolved
		FROM self_reflections
		WHERE task_id = ?
		ORDER BY created_at DESC
		LIMIT 10
	`, taskID)
	if err != nil {
		return "", fmt.Errorf("adapt: %w", err)
	}
	defer rows.Close()

	var failures []map[string]interface{}
	var unresolvedCount int
	var resolvedCount int
	for rows.Next() {
		var e, c, l, s string
		var a int
		var r int
		if err := rows.Scan(&e, &a, &c, &l, &s, &r); err == nil {
			entry := map[string]interface{}{
				"error":    e,
				"attempt":  a,
				"context":  c,
				"lesson":   l,
				"strategy": s,
				"resolved": r == 1,
			}
			failures = append(failures, entry)
			if r == 1 {
				resolvedCount++
			} else {
				unresolvedCount++
			}
		}
	}

	// Build adaptive strategy
	var advice []string

	if unresolvedCount > 0 && resolvedCount > 0 {
		advice = append(advice, "✅ Some failures were resolved. Apply the successful strategies to current issues.")
	}

	if unresolvedCount >= 3 {
		advice = append(advice, "🔄 3+ unresolved failures. Consider: different approach, ask for help, or break task into smaller pieces.")
	}

	// Check error patterns
	if errMsg != "" {
		if strings.Contains(errMsg, "permission") || strings.Contains(errMsg, "Permission") {
			advice = append(advice, "🔒 Permission error detected. Try: check file ownership, use sudo, or verify Docker user mapping.")
		} else if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "No such file") {
			advice = append(advice, "📁 Not-found error. Verify path: use glob_search to find the file first.")
		} else if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline") {
			advice = append(advice, "⏱️ Timeout. Consider: simplifying the operation, splitting into smaller steps, or increasing timeout.")
		} else if strings.Contains(errMsg, "syntax") || strings.Contains(errMsg, "unexpected") {
			advice = append(advice, "🔧 Syntax error. Use read_file to check current content before editing.")
		} else if strings.Contains(errMsg, "already exists") {
			advice = append(advice, "♻️ Already exists. Check if you need to update instead of create.")
		}
	}

	result := map[string]interface{}{
		"action":          "adapt",
		"success":         true,
		"task_id":         taskID,
		"failures":        failures,
		"unresolved":      unresolvedCount,
		"resolved":        resolvedCount,
		"advice":          advice,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

// history shows reflection history for a task
func (s *SelfReflectionSkill) history(input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	taskID, _ := input["task_id"].(string)
	if taskID == "" {
		taskID = "general"
	}

	rows, err := s.db.Query(`
		SELECT id, error, attempt, context, lesson, strategy, resolved, created_at
		FROM self_reflections
		WHERE task_id = ?
		ORDER BY created_at DESC
		LIMIT 20
	`, taskID)
	if err != nil {
		return "", fmt.Errorf("history: %w", err)
	}
	defer rows.Close()

	var entries []ReflectionEntry
	for rows.Next() {
		var e ReflectionEntry
		var resolved int
		if err := rows.Scan(&e.ID, &e.Error, &e.Attempt, &e.Context, &e.Lesson, &e.Strategy, &resolved, &e.CreatedAt); err == nil {
			e.Resolved = resolved == 1
			entries = append(entries, e)
		}
	}

	result := map[string]interface{}{
		"action":    "history",
		"success":   true,
		"task_id":   taskID,
		"entries":   entries,
		"count":     len(entries),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

// detectPatterns finds repeated error patterns across tasks
func (s *SelfReflectionSkill) detectPatterns(input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	taskID, _ := input["task_id"].(string)

	// Find error patterns
	query := `
		SELECT error, COUNT(*) as cnt, MAX(attempt) as max_attempt, 
		       SUM(CASE WHEN resolved = 1 THEN 1 ELSE 0 END) as resolved_cnt
		FROM self_reflections
	`
	var args []interface{}
	if taskID != "" {
		query += " WHERE task_id = ?"
		args = append(args, taskID)
	}
	query += " GROUP BY error HAVING cnt > 1 ORDER BY cnt DESC LIMIT 10"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return "", fmt.Errorf("detect patterns: %w", err)
	}
	defer rows.Close()

	var patterns []map[string]interface{}
	for rows.Next() {
		var error string
		var cnt, maxAttempt, resolvedCnt int
		if err := rows.Scan(&error, &cnt, &maxAttempt, &resolvedCnt); err == nil {
			patterns = append(patterns, map[string]interface{}{
				"error":         error,
				"occurrences":   cnt,
				"max_attempt":   maxAttempt,
				"times_resolved": resolvedCnt,
			})
		}
	}

	// Overall stats
	var total, resolved, unresolved int
	if taskID != "" {
		s.db.QueryRow("SELECT COUNT(*), SUM(CASE WHEN resolved=1 THEN 1 ELSE 0 END) FROM self_reflections WHERE task_id=?", taskID).Scan(&total, &resolved)
	} else {
		s.db.QueryRow("SELECT COUNT(*), SUM(CASE WHEN resolved=1 THEN 1 ELSE 0 END) FROM self_reflections").Scan(&total, &resolved)
	}
	unresolved = total - resolved

	result := map[string]interface{}{
		"action":    "pattern",
		"success":   true,
		"task_id":   taskID,
		"patterns":  patterns,
		"stats": map[string]interface{}{
			"total":      total,
			"resolved":   resolved,
			"unresolved": unresolved,
		},
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

// reset clears reflections for a task
func (s *SelfReflectionSkill) reset(input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	taskID, _ := input["task_id"].(string)
	if taskID == "" {
		return "", fmt.Errorf("task_id is required")
	}

	result, err := s.db.Exec("DELETE FROM self_reflections WHERE task_id = ?", taskID)
	if err != nil {
		return "", fmt.Errorf("reset: %w", err)
	}

	affected, _ := result.RowsAffected()

	resp := map[string]interface{}{
		"action":   "reset",
		"success":  true,
		"task_id":  taskID,
		"deleted":  affected,
	}

	b, _ := json.MarshalIndent(resp, "", "  ")
	return string(b), nil
}

// getLessons returns all resolved lessons for reuse
func (s *SelfReflectionSkill) getLessons(input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	taskID, _ := input["task_id"].(string)

	query := `
		SELECT task_id, error, lesson, strategy, created_at
		FROM self_reflections
		WHERE resolved = 1 AND lesson != ''
	`
	var args []interface{}
	if taskID != "" {
		query += " AND task_id = ?"
		args = append(args, taskID)
	}
	query += " ORDER BY created_at DESC LIMIT 20"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return "", fmt.Errorf("lessons: %w", err)
	}
	defer rows.Close()

	var lessons []map[string]interface{}
	for rows.Next() {
		var tID, error, lesson, strategy, createdAt string
		if err := rows.Scan(&tID, &error, &lesson, &strategy, &createdAt); err == nil {
			lessons = append(lessons, map[string]interface{}{
				"task_id":    tID,
				"error":      error,
				"lesson":     lesson,
				"strategy":   strategy,
				"created_at": createdAt,
			})
		}
	}

	result := map[string]interface{}{
		"action":  "lessons",
		"success": true,
		"lessons": lessons,
		"count":   len(lessons),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func truncateError(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

func (s *SelfReflectionSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}