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

// ContextManagerSkill manages conversation context and memory
type ContextManagerSkill struct {
	db *sql.DB
}

func init() {
	registry.RegisterFactory("context_manager", func(deps *registry.Deps) registry.Skill {
		return &ContextManagerSkill{db: deps.DB}
	})
}

func (s *ContextManagerSkill) Name() string {
	return "context_manager"
}

func (s *ContextManagerSkill) Description() string {
	return `Manage conversation context and memory. Input: {"action": "compress|search|remember|forget|summary|stats", "query": "...", "content": "...", "category": "project|user|global"}`
}

type ContextEntry struct {
	ID         string `json:"id"`
	SessionID  string `json:"session_id"`
	Category   string `json:"category"` // project, user, global
	Key        string `json:"key"`
	Value      string `json:"value"`
	Importance int    `json:"importance"` // 1-10
	CreatedAt  string `json:"created_at"`
	AccessedAt string `json:"accessed_at"`
}

func (s *ContextManagerSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	action, _ := input["action"].(string)
	sessionID, _ := input["session_id"].(string)

	switch action {
	case "remember":
		return s.remember(sessionID, input)
	case "search":
		return s.search(input)
	case "forget":
		return s.forget(input)
	case "summary":
		return s.getSummary(sessionID)
	case "stats":
		return s.getStats(sessionID)
	case "compress":
		return s.compressContext(sessionID, input)
	case "list":
		return s.listEntries(sessionID, input)
	// New actions for project state tracking
	case "track_file":
		return s.trackFile(sessionID, input)
	case "track_progress":
		return s.trackProgress(sessionID, input)
	case "get_project_state":
		return s.getProjectState(sessionID, input)
	case "update_task_status":
		return s.updateTaskStatus(sessionID, input)
	default:
		return "", fmt.Errorf("unknown action: %s (use remember|search|forget|summary|stats|compress|list|track_file|track_progress|get_project_state|update_task_status)", action)
	}
}

func (s *ContextManagerSkill) ensureTable() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS context_entries (
			id TEXT PRIMARY KEY,
			session_id TEXT,
			category TEXT DEFAULT 'global',
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			importance INTEGER DEFAULT 5,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			accessed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Create index for faster searches
	_, err = s.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_context_session ON context_entries(session_id)
	`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_context_category ON context_entries(category)
	`)
	if err != nil {
		return err
	}

	return nil
}

