package service

import (
	"database/sql"
	"time"
)

type BadgeDefinition struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type UserBadge struct {
	ID       int64     `json:"id"`
	UserID   string    `json:"user_id"`
	BadgeKey string    `json:"badge_key"`
	EarnedAt time.Time `json:"earned_at"`
}

type BadgeService struct {
	db *sql.DB
}

func NewBadgeService(db *sql.DB) *BadgeService {
	return &BadgeService{db: db}
}

func (s *BadgeService) GetDefinitions() []BadgeDefinition {
	return []BadgeDefinition{
		{Key: "first_module", Name: "初出茅庐", Description: "发布第一个模块", Icon: "rocket_launch"},
		{Key: "module_master", Name: "模块大师", Description: "发布 10 个模块", Icon: "auto_awesome"},
		{Key: "reviewer", Name: "评论达人", Description: "评价 5 个模块", Icon: "rate_review"},
		{Key: "explorer", Name: "探索者", Description: "访问所有页面", Icon: "explore"},
		{Key: "active_user", Name: "活跃用户", Description: "连续登录 7 天", Icon: "local_fire_department"},
		{Key: "api_user", Name: "API 开发者", Description: "创建第一个 API 密钥", Icon: "vpn_key"},
		{Key: "team_player", Name: "团队协作", Description: "加入 3 个项目", Icon: "groups"},
		{Key: "beta_tester", Name: "早期体验者", Description: "完成注册", Icon: "science"},
	}
}

func (s *BadgeService) GetUserBadges(userID string) ([]UserBadge, error) {
	rows, err := s.db.Query("SELECT id, user_id, badge_key, earned_at FROM user_badges WHERE user_id = ? ORDER BY earned_at DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var badges []UserBadge
	for rows.Next() {
		var b UserBadge
		if err := rows.Scan(&b.ID, &b.UserID, &b.BadgeKey, &b.EarnedAt); err != nil {
			continue
		}
		badges = append(badges, b)
	}
	return badges, nil
}

func (s *BadgeService) CheckAndAward(userID string) ([]BadgeDefinition, error) {
	var awarded []BadgeDefinition
	defs := s.GetDefinitions()

	for _, d := range defs {
		var count int
		s.db.QueryRow("SELECT COUNT(*) FROM user_badges WHERE user_id = ? AND badge_key = ?", userID, d.Key).Scan(&count)
		if count > 0 {
			continue
		}

		earn := false
		switch d.Key {
		case "beta_tester":
			var emailVerified int
			s.db.QueryRow("SELECT email_verified FROM users WHERE id = ?", userID).Scan(&emailVerified)
			earn = true
		case "api_user":
			var keyCount int
			s.db.QueryRow("SELECT COUNT(*) FROM api_keys WHERE user_id = ?", userID).Scan(&keyCount)
			earn = keyCount > 0
		case "first_module":
			var modCount int
			s.db.QueryRow("SELECT COUNT(*) FROM market_modules WHERE author_uid = ?", userID).Scan(&modCount)
			earn = modCount >= 1
		case "module_master":
			var modCount int
			s.db.QueryRow("SELECT COUNT(*) FROM market_modules WHERE author_uid = ?", userID).Scan(&modCount)
			earn = modCount >= 10
		case "reviewer":
			var revCount int
			s.db.QueryRow("SELECT COUNT(*) FROM market_reviews WHERE uid = ?", userID).Scan(&revCount)
			earn = revCount >= 5
		case "team_player":
			var teamCount int
			s.db.QueryRow("SELECT COUNT(*) FROM team_members WHERE user_id = ?", userID).Scan(&teamCount)
			earn = teamCount >= 3
		case "explorer", "active_user":
			earn = false
		}

		if earn {
			s.db.Exec("INSERT OR IGNORE INTO user_badges (user_id, badge_key, earned_at) VALUES (?, ?, ?)", userID, d.Key, time.Now())
			awarded = append(awarded, d)
		}
	}
	return awarded, nil
}

func (s *BadgeService) AwardBadge(userID string, badgeKey string) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO user_badges (user_id, badge_key, earned_at) VALUES (?, ?, ?)", userID, badgeKey, time.Now())
	return err
}
