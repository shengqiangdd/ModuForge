package service

import (
	"database/sql"
	"fmt"
	"time"
)

type AnalyticsService struct {
	db *sql.DB
}

func NewAnalyticsService(db *sql.DB) *AnalyticsService {
	return &AnalyticsService{db: db}
}

type BuildStats struct {
	TotalBuilds      int     `json:"total_builds"`
	SuccessfulBuilds int     `json:"successful_builds"`
	FailedBuilds     int     `json:"failed_builds"`
	AvgDurationMs    float64 `json:"avg_duration_ms"`
	SuccessRate      float64 `json:"success_rate"`
}

type BuildTrend struct {
	Date    string `json:"date"`
	Count   int    `json:"count"`
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
}

type ModuleStats struct {
	TotalModules  int             `json:"total_modules"`
	TotalInstalls int             `json:"total_installs"`
	TotalStars    int             `json:"total_stars"`
	TopCategories []CategoryCount `json:"top_categories"`
}

type CategoryCount struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

type SystemStats struct {
	Projects     int    `json:"projects"`
	Users        int    `json:"users"`
	TotalBuilds  int    `json:"total_builds"`
	TotalModules int    `json:"total_modules"`
	Uptime       string `json:"uptime"`
	DBSize       string `json:"db_size"`
}

var startTime = time.Now()

func (s *AnalyticsService) GetBuildStats() (*BuildStats, error) {
	stats := &BuildStats{}

	s.db.QueryRow("SELECT COUNT(*) FROM build_tasks").Scan(&stats.TotalBuilds)
	s.db.QueryRow("SELECT COUNT(*) FROM build_tasks WHERE status = 'success'").Scan(&stats.SuccessfulBuilds)
	s.db.QueryRow("SELECT COUNT(*) FROM build_tasks WHERE status = 'failed'").Scan(&stats.FailedBuilds)

	if stats.TotalBuilds > 0 {
		stats.SuccessRate = float64(stats.SuccessfulBuilds) / float64(stats.TotalBuilds) * 100
	}

	s.db.QueryRow(`SELECT COALESCE(AVG(CAST((julianday(updated_at) - julianday(created_at)) * 86400000 AS INTEGER)), 0)
		FROM build_tasks WHERE status = 'success'`).Scan(&stats.AvgDurationMs)

	return stats, nil
}

func (s *AnalyticsService) GetBuildTrends(days int) ([]BuildTrend, error) {
	if days <= 0 {
		days = 30
	}
	rows, err := s.db.Query(`SELECT DATE(created_at) as d, COUNT(*),
		SUM(CASE WHEN status='success' THEN 1 ELSE 0 END),
		SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END)
		FROM build_tasks
		WHERE created_at >= DATE('now', ?)
		GROUP BY d ORDER BY d`,
		fmt.Sprintf("-%d days", days))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trends []BuildTrend
	for rows.Next() {
		var t BuildTrend
		rows.Scan(&t.Date, &t.Count, &t.Success, &t.Failed)
		trends = append(trends, t)
	}
	return trends, nil
}

func (s *AnalyticsService) GetModuleStats() (*ModuleStats, error) {
	stats := &ModuleStats{}

	s.db.QueryRow("SELECT COUNT(*) FROM market_modules").Scan(&stats.TotalModules)
	s.db.QueryRow("SELECT COALESCE(SUM(installs), 0) FROM market_modules").Scan(&stats.TotalInstalls)
	s.db.QueryRow("SELECT COALESCE(SUM(stars), 0) FROM market_modules").Scan(&stats.TotalStars)

	rows, _ := s.db.Query("SELECT category, COUNT(*) as cnt FROM market_modules GROUP BY category ORDER BY cnt DESC LIMIT 10")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var c CategoryCount
			rows.Scan(&c.Category, &c.Count)
			stats.TopCategories = append(stats.TopCategories, c)
		}
	}

	return stats, nil
}

func (s *AnalyticsService) GetSystemStats() (*SystemStats, error) {
	stats := &SystemStats{}

	s.db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&stats.Projects)
	s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.Users)
	s.db.QueryRow("SELECT COUNT(*) FROM build_tasks").Scan(&stats.TotalBuilds)
	s.db.QueryRow("SELECT COUNT(*) FROM market_modules").Scan(&stats.TotalModules)

	uptime := time.Since(startTime)
	if uptime < time.Minute {
		stats.Uptime = fmt.Sprintf("%.0fs", uptime.Seconds())
	} else if uptime < time.Hour {
		stats.Uptime = fmt.Sprintf("%.0fm", uptime.Minutes())
	} else {
		stats.Uptime = fmt.Sprintf("%.0fh%dm", uptime.Hours(), int(uptime.Minutes())%60)
	}

	var size int64
	s.db.QueryRow("SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()").Scan(&size)
	if size > 1024*1024 {
		stats.DBSize = fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	} else if size > 1024 {
		stats.DBSize = fmt.Sprintf("%.1f KB", float64(size)/1024)
	} else {
		stats.DBSize = fmt.Sprintf("%d B", size)
	}

	return stats, nil
}
