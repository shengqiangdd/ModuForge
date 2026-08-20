package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/moduforge/backend/internal/domain"
)

type CollaborationService struct {
	db *sql.DB
}

func NewCollaborationService(db *sql.DB) *CollaborationService {
	return &CollaborationService{db: db}
}

func (s *CollaborationService) GetDB() *sql.DB { return s.db }

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
	if err != nil {
		return fmt.Errorf("remove collaborator: %w", err)
	}
	return nil
}

func (s *CollaborationService) AddComment(ctx context.Context, projectID, userID, username, filePath, content string, lineNumber int) (*Comment, error) {
	id := fmt.Sprintf("comment_%s", uuid.New().String()[:8])
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO comments (id, project_id, user_id, username, file_path, line_number, content, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, projectID, userID, username, filePath, lineNumber, content, time.Now())
	if err != nil {
		return nil, fmt.Errorf("add comment: %w", err)
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
		return nil, fmt.Errorf("list comments: %w", err)
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
	if err != nil {
		return fmt.Errorf("resolve comment: %w", err)
	}
	return nil
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
	if err != nil {
		return fmt.Errorf("upsert edit session: %w", err)
	}
	return nil
}

func (s *CollaborationService) ListEditSessions(ctx context.Context, projectID string) ([]EditSession, error) {
	s.db.ExecContext(ctx, `DELETE FROM edit_sessions WHERE project_id = ? AND last_active < ?`,
		projectID, time.Now().Add(-5*time.Minute))

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, user_id, username, file_path, cursor_line, cursor_col, selection_start_line, selection_start_col, selection_end_line, selection_end_col, color, connected_at, last_active
		 FROM edit_sessions WHERE project_id = ?`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list edit sessions: %w", err)
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
	if err != nil {
		return fmt.Errorf("remove edit session: %w", err)
	}
	return nil
}

// ===== Team Members =====

func (s *CollaborationService) AddTeamMember(ctx context.Context, projectID, userID, role, invitedBy string) (*domain.TeamMember, error) {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO team_members (project_id, user_id, role, invited_by) VALUES (?, ?, ?, ?)`,
		projectID, userID, role, invitedBy)
	if err != nil {
		return nil, fmt.Errorf("add team member: %w", err)
	}
	var m domain.TeamMember
	err = s.db.QueryRowContext(ctx,
		`SELECT id, project_id, user_id, role, COALESCE(invited_by,''), created_at FROM team_members WHERE project_id=? AND user_id=?`,
		projectID, userID,
	).Scan(&m.ID, &m.ProjectID, &m.UserID, &m.Role, &m.InvitedBy, &m.CreatedAt)
	return &m, err
}

func (s *CollaborationService) RemoveTeamMember(ctx context.Context, projectID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM team_members WHERE project_id = ? AND user_id = ?`, projectID, userID)
	if err != nil {
		return fmt.Errorf("remove team member: %w", err)
	}
	return nil
}

func (s *CollaborationService) GetTeamMembers(ctx context.Context, projectID string) ([]domain.TeamMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, user_id, role, COALESCE(invited_by,''), created_at FROM team_members WHERE project_id=?`, projectID)
	if err != nil {
		return nil, fmt.Errorf("get team members: %w", err)
	}
	defer rows.Close()
	var result []domain.TeamMember
	for rows.Next() {
		var m domain.TeamMember
		rows.Scan(&m.ID, &m.ProjectID, &m.UserID, &m.Role, &m.InvitedBy, &m.CreatedAt)
		result = append(result, m)
	}
	return result, nil
}

func (s *CollaborationService) UpdateMemberRole(ctx context.Context, projectID, userID, role string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE team_members SET role=? WHERE project_id=? AND user_id=?`, role, projectID, userID)
	if err != nil {
		return fmt.Errorf("update member role: %w", err)
	}
	return nil
}

// role hierarchy: owner > admin > member > viewer
var roleWeight = map[string]int{
	"owner":  4,
	"admin":  3,
	"member": 2,
	"viewer": 1,
}

func (s *CollaborationService) CheckPermission(ctx context.Context, projectID, userID string, minRole string) bool {
	// Project owner always has full access
	var ownerID string
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM projects WHERE id=?`, projectID).Scan(&ownerID)
	if err == nil && ownerID == userID {
		return true
	}

	var role string
	err = s.db.QueryRowContext(ctx,
		`SELECT role FROM team_members WHERE project_id=? AND user_id=?`, projectID, userID).Scan(&role)
	if err != nil {
		return false
	}
	return roleWeight[role] >= roleWeight[minRole]
}

// ===== Audit Log =====

func (s *CollaborationService) LogAudit(ctx context.Context, projectID, userID, action, details string) {
	s.db.ExecContext(ctx,
		`INSERT INTO audit_logs (project_id, user_id, action, details) VALUES (?, ?, ?, ?)`,
		projectID, userID, action, details)
}

func (s *CollaborationService) GetAuditLogs(ctx context.Context, projectID string, limit, offset int) ([]domain.AuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, COALESCE(project_id,''), COALESCE(user_id,''), action, details, created_at FROM audit_logs WHERE project_id=? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		projectID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.AuditLog
	for rows.Next() {
		var a domain.AuditLog
		rows.Scan(&a.ID, &a.ProjectID, &a.UserID, &a.Action, &a.Details, &a.CreatedAt)
		result = append(result, a)
	}
	return result, nil
}
