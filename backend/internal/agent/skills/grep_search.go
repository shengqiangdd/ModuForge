package skills

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"github.com/moduforge/backend/internal/agent/registry"
)

type GrepSearchSkill struct {
	projectPath string
	db          *sql.DB
}

func NewGrepSearchSkillWithDB(projectPath string, db *sql.DB) *GrepSearchSkill {
	return &GrepSearchSkill{projectPath: projectPath, db: db}
}

func (s *GrepSearchSkill) Name() string {
	return "grep_search"
}

func (s *GrepSearchSkill) Description() string {
	return `Search file contents by pattern (like grep -rn).
Input: {"pattern": "...", "project_id": "...", "include": "*.go" (optional, file glob filter), "max_results": 50 (optional)}.
Returns matching lines with file paths and line numbers.`
}

func (s *GrepSearchSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	pattern, _ := input["pattern"].(string)
	projectID, _ := input["project_id"].(string)
	includeFilter, _ := input["include"].(string)
	maxResults := 50
	if v, ok := input["max_results"].(float64); ok && v > 0 {
		maxResults = int(v)
	}

	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	projectPath := ResolveProjectPath(s.db, s.projectPath, projectID)

	// Ensure files exist on disk
	if projectID != "" {
		_ = s.syncProjectToDisk(projectID, projectPath)
	}

	// Compile regex
	re, err := regexp.Compile(pattern)
	if err != nil {
		// Fallback to literal string match
		return s.literalSearch(projectPath, pattern, includeFilter, maxResults)
	}

	var results []string
	matches := 0

	err = filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip binary/large files
		if info.Size() > 1024*1024 { // 1MB
			return nil
		}

		// Skip hidden dirs and common non-source dirs
		rel, _ := filepath.Rel(projectPath, path)
		if strings.HasPrefix(rel, ".") || strings.HasPrefix(rel, "node_modules") ||
			strings.HasPrefix(rel, "target") || strings.HasPrefix(rel, ".git") {
			return nil
		}

		// Apply include filter
		if includeFilter != "" {
			matched, _ := filepath.Match(includeFilter, filepath.Base(path))
			if !matched {
				return nil
			}
		}

		// Skip binary files by extension
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".gif", ".ico", ".so", ".a", ".o",
			".zip", ".tar", ".gz", ".exe", ".bin", ".dat":
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				relPath, _ := filepath.Rel(projectPath, path)
				results = append(results, fmt.Sprintf("%s:%d: %s", relPath, lineNum, strings.TrimSpace(line)))
				matches++
				if matches >= maxResults {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return fmt.Sprintf("No matches found for pattern: %s", pattern), nil
	}

	output := fmt.Sprintf("Found %d matches:\n\n", len(results))
	for _, r := range results {
		output += r + "\n"
	}

	return output, nil
}

func (s *GrepSearchSkill) literalSearch(projectPath, pattern, includeFilter string, maxResults int) (string, error) {
	var results []string
	matches := 0
	patternLower := strings.ToLower(pattern)

	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.Size() > 1024*1024 {
			return nil
		}
		rel, _ := filepath.Rel(projectPath, path)
		if strings.HasPrefix(rel, ".") || strings.HasPrefix(rel, "node_modules") ||
			strings.HasPrefix(rel, "target") || strings.HasPrefix(rel, ".git") {
			return nil
		}
		if includeFilter != "" {
			matched, _ := filepath.Match(includeFilter, filepath.Base(path))
			if !matched {
				return nil
			}
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if strings.Contains(strings.ToLower(line), patternLower) {
				relPath, _ := filepath.Rel(projectPath, path)
				results = append(results, fmt.Sprintf("%s:%d: %s", relPath, lineNum, strings.TrimSpace(line)))
				matches++
				if matches >= maxResults {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return fmt.Sprintf("No matches found for: %s", pattern), nil
	}

	output := fmt.Sprintf("Found %d matches:\n\n", len(results))
	for _, r := range results {
		output += r + "\n"
	}
	return output, nil
}

// syncProjectToDisk ensures project files from DB exist on disk for searching.
func (s *GrepSearchSkill) syncProjectToDisk(projectID, projectDir string) error {
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
		log.Printf("[GrepSearchSkill] synced %d files from DB for project %s", synced, projectID)
	}
	return nil
}

func (s *GrepSearchSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  true,
		Essential: false,
		Core:      true,
		NeedsDB:   false,
		NeedsLLM:  false,
	}
}
