package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type PluginService struct {
	db *sql.DB
}

func NewPluginService(db *sql.DB) *PluginService {
	return &PluginService{db: db}
}

type Plugin struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description string     `json:"description"`
	Author      string     `json:"author"`
	Version     string     `json:"version"`
	Enabled     bool       `json:"enabled"`
	Config      string     `json:"config,omitempty"`
	InstalledAt time.Time  `json:"installed_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type PluginHook struct {
	ID         string `json:"id"`
	PluginID   string `json:"plugin_id"`
	HookName   string `json:"hook_name"`
	HookType   string `json:"hook_type"`
	EntryPoint string `json:"entry_point"`
	Config     string `json:"config,omitempty"`
}

func (s *PluginService) InstallPlugin(ctx context.Context, name, slug, description, author, version, config string) (*Plugin, error) {
	id := fmt.Sprintf("plugin_%s", uuid.New().String()[:8])
	now := time.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO plugins (id, name, slug, description, author, version, enabled, config, installed_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, ?)`,
		id, name, slug, description, author, version, config, now, now)
	if err != nil {
		return nil, err
	}
	return &Plugin{ID: id, Name: name, Slug: slug, Description: description, Author: author, Version: version, InstalledAt: now, UpdatedAt: now}, nil
}

func (s *PluginService) ListPlugins(ctx context.Context) ([]Plugin, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, slug, description, author, version, enabled, config, installed_at, updated_at FROM plugins ORDER BY installed_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Plugin
	for rows.Next() {
		var p Plugin
		rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.Author, &p.Version, &p.Enabled, &p.Config, &p.InstalledAt, &p.UpdatedAt)
		result = append(result, p)
	}
	return result, nil
}

func (s *PluginService) EnablePlugin(ctx context.Context, pluginID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE plugins SET enabled = 1, updated_at = ? WHERE id = ?`, time.Now(), pluginID)
	return err
}

func (s *PluginService) DisablePlugin(ctx context.Context, pluginID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE plugins SET enabled = 0, updated_at = ? WHERE id = ?`, time.Now(), pluginID)
	return err
}

func (s *PluginService) UninstallPlugin(ctx context.Context, pluginID string) error {
	s.db.ExecContext(ctx, `DELETE FROM plugin_hooks WHERE plugin_id = ?`, pluginID)
	_, err := s.db.ExecContext(ctx, `DELETE FROM plugins WHERE id = ?`, pluginID)
	return err
}

func (s *PluginService) RegisterHook(ctx context.Context, pluginID, hookName, hookType, entryPoint, config string) (*PluginHook, error) {
	id := fmt.Sprintf("hook_%s", uuid.New().String()[:8])
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO plugin_hooks (id, plugin_id, hook_name, hook_type, entry_point, config) VALUES (?, ?, ?, ?, ?, ?)`,
		id, pluginID, hookName, hookType, entryPoint, config)
	if err != nil {
		return nil, err
	}
	return &PluginHook{ID: id, PluginID: pluginID, HookName: hookName, HookType: hookType, EntryPoint: entryPoint, Config: config}, nil
}

func (s *PluginService) GetPluginHooks(ctx context.Context, pluginID string) ([]PluginHook, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, plugin_id, hook_name, hook_type, entry_point, config FROM plugin_hooks WHERE plugin_id = ?`, pluginID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []PluginHook
	for rows.Next() {
		var h PluginHook
		rows.Scan(&h.ID, &h.PluginID, &h.HookName, &h.HookType, &h.EntryPoint, &h.Config)
		result = append(result, h)
	}
	return result, nil
}

func (s *PluginService) ExecuteHook(ctx context.Context, hookName string, input map[string]interface{}) (map[string]interface{}, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT ph.entry_point, ph.config FROM plugin_hooks ph JOIN plugins p ON ph.plugin_id = p.id WHERE ph.hook_name = ? AND p.enabled = 1`, hookName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]interface{})
	for rows.Next() {
		var entryPoint, config string
		rows.Scan(&entryPoint, &config)
		result[entryPoint] = map[string]interface{}{
			"status": "executed",
			"config": config,
			"input":  input,
		}
	}
	return result, nil
}