func (s *ContextManagerSkill) remember(sessionID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	key, _ := input["key"].(string)
	value, _ := input["value"].(string)
	category, _ := input["category"].(string)
	importance := 5
	if imp, ok := input["importance"].(float64); ok {
		importance = int(imp)
	}

	if key == "" || value == "" {
		return "", fmt.Errorf("key and value are required")
	}
	if category == "" {
		category = "global"
	}

	// Generate ID
	entryID := fmt.Sprintf("ctx_%s_%s", sessionID, key)

	// Upsert: update if exists, insert if not
	_, err := s.db.Exec(`
		INSERT INTO context_entries (id, session_id, category, key, value, importance)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			value = excluded.value,
			importance = excluded.importance,
			accessed_at = CURRENT_TIMESTAMP
	`, entryID, sessionID, category, key, value, importance)
	if err != nil {
		return "", fmt.Errorf("remember: %w", err)
	}

	result := map[string]interface{}{
		"action":     "remember",
		"success":    true,
		"entry_id":   entryID,
		"key":        key,
		"category":   category,
		"importance": importance,
		"message":    fmt.Sprintf("Remembered: %s", key),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *ContextManagerSkill) search(input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	query, _ := input["query"].(string)
	category, _ := input["category"].(string)
	sessionID, _ := input["session_id"].(string)
	limit := 10
	if l, ok := input["limit"].(float64); ok {
		limit = int(l)
	}

	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	// Build search query
	var conditions []string
	var args []interface{}

	if query != "" {
		// Use LIKE for simple text search (could be enhanced with FTS5)
		conditions = append(conditions, "(key LIKE ? OR value LIKE ?)")
		args = append(args, "%"+query+"%", "%"+query+"%")
	}

	if category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, category)
	}

	if sessionID != "" {
		conditions = append(conditions, "(session_id = ? OR session_id IS NULL)")
		args = append(args, sessionID)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	querySQL := fmt.Sprintf(`
		SELECT id, session_id, category, key, value, importance, created_at, accessed_at
		FROM context_entries %s
		ORDER BY importance DESC, accessed_at DESC
		LIMIT ?
	`, whereClause)
	args = append(args, limit)

	rows, err := s.db.Query(querySQL, args...)
	if err != nil {
		return "", fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	var entries []ContextEntry
	for rows.Next() {
		var e ContextEntry
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Category, &e.Key, &e.Value, &e.Importance, &e.CreatedAt, &e.AccessedAt); err == nil {
			// Update access time
			s.db.Exec("UPDATE context_entries SET accessed_at = CURRENT_TIMESTAMP WHERE id = ?", e.ID)
			entries = append(entries, e)
		}
	}

	result := map[string]interface{}{
		"action":  "search",
		"success": true,
		"query":   query,
		"results": entries,
		"count":   len(entries),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *ContextManagerSkill) forget(input map[string]interface{}) (string, error) {
	entryID, _ := input["entry_id"].(string)
	key, _ := input["key"].(string)
	sessionID, _ := input["session_id"].(string)

	if entryID != "" {
		_, err := s.db.Exec("DELETE FROM context_entries WHERE id = ?", entryID)
		if err != nil {
			return "", fmt.Errorf("forget: %w", err)
		}
	} else if key != "" && sessionID != "" {
		_, err := s.db.Exec("DELETE FROM context_entries WHERE key = ? AND session_id = ?", key, sessionID)
		if err != nil {
			return "", fmt.Errorf("forget: %w", err)
		}
	} else {
		return "", fmt.Errorf("entry_id or (key + session_id) is required")
	}

	result := map[string]interface{}{
		"action":  "forget",
		"success": true,
		"message": "Entry forgotten",
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *ContextManagerSkill) getSummary(sessionID string) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	var totalEntries, projectCount, userCount, globalCount, highCount, mediumCount, lowCount int

	s.db.QueryRow("SELECT COUNT(*) FROM context_entries WHERE session_id = ? OR session_id IS NULL", sessionID).Scan(&totalEntries)
	s.db.QueryRow("SELECT COUNT(*) FROM context_entries WHERE category = 'project' AND (session_id = ? OR session_id IS NULL)", sessionID).Scan(&projectCount)
	s.db.QueryRow("SELECT COUNT(*) FROM context_entries WHERE category = 'user' AND (session_id = ? OR session_id IS NULL)", sessionID).Scan(&userCount)
	s.db.QueryRow("SELECT COUNT(*) FROM context_entries WHERE category = 'global' AND (session_id = ? OR session_id IS NULL)", sessionID).Scan(&globalCount)
	s.db.QueryRow("SELECT COUNT(*) FROM context_entries WHERE importance >= 7 AND (session_id = ? OR session_id IS NULL)", sessionID).Scan(&highCount)
	s.db.QueryRow("SELECT COUNT(*) FROM context_entries WHERE importance >= 4 AND importance < 7 AND (session_id = ? OR session_id IS NULL)", sessionID).Scan(&mediumCount)
	s.db.QueryRow("SELECT COUNT(*) FROM context_entries WHERE importance < 4 AND (session_id = ? OR session_id IS NULL)", sessionID).Scan(&lowCount)

	// Get recent important entries
	rows, err := s.db.Query(`
		SELECT key, value, importance, category
		FROM context_entries
		WHERE (session_id = ? OR session_id IS NULL) AND importance >= 7
		ORDER BY accessed_at DESC
		LIMIT 5
	`, sessionID)
	if err != nil {
		return "", fmt.Errorf("summary: %w", err)
	}
	defer rows.Close()

	var importantEntries []map[string]interface{}
	for rows.Next() {
		var key, value, category string
		var importance int
		if err := rows.Scan(&key, &value, &importance, &category); err == nil {
			// Truncate long values
			if len(value) > 100 {
				value = value[:100] + "..."
			}
			importantEntries = append(importantEntries, map[string]interface{}{
				"key":        key,
				"value":      value,
				"importance": importance,
				"category":   category,
			})
		}
	}

	result := map[string]interface{}{
		"action":        "summary",
		"success":       true,
		"total_entries": totalEntries,
		"by_category": map[string]int{
			"project": projectCount,
			"user":    userCount,
			"global":  globalCount,
		},
		"by_importance": map[string]int{
			"high":   highCount,
			"medium": mediumCount,
			"low":    lowCount,
		},
		"recent_important": importantEntries,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *ContextManagerSkill) getStats(sessionID string) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	var totalEntries int
	var totalSize int
	s.db.QueryRow("SELECT COUNT(*) FROM context_entries WHERE session_id = ? OR session_id IS NULL", sessionID).Scan(&totalEntries)
	s.db.QueryRow("SELECT COALESCE(SUM(LENGTH(value)), 0) FROM context_entries WHERE session_id = ? OR session_id IS NULL", sessionID).Scan(&totalSize)

	// Get oldest and newest
	var oldest, newest string
	s.db.QueryRow("SELECT MIN(created_at) FROM context_entries WHERE session_id = ? OR session_id IS NULL", sessionID).Scan(&oldest)
	s.db.QueryRow("SELECT MAX(created_at) FROM context_entries WHERE session_id = ? OR session_id IS NULL", sessionID).Scan(&newest)

	result := map[string]interface{}{
		"action":     "stats",
		"success":    true,
		"total":      totalEntries,
		"total_size": totalSize,
		"avg_size":   0,
		"oldest":     oldest,
		"newest":     newest,
	}
	if totalEntries > 0 {
		result["avg_size"] = totalSize / totalEntries
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *ContextManagerSkill) compressContext(sessionID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	maxEntries := 100
	if m, ok := input["max_entries"].(float64); ok {
		maxEntries = int(m)
	}

	// Get current count
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM context_entries WHERE session_id = ?", sessionID).Scan(&count)

	if count <= maxEntries {
		result := map[string]interface{}{
			"action":  "compress",
			"success": true,
			"message": fmt.Sprintf("No compression needed (%d entries, limit %d)", count, maxEntries),
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return string(b), nil
	}

	// Delete low-importance old entries
	_, err := s.db.Exec(`
		DELETE FROM context_entries
		WHERE session_id = ? AND importance < 5
		AND id NOT IN (
			SELECT id FROM context_entries
			WHERE session_id = ? AND importance < 5
			ORDER BY accessed_at DESC
			LIMIT ?
		)
	`, sessionID, sessionID, maxEntries/2)
	if err != nil {
		return "", fmt.Errorf("compress: %w", err)
	}

	// Get new count
	var newCount int
	s.db.QueryRow("SELECT COUNT(*) FROM context_entries WHERE session_id = ?", sessionID).Scan(&newCount)

	result := map[string]interface{}{
		"action":  "compress",
		"success": true,
		"before":  count,
		"after":   newCount,
		"removed": count - newCount,
		"message": fmt.Sprintf("Compressed from %d to %d entries", count, newCount),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *ContextManagerSkill) listEntries(sessionID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	category, _ := input["category"].(string)
	limit := 20
	if l, ok := input["limit"].(float64); ok {
		limit = int(l)
	}

	var conditions []string
	var args []interface{}

	if sessionID != "" {
		conditions = append(conditions, "(session_id = ? OR session_id IS NULL)")
		args = append(args, sessionID)
	}
	if category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, category)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, session_id, category, key, value, importance, created_at
		FROM context_entries %s
		ORDER BY importance DESC, created_at DESC
		LIMIT ?
	`, whereClause)
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return "", fmt.Errorf("list: %w", err)
	}
	defer rows.Close()

	var entries []ContextEntry
	for rows.Next() {
		var e ContextEntry
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Category, &e.Key, &e.Value, &e.Importance, &e.CreatedAt); err == nil {
			entries = append(entries, e)
		}
	}

	result := map[string]interface{}{
		"action":  "list",
		"success": true,
		"entries": entries,
		"count":   len(entries),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

// New functions for project state tracking

func (s *ContextManagerSkill) trackFile(sessionID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	filePath, _ := input["path"].(string)
	action, _ := input["action"].(string) // created, modified, deleted
	projectID, _ := input["project_id"].(string)

	if filePath == "" || action == "" {
		return "", fmt.Errorf("path and action are required")
	}

	// Store file tracking entry
	key := fmt.Sprintf("file:%s:%s", projectID, filePath)
	value := fmt.Sprintf(`{"path":"%s","action":"%s","timestamp":"%s"}`, filePath, action, time.Now().Format(time.RFC3339))

	_, err := s.db.Exec(`
		INSERT INTO context_entries (id, session_id, category, key, value, importance)
		VALUES (?, ?, 'project', ?, ?, 7)
		ON CONFLICT(id) DO UPDATE SET
			value = excluded.value,
			accessed_at = CURRENT_TIMESTAMP
	`, fmt.Sprintf("file_%s_%s", sessionID, filePath), sessionID, key, value)
	if err != nil {
		return "", fmt.Errorf("track_file: %w", err)
	}

	result := map[string]interface{}{
		"action":    "track_file",
		"success":   true,
		"path":      filePath,
		"file_action": action,
		"project_id": projectID,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *ContextManagerSkill) trackProgress(sessionID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	taskID, _ := input["task_id"].(string)
	status, _ := input["status"].(string) // pending, in_progress, completed, failed
	progress, _ := input["progress"].(float64)
	projectID, _ := input["project_id"].(string)

	if taskID == "" || status == "" {
		return "", fmt.Errorf("task_id and status are required")
	}

	// Store progress entry
	key := fmt.Sprintf("progress:%s:%s", projectID, taskID)
	value := fmt.Sprintf(`{"task_id":"%s","status":"%s","progress":%.1f,"timestamp":"%s"}`, 
		taskID, status, progress, time.Now().Format(time.RFC3339))

	_, err := s.db.Exec(`
		INSERT INTO context_entries (id, session_id, category, key, value, importance)
		VALUES (?, ?, 'project', ?, ?, 8)
		ON CONFLICT(id) DO UPDATE SET
			value = excluded.value,
			accessed_at = CURRENT_TIMESTAMP
	`, fmt.Sprintf("progress_%s_%s", sessionID, taskID), sessionID, key, value)
	if err != nil {
		return "", fmt.Errorf("track_progress: %w", err)
	}

	result := map[string]interface{}{
		"action":    "track_progress",
		"success":   true,
		"task_id":   taskID,
		"status":    status,
		"progress":  progress,
		"project_id": projectID,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *ContextManagerSkill) getProjectState(sessionID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	projectID, _ := input["project_id"].(string)
	if projectID == "" {
		return "", fmt.Errorf("project_id is required")
	}

	// Get all files for this project
	rows, err := s.db.Query(`
		SELECT key, value, importance, created_at, accessed_at
		FROM context_entries
		WHERE session_id = ? AND key LIKE ? AND category = 'project'
		ORDER BY accessed_at DESC
	`, sessionID, fmt.Sprintf("file:%s:%%", projectID))
	if err != nil {
		return "", fmt.Errorf("get_project_state: %w", err)
	}
	defer rows.Close()

	var files []map[string]interface{}
	for rows.Next() {
		var key, value, createdAt, accessedAt string
		var importance int
		if err := rows.Scan(&key, &value, &importance, &createdAt, &accessedAt); err == nil {
			// Parse the value JSON
			var fileData map[string]interface{}
			if json.Unmarshal([]byte(value), &fileData) == nil {
				files = append(files, fileData)
			}
		}
	}

	// Get progress for this project
	progressRows, err := s.db.Query(`
		SELECT key, value, importance, created_at, accessed_at
		FROM context_entries
		WHERE session_id = ? AND key LIKE ? AND category = 'project'
		ORDER BY accessed_at DESC
	`, sessionID, fmt.Sprintf("progress:%s:%%", projectID))
	if err != nil {
		return "", fmt.Errorf("get_project_state: %w", err)
	}
	defer progressRows.Close()

	var progress []map[string]interface{}
	for progressRows.Next() {
		var key, value, createdAt, accessedAt string
		var importance int
		if err := progressRows.Scan(&key, &value, &importance, &createdAt, &accessedAt); err == nil {
			// Parse the value JSON
			var progressData map[string]interface{}
			if json.Unmarshal([]byte(value), &progressData) == nil {
				progress = append(progress, progressData)
			}
		}
	}

	result := map[string]interface{}{
		"action":     "get_project_state",
		"success":    true,
		"project_id": projectID,
		"files":      files,
		"progress":   progress,
		"file_count": len(files),
		"task_count": len(progress),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *ContextManagerSkill) updateTaskStatus(sessionID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	taskID, _ := input["task_id"].(string)
	status, _ := input["status"].(string)
	result_, _ := input["result"].(string)
	projectID, _ := input["project_id"].(string)

	if taskID == "" || status == "" {
		return "", fmt.Errorf("task_id and status are required")
	}

	// Update or create task status entry
	key := fmt.Sprintf("task_status:%s:%s", projectID, taskID)
	value := fmt.Sprintf(`{"task_id":"%s","status":"%s","result":"%s","timestamp":"%s"}`, 
		taskID, status, result_, time.Now().Format(time.RFC3339))

	_, err := s.db.Exec(`
		INSERT INTO context_entries (id, session_id, category, key, value, importance)
		VALUES (?, ?, 'project', ?, ?, 9)
		ON CONFLICT(id) DO UPDATE SET
			value = excluded.value,
			accessed_at = CURRENT_TIMESTAMP
	`, fmt.Sprintf("task_status_%s_%s", sessionID, taskID), sessionID, key, value)
	if err != nil {
		return "", fmt.Errorf("update_task_status: %w", err)
	}

	result := map[string]interface{}{
		"action":    "update_task_status",
		"success":   true,
		"task_id":   taskID,
		"status":    status,
		"result":    result_,
		"project_id": projectID,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *ContextManagerSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
