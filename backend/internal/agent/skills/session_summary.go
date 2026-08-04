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

// SessionSummarySkill compresses completed sessions into reusable knowledge summaries.
type SessionSummarySkill struct {
	db *sql.DB
}

func init() {
	registry.RegisterFactory("session_summary", func(deps *registry.Deps) registry.Skill {
		return &SessionSummarySkill{db: deps.DB}
	})
}

func (s *SessionSummarySkill) Name() string {
	return "session_summary"
}

func (s *SessionSummarySkill) Description() string {
	return `Session summarization and compression. Input: {"action": "create|list|get|delete|export", "session_id": "...", "summary": "...", "decisions": [...], "files_changed": [...]}`
}

type SessionSummary struct {
	ID           string   `json:"id"`
	SessionID    string   `json:"session_id"`
	Summary      string   `json:"summary"`
	Decisions    []string `json:"decisions"`
	FilesChanged []string `json:"files_changed"`
	ErrorsFixed  []string `json:"errors_fixed"`
	ToolsUsed    []string `json:"tools_used"`
	TokenSaved   int      `json:"token_saved"`
	CreatedAt    string   `json:"created_at"`
}

func (s *SessionSummarySkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	action, _ := input["action"].(string)

	switch action {
	case "create":
		return s.createSummary(input)
	case "list":
		return s.listSummaries(input)
	case "get":
		return s.getSummary(input)
	case "delete":
		return s.deleteSummary(input)
	case "export":
		return s.exportSummaries(input)
	case "stats":
		return s.getStats()
	default:
		return "", fmt.Errorf("unknown action: %s (use create|list|get|delete|export|stats)", action)
	}
}

