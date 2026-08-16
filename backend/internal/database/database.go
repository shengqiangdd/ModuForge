package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
	"github.com/moduforge/backend/internal/config"
)

// Init 初始化 SQLite 数据库，执行迁移
func Init(cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", cfg.DatabasePath+"?_journal_mode=WAL&_busy_timeout=5000&_loc=auto")
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

func tableExists(db *sql.DB, name string) bool {
	var cnt int
	_ = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&cnt)
	return cnt > 0
}

func columnExists(db *sql.DB, table, col string) bool {
	// Validate table name to prevent SQL injection (only allow alphanumeric + underscore)
	for _, c := range table {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err == nil {
			if name == col {
				return true
			}
		}
	}
	return false
}

func migrate(db *sql.DB) error {
	// All statements are idempotent — safe to run on every startup.
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id            TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			username      TEXT NOT NULL UNIQUE,
			email         TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			password_changed_at TEXT,
			created_at    TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id          TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			user_id     TEXT NOT NULL REFERENCES users(id),
			name        TEXT NOT NULL,
			module_type TEXT NOT NULL DEFAULT 'universal',
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
			trigger       TEXT NOT NULL DEFAULT 'manual',
			commit_hash   TEXT NOT NULL DEFAULT '',
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

		// Market tables
		`CREATE TABLE IF NOT EXISTS market_modules (
			id TEXT PRIMARY KEY,
			slug TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			author TEXT,
			author_uid TEXT,
			description TEXT,
			version TEXT,
			module_type TEXT,
			tags TEXT,
			stars INTEGER DEFAULT 0,
			downloads INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_market_modules_stars ON market_modules(stars DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_market_modules_created ON market_modules(created_at)`,

		// AI prompts table
		`CREATE TABLE IF NOT EXISTS ai_prompts (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			mode       TEXT NOT NULL,
			user_id    TEXT NOT NULL DEFAULT '',
			content    TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(mode, user_id)
		)`,
		`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('generate', '')`,
		`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('chat', '')`,
		`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('repair', '')`,
		`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('agent', '')`,

		`CREATE TABLE IF NOT EXISTS adb_saved_devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			address TEXT NOT NULL UNIQUE,
			name TEXT DEFAULT '',
			last_connected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// MCP server configurations (managed via UI/API, persisted across restarts)
		// headers is a JSON object string, e.g. {"Authorization":"Bearer xxx"}
		`CREATE TABLE IF NOT EXISTS mcp_servers (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT NOT NULL UNIQUE,
			url        TEXT NOT NULL,
			headers    TEXT NOT NULL DEFAULT '{}',
			enabled    INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// AI Conversations persistence
	`CREATE TABLE IF NOT EXISTS ai_conversations (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		title TEXT DEFAULT '',
		mode TEXT DEFAULT '',
		messages TEXT DEFAULT '[]',
		model TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ai_conversations_user ON ai_conversations(user_id)`,
	// Individual conversation messages for multi-turn persistence
	`CREATE TABLE IF NOT EXISTS conversation_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_conv_msg_session ON conversation_messages(session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_conv_msg_user ON conversation_messages(user_id)`,
}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("migration: %s: %w", m[:60], err)
		}
	}

	// Phase 2: conditional migration — add 'universal' to module_type if not present
	if tableExists(db, "projects") && !columnExists(db, "projects", "description") {
		// Old schema without description column — safe to add
		_, _ = db.Exec(`ALTER TABLE projects ADD COLUMN description TEXT DEFAULT ''`)
	}

	// Phase 3: add user_id column to ai_prompts for per-user prompts
	if tableExists(db, "ai_prompts") && !columnExists(db, "ai_prompts", "user_id") {
		// SQLite can't add UNIQUE constraint after creation, so recreate the table
		_, _ = db.Exec(`
			CREATE TABLE IF NOT EXISTS ai_prompts_new (
				id         INTEGER PRIMARY KEY AUTOINCREMENT,
				mode       TEXT NOT NULL,
				user_id    TEXT NOT NULL DEFAULT '',
				content    TEXT NOT NULL,
				updated_at TEXT NOT NULL DEFAULT (datetime('now')),
				UNIQUE(mode, user_id)
			)
		`)
		_, _ = db.Exec(`INSERT OR IGNORE INTO ai_prompts_new (id, mode, content, updated_at) SELECT id, mode, content, updated_at FROM ai_prompts`)
		_, _ = db.Exec(`DROP TABLE IF EXISTS ai_prompts`)
		_, _ = db.Exec(`ALTER TABLE ai_prompts_new RENAME TO ai_prompts`)
	}
	// Ensure the index exists (idempotent)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_ai_prompts_user ON ai_prompts(user_id)`)

	// Add project_id column to ai_conversations if not exists
	if _, err := db.Exec(`ALTER TABLE ai_conversations ADD COLUMN project_id TEXT DEFAULT ''`); err != nil {
		// Column already exists or other error — log but don't crash
		log.Printf("migration: alter ai_conversations add project_id: %v", err)
	}

	// Add agent_mode column to ai_conversations if not exists
	if _, err := db.Exec(`ALTER TABLE ai_conversations ADD COLUMN agent_mode TEXT DEFAULT 'act'`); err != nil {
		log.Printf("migration: alter ai_conversations add agent_mode: %v", err)
	}

	// Add password_changed_at column to users if not exists
	if tableExists(db, "users") && !columnExists(db, "users", "password_changed_at") {
		_, _ = db.Exec(`ALTER TABLE users ADD COLUMN password_changed_at TEXT`)
	}

	// Phase 4: remove CHECK constraint from projects.module_type to allow dynamic types
	if tableExists(db, "projects") {
		// Verify by trying to insert a test value — if CHECK fails, recreate table
		if _, err := db.Exec(`UPDATE projects SET module_type='performance' WHERE id='__migration_test__'`); err != nil {
			// CHECK constraint still active — recreate table to remove it
			_, _ = db.Exec(`PRAGMA foreign_keys=OFF`)
			_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS projects_new (
				id          TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
				user_id     TEXT NOT NULL REFERENCES users(id),
				name        TEXT NOT NULL,
				module_type TEXT NOT NULL DEFAULT 'universal',
				description TEXT DEFAULT '',
				created_at  TEXT NOT NULL DEFAULT (datetime('now')),
				updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
				deleted_at  TEXT
			)`)
			_, _ = db.Exec(`INSERT INTO projects_new (id,user_id,name,module_type,description,created_at,updated_at,deleted_at) SELECT id,user_id,name,module_type,description,created_at,updated_at,deleted_at FROM projects`)
			_, _ = db.Exec(`DROP TABLE IF EXISTS projects_old`)
			_, _ = db.Exec(`ALTER TABLE projects RENAME TO projects_old`)
			_, _ = db.Exec(`ALTER TABLE projects_new RENAME TO projects`)
			_, _ = db.Exec(`DROP TABLE IF EXISTS projects_old`)
			_, _ = db.Exec(`PRAGMA foreign_keys=ON`)
			log.Printf("migration: recreated projects table to remove CHECK constraint")
		}
		// Clean up the test row if it was inserted
		_, _ = db.Exec(`DELETE FROM projects WHERE id='__migration_test__'`)
	}

	return nil
}
