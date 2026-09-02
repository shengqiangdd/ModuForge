package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/moduforge/backend/internal/agent/registry"
)

// PerfMonitorSkill detects performance anti-patterns in shell scripts, Go, Rust, and C/C++ source files.
type PerfMonitorSkill struct{}

func NewPerfMonitorSkill() *PerfMonitorSkill {
	return &PerfMonitorSkill{}
}

func (s *PerfMonitorSkill) Name() string { return "perf_monitor" }

func (s *PerfMonitorSkill) Description() string {
	return "Scan module source files (.sh/.go/.rs/.kt/.c/.cpp) for performance hotspots. Detects: shell anti-patterns (cat | grep | grep chains), Go issues (goroutine leaks, defer in loops, resource leaks), Rust patterns (excessive cloning), C/C++ issues (raw pointers, buffer overflows). Returns JSON report with issues, score, and fix suggestions."
}

type PerfIssue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Fix         string `json:"fix"`
	Snippet     string `json:"snippet"`
}

type PerfReport struct {
	TotalFiles      int         `json:"total_files"`
	Issues          []PerfIssue `json:"issues"`
	Score           int         `json:"score"`
	Recommendations []string    `json:"recommendations"`
}

var reCache = make(map[string]*regexp.Regexp)
var dupPrio = map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}

func getCachedRE(pattern string) *regexp.Regexp {
	if re, ok := reCache[pattern]; ok {
		return re
	}
	compiled := regexp.MustCompile(pattern)
	reCache[pattern] = compiled
	return compiled
}

func extractContent(input map[string]interface{}, path string) string {
	if c, ok := input["contents"].(map[string]string); ok {
		if v, ok := c[path]; ok {
			return v
		}
	}
	if c, ok := input["contents"].(map[string]interface{}); ok {
		if v, ok := c[path].(string); ok {
			return v
		}
	}
	if v, ok := input["content"].(string); ok {
		return v
	}
	if m, ok := input["content"].(map[string]interface{}); ok {
		if s, ok := m[path].(string); ok {
			return s
		}
	}
	return ""
}

func (s *PerfMonitorSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	var filePaths []string
	if fArr, ok := input["files"].([]interface{}); ok {
		for _, fp := range fArr {
			if p, ok := fp.(string); ok && p != "" {
				filePaths = append(filePaths, p)
			}
		}
	}
	if pathVal, hasPath := input["path"]; hasPath {
		if p, ok := pathVal.(string); ok && p != "" {
			filePaths = append(filePaths, p)
		}
	}

	var allIssues []PerfIssue
	totalFiles := 0
	languageFilter, _ := input["language"].(string)

	for _, pathStr := range filePaths {
		content := extractContent(input, pathStr)
		if content == "" {
			continue
		}
		language := detectLanguage(pathStr)
		if languageFilter != "" && !strings.EqualFold(language, languageFilter) {
			continue
		}

		ext := strings.ToLower(filepath.Ext(pathStr))
		if ext == "" || (ext != ".sh" && ext != ".go" && ext != ".rs" && ext != ".kt" && ext != ".c" && ext != ".cpp") {
			if ext == "" {
				language = detectLanguage(pathStr)
			}
		}

		totalFiles++
		lines := strings.Split(content, "\n")

		switch strings.ToLower(language) {
		case "shell", "sh", "bash":
			allIssues = append(allIssues, scanShell(lines, pathStr)...)
		case "go":
			allIssues = append(allIssues, scanGo(lines, pathStr)...)
		case "rust":
			allIssues = append(allIssues, scanRust(lines, pathStr)...)
		case "c", "c++", "cpp":
			allIssues = append(allIssues, scanCpp(lines, pathStr)...)
		default:
			allIssues = append(allIssues, scanGeneric(lines, pathStr, language)...)
		}
	}

	return buildJSONReport(totalFiles, allIssues), nil
}

func (s *PerfMonitorSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{ReadOnly: true, Essential: false, Core: false, NeedsDB: false, NeedsLLM: false}
}

// ===== SHELL SCANNING =====

var shellPatternRe = func() map[string]string {
	m := make(map[string]string)
	m["pipe_chain"]   = `\bcat\b\s*\|\s*\bgrep\b.*\|\s*\bgrep\b`
	m["uuoc"]         = `\bcat\b\s+\S+\s*\|\s*\bgrep\b`
	m["echo_read"]    = `\becho\b.*\|\s*while\s+read\b`
	m["cat_read"]     = `\bcat\b.*\|\s*while\s+read\b`
	m["find_loop"]    = `find\s+\S+.*\b(for|while)\b`
	m["shebang_bash"] = `^#!\s*/usr/bin/env\b.*bash`
	m["sleep_zero"]   = `sleep\s+0\b`
	m["eval_risk"]    = `(?<!\w)eval\s+\(`
	return m
}()

