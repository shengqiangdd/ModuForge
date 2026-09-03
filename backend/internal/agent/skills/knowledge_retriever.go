package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moduforge/backend/internal/agent/registry"
	"github.com/moduforge/backend/internal/storage"
)

// KnowledgeRetrieverSkill searches for relevant code patterns and documentation
type KnowledgeRetrieverSkill struct {
	db      *sql.DB
	storage storage.StorageAdapter // optional S3 storage backend
}

func init() {
	registry.RegisterFactory("knowledge_retriever", func(deps *registry.Deps) registry.Skill {
		skill := &KnowledgeRetrieverSkill{db: deps.DB}
		if st := getStorage(deps); st != nil {
			skill.storage = st
		}
		return skill
	})
}

func (s *KnowledgeRetrieverSkill) Name() string {
	return "knowledge_retriever"
}

func (s *KnowledgeRetrieverSkill) Description() string {
	return `Search for relevant code patterns, documentation, and examples. Input: {"query": "...", "project_id": "...", "language": "...", "type": "code|doc|example", "limit": 10}`
}

type KnowledgeResult struct {
	Source    string  `json:"source"` // file path or documentation
	Type      string  `json:"type"`   // code, doc, example
	Content   string  `json:"content"`
	Relevance float64 `json:"relevance"`
	Context   string  `json:"context"` // surrounding code or description
}

type KnowledgeSearchResult struct {
	Query     string            `json:"query"`
	Results   []KnowledgeResult `json:"results"`
	Count     int               `json:"count"`
	TotalHits int               `json:"total_hits"`
}

func (s *KnowledgeRetrieverSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	query, _ := input["query"].(string)
	projectID, _ := input["project_id"].(string)
	language, _ := input["language"].(string)
	searchType, _ := input["type"].(string)
	limit := 10
	if l, ok := input["limit"].(float64); ok {
		limit = int(l)
	}

	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	if searchType == "" {
		searchType = "code"
	}

	// Search based on type
	var results []KnowledgeResult
	var err error

	switch searchType {
	case "code":
		results, err = s.searchCode(ctx, query, projectID, language, limit)
	case "doc":
		results, err = s.searchDocumentation(query, projectID, limit)
	case "example":
		results, err = s.searchExamples(query, projectID, language, limit)
	default:
		return "", fmt.Errorf("unknown type: %s (use code|doc|example)", searchType)
	}

	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	result := KnowledgeSearchResult{
		Query:     query,
		Results:   results,
		Count:     len(results),
		TotalHits: len(results),
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(output), nil
}

func (s *KnowledgeRetrieverSkill) searchCode(ctx context.Context, query, projectID, language string, limit int) ([]KnowledgeResult, error) {
	var results []KnowledgeResult

	// Search in project files if projectID is provided
	if projectID != "" && s.db != nil {
		rows, err := s.db.Query(`
			SELECT path FROM project_files 
			WHERE project_id = ?
			ORDER BY path
		`, projectID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var path string
			if err := rows.Scan(&path); err != nil {
				continue
			}
			content, err := readFileContent(ctx, s.storage, s.db, projectID, path)
			if err != nil {
				continue
			}
			// Find the matching line
			lines := strings.Split(content, "\n")
			for i, line := range lines {
				if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
					// Get context (surrounding lines)
					start := i - 2
					if start < 0 {
						start = 0
					}
					end := i + 3
					if end > len(lines) {
						end = len(lines)
					}
					context := strings.Join(lines[start:end], "\n")

					results = append(results, KnowledgeResult{
						Source:    path,
						Type:      "code",
						Content:   line,
						Relevance: 0.8,
						Context:   context,
					})
					break
				}
			}
			if len(results) >= limit {
				break
			}
		}
	}

	// Also search in local workspace if available
	workspaceDir := os.Getenv("WORKSPACE_DIR")
	if workspaceDir == "" {
		workspaceDir = "."
	}

	// Search for Go files
	if language == "" || language == "go" {
		goFiles, _ := filepath.Glob(filepath.Join(workspaceDir, "**/*.go"))
		for _, file := range goFiles {
			if len(results) >= limit {
				break
			}
			content, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			contentStr := string(content)
			if strings.Contains(strings.ToLower(contentStr), strings.ToLower(query)) {
				lines := strings.Split(contentStr, "\n")
				for i, line := range lines {
					if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
						start := i - 2
						if start < 0 {
							start = 0
						}
						end := i + 3
						if end > len(lines) {
							end = len(lines)
						}
						context := strings.Join(lines[start:end], "\n")

						results = append(results, KnowledgeResult{
							Source:    file,
							Type:      "code",
							Content:   line,
							Relevance: 0.7,
							Context:   context,
						})
						break
					}
				}
			}
		}
	}

	return results, nil
}

func (s *KnowledgeRetrieverSkill) searchDocumentation(query, projectID string, limit int) ([]KnowledgeResult, error) {
	var results []KnowledgeResult

	// Search in MD files
	workspaceDir := os.Getenv("WORKSPACE_DIR")
	if workspaceDir == "" {
		workspaceDir = "."
	}

	mdFiles, _ := filepath.Glob(filepath.Join(workspaceDir, "**/*.md"))
	for _, file := range mdFiles {
		if len(results) >= limit {
			break
		}
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		contentStr := string(content)
		if strings.Contains(strings.ToLower(contentStr), strings.ToLower(query)) {
			// Find the matching section
			lines := strings.Split(contentStr, "\n")
			for i, line := range lines {
				if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
					// Get section context (up to next heading or 10 lines)
					end := i + 10
					if end > len(lines) {
						end = len(lines)
					}
					context := strings.Join(lines[i:end], "\n")

					results = append(results, KnowledgeResult{
						Source:    file,
						Type:      "doc",
						Content:   line,
						Relevance: 0.6,
						Context:   context,
					})
					break
				}
			}
		}
	}

	return results, nil
}

func (s *KnowledgeRetrieverSkill) searchExamples(query, projectID, language string, limit int) ([]KnowledgeResult, error) {
	var results []KnowledgeResult

	// Search for example files or test files
	workspaceDir := os.Getenv("WORKSPACE_DIR")
	if workspaceDir == "" {
		workspaceDir = "."
	}

	// Look for example patterns
	patterns := []string{
		"**/example*.go",
		"**/*_test.go",
		"**/examples/**/*.go",
		"**/test/**/*.go",
	}

	for _, pattern := range patterns {
		files, _ := filepath.Glob(filepath.Join(workspaceDir, pattern))
		for _, file := range files {
			if len(results) >= limit {
				break
			}
			content, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			contentStr := string(content)
			if strings.Contains(strings.ToLower(contentStr), strings.ToLower(query)) {
				results = append(results, KnowledgeResult{
					Source:    file,
					Type:      "example",
					Content:   contentStr[:minInt(500, len(contentStr))], // First 500 chars
					Relevance: 0.5,
					Context:   "Example/test file",
				})
			}
		}
	}

	return results, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *KnowledgeRetrieverSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  true,
		Essential: false,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
