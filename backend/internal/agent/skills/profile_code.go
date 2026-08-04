package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type ProfileCodeSkill struct{}

func NewProfileCodeSkill() *ProfileCodeSkill {
	return &ProfileCodeSkill{}
}

func (s *ProfileCodeSkill) Name() string {
	return "profile_code"
}

func (s *ProfileCodeSkill) Description() string {
	return "Static performance analysis with optimization suggestions. Input: {\"files\": {\"path\": \"content\", ...}}. Returns key issues with fixes."
}

type profileIssue struct {
	Severity string `json:"severity"`
	Rule     string `json:"rule"`
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message"`
	Suggestion string `json:"suggestion"`
}

type profileReport struct {
	Score         int            `json:"performance_score"`
	Rating        string         `json:"rating"`
	Issues        []profileIssue `json:"issues"`
	TotalFiles    int            `json:"total_files_analyzed"`
}

func (s *ProfileCodeSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	filesRaw, ok := input["files"]
	if !ok {
		return "", fmt.Errorf("files is required")
	}
	filesMap, ok := filesRaw.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("files must be an object")
	}

	var allIssues []profileIssue
	fileCount := 0

	for filePath, contentRaw := range filesMap {
		content, _ := contentRaw.(string)
		if content == "" {
			continue
		}
		fileCount++
		lang := detectLanguage(filePath)
		allIssues = append(allIssues, s.profileFile(filePath, content, lang)...)
	}

	score := 100
	for _, issue := range allIssues {
		switch issue.Severity {
		case "critical":
			score -= 15
		case "warning":
			score -= 5
		case "info":
			score -= 1
		}
	}
	if score < 0 {
		score = 0
	}

	rating := "excellent"
	switch {
	case score >= 90:
		rating = "excellent"
	case score >= 70:
		rating = "good"
	case score >= 50:
		rating = "fair"
	default:
		rating = "poor"
	}

	report := profileReport{
		Score:      score,
		Rating:     rating,
		Issues:     allIssues,
		TotalFiles: fileCount,
	}

	b, _ := json.MarshalIndent(report, "", "  ")
	return string(b), nil
}

func (s *ProfileCodeSkill) profileFile(filePath, content, lang string) []profileIssue {
	switch lang {
	case "shell":
		return s.profileShell(filePath, content)
	case "rust":
		return s.profileRust(filePath, content)
	case "go":
		return s.profileGo(filePath, content)
	case "python":
		return s.profilePython(filePath, content)
	case "cpp", "c":
		return s.profileCpp(filePath, content)
	}
	return nil
}

var pipeRe = regexp.MustCompile(`\|`)
var subshellRe = regexp.MustCompile(`\$\(`)
var catPipeRe = regexp.MustCompile(`cat\s+\S+\s*\|`)

func (s *ProfileCodeSkill) profileShell(filePath, content string) []profileIssue {
	var issues []profileIssue
	lines := strings.Split(content, "\n")

	pipeCount := 0
	subshellCount := 0
	loopCount := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		if catPipeRe.MatchString(trimmed) {
			issues = append(issues, profileIssue{
				Severity: "warning", Rule: "useless_cat", File: filePath, Line: i + 1,
				Message:    "Useless use of cat (UUOC) - use input redirection instead",
				Suggestion: "Replace 'cat file | cmd' with 'cmd < file'",
			})
		}

		pipeCount += len(pipeRe.FindAllString(trimmed, -1))
		subshellCount += len(subshellRe.FindAllString(trimmed, -1))

		if strings.Contains(trimmed, "for ") && strings.Contains(trimmed, "in ") {
			loopCount++
		}
		if strings.Contains(trimmed, "while ") && strings.Contains(trimmed, "do") {
			loopCount++
		}
	}

	if pipeCount > 3 {
		issues = append(issues, profileIssue{
			Severity: "info", Rule: "excessive_pipes", File: filePath,
			Message:    fmt.Sprintf("Script uses %d pipes - each pipe creates a subshell", pipeCount),
			Suggestion: "Consider combining operations with awk or a temporary file",
		})
	}

	if subshellCount > 5 {
		issues = append(issues, profileIssue{
			Severity: "info", Rule: "excessive_subshells", File: filePath,
			Message:    fmt.Sprintf("Script creates %d subshells - expensive on Android", subshellCount),
			Suggestion: "Minimize $() usage, assign results to variables early",
		})
	}

	if loopCount > 0 && pipeCount > loopCount*2 {
		issues = append(issues, profileIssue{
			Severity: "info", Rule: "loop_pipe", File: filePath,
			Message:    "Pipe inside loop causes repeated subshell creation",
			Suggestion: "Move pipe outside the loop or use a temp file",
		})
	}

	return issues
}