func scanShell(lines []string, path string) []PerfIssue {
	var issues []PerfIssue
	jlines := strings.Join(lines, "\n")
	infiniteLoopRe := getCachedRE(`while\s+true|for\s+;;`)

	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}

		if re := shellPatternRe["pipe_chain"]; re != "" && matched(t, re) {
			issues = append(issues, PerfIssue{Type: "shell_pipe_chain", Severity: "high", File: path, Line: i + 1, Description: "Inefficient triple-pipe chain: cat | grep | grep", Impact: "Unnecessary cat; two grep processes; high CPU/memory", Fix: "Use grep -E 'pattern1|pattern2' $FILE or awk instead", Snippet: t})
		} else if re := shellPatternRe["uuoc"]; re != "" && matched(t, re) {
			issues = append(issues, PerfIssue{Type: "useless_cat", Severity: "low", File: path, Line: i + 1, Description: "Useless use of cat (UUOC)", Impact: "Spawns unnecessary process", Fix: "grep -q \"pattern\" \"$FILE\"", Snippet: t})
		} else if re := shellPatternRe["cat_read"]; re != "" && matched(t, re) {
			issues = append(issues, PerfIssue{Type: "subshell_var_loss", Severity: "medium", File: path, Line: i + 1, Description: "Pipe to while read loses variable scope", Impact: "Variable changes disappear after pipe ends", Fix: "Use process substitution: done < <(cmd)", Snippet: t})
		} else if re := shellPatternRe["echo_read"]; re != "" && matched(t, re) {
			issues = append(issues, PerfIssue{Type: "subshell_var_loss", Severity: "medium", File: path, Line: i + 1, Description: "Pipe to while read loses variable scope", Impact: "Variable changes disappear after pipe ends", Fix: "Use process substitution: done < <(cmd)", Snippet: t})
		} else if re := shellPatternRe["find_loop"]; re != "" && matched(t, re) {
			issues = append(issues, PerfIssue{Type: "find_in_loop", Severity: "high", File: path, Line: i + 1, Description: "find inside loop — repeated traversal", Impact: "O(n²): directory scan on every iteration", Fix: "Run find once into tmp file, iterate over result", Snippet: t})
		} else if re := shellPatternRe["eval_risk"]; re != "" && matched(t, re) {
			issues = append(issues, PerfIssue{Type: "eval_risk", Severity: "high", File: path, Line: i + 1, Description: "eval re-parses script every time", Impact: "CPU overhead; security risk", Fix: "Avoid eval; use arrays or direct assignment", Snippet: t})
		}

		if i == 0 && matched(t, shellPatternRe["shebang_bash"]) {
			issues = append(issues, PerfIssue{Type: "shebang_choice", Severity: "low", File: path, Line: 1, Description: "Shebang uses bash instead of sh", Impact: "Magisk uses ash/toybox — no bash features", Fix: "Use #!/system/bin/sh", Snippet: t})
		}

		if i == 0 && infiniteLoopRe.MatchString(t) {
			issues = append(issues, PerfIssue{Type: "infinite_no_guard", Severity: "medium", File: path, Line: 1, Description: "Infinite loop without trap/cleanup", Impact: "No recovery path on failure", Fix: "Add trap EXIT/HUP/INT and max retry counter", Snippet: t})
		}
	}

	// Check sleep 0 in loop context
	sleepZeroRe := getCachedRE(`sleep\s+0\b`)
	forRangeRe := getCachedRE(`while\s+true|for\s+;;`)
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			if sleepZeroRe.MatchString(line) {
				ctx := getNeighborLines(lines, i, 5)
				if forRangeRe.MatchString(ctx) {
					issues = append(issues, PerfIssue{Type: "busy_wait", Severity: "medium", File: path, Line: i + 1, Description: "Busy-wait with sleep 0 wastes CPU", Impact: "Does not yield processor", Fix: "sleep 1", Snippet: strings.TrimSpace(line)})
				}
			}
		}
	}

	if infiniteLoopRe.MatchString(jlines) {
		found := false
		for _, iss := range issues {
			if iss.Type == "infinite_no_guard" { found = true; break }
		}
		if !found {
			issues = append(issues, PerfIssue{Type: "infinite_no_guard", Severity: "medium", File: path, Line: 1, Description: "Infinite loop without trap/cleanup", Impact: "No recovery path", Fix: "Add trap EXIT/HUP/INT and max retry counter", Snippet: ""})
		}
	}

	return dedup(issues)
}

