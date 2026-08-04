package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/database"
	"github.com/moduforge/backend/internal/domain"
)

func scanModule(rows interface{ Scan(...interface{}) error }) (*domain.MarketModule, error) {
	var m domain.MarketModule
	err := rows.Scan(&m.ID, &m.Title, &m.Slug, &m.Description, &m.Category, &m.Tags,
		&m.Version, &m.VersionCode, &m.Changelog, &m.ParentID,
		&m.Author, &m.AuthorUID, &m.License,
		&m.Dependencies, &m.CoverImage,
		&m.Stars, &m.Installs, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

const moduleColumns = "id, title, slug, description, category, tags, version, version_code, COALESCE(changelog,''), COALESCE(parent_id,''), author, COALESCE(author_uid,''), COALESCE(license,''), COALESCE(dependencies,''), COALESCE(cover_image,''), stars, installs, created_at, updated_at"

type SQLiteMarketService struct {
	db *database.DB
}

func NewSQLiteMarketService(db *database.DB) *SQLiteMarketService {
	return &SQLiteMarketService{db: db}
}

func (s *SQLiteMarketService) ListModules(query, category, sort string, page, perPage int) ([]*domain.MarketModule, int) {
	where := []string{}
	args := []interface{}{}

	if category != "" {
		where = append(where, "category = ?")
		args = append(args, category)
	}
	if query != "" {
		where = append(where, "(title LIKE ? OR description LIKE ? OR tags LIKE ?)")
		q := "%" + query + "%"
		args = append(args, q, q, q)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	orderClause := "stars DESC"
	switch sort {
	case "installs":
		orderClause = "installs DESC"
	case "newest":
		orderClause = "created_at DESC"
	case "title":
		orderClause = "title ASC"
	}

	countQuery := "SELECT COUNT(*) FROM market_modules " + whereClause
	var total int
	s.db.Conn.QueryRow(countQuery, args...).Scan(&total)

	offset := (page - 1) * perPage
	querySQL := fmt.Sprintf(
		"SELECT %s FROM market_modules %s ORDER BY %s LIMIT ? OFFSET ?",
		moduleColumns, whereClause, orderClause,
	)
	args = append(args, perPage, offset)

	rows, err := s.db.Conn.Query(querySQL, args...)
	if err != nil {
		return nil, 0
	}
	defer rows.Close()

	var modules []*domain.MarketModule
	for rows.Next() {
		m, err := scanModule(rows)
		if err != nil {
			continue
		}
		modules = append(modules, m)
	}
	return modules, total
}

func (s *SQLiteMarketService) GetModule(slugOrID string) (*domain.MarketModule, error) {
	return scanModule(s.db.Conn.QueryRow(
		"SELECT "+moduleColumns+" FROM market_modules WHERE slug = ? OR id = ?",
		slugOrID, slugOrID,
	))
}

func (s *SQLiteMarketService) StarModule(slugOrID string) (int, error) {
	result, err := s.db.Conn.Exec("UPDATE market_modules SET stars = stars + 1 WHERE slug = ? OR id = ?", slugOrID, slugOrID)
	if err != nil {
		return 0, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return 0, fmt.Errorf("module not found")
	}
	var stars int
	s.db.Conn.QueryRow("SELECT stars FROM market_modules WHERE slug = ? OR id = ?", slugOrID, slugOrID).Scan(&stars)
	return stars, nil
}

func (s *SQLiteMarketService) AddReview(moduleID, uid, username string, rating int, comment string) error {
	_, err := s.db.Conn.Exec(
		"INSERT INTO market_reviews (id, module_id, uid, username, rating, comment, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		fmt.Sprintf("rev_%d", time.Now().UnixNano()), moduleID, uid, username, rating, comment, time.Now(),
	)
	return err
}

func (s *SQLiteMarketService) GetReviews(moduleID string) []*domain.MarketReview {
	rows, _ := s.db.Conn.Query(
		"SELECT id, module_id, uid, username, rating, comment, created_at FROM market_reviews WHERE module_id = ? ORDER BY created_at DESC",
		moduleID,
	)
	defer rows.Close()
	var reviews []*domain.MarketReview
	for rows.Next() {
		var r domain.MarketReview
		rows.Scan(&r.ID, &r.ModuleID, &r.UID, &r.Username, &r.Rating, &r.Comment, &r.CreatedAt)
		reviews = append(reviews, &r)
	}
	return reviews
}

func (s *SQLiteMarketService) PublishModule(mod *domain.MarketModule) (*domain.MarketModule, error) {
	mod.ID = fmt.Sprintf("mod_%d", time.Now().UnixMilli())
	mod.Slug = strings.ToLower(strings.ReplaceAll(mod.Title, " ", "-"))
	mod.CreatedAt = time.Now()
	mod.UpdatedAt = time.Now()

	if mod.Dependencies == "" {
		mod.Dependencies = "[]"
	}
	_, err := s.db.Conn.Exec(
		"INSERT INTO market_modules (id, title, slug, description, category, tags, version, version_code, changelog, parent_id, author, author_uid, license, dependencies, cover_image, stars, installs, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?)",
		mod.ID, mod.Title, mod.Slug, mod.Description, mod.Category, mod.Tags, mod.Version, mod.VersionCode, mod.Changelog, mod.ParentID, mod.Author, mod.AuthorUID, mod.License, mod.Dependencies, mod.CoverImage, mod.CreatedAt, mod.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Create initial version record
	_, err = s.db.Conn.Exec(
		"INSERT INTO module_versions (module_id, version, changelog, created_at) VALUES (?, ?, ?, ?)",
		mod.ID, mod.Version, mod.Changelog, mod.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return mod, nil
}

func (s *SQLiteMarketService) GetModuleVersions(slug string) ([]*domain.ModuleVersion, error) {
	var modID string
	err := s.db.Conn.QueryRow("SELECT id FROM market_modules WHERE slug = ?", slug).Scan(&modID)
	if err != nil {
		return nil, fmt.Errorf("module not found: %w", err)
	}

	rows, err := s.db.Conn.Query(
		"SELECT id, module_id, version, COALESCE(changelog,''), COALESCE(file_hash,''), COALESCE(file_path,''), created_at FROM module_versions WHERE module_id = ? ORDER BY created_at DESC",
		modID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []*domain.ModuleVersion
	for rows.Next() {
		var v domain.ModuleVersion
		if err := rows.Scan(&v.ID, &v.ModuleID, &v.Version, &v.Changelog, &v.FileHash, &v.FilePath, &v.CreatedAt); err != nil {
			continue
		}
		versions = append(versions, &v)
	}
	return versions, nil
}

func (s *SQLiteMarketService) RollbackModule(slug, version string) (*domain.MarketModule, error) {
	mod, err := s.GetModule(slug)
	if err != nil {
		return nil, err
	}

	var rollbackVersion domain.ModuleVersion
	err = s.db.Conn.QueryRow(
		"SELECT id, module_id, version, COALESCE(changelog,''), COALESCE(file_hash,''), COALESCE(file_path,''), created_at FROM module_versions WHERE module_id = ? AND version = ? ORDER BY created_at DESC LIMIT 1",
		mod.ID, version,
	).Scan(&rollbackVersion.ID, &rollbackVersion.ModuleID, &rollbackVersion.Version, &rollbackVersion.Changelog, &rollbackVersion.FileHash, &rollbackVersion.FilePath, &rollbackVersion.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("version %s not found for module %s", version, slug)
	}

	// Record current version as parent before rollback
	parentID := mod.ID

	// Update module version and changelog
	_, err = s.db.Conn.Exec(
		"UPDATE market_modules SET version = ?, changelog = ?, parent_id = ?, updated_at = ? WHERE id = ?",
		rollbackVersion.Version, rollbackVersion.Changelog, parentID, time.Now(), mod.ID,
	)
	if err != nil {
		return nil, err
	}

	// Create a rollback version record
	_, err = s.db.Conn.Exec(
		"INSERT INTO module_versions (module_id, version, changelog, created_at) VALUES (?, ?, ?, ?)",
		mod.ID, rollbackVersion.Version, "Rolled back to "+rollbackVersion.Version, time.Now(),
	)

	return s.GetModule(slug)
}

func (s *SQLiteMarketService) UpdateModuleVersion(slug, version, changelog string) (*domain.MarketModule, error) {
	mod, err := s.GetModule(slug)
	if err != nil {
		return nil, err
	}

	mod.ParentID = mod.ID
	mod.Version = version
	mod.Changelog = changelog
	mod.UpdatedAt = time.Now()

	_, err = s.db.Conn.Exec(
		"UPDATE market_modules SET version = ?, changelog = ?, parent_id = ?, updated_at = ? WHERE id = ?",
		mod.Version, mod.Changelog, mod.ParentID, mod.UpdatedAt, mod.ID,
	)
	if err != nil {
		return nil, err
	}

	// Create version record
	_, err = s.db.Conn.Exec(
		"INSERT INTO module_versions (module_id, version, changelog, created_at) VALUES (?, ?, ?, ?)",
		mod.ID, mod.Version, mod.Changelog, mod.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return mod, nil
}

func (s *SQLiteMarketService) UpdateTrendingModules(limit int) []*domain.MarketModule {
	rows, _ := s.db.Conn.Query(
		"SELECT "+moduleColumns+" FROM market_modules WHERE stars > 100 ORDER BY stars DESC LIMIT ?",
		limit,
	)
	defer rows.Close()
	var modules []*domain.MarketModule
	for rows.Next() {
		m, err := scanModule(rows)
		if err != nil {
			continue
		}
		modules = append(modules, m)
	}
	return modules
}

func (s *SQLiteMarketService) TrendingModules(limit int) []*domain.MarketModule {
	return s.UpdateTrendingModules(limit)
}

func (s *SQLiteMarketService) Categories() []string {
	return []string{"system", "ui", "audio", "display", "utility"}
}

// ===== Screenshots =====

func (s *SQLiteMarketService) AddScreenshot(slug, url string) (*domain.ModuleScreenshot, error) {
	mod, err := s.GetModule(slug)
	if err != nil {
		return nil, err
	}

	// Check count
	var count int
	s.db.Conn.QueryRow("SELECT COUNT(*) FROM module_screenshots WHERE module_id = ?", mod.ID).Scan(&count)
	if count >= 5 {
		return nil, fmt.Errorf("maximum 5 screenshots allowed")
	}

	var maxOrder int
	s.db.Conn.QueryRow("SELECT COALESCE(MAX(sort_order), -1) FROM module_screenshots WHERE module_id = ?", mod.ID).Scan(&maxOrder)

	result, err := s.db.Conn.Exec(
		"INSERT INTO module_screenshots (module_id, url, sort_order) VALUES (?, ?, ?)",
		mod.ID, url, maxOrder+1,
	)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &domain.ModuleScreenshot{ID: id, ModuleID: mod.ID, URL: url, SortOrder: maxOrder + 1}, nil
}

func (s *SQLiteMarketService) GetScreenshots(slug string) ([]*domain.ModuleScreenshot, error) {
	mod, err := s.GetModule(slug)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Conn.Query(
		"SELECT id, module_id, url, sort_order, created_at FROM module_screenshots WHERE module_id = ? ORDER BY sort_order ASC",
		mod.ID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*domain.ModuleScreenshot
	for rows.Next() {
		var ss domain.ModuleScreenshot
		if err := rows.Scan(&ss.ID, &ss.ModuleID, &ss.URL, &ss.SortOrder, &ss.CreatedAt); err != nil {
			continue
		}
		list = append(list, &ss)
	}
	return list, nil
}

func (s *SQLiteMarketService) DeleteScreenshot(id int64) error {
	_, err := s.db.Conn.Exec("DELETE FROM module_screenshots WHERE id = ?", id)
	return err
}

func (s *SQLiteMarketService) UpdateModuleCoverImage(slug, coverURL string) error {
	_, err := s.db.Conn.Exec("UPDATE market_modules SET cover_image = ? WHERE slug = ?", coverURL, slug)
	return err
}

func (s *SQLiteMarketService) GetScreenshotsByModuleID(moduleID string) ([]*domain.ModuleScreenshot, error) {
	rows, err := s.db.Conn.Query(
		"SELECT id, module_id, url, sort_order, created_at FROM module_screenshots WHERE module_id = ? ORDER BY sort_order ASC",
		moduleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*domain.ModuleScreenshot
	for rows.Next() {
		var ss domain.ModuleScreenshot
		if err := rows.Scan(&ss.ID, &ss.ModuleID, &ss.URL, &ss.SortOrder, &ss.CreatedAt); err != nil {
			continue
		}
		list = append(list, &ss)
	}
	return list, nil
}

// ===== Dependency Resolution =====

func (s *SQLiteMarketService) GetModuleDependencies(slug string) ([]domain.ModuleDependency, error) {
	mod, err := s.GetModule(slug)
	if err != nil {
		return nil, err
	}
	if mod.Dependencies == "" || mod.Dependencies == "[]" {
		return []domain.ModuleDependency{}, nil
	}
	var deps []domain.ModuleDependency
	if err := json.Unmarshal([]byte(mod.Dependencies), &deps); err != nil {
		return nil, fmt.Errorf("parse dependencies: %w", err)
	}
	return deps, nil
}

func (s *SQLiteMarketService) resolveDeps(slug string, visited map[string]bool, level int) (*domain.DependencyNode, error) {
	if visited[slug] {
		return nil, fmt.Errorf("circular dependency detected: %s", slug)
	}
	visited[slug] = true

	mod, err := s.GetModule(slug)
	if err != nil {
		return nil, err
	}

	node := &domain.DependencyNode{Module: mod, Level: level}

	deps, err := s.GetModuleDependencies(slug)
	if err != nil {
		return node, nil
	}

	for _, dep := range deps {
		if dep.Optional {
			continue
		}
		child, err := s.resolveDeps(dep.ID, visited, level+1)
		if err != nil {
			return node, fmt.Errorf("resolve dep %s: %w", dep.ID, err)
		}
		if child != nil {
			node.Children = append(node.Children, child)
		}
	}

	return node, nil
}

func (s *SQLiteMarketService) ResolveDependencies(slug string) (*domain.DependencyNode, error) {
	visited := make(map[string]bool)
	return s.resolveDeps(slug, visited, 0)
}

func (s *SQLiteMarketService) CheckDependencyConflicts(slug string) ([]domain.Conflict, error) {
	var conflicts []domain.Conflict

	deps, err := s.GetModuleDependencies(slug)
	if err != nil {
		return nil, err
	}

	visited := make(map[string]bool)
	visited[slug] = true

	for _, dep := range deps {
		if err := s.checkDepCycle(dep.ID, visited, &conflicts, slug); err != nil {
			conflicts = append(conflicts, domain.Conflict{
				ModuleA: slug,
				ModuleB: dep.ID,
				Type:    "circular",
				Detail:  err.Error(),
			})
		}
	}

	for _, dep := range deps {
		depMod, err := s.GetModule(dep.ID)
		if err != nil {
			continue
		}
		if dep.MinVersion != "" && depMod.Version != "" {
			if cmp := compareVersions(depMod.Version, dep.MinVersion); cmp < 0 {
				conflicts = append(conflicts, domain.Conflict{
					ModuleA: slug,
					ModuleB: dep.ID,
					Type:    "version_mismatch",
					Detail: fmt.Sprintf("需要 %s >= %s，当前版本 %s", dep.ID, dep.MinVersion, depMod.Version),
				})
			}
		}
	}

	return conflicts, nil
}

func (s *SQLiteMarketService) checkDepCycle(slug string, visited map[string]bool, conflicts *[]domain.Conflict, rootSlug string) error {
	if visited[slug] {
		return fmt.Errorf("circular dependency: %s", slug)
	}
	visited[slug] = true

	deps, err := s.GetModuleDependencies(slug)
	if err != nil {
		visited[slug] = false
		return nil
	}

	for _, dep := range deps {
		if err := s.checkDepCycle(dep.ID, visited, conflicts, rootSlug); err != nil {
			visited[slug] = false
			return err
		}
	}

	visited[slug] = false
	return nil
}

func (s *SQLiteMarketService) GetModuleDemo(slug string) (*domain.ModuleDemo, error) {
	mod, err := s.GetModule(slug)
	if err != nil {
		return nil, fmt.Errorf("module not found: %w", err)
	}

	demo := &domain.ModuleDemo{
		Slug:  mod.Slug,
		Title: mod.Title,
	}

	switch mod.Category {
	case "system":
		demo.Props = []domain.DemoProp{
			{Path: "system/build.prop", Prop: "ro.build.version.sdk", Before: "34", After: "35"},
			{Path: "system/build.prop", Prop: "ro.build.version.release", Before: "14", After: "15"},
			{Path: "system/build.prop", Prop: "ro.product.model", Before: "Pixel 8", After: "Pixel 8 Pro"},
		}
		demo.Files = []string{"/data/adb/modules/" + mod.Slug + "/system/build.prop"}
		demo.SimulatedOutput = "  MKSTOCK 1.0.0 (2024-01-15)\n" +
			"  - Installing Magisk Module: " + mod.Title + "\n" +
			"  - Extracting module files...\n" +
			"  - Mounting system overlay...\n" +
			"  - Patching system/build.prop (3 props)\n" +
			"    · ro.build.version.sdk: 34 → 35\n" +
			"    · ro.build.version.release: 14 → 15\n" +
			"    · ro.product.model: Pixel 8 → Pixel 8 Pro\n" +
			"  - Setting permissions...\n" +
			"  - Done! Reboot recommended."
	case "ui":
		demo.Props = []domain.DemoProp{
			{Path: "system/build.prop", Prop: "ro.config.miui_round_icon", Before: "0", After: "1"},
			{Path: "system/build.prop", Prop: "persist.sys.ui.hw", Before: "0", After: "1"},
		}
		demo.Files = []string{"/data/adb/modules/" + mod.Slug + "/system/media/theme/default"}
		demo.SimulatedOutput = "  MKSTOCK 1.0.0 (2024-01-15)\n" +
			"  - Installing Magisk Module: " + mod.Title + "\n" +
			"  - Extracting module files...\n" +
			"  - Deploying UI theme overlay...\n" +
			"  - Patching system/build.prop (2 props)\n" +
			"    · ro.config.miui_round_icon: 0 → 1\n" +
			"    · persist.sys.ui.hw: 0 → 1\n" +
			"  - Applying icon pack...\n" +
			"  - Done! Reboot recommended."
	case "audio":
		demo.Props = []domain.DemoProp{
			{Path: "system/build.prop", Prop: "ro.config.media_vol_steps", Before: "15", After: "30"},
			{Path: "system/build.prop", Prop: "ro.config.vc_call_vol_steps", Before: "7", After: "15"},
			{Path: "system/etc/audio_policy.conf", Prop: "sampling_rates", Before: "48000", After: "96000"},
		}
		demo.Files = []string{"/data/adb/modules/" + mod.Slug + "/system/etc/audio_policy.conf"}
		demo.SimulatedOutput = "  MKSTOCK 1.0.0 (2024-01-15)\n" +
			"  - Installing Magisk Module: " + mod.Title + "\n" +
			"  - Extracting module files...\n" +
			"  - Patching audio policy...\n" +
			"  - Modifying system/build.prop (2 props)\n" +
			"    · ro.config.media_vol_steps: 15 → 30\n" +
			"    · ro.config.vc_call_vol_steps: 7 → 15\n" +
			"  - Patching audio_policy.conf\n" +
			"    · sampling_rates: 48000 → 96000\n" +
			"  - Done! Reboot recommended."
	case "display":
		demo.Props = []domain.DemoProp{
			{Path: "system/build.prop", Prop: "ro.sf.lcd_density", Before: "440", After: "380"},
			{Path: "system/build.prop", Prop: "persist.sys.powersaving", Before: "1", After: "0"},
		}
		demo.Files = []string{"/data/adb/modules/" + mod.Slug + "/system/etc/display_hal.conf"}
		demo.SimulatedOutput = "  MKSTOCK 1.0.0 (2024-01-15)\n" +
			"  - Installing Magisk Module: " + mod.Title + "\n" +
			"  - Extracting module files...\n" +
			"  - Configuring display settings...\n" +
			"  - Patching system/build.prop (2 props)\n" +
			"    · ro.sf.lcd_density: 440 → 380\n" +
			"    · persist.sys.powersaving: 1 → 0\n" +
			"  - Deploying display HAL config...\n" +
			"  - Done! Reboot recommended."
	default:
		demo.Props = []domain.DemoProp{
			{Path: "system/build.prop", Prop: "ro.config.custom_prop", Before: "(unset)", After: mod.Title},
		}
		demo.Files = []string{"/data/adb/modules/" + mod.Slug + "/system/etc/" + mod.Slug + ".conf"}
		demo.SimulatedOutput = "  MKSTOCK 1.0.0 (2024-01-15)\n" +
			"  - Installing Magisk Module: " + mod.Title + "\n" +
			"  - Extracting module files...\n" +
			"  - Applying configuration...\n" +
			"  - Patching system/build.prop\n" +
			"    · ro.config.custom_prop: (unset) → " + mod.Title + "\n" +
			"  - Deploying config files...\n" +
			"  - Done! Reboot recommended."
	}

	return demo, nil
}

func (s *SQLiteMarketService) SetModuleTags(slug string, tagIDs []int) error {
	tx, err := s.db.Conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.Exec("DELETE FROM module_tag_relations WHERE module_slug = ?", slug)
	for _, tagID := range tagIDs {
		tx.Exec("INSERT OR IGNORE INTO module_tag_relations (module_slug, tag_id) VALUES (?, ?)", slug, tagID)
		tx.Exec("UPDATE module_tags SET usage_count = (SELECT COUNT(*) FROM module_tag_relations WHERE tag_id = ?) WHERE id = ?", tagID, tagID)
	}
	return tx.Commit()
}

func (s *SQLiteMarketService) GetModuleTags(slug string) ([]domain.ModuleTag, error) {
	rows, err := s.db.Conn.Query(
		"SELECT t.id, t.name, t.color, t.usage_count FROM module_tags t INNER JOIN module_tag_relations r ON t.id = r.tag_id WHERE r.module_slug = ?",
		slug,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []domain.ModuleTag
	for rows.Next() {
		var t domain.ModuleTag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.UsageCount); err != nil {
			continue
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func (s *SQLiteMarketService) CompareModules(slug1, slug2 string) (*domain.ModuleComparison, error) {
	m1, err := s.GetModule(slug1)
	if err != nil {
		return nil, fmt.Errorf("module %s not found", slug1)
	}
	m2, err := s.GetModule(slug2)
	if err != nil {
		return nil, fmt.Errorf("module %s not found", slug2)
	}

	getDepCount := func(depsJSON string) int {
		if depsJSON == "" || depsJSON == "[]" {
			return 0
		}
		var deps []domain.ModuleDependency
		if err := json.Unmarshal([]byte(depsJSON), &deps); err != nil {
			return 0
		}
		return len(deps)
	}

	getAvgRating := func(moduleID string) float64 {
		var avg sql.NullFloat64
		s.db.Conn.QueryRow("SELECT AVG(rating) FROM market_reviews WHERE module_id = ?", moduleID).Scan(&avg)
		if avg.Valid {
			return avg.Float64
		}
		return 0
	}

	return &domain.ModuleComparison{
		TitleA:       m1.Title,
		TitleB:       m2.Title,
		DescriptionA: m1.Description,
		DescriptionB: m2.Description,
		VersionA:     m1.Version,
		VersionB:     m2.Version,
		StarsA:       m1.Stars,
		StarsB:       m2.Stars,
		InstallsA:    m1.Installs,
		InstallsB:    m2.Installs,
		CategoryA:    m1.Category,
		CategoryB:    m2.Category,
		AuthorA:      m1.Author,
		AuthorB:      m2.Author,
		LicenseA:     m1.License,
		LicenseB:     m2.License,
		DepCountA:    getDepCount(m1.Dependencies),
		DepCountB:    getDepCount(m2.Dependencies),
		RatingA:      getAvgRating(m1.ID),
		RatingB:      getAvgRating(m2.ID),
	}, nil
}

type HealthScore struct {
	Score    int              `json:"score"`
	Level    string           `json:"level"`
	Details  []HealthDetail   `json:"details"`
}

type HealthDetail struct {
	Name   string `json:"name"`
	Label  string `json:"label"`
	Score  int    `json:"score"`
	Max    int    `json:"max"`
}

func (s *SQLiteMarketService) GetModuleHealth(slug string) (*HealthScore, error) {
	mod, err := s.GetModule(slug)
	if err != nil {
		return nil, err
	}

	hs := &HealthScore{Details: []HealthDetail{}}
	updatedAt := mod.UpdatedAt
	daysSinceUpdate := int(time.Since(updatedAt).Hours() / 24)

	updateScore := 0
	switch {
	case daysSinceUpdate <= 30:
		updateScore = 20
	case daysSinceUpdate <= 90:
		updateScore = 10
	}
	hs.Details = append(hs.Details, HealthDetail{Name: "update", Label: "更新时间", Score: updateScore, Max: 20})

	var reviewCount int
	var avgRating float64
	s.db.Conn.QueryRow("SELECT COUNT(*), COALESCE(AVG(rating), 0) FROM market_reviews WHERE module_id = ?", mod.ID).Scan(&reviewCount, &avgRating)

	reviewScore := reviewCount / 5 * 5
	if reviewScore > 20 {
		reviewScore = 20
	}
	hs.Details = append(hs.Details, HealthDetail{Name: "reviews", Label: "评价数量", Score: reviewScore, Max: 20})

	ratingScore := 0
	switch {
	case avgRating >= 4.5:
		ratingScore = 20
	case avgRating >= 4.0:
		ratingScore = 15
	case avgRating >= 3.0:
		ratingScore = 10
	}
	hs.Details = append(hs.Details, HealthDetail{Name: "rating", Label: "评价均分", Score: ratingScore, Max: 20})

	installScore := 0
	if mod.Installs > 100 {
		installScore = 10
	} else if mod.Installs > 50 {
		installScore = 5
	}
	hs.Details = append(hs.Details, HealthDetail{Name: "installs", Label: "安装量", Score: installScore, Max: 10})

	depScore := 10
	deps, _ := s.GetModuleDependencies(slug)
	for _, dep := range deps {
		depMod, depErr := s.GetModule(dep.ID)
		if depErr != nil {
			depScore -= 5
			continue
		}
		depDays := int(time.Since(depMod.UpdatedAt).Hours() / 24)
		if depDays > 365 {
			depScore -= 5
		}
	}
	if depScore < 0 {
		depScore = 0
	}
	hs.Details = append(hs.Details, HealthDetail{Name: "dependencies", Label: "依赖健康", Score: depScore, Max: 10})

	total := updateScore + reviewScore + ratingScore + installScore + depScore
	if total > 100 {
		total = 100
	}
	hs.Score = total
	switch {
	case total >= 80:
		hs.Level = "excellent"
	case total >= 60:
		hs.Level = "good"
	default:
		hs.Level = "warning"
	}
	return hs, nil
}

func (s *SQLiteMarketService) GetModuleHealthDetails(slug string) (*HealthScore, error) {
	return s.GetModuleHealth(slug)
}

func compareVersions(a, b string) int {
	aParts := strings.Split(strings.TrimPrefix(a, "v"), ".")
	bParts := strings.Split(strings.TrimPrefix(b, "v"), ".")

	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		var aNum, bNum int
		if i < len(aParts) {
			fmt.Sscanf(aParts[i], "%d", &aNum)
		}
		if i < len(bParts) {
			fmt.Sscanf(bParts[i], "%d", &bNum)
		}
		if aNum < bNum {
			return -1
		}
		if aNum > bNum {
			return 1
		}
	}
	return 0
}
