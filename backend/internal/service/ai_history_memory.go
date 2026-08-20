package service

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type MemoryEntry struct {
	ID         int64     `json:"id"`
	UserID     string    `json:"user_id"`
	MemoryType string    `json:"memory_type"`
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type MemoryStore struct {
	db *sql.DB
}

func NewMemoryStore(db *sql.DB) *MemoryStore {
	s := &MemoryStore{db: db}
	s.ensureTable()
	return s
}

func (ms *MemoryStore) ensureTable() {
	ms.db.Exec(`CREATE TABLE IF NOT EXISTS agent_memory (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		memory_type TEXT NOT NULL,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	ms.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_memory_unique ON agent_memory(user_id, memory_type, key)`)
}

func (ms *MemoryStore) SaveMemory(userID, memoryType, key, value string) error {
	now := time.Now()
	_, err := ms.db.Exec(
		`INSERT INTO agent_memory (user_id, memory_type, key, value, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, memory_type, key) DO UPDATE SET value = ?, updated_at = ?`,
		userID, memoryType, key, value, now, now, value, now,
	)
	return err
}

func (ms *MemoryStore) GetMemory(userID, memoryType, key string) (string, error) {
	var value string
	err := ms.db.QueryRow(
		"SELECT value FROM agent_memory WHERE user_id = ? AND memory_type = ? AND key = ?",
		userID, memoryType, key,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (ms *MemoryStore) ListMemory(userID, memoryType string) ([]MemoryEntry, error) {
	rows, err := ms.db.Query(
		"SELECT id, user_id, memory_type, key, value, created_at, updated_at FROM agent_memory WHERE user_id = ? AND memory_type = ? ORDER BY updated_at DESC",
		userID, memoryType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.MemoryType, &e.Key, &e.Value, &e.CreatedAt, &e.UpdatedAt); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (ms *MemoryStore) ListAllMemory(userID string) ([]MemoryEntry, error) {
	rows, err := ms.db.Query(
		"SELECT id, user_id, memory_type, key, value, created_at, updated_at FROM agent_memory WHERE user_id = ? ORDER BY memory_type, updated_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.MemoryType, &e.Key, &e.Value, &e.CreatedAt, &e.UpdatedAt); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (ms *MemoryStore) DeleteMemory(userID, memoryType, key string) error {
	_, err := ms.db.Exec(
		"DELETE FROM agent_memory WHERE user_id = ? AND memory_type = ? AND key = ?",
		userID, memoryType, key,
	)
	return err
}

func (ms *MemoryStore) DeleteAllMemory(userID string) error {
	_, err := ms.db.Exec("DELETE FROM agent_memory WHERE user_id = ?", userID)
	return err
}

func (ms *MemoryStore) LoadUserPreferences(userID string) string {
	var sb strings.Builder

	// Load user preferences
	entries, err := ms.ListMemory(userID, "user_preference")
	if err == nil && len(entries) > 0 {
		sb.WriteString("\n[User Preferences from memory]\n")
		for _, e := range entries {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", e.Key, e.Value))
		}
	}

	// Load project context
	projEntries, err := ms.ListMemory(userID, "project_context")
	if err == nil && len(projEntries) > 0 {
		sb.WriteString("\n[Project Context from memory]\n")
		for _, e := range projEntries {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", e.Key, e.Value))
		}
	}

	return sb.String()
}
