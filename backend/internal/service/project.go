package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/moduforge/backend/internal/domain"
)

type ProjectService struct {
	db          *sql.DB
	storagePath string // e.g. "data/storage" — base path for project files on disk
}

func NewProjectService(db *sql.DB, storagePath string) *ProjectService {
	return &ProjectService{db: db, storagePath: storagePath}
}

// diskPath returns the on-disk directory for a project's files.
func (s *ProjectService) diskPath(projectID string) string {
	return filepath.Join(s.storagePath, "projects", projectID)
}

func (s *ProjectService) List(ctx context.Context, userID string) ([]domain.Project, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, module_type, description, 
			 COALESCE(git_url,''), COALESCE(git_branch,'main'), COALESCE(build_cron,''),
			 COALESCE(auto_build,0), created_at, updated_at
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
		var autoBuild int
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.ModuleType, &p.Description,
			&p.GitURL, &p.GitBranch, &p.BuildCron, &autoBuild, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.AutoBuild = autoBuild == 1
		projects = append(projects, p)
	}
	return projects, nil
}

func (s *ProjectService) Create(ctx context.Context, userID string, req *domain.CreateProjectInput) (*domain.Project, error) {
	var p domain.Project
	p.ID = uuid.New().String()

	// Auto-infer module_type from description if not explicitly set
	moduleType := req.ModuleType
	if moduleType == "" || moduleType == "universal" {
		moduleType = inferModuleType(req.Name, req.Description)
	}

	err := s.db.QueryRowContext(ctx,
		`INSERT INTO projects (id, user_id, name, module_type, description)
		 VALUES (?, ?, ?, ?, ?)
		 RETURNING id, user_id, name, module_type, description, created_at, updated_at`,
		p.ID, userID, req.Name, moduleType, req.Description,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.ModuleType, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return &p, nil
}

// inferModuleType analyzes the project name and description to determine the best module type.
// It uses keyword matching to classify the project's primary purpose.
func inferModuleType(name, description string) domain.ModuleType {
	text := strings.ToLower(name + " " + description)

	// Priority order: more specific matches first
	typePatterns := []struct {
		keywords []string
		mtype    domain.ModuleType
	}{
		{[]string{"performance", "optimization", "优化", "调优", "性能", "tune", "调度", "scheduler", "governor", "cpufreq", "gpu"},
			domain.ModulePerformance},
		{[]string{"security", "security", "安全", "protection", "保护", "selinux", "防篡改", "detect", "检测", "入侵", "ids"},
			domain.ModuleMagisk},
		{[]string{"monitor", "监控", "monitoring", "log", "日志", "observability", "可观测", "metrics", "指标"},
			domain.ModuleUniversal},
		{[]string{"network", "网络", "proxy", "代理", "vpn", "流量", "firewall", "防火墙"},
			domain.ModuleUniversal},
		{[]string{"tool", "工具", "utility", "辅助", "script", "脚本", "manager", "管理"},
			domain.ModuleUniversal},
		{[]string{"game", "游戏", "gaming", "gpu", "graphics", "渲染", "render"},
			domain.ModulePerformance},
		{[]string{"battery", "电池", "power", "能耗", "energy", "省电", "power_save"},
			domain.ModulePerformance},
		{[]string{"c++", "rust", "go", "native", "daemon", "守护进程", "engine", "引擎"},
			domain.ModulePerformance},
	}

	for _, p := range typePatterns {
		for _, kw := range p.keywords {
			if strings.Contains(text, kw) {
				return p.mtype
			}
		}
	}

	return domain.ModuleUniversal
}

func (s *ProjectService) Get(ctx context.Context, id string) (*domain.Project, error) {
	return s.GetByUser(ctx, id, "")
}

// GetByUser returns a project, optionally filtered by userID for ownership check.
// If userID is empty, no ownership check is performed (admin/system calls).
func (s *ProjectService) GetByUser(ctx context.Context, id string, userID string) (*domain.Project, error) {
	var p domain.Project
	var autoBuild int
	var err error
	if userID != "" {
		err = s.db.QueryRowContext(ctx,
			`SELECT id, user_id, name, module_type, description, 
				 COALESCE(git_url,''), COALESCE(git_branch,'main'), COALESCE(build_cron,''),
				 COALESCE(auto_build,0), created_at, updated_at
			 FROM projects WHERE id = ? AND user_id = ? AND deleted_at IS NULL`, id, userID,
		).Scan(&p.ID, &p.UserID, &p.Name, &p.ModuleType, &p.Description,
			&p.GitURL, &p.GitBranch, &p.BuildCron, &autoBuild, &p.CreatedAt, &p.UpdatedAt)
	} else {
		err = s.db.QueryRowContext(ctx,
			`SELECT id, user_id, name, module_type, description, 
				 COALESCE(git_url,''), COALESCE(git_branch,'main'), COALESCE(build_cron,''),
				 COALESCE(auto_build,0), created_at, updated_at
			 FROM projects WHERE id = ? AND deleted_at IS NULL`, id,
		).Scan(&p.ID, &p.UserID, &p.Name, &p.ModuleType, &p.Description,
			&p.GitURL, &p.GitBranch, &p.BuildCron, &autoBuild, &p.CreatedAt, &p.UpdatedAt)
	}
	if err != nil {
		return nil, fmt.Errorf("project not found")
	}
	p.AutoBuild = autoBuild == 1
	return &p, nil
}

// CheckOwnership returns true if the user owns the project.
func (s *ProjectService) CheckOwnership(ctx context.Context, projectID, userID string) bool {
	var ownerID string
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM projects WHERE id=? AND deleted_at IS NULL`, projectID).Scan(&ownerID)
	return err == nil && ownerID == userID
}

func (s *ProjectService) Update(ctx context.Context, id string, req *domain.UpdateProjectInput) (*domain.Project, error) {
	return s.UpdateByUser(ctx, id, "", req)
}

// UpdateByUser updates a project with optional ownership check.
func (s *ProjectService) UpdateByUser(ctx context.Context, id string, userID string, req *domain.UpdateProjectInput) (*domain.Project, error) {
	if req.Name != nil || req.ModuleType != nil || req.Description != nil || req.GitURL != nil || req.GitBranch != nil || req.BuildCron != nil || req.AutoBuild != nil {
		p, err := s.GetByUser(ctx, id, userID)
		if err != nil {
			return nil, err
		}

		name := p.Name
		desc := p.Description
		moduleType := p.ModuleType

		if req.Name != nil {
			name = *req.Name
		}
		if req.Description != nil {
			desc = *req.Description
		}
		if req.ModuleType != nil {
			moduleType = *req.ModuleType
		}

		_, err = s.db.ExecContext(ctx,
			`UPDATE projects SET name=?, module_type=?, description=?, 
			 git_url=COALESCE(?,git_url), git_branch=COALESCE(?,git_branch), 
			 build_cron=COALESCE(?,build_cron), auto_build=COALESCE(?,auto_build),
			 updated_at=datetime('now')
			 WHERE id=? AND deleted_at IS NULL`,
			name, moduleType, desc, req.GitURL, req.GitBranch, req.BuildCron, req.AutoBuild, id,
		)
		if err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, id)
}

func (s *ProjectService) Delete(ctx context.Context, id string) error {
	return s.DeleteWithRecycle(ctx, id, "", "")
}

// DeleteByUser deletes a project with ownership check.
func (s *ProjectService) DeleteByUser(ctx context.Context, id string, userID string) error {
	if userID != "" && !s.CheckOwnership(ctx, id, userID) {
		return fmt.Errorf("permission denied: you do not own this project")
	}
	return s.DeleteWithRecycle(ctx, id, userID, "")
}

// DeleteWithRecycle deletes a project and inserts a recycle bin entry, all in one transaction.
func (s *ProjectService) DeleteWithRecycle(ctx context.Context, id string, userID string, name string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Collect artifact paths BEFORE deleting build_tasks
	artifactPaths := []string{}
	rows, qErr := tx.QueryContext(ctx, "SELECT artifact_path FROM build_tasks WHERE project_id=?", id)
	if qErr == nil && rows != nil {
		for rows.Next() {
			var p string
			if rows.Scan(&p) == nil && p != "" {
				artifactPaths = append(artifactPaths, p)
			}
		}
		rows.Close()
	}

	// 2. Delete all project-related data.
	//    Use individual statements so a missing table doesn't poison the transaction.
	//    We query the sqlite_master to only DELETE from tables that actually exist.
	existingTables := map[string]bool{}
	tRows, _ := tx.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table'")
	if tRows != nil {
		for tRows.Next() {
			var t string
			if tRows.Scan(&t) == nil {
				existingTables[t] = true
			}
		}
		tRows.Close()
	}

	projectTables := []string{
		"build_tasks", "project_files", "file_comments", "comments",
		"collaborators", "edit_sessions", "team_members", "activities",
		"vulnerability_scans", "project_versions", "git_branches",
		"collaboration_sessions", "module_vuln_scans", "permission_audits",
	}
	for _, table := range projectTables {
		if !existingTables[table] {
			continue
		}
		// Also verify the table has a project_id column
		var hasCol bool
		cRows, _ := tx.QueryContext(ctx,
			fmt.Sprintf("SELECT 1 FROM pragma_table_info('%s') WHERE name='project_id'", table))
		if cRows != nil {
			if cRows.Next() {
				hasCol = true
			}
			cRows.Close()
		}
		if hasCol {
			_, _ = tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE project_id=?", table), id)
		}
	}

	// 3. Soft delete the project
	_, err = tx.ExecContext(ctx, `UPDATE projects SET deleted_at=datetime('now') WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("soft delete project: %w", err)
	}

	// 4. Insert recycle bin entry (inside the same transaction!)
	if userID != "" && name != "" {
		projData := fmt.Sprintf(`{"id":"%s","name":"%s"}`, id, name)
		expires := time.Now().AddDate(0, 0, 30)
		// Use a subquery to get the numeric rowid for item_id (recycle_bin.item_id is INTEGER)
		_, _ = tx.ExecContext(ctx,
			`INSERT INTO recycle_bin (user_id, item_type, item_id, item_name, item_data, expires_at)
			 VALUES (?, 'project', (SELECT rowid FROM projects WHERE id=?), ?, ?, ?)`,
			userID, id, name, projData, expires)
	}

	// 5. Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// 6. Remove cached build artifacts from disk (after successful commit)
	for _, p := range artifactPaths {
		os.Remove(p)
	}

	return nil
}

