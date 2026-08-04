package skills

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

type ReadFileSkill struct {
	db *sql.DB
}

func NewReadFileSkill(db *sql.DB) *ReadFileSkill {
	return &ReadFileSkill{db: db}
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

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if projectID == "" {
		return "", fmt.Errorf("project_id is required")
	}
	if s.db == nil {
		return "数据库不可用，无法读取文件", nil
	}

	var content string
	err := s.db.QueryRowContext(ctx,
		`SELECT content FROM project_files WHERE project_id=? AND path=?`, projectID, path,
	).Scan(&content)
	if err == sql.ErrNoRows {
		return fmt.Sprintf("文件未找到: %s", path), nil
	}
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %w", err)
	}

	// 支持行范围读取
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	startLine := 1
	endLine := totalLines
	hasExplicitRange := false

	if v, ok := input["start_line"].(float64); ok && int(v) > 0 {
		startLine = int(v)
		hasExplicitRange = true
	}
	if v, ok := input["end_line"].(float64); ok && int(v) > 0 {
		endLine = int(v)
		hasExplicitRange = true
	}

	// 边界修正
	if startLine < 1 {
		startLine = 1
	}
	if endLine > totalLines {
		endLine = totalLines
	}
	if startLine > endLine {
		return fmt.Sprintf("File: %s\n\n(start_line %d > end_line %d, total %d lines)", path, startLine, endLine, totalLines), nil
	}

	// For large files without explicit range, return intelligent summary
	if totalLines > 500 && !hasExplicitRange {
		return s.readLargeFileSmart(path, lines, totalLines), nil
	}

	// 提取指定范围的行
	selected := lines[startLine-1 : endLine]
	var sb strings.Builder
	for i, line := range selected {
		sb.WriteString(fmt.Sprintf("%4d | %s\n", startLine+i, line))
	}

	result := sb.String()
	truncated := false
	if len(result) > 8000 {
		result = result[:8000] + "\n... [truncated — use smaller line range]"
		truncated = true
	}

	header := fmt.Sprintf("File: %s (lines %d-%d of %d)", path, startLine, endLine, totalLines)
	if truncated {
		header += " [content truncated]"
	}

	return fmt.Sprintf("%s\n\n%s", header, result), nil
}

// readLargeFileSmart intelligently extracts key sections from large files.
func (s *ReadFileSkill) readLargeFileSmart(path string, lines []string, totalLines int) string {
	var sb strings.Builder
	lang := detectLanguage(path)

	sb.WriteString(fmt.Sprintf("File: %s (%d lines) — intelligent summary\n\n", path, totalLines))

	// Always include first 20 lines (header, imports, package declaration)
	headerEnd := 20
	if headerEnd > totalLines {
		headerEnd = totalLines
	}
	sb.WriteString(fmt.Sprintf("=== HEADER (lines 1-%d) ===\n", headerEnd))
	for i := 0; i < headerEnd; i++ {
		sb.WriteString(fmt.Sprintf("%4d | %s\n", i+1, lines[i]))
	}
	sb.WriteString("\n")

	// Find and include key definitions (functions, structs, main logic)
	type section struct {
		start int
		end   int
		label string
	}
	var sections []section

	defPatterns := getDefinitionPatterns(lang)
	inBlock := false
	blockStart := 0
	braceDepth := 0

	for i, line := range lines {
		if i < headerEnd {
			continue
		}
		trimmed := strings.TrimSpace(line)

		// Detect function/struct/class definitions
		if !inBlock {
			for _, pat := range defPatterns {
				if pat.MatchString(trimmed) {
					blockStart = i
					inBlock = true
					braceDepth = 0
					break
				}
			}
		}

		if inBlock {
			braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
			if braceDepth <= 0 && (strings.Contains(line, "}") || i > blockStart) {
				end := i + 1
				// Cap section length at 50 lines
				if end-blockStart > 50 {
					end = blockStart + 50
				}
				// Only add sections > 3 lines
				if end-blockStart > 3 {
					label := strings.TrimSpace(lines[blockStart])
					if len(label) > 60 {
						label = label[:60] + "..."
					}
					sections = append(sections, section{start: blockStart + 1, end: end, label: label})
				}
				inBlock = false
				// Max 8 sections
				if len(sections) >= 8 {
					break
				}
			}
		}
	}

	if len(sections) > 0 {
		sb.WriteString(fmt.Sprintf("=== KEY DEFINITIONS (%d found) ===\n", len(sections)))
		for _, sec := range sections {
			sb.WriteString(fmt.Sprintf("--- %s (lines %d-%d) ---\n", sec.label, sec.start, sec.end))
			for i := sec.start - 1; i < sec.end && i < totalLines; i++ {
				sb.WriteString(fmt.Sprintf("%4d | %s\n", i+1, lines[i]))
			}
			sb.WriteString("\n")
		}
	}

	// Include last 10 lines (usually closing braces, may contain important constants)
	if totalLines > 30 {
		footerStart := totalLines - 10
		if footerStart < headerEnd {
			footerStart = headerEnd
		}
		sb.WriteString(fmt.Sprintf("=== FOOTER (lines %d-%d) ===\n", footerStart+1, totalLines))
		for i := footerStart; i < totalLines; i++ {
			sb.WriteString(fmt.Sprintf("%4d | %s\n", i+1, lines[i]))
		}
	}

	result := sb.String()
	if len(result) > 8000 {
		result = result[:8000] + "\n... [summary truncated — use start_line/end_line for specific sections]"
	}

	return result
}

