package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/moduforge/backend/internal/domain"
)

type ProjectService struct {
	db *sql.DB
}

func NewProjectService(db *sql.DB) *ProjectService {
	return &ProjectService{db: db}
}

func (s *ProjectService) List(ctx context.Context, userID string) ([]domain.Project, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, module_type, description, created_at, updated_at
		 FROM projects WHERE user_id = ? AND deleted_at IS NULL ORDER BY updated_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []domain.Project
	for rows.Next() {
		var p domain.Project
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.ModuleType, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, nil
}

func (s *ProjectService) Create(ctx context.Context, userID string, req *domain.CreateProjectInput) (*domain.Project, error) {
	var p domain.Project
	p.ID = uuid.New().String()
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO projects (id, user_id, name, module_type, description)
		 VALUES (?, ?, ?, ?, ?)
		 RETURNING id, user_id, name, module_type, description, created_at, updated_at`,
		p.ID, userID, req.Name, "universal", req.Description,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.ModuleType, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return &p, nil
}

func (s *ProjectService) Get(ctx context.Context, id string) (*domain.Project, error) {
	var p domain.Project
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, module_type, description, created_at, updated_at
		 FROM projects WHERE id = ? AND deleted_at IS NULL`, id,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.ModuleType, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("project not found")
	}
	return &p, nil
}

func (s *ProjectService) Update(ctx context.Context, id string, req *domain.UpdateProjectInput) (*domain.Project, error) {
	if req.Name != nil || req.ModuleType != nil || req.Description != nil {
		p, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}

		name := p.Name
		desc := p.Description

		if req.Name != nil {
			name = *req.Name
		}
		if req.Description != nil {
			desc = *req.Description
		}

		_, err = s.db.ExecContext(ctx,
			`UPDATE projects SET name=?, module_type='universal', description=?, updated_at=datetime('now')
			 WHERE id=? AND deleted_at IS NULL`,
			name, desc, id,
		)
		if err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, id)
}

func (s *ProjectService) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE projects SET deleted_at=datetime('now') WHERE id=?`, id)
	return err
}

func (s *ProjectService) ListFiles(ctx context.Context, projectID string) ([]domain.ProjectFile, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, path, content, created_at, updated_at
		 FROM project_files WHERE project_id=? ORDER BY path`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []domain.ProjectFile
	for rows.Next() {
		var f domain.ProjectFile
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.Path, &f.Content, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

func (s *ProjectService) GetFile(ctx context.Context, projectID, path string) (*domain.ProjectFile, error) {
	var f domain.ProjectFile
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, path, content, created_at, updated_at
		 FROM project_files WHERE project_id=? AND path=?`, projectID, path,
	).Scan(&f.ID, &f.ProjectID, &f.Path, &f.Content, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("file not found")
	}
	return &f, nil
}

func (s *ProjectService) SaveFile(ctx context.Context, projectID, path, content string) (*domain.ProjectFile, error) {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO project_files (project_id, path, content)
		 VALUES (?, ?, ?)
		 ON CONFLICT(project_id, path) DO UPDATE SET content=?, updated_at=datetime('now')`,
		projectID, path, content, content)
	if err != nil {
		return nil, err
	}
	return s.GetFile(ctx, projectID, path)
}
