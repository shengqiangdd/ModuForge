package service

import (
	"github.com/moduforge/backend/internal/domain"
)

func (s *SQLiteMarketService) Categories() []string {
	return []string{"system", "ui", "audio", "display", "utility"}
}

func (s *SQLiteMarketService) TrendingModules(limit int) []*domain.MarketModule {
	return s.UpdateTrendingModules(limit)
}

func (s *SQLiteMarketService) UpdateTrendingModules(limit int) []*domain.MarketModule {
	rows, _ := s.db.Conn.Query(
		"SELECT "+moduleColumns+" FROM market_modules WHERE stars > 100 ORDER BY stars DESC LIMIT ?",
		limit,
	)
	defer rows.Close()
	var modules []*domain.MarketModule
	for rows.Next() {
		m, err := scanModule(rows)
		if err != nil {
			continue
		}
		modules = append(modules, m)
	}
	return modules
}
