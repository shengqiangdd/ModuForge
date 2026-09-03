package database

import (
	"log"
	"strings"
	"time"
)

// addColumnIfMissing returns ALTER TABLE statements for columns that may not exist in older schemas.
func addColumnIfMissing() []string {
	return []string{
		"ALTER TABLE comments ADD COLUMN resolved INTEGER DEFAULT 0",
		"ALTER TABLE provider_configs ADD COLUMN models_json TEXT",
		"ALTER TABLE market_modules ADD COLUMN changelog TEXT DEFAULT ''",
		"ALTER TABLE market_modules ADD COLUMN parent_id TEXT DEFAULT ''",
		"ALTER TABLE market_modules ADD COLUMN cover_image TEXT DEFAULT ''",
		"ALTER TABLE market_modules ADD COLUMN dependencies TEXT DEFAULT ''",
		"ALTER TABLE market_modules ADD COLUMN author_uid TEXT DEFAULT ''",
		"ALTER TABLE adb_saved_devices ADD COLUMN user_id TEXT DEFAULT ''",
		"ALTER TABLE users ADD COLUMN totp_secret TEXT DEFAULT ''",
		"ALTER TABLE users ADD COLUMN totp_enabled INTEGER DEFAULT 0",
		"ALTER TABLE users ADD COLUMN email_verified INTEGER DEFAULT 0",
		"ALTER TABLE users ADD COLUMN verify_token TEXT DEFAULT ''",
		"ALTER TABLE users ADD COLUMN verify_expires DATETIME",
		"ALTER TABLE users ADD COLUMN avatar_url TEXT DEFAULT ''",
		"ALTER TABLE users ADD COLUMN display_name TEXT DEFAULT ''",
		"ALTER TABLE users ADD COLUMN bio TEXT DEFAULT ''",
		"ALTER TABLE users ADD COLUMN location TEXT DEFAULT ''",
		"ALTER TABLE users ADD COLUMN website TEXT DEFAULT ''",
		"ALTER TABLE users ADD COLUMN github_token TEXT DEFAULT ''",
		// Add project_id to ai_conversations for linking conversations to projects
		"ALTER TABLE ai_conversations ADD COLUMN project_id TEXT DEFAULT ''",
		"ALTER TABLE module_screenshots ADD COLUMN caption TEXT DEFAULT ''",
		// Add file_count and total_size to existing module_versions table
		"ALTER TABLE module_versions ADD COLUMN file_count INTEGER DEFAULT 0",
		"ALTER TABLE module_versions ADD COLUMN total_size INTEGER DEFAULT 0",
		// Add build trigger config columns to projects
		"ALTER TABLE projects ADD COLUMN git_url TEXT DEFAULT ''",
		"ALTER TABLE projects ADD COLUMN git_branch TEXT DEFAULT 'main'",
		"ALTER TABLE projects ADD COLUMN build_cron TEXT DEFAULT ''",
		"ALTER TABLE projects ADD COLUMN auto_build INTEGER DEFAULT 0",
		// Build schedule table
		`CREATE TABLE IF NOT EXISTS build_schedules (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			cron_expr TEXT NOT NULL,
			target TEXT DEFAULT 'universal',
			arch TEXT DEFAULT 'arm64',
			is_active INTEGER DEFAULT 1,
			last_build_at DATETIME,
			next_build_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		)`,
		// Add trigger and commit_hash to existing build_tasks table
		"ALTER TABLE build_tasks ADD COLUMN trigger TEXT DEFAULT 'manual'",
		"ALTER TABLE build_tasks ADD COLUMN commit_hash TEXT DEFAULT ''",
		// MCP policy: three-state permission mode (allow/deny/ask)
		"ALTER TABLE mcp_tool_policies ADD COLUMN mode TEXT NOT NULL DEFAULT 'deny'",
		// Non-agent conversations aggregate LLM token usage (P3 usage stats)
		"ALTER TABLE ai_conversations ADD COLUMN token_usage INTEGER DEFAULT 0",
		// Backfill: old allow_auto=1 policies become mode='allow'
		"UPDATE mcp_tool_policies SET mode='allow' WHERE allow_auto=1 AND mode='deny'",
		// Fix existing NULL values in build_tasks
		"UPDATE build_tasks SET log='' WHERE log IS NULL",
		"UPDATE build_tasks SET target='' WHERE target IS NULL",
		"UPDATE build_tasks SET artifact_path='' WHERE artifact_path IS NULL",
		// LLM config persistence
		`CREATE TABLE IF NOT EXISTS llm_config (
			id          TEXT PRIMARY KEY DEFAULT 'default',
			provider    TEXT DEFAULT '',
			model_id    TEXT DEFAULT '',
			endpoint    TEXT DEFAULT '',
			api_key     TEXT DEFAULT '',
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Daily AI usage aggregation (survives restarts, powers cost/usage trends)
		`CREATE TABLE IF NOT EXISTS ai_usage_daily (
			date TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT '',
			llm_call_count INTEGER DEFAULT 0,
			llm_token_usage INTEGER DEFAULT 0,
			tool_call_count INTEGER DEFAULT 0,
			error_count INTEGER DEFAULT 0,
			retry_count INTEGER DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (date, user_id)
		)`,
	}
}

// migrateAIUsageDaily handles the ai_usage_daily schema migration.
func (db *DB) migrateAIUsageDaily() {
	// Migrate legacy ai_usage_daily (no user_id, single global row per date):
	// rebuild with user_id and carry old rows over as user_id='' (global).
	db.Conn.Exec(`ALTER TABLE ai_usage_daily RENAME TO ai_usage_daily_legacy`)
	if _, err := db.Conn.Exec(`SELECT user_id FROM ai_usage_daily`); err != nil {
		// old schema detected: copy and drop
		db.Conn.Exec(`CREATE TABLE IF NOT EXISTS ai_usage_daily (
			date TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT '',
			llm_call_count INTEGER DEFAULT 0,
			llm_token_usage INTEGER DEFAULT 0,
			tool_call_count INTEGER DEFAULT 0,
			error_count INTEGER DEFAULT 0,
			retry_count INTEGER DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (date, user_id)
		)`)
		db.Conn.Exec(`INSERT OR IGNORE INTO ai_usage_daily (date, user_id, llm_call_count, llm_token_usage, tool_call_count, error_count, retry_count, updated_at)
			SELECT date, '', llm_call_count, llm_token_usage, tool_call_count, error_count, retry_count, updated_at FROM ai_usage_daily_legacy`)
		db.Conn.Exec(`DROP TABLE ai_usage_daily_legacy`)
		log.Println("[DB] migrated ai_usage_daily to user-scoped schema")
	} else {
		// new schema already in place; drop temp table
		db.Conn.Exec(`DROP TABLE IF EXISTS ai_usage_daily_legacy`)
	}
}

// migrateDashboardWidgets fixes dashboard_widgets.user_id type and dedup.
func (db *DB) migrateDashboardWidgets() {
	// Fix dashboard_widgets.user_id type: INTEGER → TEXT (users.id is UUID string)
	var colType string
	db.Conn.QueryRow("SELECT type FROM pragma_table_info('dashboard_widgets', 'user_id')").Scan(&colType)
	if colType == "INTEGER" || colType == "" {
		log.Println("[DB] Fixing dashboard_widgets.user_id type from INTEGER to TEXT")
		db.Conn.Exec(`CREATE TABLE IF NOT EXISTS dashboard_widgets_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			widget_type TEXT NOT NULL,
			title TEXT NOT NULL,
			config TEXT DEFAULT '{}',
			position_x INTEGER DEFAULT 0,
			position_y INTEGER DEFAULT 0,
			width INTEGER DEFAULT 1,
			height INTEGER DEFAULT 1,
			is_visible INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`)
		db.Conn.Exec(`INSERT INTO dashboard_widgets_new SELECT * FROM dashboard_widgets`)
		db.Conn.Exec(`DROP TABLE dashboard_widgets`)
		db.Conn.Exec(`ALTER TABLE dashboard_widgets_new RENAME TO dashboard_widgets`)
		db.Conn.Exec(`CREATE INDEX IF NOT EXISTS idx_dashboard_widgets_user ON dashboard_widgets(user_id)`)
	}

	// Add unique index on (user_id, widget_type) to prevent duplicates
	if _, err := db.Conn.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_dashboard_widgets_unique_type ON dashboard_widgets(user_id, widget_type)`); err != nil {
		log.Printf("[DB] Unique index creation: %v", err)
	} else {
		log.Println("[DB] Dashboard unique index ensured")
	}

	// Clean up existing duplicate widgets (keep lowest id per user_id+widget_type)
	if _, err := db.Conn.Exec(`DELETE FROM dashboard_widgets WHERE id NOT IN (
		SELECT MIN(id) FROM dashboard_widgets GROUP BY user_id, widget_type
	)`); err != nil {
		log.Printf("[DB] Widget dedup cleanup: %v", err)
	} else {
		log.Println("[DB] Widget dedup cleanup done")
	}
}

