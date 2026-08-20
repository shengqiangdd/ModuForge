package service

import (
	"github.com/moduforge/backend/internal/domain"
)

func (s *SQLiteMarketService) SetModuleTags(slug string, tagIDs []int) error {
	tx, err := s.db.Conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.Exec("DELETE FROM module_tag_relations WHERE module_slug = ?", slug)
	for _, tagID := range tagIDs {
		tx.Exec("INSERT OR IGNORE INTO module_tag_relations (module_slug, tag_id) VALUES (?, ?)", slug, tagID)
		tx.Exec("UPDATE module_tags SET usage_count = (SELECT COUNT(*) FROM module_tag_relations WHERE tag_id = ?) WHERE id = ?", tagID, tagID)
	}
	return tx.Commit()
}

func (s *SQLiteMarketService) GetModuleTags(slug string) ([]domain.ModuleTag, error) {
	rows, err := s.db.Conn.Query(
		"SELECT t.id, t.name, t.color, t.usage_count FROM module_tags t INNER JOIN module_tag_relations r ON t.id = r.tag_id WHERE r.module_slug = ?",
		slug,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []domain.ModuleTag
	for rows.Next() {
		var t domain.ModuleTag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.UsageCount); err != nil {
			continue
		}
		tags = append(tags, t)
	}
	return tags, nil
}
