package service

import (
	"fmt"
	"time"

	"github.com/moduforge/backend/internal/domain"
)

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
