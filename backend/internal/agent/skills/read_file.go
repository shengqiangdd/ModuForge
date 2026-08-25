package skills

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/moduforge/backend/internal/agent/registry"
	"github.com/moduforge/backend/internal/storage"
)

// FileHashCacheI is the interface for file hash caching (matches registry.FileHashCacheI).
type FileHashCacheI interface {
	Get(path string) string
	Set(path, hash string)
	Invalidate(path string)
}

type ReadFileSkill struct {
	db          *sql.DB
	projectPath string
	fileHash    FileHashCacheI
	storage     storage.StorageAdapter // optional S3 storage backend
}

func NewReadFileSkill(db *sql.DB) *ReadFileSkill {
	return &ReadFileSkill{db: db}
}

func NewReadFileSkillWithDB(projectPath string, db *sql.DB) *ReadFileSkill {
	return &ReadFileSkill{db: db, projectPath: projectPath}
}

// WithStorage sets the S3 storage adapter. When set, files are read from S3
// instead of disk/DB.
func (s *ReadFileSkill) WithStorage(st storage.StorageAdapter) *ReadFileSkill {
	s.storage = st
	return s
}

// SetFileHashCache sets the file hash cache for UNCHANGED detection.
func (s *ReadFileSkill) SetFileHashCache(cache FileHashCacheI) {
	s.fileHash = cache
}

func (s *ReadFileSkill) Name() string {
	return "read_file"
}

func (s *ReadFileSkill) Description() string {
	return "Read a project file. Input: {\"path\": \"...\", \"project_id\": \"...\", \"start_line\" (optional): 1-based start, \"end_line\" (optional): 1-based end}. For large files (>500 lines), automatically returns key sections (imports, functions, main logic) if no range specified."
}

func (s *ReadFileSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, _ := input["path"].(string)
	projectID, _ := input["project_id"].(string)
	startLine, _ := input["start_line"].(float64)
	endLine, _ := input["end_line"].(float64)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// S3 storage path: read from S3 directly, fall back to DB on failure
	if s.storage != nil {
		content, err := s.storage.Read(ctx, s.storagePath(projectID, path))
		if err != nil {
			// S3 read failed — fall back to DB content column (same as project service)
			slog.Warn("s3 read failed in read_file skill, falling back to db", "path", path, "error", err)
		} else if len(content) > 0 {
			// Differential-cache also applies to S3 reads (production path).
			if s.fileHash != nil && startLine == 0 && endLine == 0 {
				h := sha256.Sum256(content)
				hash := fmt.Sprintf("%x", h)
				prev := s.fileHash.Get(path)
				totalLines := len(strings.Split(string(content), "\n"))
				if prev != "" && prev == hash && totalLines > 500 {
					return fmt.Sprintf(
						"File: %s (%d lines) — UNCHANGED since last read (sha256:%s).\n"+
							"Content is identical to what you already have in context. "+
							"No need to re-analyze. Use start_line/end_line to read any specific section if required.",
						path, totalLines, hash[:12],
					), nil
				}
				s.fileHash.Set(path, hash)
			}
			return s.formatContent(path, string(content), startLine, endLine), nil
		}
		// content is empty or S3 failed — fall through to legacy disk/DB path below
	}
		// Differential-cache also applies to S3 reads (production path).
		if s.fileHash != nil && startLine == 0 && endLine == 0 {
			h := sha256.Sum256(content)
			hash := fmt.Sprintf("%x", h)
			prev := s.fileHash.Get(path)
			totalLines := len(strings.Split(string(content), "\n"))
			if prev != "" && prev == hash && totalLines > 500 {
				return fmt.Sprintf(
					"File: %s (%d lines) — UNCHANGED since last read (sha256:%s).\n"+
						"Content is identical to what you already have in context. "+
						"No need to re-analyze. Use start_line/end_line to read any specific section if required.",
					path, totalLines, hash[:12],
				), nil
			}
			s.fileHash.Set(path, hash)
		}
		return s.formatContent(path, string(content), startLine, endLine), nil
	}

	// Legacy path: try disk first, then DB
	basePath := ResolveProjectPath(s.db, s.projectPath, projectID)
	fullPath := filepath.Join(basePath, path)
	fromDB := false

	// Try to read from disk
	content, err := os.ReadFile(fullPath)
	if err != nil {
		// Fallback: read from database
		if s.db != nil && projectID != "" {
			var dbContent string
			err := s.db.QueryRow(
				`SELECT content FROM project_files WHERE project_id=? AND path=?`, projectID, path,
			).Scan(&dbContent)
			if err != nil {
				return "", fmt.Errorf("file not found on disk or in database: %s", path)
			}
			content = []byte(dbContent)
			fromDB = true
		} else {
			return "", fmt.Errorf("file not found: %s (disk: %v)", path, err)
		}
	}

	// Differential-cache: if the file hash is unchanged from the last read AND
	// this is a full-file read of a large file (>500 lines), we can avoid
	// re-emitting the whole (potentially huge) smart-summary and just confirm
	// "unchanged", telling the model to use start_line/end_line for detail.
	// Small files are cheap to re-read, so we always return them in full to
	// avoid any risk of stale-context confusion.
	if s.fileHash != nil && !fromDB && startLine == 0 && endLine == 0 {
		h := sha256.Sum256(content)
		hash := fmt.Sprintf("%x", h)
		prev := s.fileHash.Get(path)
		totalLines := len(strings.Split(string(content), "\n"))
		if prev != "" && prev == hash && totalLines > 500 {
			return fmt.Sprintf(
				"File: %s (%d lines) — UNCHANGED since last read (sha256:%s).\n"+
					"Content is identical to what you already have in context. "+
					"No need to re-analyze. Use start_line/end_line to read any specific section if required.",
				path, totalLines, hash[:12],
			), nil
		}
		s.fileHash.Set(path, hash)
	}

	return s.formatContent(path, string(content), startLine, endLine), nil
}