// definitionPatternsCache caches compiled regex patterns per language (P1 optimization).
var definitionPatternsCache = map[string][]*regexp.Regexp{
	"go": {
		regexp.MustCompile(`^func\s`),
		regexp.MustCompile(`^type\s+\w+\s+struct`),
		regexp.MustCompile(`^type\s+\w+\s+interface`),
		regexp.MustCompile(`^func\s*\(\s*\w+\s+\*?\w+\)\s+\w+`),
	},
	"rust": {
		regexp.MustCompile(`^pub\s+fn\s`),
		regexp.MustCompile(`^fn\s`),
		regexp.MustCompile(`^pub\s+struct\s`),
		regexp.MustCompile(`^pub\s+impl\s`),
		regexp.MustCompile(`^impl\s`),
		regexp.MustCompile(`^pub\s+enum\s`),
	},
	"shell": {
		regexp.MustCompile(`^\w+\s*\(\s*\)\s*\{`),
		regexp.MustCompile(`^function\s+\w+`),
	},
	"python": {
		regexp.MustCompile(`^def\s+\w+`),
		regexp.MustCompile(`^class\s+\w+`),
	},
	"cpp": {
		regexp.MustCompile(`^(?:static\s+)?(?:void|int|char|bool|float|double|long|unsigned|auto)\s+\w+\s*\(`),
		regexp.MustCompile(`^(?:pub\s+)?(?:struct|class|enum)\s+\w+`),
	},
	"c": {
		regexp.MustCompile(`^(?:static\s+)?(?:void|int|char|bool|float|double|long|unsigned|auto)\s+\w+\s*\(`),
		regexp.MustCompile(`^(?:pub\s+)?(?:struct|class|enum)\s+\w+`),
	},
	"javascript": {
		regexp.MustCompile(`^function\s+\w+`),
		regexp.MustCompile(`^(?:export\s+)?class\s+\w+`),
		regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+\w+\s*=\s*(?:async\s+)?\(`),
	},
	"typescript": {
		regexp.MustCompile(`^function\s+\w+`),
		regexp.MustCompile(`^(?:export\s+)?class\s+\w+`),
		regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+\w+\s*=\s*(?:async\s+)?\(`),
	},
}

// getDefinitionPatterns returns cached regex patterns for detecting key definitions per language.
func getDefinitionPatterns(lang string) []*regexp.Regexp {
	return definitionPatternsCache[lang]
}

func (s *ReadFileSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  true,
		Essential: true,
		Core:      true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
