package handler

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/domain"
	"github.com/moduforge/backend/internal/storage"
)

type ModuleVersionHandler struct {
	db *sql.DB
	s3 *storage.S3Adapter // optional S3-compatible storage for project files
}

func NewModuleVersionHandler(db *sql.DB) *ModuleVersionHandler {
	return &ModuleVersionHandler{db: db}
}

// SetS3 injects the optional S3 adapter used to read/write project file content.
func (h *ModuleVersionHandler) SetS3(adapter *storage.S3Adapter) {
	h.s3 = adapter
}

// s3ObjectKey returns the S3 object key for a project file.
func (h *ModuleVersionHandler) s3ObjectKey(projectID, path string) string {
	return projectID + "/" + strings.TrimPrefix(path, "/")
}

// readContent returns file content: S3 first (authoritative), DB content fallback.
func (h *ModuleVersionHandler) readContent(ctx context.Context, projectID, path string) (string, error) {
	if h.s3 != nil {
		data, err := h.s3.Read(ctx, h.s3ObjectKey(projectID, path))
		if err == nil {
			return string(data), nil
		}
		slog.Warn("s3 read failed, falling back to db content", "project", projectID, "path", path, "error", err)
	}
	var content string
	err := h.db.QueryRowContext(ctx,
		`SELECT COALESCE(content,'') FROM project_files WHERE project_id=? AND path=?`, projectID, path,
	).Scan(&content)
	if err != nil {
		return "", err
	}
	return content, nil
}

// saveContent persists file content: S3 (authoritative) + DB metadata.
func (h *ModuleVersionHandler) saveContent(ctx context.Context, projectID, path, content string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	sha := storage.ComputeSHA256([]byte(content))
	size := int64(len(content))

	if h.s3 != nil {
		if err := h.s3.Write(ctx, h.s3ObjectKey(projectID, path), []byte(content)); err != nil {
			return fmt.Errorf("s3 write failed: %w", err)
		}
		_, err := h.db.ExecContext(ctx,
			`INSERT INTO project_files (project_id, path, content, created_at, updated_at, sha256, file_size, mtime)
			 VALUES (?, ?, NULL, ?, ?, ?, ?, ?)
			 ON CONFLICT(project_id, path) DO UPDATE SET content=NULL, updated_at=datetime('now'), sha256=?, file_size=?, mtime=?`,
			projectID, path, now, now, sha, size, now, sha, size, now)
		return err
	}

	_, err := h.db.ExecContext(ctx,
		`INSERT INTO project_files (project_id, path, content, created_at, updated_at, sha256, file_size, mtime)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(project_id, path) DO UPDATE SET content=?, updated_at=datetime('now'), sha256=?, file_size=?, mtime=?`,
		projectID, path, content, now, now, sha, size, now, content, sha, size, now)
	return err
}