// ===== GO SCANNING =====

var goPatternRe = func() map[string]string {
	m := make(map[string]string)
	m["str_append"]   = `\w+\s*\+=\s+"[^"]*`
	m["go_func"]      = `\bgo\s+func\b`
	m["defer_call"]   = `\tdefer \w+\(`
	m["json_marsh"]   = `json\.Marshal\(`
	m["sprintf"]      = `fmt\.Sprintf\(`
	m["append_slic"]  = `append\(.*,`
	m["time_now"]     = `time\.Now\(\)`
	m["unbuf_chan"]   = `make\(chan\s`
	m["open_res"]     = `os\.Open|bufio\.NewReader|net\.Dial\(|db\.Query\(`
	return m
}()

func scanGo(lines []string, path string) []PerfIssue {
	var issues []PerfIssue

	inForRange := false
	var stmtCtx []bool // tracks brace nesting
	hasClose := false
	preLoopCode := ""

	for i, line := range lines {
		t := strings.TrimSpace(line)
		pc := preLoopCode
		if len(preLoopCode) > 800 {
			preLoopCode = preLoopCode[len(preLoopCode)-600:]
		}
		preLoopCode += t + "\n"

		forIdx := strings.Index(t, "for ")

		if strings.HasPrefix(t, "//") || strings.HasPrefix(t, "*") {
			continue
		}

		opens := hasFuncStr(t, goPatternRe["go_func"])
		closes := hasFuncStr(t, "}\n") || t == "}"
		braceDiff := strings.Count(t, "{") - strings.Count(t, "}")

		if forIdx >= 0 && strings.Contains(t[forIdx:], "range") {
			inForRange = true
			stmtCtx = append(stmtCtx, true)
		}
		if forIdx >= 0 {
			inForRange = true
			stmtCtx = append(stmtCtx, true)
		}
		if bracesMatched(t) {
			lastN := 0
			for _, val := range stmtCtx {
				if val { lastN++; break }
			}
			if lastN > 0 && closes && hasFuncStr(t, "}") {
				inForRange = false
				stmtCtx = stmtCtx[:len(stmtCtx)-1]
			}
		}

		if hasFuncStr(t, ".Close(") || hasFuncStr(t, "defer ") {
			hasClose = true
		}

		_ = opens
		_ = closes
		_ = braceDiff

		if inForRange && hasFuncStr(t, goPatternRe["str_append"]) {
			issues = append(issues, PerfIssue{Type: "string_concat_loop", Severity: "medium", File: path, Line: i + 1, Description: "String += inside loop: O(nÂ²) allocation", Impact: "Allocates new string each iteration", Fix: "Use strings.Builder", Snippet: t})
		}
		if hasFuncStr(t, goPatternRe["go_func"]) {
			sctx := joinLines(lines, maxInt(i-4, 0), i)
			cancelCtx := hasFuncStr(sctx, "context.WithCancel") || hasFuncStr(sctx, "cancel()")
			if !cancelCtx {
				issues = append(issues, PerfIssue{Type: "goroutine_leak", Severity: "medium", File: path, Line: i + 1, Description: "Goroutine launched without cancellation", Impact: "Cannot abort externally; goroutine leak on error", Fix: "Use context.WithCancel; defer cancel(); select { case <-ctx.Done(): return", Snippet: t})
			}
		}
		if inForRange && hasFuncStr(t, goPatternRe["defer_call"]) {
			issues = append(issues, PerfIssue{Type: "defer_in_loop", Severity: "medium", File: path, Line: i + 1, Description: "defer inside loop accumulates calls", Impact: "Deferred call runs on function exit, not iteration end", Fix: "Replace with explicit close at loop body end", Snippet: t})
		}
		if inForRange && hasFuncStr(t, goPatternRe["json_marsh"]) {
			issues = append(issues, PerfIssue{Type: "json_marshal_loop", Severity: "medium", File: path, Line: i + 1, Description: "json.Marshal in loop", Impact: "Expensive repeated serialization", Fix: "Batch into slice and marshal once", Snippet: t})
		}
		if inForRange && hasFuncStr(t, goPatternRe["sprintf"]) {
			issues = append(issues, PerfIssue{Type: "sprintf_loop", Severity: "low", File: path, Line: i + 1, Description: "fmt.Sprintf in loop allocates reflectionally", Impact: "Slower than strconv for simple conversions", Fix: "Use strconv.Itoa or strconv.FormatFloat", Snippet: t})
		}
		if inForRange && hasFuncStr(t, goPatternRe["time_now"]) {
			issues = append(issues, PerfIssue{Type: "time_now_loop", Severity: "low", File: path, Line: i + 1, Description: "time.Now syscall inside tight loop", Impact: "Syscall per iteration; expensive", Fix: "Cache timestamp outside loop", Snippet: t})
		}
		if inForRange && hasFuncStr(t, goPatternRe["append_slic"]) {
			hasMake := hasFuncStr(pc, "make(") && hasFuncStr(pc, ",")
			if !hasMake {
				issues = append(issues, PerfIssue{Type: "slice_grow_loop", Severity: "low", File: path, Line: i + 1, Description: "append in loop without pre-allocation", Impact: "Capacity doubles; multiple reallocations", Fix: "Pre-allocate with make([]T, 0, cap)", Snippet: t})
			}
		}
		if hasFuncStr(t, goPatternRe["unbuf_chan"]) {
			issues = append(issues, PerfIssue{Type: "channel_buffering", Severity: "low", File: path, Line: i + 1, Description: "Unbuffered channel may block producer/consumer", Impact: "Reduces throughput", Fix: "Consider buffered channel: make(chan T, N)", Snippet: t})
		}
		if hasFuncStr(t, goPatternRe["open_res"]) && !hasClose {
			rname := extractResName(t)
			issues = append(issues, PerfIssue{Type: "resource_leak", Severity: "high", File: path, Line: i + 1, Description: fmt.Sprintf("Resource opened (%s) without tracked cleanup", rname), Impact: "Leaks FD/memory; hits OS limits", Fix: "Add defer <type>.Close()", Snippet: t})
		}

	}

	return dedup(issues)
}

