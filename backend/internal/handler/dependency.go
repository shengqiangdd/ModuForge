package handler

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type DependencyHandler struct {
	db *sql.DB
	fr *service.FileContentRepo // S3-first content access (optional)
}

func NewDependencyHandler(db *sql.DB) *DependencyHandler {
	return &DependencyHandler{db: db}
}

// SetFileContentRepo injects the S3-first file content repository.
func (h *DependencyHandler) SetFileContentRepo(fr *service.FileContentRepo) {
	h.fr = fr
}

type DependencyAnalysis struct {
	ModuleID     string              `json:"module_id"`
	Dependencies []DependencyItem    `json:"dependencies"`
	Missing      []DependencyItem    `json:"missing"`
	Tree         *DependencyTreeNode `json:"tree"`
	Warnings     []string            `json:"warnings"`
}

type DependencyItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Version       string `json:"version,omitempty"`
	MinVersion    string `json:"min_version,omitempty"`
	Source        string `json:"source"` // file_reference, system_prop, so_library
	ReferencePath string `json:"reference_path"`
	Required      bool   `json:"required"`
}

type DependencyTreeNode struct {
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Version  string              `json:"version,omitempty"`
	Level    int                 `json:"level"`
	Status   string              `json:"status"` // resolved, missing, conflict
	Children []*DependencyTreeNode `json:"children,omitempty"`
}

// POST /projects/:id/analyze-deps — Analyze module dependencies
func (h *DependencyHandler) AnalyzeDependencies(c fiber.Ctx) error {
	projectID := c.Params("id")

	// Get all project files (S3 first)
	fileMap, err := h.fr.ReadAllContent(c.Context(), projectID)
	if err != nil {
		return InternalError(c, "读取项目文件失败")
	}

	type FileData struct {
		Path    string
		Content string
	}
	var files []FileData
	for path, content := range fileMap {
		files = append(files, FileData{Path: path, Content: content})
	}

	analysis := &DependencyAnalysis{
		ModuleID:     projectID,
		Dependencies: []DependencyItem{},
		Missing:      []DependencyItem{},
		Warnings:     []string{},
	}

	// Known module dependencies map (module_id -> module info)
	knownModules := make(map[string]map[string]string)
	modRows, err := h.db.Query("SELECT id, title, version FROM market_modules")
	if err == nil {
		defer modRows.Close()
		for modRows.Next() {
			var id, title, version string
			if err := modRows.Scan(&id, &title, &version); err == nil {
				knownModules[strings.ToLower(title)] = map[string]string{
					"id": id, "title": title, "version": version,
				}
				knownModules[strings.ToLower(id)] = map[string]string{
					"id": id, "title": title, "version": version,
				}
			}
		}
	}

	// Pattern to find module references in scripts
	moduleRefPattern := regexp.MustCompile(`(?:require|import|source|load)\s+(?:module[_-]?)?['"]?([a-zA-Z0-9_-]+)['"]?`)
	// Pattern for system.prop references
	propRefPattern := regexp.MustCompile(`ro\.(?:module|mod)\.([a-zA-Z0-9_-]+)`)
	// Pattern for .so library references
	soPattern := regexp.MustCompile(`(?:require|dlopen|load)\s+['"]?([a-zA-Z0-9_]+\.so)['"]?`)

	for _, f := range files {
		ext := strings.ToLower(f.Path)
		isScript := strings.HasSuffix(ext, ".sh") || strings.HasSuffix(ext, ".bash") || strings.Contains(ext, "install") || strings.Contains(ext, "config")
		isProp := strings.HasSuffix(ext, ".prop")

		if isScript {
			matches := moduleRefPattern.FindAllStringSubmatch(f.Content, -1)
			for _, match := range matches {
				if len(match) > 1 {
					name := strings.ToLower(match[1])
					item := DependencyItem{
						Name:          name,
						Source:        "file_reference",
						ReferencePath: f.Path,
						Required:      true,
					}
					if mod, ok := knownModules[name]; ok {
						item.ID = mod["id"]
						item.Version = mod["version"]
					} else {
						item.ID = name
						analysis.Missing = append(analysis.Missing, item)
					}
					analysis.Dependencies = append(analysis.Dependencies, item)
				}
			}
		}

		if isProp {
			matches := propRefPattern.FindAllStringSubmatch(f.Content, -1)
			for _, match := range matches {
				if len(match) > 1 {
					name := strings.ToLower(match[1])
					item := DependencyItem{
						Name:          name,
						Source:        "system_prop",
						ReferencePath: f.Path,
						Required:      false,
					}
					if mod, ok := knownModules[name]; ok {
						item.ID = mod["id"]
						item.Version = mod["version"]
					} else {
						item.ID = name
					}
					analysis.Dependencies = append(analysis.Dependencies, item)
				}
			}
		}

		// Check for .so references in any file
		soMatches := soPattern.FindAllStringSubmatch(f.Content, -1)
		for _, match := range soMatches {
			if len(match) > 1 {
				item := DependencyItem{
					ID:            match[1],
					Name:          match[1],
					Source:        "so_library",
					ReferencePath: f.Path,
					Required:      true,
				}
				analysis.Dependencies = append(analysis.Dependencies, item)
			}
		}
	}

	// Check for conflicts (multiple versions of same dependency)
	depCounts := make(map[string]int)
	for _, dep := range analysis.Dependencies {
		depCounts[dep.ID]++
	}
	for id, count := range depCounts {
		if count > 1 {
			analysis.Warnings = append(analysis.Warnings,
				fmt.Sprintf("依赖 '%s' 在多个位置被引用 (%d 次)", id, count))
		}
	}

	return c.JSON(analysis)
}