// POST /projects/:id/versions — Create a new version snapshot from current project files
func (h *ModuleVersionHandler) CreateVersion(c fiber.Ctx) error {
	projectID := c.Params("id")
	uid := c.Locals("uid")
	if uid == nil {
		return Unauthorized(c, "未授权")
	}

	var req struct {
		Version   string `json:"version"`
		Changelog string `json:"changelog"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "请求格式无效")
	}
	if req.Version == "" {
		return ValidationError(c, "版本号不能为空")
	}

	// Check version uniqueness
	var exists int
	h.db.QueryRow("SELECT COUNT(*) FROM project_versions WHERE project_id=? AND version=?", projectID, req.Version).Scan(&exists)
	if exists > 0 {
		return ErrorResponse(c, 409, "版本号已存在", ErrCodeConflict)
	}

	// Snapshot all current files
	rows, err := h.db.Query("SELECT path FROM project_files WHERE project_id=?", projectID)
	if err != nil {
		return InternalError(c, "读取项目文件失败")
	}
	defer rows.Close()

	type FileSnapshot struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	var snapshots []FileSnapshot
	fileCount := 0
	totalSize := 0
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			continue
		}
		content, cErr := h.readContent(c.Context(), projectID, path)
		if cErr != nil {
			continue
		}
		snapshots = append(snapshots, FileSnapshot{Path: path, Content: content})
		fileCount++
		totalSize += len(content)
	}

	snapshotJSON, _ := json.Marshal(snapshots)
	hash := fmt.Sprintf("%x", sha256.Sum256(snapshotJSON))

	_, err = h.db.Exec(
		`INSERT INTO project_versions (project_id, version, changelog, file_count, total_size, snapshot, file_hash, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		projectID, req.Version, req.Changelog, fileCount, totalSize, string(snapshotJSON), hash, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return InternalError(c, "创建版本失败")
	}

	return c.Status(201).JSON(fiber.Map{
		"version":    req.Version,
		"changelog":  req.Changelog,
		"file_count": fileCount,
		"total_size": totalSize,
		"file_hash":  hash,
	})
}

// GET /projects/:id/versions — List all versions for a project
func (h *ModuleVersionHandler) ListVersions(c fiber.Ctx) error {
	projectID := c.Params("id")

	rows, err := h.db.Query(
		`SELECT id, project_id, version, changelog, file_count, total_size, file_hash, created_at
		 FROM project_versions WHERE project_id=? ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return InternalError(c, "查询版本失败")
	}
	defer rows.Close()

	type VersionInfo struct {
		ID        int64  `json:"id"`
		ProjectID string `json:"project_id"`
		Version   string `json:"version"`
		Changelog string `json:"changelog"`
		FileCount int    `json:"file_count"`
		TotalSize int    `json:"total_size"`
		FileHash  string `json:"file_hash"`
		CreatedAt string `json:"created_at"`
	}

	var versions []VersionInfo
	for rows.Next() {
		var v VersionInfo
		if err := rows.Scan(&v.ID, &v.ProjectID, &v.Version, &v.Changelog, &v.FileCount, &v.TotalSize, &v.FileHash, &v.CreatedAt); err == nil {
			versions = append(versions, v)
		}
	}
	if versions == nil {
		versions = []VersionInfo{}
	}
	return c.JSON(fiber.Map{"versions": versions})
}

// POST /projects/:id/versions/:version/rollback — Rollback to a specific version
func (h *ModuleVersionHandler) RollbackVersion(c fiber.Ctx) error {
	projectID := c.Params("id")
	version := c.Params("version")
	uid := c.Locals("uid")
	if uid == nil {
		return Unauthorized(c, "未授权")
	}

	var snapshot string
	err := h.db.QueryRow(
		"SELECT snapshot FROM project_versions WHERE project_id=? AND version=?",
		projectID, version,
	).Scan(&snapshot)
	if err != nil {
		return NotFound(c, "版本不存在")
	}

	type FileSnapshot struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	var snapshots []FileSnapshot
	if err := json.Unmarshal([]byte(snapshot), &snapshots); err != nil {
		return InternalError(c, "版本快照数据损坏")
	}

	tx, err := h.db.Begin()
	if err != nil {
		return InternalError(c, "事务启动失败")
	}

	// Clear current files metadata
	oldRows, err := tx.Query("SELECT path FROM project_files WHERE project_id=?", projectID)
	if err != nil {
		tx.Rollback()
		return InternalError(c, "读取旧文件失败")
	}
	var oldPaths []string
	for oldRows.Next() {
		var p string
		if oldRows.Scan(&p) == nil {
			oldPaths = append(oldPaths, p)
		}
	}
	oldRows.Close()

	if _, err := tx.Exec("DELETE FROM project_files WHERE project_id=?", projectID); err != nil {
		tx.Rollback()
		return InternalError(c, "清理旧文件失败")
	}
	if err := tx.Commit(); err != nil {
		return InternalError(c, "提交事务失败")
	}

	// Restore files from snapshot: S3 (authoritative) + DB metadata.
	// Each file is written atomically (S3 object + DB metadata row).
	restored := 0
	for _, fs := range snapshots {
		if err := h.saveContent(c.Context(), projectID, fs.Path, fs.Content); err != nil {
			return InternalError(c, "恢复文件失败: "+fs.Path)
		}
		restored++
	}
	// Remove S3 objects that are no longer part of the snapshot
	for _, p := range oldPaths {
		found := false
		for _, fs := range snapshots {
			if fs.Path == p {
				found = true
				break
			}
		}
		if !found && h.s3 != nil {
			if err := h.s3.Delete(c.Context(), h.s3ObjectKey(projectID, p)); err != nil {
				slog.Warn("s3 delete stale file during rollback", "project", projectID, "path", p, "error", err)
			}
		}
	}

	return c.JSON(fiber.Map{"message": "回滚成功", "version": version, "files_restored": restored})
}

// GET /projects/:id/versions/:version/diff — Compare two versions
func (h *ModuleVersionHandler) VersionDiff(c fiber.Ctx) error {
	projectID := c.Params("id")
	versionA := c.Query("from")
	versionB := c.Query("to")

	if versionA == "" || versionB == "" {
		return ValidationError(c, "需要指定 from 和 to 版本参数")
	}

	type VersionData struct {
		Version  string
		Snapshot string
	}

	getVersion := func(v string) (*VersionData, error) {
		var snap VersionData
		err := h.db.QueryRow(
			"SELECT version, snapshot FROM project_versions WHERE project_id=? AND version=?",
			projectID, v,
		).Scan(&snap.Version, &snap.Snapshot)
		if err != nil {
			return nil, fmt.Errorf("版本 %s 不存在", v)
		}
		return &snap, nil
	}

	dataA, err := getVersion(versionA)
	if err != nil {
		return NotFound(c, err.Error())
	}
	dataB, err := getVersion(versionB)
	if err != nil {
		return NotFound(c, err.Error())
	}

	type FileSnapshot struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}

	parseFiles := func(snapshot string) map[string]string {
		var files []FileSnapshot
		json.Unmarshal([]byte(snapshot), &files)
		result := make(map[string]string)
		for _, f := range files {
			result[f.Path] = f.Content
		}
		return result
	}

	filesA := parseFiles(dataA.Snapshot)
	filesB := parseFiles(dataB.Snapshot)

	type DiffItem struct {
		Path   string `json:"path"`
		Status string `json:"status"` // added, removed, modified, unchanged
	}

	var diffs []DiffItem
	allPaths := make(map[string]bool)
	for p := range filesA {
		allPaths[p] = true
	}
	for p := range filesB {
		allPaths[p] = true
	}

	for p := range allPaths {
		contentA, existsA := filesA[p]
		contentB, existsB := filesB[p]
		if !existsA {
			diffs = append(diffs, DiffItem{Path: p, Status: "added"})
		} else if !existsB {
			diffs = append(diffs, DiffItem{Path: p, Status: "removed"})
		} else if contentA != contentB {
			diffs = append(diffs, DiffItem{Path: p, Status: "modified"})
		} else {
			diffs = append(diffs, DiffItem{Path: p, Status: "unchanged"})
		}
	}

	return c.JSON(fiber.Map{
		"version_a": versionA,
		"version_b": versionB,
		"diffs":     diffs,
		"total":     len(diffs),
	})
}

// ModuleVersionLister is the interface needed from market service
type ModuleVersionLister interface {
	GetModuleVersions(slug string) ([]*domain.ModuleVersion, error)
	RollbackModule(slug, version string) (*domain.MarketModule, error)
	UpdateModuleVersion(slug, version, changelog string) (*domain.MarketModule, error)
}

// MarketModuleVersionHandler handles market module version operations
type MarketModuleVersionHandler struct {
	market ModuleVersionLister
}

func NewMarketModuleVersionHandler(market ModuleVersionLister) *MarketModuleVersionHandler {
	return &MarketModuleVersionHandler{market: market}
}

// GET /market/module/:slug/versions
func (h *MarketModuleVersionHandler) GetVersions(c fiber.Ctx) error {
	slug := c.Params("slug")
	versions, err := h.market.GetModuleVersions(slug)
	if err != nil {
		return NotFound(c, err.Error())
	}
	return c.JSON(fiber.Map{"versions": versions})
}

// POST /market/module/:slug/rollback
func (h *MarketModuleVersionHandler) Rollback(c fiber.Ctx) error {
	slug := c.Params("slug")
	var req struct {
		Version string `json:"version"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "请求格式无效")
	}
	if req.Version == "" {
		return ValidationError(c, "版本号不能为空")
	}
	mod, err := h.market.RollbackModule(slug, req.Version)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(mod)
}

// POST /market/module/:slug/version
func (h *MarketModuleVersionHandler) UpdateVersion(c fiber.Ctx) error {
	slug := c.Params("slug")
	var req struct {
		Version   string `json:"version"`
		Changelog string `json:"changelog"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "请求格式无效")
	}
	if req.Version == "" {
		return ValidationError(c, "版本号不能为空")
	}
	mod, err := h.market.UpdateModuleVersion(slug, req.Version, req.Changelog)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(mod)
}