func (s *ProfileCodeSkill) profileRust(filePath, content string) []profileIssue {
	var issues []profileIssue
	lines := strings.Split(content, "\n")

	cloneCount := 0
	allocCount := 0
	unwrapCount := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		if strings.Contains(trimmed, ".clone()") {
			cloneCount++
			if cloneCount <= 3 {
				issues = append(issues, profileIssue{
					Severity: "warning", Rule: "clone_usage", File: filePath, Line: i + 1,
					Message:    "Use of .clone() - may cause unnecessary memory allocation",
					Suggestion: "Consider using references (&) or Cow<'_, str> to avoid cloning",
				})
			}
		}

		if strings.Contains(trimmed, "Vec::new") || strings.Contains(trimmed, "String::new") || strings.Contains(trimmed, "HashMap::new") {
			allocCount++
		}

		if strings.Contains(trimmed, ".unwrap()") {
			unwrapCount++
		}

		if strings.Contains(trimmed, "to_string()") || strings.Contains(trimmed, "to_owned()") {
			allocCount++
		}
	}

	if cloneCount > 5 {
		issues = append(issues, profileIssue{
			Severity: "warning", Rule: "excessive_clone", File: filePath,
			Message:    fmt.Sprintf("Found %d .clone() calls - potential performance issue", cloneCount),
			Suggestion: "Refactor to use references where possible, or use Arc/Rc for shared ownership",
		})
	}

	if allocCount > 10 {
		issues = append(issues, profileIssue{
			Severity: "info", Rule: "frequent_allocation", File: filePath,
			Message:    fmt.Sprintf("~%d heap allocations identified - consider pre-allocation", allocCount),
			Suggestion: "Use with_capacity(), reuse buffers, or use stack allocation where possible",
		})
	}

	return issues
}

func (s *ProfileCodeSkill) profileGo(filePath, content string) []profileIssue {
	var issues []profileIssue
	lines := strings.Split(content, "\n")

	stringConcatCount := 0
	hasBufferPool := false
	goroutineCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		if strings.Contains(trimmed, "sync.Pool") || strings.Contains(trimmed, "bytes.Buffer") {
			hasBufferPool = true
		}

		if strings.Contains(trimmed, "go ") && !strings.HasPrefix(trimmed, "//") {
			goroutineCount++
		}

		if strings.Contains(trimmed, "+= ") && !strings.Contains(trimmed, "+=") {
			continue
		}
	}

	if strings.Contains(content, "+") {
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.Contains(trimmed, "+= ") || strings.Contains(trimmed, "= ") && strings.Contains(trimmed, "+") {
				if !strings.Contains(trimmed, "//") && !strings.HasPrefix(trimmed, "//") {
					stringConcatCount++
				}
			}
		}
	}

	if stringConcatCount > 3 && !hasBufferPool {
		issues = append(issues, profileIssue{
			Severity: "warning", Rule: "string_concat_loop", File: filePath,
			Message:    fmt.Sprintf("~%d string concatenations detected - O(n^2) memory copy", stringConcatCount),
			Suggestion: "Use strings.Builder or bytes.Buffer for building strings",
		})
	}

	if goroutineCount > 5 {
		issues = append(issues, profileIssue{
			Severity: "info", Rule: "many_goroutines", File: filePath,
			Message:    fmt.Sprintf("~%d goroutines spawned - ensure proper lifecycle management", goroutineCount),
			Suggestion: "Use worker pools or errgroup.Group for controlled concurrency",
		})
	}

	if !hasBufferPool && goroutineCount > 0 {
		issues = append(issues, profileIssue{
			Severity: "info", Rule: "buffer_pool", File: filePath,
			Message:    "Goroutines detected but no buffer pooling (sync.Pool)",
			Suggestion: "Consider using sync.Pool to reduce GC pressure in concurrent code",
		})
	}

	return issues
}

