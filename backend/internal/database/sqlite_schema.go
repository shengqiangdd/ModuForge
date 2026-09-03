package database

// schemaMigrations contains all CREATE TABLE statements for the database schema.
func schemaMigrations() []string {
	return []string{
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
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			sha256 TEXT DEFAULT '',
			file_size INTEGER DEFAULT 0,
			mtime TEXT DEFAULT '',
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
		`CREATE TABLE IF NOT EXISTS backup_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			schedule_id INTEGER,
			schedule_name TEXT DEFAULT '',
			status TEXT NOT NULL DEFAULT 'success',
			size_bytes INTEGER DEFAULT 0,
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			finished_at DATETIME,
			FOREIGN KEY (schedule_id) REFERENCES backup_schedules(id) ON DELETE SET NULL
		)`,
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
		// MCP tool permission policies — which write tools the Agent may call
		// automatically without user confirmation (Claude Code permission mode).
		// mode: 'allow' (auto), 'deny' (blocked), 'ask' (per-call user confirmation)
		`CREATE TABLE IF NOT EXISTS mcp_tool_policies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			server TEXT NOT NULL,
			tool TEXT NOT NULL,
			allow_auto INTEGER NOT NULL DEFAULT 0,
			mode TEXT NOT NULL DEFAULT 'deny',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(server, tool)
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
		// NOTE: module_dependencies table removed — no handlers, no active code
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

		// NOTE: git_branches table removed — no handlers, no active code
		// NOTE: collaboration_sessions table removed — no handlers, no active code

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
}
