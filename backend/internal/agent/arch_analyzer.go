package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// ArchAnalyzer performs deep architecture analysis on projects.
type ArchAnalyzer struct {
	mu sync.Mutex
}

// ArchReport holds the analysis result.
type ArchReport struct {
	ProjectRoot  string        `json:"project_root"`
	Language     string        `json:"language"`
	TotalFiles   int           `json:"total_files"`
	TotalLines   int           `json:"total_lines"`
	Dependencies []string      `json:"dependencies"`
	Packages     []PackageInfo `json:"packages"`
	EntryPoints  []string      `json:"entry_points"`
	Architecture string        `json:"architecture"` // "monolith", "microservice", "library", "cli"
	Coupling     float64       `json:"coupling"`     // 0.0-1.0 coupling score
	Cohesion     float64       `json:"cohesion"`     // 0.0-1.0 cohesion score
	Suggestions  []string      `json:"suggestions"`
	Issues       []ArchIssue   `json:"issues"`
}

// PackageInfo holds info about a package/directory.
type PackageInfo struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	Files     int      `json:"files"`
	Lines     int      `json:"lines"`
	Imports   []string `json:"imports"`
	Functions int      `json:"functions"`
	HasTests  bool     `json:"has_tests"`
}

// ArchIssue represents an architecture issue.
type ArchIssue struct {
	Severity string `json:"severity"` // "high", "medium", "low"
	Category string `json:"category"` // "coupling", "naming", "structure", "security"
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
}

// NewArchAnalyzer creates a new architecture analyzer.
func NewArchAnalyzer() *ArchAnalyzer {
	return &ArchAnalyzer{}
}

// AnalyzeProject performs full architecture analysis.
func (aa *ArchAnalyzer) AnalyzeProject(projectDir string) (*ArchReport, error) {
	aa.mu.Lock()
	defer aa.mu.Unlock()

	report := &ArchReport{
		ProjectRoot: projectDir,
	}

	// Detect language
	report.Language = aa.detectLanguage(projectDir)

	// Walk the project
	err := filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// Skip hidden dirs and vendor
		if strings.Contains(path, "/.") || strings.Contains(path, "/vendor/") {
			return nil
		}

		report.TotalFiles++

		// Count lines
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		lines := strings.Split(string(content), "\n")
		report.TotalLines += len(lines)

		// Check for entry points
		if aa.isEntryPoint(path, string(content)) {
			report.EntryPoints = append(report.EntryPoints, path)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Detect architecture pattern
	report.Architecture = aa.detectArchitecture(projectDir, report)

	// Detect dependencies
	report.Dependencies = aa.detectDependencies(projectDir)

	// Calculate coupling and cohesion
	report.Coupling, report.Cohesion = aa.calculateMetrics(projectDir)

	// Generate suggestions
	report.Suggestions = aa.generateSuggestions(report)

	// Detect issues
	report.Issues = aa.detectIssues(projectDir)

	return report, nil
}

// detectLanguage detects the primary language of a project.
func (aa *ArchAnalyzer) detectLanguage(projectDir string) string {
	langCount := map[string]int{}
	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		switch ext {
		case ".go":
			langCount["go"]++
		case ".rs":
			langCount["rust"]++
		case ".py":
			langCount["python"]++
		case ".js", ".ts", ".tsx", ".jsx":
			langCount["javascript"]++
		case ".java":
			langCount["java"]++
		case ".cpp", ".c", ".h":
			langCount["c"]++
		}
		return nil
	})

	best := "unknown"
	bestCount := 0
	for lang, count := range langCount {
		if count > bestCount {
			best = lang
			bestCount = count
		}
	}
	return best
}

// isEntryPoint checks if a file is likely an entry point.
func (aa *ArchAnalyzer) isEntryPoint(path string, content string) bool {
	base := filepath.Base(path)
	// Go
	if base == "main.go" || base == "cmd.go" {
		return true
	}
	if strings.HasSuffix(path, "/cmd/") && base == "main.go" {
		return true
	}
	// Rust
	if base == "main.rs" || base == "lib.rs" {
		return true
	}
	// Python
	if base == "__main__.py" || base == "manage.py" || base == "app.py" {
		return true
	}
	// JavaScript/TypeScript
	if base == "index.ts" || base == "index.js" || base == "server.ts" || base == "app.ts" {
		return true
	}
	return false
}