func (s *ProfileCodeSkill) profilePython(filePath, content string) []profileIssue {
	var issues []profileIssue
	lines := strings.Split(content, "\n")

	nestedLoops := 0
	hasGenerator := false
	listCompCount := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.Contains(trimmed, "for ") && strings.Contains(trimmed, "in ") {
			if i > 0 {
				prevLine := strings.TrimSpace(lines[i-1])
				if strings.Contains(prevLine, "for ") || strings.Contains(prevLine, "while ") {
					nestedLoops++
				}
			}
		}

		if strings.Contains(trimmed, "yield") || strings.Contains(trimmed, "generator") || strings.Contains(trimmed, "()") {
			hasGenerator = true
		}

		if strings.Contains(trimmed, "[") && strings.Contains(trimmed, "for ") && strings.Contains(trimmed, "in ") && strings.Contains(trimmed, "]") {
			listCompCount++
		}
	}

	if nestedLoops > 1 {
		issues = append(issues, profileIssue{
			Severity: "warning", Rule: "nested_loops", File: filePath,
			Message:    fmt.Sprintf("Found ~%d levels of nested loops - O(n^%d) complexity risk", nestedLoops+1, nestedLoops+2),
			Suggestion: "Use itertools.product(), dict lookups, or set operations to flatten",
		})
	}

	if !hasGenerator && listCompCount == 0 && len(lines) > 20 {
		issues = append(issues, profileIssue{
			Severity: "info", Rule: "missing_generators", File: filePath,
			Message:    "No generators or comprehensions found - consider lazy evaluation",
			Suggestion: "Use generator expressions or comprehensions for memory efficiency with large data",
		})
	}

	hasMap := strings.Contains(content, "map(")
	hasFilter := strings.Contains(content, "filter(")

	if !hasMap && !hasFilter && len(lines) > 30 {
		issues = append(issues, profileIssue{
			Severity: "info", Rule: "functional_style", File: filePath,
			Message:    "No use of map/filter - manual loops may be slower",
			Suggestion: "Consider using map(), filter(), or list comprehensions for better performance",
		})
	}

	return issues
}

var cppStringCopyRe = regexp.MustCompile(`std::string\s+\w+\s*=\s*\w+\s*\+`)
var cppVectorPushRe = regexp.MustCompile(`\.push_back\(`)
var cppRawNewRe = regexp.MustCompile(`\bnew\s+\w+[\[\(]`)

func (s *ProfileCodeSkill) profileCpp(filePath, content string) []profileIssue {
	var issues []profileIssue
	lines := strings.Split(content, "\n")

	stringConcatCount := 0
	pushBackCount := 0
	rawNewCount := 0
	hasReserve := strings.Contains(content, ".reserve(")
	hasSmartPtr := strings.Contains(content, "unique_ptr") || strings.Contains(content, "shared_ptr")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		if cppStringCopyRe.MatchString(trimmed) {
			stringConcatCount++
			if stringConcatCount <= 2 {
				issues = append(issues, profileIssue{
					Severity: "warning", Rule: "string_concat", File: filePath, Line: i + 1,
					Message:    "String concatenation with + creates temporary objects",
					Suggestion: "Use std::string::append() or std::ostringstream to avoid copies",
				})
			}
		}

		if cppVectorPushRe.MatchString(trimmed) {
			pushBackCount++
		}

		if cppRawNewRe.MatchString(trimmed) && !hasSmartPtr {
			rawNewCount++
		}
	}

	if stringConcatCount > 3 {
		issues = append(issues, profileIssue{
			Severity: "warning", Rule: "repeated_string_concat", File: filePath,
			Message:    fmt.Sprintf("~%d string concatenations - each creates a temporary std::string", stringConcatCount),
			Suggestion: "Use std::ostringstream, fmt::format(), or reserve+append pattern",
		})
	}

	if pushBackCount > 5 && !hasReserve {
		issues = append(issues, profileIssue{
			Severity: "info", Rule: "vector_no_reserve", File: filePath,
			Message:    fmt.Sprintf("~%d push_back() calls without .reserve() - causes repeated reallocations", pushBackCount),
			Suggestion: "Call vec.reserve(N) before the loop to pre-allocate memory",
		})
	}

	if rawNewCount > 3 && !hasSmartPtr {
		issues = append(issues, profileIssue{
			Severity: "warning", Rule: "raw_allocation", File: filePath,
			Message:    fmt.Sprintf("~%d raw new allocations without smart pointers", rawNewCount),
			Suggestion: "Use std::unique_ptr or std::shared_ptr to prevent memory leaks",
		})
	}

	if strings.Contains(content, "std::endl") && !strings.Contains(content, "// flush") {
		issues = append(issues, profileIssue{
			Severity: "info", Rule: "endl_flush", File: filePath,
			Message:    "std::endl forces a flush - use '\\n' for better performance",
			Suggestion: "Replace std::endl with '\\n' unless explicit flush is needed",
		})
	}

	return issues
}

func (s *ProfileCodeSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  true,
		Essential: false,
		NeedsDB:   false,
		NeedsLLM:  true,
	}
}