// ===== RUST SCANNING =====

var rustPatternRe = func() map[string]string {
	m := make(map[string]string)
	m["clone"]       = `\.\s*clone\(\s*\)`
	m["unwrap"]      = `\.\s*unwrap\(\s*\)`
	m["vec_push"]    = `\w+\.push\(`
	m["println"]     = `(println!|eprintln!)\(`
	m["string_from"] = `String::from\(`
	return m
}()

func scanRust(lines []string, path string) []PerfIssue {
	var issues []PerfIssue
	jlines := strings.Join(lines, "\n")
	totalClones := countMatches(jlines, rustPatternRe["clone"])
	totalPushes := countMatches(jlines, rustPatternRe["vec_push"])

	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "//") {
			continue
		}

		if totalClones > 5 && hasFuncStr(t, rustPatternRe["clone"]) {
			issues = append(issues, PerfIssue{Type: "rust_clone_abuse", Severity: "medium", File: path, Line: i + 1, Description: fmt.Sprintf("%d clones in file — excessive allocation", totalClones), Impact: "Wastes CPU and heap", Fix: "Use references (&T), Cow<T>, or Arc<T>", Snippet: t})
		}
		if hasFuncStr(t, rustPatternRe["unwrap"]) && !hasFuncStr(t, "#[test]") {
			issues = append(issues, PerfIssue{Type: "rust_unwrap_panic", Severity: "low", File: path, Line: i + 1, Description: ".unwrap() panics on None/Err", Impact: "Production crash on unexpected input", Fix: "Use .expect(\"msg\") or ? operator", Snippet: t})
		}
		if totalPushes > 10 && hasFuncStr(t, rustPatternRe["vec_push"]) {
			hasResrv := hasPrevLines(lines, i, 15, "reserve(")
			if !hasResrv {
				issues = append(issues, PerfIssue{Type: "rust_vec_grow", Severity: "low", File: path, Line: i + 1, Description: fmt.Sprintf("%d vec pushes without reserve", totalPushes), Impact: "Repeated growth reallocation", Fix: "Vec::with_capacity(n)", Snippet: t})
			}
		}
		if hasFuncStr(t, rustPatternRe["println"]) {
			issues = append(issues, PerfIssue{Type: "rust_println_prod", Severity: "low", File: path, Line: i + 1, Description: "println! in production code", Impact: "Blocking stdout; bad for logging", Fix: "Use log crate or tracing subscriber", Snippet: t})
		}
	}
	return dedup(issues)
}