func (s *ReadFileSkill) formatContent(path, content string, startLine, endLine float64) string {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// If no range specified, return full content
	if startLine == 0 && endLine == 0 {
		// For large files with no range, use smart reading
		if totalLines > 500 {
			return s.readLargeFileSmart(path, lines, totalLines, false)
		}
		return content
	}

	// Validate range
	start := int(startLine)
	end := int(endLine)
	if start < 1 {
		start = 1
	}
	if end > totalLines || end == 0 {
		end = totalLines
	}
	if start > end {
		start = end
	}
	if start > totalLines {
		start = totalLines
	}

	// Include line numbers
	var result strings.Builder
	result.WriteString(fmt.Sprintf("File: %s (%d/%d lines)\n", path, end-start+1, totalLines))
	for i := start - 1; i < end; i++ {
		result.WriteString(fmt.Sprintf("%d:> %s\n", i+1, lines[i]))
	}
	return result.String()
}

func (s *ReadFileSkill) readLargeFileSmart(path string, lines []string, totalLines int, fromDB bool) string {
	var result strings.Builder
	result.WriteString(fmt.Sprintf("File: %s (%d lines total)\n", path, totalLines))
	result.WriteString("--- First 10 lines ---\n")
	for i := 0; i < 10 && i < totalLines; i++ {
		result.WriteString(fmt.Sprintf("%d:> %s\n", i+1, lines[i]))
	}

	// Find and include key definitions (functions, structs, main logic)
	result.WriteString("--- Key definitions ---\n")
	found := 0
	seen := make(map[string]bool)

	// Detect language from file extension
	lang := detectLanguage(path)
	patterns := getDefinitionPatterns(lang)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Check against patterns
		for _, pat := range patterns {
			if pat.MatchString(trimmed) {
				key := strings.TrimSpace(trimmed)
				if !seen[key] {
					seen[key] = true
					result.WriteString(fmt.Sprintf("  %d:> %s\n", i+1, trimmed))
					found++
					if found >= 30 {
						goto done
					}
				}
				break
			}
		}
	}
