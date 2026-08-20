package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/domain"
)

func (s *SQLiteMarketService) ListModules(query, category, sort string, page, perPage int) ([]*domain.MarketModule, int) {
	where := []string{}
	args := []interface{}{}

	if category != "" {
		where = append(where, "category = ?")
		args = append(args, category)
	}
	if query != "" {
		where = append(where, "(title LIKE ? OR description LIKE ? OR tags LIKE ?)")
		q := "%" + query + "%"
		args = append(args, q, q, q)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	orderClause := "stars DESC"
	switch sort {
	case "installs":
		orderClause = "installs DESC"
	case "newest":
		orderClause = "created_at DESC"
	case "title":
		orderClause = "title ASC"
	}

	countQuery := "SELECT COUNT(*) FROM market_modules " + whereClause
	var total int
	s.db.Conn.QueryRow(countQuery, args...).Scan(&total)

	offset := (page - 1) * perPage
	querySQL := fmt.Sprintf(
		"SELECT %s FROM market_modules %s ORDER BY %s LIMIT ? OFFSET ?",
		moduleColumns, whereClause, orderClause,
	)
	args = append(args, perPage, offset)

	rows, err := s.db.Conn.Query(querySQL, args...)
	if err != nil {
		return nil, 0
	}
	defer rows.Close()

	var modules []*domain.MarketModule
	for rows.Next() {
		m, err := scanModule(rows)
		if err != nil {
			continue
		}
		modules = append(modules, m)
	}
	return modules, total
}

func (s *SQLiteMarketService) GetModule(slugOrID string) (*domain.MarketModule, error) {
	return scanModule(s.db.Conn.QueryRow(
		"SELECT "+moduleColumns+" FROM market_modules WHERE slug = ? OR id = ?",
		slugOrID, slugOrID,
	))
}

func (s *SQLiteMarketService) PublishModule(mod *domain.MarketModule) (*domain.MarketModule, error) {
	mod.ID = fmt.Sprintf("mod_%d", time.Now().UnixMilli())
	mod.Slug = strings.ToLower(strings.ReplaceAll(mod.Title, " ", "-"))
	mod.CreatedAt = time.Now()
	mod.UpdatedAt = time.Now()

	if mod.Dependencies == "" {
		mod.Dependencies = "[]"
	}
	_, err := s.db.Conn.Exec(
		"INSERT INTO market_modules (id, title, slug, description, category, tags, version, version_code, changelog, parent_id, author, author_uid, license, dependencies, cover_image, stars, installs, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?)",
		mod.ID, mod.Title, mod.Slug, mod.Description, mod.Category, mod.Tags, mod.Version, mod.VersionCode, mod.Changelog, mod.ParentID, mod.Author, mod.AuthorUID, mod.License, mod.Dependencies, mod.CoverImage, mod.CreatedAt, mod.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Create initial version record
	_, err = s.db.Conn.Exec(
		"INSERT INTO module_versions (module_id, version, changelog, created_at) VALUES (?, ?, ?, ?)",
		mod.ID, mod.Version, mod.Changelog, mod.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return mod, nil
}

func (s *SQLiteMarketService) UpdateModuleCoverImage(slug, coverURL string) error {
	_, err := s.db.Conn.Exec("UPDATE market_modules SET cover_image = ? WHERE slug = ?", coverURL, slug)
	return err
}