func (s *ProjectService) checkProjectOwnership(ctx context.Context, projectID, userID string) error {
	var ownerID string
	err := s.db.QueryRowContext(ctx, "SELECT user_id FROM projects WHERE id = ? AND deleted_at IS NULL", projectID).Scan(&ownerID)
	if err != nil {
		return fmt.Errorf("project not found")
	}
	if ownerID != "" && ownerID != userID {
		return fmt.Errorf("access denied")
	}
	return nil
}

func (s *ProjectService) ListFiles(ctx context.Context, projectID, userID string) ([]domain.ProjectFile, error) {
	if projectID == "" {
		return []domain.ProjectFile{}, nil
	}
	if err := s.checkProjectOwnership(ctx, projectID, userID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, path, created_at, updated_at, sha256, file_size, mtime
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

func (s *ProjectService) GetFile(ctx context.Context, projectID, path, userID string) (*domain.ProjectFile, error) {
	if err := s.checkProjectOwnership(ctx, projectID, userID); err != nil {
		return nil, err
	}
	var f domain.ProjectFile
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, path, created_at, updated_at, sha256, file_size, mtime
		 FROM project_files WHERE project_id=? AND path=?`, projectID, path,
	).Scan(&f.ID, &f.ProjectID, &f.Path, &f.CreatedAt, &f.UpdatedAt, &f.SHA256, &f.FileSize, &f.MTime)
	if err != nil {
		return nil, fmt.Errorf("file not found")
	}
	// Read content from disk (S3-synced)
	if s.storagePath != "" {
		fullPath := filepath.Join(s.diskPath(projectID), path)
		if data, err := os.ReadFile(fullPath); err == nil {
			f.Content = string(data)
		}
	}
	return &f, nil
}

type SearchResult struct {
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	FilePath    string `json:"file_path"`
	Line        int    `json:"line,omitempty"`
	Context     string `json:"context"`
}

func (s *ProjectService) SearchAll(ctx context.Context, userID, query string) ([]SearchResult, error) {
	if query == "" {
		return nil, nil
	}

	projects, err := s.List(ctx, userID)
	if err != nil {
		return nil, err
	}

	var results []SearchResult

	for _, p := range projects {
		// Search project name
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(query)) {
			results = append(results, SearchResult{
				ProjectID:   p.ID,
				ProjectName: p.Name,
				FilePath:    "",
				Context:     p.Name,
			})
		}

		// Search file contents
		files, err := s.ListFiles(ctx, p.ID, userID)
		if err != nil {
			continue
		}
		for _, f := range files {
			// Search file path
			if strings.Contains(strings.ToLower(f.Path), strings.ToLower(query)) {
				results = append(results, SearchResult{
					ProjectID:   p.ID,
					ProjectName: p.Name,
					FilePath:    f.Path,
					Context:     f.Path,
				})
			}
			// Search file content
			if f.Content != "" {
				lower := strings.ToLower(f.Content)
				idx := strings.Index(lower, strings.ToLower(query))
				if idx >= 0 {
					start := idx - 25
					if start < 0 {
						start = 0
					}
					end := idx + len(query) + 25
					if end > len(f.Content) {
						end = len(f.Content)
					}
					context := f.Content[start:end]
					results = append(results, SearchResult{
						ProjectID:   p.ID,
						ProjectName: p.Name,
						FilePath:    f.Path,
						Context:     context,
					})
				}
			}
		}

		if len(results) >= 50 {
			break
		}
	}

	if len(results) > 50 {
		results = results[:50]
	}
	return results, nil
}

func (s *ProjectService) SaveFile(ctx context.Context, projectID, path, content, userID string) (*domain.ProjectFile, error) {
	if err := s.checkProjectOwnership(ctx, projectID, userID); err != nil {
		return nil, err
	}

	// 1. Write to disk (authoritative storage after S3 migration)
	fullPath := filepath.Join(s.diskPath(projectID), path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	// 2. Calculate checksum & size
	h := sha256.New()
	io.WriteString(h, content)
	sha256hex := hex.EncodeToString(h.Sum(nil))
	fileSize := int64(len(content))
	now := time.Now().UTC().Format(time.RFC3339)

	// 3. Update database metadata (no content column — content lives on disk/S3)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO project_files (project_id, path, sha256, file_size, mtime, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		 ON CONFLICT(project_id, path) DO UPDATE SET
		   sha256=?, file_size=?, mtime=?, updated_at=datetime('now')`,
		projectID, path, sha256hex, fileSize, now,
		sha256hex, fileSize, now)
	if err != nil {
		return nil, fmt.Errorf("db update: %w", err)
	}

	return &domain.ProjectFile{
		ID:        0, // caller can re-GET to fill the real ID
		ProjectID: projectID,
		Path:      path,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// DeleteFile removes a file from both the database and disk.
func (s *ProjectService) DeleteFile(ctx context.Context, projectID, path, userID string) error {
	if err := s.checkProjectOwnership(ctx, projectID, userID); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM project_files WHERE project_id=? AND path=?`, projectID, path)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("file not found")
	}
	// Remove from disk (best-effort)
	if s.storagePath != "" {
		fullPath := filepath.Join(s.diskPath(projectID), path)
		os.Remove(fullPath)
	}
	return nil
}
