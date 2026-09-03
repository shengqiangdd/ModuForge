package skills

import (
	"context"
	"fmt"
	"github.com/moduforge/backend/internal/agent/registry"
	"math"
	"strings"
)

// CodeQualitySkill measures code quality metrics: cyclomatic complexity,
// function length, file length, duplication detection, naming conventions.
type CodeQualitySkill struct{}

func NewCodeQualitySkill() *CodeQualitySkill {
	return &CodeQualitySkill{}
}

func (s *CodeQualitySkill) Name() string { return "code_quality" }

func (s *CodeQualitySkill) Description() string {
	return "Analyze code quality metrics. Input: {\"path\": \"...\", \"content\": \"...\", \"language\": \"go|rust|c++|shell|python|typescript\"}. Returns: cyclomatic complexity, function lengths, file length, duplication patterns, naming issues, and overall quality score with specific fix suggestions."
}

type QualityMetrics struct {
	FilePath          string            `json:"file_path"`
	TotalLines        int               `json:"total_lines"`
	BlankLines        int               `json:"blank_lines"`
	CommentLines      int               `json:"comment_lines"`
	CodeLines         int               `json:"code_lines"`
	AvgFunctionLength float64           `json:"avg_function_length"`
	MaxFunctionLength int               `json:"max_function_length"`
	MaxCyclomatic     int               `json:"max_cyclomatic_complexity"`
	AvgCyclomatic     float64           `json:"avg_cyclomatic_complexity"`
	DuplicationBlocks int               `json:"duplication_blocks"`
	Functions         []FunctionMetrics `json:"functions"`
	NamingIssues      []string          `json:"naming_issues"`
	Score             int               `json:"score"`
	Grade             string            `json:"grade"`
	Issues            []string          `json:"issues"`
	Suggestions       []string          `json:"suggestions"`
}

type FunctionMetrics struct {
	Name       string `json:"name"`
	Line       int    `json:"line"`
	Length     int    `json:"length"`
	Complexity int    `json:"complexity"`
}

func (s *CodeQualitySkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	content, _ := input["content"].(string)
	language, _ := input["language"].(string)
	path, _ := input["path"].(string)

	if content == "" {
		return "", fmt.Errorf("content is required")
	}
	if language == "" {
		language = detectLanguage(path)
	}

	lines := strings.Split(content, "\n")
	metrics := analyzeCodeQuality(path, lines, language)
	return formatQualityReport(metrics), nil
}

func (s *CodeQualitySkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  true,
		Essential: false,
		Core:      false,
		NeedsDB:   false,
		NeedsLLM:  false,
	}
}

