package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type CollaborationService struct {
	db *sql.DB
}

func NewCollaborationService(db *sql.DB) *CollaborationService {
	return &CollaborationService{db: db}
}

type Collaborator struct {
	ID        string     `json:"id"`
	ProjectID string     `json:"project_id"`
	UserID    string     `json:"user_id"`
	Username  string     `json:"username"`
	Role      string     `json:"role"`
	InvitedAt time.Time  `json:"invited_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
}

type Comment struct {
	ID         string     `json:"id"`
	ProjectID  string     `json:"project_id"`
	UserID     string     `json:"user_id"`
	Username   string     `json:"username"`
	FilePath   string     `json:"file_path"`
	LineNumber int        `json:"line_number"`
	Content    string     `json:"content"`
	Resolved   bool       `json:"resolved"`
	CreatedAt  time.Time  `json:"created_at"`
}

type EditSession struct {
	ID                  string     `json:"id"`
	ProjectID           string     `json:"project_id"`
	UserID              string     `json:"user_id"`
	Username            string     `json:"username"`
	FilePath            string     `json:"file_path"`
	CursorLine          int        `json:"cursor_line"`
	CursorCol           int        `json:"cursor_col"`
	SelectionStartLine  int        `json:"selection_start_line"`
	SelectionStartCol   int        `json:"selection_start_col"`
	SelectionEndLine    int        `json:"selection_end_line"`
	SelectionEndCol     int        `json:"selection_end_col"`
	Color               string     `json:"color"`
	ConnectedAt         time.Time  `json:"connected_at"`
	LastActive          time.Time  `json:"last_active"`
}

func (s *CollaborationService) AddCollaborator(ctx context.Context, projectID, userID, role string) (*Collaborator, error) {
	id := fmt.Sprintf("collab_%s", uuid.New().String()[:8])
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO collaborators (id, project_id, user_id, role, invited_at) VALUES (?, ?, ?, ?, ?)`,
		id, projectID, userID, role, time.Now())
	if err != nil {
		return nil, err
	}
	return &Collaborator{ID: id, ProjectID: projectID, UserID: userID, Role: role, InvitedAt: time.Now()}, nil
}

func (s *CollaborationService) ListCollaborators(ctx context.Context, projectID string) ([]Collaborator, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.project_id, c.user_id, COALESCE(u.username, c.user_id), c.role, c.invited_at, c.accepted_at
		 FROM collaborators c LEFT JOIN users u ON c.user_id = u.id
		 WHERE c.project_id = ?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Collaborator
	for rows.Next() {
		var c Collaborator
		rows.Scan(&c.ID, &c.ProjectID, &c.UserID, &c.Username, &c.Role, &c.InvitedAt, &c.AcceptedAt)
		result = append(result, c)
	}
	return result, nil
}

func (s *CollaborationService) RemoveCollaborator(ctx context.Context, projectID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM collaborators WHERE project_id = ? AND user_id = ?`, projectID, userID)
	return err
}

func (s *CollaborationService) AddComment(ctx context.Context, projectID, userID, username, filePath, content string, lineNumber int) (*Comment, error) {
	id := fmt.Sprintf("comment_%s", uuid.New().String()[:8])
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO comments (id, project_id, user_id, username, file_path, line_number, content, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, projectID, userID, username, filePath, lineNumber, content, time.Now())
	if err != nil {
		return nil, err
	}
	return &Comment{ID: id, ProjectID: projectID, UserID: userID, Username: username, FilePath: filePath, LineNumber: lineNumber, Content: content, CreatedAt: time.Now()}, nil
}

func (s *CollaborationService) ListComments(ctx context.Context, projectID, filePath string) ([]Comment, error) {
	var rows *sql.Rows
	var err error
	if filePath != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, project_id, user_id, username, file_path, line_number, content, resolved, created_at
			 FROM comments WHERE project_id = ? AND file_path = ? ORDER BY created_at DESC`, projectID, filePath)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, project_id, user_id, username, file_path, line_number, content, resolved, created_at
			 FROM comments WHERE project_id = ? ORDER BY created_at DESC`, projectID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Comment
	for rows.Next() {
		var c Comment
		rows.Scan(&c.ID, &c.ProjectID, &c.UserID, &c.Username, &c.FilePath, &c.LineNumber, &c.Content, &c.Resolved, &c.CreatedAt)
		result = append(result, c)
	}
	return result, nil
}

func (s *CollaborationService) ResolveComment(ctx context.Context, commentID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE comments SET resolved = 1 WHERE id = ?`, commentID)
	return err
}

func (s *CollaborationService) UpsertEditSession(ctx context.Context, session *EditSession) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO edit_sessions (id, project_id, user_id, username, file_path, cursor_line, cursor_col, selection_start_line, selection_start_col, selection_end_line, selection_end_col, color, connected_at, last_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET file_path=excluded.file_path, cursor_line=excluded.cursor_line, cursor_col=excluded.cursor_col,
		 selection_start_line=excluded.selection_start_line, selection_start_col=excluded.selection_start_col,
		 selection_end_line=excluded.selection_end_line, selection_end_col=excluded.selection_end_col, last_active=excluded.last_active`,
		session.ID, session.ProjectID, session.UserID, session.Username, session.FilePath,
		session.CursorLine, session.CursorCol, session.SelectionStartLine, session.SelectionStartCol,
		session.SelectionEndLine, session.SelectionEndCol, session.Color, session.ConnectedAt, time.Now())
	return err
}

func (s *CollaborationService) ListEditSessions(ctx context.Context, projectID string) ([]EditSession, error) {
	s.db.ExecContext(ctx, `DELETE FROM edit_sessions WHERE project_id = ? AND last_active < ?`,
		projectID, time.Now().Add(-5*time.Minute))

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, user_id, username, file_path, cursor_line, cursor_col, selection_start_line, selection_start_col, selection_end_line, selection_end_col, color, connected_at, last_active
		 FROM edit_sessions WHERE project_id = ?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []EditSession
	for rows.Next() {
		var e EditSession
		rows.Scan(&e.ID, &e.ProjectID, &e.UserID, &e.Username, &e.FilePath, &e.CursorLine, &e.CursorCol, &e.SelectionStartLine, &e.SelectionStartCol, &e.SelectionEndLine, &e.SelectionEndCol, &e.Color, &e.ConnectedAt, &e.LastActive)
		result = append(result, e)
	}
	return result, nil
}

func (s *CollaborationService) RemoveEditSession(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM edit_sessions WHERE id = ?`, sessionID)
	return err
}
