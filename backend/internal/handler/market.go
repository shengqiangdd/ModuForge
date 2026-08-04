package handler

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/domain"
	"github.com/moduforge/backend/internal/service"
)

type ModuleLister interface {
	ListModules(query, category, sort string, page, perPage int) ([]*domain.MarketModule, int)
	GetModule(slugOrID string) (*domain.MarketModule, error)
	StarModule(slugOrID string) (int, error)
	AddReview(moduleID, uid, username string, rating int, comment string) error
	GetReviews(moduleID string) []*domain.MarketReview
	PublishModule(mod *domain.MarketModule) (*domain.MarketModule, error)
	TrendingModules(limit int) []*domain.MarketModule
	Categories() []string
	GetModuleVersions(slug string) ([]*domain.ModuleVersion, error)
	RollbackModule(slug, version string) (*domain.MarketModule, error)
	UpdateModuleVersion(slug, version, changelog string) (*domain.MarketModule, error)
	GetModuleDependencies(slug string) ([]domain.ModuleDependency, error)
	ResolveDependencies(slug string) (*domain.DependencyNode, error)
	CheckDependencyConflicts(slug string) ([]domain.Conflict, error)
	AddScreenshot(slug, url string) (*domain.ModuleScreenshot, error)
	GetScreenshots(slug string) ([]*domain.ModuleScreenshot, error)
	GetScreenshotsByModuleID(moduleID string) ([]*domain.ModuleScreenshot, error)
	DeleteScreenshot(id int64) error
	UpdateModuleCoverImage(slug, coverURL string) error
	CompareModules(slug1, slug2 string) (*domain.ModuleComparison, error)
	GetModuleDemo(slug string) (*domain.ModuleDemo, error)
	GetModuleHealth(slug string) (*service.HealthScore, error)
	SetModuleTags(slug string, tagIDs []int) error
	GetModuleTags(slug string) ([]domain.ModuleTag, error)
}

type MarketHandler struct {
	market      ModuleLister
	storagePath string
	db          *sql.DB
}

func NewMarketHandlerWithDB(market ModuleLister, storagePath string, db *sql.DB) *MarketHandler {
	return &MarketHandler{market: market, storagePath: storagePath, db: db}
}

func NewMarketHandler(market ModuleLister, storagePath string) *MarketHandler {
	return &MarketHandler{market: market, storagePath: storagePath}
}

// GET /market/modules?query=X&category=Y&sort=Z&page=1&per_page=20
func (h *MarketHandler) ListModules(c fiber.Ctx) error {
	query := c.Query("query")
	category := c.Query("category")
	sortBy := c.Query("sort", "stars")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}

	c.Set("Cache-Control", "public, max-age=60")
	modules, total := h.market.ListModules(query, category, sortBy, page, perPage)
	return c.JSON(fiber.Map{"modules": modules, "total": total, "page": page, "per_page": perPage})
}

// GET /market/module/:slug
func (h *MarketHandler) GetModule(c fiber.Ctx) error {
	slug := c.Params("slug")
	mod, err := h.market.GetModule(slug)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	screenshots, _ := h.market.GetScreenshots(slug)
	mod.Screenshots = screenshots
	c.Set("Cache-Control", "public, max-age=120")
	return c.JSON(mod)
}

// POST /market/module/:slug/star
func (h *MarketHandler) StarModule(c fiber.Ctx) error {
	slug := c.Params("slug")
	stars, err := h.market.StarModule(slug)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"stars": stars})
}

