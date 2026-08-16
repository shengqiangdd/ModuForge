package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

type DB struct {
	Conn *sql.DB
}

func NewSQLiteDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=10000&_foreign_keys=ON&_loc=auto")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite with WAL mode supports concurrent readers.
	// MaxOpenConns > 1 allows concurrent reads while writes are serialized by SQLite.
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(4)
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
		`CREATE INDEX IF NOT EXISTS idx_market_modules_created ON market_modules(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_market_modules_stars ON market_modules(stars DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_market_modules_installs ON market_modules(installs DESC)`,
		`CREATE TABLE IF NOT EXISTS module_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			module_id TEXT NOT NULL,
			version TEXT NOT NULL,
			changelog TEXT DEFAULT '',
			file_hash TEXT DEFAULT '',
			file_path TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (module_id) REFERENCES market_modules(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_module_versions_module ON module_versions(module_id)`,
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
			trigger TEXT DEFAULT 'manual',
			commit_hash TEXT DEFAULT '',
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
		// Provider configs: per-user overrides for preset providers
		`CREATE TABLE IF NOT EXISTS provider_configs (
			id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			endpoint TEXT,
			api_key TEXT,
			is_active INTEGER DEFAULT 1,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS team_members (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			invited_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id TEXT,
			user_id TEXT,
			action TEXT NOT NULL,
			details TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_team_members_project ON team_members(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_team_members_user ON team_members(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_project ON audit_logs(project_id)`,

		`CREATE TABLE IF NOT EXISTS notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			link TEXT DEFAULT '',
			is_read INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications(user_id, is_read)`,
		`CREATE TABLE IF NOT EXISTS activities (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			project_id INTEGER,
			activity_type TEXT NOT NULL,
			description TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_activities_user ON activities(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_activities_project ON activities(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_activities_created ON activities(created_at)`,

		// Custom providers: user-defined OpenAI-compatible providers
		`CREATE TABLE IF NOT EXISTS custom_providers (
			id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			endpoint TEXT NOT NULL,
			api_key TEXT,
			models_json TEXT,
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS module_screenshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			module_id TEXT NOT NULL,
			url TEXT NOT NULL,
			sort_order INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (module_id) REFERENCES market_modules(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_screenshots_module ON module_screenshots(module_id)`,
		`CREATE TABLE IF NOT EXISTS email_config (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			smtp_host TEXT NOT NULL DEFAULT '',
			smtp_port INTEGER DEFAULT 587,
			smtp_user TEXT DEFAULT '',
			smtp_password TEXT DEFAULT '',
			from_name TEXT DEFAULT 'ModuForge',
			from_email TEXT DEFAULT '',
			use_tls INTEGER DEFAULT 1,
			is_active INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			key_hash TEXT NOT NULL,
			key_prefix TEXT NOT NULL,
			permissions TEXT DEFAULT '["read"]',
			last_used_at DATETIME,
			expires_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS custom_skills (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			prompt TEXT NOT NULL,
			input_schema TEXT DEFAULT '{}',
			is_public INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS skill_evolution (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			skill_id INTEGER NOT NULL,
			user_id TEXT NOT NULL,
			input TEXT NOT NULL,
			output TEXT,
			success INTEGER DEFAULT 1,
			duration_ms INTEGER DEFAULT 0,
			feedback TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (skill_id) REFERENCES custom_skills(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS search_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			query TEXT NOT NULL,
			result_count INTEGER DEFAULT 0,
			searched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_search_history_user ON search_history(user_id)`,
		`CREATE TABLE IF NOT EXISTS favorites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			item_type TEXT NOT NULL,
			item_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id, item_type, item_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_favorites_user ON favorites(user_id)`,
		`CREATE TABLE IF NOT EXISTS module_tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			color TEXT DEFAULT '#6366f1',
			usage_count INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS module_tag_relations (
			module_slug TEXT NOT NULL,
			tag_id INTEGER NOT NULL,
			PRIMARY KEY (module_slug, tag_id),
			FOREIGN KEY (tag_id) REFERENCES module_tags(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS user_badges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			badge_key TEXT NOT NULL,
			earned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id, badge_key)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_badges_user ON user_badges(user_id)`,
		`CREATE TABLE IF NOT EXISTS module_changelogs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			module_slug TEXT NOT NULL,
			version TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (module_slug) REFERENCES market_modules(slug) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_changelogs_slug ON module_changelogs(module_slug)`,
		`CREATE TABLE IF NOT EXISTS crash_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id TEXT NOT NULL,
			module_slug TEXT DEFAULT '',
			error_type TEXT NOT NULL,
			stack_trace TEXT NOT NULL,
			device_info TEXT DEFAULT '{}',
			app_version TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_crash_logs_device ON crash_logs(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_crash_logs_module ON crash_logs(module_slug)`,
		// Module patterns for learning
		`CREATE TABLE IF NOT EXISTS module_patterns (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			module_type TEXT NOT NULL,
			pattern_type TEXT NOT NULL DEFAULT 'success',
			pattern TEXT NOT NULL,
			context TEXT DEFAULT '',
			success_count INTEGER DEFAULT 1,
			total_count INTEGER DEFAULT 1,
			usage_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(module_type, pattern)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_module_patterns_type ON module_patterns(module_type)`,
		// Prompt version control
		`CREATE TABLE IF NOT EXISTS prompt_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			skill_id TEXT NOT NULL,
			prompt TEXT NOT NULL,
			version INTEGER NOT NULL,
			change_reason TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_prompt_versions_skill ON prompt_versions(skill_id)`,
		// Cross-user shared patterns
		`CREATE TABLE IF NOT EXISTS shared_patterns (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			module_type TEXT NOT NULL,
			pattern TEXT NOT NULL,
			success_rate REAL,
			usage_count INTEGER DEFAULT 1,
			is_shared BOOLEAN DEFAULT 0,
			shared_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id, module_type, pattern)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_shared_patterns_shared ON shared_patterns(is_shared)`,
		`CREATE INDEX IF NOT EXISTS idx_shared_patterns_type ON shared_patterns(module_type)`,
		// Agent presets
		`CREATE TABLE IF NOT EXISTS agent_presets (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			style TEXT DEFAULT 'professional',
			system_prompt TEXT DEFAULT '',
			temperature REAL DEFAULT 0.7,
			max_tokens INTEGER DEFAULT 4096,
			is_default INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_presets_user ON agent_presets(user_id)`,
		`CREATE TABLE IF NOT EXISTS vulnerability_scans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL,
			scanner TEXT NOT NULL,
			total_deps INTEGER DEFAULT 0,
			vulnerable_deps INTEGER DEFAULT 0,
			critical_count INTEGER DEFAULT 0,
			high_count INTEGER DEFAULT 0,
			medium_count INTEGER DEFAULT 0,
			low_count INTEGER DEFAULT 0,
			results TEXT DEFAULT '[]',
			scanned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS dashboard_widgets (
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
		)`,
		`CREATE INDEX IF NOT EXISTS idx_dashboard_widgets_user ON dashboard_widgets(user_id)`,
		`CREATE TABLE IF NOT EXISTS webhook_deliveries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hook_id INTEGER NOT NULL,
			event TEXT NOT NULL,
			payload TEXT NOT NULL,
			response_status INTEGER DEFAULT 0,
			response_body TEXT DEFAULT '',
			success INTEGER DEFAULT 0,
			duration_ms INTEGER DEFAULT 0,
			error_message TEXT DEFAULT '',
			delivered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (hook_id) REFERENCES plugin_hooks(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_hook ON webhook_deliveries(hook_id)`,
		`CREATE TABLE IF NOT EXISTS backup_schedules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			frequency TEXT NOT NULL DEFAULT 'daily',
			keep_count INTEGER DEFAULT 7,
			last_backup_at DATETIME,
			next_backup_at DATETIME,
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_backup_schedules_user ON backup_schedules(user_id)`,
		`CREATE TABLE IF NOT EXISTS glossary (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			term TEXT NOT NULL UNIQUE,
			definition TEXT NOT NULL,
			category TEXT DEFAULT 'general',
			related_terms TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS recycle_bin (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			item_type TEXT NOT NULL,
			item_id INTEGER NOT NULL,
			item_name TEXT NOT NULL,
			item_data TEXT NOT NULL,
			deleted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_recycle_bin_user ON recycle_bin(user_id)`,
		`CREATE TABLE IF NOT EXISTS adb_saved_devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			address TEXT NOT NULL,
			name TEXT DEFAULT '',
			user_id TEXT DEFAULT '',
			last_connected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, address)
		)`,
		// MCP server configurations (UI/API managed, persisted across restarts)
		// headers is a JSON object string, e.g. {"Authorization":"Bearer xxx"}
		`CREATE TABLE IF NOT EXISTS mcp_servers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			url TEXT NOT NULL,
			headers TEXT NOT NULL DEFAULT '{}',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// ===== Batch 2: Module System Enhancements =====
		// Feature 1: Project Version Management (snapshots)
		`CREATE TABLE IF NOT EXISTS project_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id TEXT NOT NULL,
			version TEXT NOT NULL,
			changelog TEXT DEFAULT '',
			file_count INTEGER DEFAULT 0,
			total_size INTEGER DEFAULT 0,
			snapshot TEXT DEFAULT '[]',
			file_hash TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(project_id, version),
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_project_versions_project ON project_versions(project_id)`,
		// Feature 2: Module Template Marketplace
		`CREATE TABLE IF NOT EXISTS module_templates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			category TEXT DEFAULT '',
			author TEXT DEFAULT '',
			author_uid TEXT DEFAULT '',
			downloads INTEGER DEFAULT 0,
			rating REAL DEFAULT 0,
			module_data TEXT DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_module_templates_category ON module_templates(category)`,
		`CREATE INDEX IF NOT EXISTS idx_module_templates_downloads ON module_templates(downloads DESC)`,
		`CREATE TABLE IF NOT EXISTS template_ratings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			template_id INTEGER NOT NULL,
			user_id TEXT NOT NULL,
			rating REAL NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(template_id, user_id),
			FOREIGN KEY (template_id) REFERENCES module_templates(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_template_ratings_template ON template_ratings(template_id)`,
		// Feature 3: Module Dependency Resolution
		`CREATE TABLE IF NOT EXISTS module_dependencies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			module_id TEXT NOT NULL,
			dependency_id TEXT NOT NULL,
			min_version TEXT DEFAULT '',
			optional INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(module_id, dependency_id),
			FOREIGN KEY (module_id) REFERENCES market_modules(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_module_dependencies_module ON module_dependencies(module_id)`,
		`CREATE INDEX IF NOT EXISTS idx_module_dependencies_dep ON module_dependencies(dependency_id)`,
		// Batch 3: Security Enhancements
		// Feature 1: Module Code Signing
		`CREATE TABLE IF NOT EXISTS module_signatures (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			module_id TEXT NOT NULL,
			public_key TEXT NOT NULL,
			signature TEXT NOT NULL,
			file_hash TEXT NOT NULL,
			signed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(module_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_module_signatures_module ON module_signatures(module_id)`,
		// Feature 2: Vulnerability Scanning (extended)
		`CREATE TABLE IF NOT EXISTS module_vuln_scans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			module_id TEXT NOT NULL,
			project_id TEXT NOT NULL,
			total_issues INTEGER DEFAULT 0,
			critical_count INTEGER DEFAULT 0,
			high_count INTEGER DEFAULT 0,
			medium_count INTEGER DEFAULT 0,
			low_count INTEGER DEFAULT 0,
			results TEXT DEFAULT '[]',
			scanned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (module_id) REFERENCES market_modules(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_module_vuln_scans_module ON module_vuln_scans(module_id)`,
		`CREATE INDEX IF NOT EXISTS idx_module_vuln_scans_project ON module_vuln_scans(project_id)`,
		// Feature 3: Permission Audit
		`CREATE TABLE IF NOT EXISTS permission_audits (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			module_id TEXT NOT NULL,
			project_id TEXT NOT NULL,
			total_permissions INTEGER DEFAULT 0,
			dangerous_count INTEGER DEFAULT 0,
			risk_score INTEGER DEFAULT 0,
			results TEXT DEFAULT '[]',
			audited_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (module_id) REFERENCES market_modules(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_permission_audits_module ON permission_audits(module_id)`,
		`CREATE INDEX IF NOT EXISTS idx_permission_audits_project ON permission_audits(project_id)`,

		// ===== Batch 4: Collaboration + Git + Comments =====
		// Feature 1: File Comments System
		`CREATE TABLE IF NOT EXISTS file_comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id TEXT NOT NULL,
			file_path TEXT NOT NULL,
			user_id TEXT NOT NULL,
			username TEXT DEFAULT '',
			line_number INTEGER,
			content TEXT NOT NULL,
			parent_id INTEGER DEFAULT 0,
			resolved INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_file_comments_project ON file_comments(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_file_comments_file ON file_comments(project_id, file_path)`,
		`CREATE INDEX IF NOT EXISTS idx_file_comments_parent ON file_comments(parent_id)`,

		// Feature 2: Git Branches (extends existing git history)
		`CREATE TABLE IF NOT EXISTS git_branches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id TEXT NOT NULL,
			name TEXT NOT NULL,
			is_default INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(project_id, name),
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_git_branches_project ON git_branches(project_id)`,

		// Feature 3: Collaboration Sessions (real-time)
		`CREATE TABLE IF NOT EXISTS collaboration_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			username TEXT DEFAULT '',
			file_path TEXT DEFAULT '',
			cursor_line INTEGER DEFAULT 0,
			cursor_column INTEGER DEFAULT 0,
			connected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_active DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_collab_sessions_project ON collaboration_sessions(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_collab_sessions_active ON collaboration_sessions(project_id, last_active)`,

		// ===== AI Conversations persistence =====
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
		`CREATE TABLE IF NOT EXISTS conversation_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ===== Batch 5: Performance Indexes =====
		// High-frequency query indexes
		`CREATE INDEX IF NOT EXISTS idx_projects_user_id ON projects(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_projects_updated_at ON projects(updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_conversations_user_id ON ai_conversations(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_conversations_updated_at ON ai_conversations(updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_skill_evolution_skill_id ON skill_evolution(skill_id)`,
		`CREATE INDEX IF NOT EXISTS idx_skill_evolution_user_id ON skill_evolution(user_id)`,
		// Composite indexes for common query patterns
		`CREATE INDEX IF NOT EXISTS idx_projects_user_updated ON projects(user_id, updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_conversations_user_updated ON ai_conversations(user_id, updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_skill_evolution_skill_user ON skill_evolution(skill_id, user_id)`,

		// ===== Batch 6: Log Aggregation =====
		`CREATE TABLE IF NOT EXISTS app_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			level TEXT NOT NULL,
			module TEXT,
			message TEXT NOT NULL,
			details TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_app_logs_level ON app_logs(level)`,
		`CREATE INDEX IF NOT EXISTS idx_app_logs_module ON app_logs(module)`,
		`CREATE INDEX IF NOT EXISTS idx_app_logs_created ON app_logs(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_app_logs_level_created ON app_logs(level, created_at)`,
	}

	for _, m := range migrations {
		if _, err := db.Conn.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %s: %w", m[:50], err)
		}
	}

	// Post-migration: add columns that may not exist in older schemas
	addColumnIfMissing := []string{
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
		"CREATE INDEX IF NOT EXISTS idx_project_files_project ON project_files(project_id)",
		// Add project_id to ai_conversations for linking conversations to projects
		"ALTER TABLE ai_conversations ADD COLUMN project_id TEXT DEFAULT ''",
		"CREATE INDEX IF NOT EXISTS idx_build_tasks_project ON build_tasks(project_id)",
		"CREATE INDEX IF NOT EXISTS idx_build_tasks_status ON build_tasks(status)",
		"CREATE INDEX IF NOT EXISTS idx_comments_project ON comments(project_id)",
		"CREATE INDEX IF NOT EXISTS idx_edit_sessions_project ON edit_sessions(project_id)",
		"CREATE INDEX IF NOT EXISTS idx_collaborators_project ON collaborators(project_id)",
		"CREATE INDEX IF NOT EXISTS idx_collaborators_user ON collaborators(user_id)",
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
		"CREATE INDEX IF NOT EXISTS idx_build_schedules_project ON build_schedules(project_id)",
		"CREATE INDEX IF NOT EXISTS idx_build_schedules_next ON build_schedules(is_active, next_build_at)",
		// Add trigger and commit_hash to existing build_tasks table
		"ALTER TABLE build_tasks ADD COLUMN trigger TEXT DEFAULT 'manual'",
		"ALTER TABLE build_tasks ADD COLUMN commit_hash TEXT DEFAULT ''",
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
	}
	for _, m := range addColumnIfMissing {
		db.Conn.Exec(m) // ignore errors for ALTER TABLE
	}

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

	// Migrate user_id columns from INTEGER to TEXT in 8 affected tables.
	// Users.id is UUID (TEXT), so user_id must also be TEXT for foreign keys to work.
	db.migrateUserIDTypes()

	// Migrate adb_saved_devices: rebuild with UNIQUE(user_id, address) instead of UNIQUE(address)
	db.migrateADBSavedDevices()

	// S3 storage migration: add metadata columns to project_files
	db.migrateProjectFilesS3()

	log.Println("[DB] SQLite migrations complete")
	return nil
}

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
			}
		}
	}

	if !hasSHA256 {
		db.Conn.Exec("ALTER TABLE project_files ADD COLUMN sha256 TEXT DEFAULT ''")
	}
	if !hasSize {
		db.Conn.Exec("ALTER TABLE project_files ADD COLUMN file_size INTEGER DEFAULT 0")
	}
	if !hasMTime {
		db.Conn.Exec("ALTER TABLE project_files ADD COLUMN mtime TEXT DEFAULT ''")
	}
}

func (db *DB) Close() error {
	return db.Conn.Close()
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

		if _, err := tx.Exec("INSERT INTO "+newTable+" ("+colList+") SELECT "+colList+" FROM "+table); err != nil {
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

func (db *DB) SeedGlossary() error {
	terms := []struct {
		term, definition, category, related string
	}{
		{"Module", "模块是 ModuForge 中的基本功能单元，可用于扩展或修改 Android 系统的行为。", "general", "ADB,Build,Widget"},
		{"ADB", "Android Debug Bridge，用于与 Android 设备通信的命令行工具。", "dev", "Device,Shell,Debug"},
		{"Build（构建）", "将源代码编译为可分发的模块包的过程。", "dev", "Module,CI/CD,ZIP"},
		{"Widget", "仪表盘上的可自定义显示组件，用于展示特定信息。", "general", "Dashboard,Analytics"},
		{"CI/CD", "持续集成与持续部署，自动化构建和发布流程。", "dev", "Build,Webhook,Auto"},
		{"Webhook", "当特定事件发生时触发的 HTTP 回调，用于通知外部系统。", "dev", "CI/CD,API,Git"},
		{"API Key", "用于程序化访问 ModuForge API 的认证令牌。", "security", "Auth,JWT,Token"},
		{"2FA（两步验证）", "通过额外验证码增强账户安全的认证方式。", "security", "TOTP,Auth,Security"},
		{"Changelog", "记录模块每个版本变更内容的文档。", "general", "Version,Module,Update"},
		{"Vulnerability（漏洞）", "代码中可能被利用的安全缺陷。", "security", "Security,Scan,Audit"},
		{"TOTP", "基于时间的一次性密码，用于双因素认证。", "security", "2FA,Auth,Secret"},
		{"Screenshot（截图）", "模块功能的可视化展示图片。", "general", "Module,Gallery,Preview"},
		{"Benchmark（基准测试）", "测量和评估设备性能的标准化测试。", "dev", "Performance,Device,Test"},
		{"Deploy（部署）", "将构建产物安装到目标设备的过程。", "dev", "Build,Install,Device"},
		{"Plugin（插件）", "可扩展 ModuForge 功能的附加组件。", "general", "Extension,Hook,Module"},
		{"Audit Log（审计日志）", "记录系统中重要操作事件的日志。", "security", "Security,Log,Tracking"},
		{"Provider（提供商）", "提供 AI 模型 API 的服务商配置。", "ai", "AI,LLM,Model"},
		{"Prompt（提示词）", "发送给 AI 模型以引导其输出的指令文本。", "ai", "AI,Generation,Template"},
		{"Rollback（回滚）", "将模块恢复到之前版本的操作。", "general", "Version,Restore,Backup"},
		{"License（许可证）", "定义模块使用、修改和分发条款的法律文件。", "general", "Module,Legal,Open Source"},
	}
	for _, t := range terms {
		db.Conn.Exec("INSERT OR IGNORE INTO glossary (term, definition, category, related_terms) VALUES (?, ?, ?, ?)", t.term, t.definition, t.category, t.related)
	}
	log.Printf("[DB] Seeded %d glossary terms", len(terms))
	return nil
}

func (db *DB) SeedAdminUser() error {
	// 确保存在至少一个 admin 用户（即使已有其他用户）
	var adminCount int
	db.Conn.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&adminCount)
	if adminCount > 0 {
		return nil // 已有管理员，跳过
	}

	// 默认管理员凭据，可通过环境变量覆盖
	username := getEnvOrDefault("ADMIN_USERNAME", "admin")
	email := getEnvOrDefault("ADMIN_EMAIL", "admin@moduforge.local")
	password := getEnvOrDefault("ADMIN_PASSWORD", "admin123")

	if password == "admin123" && os.Getenv("ADMIN_PASSWORD") == "" {
		log.Printf("[DB] WARNING: using default admin password 'admin123'. Set ADMIN_PASSWORD env var in production.")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	// 检查用户名是否已存在
	var existingUser int
	db.Conn.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&existingUser)
	if existingUser > 0 {
		// 用户已存在但不是 admin，提升为 admin
		_, err = db.Conn.Exec("UPDATE users SET role = 'admin' WHERE username = ?", username)
		if err != nil {
			return fmt.Errorf("promote admin user: %w", err)
		}
		log.Printf("[DB] Promoted existing user '%s' to admin", username)
		return nil
	}

	_, err = db.Conn.Exec(
		`INSERT INTO users (id, username, email, password_hash, role, email_verified)
		 VALUES (?, ?, ?, ?, 'admin', 1)`,
		uuid.New().String(), username, email, string(hash),
	)
	if err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}
	log.Printf("[DB] Seeded admin user: %s (password set via env or default)", username)
	return nil
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (db *DB) SeedMarketData() error {
	var count int
	db.Conn.QueryRow("SELECT COUNT(*) FROM market_modules").Scan(&count)
	if count > 0 {
		now := time.Now()
		db.Conn.Exec("UPDATE market_modules SET created_at = ?, updated_at = ? WHERE created_at = '0001-01-01 00:00:00' OR created_at IS NULL", now, now)
		seedStars := map[string]int{"mod_0001": 128, "mod_0002": 89, "mod_0003": 156, "mod_0004": 234, "mod_0005": 312, "mod_0006": 198, "mod_0007": 76, "mod_0008": 456, "mod_0009": 45, "mod_0010": 67}
		seedInstalls := map[string]int{"mod_0001": 3500, "mod_0002": 2100, "mod_0003": 4200, "mod_0004": 5800, "mod_0005": 7600, "mod_0006": 4500, "mod_0007": 1800, "mod_0008": 12000, "mod_0009": 900, "mod_0010": 2300}
		for id, stars := range seedStars {
			db.Conn.Exec("UPDATE market_modules SET stars = ?, installs = ? WHERE id = ? AND stars = 0", stars, seedInstalls[id], id)
		}
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