// startWALCheckpoint starts a goroutine for periodic WAL checkpointing.
func (db *DB) startWALCheckpoint() {
	// Periodic WAL checkpoint keeps the WAL file bounded (agent runs can write
	// bursts of steps/messages). PASSIVE never blocks writers; if checkpointing
	// would contend it simply skips and the next tick retries.
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if _, err := db.Conn.Exec(`PRAGMA wal_checkpoint(PASSIVE)`); err != nil {
				log.Printf("[DB] WAL checkpoint: %v", err)
			}
		}
	}()
	log.Println("[DB] WAL checkpoint scheduler started (15m interval)")
}

// migrateProjectFilesS3 adds S3 metadata columns to project_files and removes the content column.
func (db *DB) migrateProjectFilesS3() {
	// Check if columns exist using PRAGMA
	rows, err := db.Conn.Query("PRAGMA table_info(project_files)")
	if err != nil {
		log.Printf("migrateProjectFilesS3: PRAGMA failed: %v", err)
		return
	}
	defer rows.Close()

	hasSHA256 := false
	hasSize := false
	hasMTime := false
	hasContent := false

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt *string
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
			switch name {
			case "sha256":
				hasSHA256 = true
			case "file_size":
				hasSize = true
			case "mtime":
				hasMTime = true
			case "content":
				hasContent = true
			}
		}
	}

	// Add S3 metadata columns if missing
	if !hasSHA256 {
		db.Conn.Exec("ALTER TABLE project_files ADD COLUMN sha256 TEXT DEFAULT ''")
	}
	if !hasSize {
		db.Conn.Exec("ALTER TABLE project_files ADD COLUMN file_size INTEGER DEFAULT 0")
	}
	if !hasMTime {
		db.Conn.Exec("ALTER TABLE project_files ADD COLUMN mtime TEXT DEFAULT ''")
	}

	// Drop the content column if it exists (S3 is now the sole source of truth)
	if hasContent {
		log.Println("[DB] Dropping content column from project_files (S3 is now sole source of truth)")
		tx, err := db.Conn.Begin()
		if err != nil {
			log.Printf("migrateProjectFilesS3: begin tx failed: %v", err)
			return
		}

		// Create new table without content column
		if _, err := tx.Exec(`CREATE TABLE project_files_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id TEXT NOT NULL,
			path TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			sha256 TEXT DEFAULT '',
			file_size INTEGER DEFAULT 0,
			mtime TEXT DEFAULT '',
			FOREIGN KEY (project_id) REFERENCES projects(id),
			UNIQUE(project_id, path)
		)`); err != nil {
			tx.Rollback()
			log.Printf("migrateProjectFilesS3: create new table failed: %v", err)
			return
		}

		// Copy data (excluding content column)
		if _, err := tx.Exec(`INSERT INTO project_files_new (id, project_id, path, created_at, updated_at, sha256, file_size, mtime)
			SELECT id, project_id, path, created_at, updated_at, sha256, file_size, mtime FROM project_files`); err != nil {
			tx.Rollback()
			log.Printf("migrateProjectFilesS3: copy data failed: %v", err)
			return
		}

		// Drop old table and rename new
		if _, err := tx.Exec(`DROP TABLE project_files`); err != nil {
			tx.Rollback()
			log.Printf("migrateProjectFilesS3: drop old table failed: %v", err)
			return
		}
		if _, err := tx.Exec(`ALTER TABLE project_files_new RENAME TO project_files`); err != nil {
			tx.Rollback()
			log.Printf("migrateProjectFilesS3: rename table failed: %v", err)
			return
		}

		// Recreate indexes
		if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_project_files_project ON project_files(project_id)`); err != nil {
			log.Printf("migrateProjectFilesS3: recreate index failed: %v", err)
		}

		if err := tx.Commit(); err != nil {
			log.Printf("migrateProjectFilesS3: commit failed: %v", err)
			return
		}
		log.Println("[DB] Successfully dropped content column from project_files")
	}
}

// migrateUserIDTypes fixes tables where user_id was created as INTEGER
// but should be TEXT (since users.id is a UUID string).
// SQLite doesn't support ALTER COLUMN type changes, so we recreate each table.
func (db *DB) migrateUserIDTypes() {
	tables := []string{
		"notifications", "activities", "api_keys", "search_history",
		"favorites", "user_badges", "backup_schedules", "recycle_bin",
	}

	for _, table := range tables {
		var colType string
		err := db.Conn.QueryRow("SELECT type FROM pragma_table_info('" + table + "', 'user_id')").Scan(&colType)
		if err != nil || colType == "TEXT" {
			continue // column already correct or table doesn't exist yet
		}

		log.Printf("[DB] Migrating %s.user_id from %s to TEXT", table, colType)

		// Get the original CREATE TABLE SQL and the column list
		var createSQL string
		err = db.Conn.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='" + table + "'").Scan(&createSQL)
		if err != nil {
			log.Printf("[DB] Could not get CREATE TABLE for %s: %v", table, err)
			continue
		}

		// Get all column names in order
		rows, err := db.Conn.Query("PRAGMA table_info('" + table + "')")
		if err != nil {
			log.Printf("[DB] Could not get columns for %s: %v", table, err)
			continue
		}
		var cols []string
		for rows.Next() {
			var cid int
			var name, ctype string
			var notnull int
			var dflt interface{}
			var pk int
			rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
			cols = append(cols, name)
		}
		rows.Close()

		colList := ""
		for i, c := range cols {
			if i > 0 {
				colList += ", "
			}
			colList += c
		}

		// Recreate with user_id as TEXT
		newTable := table + "_migrated_uid"
		tx, err := db.Conn.Begin()
		if err != nil {
			log.Printf("[DB] Could not begin transaction for %s: %v", table, err)
			continue
		}

		// Get original table SQL and modify it
		modifiedSQL := createSQL
		// Replace "user_id INTEGER" with "user_id TEXT" in the CREATE statement
		modifiedSQL = replaceColType(modifiedSQL, "user_id INTEGER", "user_id TEXT")
		// Change table name to temp name
		modifiedSQL = modifiedSQL[:len("CREATE TABLE IF NOT EXISTS ")] + newTable + modifiedSQL[len("CREATE TABLE IF NOT EXISTS "+table):]

		if _, err := tx.Exec(modifiedSQL); err != nil {
			tx.Rollback()
			log.Printf("[DB] Could not create migrated table %s: %v", newTable, err)
			continue
		}

		if _, err := tx.Exec("INSERT INTO " + newTable + " (" + colList + ") SELECT " + colList + " FROM " + table); err != nil {
			tx.Rollback()
			log.Printf("[DB] Could not copy data from %s to %s: %v", table, newTable, err)
			continue
		}

		if _, err := tx.Exec("DROP TABLE " + table); err != nil {
			tx.Rollback()
			log.Printf("[DB] Could not drop old table %s: %v", table, err)
			continue
		}

		if _, err := tx.Exec("ALTER TABLE " + newTable + " RENAME TO " + table); err != nil {
			tx.Rollback()
			log.Printf("[DB] Could not rename %s to %s: %v", newTable, table, err)
			continue
		}

		if err := tx.Commit(); err != nil {
			log.Printf("[DB] Could not commit migration for %s: %v", table, err)
			continue
		}

		log.Printf("[DB] Successfully migrated %s.user_id to TEXT", table)
	}
}

// replaceColType does a simple substring replacement for column type changes in DDL.
func replaceColType(sql, old, new string) string {
	result := ""
	for i := 0; i < len(sql); {
		if i+len(old) <= len(sql) && sql[i:i+len(old)] == old {
			// Make sure we're matching the column definition, not part of another word
			if i == 0 || sql[i-1] == ' ' || sql[i-1] == '\t' || sql[i-1] == '\n' || sql[i-1] == '(' || sql[i-1] == ',' {
				result += new
				i += len(old)
			} else {
				result += string(sql[i])
				i++
			}
		} else {
			result += string(sql[i])
			i++
		}
	}
	return result
}

// migrateADBSavedDevices rebuilds adb_saved_devices to change UNIQUE(address) → UNIQUE(user_id, address).
func (db *DB) migrateADBSavedDevices() {
	// Check if user_id column exists
	var cnt int
	err := db.Conn.QueryRow("SELECT COUNT(*) FROM pragma_table_info('adb_saved_devices', 'user_id')").Scan(&cnt)
	if err != nil || cnt == 0 {
		// column doesn't exist yet; addColumnIfMissing already handles it
		return
	}

	// Check current unique constraint — if already UNIQUE(user_id, address), skip
	var createSQL string
	err = db.Conn.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='adb_saved_devices'").Scan(&createSQL)
	if err != nil {
		return
	}
	// If old UNIQUE(address) still present, rebuild
	if strings.Contains(createSQL, "UNIQUE(address)") && !strings.Contains(createSQL, "UNIQUE(user_id") {
		log.Println("[DB] Migrating adb_saved_devices: UNIQUE(address) → UNIQUE(user_id, address)")
		tx, err := db.Conn.Begin()
		if err != nil {
			log.Printf("[DB] adb migration tx error: %v", err)
			return
		}
		tx.Exec(`CREATE TABLE adb_saved_devices_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			address TEXT NOT NULL,
			name TEXT DEFAULT '',
			user_id TEXT DEFAULT '',
			last_connected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, address)
		)`)
		tx.Exec(`INSERT INTO adb_saved_devices_new (id, address, name, user_id, last_connected_at, created_at)
			SELECT id, address, name, user_id, last_connected_at, created_at FROM adb_saved_devices`)
		tx.Exec(`DROP TABLE adb_saved_devices`)
		tx.Exec(`ALTER TABLE adb_saved_devices_new RENAME TO adb_saved_devices`)
		if err := tx.Commit(); err != nil {
			log.Printf("[DB] adb migration commit error: %v", err)
			tx.Rollback()
		} else {
			log.Println("[DB] adb_saved_devices migration complete")
		}
	}
}
