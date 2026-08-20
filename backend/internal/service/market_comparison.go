package service

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/moduforge/backend/internal/domain"
)

func (s *SQLiteMarketService) CompareModules(slug1, slug2 string) (*domain.ModuleComparison, error) {
	m1, err := s.GetModule(slug1)
	if err != nil {
		return nil, fmt.Errorf("module %s not found", slug1)
	}
	m2, err := s.GetModule(slug2)
	if err != nil {
		return nil, fmt.Errorf("module %s not found", slug2)
	}

	getDepCount := func(depsJSON string) int {
		if depsJSON == "" || depsJSON == "[]" {
			return 0
		}
		var deps []domain.ModuleDependency
		if err := json.Unmarshal([]byte(depsJSON), &deps); err != nil {
			return 0
		}
		return len(deps)
	}

	getAvgRating := func(moduleID string) float64 {
		var avg sql.NullFloat64
		s.db.Conn.QueryRow("SELECT AVG(rating) FROM market_reviews WHERE module_id = ?", moduleID).Scan(&avg)
		if avg.Valid {
			return avg.Float64
		}
		return 0
	}

	return &domain.ModuleComparison{
		TitleA:       m1.Title,
		TitleB:       m2.Title,
		DescriptionA: m1.Description,
		DescriptionB: m2.Description,
		VersionA:     m1.Version,
		VersionB:     m2.Version,
		StarsA:       m1.Stars,
		StarsB:       m2.Stars,
		InstallsA:    m1.Installs,
		InstallsB:    m2.Installs,
		CategoryA:    m1.Category,
		CategoryB:    m2.Category,
		AuthorA:      m1.Author,
		AuthorB:      m2.Author,
		LicenseA:     m1.License,
		LicenseB:     m2.License,
		DepCountA:    getDepCount(m1.Dependencies),
		DepCountB:    getDepCount(m2.Dependencies),
		RatingA:      getAvgRating(m1.ID),
		RatingB:      getAvgRating(m2.ID),
	}, nil
}