func analyzeCodeQuality(path string, lines []string, language string) QualityMetrics {
	m := QualityMetrics{
		FilePath:    path,
		TotalLines:  len(lines),
		Functions:   make([]FunctionMetrics, 0),
		Issues:      make([]string, 0),
		Suggestions: make([]string, 0),
	}

	// Phase 1: Count lines and detect functions
	inBlockComment := false
	inFunction := false
	funcName := ""
	funcStart := 0
	funcLines := 0
	braceDepth := 0

	// Duplication detection: hash of 3-line sliding window
	lineHashes := make(map[string]int)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Block comment tracking
		if strings.Contains(trimmed, "/*") {
			inBlockComment = true
		}
		if strings.Contains(trimmed, "*/") {
			inBlockComment = false
			m.CommentLines++
			continue
		}

		// Blank line
		if trimmed == "" {
			m.BlankLines++
			continue
		}

		// Comment line
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || inBlockComment {
			m.CommentLines++
			continue
		}

		m.CodeLines++

		// 3-line sliding window for duplication
		if i+2 < len(lines) {
			key := trimmed + "|" + strings.TrimSpace(lines[i+1]) + "|" + strings.TrimSpace(lines[i+2])
			if len(key) > 30 {
				lineHashes[key]++
			}
		}

		// Brace tracking
		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")

		// Function detection
		if isFunctionStart(trimmed, language) {
			if inFunction && funcLines > 0 {
				complexity := estimateComplexity(lines[funcStart:i], language)
				m.Functions = append(m.Functions, FunctionMetrics{
					Name:       funcName,
					Line:       funcStart + 1,
					Length:     funcLines,
					Complexity: complexity,
				})
			}
			inFunction = true
			funcName = extractFunctionName(trimmed, language)
			funcStart = i
			funcLines = 0
		}

		if inFunction {
			funcLines++
		}
	}

	// Close last function
	if inFunction && funcLines > 0 {
		complexity := estimateComplexity(lines[funcStart:], language)
		m.Functions = append(m.Functions, FunctionMetrics{
			Name:       funcName,
			Line:       funcStart + 1,
			Length:     funcLines,
			Complexity: complexity,
		})
	}

	// Count duplication blocks (3+ repeated lines)
	for _, count := range lineHashes {
		if count >= 2 {
			m.DuplicationBlocks += count - 1
		}
	}

	// Phase 2: Calculate aggregate metrics
	if len(m.Functions) > 0 {
		totalLen := 0
		totalComplexity := 0
		m.MaxFunctionLength = 0
		m.MaxCyclomatic = 0
		for _, f := range m.Functions {
			totalLen += f.Length
			totalComplexity += f.Complexity
			if f.Length > m.MaxFunctionLength {
				m.MaxFunctionLength = f.Length
			}
			if f.Complexity > m.MaxCyclomatic {
				m.MaxCyclomatic = f.Complexity
			}
		}
		m.AvgFunctionLength = float64(totalLen) / float64(len(m.Functions))
		m.AvgCyclomatic = float64(totalComplexity) / float64(len(m.Functions))
	}

	// Phase 3: Detect naming issues
	m.NamingIssues = detectNamingIssues(lines, language)

	// Phase 4: Identify issues and suggestions
	m.Score = 100
	identifyIssues(&m)
	identifySuggestions(&m)

	// Phase 5: Assign grade
	m.Grade = scoreToGrade(m.Score)

	return m
}

func isFunctionStart(line, language string) bool {
	switch language {
	case "go":
		return strings.HasPrefix(line, "func ")
	case "rust":
		return strings.HasPrefix(line, "fn ") || strings.HasPrefix(line, "pub fn ") ||
			strings.HasPrefix(line, "pub(crate) fn ") || strings.HasPrefix(line, "async fn ")
	case "python":
		return strings.HasPrefix(line, "def ")
	case "shell":
		return strings.HasPrefix(line, "function ") || (strings.HasSuffix(line, "() {") && !strings.HasPrefix(line, "#"))
	case "c++", "c":
		return strings.HasPrefix(line, "void ") || strings.HasPrefix(line, "int ") ||
			strings.HasPrefix(line, "bool ") || strings.HasPrefix(line, "char ") ||
			strings.HasPrefix(line, "static ") || strings.HasPrefix(line, "unsigned ")
	case "typescript", "javascript":
		return strings.HasPrefix(line, "function ") || strings.HasPrefix(line, "async function ") ||
			(strings.Contains(line, "=>") && strings.Contains(line, "const "))
	}
	return false
}

func extractFunctionName(line, language string) string {
	switch language {
	case "go":
		// func FuncName(...) or func (r *Type) FuncName(...)
		if idx := strings.Index(line, "func "); idx >= 0 {
			rest := line[idx+5:]
			// Skip receiver
			if strings.HasPrefix(rest, "(") {
				if closeIdx := strings.Index(rest, ")"); closeIdx >= 0 {
					rest = rest[closeIdx+1:]
				}
			}
			rest = strings.TrimSpace(rest)
			if spaceIdx := strings.IndexAny(rest, "( "); spaceIdx >= 0 {
				return rest[:spaceIdx]
			}
			return rest
		}
	case "rust":
		// fn name( or pub fn name(
		rest := line
		for _, prefix := range []string{"pub(", "pub ", "pub(crate) ", "async "} {
			rest = strings.TrimPrefix(rest, prefix)
		}
		rest = strings.TrimPrefix(rest, "fn ")
		rest = strings.TrimSpace(rest)
		if idx := strings.IndexAny(rest, "(< "); idx >= 0 {
			return rest[:idx]
		}
	case "python":
		// def name(
		rest := strings.TrimPrefix(line, "def ")
		rest = strings.TrimSpace(rest)
		if idx := strings.IndexAny(rest, "( "); idx >= 0 {
			return rest[:idx]
		}
	case "shell":
		// function name() {
		rest := strings.TrimPrefix(line, "function ")
		rest = strings.TrimSpace(rest)
		if idx := strings.IndexAny(rest, "( "); idx >= 0 {
			return rest[:idx]
		}
	}
	return "anonymous"
}

