package service

import (
	"database/sql"
	"time"
)

type Notification struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Link      string    `json:"link"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

type NotificationService struct {
	db *sql.DB
}

func NewNotificationService(db *sql.DB) *NotificationService {
	return &NotificationService{db: db}
}

func (s *NotificationService) Create(userID string, notifType, title, message, link string) error {
	_, err := s.db.Exec(
		"INSERT INTO notifications (user_id, type, title, message, link) VALUES (?, ?, ?, ?, ?)",
		userID, notifType, title, message, link,
	)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	NotifyUser(userID, "notification", map[string]interface{}{
		"type": notifType, "title": title, "message": message, "link": link,
	})
	return nil
}

func (s *NotificationService) List(userID string, limit, offset int) ([]Notification, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		"SELECT id, user_id, type, title, message, link, is_read, created_at FROM notifications WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?",
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Notification
	for rows.Next() {
		var n Notification
		var isRead int
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Message, &n.Link, &isRead, &n.CreatedAt); err != nil {
			return nil, err
		}
		n.IsRead = isRead == 1
		list = append(list, n)
	}
	return list, nil
}

func (s *NotificationService) UnreadCount(userID string) (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = ? AND is_read = 0", userID).Scan(&count)
	return count, err
}

func (s *NotificationService) MarkRead(userID string, notifID int64) error {
	_, err := s.db.Exec("UPDATE notifications SET is_read = 1 WHERE id = ? AND user_id = ?", notifID, userID)
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	return nil
}

func (s *NotificationService) MarkAllRead(userID string) error {
	_, err := s.db.Exec("UPDATE notifications SET is_read = 1 WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}

func (s *NotificationService) Delete(userID string, notifID int64) error {
	_, err := s.db.Exec("DELETE FROM notifications WHERE id = ? AND user_id = ?", notifID, userID)
	if err != nil {
		return fmt.Errorf("delete notification: %w", err)
	}
	return nil
}