// ===== CPP SCANNING =====

var cppPatternRe = func() map[string]string {
	m := make(map[string]string)
	m["raw_new"]      = `\bnew\s+[a-zA-Z_]`
	m["strcpy"]       = `\b(strcpy|strcat|sprintf|gets)\b`
	m["cout_printf"]  = `std::cout|std::cerr|\bprintf\b`
	m["raw_delete"]   = `\bdelete\s+(?![])`
	return m
}()

func scanCpp(lines []string, path string) []PerfIssue {
	var issues []PerfIssue

	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "/*") {
			continue
		}

		if hasFuncStr(t, cppPatternRe["strcpy"]) && !hasFuncStr(t, "snprintf") && !hasFuncStr(t, "strncpy") {
			funcType := ""
			if matched(t, `strcpy`) { funcType = "strcpy" }
			if matched(t, `strcat`) { funcType = "strcat" }
			if matched(t, `sprintf`) { funcType = "sprintf" }
			if matched(t, `gets`) { funcType = "gets" }
			issues = append(issues, PerfIssue{Type: "cpp_buffer_overflow", Severity: "critical", File: path, Line: i + 1, Description: "Unsafe " + funcType + " — buffer overflow vulnerability", Impact: "Stack/heap corruption; exploitable", Fix: "Use snprintf, strncpy, or std::string", Snippet: t})
		}

		if hasFuncStr(t, cppPatternRe["raw_new"]) && !hasFuncStr(t, "unique_ptr") && !hasFuncStr(t, "shared_ptr") && !hasFuncStr(t, "make_unique") && !hasFuncStr(t, "make_shared") {
			issues = append(issues, PerfIssue{Type: "cpp_raw_allocation", Severity: "high", File: path, Line: i + 1, Description: "Raw new without smart pointer", Impact: "Memory leak / double-free risk", Fix: "Use std::make_unique or std::make_shared", Snippet: t})
		}

		if hasFuncStr(t, cppPatternRe["cout_printf"]) {
			issues = append(issues, PerfIssue{Type: "cpp_io_blocking", Severity: "low", File: path, Line: i + 1, Description: "Synced I/O slows execution", Impact: "I/O flush between writes", Fix: "ios_base::sync_with_stdio(false); cin.tie(nullptr);", Snippet: t})
		}
	}
	return dedup(issues)
}

// ===== GENERIC SCANNING =====

func scanGeneric(lines []string, path, lang string) []PerfIssue {
	if strings.HasSuffix(path, ".kt") {
		var issues []PerfIssue
		emptyLambdaRe := getCachedRE(`\.\S+\(\)\s*\{\s*->\s*$`)
		for i, line := range lines {
			if emptyLambdaRe.MatchString(line) {
				issues = append(issues, PerfIssue{Type: "kt_empty_lambda", Severity: "medium", File: path, Line: i + 1, Description: "Empty lambda body wastes allocation", Impact: "Creates object for no-op", Fix: "Remove or implement the callback", Snippet: strings.TrimSpace(line)})
			}
		}
		return issues
	}
	return nil
}

// ===== HELPERS =====

func matched(s, pat string) bool {
	if re := getCachedRE(pat); re != nil {
		return re.MatchString(s)
	}
	return false
}

func hasFuncStr(s, pat string) bool {
	return matched(s, pat)
}

func matchesCount(s, pat string) int {
	re := getCachedRE(pat)
	if re == nil {
		return 0
	}
	return len(re.FindAllString(s, -1))
}

func countMatches(s, pat string) int {
	return matchesCount(s, pat)
}

func hasPrevLines(lines []string, idx, n int, sub string) bool {
	start := 0
	if idx > n { start = idx - n }
	for j := start; j < idx; j++ {
		if strings.Contains(lines[j], sub) { return true }
	}
	return false
}

func joinLines(lines []string, lo, hi int) string {
	if hi < lo {
		return ""
	}
	if hi > len(lines) {
		hi = len(lines)
	}
	return strings.Join(lines[lo:hi], "\n")
}

func braceCount(s string) int {
	return strings.Count(s, "{") - strings.Count(s, "}")
}

