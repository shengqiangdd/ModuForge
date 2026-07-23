package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	Conn *sql.DB
}

func NewSQLiteDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(2)
	conn.SetConnMaxLifetime(0)

	db := &DB{Conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func (db *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT DEFAULT 'user',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS market_modules (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			slug TEXT UNIQUE NOT NULL,
			description TEXT,
			category TEXT,
			tags TEXT,
			version TEXT,
			version_code INTEGER,
			author TEXT,
			author_uid TEXT,
			license TEXT,
			stars INTEGER DEFAULT 0,
			installs INTEGER DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS market_reviews (
			id TEXT PRIMARY KEY,
			module_id TEXT NOT NULL,
			uid TEXT,
			username TEXT,
			rating INTEGER CHECK(rating BETWEEN 1 AND 5),
			comment TEXT,
			created_at DATETIME,
			FOREIGN KEY (module_id) REFERENCES market_modules(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_market_modules_category ON market_modules(category)`,
		`CREATE INDEX IF NOT EXISTS idx_market_modules_slug ON market_modules(slug)`,
		`CREATE INDEX IF NOT EXISTS idx_market_reviews_module ON market_reviews(module_id)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			module_type TEXT DEFAULT 'magisk',
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS project_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id TEXT NOT NULL,
			path TEXT NOT NULL,
			content TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (project_id) REFERENCES projects(id),
			UNIQUE(project_id, path)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_projects_user ON projects(user_id)`,
		`CREATE TABLE IF NOT EXISTS benchmark_results (
			id TEXT PRIMARY KEY,
			module_id TEXT NOT NULL,
			device_serial TEXT,
			before_data TEXT,
			after_data TEXT,
			diff_data TEXT,
			created_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS build_tasks (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			status TEXT DEFAULT 'pending',
			target TEXT,
			log TEXT,
			artifact_path TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (project_id) REFERENCES projects(id)
		)`,
		`CREATE TABLE IF NOT EXISTS collaborators (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT DEFAULT 'viewer',
			invited_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (project_id) REFERENCES projects(id)
		)`,
		`CREATE TABLE IF NOT EXISTS comments (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			username TEXT,
			file_path TEXT,
			line_number INTEGER,
			content TEXT,
			resolved INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (project_id) REFERENCES projects(id)
		)`,
		`CREATE TABLE IF NOT EXISTS edit_sessions (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			username TEXT,
			file_path TEXT,
			cursor_line INTEGER DEFAULT 0,
			cursor_col INTEGER DEFAULT 0,
			selection_start_line INTEGER DEFAULT 0,
			selection_start_col INTEGER DEFAULT 0,
			selection_end_line INTEGER DEFAULT 0,
			selection_end_col INTEGER DEFAULT 0,
			color TEXT,
			connected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_active DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (project_id) REFERENCES projects(id)
		)`,
		`CREATE TABLE IF NOT EXISTS plugins (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT UNIQUE NOT NULL,
			description TEXT,
			author TEXT,
			version TEXT,
			enabled INTEGER DEFAULT 0,
			config TEXT,
			installed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
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
		if _, err := db.Conn.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %s: %w", m[:50], err)
		}
	}

	// Post-migration: add columns that may not exist in older schemas
	addColumnIfMissing := []string{
		"ALTER TABLE comments ADD COLUMN resolved INTEGER DEFAULT 0",
		"CREATE INDEX IF NOT EXISTS idx_project_files_project ON project_files(project_id)",
		"CREATE INDEX IF NOT EXISTS idx_build_tasks_project ON build_tasks(project_id)",
		"CREATE INDEX IF NOT EXISTS idx_build_tasks_status ON build_tasks(status)",
	}
	for _, m := range addColumnIfMissing {
		db.Conn.Exec(m) // ignore errors for ALTER TABLE
	}

	log.Println("[DB] SQLite migrations complete")
	return nil
}

func (db *DB) Close() error {
	return db.Conn.Close()
}

func (db *DB) SeedMarketData() error {
	var count int
	db.Conn.QueryRow("SELECT COUNT(*) FROM market_modules").Scan(&count)
	if count > 0 {
		return nil
	}

	seeds := []struct {
		id, title, slug, desc, cat, tags, ver, author, lic string
		stars, installs                                    int
	}{
		{"mod_0001", "System Prop Tweaks", "system-prop-tweaks", "Comprehensive system property modifications for performance and battery optimization.", "system", "system,prop,performance", "v2.1", "ModuForge Team", "MIT", 128, 3500},
		{"mod_0002", "Custom Boot Animation", "boot-animation", "Replace default boot animation with custom designs.", "ui", "boot,animation,custom", "v1.3", "DevMaster", "Apache-2.0", 89, 2100},
		{"mod_0003", "Audio Enhancement", "audio-enhance", "Improve audio quality with custom DAC configurations.", "audio", "audio,dac,equalizer", "v1.8", "SoundModder", "GPL-3.0", 156, 4200},
		{"mod_0004", "GPU Overclock Pro", "gpu-overclock", "Safe GPU frequency adjustments for better gaming.", "display", "gpu,overclock,gaming", "v1.5", "GameTuner", "MIT", 234, 5800},
		{"mod_0005", "Network Firewall", "network-firewall", "Per-app network access control with ad blocking.", "utility", "network,firewall,adblock", "v2.0", "PrivacyGuard", "GPL-3.0", 312, 7600},
		{"mod_0006", "Battery Saver Max", "battery-saver", "Intelligent battery management with Doze optimization.", "system", "battery,doze,performance", "v1.4", "BatteryPro", "MIT", 198, 4500},
		{"mod_0007", "Display Calibrator", "display-calibrate", "Professional display calibration with ICC profiles.", "display", "display,calibrate,color", "v1.2", "ColorExpert", "MIT", 76, 1800},
		{"mod_0008", "Hosts AdBlock", "hosts-adblock", "Hosts file based ad blocker with auto-update.", "utility", "adblock,hosts,privacy", "v3.0", "AdGuardFork", "GPL-3.0", 456, 12000},
		{"mod_0009", "Magisk Manager Lite", "magisk-lite", "Lightweight Magisk module management alternative.", "system", "magisk,manager,lite", "v1.1", "LiteDev", "Apache-2.0", 45, 900},
		{"mod_0010", "Notification Sound Pack", "notification-sounds", "50+ notification sounds organized by category.", "ui", "notification,sounds,ringtones", "v1.6", "SoundPack", "CC-BY-4.0", 67, 2300},
	}

	stmt, err := db.Conn.Prepare("INSERT INTO market_modules (id, title, slug, description, category, tags, version, version_code, author, license, stars, installs, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, s := range seeds {
		_, err := stmt.Exec(s.id, s.title, s.slug, s.desc, s.cat, s.tags, s.ver, 0, s.author, s.lic, s.stars, s.installs, now, now)
		if err != nil {
			return fmt.Errorf("seed %s: %w", s.title, err)
		}
	}

	log.Printf("[DB] Seeded %d market modules\n", len(seeds))
	return nil
}
