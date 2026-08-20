package service

import (
	"fmt"

	"github.com/moduforge/backend/internal/domain"
)

func (s *SQLiteMarketService) AddScreenshot(slug, url string) (*domain.ModuleScreenshot, error) {
	mod, err := s.GetModule(slug)
	if err != nil {
		return nil, err
	}

	// Check count
	var count int
	s.db.Conn.QueryRow("SELECT COUNT(*) FROM module_screenshots WHERE module_id = ?", mod.ID).Scan(&count)
	if count >= 5 {
		return nil, fmt.Errorf("maximum 5 screenshots allowed")
	}

	var maxOrder int
	s.db.Conn.QueryRow("SELECT COALESCE(MAX(sort_order), -1) FROM module_screenshots WHERE module_id = ?", mod.ID).Scan(&maxOrder)

	result, err := s.db.Conn.Exec(
		"INSERT INTO module_screenshots (module_id, url, sort_order) VALUES (?, ?, ?)",
		mod.ID, url, maxOrder+1,
	)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &domain.ModuleScreenshot{ID: id, ModuleID: mod.ID, URL: url, SortOrder: maxOrder + 1}, nil
}

func (s *SQLiteMarketService) GetScreenshots(slug string) ([]*domain.ModuleScreenshot, error) {
	mod, err := s.GetModule(slug)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Conn.Query(
		"SELECT id, module_id, url, sort_order, created_at FROM module_screenshots WHERE module_id = ? ORDER BY sort_order ASC",
		mod.ID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*domain.ModuleScreenshot
	for rows.Next() {
		var ss domain.ModuleScreenshot
		if err := rows.Scan(&ss.ID, &ss.ModuleID, &ss.URL, &ss.SortOrder, &ss.CreatedAt); err != nil {
			continue
		}
		list = append(list, &ss)
	}
	return list, nil
}

func (s *SQLiteMarketService) DeleteScreenshot(id int64) error {
	_, err := s.db.Conn.Exec("DELETE FROM module_screenshots WHERE id = ?", id)
	return err
}

func (s *SQLiteMarketService) GetScreenshotsByModuleID(moduleID string) ([]*domain.ModuleScreenshot, error) {
	rows, err := s.db.Conn.Query(
		"SELECT id, module_id, url, sort_order, created_at FROM module_screenshots WHERE module_id = ? ORDER BY sort_order ASC",
		moduleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*domain.ModuleScreenshot
	for rows.Next() {
		var ss domain.ModuleScreenshot
		if err := rows.Scan(&ss.ID, &ss.ModuleID, &ss.URL, &ss.SortOrder, &ss.CreatedAt); err != nil {
			continue
		}
		list = append(list, &ss)
	}
	return list, nil
}