func bracesMatched(s string) bool {
	return braceCount(s) <= 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func getNeighborLines(lines []string, idx, n int) string {
	start := 0
	if idx > n { start = idx - n }
	end := idx + n + 1
	if end > len(lines) { end = len(lines) }
	return strings.Join(lines[start:end], "\n")
}

func extractResName(t string) string {
	switch {
	case strings.Contains(t, "os.Open"): return "File"
	case strings.Contains(t, "bufio.New"): return "BufferedReader"
	case strings.Contains(t, "net.Dial"): return "Conn"
	default: return "Unknown"
	}
}

// ===== REPORT BUILDING =====

func buildJSONReport(totalFiles int, issues []PerfIssue) string {
	score := 100 - len(issues)*5
	if score < 0 {
		score = 0
	}
	recs := genRecs(issues)
	data, err := json.Marshal(PerfReport{TotalFiles: totalFiles, Issues: issues, Score: score, Recommendations: recs})
	if err != nil {
		return fmt.Sprintf(`{"error":"%s","total_files":%d,"score":0}`, err.Error(), totalFiles)
	}
	return string(data)
}

func genRecs(issues []PerfIssue) []string {
	typeSeen := make(map[string]bool)
	recMap := map[string]string{
		"shell_pipe_chain":     "Replace cat | grep | grep with grep -E or awk for single-pass processing",
		"useless_cat":          "Pass file arguments directly to commands that support them instead of cat",
		"goroutine_leak":       "Ensure goroutines have cancellation via context or quit channels",
		"defer_in_loop":        "Move defer outside loops; use explicit per-iteration cleanup",
		"resource_leak":        "Always pair resource opens with defer close",
		"string_concat_loop":   "Use strings.Builder for iterative string construction",
		"sprintf_loop":         "Use strconv for simple numeric formatting in loops",
		"json_marshal_loop":    "Batch data and marshal once, or stream with json.Encoder",
		"time_now_loop":        "Sample timestamps once outside the loop",
		"slice_grow_loop":      "Pre-allocate slices with make(T, 0, expectedCap)",
		"channel_buffering":    "Use buffered channels where producers/consumers differ in rate",
		"infinite_no_guard":    "Add trap EXIT/HUP/INT handlers and max-retry logic",
		"busy_wait":            "Increase sleep interval; even sleep 1 reduces CPU waste vs 0",
		"subshell_var_loss":    "Use process substitution for variable preservation in pipes",
		"find_in_loop":         "Execute find once and iterate results; avoid repeated scans",
		"eval_risk":            "Avoid eval; use arrays or parse safely without re-parsing",
		"shebang_choice":       "Use #!/system/bin/sh for Magisk/Apatch compatibility",
		"rust_clone_abuse":     "Minimize clones via borrows, &T references, Cow<T>, Arc<T>",
		"rust_unwrap_panic":    "Replace .unwrap() with .expect() or ? for safe propagation",
		"rust_vec_grow":        "Pre-allocate Vec::with_capacity(n) before pushing",
		"rust_println_prod":    "Use log crate or tracing instead of println!",
		"cpp_buffer_overflow":  "Replace strcpy/strcat/sprintf with snprintf or std::string",
		"cpp_raw_allocation":   "Eliminate manual new/delete with unique_ptr/shared_ptr",
		"cpp_io_blocking":      "Add ios_base::sync_with_stdio(false); cin.tie(nullptr);",
	}
	var recs []string
	for _, iss := range issues {
		t := iss.Type
		if typeSeen[t] { continue }
		typeSeen[t] = true
		if r, ok := recMap[t]; ok {
			recs = append(recs, r)
		}
	}
	if len(recs) == 0 {
		recs = append(recs, "No performance issues detected.")
	}
	return recs
}

func dedup(issues []PerfIssue) []PerfIssue {
	type key struct {
		file string
		typ  string
		line int
	}
	seen := make(map[key]bool)
	out := make([]PerfIssue, 0, len(issues))
	bestPrio := make(map[key]int)

	for _, iss := range issues {
		k := key{file: iss.File, typ: iss.Type, line: iss.Line}
		if !seen[k] {
			seen[k] = true
			bestPrio[k] = dupPrio[iss.Severity]
			out = append(out, iss)
		} else if dupPrio[iss.Severity] < bestPrio[k] {
			idx := len(out) - 1
			for i := len(out) - 1; i >= 0; i-- {
				ik := key{file: out[i].File, typ: out[i].Type, line: out[i].Line}
				if ik == k {
					idx = i
					break
				}
			}
			out[idx] = iss
		}
	}
	return out
}

