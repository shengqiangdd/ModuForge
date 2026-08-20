package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type SessionSummary struct {
	ID           int64    `json:"id"`
	UserID       string   `json:"user_id"`
	SessionID    string   `json:"session_id"`
	ProjectID    string   `json:"project_id"`
	Summary      string   `json:"summary"`
	KeyDecisions []string `json:"key_decisions"`
	FilesChanged []string `json:"files_changed"`
	CreatedAt    string   `json:"created_at"`
}

type KnowledgeEntry struct {
	ID        int64  `json:"id"`
	UserID    string `json:"user_id"`
	ProjectID string `json:"project_id"`
	Category  string `json:"category"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt string `json:"updated_at"`
}

type MemoryV2Store struct {
	db *sql.DB
}

func NewMemoryV2Store(db *sql.DB) *MemoryV2Store {
	s := &MemoryV2Store{db: db}
	s.ensureTables()
	return s
}

func (s *MemoryV2Store) ensureTables() {
	s.db.Exec(`CREATE TABLE IF NOT EXISTS session_summaries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		project_id TEXT DEFAULT '',
		summary TEXT NOT NULL,
		key_decisions TEXT DEFAULT '[]',
		files_changed TEXT DEFAULT '[]',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_ss_user ON session_summaries(user_id)`)
	s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_ss_project ON session_summaries(project_id)`)
	s.db.Exec(`CREATE TABLE IF NOT EXISTS project_knowledge (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		project_id TEXT NOT NULL,
		category TEXT NOT NULL,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, project_id, category, key)
	)`)
	s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_pk_proj ON project_knowledge(user_id, project_id)`)
}

func (s *MemoryV2Store) SaveSummary(userID, sessionID, projectID, summary string, decisions, files []string) error {
	decJSON, _ := json.Marshal(decisions)
	filesJSON, _ := json.Marshal(files)
	_, err := s.db.Exec(`INSERT INTO session_summaries (user_id, session_id, project_id, summary, key_decisions, files_changed) VALUES (?, ?, ?, ?, ?, ?)`, userID, sessionID, projectID, summary, string(decJSON), string(filesJSON))
	if err != nil {
		return fmt.Errorf("save session summary: %w", err)
	}
	return nil
}

func (s *MemoryV2Store) GetProjectSummaries(userID, projectID string, limit int) ([]SessionSummary, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`SELECT id, user_id, session_id, project_id, summary, key_decisions, files_changed, created_at FROM session_summaries WHERE user_id=? AND project_id=? ORDER BY created_at DESC LIMIT ?`, userID, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []SessionSummary
	for rows.Next() {
		var ss SessionSummary
		var decJSON, filesJSON string
		if err := rows.Scan(&ss.ID, &ss.UserID, &ss.SessionID, &ss.ProjectID, &ss.Summary, &decJSON, &filesJSON, &ss.CreatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(decJSON), &ss.KeyDecisions)
		json.Unmarshal([]byte(filesJSON), &ss.FilesChanged)
		results = append(results, ss)
	}
	return results, nil
}

func (s *MemoryV2Store) GetRecentUserSummaries(userID string, limit int) ([]SessionSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT id, user_id, session_id, project_id, summary, key_decisions, files_changed, created_at FROM session_summaries WHERE user_id=? ORDER BY created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []SessionSummary
	for rows.Next() {
		var ss SessionSummary
		var decJSON, filesJSON string
		if err := rows.Scan(&ss.ID, &ss.UserID, &ss.SessionID, &ss.ProjectID, &ss.Summary, &decJSON, &filesJSON, &ss.CreatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(decJSON), &ss.KeyDecisions)
		json.Unmarshal([]byte(filesJSON), &ss.FilesChanged)
		results = append(results, ss)
	}
	return results, nil
}

func (s *MemoryV2Store) SaveKnowledge(userID, projectID, category, key, value string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := s.db.Exec(`INSERT INTO project_knowledge (user_id, project_id, category, key, value, updated_at) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(user_id, project_id, category, key) DO UPDATE SET value=?, updated_at=?`, userID, projectID, category, key, value, now, value, now)
	if err != nil {
		return fmt.Errorf("save knowledge: %w", err)
	}
	return nil
}

func (s *MemoryV2Store) GetKnowledge(userID, projectID, category, key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM project_knowledge WHERE user_id=? AND project_id=? AND category=? AND key=?`, userID, projectID, category, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *MemoryV2Store) ListKnowledge(userID, projectID, category string) ([]KnowledgeEntry, error) {
	var rows *sql.Rows
	var err error
	if category != "" {
		rows, err = s.db.Query(`SELECT id, user_id, project_id, category, key, value, updated_at FROM project_knowledge WHERE user_id=? AND project_id=? AND category=? ORDER BY updated_at DESC`, userID, projectID, category)
	} else {
		rows, err = s.db.Query(`SELECT id, user_id, project_id, category, key, value, updated_at FROM project_knowledge WHERE user_id=? AND project_id=? ORDER BY category, updated_at DESC`, userID, projectID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []KnowledgeEntry
	for rows.Next() {
		var e KnowledgeEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.ProjectID, &e.Category, &e.Key, &e.Value, &e.UpdatedAt); err != nil {
			continue
		}
		results = append(results, e)
	}
	return results, nil
}

func (s *MemoryV2Store) DeleteKnowledge(userID, projectID, category, key string) error {
	_, err := s.db.Exec(`DELETE FROM project_knowledge WHERE user_id=? AND project_id=? AND category=? AND key=?`, userID, projectID, category, key)
	if err != nil {
		return fmt.Errorf("delete knowledge: %w", err)
	}
	return nil
}

func (s *MemoryV2Store) DeleteProjectKnowledge(userID, projectID string) error {
	_, err := s.db.Exec(`DELETE FROM project_knowledge WHERE user_id=? AND project_id=?`, userID, projectID)
	if err != nil {
		return fmt.Errorf("delete project knowledge: %w", err)
	}
	return nil
}

func (s *MemoryV2Store) LoadProjectContextForAgent(userID, projectID string) string {
	var sb strings.Builder
	entries, err := s.ListKnowledge(userID, projectID, "")
	if err == nil && len(entries) > 0 {
		sb.WriteString("\n[Project Knowledge from memory]\n")
		currentCat := ""
		catLabels := map[string]string{"architecture": "架构决策", "decision": "技术决策", "issue": "已知问题", "file_purpose": "文件用途", "tech_stack": "技术栈", "requirement": "需求记录"}
		for _, e := range entries {
			if e.Category != currentCat {
				currentCat = e.Category
				label := catLabels[e.Category]
				if label == "" {
					label = e.Category
				}
				sb.WriteString(fmt.Sprintf("\n### %s\n", label))
			}
			sb.WriteString(fmt.Sprintf("- %s: %s\n", e.Key, e.Value))
		}
	}
	summaries, err := s.GetProjectSummaries(userID, projectID, 5)
	if err == nil && len(summaries) > 0 {
		sb.WriteString("\n[Recent Session History]\n")
		for _, ss := range summaries {
			if len(ss.CreatedAt) >= 10 {
				sb.WriteString(fmt.Sprintf("\n%s:\n%s\n", ss.CreatedAt[:10], ss.Summary))
			}
			if len(ss.KeyDecisions) > 0 {
				sb.WriteString("  Decisions: " + strings.Join(ss.KeyDecisions, "; ") + "\n")
			}
		}
	}
	return sb.String()
}