// GET /projects/:id/dependencies — Get dependency tree
func (h *DependencyHandler) GetDependencyTree(c fiber.Ctx) error {
	projectID := c.Params("id")

	// Get module info
	var moduleName, moduleVersion string
	err := h.db.QueryRow(
		`SELECT p.name, COALESCE((SELECT version FROM project_versions WHERE project_id=p.id ORDER BY created_at DESC LIMIT 1), '1.0.0')
		 FROM projects p WHERE p.id=?`, projectID,
	).Scan(&moduleName, &moduleVersion)
	if err != nil {
		return NotFound(c, "项目不存在")
	}

	tree := &DependencyTreeNode{
		ID:      projectID,
		Name:    moduleName,
		Version: moduleVersion,
		Level:   0,
		Status:  "resolved",
	}

	// Build tree from analysis (S3 first)
	fileMap, err := h.fr.ReadAllContent(c.Context(), projectID)
	if err == nil {
		knownModules := make(map[string]map[string]string)
		modRows, err2 := h.db.Query("SELECT id, title, version FROM market_modules")
		if err2 == nil {
			for modRows.Next() {
				var id, title, version string
				if err := modRows.Scan(&id, &title, &version); err == nil {
					knownModules[strings.ToLower(title)] = map[string]string{
						"id": id, "title": title, "version": version,
					}
				}
			}
			modRows.Close()
		}

		moduleRefPattern := regexp.MustCompile(`(?:require|import|source|load)\s+(?:module[_-]?)?['"]?([a-zA-Z0-9_-]+)['"]?`)
		seen := make(map[string]bool)

		for path, content := range fileMap {
			if strings.HasSuffix(path, ".sh") || strings.HasSuffix(path, ".bash") || strings.Contains(path, "install") {
				matches := moduleRefPattern.FindAllStringSubmatch(content, -1)
				for _, match := range matches {
					if len(match) > 1 && !seen[match[1]] {
						seen[match[1]] = true
						name := strings.ToLower(match[1])
						child := &DependencyTreeNode{
							Name:    name,
							Level:   1,
							Status:  "missing",
						}
						if mod, ok := knownModules[name]; ok {
							child.ID = mod["id"]
							child.Version = mod["version"]
							child.Status = "resolved"
						} else {
							child.ID = name
						}
						tree.Children = append(tree.Children, child)
					}
				}
			}
		}
	}

	return c.JSON(tree)
}

// POST /projects/:id/resolve-deps — Auto-resolve dependencies
func (h *DependencyHandler) ResolveDependencies(c fiber.Ctx) error {
	projectID := c.Params("id")

	var req struct {
		AutoInstall bool `json:"auto_install"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		// Default behavior
		req.AutoInstall = false
	}

	// Run analysis first (S3 first)
	fileMap, err := h.fr.ReadAllContent(c.Context(), projectID)
	if err != nil {
		return InternalError(c, "读取项目文件失败")
	}

	knownModules := make(map[string]map[string]string)
	modRows, err := h.db.Query("SELECT id, title, version FROM market_modules")
	if err == nil {
		defer modRows.Close()
		for modRows.Next() {
			var id, title, version string
			if err := modRows.Scan(&id, &title, &version); err == nil {
				knownModules[strings.ToLower(title)] = map[string]string{
					"id": id, "title": title, "version": version,
				}
			}
		}
	}

	type ResolutionResult struct {
		Resolved   []DependencyItem `json:"resolved"`
		Unresolved []DependencyItem `json:"unresolved"`
		Actions    []string         `json:"actions"`
	}

	result := ResolutionResult{
		Resolved:   []DependencyItem{},
		Unresolved: []DependencyItem{},
		Actions:    []string{},
	}

	moduleRefPattern := regexp.MustCompile(`(?:require|import|source|load)\s+(?:module[_-]?)?['"]?([a-zA-Z0-9_-]+)['"]?`)
	seen := make(map[string]bool)

	for path, content := range fileMap {
		if strings.HasSuffix(path, ".sh") || strings.HasSuffix(path, ".bash") || strings.Contains(path, "install") {
			matches := moduleRefPattern.FindAllStringSubmatch(content, -1)
			for _, match := range matches {
				if len(match) > 1 && !seen[match[1]] {
					seen[match[1]] = true
					name := strings.ToLower(match[1])
					item := DependencyItem{
						Name:          name,
						Source:        "file_reference",
						ReferencePath: path,
						Required:      true,
					}
					if mod, ok := knownModules[name]; ok {
						item.ID = mod["id"]
						item.Version = mod["version"]
						result.Resolved = append(result.Resolved, item)
						result.Actions = append(result.Actions,
							fmt.Sprintf("依赖 '%s' 已在市场中找到 (版本 %s)", name, mod["version"]))
					} else {
						item.ID = name
						result.Unresolved = append(result.Unresolved, item)
						result.Actions = append(result.Actions,
							fmt.Sprintf("警告: 依赖 '%s' 未在市场中找到", name))
					}
				}
			}
		}
	}

	return c.JSON(result)
}