func (s *SessionSummarySkill) ensureTable() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS session_summaries (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			summary TEXT NOT NULL,
			decisions TEXT DEFAULT '[]',
			files_changed TEXT DEFAULT '[]',
			errors_fixed TEXT DEFAULT '[]',
			tools_used TEXT DEFAULT '[]',
			token_saved INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_summaries_session ON session_summaries(session_id)
	`)
	if err != nil {
		return err
	}

	return nil
}

func (s *SessionSummarySkill) createSummary(input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	sessionID, _ := input["session_id"].(string)
	summary, _ := input["summary"].(string)
	decisions, _ := input["decisions"].([]interface{})
	filesChanged, _ := input["files_changed"].([]interface{})
	errorsFixed, _ := input["errors_fixed"].([]interface{})
	toolsUsed, _ := input["tools_used"].([]interface{})
	tokenSaved := 0
	if ts, ok := input["token_saved"].(float64); ok {
		tokenSaved = int(ts)
	}

	if summary == "" {
		return "", fmt.Errorf("summary is required")
	}
	if sessionID == "" {
		sessionID = fmt.Sprintf("session_%d", time.Now().UnixMilli())
	}

	id := fmt.Sprintf("sum_%s_%d", sessionID, time.Now().UnixMilli())

	// Convert arrays to JSON
	decisionsJSON := toJSONArray(decisions)
	filesJSON := toJSONArray(filesChanged)
	errorsJSON := toJSONArray(errorsFixed)
	toolsJSON := toJSONArray(toolsUsed)

	_, err := s.db.Exec(`
		INSERT INTO session_summaries (id, session_id, summary, decisions, files_changed, errors_fixed, tools_used, token_saved)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, sessionID, summary, decisionsJSON, filesJSON, errorsJSON, toolsJSON, tokenSaved)
	if err != nil {
		return "", fmt.Errorf("create summary: %w", err)
	}

	result := map[string]interface{}{
		"action":     "create",
		"success":    true,
		"id":         id,
		"session_id": sessionID,
		"message":    "Session summary created",
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SessionSummarySkill) listSummaries(input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	limit := 20
	if l, ok := input["limit"].(float64); ok {
		limit = int(l)
	}

	rows, err := s.db.Query(`
		SELECT id, session_id, summary, decisions, files_changed, errors_fixed, tools_used, token_saved, created_at
		FROM session_summaries
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return "", fmt.Errorf("list summaries: %w", err)
	}
	defer rows.Close()

	var summaries []SessionSummary
	for rows.Next() {
		var sm SessionSummary
		var decisionsJSON, filesJSON, errorsJSON, toolsJSON string
		if err := rows.Scan(&sm.ID, &sm.SessionID, &sm.Summary, &decisionsJSON, &filesJSON, &errorsJSON, &toolsJSON, &sm.TokenSaved, &sm.CreatedAt); err == nil {
			json.Unmarshal([]byte(decisionsJSON), &sm.Decisions)
			json.Unmarshal([]byte(filesJSON), &sm.FilesChanged)
			json.Unmarshal([]byte(errorsJSON), &sm.ErrorsFixed)
			json.Unmarshal([]byte(toolsJSON), &sm.ToolsUsed)
			// Truncate summary for list view
			if len(sm.Summary) > 200 {
				sm.Summary = sm.Summary[:200] + "..."
			}
			summaries = append(summaries, sm)
		}
	}

	result := map[string]interface{}{
		"action":    "list",
		"success":   true,
		"summaries": summaries,
		"count":     len(summaries),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SessionSummarySkill) getSummary(input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	summaryID, _ := input["summary_id"].(string)
	sessionID, _ := input["session_id"].(string)

	var sm SessionSummary
	var decisionsJSON, filesJSON, errorsJSON, toolsJSON string

	if summaryID != "" {
		err := s.db.QueryRow(`
			SELECT id, session_id, summary, decisions, files_changed, errors_fixed, tools_used, token_saved, created_at
			FROM session_summaries WHERE id = ?
		`, summaryID).Scan(&sm.ID, &sm.SessionID, &sm.Summary, &decisionsJSON, &filesJSON, &errorsJSON, &toolsJSON, &sm.TokenSaved, &sm.CreatedAt)
		if err != nil {
			return "", fmt.Errorf("summary not found: %w", err)
		}
	} else if sessionID != "" {
		err := s.db.QueryRow(`
			SELECT id, session_id, summary, decisions, files_changed, errors_fixed, tools_used, token_saved, created_at
			FROM session_summaries WHERE session_id = ?
			ORDER BY created_at DESC LIMIT 1
		`, sessionID).Scan(&sm.ID, &sm.SessionID, &sm.Summary, &decisionsJSON, &filesJSON, &errorsJSON, &toolsJSON, &sm.TokenSaved, &sm.CreatedAt)
		if err != nil {
			return "", fmt.Errorf("summary not found: %w", err)
		}
	} else {
		return "", fmt.Errorf("summary_id or session_id is required")
	}

	json.Unmarshal([]byte(decisionsJSON), &sm.Decisions)
	json.Unmarshal([]byte(filesJSON), &sm.FilesChanged)
	json.Unmarshal([]byte(errorsJSON), &sm.ErrorsFixed)
	json.Unmarshal([]byte(toolsJSON), &sm.ToolsUsed)

	result := map[string]interface{}{
		"action":  "get",
		"success": true,
		"summary": sm,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SessionSummarySkill) deleteSummary(input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	summaryID, _ := input["summary_id"].(string)
	if summaryID == "" {
		return "", fmt.Errorf("summary_id is required")
	}

	_, err := s.db.Exec("DELETE FROM session_summaries WHERE id = ?", summaryID)
	if err != nil {
		return "", fmt.Errorf("delete summary: %w", err)
	}

	result := map[string]interface{}{
		"action":  "delete",
		"success": true,
		"id":      summaryID,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SessionSummarySkill) exportSummaries(input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	rows, err := s.db.Query(`
		SELECT session_id, summary, decisions, files_changed, errors_fixed, created_at
		FROM session_summaries
		ORDER BY created_at DESC
		LIMIT 50
	`)
	if err != nil {
		return "", fmt.Errorf("export: %w", err)
	}
	defer rows.Close()

	var lines []string
	lines = append(lines, "# Session Summaries Export")
	lines = append(lines, fmt.Sprintf("Exported: %s\n", time.Now().Format(time.RFC3339)))

	for rows.Next() {
		var sessionID, summary, decisionsJSON, filesJSON, errorsJSON, createdAt string
		if err := rows.Scan(&sessionID, &summary, &decisionsJSON, &filesJSON, &errorsJSON, &createdAt); err == nil {
			lines = append(lines, fmt.Sprintf("## Session: %s (%s)", sessionID, createdAt))
			lines = append(lines, summary)

			var decisions []string
			json.Unmarshal([]byte(decisionsJSON), &decisions)
			if len(decisions) > 0 {
				lines = append(lines, "\n**Decisions:**")
				for _, d := range decisions {
					lines = append(lines, "- "+d)
				}
			}

			var files []string
			json.Unmarshal([]byte(filesJSON), &files)
			if len(files) > 0 {
				lines = append(lines, "\n**Files changed:** "+strings.Join(files, ", "))
			}

			var errors []string
			json.Unmarshal([]byte(errorsJSON), &errors)
			if len(errors) > 0 {
				lines = append(lines, "\n**Errors fixed:**")
				for _, e := range errors {
					lines = append(lines, "- "+e)
				}
			}

			lines = append(lines, "\n---")
		}
	}

	result := map[string]interface{}{
		"action":  "export",
		"success": true,
		"content": strings.Join(lines, "\n"),
		"count":   len(lines),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *SessionSummarySkill) getStats() (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	var totalSummaries, totalTokenSaved int
	s.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(token_saved), 0) FROM session_summaries").Scan(&totalSummaries, &totalTokenSaved)

	var uniqueSessions int
	s.db.QueryRow("SELECT COUNT(DISTINCT session_id) FROM session_summaries").Scan(&uniqueSessions)

	result := map[string]interface{}{
		"action":            "stats",
		"success":           true,
		"total_summaries":   totalSummaries,
		"unique_sessions":   uniqueSessions,
		"total_token_saved": totalTokenSaved,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func toJSONArray(arr []interface{}) string {
	if arr == nil {
		return "[]"
	}
	b, _ := json.Marshal(arr)
	return string(b)
}

func (s *SessionSummarySkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
