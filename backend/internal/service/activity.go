package service

import (
	"database/sql"
	"time"
)

type Activity struct {
	ID           int64     `json:"id"`
	UserID       string    `json:"user_id"`
	ProjectID    int64     `json:"project_id,omitempty"`
	ActivityType string    `json:"activity_type"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
}

type ActivityService struct {
	db *sql.DB
}

func NewActivityService(db *sql.DB) *ActivityService {
	return &ActivityService{db: db}
}

func (s *ActivityService) Log(userID string, projectID int64, activityType, description string) error {
	var pid interface{}
	if projectID > 0 {
		pid = projectID
	}
	_, err := s.db.Exec(
		"INSERT INTO activities (user_id, project_id, activity_type, description) VALUES (?, ?, ?, ?)",
		userID, pid, activityType, description,
	)
	return err
}

func (s *ActivityService) GetProjectActivities(projectID int64, limit, offset int) ([]Activity, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		"SELECT id, user_id, project_id, activity_type, description, created_at FROM activities WHERE project_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?",
		projectID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Activity
	for rows.Next() {
		var a Activity
		var pid sql.NullInt64
		if err := rows.Scan(&a.ID, &a.UserID, &pid, &a.ActivityType, &a.Description, &a.CreatedAt); err != nil {
			return nil, err
		}
		if pid.Valid {
			a.ProjectID = pid.Int64
		}
		list = append(list, a)
	}
	return list, nil
}

func (s *ActivityService) GetUserActivities(userID string, limit, offset int) ([]Activity, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		"SELECT id, user_id, project_id, activity_type, description, created_at FROM activities WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?",
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Activity
	for rows.Next() {
		var a Activity
		var pid sql.NullInt64
		if err := rows.Scan(&a.ID, &a.UserID, &pid, &a.ActivityType, &a.Description, &a.CreatedAt); err != nil {
			return nil, err
		}
		if pid.Valid {
			a.ProjectID = pid.Int64
		}
		list = append(list, a)
	}
	return list, nil
}
