package database

// orphanTables returns tables that should be dropped after migration.
func orphanTables() []string {
	return []string{
		"DROP TABLE IF EXISTS plugins",
		"DROP TABLE IF EXISTS plugin_hooks",
		"DROP TABLE IF EXISTS webhook_deliveries",
		"DROP TABLE IF EXISTS permission_audits",
		"DROP TABLE IF EXISTS module_dependencies",
		"DROP TABLE IF EXISTS git_branches",
		"DROP TABLE IF EXISTS collaboration_sessions",
	}
}

// orphanIndexes returns indexes that should be dropped after migration.
func orphanIndexes() []string {
	return []string{
		"DROP INDEX IF EXISTS idx_webhook_deliveries_hook",
		"DROP INDEX IF EXISTS idx_permission_audits_module",
		"DROP INDEX IF EXISTS idx_permission_audits_project",
	}
}

// cleanupIndexes returns indexes on empty/scaffold tables that should be dropped.
func cleanupIndexes() []string {
	return []string{
		"idx_market_reviews_module",
		"idx_comments_project",
		"idx_edit_sessions_project",
		"idx_collaborators_project",
		"idx_team_members_project",
		"idx_team_members_user",
		"idx_audit_logs_user",
		"idx_audit_logs_project",
		"idx_module_ratings_module",
		"idx_module_downloads_module",
		"idx_module_downloads_user",
		"idx_module_reviews_module",
		"idx_module_dependencies_module",
		"idx_module_dependencies_dep",
		"idx_git_branches_project",
		"idx_git_branches_name",
		"idx_git_commits_branch",
		"idx_git_diffs_commit",
		"idx_plugin_hooks_plugin",
		"idx_plugin_configs_plugin",
		"idx_collaboration_sessions_project",
	}
}
