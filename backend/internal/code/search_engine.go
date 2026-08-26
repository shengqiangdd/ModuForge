package code

import (
	"regexp"
	"strings"
)

// SearchEngine 代码搜索引擎
type SearchEngine struct{}

// NewSearchEngine 创建搜索引擎
func NewSearchEngine() *SearchEngine {
	return &SearchEngine{}
}

// SearchResult 搜索结果
type SearchResult struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
	Match   string `json:"match"`
	Type    string `json:"type"` // function, class, variable, import, comment
}

// SearchQuery 搜索查询
type SearchQuery struct {
	Pattern      string `json:"pattern"`
	Language     string `json:"language"`
	Type         string `json:"type"` // all, function, class, variable, import, regex
	CaseSensitive bool  `json:"case_sensitive"`
}

// Search 搜索代码
func (s *SearchEngine) Search(files map[string]string, query SearchQuery) []SearchResult {
	results := make([]SearchResult, 0)

	for fileName, code := range files {
		lines := strings.Split(code, "\n")

		for i, line := range lines {
			matched := false

			if query.Type == "regex" {
				// 正则表达式匹配
				flags := ""
				if !query.CaseSensitive {
					flags = "(?i)"
				}
				if re, err := regexp.Compile(flags + query.Pattern); err == nil {
					matched = re.MatchString(line)
				}
			} else {
				// 简单文本匹配
				searchLine := line
				searchPattern := query.Pattern
				if !query.CaseSensitive {
					searchLine = strings.ToLower(searchLine)
					searchPattern = strings.ToLower(searchPattern)
				}
				matched = strings.Contains(searchLine, searchPattern)
			}

			if matched {
				resultType := s.detectType(line, query.Language)

				// 类型过滤
				if query.Type != "all" && query.Type != "regex" && resultType != query.Type {
					continue
				}

				results = append(results, SearchResult{
					File:    fileName,
					Line:    i + 1,
					Content: strings.TrimSpace(line),
					Match:   query.Pattern,
					Type:    resultType,
				})
			}
		}
	}

	return results
}

func (s *SearchEngine) detectType(line string, language string) string {
	trimmed := strings.TrimSpace(line)

	switch strings.ToLower(language) {
	case "go":
		if strings.HasPrefix(trimmed, "func ") {
			return "function"
		}
		if strings.HasPrefix(trimmed, "type ") && strings.Contains(trimmed, "struct") {
			return "class"
		}
		if strings.HasPrefix(trimmed, "import ") {
			return "import"
		}
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			return "comment"
		}

	case "javascript", "js", "typescript", "ts":
		if strings.Contains(trimmed, "function ") || strings.Contains(trimmed, "=>") {
			return "function"
		}
		if strings.HasPrefix(trimmed, "class ") {
			return "class"
		}
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "const ") && strings.Contains(trimmed, "require(") {
			return "import"
		}
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			return "comment"
		}

	case "python", "py":
		if strings.HasPrefix(trimmed, "def ") {
			return "function"
		}
		if strings.HasPrefix(trimmed, "class ") {
			return "class"
		}
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "from ") {
			return "import"
		}
		if strings.HasPrefix(trimmed, "#") {
			return "comment"
		}
	}

	return "variable"
}
