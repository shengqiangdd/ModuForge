package service

import (
	"time"
)

type HealthScore struct {
	Score    int            `json:"score"`
	Level    string         `json:"level"`
	Details  []HealthDetail `json:"details"`
}

type HealthDetail struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Score int    `json:"score"`
	Max   int    `json:"max"`
}

func (s *SQLiteMarketService) GetModuleHealth(slug string) (*HealthScore, error) {
	mod, err := s.GetModule(slug)
	if err != nil {
		return nil, err
	}

	hs := &HealthScore{Details: []HealthDetail{}}
	updatedAt := mod.UpdatedAt
	daysSinceUpdate := int(time.Since(updatedAt).Hours() / 24)

	updateScore := 0
	switch {
	case daysSinceUpdate <= 30:
		updateScore = 20
	case daysSinceUpdate <= 90:
		updateScore = 10
	}
	hs.Details = append(hs.Details, HealthDetail{Name: "update", Label: "更新时间", Score: updateScore, Max: 20})

	var reviewCount int
	var avgRating float64
	s.db.Conn.QueryRow("SELECT COUNT(*), COALESCE(AVG(rating), 0) FROM market_reviews WHERE module_id = ?", mod.ID).Scan(&reviewCount, &avgRating)

	reviewScore := reviewCount / 5 * 5
	if reviewScore > 20 {
		reviewScore = 20
	}
	hs.Details = append(hs.Details, HealthDetail{Name: "reviews", Label: "评价数量", Score: reviewScore, Max: 20})

	ratingScore := 0
	switch {
	case avgRating >= 4.5:
		ratingScore = 20
	case avgRating >= 4.0:
		ratingScore = 15
	case avgRating >= 3.0:
		ratingScore = 10
	}
	hs.Details = append(hs.Details, HealthDetail{Name: "rating", Label: "评价均分", Score: ratingScore, Max: 20})

	installScore := 0
	if mod.Installs > 100 {
		installScore = 10
	} else if mod.Installs > 50 {
		installScore = 5
	}
	hs.Details = append(hs.Details, HealthDetail{Name: "installs", Label: "安装量", Score: installScore, Max: 10})

	depScore := 10
	deps, _ := s.GetModuleDependencies(slug)
	for _, dep := range deps {
		depMod, depErr := s.GetModule(dep.ID)
		if depErr != nil {
			depScore -= 5
			continue
		}
		depDays := int(time.Since(depMod.UpdatedAt).Hours() / 24)
		if depDays > 365 {
			depScore -= 5
		}
	}
	if depScore < 0 {
		depScore = 0
	}
	hs.Details = append(hs.Details, HealthDetail{Name: "dependencies", Label: "依赖健康", Score: depScore, Max: 10})

	total := updateScore + reviewScore + ratingScore + installScore + depScore
	if total > 100 {
		total = 100
	}
	hs.Score = total
	switch {
	case total >= 80:
		hs.Level = "excellent"
	case total >= 60:
		hs.Level = "good"
	default:
		hs.Level = "warning"
	}
	return hs, nil
}

func (s *SQLiteMarketService) GetModuleHealthDetails(slug string) (*HealthScore, error) {
	return s.GetModuleHealth(slug)
}
