package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/database"
	"github.com/moduforge/backend/internal/domain"
)

type SQLiteMarketService struct {
	db *database.DB
}

func NewSQLiteMarketService(db *database.DB) *SQLiteMarketService {
	return &SQLiteMarketService{db: db}
}

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
		"SELECT id, title, slug, description, category, tags, version, version_code, author, COALESCE(author_uid,''), COALESCE(license,''), stars, installs, created_at, updated_at FROM market_modules %s ORDER BY %s LIMIT ? OFFSET ?",
		whereClause, orderClause,
	)
	args = append(args, perPage, offset)

	rows, err := s.db.Conn.Query(querySQL, args...)
	if err != nil {
		return nil, 0
	}
	defer rows.Close()

	var modules []*domain.MarketModule
	for rows.Next() {
		var m domain.MarketModule
		rows.Scan(&m.ID, &m.Title, &m.Slug, &m.Description, &m.Category, &m.Tags, &m.Version, &m.VersionCode, &m.Author, &m.AuthorUID, &m.License, &m.Stars, &m.Installs, &m.CreatedAt, &m.UpdatedAt)
		modules = append(modules, &m)
	}
	return modules, total
}

func (s *SQLiteMarketService) GetModule(slugOrID string) (*domain.MarketModule, error) {
	row := s.db.Conn.QueryRow(
		"SELECT id, title, slug, description, category, tags, version, version_code, author, COALESCE(author_uid,''), COALESCE(license,''), stars, installs, created_at, updated_at FROM market_modules WHERE slug = ? OR id = ?",
		slugOrID, slugOrID,
	)
	var m domain.MarketModule
	if err := row.Scan(&m.ID, &m.Title, &m.Slug, &m.Description, &m.Category, &m.Tags, &m.Version, &m.VersionCode, &m.Author, &m.AuthorUID, &m.License, &m.Stars, &m.Installs, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return nil, fmt.Errorf("module not found: %w", err)
	}
	return &m, nil
}

func (s *SQLiteMarketService) StarModule(slugOrID string) (int, error) {
	result, err := s.db.Conn.Exec("UPDATE market_modules SET stars = stars + 1 WHERE slug = ? OR id = ?", slugOrID, slugOrID)
	if err != nil {
		return 0, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return 0, fmt.Errorf("module not found")
	}
	var stars int
	s.db.Conn.QueryRow("SELECT stars FROM market_modules WHERE slug = ? OR id = ?", slugOrID, slugOrID).Scan(&stars)
	return stars, nil
}

func (s *SQLiteMarketService) AddReview(moduleID, uid, username string, rating int, comment string) error {
	_, err := s.db.Conn.Exec(
		"INSERT INTO market_reviews (id, module_id, uid, username, rating, comment, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		fmt.Sprintf("rev_%d", time.Now().UnixNano()), moduleID, uid, username, rating, comment, time.Now(),
	)
	return err
}

func (s *SQLiteMarketService) GetReviews(moduleID string) []*domain.MarketReview {
	rows, _ := s.db.Conn.Query(
		"SELECT id, module_id, uid, username, rating, comment, created_at FROM market_reviews WHERE module_id = ? ORDER BY created_at DESC",
		moduleID,
	)
	defer rows.Close()
	var reviews []*domain.MarketReview
	for rows.Next() {
		var r domain.MarketReview
		rows.Scan(&r.ID, &r.ModuleID, &r.UID, &r.Username, &r.Rating, &r.Comment, &r.CreatedAt)
		reviews = append(reviews, &r)
	}
	return reviews
}

func (s *SQLiteMarketService) PublishModule(mod *domain.MarketModule) (*domain.MarketModule, error) {
	mod.ID = fmt.Sprintf("mod_%d", time.Now().UnixMilli())
	mod.Slug = strings.ToLower(strings.ReplaceAll(mod.Title, " ", "-"))
	mod.CreatedAt = time.Now()
	mod.UpdatedAt = time.Now()

	_, err := s.db.Conn.Exec(
		"INSERT INTO market_modules (id, title, slug, description, category, tags, version, version_code, author, author_uid, license, stars, installs, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?)",
		mod.ID, mod.Title, mod.Slug, mod.Description, mod.Category, mod.Tags, mod.Version, mod.VersionCode, mod.Author, mod.AuthorUID, mod.License, mod.CreatedAt, mod.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return mod, nil
}

func (s *SQLiteMarketService) TrendingModules(limit int) []*domain.MarketModule {
	rows, _ := s.db.Conn.Query(
		"SELECT id, title, slug, description, category, tags, version, version_code, author, COALESCE(author_uid,''), COALESCE(license,''), stars, installs, created_at, updated_at FROM market_modules WHERE stars > 100 ORDER BY stars DESC LIMIT ?",
		limit,
	)
	defer rows.Close()
	var modules []*domain.MarketModule
	for rows.Next() {
		var m domain.MarketModule
		rows.Scan(&m.ID, &m.Title, &m.Slug, &m.Description, &m.Category, &m.Tags, &m.Version, &m.VersionCode, &m.Author, &m.AuthorUID, &m.License, &m.Stars, &m.Installs, &m.CreatedAt, &m.UpdatedAt)
		modules = append(modules, &m)
	}
	return modules
}

func (s *SQLiteMarketService) Categories() []string {
	return []string{"system", "ui", "audio", "display", "utility"}
}
