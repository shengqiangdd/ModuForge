package skills

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type GlobSearchSkill struct {
	projectPath string
	db          *sql.DB
}

func NewGlobSearchSkill(projectPath string) *GlobSearchSkill {
	return &GlobSearchSkill{projectPath: projectPath}
}

func NewGlobSearchSkillWithDB(projectPath string, db *sql.DB) *GlobSearchSkill {
	return &GlobSearchSkill{projectPath: projectPath, db: db}
}

func (s *GlobSearchSkill) Name() string {
	return "glob_search"
}

func (s *GlobSearchSkill) Description() string {
	return `Find files matching a glob pattern.
Input: {"pattern": "*.go", "project_id": "...", "max_results": 100 (optional)}.
Returns matching file paths. Use ** for recursive matching (e.g., "**/*.js").`
}

func (s *GlobSearchSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	pattern, _ := input["pattern"].(string)
	projectID, _ := input["project_id"].(string)
	maxResults := 100
	if v, ok := input["max_results"].(float64); ok && v > 0 {
		maxResults = int(v)
	}

	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	projectPath := s.resolvePath(projectID)

	// Ensure files exist on disk
	if projectID != "" {
		_ = s.syncProjectToDisk(projectID, projectPath)
	}

	// If pattern contains **, use filepath.Walk for recursive matching
	if strings.Contains(pattern, "**") {
		return s.walkSearch(projectPath, pattern, maxResults)
	}

	// Simple glob: use filepath.Glob
	fullPattern := filepath.Join(projectPath, pattern)
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return "", fmt.Errorf("invalid glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return fmt.Sprintf("No files matching pattern: %s", pattern), nil
	}

	var results []string
	for i, m := range matches {
		if i >= maxResults {
			break
		}
		rel, _ := filepath.Rel(projectPath, m)
		results = append(results, rel)
	}

	output := fmt.Sprintf("Found %d files:\n\n", len(results))
	for _, r := range results {
		output += r + "\n"
	}
	return output, nil
}

func (s *GlobSearchSkill) walkSearch(projectPath, pattern string, maxResults int) (string, error) {
	// Convert ** glob to a simpler prefix/suffix match
	// e.g., "**/*.go" → match all .go files recursively
	// e.g., "**/test_*" → match all test_* files recursively

	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		return "", fmt.Errorf("pattern must contain exactly one **")
	}

	// parts[0] = prefix before ** (e.g., "" or "src/")
	// parts[1] = suffix after ** (e.g., "/*.go" or "/test_*.go")
	suffix := strings.TrimPrefix(parts[1], "/")

	var results []string
	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip hidden dirs
		rel, _ := filepath.Rel(projectPath, path)
		if strings.HasPrefix(rel, ".") || strings.HasPrefix(rel, "node_modules") ||
			strings.HasPrefix(rel, "target") {
			return nil
		}

		baseName := filepath.Base(path)
		matched, _ := filepath.Match(suffix, baseName)
		if matched {
			results = append(results, rel)
			if len(results) >= maxResults {
				return filepath.SkipAll
			}
		}
		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return fmt.Sprintf("No files matching pattern: %s", pattern), nil
	}

	output := fmt.Sprintf("Found %d files:\n\n", len(results))
	for _, r := range results {
		output += r + "\n"
	}
	return output, nil
}

func (s *GlobSearchSkill) resolvePath(projectID string) string {
	if s.projectPath == "" || projectID == "" {
		return s.projectPath
	}
	return ResolveProjectPath(s.db, s.projectPath, projectID)
}

// syncProjectToDisk ensures project files from DB exist on disk for searching.
func (s *GlobSearchSkill) syncProjectToDisk(projectID, projectDir string) error {
	if s.db == nil || projectID == "" {
		return nil
	}
	entries, err := os.ReadDir(projectDir)
	if err == nil && len(entries) > 0 {
		return nil
	}
	rows, err := s.db.Query(`SELECT path, content FROM project_files WHERE project_id=?`, projectID)
	if err != nil {
		return err
	}
	defer rows.Close()
	synced := 0
	for rows.Next() {
		var path, content string
		if err := rows.Scan(&path, &content); err != nil {
			continue
		}
		fullPath := filepath.Join(projectDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			continue
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			continue
		}
		synced++
	}
	if synced > 0 {
		log.Printf("[GlobSearchSkill] synced %d files from DB for project %s", synced, projectID)
	}
	return nil
}

func (s *GlobSearchSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  true,
		Essential: false,
		Core:      true,
		NeedsDB:   false,
		NeedsLLM:  false,
	}
}