// POST /market/module/:slug/review
func (h *MarketHandler) AddReview(c fiber.Ctx) error {
	slug := c.Params("slug")
	// Use authenticated user's identity, not client-supplied values
	userID := safeUserID(c)
	if userID == "" {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}
	// Look up username from database
	var username string
	if h.db != nil {
		_ = h.db.QueryRow("SELECT COALESCE(username, id) FROM users WHERE id = ?", userID).Scan(&username)
	}
	if username == "" {
		username = userID
	}
	var req struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Rating < 1 || req.Rating > 5 {
		return c.Status(422).JSON(fiber.Map{"error": "rating must be between 1 and 5"})
	}
	if len(req.Comment) > 1000 {
		return c.Status(422).JSON(fiber.Map{"error": "comment too long (max 1000)"})
	}
	if err := h.market.AddReview(slug, userID, username, req.Rating, req.Comment); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

// GET /market/module/:slug/reviews
func (h *MarketHandler) GetReviews(c fiber.Ctx) error {
	slug := c.Params("slug")
	reviews := h.market.GetReviews(slug)
	return c.JSON(fiber.Map{"reviews": reviews})
}

// POST /market/publish
func (h *MarketHandler) Publish(c fiber.Ctx) error {
	var mod domain.MarketModule
	if err := c.Bind().JSON(&mod); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if mod.Title == "" || len(mod.Title) > 100 {
		return c.Status(422).JSON(fiber.Map{"error": "title required (max 100)"})
	}
	if mod.Slug == "" || len(mod.Slug) > 100 {
		return c.Status(422).JSON(fiber.Map{"error": "slug required (max 100)"})
	}
	if len(mod.Description) > 1000 {
		return c.Status(422).JSON(fiber.Map{"error": "description too long (max 1000)"})
	}
	result, err := h.market.PublishModule(&mod)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// GET /market/trending?limit=10
func (h *MarketHandler) Trending(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	if limit < 1 {
		limit = 10
	}
	c.Set("Cache-Control", "public, max-age=120")
	modules := h.market.TrendingModules(limit)
	return c.JSON(fiber.Map{"modules": modules})
}

// GET /market/module/:slug/versions
func (h *MarketHandler) GetModuleVersions(c fiber.Ctx) error {
	slug := c.Params("slug")
	versions, err := h.market.GetModuleVersions(slug)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"versions": versions})
}

// POST /market/module/:slug/rollback
func (h *MarketHandler) RollbackModule(c fiber.Ctx) error {
	slug := c.Params("slug")
	var req struct {
		Version string `json:"version"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Version == "" {
		return c.Status(422).JSON(fiber.Map{"error": "version required"})
	}
	mod, err := h.market.RollbackModule(slug, req.Version)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(mod)
}

// POST /market/module/:slug/version — update version and create version record
func (h *MarketHandler) UpdateModuleVersion(c fiber.Ctx) error {
	slug := c.Params("slug")
	var req struct {
		Version   string `json:"version"`
		Changelog string `json:"changelog"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Version == "" {
		return c.Status(422).JSON(fiber.Map{"error": "version required"})
	}
	mod, err := h.market.UpdateModuleVersion(slug, req.Version, req.Changelog)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(mod)
}

// GET /market/categories
func (h *MarketHandler) Categories(c fiber.Ctx) error {
	c.Set("Cache-Control", "public, max-age=300")
	return c.JSON(fiber.Map{"categories": h.market.Categories()})
}

// GET /market/module/:slug/dependencies
func (h *MarketHandler) GetModuleDependencies(c fiber.Ctx) error {
	slug := c.Params("slug")
	deps, err := h.market.GetModuleDependencies(slug)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	tree, _ := h.market.ResolveDependencies(slug)
	return c.JSON(fiber.Map{"dependencies": deps, "tree": tree})
}

// POST /market/module/:slug/screenshots
func (h *MarketHandler) UploadScreenshots(c fiber.Ctx) error {
	slug := c.Params("slug")
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid multipart form"})
	}
	files := form.File["screenshots"]
	if len(files) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "no screenshots provided"})
	}
	if len(files) > 5 {
		return c.Status(422).JSON(fiber.Map{"error": "maximum 5 screenshots"})
	}

	var screenshots []*domain.ModuleScreenshot
	for _, f := range files {
		if f.Size > 2*1024*1024 {
			return c.Status(422).JSON(fiber.Map{"error": "each screenshot must be ≤ 2MB"})
		}
		// Use random filename to prevent path traversal via uploaded filename
		ext := filepath.Ext(f.Filename)
		if ext == "" {
			ext = ".png"
		}
		filename := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), uuid.New().String()[:8], ext)
		savePath := h.storagePath + "/screenshots/" + filename
		if err := c.SaveFile(f, savePath); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to save screenshot"})
		}
		url := "/uploads/screenshots/" + filename
		ss, err := h.market.AddScreenshot(slug, url)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		screenshots = append(screenshots, ss)
	}
	return c.JSON(fiber.Map{"screenshots": screenshots})
}