func estimateComplexity(lines []string, language string) int {
	complexity := 1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Decision points
		for _, kw := range []string{
			"if ", "else if ", "else {", "for ", "while ", "switch ", "case ",
			"match ", "&&", "||", "?", "catch ", "defer ",
		} {
			if strings.Contains(trimmed, kw) {
				complexity++
			}
		}
	}
	return complexity
}

func detectNamingIssues(lines []string, language string) []string {
	var issues []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || trimmed == "" {
			continue
		}
		// Go: exported names should be CamelCase
		if language == "go" && strings.HasPrefix(line, "func ") {
			name := extractFunctionName(line, "go")
			if len(name) > 0 && name[0] >= 'a' && name[0] <= 'z' && !strings.Contains(name, "_") {
				// lowercase first letter is unexported — ok
			} else if strings.Contains(name, "_") && len(name) > 2 {
				issues = append(issues, fmt.Sprintf("Line %d: function '%s' uses snake_case (prefer CamelCase in %s)", i+1, name, language))
			}
		}
		// Rust: snake_case for functions
		if (language == "rust") && (strings.Contains(line, "fn ") || strings.Contains(line, "pub fn ")) {
			name := extractFunctionName(line, "rust")
			if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
				issues = append(issues, fmt.Sprintf("Line %d: function '%s' uses CamelCase (prefer snake_case in Rust)", i+1, name))
			}
		}
	}
	return issues
}

func identifyIssues(m *QualityMetrics) {
	// Long functions
	if m.MaxFunctionLength > 80 {
		m.Issues = append(m.Issues, fmt.Sprintf("Very long function detected: %d lines (recommended: <50)", m.MaxFunctionLength))
		m.Score -= 15
	} else if m.MaxFunctionLength > 50 {
		m.Issues = append(m.Issues, fmt.Sprintf("Long function detected: %d lines (recommended: <50)", m.MaxFunctionLength))
		m.Score -= 8
	}

	// High cyclomatic complexity
	if m.MaxCyclomatic > 20 {
		m.Issues = append(m.Issues, fmt.Sprintf("Very high cyclomatic complexity: %d (recommended: <10)", m.MaxCyclomatic))
		m.Score -= 20
	} else if m.MaxCyclomatic > 10 {
		m.Issues = append(m.Issues, fmt.Sprintf("High cyclomatic complexity: %d (recommended: <10)", m.MaxCyclomatic))
		m.Score -= 10
	}

	// Too many functions
	if len(m.Functions) > 50 {
		m.Issues = append(m.Issues, fmt.Sprintf("File has %d functions — consider splitting", len(m.Functions)))
		m.Score -= 10
	}

	// Duplication
	if m.DuplicationBlocks > 5 {
		m.Issues = append(m.Issues, fmt.Sprintf("Significant code duplication: ~%d repeated blocks", m.DuplicationBlocks))
		m.Score -= 15
	} else if m.DuplicationBlocks > 2 {
		m.Issues = append(m.Issues, fmt.Sprintf("Minor code duplication: ~%d repeated blocks", m.DuplicationBlocks))
		m.Score -= 5
	}

	// File too long
	if m.TotalLines > 500 {
		m.Issues = append(m.Issues, fmt.Sprintf("File is very long: %d lines (recommended: <300)", m.TotalLines))
		m.Score -= 10
	} else if m.TotalLines > 300 {
		m.Issues = append(m.Issues, fmt.Sprintf("File is long: %d lines (recommended: <300)", m.TotalLines))
		m.Score -= 5
	}

	// Low comment ratio
	if m.CodeLines > 20 && m.CommentLines == 0 {
		m.Issues = append(m.Issues, "No comments in code — consider adding documentation")
		m.Score -= 5
	} else if m.CodeLines > 50 && float64(m.CommentLines)/float64(m.CodeLines) < 0.05 {
		m.Issues = append(m.Issues, "Very low comment ratio (<5%)")
		m.Score -= 3
	}

	// Naming issues
	if len(m.NamingIssues) > 0 {
		m.Issues = append(m.Issues, fmt.Sprintf("%d naming convention issues", len(m.NamingIssues)))
		m.Score -= len(m.NamingIssues) * 2
	}

	if m.Score < 0 {
		m.Score = 0
	}
}

