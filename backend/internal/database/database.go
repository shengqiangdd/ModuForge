package database

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/moduforge/backend/internal/config"
)

// Init 初始化 SQLite 数据库，执行迁移
func Init(cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", cfg.DatabasePath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	// 性能优化
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-8000", // 8MB cache
	} {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("pragma %s: %w", pragma, err)
		}
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id            TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			username      TEXT NOT NULL UNIQUE,
			email         TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at    TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id          TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			user_id     TEXT NOT NULL REFERENCES users(id),
			name        TEXT NOT NULL,
			module_type TEXT NOT NULL CHECK(module_type IN ('magisk','ksu','apatch','hybrid')),
			description TEXT DEFAULT '',
			created_at  TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
			deleted_at  TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS project_files (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id  TEXT NOT NULL REFERENCES projects(id),
			path        TEXT NOT NULL,
			content     TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(project_id, path)
		)`,
		`CREATE TABLE IF NOT EXISTS build_tasks (
			id            TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			project_id    TEXT NOT NULL REFERENCES projects(id),
			status        TEXT NOT NULL DEFAULT 'pending'
			              CHECK(status IN ('pending','running','success','failed','cancelled')),
			target        TEXT NOT NULL,
			log           TEXT DEFAULT '',
			artifact_path TEXT,
			created_at    TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_projects_user ON projects(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_project_files_project ON project_files(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_build_tasks_project ON build_tasks(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_build_tasks_status ON build_tasks(status)`,

		// Wave 2: Collaboration tables
		`CREATE TABLE IF NOT EXISTS collaborators (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT DEFAULT 'editor',
			invited_at DATETIME,
			accepted_at DATETIME,
			FOREIGN KEY (project_id) REFERENCES projects(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS comments (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			user_id TEXT,
			username TEXT,
			file_path TEXT,
			line_number INTEGER,
			content TEXT,
			resolved BOOLEAN DEFAULT 0,
			created_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS edit_sessions (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			user_id TEXT,
			username TEXT,
			file_path TEXT,
			cursor_line INTEGER,
			cursor_col INTEGER,
			selection_start_line INTEGER,
			selection_start_col INTEGER,
			selection_end_line INTEGER,
			selection_end_col INTEGER,
			color TEXT,
			connected_at DATETIME,
			last_active DATETIME
		)`,

		// Wave 2: Plugin tables
		`CREATE TABLE IF NOT EXISTS plugins (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT UNIQUE NOT NULL,
			description TEXT,
			author TEXT,
			version TEXT,
			enabled BOOLEAN DEFAULT 0,
			config TEXT,
			installed_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS plugin_hooks (
			id TEXT PRIMARY KEY,
			plugin_id TEXT NOT NULL,
			hook_name TEXT NOT NULL,
			hook_type TEXT,
			entry_point TEXT,
			config TEXT,
			FOREIGN KEY (plugin_id) REFERENCES plugins(id)
		)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("migration: %s: %w", m[:60], err)
		}
	}
	return nil
}