// DELETE /market/screenshots/:id
func (h *MarketHandler) DeleteScreenshot(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid screenshot id"})
	}
	if err := h.market.DeleteScreenshot(id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

// POST /market/compare
func (h *MarketHandler) Compare(c fiber.Ctx) error {
	var req struct {
		Slug1 string `json:"slug1"`
		Slug2 string `json:"slug2"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Slug1 == "" || req.Slug2 == "" {
		return c.Status(422).JSON(fiber.Map{"error": "slug1 and slug2 required"})
	}
	result, err := h.market.CompareModules(req.Slug1, req.Slug2)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// GET /market/module/:slug/demo
func (h *MarketHandler) GetDemo(c fiber.Ctx) error {
	slug := c.Params("slug")
	demo, err := h.market.GetModuleDemo(slug)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(demo)
}

// POST /market/module/:slug/check-deps
func (h *MarketHandler) GetModuleHealth(c fiber.Ctx) error {
	slug := c.Params("slug")
	health, err := h.market.GetModuleHealth(slug)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(health)
}

// GET /market/module/:slug/changelogs
func (h *MarketHandler) GetChangelogs(c fiber.Ctx) error {
	slug := c.Params("slug")
	rows, err := h.db.Query("SELECT id, module_slug, version, content, created_at FROM module_changelogs WHERE module_slug = ? ORDER BY created_at DESC", slug)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()
	type Changelog struct {
		ID        int64  `json:"id"`
		ModuleSlug string `json:"module_slug"`
		Version   string `json:"version"`
		Content   string `json:"content"`
		CreatedAt string `json:"created_at"`
	}
	var logs []Changelog
	for rows.Next() {
		var l Changelog
		if err := rows.Scan(&l.ID, &l.ModuleSlug, &l.Version, &l.Content, &l.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, l)
	}
	if logs == nil {
		logs = []Changelog{}
	}
	return c.JSON(fiber.Map{"changelogs": logs})
}

// POST /market/module/:slug/changelog
func (h *MarketHandler) CreateChangelog(c fiber.Ctx) error {
	slug := c.Params("slug")
	var req struct {
		Version string `json:"version"`
		Content string `json:"content"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Version == "" || req.Content == "" {
		return ValidationError(c, "version and content required")
	}
	_, err := h.db.Exec("INSERT INTO module_changelogs (module_slug, version, content) VALUES (?, ?, ?)", slug, req.Version, req.Content)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.Status(201).JSON(fiber.Map{"ok": true})
}

// DELETE /market/changelog/:id
func (h *MarketHandler) DeleteChangelog(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	if _, err := h.db.Exec("DELETE FROM module_changelogs WHERE id = ?", id); err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

// PUT /market/screenshots/:id — update caption
func (h *MarketHandler) UpdateScreenshot(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid screenshot id"})
	}
	var req struct {
		Caption string `json:"caption"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	_, err = h.db.Exec("UPDATE module_screenshots SET caption = ? WHERE id = ?", req.Caption, id)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *MarketHandler) CheckDependencyConflicts(c fiber.Ctx) error {
	slug := c.Params("slug")
	conflicts, err := h.market.CheckDependencyConflicts(slug)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if conflicts == nil {
		conflicts = []domain.Conflict{}
	}
	return c.JSON(fiber.Map{"conflicts": conflicts, "safe": len(conflicts) == 0})
}

// POST /market/batch/install
func (h *MarketHandler) BatchInstall(c fiber.Ctx) error {
	var req struct {
		Slugs []string `json:"slugs"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	type BatchResult struct {
		Slug  string `json:"slug"`
		Error string `json:"error,omitempty"`
	}
	results := make([]BatchResult, 0, len(req.Slugs))
	for _, slug := range req.Slugs {
		mod, err := h.market.GetModule(slug)
		if err != nil {
			results = append(results, BatchResult{Slug: slug, Error: err.Error()})
			continue
		}
		cur := mod.Installs
		h.db.Exec("UPDATE market_modules SET installs = installs + 1 WHERE slug = ?", slug)
		_ = cur
		results = append(results, BatchResult{Slug: slug})
	}
	return c.JSON(fiber.Map{"results": results})
}

// POST /market/batch/uninstall
func (h *MarketHandler) BatchUninstall(c fiber.Ctx) error {
	var req struct {
		Slugs []string `json:"slugs"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	type BatchResult struct {
		Slug  string `json:"slug"`
		Error string `json:"error,omitempty"`
	}
	results := make([]BatchResult, 0, len(req.Slugs))
	for _, slug := range req.Slugs {
		_, err := h.market.GetModule(slug)
		if err != nil {
			results = append(results, BatchResult{Slug: slug, Error: err.Error()})
			continue
		}
		h.db.Exec("UPDATE market_modules SET installs = MAX(0, installs - 1) WHERE slug = ?", slug)
		results = append(results, BatchResult{Slug: slug})
	}
	return c.JSON(fiber.Map{"results": results})
}

// POST /market/batch/update
func (h *MarketHandler) BatchUpdate(c fiber.Ctx) error {
	var req struct {
		Slugs []string `json:"slugs"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	type BatchResult struct {
		Slug    string `json:"slug"`
		Version string `json:"version,omitempty"`
		Error   string `json:"error,omitempty"`
	}
	results := make([]BatchResult, 0, len(req.Slugs))
	for _, slug := range req.Slugs {
		mod, err := h.market.GetModule(slug)
		if err != nil {
			results = append(results, BatchResult{Slug: slug, Error: err.Error()})
			continue
		}
		results = append(results, BatchResult{Slug: slug, Version: mod.Version})
	}
	return c.JSON(fiber.Map{"results": results})
}

// GET /market/module/:slug/install-stats?period=day|week|month&days=30
func (h *MarketHandler) GetInstallStats(c fiber.Ctx) error {
	slug := c.Params("slug")
	period := c.Query("period", "day")
	days, _ := strconv.Atoi(c.Query("days", "30"))
	if days < 1 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	query := ""
	switch period {
	case "week":
		query = "SELECT strftime('%Y-%W', created_at) AS period, COUNT(*) FROM crash_logs WHERE module_slug = ? AND created_at >= datetime('now', ? || ' days') GROUP BY period ORDER BY period"
	case "month":
		query = "SELECT strftime('%Y-%m', created_at) AS period, COUNT(*) FROM crash_logs WHERE module_slug = ? AND created_at >= datetime('now', ? || ' days') GROUP BY period ORDER BY period"
	default:
		query = "SELECT date(created_at) AS period, COUNT(*) FROM crash_logs WHERE module_slug = ? AND created_at >= datetime('now', ? || ' days') GROUP BY period ORDER BY period"
	}
	rows, err := h.db.Query(query, slug, -days)
	if err != nil {
		return InternalError(c, err.Error())
	}
	defer rows.Close()
	type StatPoint struct {
		Period string `json:"period"`
		Count  int    `json:"count"`
	}
	var stats []StatPoint
	for rows.Next() {
		var s StatPoint
		if err := rows.Scan(&s.Period, &s.Count); err != nil {
			continue
		}
		stats = append(stats, s)
	}
	if stats == nil {
		stats = []StatPoint{}
	}
	return c.JSON(fiber.Map{"stats": stats})
}

// GET /market/stats/trending
func (h *MarketHandler) GetTrending(c fiber.Ctx) error {
	rows, err := h.db.Query(`
		SELECT slug, title, category, installs, stars, 
			CASE WHEN installs > 0 THEN CAST(stars AS REAL) / installs ELSE 0 END AS popularity
		FROM market_modules 
		WHERE updated_at >= datetime('now', '-7 days')
		ORDER BY installs DESC LIMIT 10
	`)
	if err != nil {
		return InternalError(c, err.Error())
	}
	defer rows.Close()
	type TrendingModule struct {
		Slug       string  `json:"slug"`
		Title      string  `json:"title"`
		Category   string  `json:"category"`
		Installs   int     `json:"installs"`
		Stars      int     `json:"stars"`
		Popularity float64 `json:"popularity"`
	}
	var modules []TrendingModule
	for rows.Next() {
		var m TrendingModule
		if err := rows.Scan(&m.Slug, &m.Title, &m.Category, &m.Installs, &m.Stars, &m.Popularity); err != nil {
			continue
		}
		modules = append(modules, m)
	}
	if modules == nil {
		modules = []TrendingModule{}
	}
	return c.JSON(fiber.Map{"modules": modules})
}

// DownloadModule returns the module zip from module_versions.file_path
// GET /market/module/:slug/download
func (h *MarketHandler) DownloadModule(c fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return BadRequest(c, "slug is required")
	}

	// Find the latest version's file_path
	var filePath string
	err := h.db.QueryRow(`
		SELECT mv.file_path FROM module_versions mv
		JOIN market_modules mm ON mv.module_id = mm.id
		WHERE mm.slug = ? AND mv.file_path != ''
		ORDER BY mv.created_at DESC LIMIT 1
	`, slug).Scan(&filePath)
	if err != nil || filePath == "" {
		return c.Status(404).JSON(fiber.Map{"error": "该模块没有可下载的版本"})
	}

	// Check file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return c.Status(404).JSON(fiber.Map{"error": "模块文件不存在"})
	}

	return c.Download(filePath, slug+".zip")
}