func identifySuggestions(m *QualityMetrics) {
	if m.MaxFunctionLength > 50 {
		m.Suggestions = append(m.Suggestions, "Split long functions into smaller, focused functions (single responsibility)")
	}
	if m.MaxCyclomatic > 10 {
		m.Suggestions = append(m.Suggestions, "Reduce complexity by extracting conditions into well-named helper functions")
	}
	if m.DuplicationBlocks > 2 {
		m.Suggestions = append(m.Suggestions, "Extract duplicated code into shared utility functions")
	}
	if m.TotalLines > 300 {
		m.Suggestions = append(m.Suggestions, "Consider splitting this file into smaller, focused modules")
	}
	if len(m.NamingIssues) > 0 {
		m.Suggestions = append(m.Suggestions, "Fix naming conventions for consistency with language idioms")
	}
}

func scoreToGrade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

func formatQualityReport(m QualityMetrics) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Code Quality Report: %s\n", m.FilePath))
	sb.WriteString(fmt.Sprintf("Grade: %s (%d/100)\n\n", m.Grade, m.Score))

	sb.WriteString("📊 Metrics:\n")
	sb.WriteString(fmt.Sprintf("  Lines: %d total (%d code, %d comments, %d blank)\n",
		m.TotalLines, m.CodeLines, m.CommentLines, m.BlankLines))
	sb.WriteString(fmt.Sprintf("  Functions: %d\n", len(m.Functions)))
	if len(m.Functions) > 0 {
		sb.WriteString(fmt.Sprintf("  Avg function length: %.0f lines\n", m.AvgFunctionLength))
		sb.WriteString(fmt.Sprintf("  Max function length: %d lines\n", m.MaxFunctionLength))
		sb.WriteString(fmt.Sprintf("  Max cyclomatic complexity: %d\n", m.MaxCyclomatic))
	}
	if m.DuplicationBlocks > 0 {
		sb.WriteString(fmt.Sprintf("  Duplicated blocks: ~%d\n", m.DuplicationBlocks))
	}

	if len(m.Issues) > 0 {
		sb.WriteString("\n⚠️ Issues:\n")
		for _, issue := range m.Issues {
			sb.WriteString(fmt.Sprintf("  - %s\n", issue))
		}
	}

	if len(m.NamingIssues) > 0 {
		sb.WriteString("\n📝 Naming Issues:\n")
		for _, ni := range m.NamingIssues {
			sb.WriteString(fmt.Sprintf("  - %s\n", ni))
		}
	}

	if len(m.Suggestions) > 0 {
		sb.WriteString("\n💡 Suggestions:\n")
		for _, s := range m.Suggestions {
			sb.WriteString(fmt.Sprintf("  - %s\n", s))
		}
	}

	// Top 5 most complex functions
	if len(m.Functions) > 0 {
		sorted := make([]FunctionMetrics, len(m.Functions))
		copy(sorted, m.Functions)
		// Simple selection sort for top 5
		for i := 0; i < 5 && i < len(sorted); i++ {
			maxIdx := i
			for j := i + 1; j < len(sorted); j++ {
				if sorted[j].Complexity > sorted[maxIdx].Complexity {
					maxIdx = j
				}
			}
			sorted[i], sorted[maxIdx] = sorted[maxIdx], sorted[i]
		}
		limit := 5
		if len(sorted) < limit {
			limit = len(sorted)
		}
		sb.WriteString("\n🔍 Most Complex Functions:\n")
		for i := 0; i < limit; i++ {
			f := sorted[i]
			indicator := "🟢"
			if f.Complexity > 10 {
				indicator = "🔴"
			} else if f.Complexity > 5 {
				indicator = "🟡"
			}
			sb.WriteString(fmt.Sprintf("  %s %s (line %d): %d lines, complexity=%d\n", indicator, f.Name, f.Line, f.Length, f.Complexity))
		}
	}

	return sb.String()
}

// Unused but kept for potential future use
var _ = math.MaxInt
