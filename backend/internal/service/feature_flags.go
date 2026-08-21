package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// FeatureFlag represents a single feature toggle.
type FeatureFlag struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// FeatureFlagService manages feature flags with an in-memory cache
// backed by the feature_flags SQLite table.
type FeatureFlagService struct {
	db    *sql.DB
	mu    sync.RWMutex
	flags map[string]bool // key → enabled
}

// defaultFlags defines all known features and their initial enabled state.
// Features with active code but no user data start enabled — they are
// legitimate features that simply haven't been exercised yet.
var defaultFlags = []FeatureFlag{
	// Crash reporting
	{Key: "crash_reporting", Description: "崩溃日志上报与查看", Enabled: true},
	// Collaboration
	{Key: "collaboration", Description: "实时协作（协作者/评论/编辑会话）", Enabled: true},
	// File-level comments
	{Key: "file_comments", Description: "文件级行内评论", Enabled: true},
	// Favorites
	{Key: "favorites", Description: "收藏夹", Enabled: true},
	// Backup scheduling
	{Key: "backup_schedules", Description: "自动备份计划", Enabled: true},
	// Security scanning
	{Key: "security_scanning", Description: "安全扫描与漏洞历史", Enabled: true},
	// Badges & gamification
	// Module marketplace
	{Key: "module_marketplace", Description: "模块市场（发布/安装/评论）", Enabled: true},
	// Template marketplace
	{Key: "template_marketplace", Description: "模板市场（发布/使用/评分）", Enabled: true},
	// Email config
	{Key: "email_config", Description: "邮件通知配置", Enabled: true},
	// Tags
	{Key: "tags", Description: "标签管理", Enabled: true},
	// Build scheduling
	{Key: "build_schedules", Description: "构建计划（定时构建）", Enabled: true},
	// Benchmark
	{Key: "benchmarks", Description: "性能基准测试", Enabled: true},
	// Analytics
	{Key: "analytics", Description: "分析统计（构建趋势/系统统计）", Enabled: true},
	// Code formatting
	{Key: "code_formatting", Description: "代码格式化", Enabled: true},
	// Module signing
	{Key: "module_signing", Description: "模块签名与验证", Enabled: true},
	// Dependency analysis
	{Key: "dependency_analysis", Description: "依赖分析与解析", Enabled: true},
	// Module versions
	{Key: "module_versions", Description: "模块版本管理（版本历史/回滚）", Enabled: true},
}

// NewFeatureFlagService creates the service and ensures the table exists.
func NewFeatureFlagService(db *sql.DB) *FeatureFlagService {
	svc := &FeatureFlagService{
		db:    db,
		flags: make(map[string]bool),
	}
	svc.ensureTable()
	svc.loadFromDB()
	return svc
}

// ensureTable creates the feature_flags table if it doesn't exist.
func (s *FeatureFlagService) ensureTable() {
	s.db.Exec(`CREATE TABLE IF NOT EXISTS feature_flags (
		key         TEXT PRIMARY KEY,
		description TEXT NOT NULL DEFAULT '',
		enabled     INTEGER NOT NULL DEFAULT 1,
		created_at  TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
}

// loadFromDB reads all flags from DB and populates the in-memory cache.
// For any default flag not yet in DB, it inserts with the default value.
func (s *FeatureFlagService) loadFromDB() {
	// Upsert defaults
	for _, f := range defaultFlags {
		s.db.Exec(`INSERT OR IGNORE INTO feature_flags (key, description, enabled) VALUES (?, ?, ?)`,
			f.Key, f.Description, boolToInt(f.Enabled))
	}

	// Load all
	rows, err := s.db.Query(`SELECT key, enabled FROM feature_flags`)
	if err != nil {
		slog.Error("[FeatureFlags] failed to load from DB", "error", err)
		return
	}
	defer rows.Close()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.flags = make(map[string]bool)
	for rows.Next() {
		var key string
		var enabled int
		if err := rows.Scan(&key, &enabled); err != nil {
			continue
		}
		s.flags[key] = enabled == 1
	}
	slog.Info("[FeatureFlags] loaded", "count", len(s.flags))
}

// IsEnabled returns whether a feature is enabled. Defaults to true for
// unknown keys (fail-open).
func (s *FeatureFlagService) IsEnabled(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.flags[key]; ok {
		return v
	}
	return true // fail-open for unknown keys
}

// List returns all feature flags.
func (s *FeatureFlagService) List() []FeatureFlag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]FeatureFlag, 0, len(defaultFlags))
	for _, df := range defaultFlags {
		enabled := df.Enabled
		if v, ok := s.flags[df.Key]; ok {
			enabled = v
		}
		result = append(result, FeatureFlag{
			Key:         df.Key,
			Description: df.Description,
			Enabled:     enabled,
		})
	}
	return result
}

// SetEnabled updates a single flag, persists it, and writes an audit log entry.
func (s *FeatureFlagService) SetEnabled(key string, enabled bool, userID string) error {
	_, err := s.db.Exec(`UPDATE feature_flags SET enabled=?, updated_at=datetime('now') WHERE key=?`,
		boolToInt(enabled), key)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.flags[key] = enabled
	s.mu.Unlock()

	// Write audit log entry for the change
	state := "disabled"
	if enabled {
		state = "enabled"
	}
	s.db.Exec(`INSERT INTO audit_logs (project_id, user_id, action, details) VALUES (?, ?, ?, ?)`,
		"", userID, "feature_flag",
		fmt.Sprintf("Feature %s %s", key, state))

	slog.Info("[FeatureFlags] updated", "key", key, "enabled", enabled)
	return nil
}

// SetEnabledBatch updates multiple flags in a single transaction.
func (s *FeatureFlagService) SetEnabledBatch(items []struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}, userID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE feature_flags SET enabled=?, updated_at=datetime('now') WHERE key=?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	auditStmt, err := tx.Prepare(`INSERT INTO audit_logs (project_id, user_id, action, details) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer auditStmt.Close()

	for _, item := range items {
		if _, err := stmt.Exec(boolToInt(item.Enabled), item.Key); err != nil {
			return err
		}
		state := "disabled"
		if item.Enabled {
			state = "enabled"
		}
		if _, err := auditStmt.Exec("", userID, "feature_flag",
			fmt.Sprintf("Feature %s %s", item.Key, state)); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Update in-memory cache
	s.mu.Lock()
	for _, item := range items {
		s.flags[item.Key] = item.Enabled
	}
	s.mu.Unlock()

	slog.Info("[FeatureFlags] batch updated", "count", len(items))
	return nil
}

// Reload forces a reload from DB (e.g. after external changes).
func (s *FeatureFlagService) Reload() {
	s.loadFromDB()
}

// Refresh periodically reloads flags from DB.
func (s *FeatureFlagService) Refresh(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.loadFromDB()
		case <-ctx.Done():
			return
		}
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