// detectArchitecture detects the architecture pattern.
func (aa *ArchAnalyzer) detectArchitecture(projectDir string, report *ArchReport) string {
	hasCmd := false
	hasInternal := false
	hasPkg := false
	hasAPI := false

	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		switch base {
		case "cmd":
			hasCmd = true
		case "internal":
			hasInternal = true
		case "pkg":
			hasPkg = true
		case "api", "handlers", "routes":
			hasAPI = true
		}
		return nil
	})

	// Go project patterns
	if hasCmd && hasInternal {
		return "monolith" // standard Go project layout
	}
	if hasPkg {
		return "library"
	}
	if hasAPI && hasInternal {
		return "microservice"
	}

	// Default
	if report.TotalFiles < 10 {
		return "small_project"
	}
	return "monolith"
}

// detectDependencies detects external dependencies.
func (aa *ArchAnalyzer) detectDependencies(projectDir string) []string {
	var deps []string

	// Go: go.mod
	goMod := filepath.Join(projectDir, "go.mod")
	if content, err := os.ReadFile(goMod); err == nil {
		lines := strings.Split(string(content), "\n")
		inRequire := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "require (" {
				inRequire = true
				continue
			}
			if inRequire && line == ")" {
				inRequire = false
				continue
			}
			if inRequire && line != "" && !strings.HasPrefix(line, "//") {
				parts := strings.Fields(line)
				if len(parts) >= 1 {
					deps = append(deps, parts[0])
				}
			}
		}
	}

	// Rust: Cargo.toml
	cargoToml := filepath.Join(projectDir, "Cargo.toml")
	if content, err := os.ReadFile(cargoToml); err == nil {
		re := regexp.MustCompile(`(\w[\w-]*)\s*=\s*"`)
		matches := re.FindAllStringSubmatch(string(content), -1)
		for _, m := range matches {
			if len(m) >= 2 && m[1] != "name" && m[1] != "version" && m[1] != "edition" {
				deps = append(deps, m[1])
			}
		}
	}

	return deps
}

// calculateMetrics calculates coupling and cohesion scores.
func (aa *ArchAnalyzer) calculateMetrics(projectDir string) (float64, float64) {
	coupling := 0.5
	cohesion := 0.5

	// Simple heuristic based on directory structure
	dirCount := 0
	fileCount := 0
	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			dirCount++
		} else {
			fileCount++
		}
		return nil
	})

	if dirCount > 0 {
		cohesion = float64(fileCount) / float64(dirCount) / 10.0
		if cohesion > 1.0 {
			cohesion = 1.0
		}
	}

	return coupling, cohesion
}

// generateSuggestions generates architecture improvement suggestions.
func (aa *ArchAnalyzer) generateSuggestions(report *ArchReport) []string {
	var suggestions []string

	if report.TotalFiles > 100 {
		suggestions = append(suggestions, "项目文件数较多（>100），考虑模块化拆分")
	}
	if report.Coupling > 0.7 {
		suggestions = append(suggestions, "耦合度较高，建议减少模块间依赖")
	}
	if len(report.EntryPoints) == 0 {
		suggestions = append(suggestions, "未检测到明确的入口点文件")
	}
	if len(report.Dependencies) > 50 {
		suggestions = append(suggestions, "外部依赖较多（>50），考虑精简依赖")
	}

	return suggestions
}

// detectIssues detects architecture issues.
func (aa *ArchAnalyzer) detectIssues(projectDir string) []ArchIssue {
	var issues []ArchIssue

	// Check for common issues
	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// Skip hidden dirs
		if strings.Contains(path, "/.") {
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".rs" && ext != ".py" && ext != ".js" && ext != ".ts" {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		contentStr := string(content)
		lines := strings.Split(contentStr, "\n")

		// Check for large files
		if len(lines) > 500 {
			issues = append(issues, ArchIssue{
				Severity: "medium",
				Category: "structure",
				Message:  fmt.Sprintf("文件行数过多（%d 行），建议拆分", len(lines)),
				File:     path,
			})
		}

		// Check for TODO/FIXME
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "// TODO") || strings.HasPrefix(trimmed, "# TODO") {
				issues = append(issues, ArchIssue{
					Severity: "low",
					Category: "maintenance",
					Message:  "存在 TODO 注释，建议清理",
					File:     path,
				})
				break // one per file
			}
		}

		return nil
	})

	return issues
}