done:

	// If no key definitions found, use heuristics
	if found == 0 {
		result.WriteString("  (no definitions found — showing middle section)\n")
		mid := totalLines / 2
		for i := mid - 5; i < mid+5 && i < totalLines; i++ {
			if i >= 0 {
				result.WriteString(fmt.Sprintf("%d:> %s\n", i+1, lines[i]))
			}
		}
	}

	result.WriteString("--- Last 10 lines ---\n")
	for i := totalLines - 10; i < totalLines; i++ {
		if i >= 0 {
			result.WriteString(fmt.Sprintf("%d:> %s\n", i+1, lines[i]))
		}
	}

	result.WriteString("(Use start_line/end_line to read specific ranges)\n")
	return result.String()
}

// detectLanguage returns the language name for a file path.
// (defined in pathutil.go)

// getDefinitionPatterns returns regex patterns for finding definitions in a language.
func getDefinitionPatterns(lang string) []*regexp.Regexp {
	switch lang {
	case "go":
		return []*regexp.Regexp{
			regexp.MustCompile(`^func\s`),
			regexp.MustCompile(`^type\s+\w+\s`),
			regexp.MustCompile(`^func\s*\(\s*\w+\s+\*?\w+\)\s+\w+`),
			regexp.MustCompile(`^type\s+\w+\s+struct`),
			regexp.MustCompile(`^type\s+\w+\s+interface`),
			regexp.MustCompile(`^const\s+`),
			regexp.MustCompile(`^var\s+`),
		}
	case "rust":
		return []*regexp.Regexp{
			regexp.MustCompile(`^fn\s+\w+`),
			regexp.MustCompile(`^pub\s+fn\s+\w+`),
			regexp.MustCompile(`^struct\s+\w+`),
			regexp.MustCompile(`^enum\s+\w+`),
			regexp.MustCompile(`^trait\s+\w+`),
			regexp.MustCompile(`^impl\s+\w+`),
			regexp.MustCompile(`^pub\s+struct\s+\w+`),
			regexp.MustCompile(`^pub\s+enum\s+\w+`),
			regexp.MustCompile(`^pub\s+trait\s+\w+`),
			regexp.MustCompile(`^pub\s+impl\s+\w+`),
		}
	case "python":
		return []*regexp.Regexp{
			regexp.MustCompile(`^def\s+\w+`),
			regexp.MustCompile(`^class\s+\w+`),
			regexp.MustCompile(`^async\s+def\s+\w+`),
		}
	case "javascript":
		return []*regexp.Regexp{
			regexp.MustCompile(`^function\s+\w+`),
			regexp.MustCompile(`^const\s+\w+\s*=\s*`),
			regexp.MustCompile(`^class\s+\w+`),
			regexp.MustCompile(`^export\s+`),
			regexp.MustCompile(`^export\s+default\s+`),
		}
	case "java":
		return []*regexp.Regexp{
			regexp.MustCompile(`^public\s+(class|interface|enum)\s+\w+`),
			regexp.MustCompile(`^private\s+\w+\s+\w+\s*\(`),
			regexp.MustCompile(`^protected\s+\w+\s+\w+\s*\(`),
		}
	case "cpp", "c":
		return []*regexp.Regexp{
			regexp.MustCompile(`^\w+\s+\w+\s*\(`),
			regexp.MustCompile(`^class\s+\w+`),
			regexp.MustCompile(`^struct\s+\w+`),
			regexp.MustCompile(`^void\s+\w+\s*\(`),
			regexp.MustCompile(`^int\s+\w+\s*\(`),
		}
	default:
		return nil
	}
}

// storagePath constructs the S3 path for a project file.
// NOTE: the S3Adapter prepends its configured prefix ("projects"), so we pass
// the project-relative key here — DO NOT prefix with "projects/" again.
func (s *ReadFileSkill) storagePath(projectID, path string) string {
	return S3ObjectKey(projectID, path)
}

func (s *ReadFileSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  true,
		Essential: true,
		Core:      true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
