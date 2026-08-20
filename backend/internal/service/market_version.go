package service

import (
	"fmt"
	"time"

	"github.com/moduforge/backend/internal/domain"
)

func (s *SQLiteMarketService) GetModuleVersions(slug string) ([]*domain.ModuleVersion, error) {
	var modID string
	err := s.db.Conn.QueryRow("SELECT id FROM market_modules WHERE slug = ?", slug).Scan(&modID)
	if err != nil {
		return nil, fmt.Errorf("module not found: %w", err)
	}

	rows, err := s.db.Conn.Query(
		"SELECT id, module_id, version, COALESCE(changelog,''), COALESCE(file_hash,''), COALESCE(file_path,''), created_at FROM module_versions WHERE module_id = ? ORDER BY created_at DESC",
		modID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []*domain.ModuleVersion
	for rows.Next() {
		var v domain.ModuleVersion
		if err := rows.Scan(&v.ID, &v.ModuleID, &v.Version, &v.Changelog, &v.FileHash, &v.FilePath, &v.CreatedAt); err != nil {
			continue
		}
		versions = append(versions, &v)
	}
	return versions, nil
}

func (s *SQLiteMarketService) RollbackModule(slug, version string) (*domain.MarketModule, error) {
	mod, err := s.GetModule(slug)
	if err != nil {
		return nil, err
	}

	var rollbackVersion domain.ModuleVersion
	err = s.db.Conn.QueryRow(
		"SELECT id, module_id, version, COALESCE(changelog,''), COALESCE(file_hash,''), COALESCE(file_path,''), created_at FROM module_versions WHERE module_id = ? AND version = ? ORDER BY created_at DESC LIMIT 1",
		mod.ID, version,
	).Scan(&rollbackVersion.ID, &rollbackVersion.ModuleID, &rollbackVersion.Version, &rollbackVersion.Changelog, &rollbackVersion.FileHash, &rollbackVersion.FilePath, &rollbackVersion.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("version %s not found for module %s", version, slug)
	}

	// Record current version as parent before rollback
	parentID := mod.ID

	// Update module version and changelog
	_, err = s.db.Conn.Exec(
		"UPDATE market_modules SET version = ?, changelog = ?, parent_id = ?, updated_at = ? WHERE id = ?",
		rollbackVersion.Version, rollbackVersion.Changelog, parentID, time.Now(), mod.ID,
	)
	if err != nil {
		return nil, err
	}

	// Create a rollback version record
	if _, err = s.db.Conn.Exec(
		"INSERT INTO module_versions (module_id, version, changelog, created_at) VALUES (?, ?, ?, ?)",
		mod.ID, rollbackVersion.Version, "Rolled back to "+rollbackVersion.Version, time.Now(),
	); err != nil {
		return nil, err
	}

	return s.GetModule(slug)
}

func (s *SQLiteMarketService) UpdateModuleVersion(slug, version, changelog string) (*domain.MarketModule, error) {
	mod, err := s.GetModule(slug)
	if err != nil {
		return nil, err
	}

	mod.ParentID = mod.ID
	mod.Version = version
	mod.Changelog = changelog
	mod.UpdatedAt = time.Now()

	_, err = s.db.Conn.Exec(
		"UPDATE market_modules SET version = ?, changelog = ?, parent_id = ?, updated_at = ? WHERE id = ?",
		mod.Version, mod.Changelog, mod.ParentID, mod.UpdatedAt, mod.ID,
	)
	if err != nil {
		return nil, err
	}

	// Create version record
	_, err = s.db.Conn.Exec(
		"INSERT INTO module_versions (module_id, version, changelog, created_at) VALUES (?, ?, ?, ?)",
		mod.ID, mod.Version, mod.Changelog, mod.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return mod, nil
}
