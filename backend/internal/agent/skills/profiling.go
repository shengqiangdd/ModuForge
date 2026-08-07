package skills

import (
	"context"
	"fmt"
	"strings"
	"github.com/moduforge/backend/internal/agent/registry"
)

// ProfilingSkill provides performance profiling guidance for different languages.
// It analyzes code patterns that may cause performance issues and suggests
// profiling tools and techniques.
type ProfilingSkill struct{}

func NewProfilingSkill() *ProfilingSkill {
	return &ProfilingSkill{}
}

func (s *ProfilingSkill) Name() string { return "profiling" }

func (s *ProfilingSkill) Description() string {
	return "Analyze code for performance issues and provide profiling guidance. Input: {\"path\": \"...\", \"content\": \"...\", \"language\": \"go|rust|c++|shell|python|typescript\", \"target\": \"cpu|memory|io|all\"}. Returns: hotspot identification, profiling tool recommendations, optimization suggestions, and sample profiling commands."
}

type ProfilingResult struct {
	FilePath      string             `json:"file_path"`
	Language      string             `json:"language"`
	Target        string             `json:"target"`
	Hotspots      []PerformanceIssue `json:"hotspots"`
	Tools         []ProfilingTool    `json:"tools"`
	Suggestions   []string           `json:"suggestions"`
	SampleCommand string             `json:"sample_command"`
	Score         int                `json:"score"`
}

type PerformanceIssue struct {
	Type        string `json:"type"` // cpu, memory, io, algorithm
	Severity    string `json:"severity"`
	Line        int    `json:"line"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Fix         string `json:"fix"`
}

type ProfilingTool struct {
	Name        string `json:"name"`
	Purpose     string `json:"purpose"`
	Command     string `json:"command"`
	Language    string `json:"language"`
}

func (s *ProfilingSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	content, _ := input["content"].(string)
	language, _ := input["language"].(string)
	path, _ := input["path"].(string)
	target, _ := input["target"].(string)

	if content == "" {
		return "", fmt.Errorf("content is required")
	}
	if language == "" {
		language = detectLanguage(path)
	}
	if target == "" {
		target = "all"
	}

	lines := strings.Split(content, "\n")
	result := analyzePerformance(path, lines, language, target)
	return formatProfilingReport(result), nil
}

func (s *ProfilingSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  true,
		Essential: false,
		Core:      false,
		NeedsDB:   false,
		NeedsLLM:  false,
	}
}

func analyzePerformance(path string, lines []string, language, target string) ProfilingResult {
	r := ProfilingResult{
		FilePath: path,
		Language: language,
		Target:   target,
		Hotspots: make([]PerformanceIssue, 0),
		Tools:    make([]ProfilingTool, 0),
		Suggestions: make([]string, 0),
		Score:    100,
	}

	// Detect hotspots based on language
	switch language {
	case "go":
		r.Hotspots = append(r.Hotspots, scanGoPerformance(lines)...)
		r.Tools = append(r.Tools, goProfilingTools()...)
	case "rust":
		r.Hotspots = append(r.Hotspots, scanRustPerformance(lines)...)
		r.Tools = append(r.Tools, rustProfilingTools()...)
	case "python":
		r.Hotspots = append(r.Hotspots, scanPythonPerformance(lines)...)
		r.Tools = append(r.Tools, pythonProfilingTools()...)
	case "shell":
		r.Hotspots = append(r.Hotspots, scanShellPerformance(lines)...)
		r.Tools = append(r.Tools, shellProfilingTools()...)
	case "c++", "c":
		r.Hotspots = append(r.Hotspots, scanCppPerformance(lines)...)
		r.Tools = append(r.Tools, cppProfilingTools()...)
	case "typescript", "javascript":
		r.Hotspots = append(r.Hotspots, scanJSPerformance(lines)...)
		r.Tools = append(r.Tools, jsProfilingTools()...)
	}

	// Filter by target
	if target != "all" {
		filtered := make([]PerformanceIssue, 0)
		for _, h := range r.Hotspots {
			if h.Type == target {
				filtered = append(filtered, h)
			}
		}
		r.Hotspots = filtered
	}

	// Filter tools by target
	if target != "all" {
		filteredTools := make([]ProfilingTool, 0)
		for _, t := range r.Tools {
			if target == "cpu" && (strings.Contains(t.Name, "cpu") || strings.Contains(t.Purpose, "cpu") || strings.Contains(t.Purpose, "profil")) {
				filteredTools = append(filteredTools, t)
			} else if target == "memory" && (strings.Contains(t.Name, "mem") || strings.Contains(t.Purpose, "mem") || strings.Contains(t.Purpose, "alloc")) {
				filteredTools = append(filteredTools, t)
			} else if target == "io" && (strings.Contains(t.Name, "io") || strings.Contains(t.Purpose, "io") || strings.Contains(t.Purpose, "block")) {
				filteredTools = append(filteredTools, t)
			}
		}
		if len(filteredTools) > 0 {
			r.Tools = filteredTools
		}
	}

	// Score based on issues found
	for _, h := range r.Hotspots {
		switch h.Severity {
		case "high":
			r.Score -= 15
		case "medium":
			r.Score -= 8
		case "low":
			r.Score -= 3
		}
	}
	if r.Score < 0 {
		r.Score = 0
	}

	// Generate suggestions
	r.Suggestions = generateProfilingSuggestions(r)

	// Sample profiling command
	r.SampleCommand = getSampleProfilingCommand(language, target)

	return r
}

// ── Go Performance Scanning ──

func scanGoPerformance(lines []string) []PerformanceIssue {
	var issues []PerformanceIssue
	goPatterns := []struct {
		name     string
		pattern  string
		pType    string
		severity string
		impact   string
		fix      string
	}{
		{"string_concat_loop", "strings.", "cpu", "medium", "O(n²) string concatenation in loop",
			"use strings.Builder or bytes.Buffer"},
		{"goroutine_leak", "go func", "memory", "medium", "Goroutine may leak without cancellation",
			"context.WithCancel, errgroup, or sync.WaitGroup"},
		{"sync_map_heavy", "sync.Map", "cpu", "low", "sync.Map has high overhead for small maps",
			"use sync.RWMutex with regular map for <1000 entries"},
		{"json_marshal_loop", "json.Marshal", "cpu", "medium", "JSON marshaling in loop is expensive",
			"use json.Encoder or cache struct tags"},
		{"fmt_sprintf_loop", "fmt.Sprintf", "cpu", "low", "fmt.Sprintf allocates in hot path",
			"use strconv for simple conversions"},
		{"append_in_loop", "append(", "memory", "low", "Repeated append may cause excessive allocation",
			"pre-allocate slice with make([]T, 0, expectedCap)"},
		{"defer_in_loop", "defer ", "cpu", "medium", "defer in loop adds overhead per iteration",
			"move defer outside loop or use explicit cleanup"},
		{"time_now_loop", "time.Now()", "cpu", "low", "time.Now() syscalls in tight loop",
			"batch time checks, use monotonic clock"},
		{"channel_unbuffered", "make(chan", "io", "medium", "Unbuffered channel may block goroutines",
			"consider buffered channel: make(chan T, size)"},
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		for _, p := range goPatterns {
			if strings.Contains(trimmed, p.pattern) {
				issues = append(issues, PerformanceIssue{
					Type:        p.pType,
					Severity:    p.severity,
					Line:        i + 1,
					Description: fmt.Sprintf("Performance issue: %s", p.name),
					Impact:      p.impact,
					Fix:         p.fix,
				})
			}
		}
	}
	return issues
}

func goProfilingTools() []ProfilingTool {
	return []ProfilingTool{
		{Name: "pprof (CPU)", Purpose: "CPU profiling", Language: "go", Command: "go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30"},
		{Name: "pprof (Memory)", Purpose: "Memory profiling", Language: "go", Command: "go tool pprof http://localhost:6060/debug/pprof/heap"},
		{Name: "pprof (Goroutine)", Purpose: "Goroutine leak detection", Language: "go", Command: "go tool pprof http://localhost:6060/debug/pprof/goroutine"},
		{Name: "trace", Purpose: "Execution tracing", Language: "go", Command: "go test -trace trace.out && go tool trace trace.out"},
		{Name: "benchstat", Purpose: "Benchmark comparison", Language: "go", Command: "go test -bench=. -benchmem -count=5 | benchstat"},
		{Name: "runtime.SetMutexProfileFraction", Purpose: "Mutex contention", Language: "go", Command: "go tool pprof http://localhost:6060/debug/pprof/mutex"},
		{Name: "runtime.SetBlockProfileRate", Purpose: "Block profiling", Language: "go", Command: "go tool pprof http://localhost:6060/debug/pprof/block"},
	}
}

// ── Rust Performance Scanning ──

func scanRustPerformance(lines []string) []PerformanceIssue {
	var issues []PerformanceIssue
	rustPatterns := []struct {
		name     string
		pattern  string
		pType    string
		severity string
		impact   string
		fix      string
	}{
		{"clone_abuse", ".clone()", "cpu", "medium", "Excessive cloning wastes CPU and memory",
			"use references, Cow<str>, or Arc for shared ownership"},
		{"unwrap_in_loop", ".unwrap()", "cpu", "low", "Unwrap in hot path may panic unexpectedly",
			"use expect() with message or proper error handling"},
		{"vec_push_loop", "push(", "memory", "low", "Repeated push without pre-allocation",
			"use Vec::with_capacity(n) to pre-allocate"},
		{"string_allocation", "String::from", "memory", "medium", "Heap allocation on each call",
			"reuse strings, use &str references where possible"},
		{"collect_large", ".collect::<Vec", "memory", "medium", "Collecting into large Vec",
			"consider iterators, use .into_iter() to avoid cloning"},
		{"println_debug", "println!", "io", "low", "println! in production code is slow",
			"use log crate or tracing for structured logging"},
		{"regex_compile_loop", "Regex::new", "cpu", "high", "Regex compilation in loop is very expensive",
			"compile regex once with lazy_static or once_cell"},
		{"hashmap_default", "HashMap::new", "memory", "low", "Default hasher may not be optimal",
			"use ahash or fxhash for better performance"},
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		for _, p := range rustPatterns {
			if strings.Contains(trimmed, p.pattern) {
				issues = append(issues, PerformanceIssue{
					Type:        p.pType,
					Severity:    p.severity,
					Line:        i + 1,
					Description: fmt.Sprintf("Performance issue: %s", p.name),
					Impact:      p.impact,
					Fix:         p.fix,
				})
			}
		}
	}
	return issues
}

func rustProfilingTools() []ProfilingTool {
	return []ProfilingTool{
		{Name: "cargo-flamegraph", Purpose: "CPU flame graph", Language: "rust", Command: "cargo install flamegraph && cargo flamegraph"},
		{Name: "perf", Purpose: "CPU profiling (Linux)", Language: "rust", Command: "perf record -g -- target/release/binary && perf report"},
		{Name: "valgrind", Purpose: "Memory leak detection", Language: "rust", Command: "valgrind --leak-check=full ./target/release/binary"},
		{Name: "criterion", Purpose: "Micro-benchmarking", Language: "rust", Command: "cargo bench"},
		{Name: "dhat", Purpose: "Heap profiling", Language: "rust", Command: "DHAT=1 ./target/release/binary"},
		{Name: "tracing", Purpose: "Structured profiling", Language: "rust", Command: "cargo install tracing-subscriber"},
	}
}

// ── Python Performance Scanning ──

func scanPythonPerformance(lines []string) []PerformanceIssue {
	var issues []PerformanceIssue
	pyPatterns := []struct {
		name     string
		pattern  string
		pType    string
		severity string
		impact   string
		fix      string
	}{
		{"list_comprehension", "for ", "cpu", "low", "Loop could be a list comprehension",
			"use list comprehension or map/filter for better performance"},
		{"string_concat_loop", "+= ", "cpu", "medium", "String concatenation in loop is O(n²)",
			"use ''.join() or io.StringIO"},
		{"global_variable", "global ", "cpu", "low", "Global variable access is slower than local",
			"pass as parameter or use local variable"},
		{"import_in_loop", "import ", "cpu", "medium", "Import inside loop is expensive",
			"move imports to top of file"},
		{"json_loads_loop", "json.loads", "cpu", "medium", "JSON parsing in loop",
			"batch processing or use orjson/ujson"},
		{"pandas_chain", ".apply(", "cpu", "medium", "Row-wise apply is slow",
			"use vectorized operations with pandas/numpy"},
		{"sqlite_no_index", "SELECT", "io", "high", "Full table scan without index",
			"add database indexes for frequently queried columns"},
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		for _, p := range pyPatterns {
			if strings.Contains(trimmed, p.pattern) {
				issues = append(issues, PerformanceIssue{
					Type:        p.pType,
					Severity:    p.severity,
					Line:        i + 1,
					Description: fmt.Sprintf("Performance issue: %s", p.name),
					Impact:      p.impact,
					Fix:         p.fix,
				})
			}
		}
	}
	return issues
}

func pythonProfilingTools() []ProfilingTool {
	return []ProfilingTool{
		{Name: "cProfile", Purpose: "CPU profiling", Language: "python", Command: "python -m cProfile -s cumtime script.py"},
		{Name: "line_profiler", Purpose: "Line-by-line CPU profiling", Language: "python", Command: "kernprof -l -v script.py"},
		{Name: "memory_profiler", Purpose: "Memory profiling", Language: "python", Command: "python -m memory_profiler script.py"},
		{Name: "py-spy", Purpose: "Sampling profiler", Language: "python", Command: "py-spy record -o profile.svg -- python script.py"},
		{Name: "tracemalloc", Purpose: "Memory allocation tracking", Language: "python", Command: "python -m tracemalloc -o trace.log script.py"},
		{Name: "scalene", Purpose: "CPU+Memory profiler", Language: "python", Command: "scalene script.py"},
	}
}

// ── Shell Performance Scanning ──

func scanShellPerformance(lines []string) []PerformanceIssue {
	var issues []PerformanceIssue
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// cat in loop
		if strings.Contains(trimmed, "cat ") && strings.Contains(trimmed, "$(") {
			issues = append(issues, PerformanceIssue{
				Type: "io", Severity: "medium", Line: i + 1,
				Description: "Performance issue: cat with command substitution",
				Impact: "Spawns subprocess per iteration",
				Fix: "Use built-in parameter expansion or read with while loop",
			})
		}
		// grep without -q
		if strings.Contains(trimmed, "grep ") && !strings.Contains(trimmed, "-q ") && strings.Contains(trimmed, "if ") {
			issues = append(issues, PerformanceIssue{
				Type: "io", Severity: "low", Line: i + 1,
				Description: "Performance issue: grep without -q flag",
				Impact: "Output goes to stdout even when only checking exit code",
				Fix: "Add -q flag: grep -q pattern file",
			})
		}
		// find in loop
		if strings.Contains(trimmed, "find ") && (strings.Contains(trimmed, "for ") || strings.Contains(trimmed, "while ")) {
			issues = append(issues, PerformanceIssue{
				Type: "io", Severity: "high", Line: i + 1,
				Description: "Performance issue: find in loop",
				Impact: "Recursive directory traversal per iteration",
				Fix: "Use find -exec or xargs for batch processing",
			})
		}
	}
	return issues
}

func shellProfilingTools() []ProfilingTool {
	return []ProfilingTool{
		{Name: "time", Purpose: "Execution time", Language: "shell", Command: "time ./script.sh"},
		{Name: "strace", Purpose: "System call tracing", Language: "shell", Command: "strace -c ./script.sh"},
		{Name: "shellcheck", Purpose: "Performance hints", Language: "shell", Command: "shellcheck -S warning script.sh"},
	}
}

// ── C/C++ Performance Scanning ──

func scanCppPerformance(lines []string) []PerformanceIssue {
	var issues []PerformanceIssue
	cppPatterns := []struct {
		name     string
		pattern  string
		pType    string
		severity string
		impact   string
		fix      string
	}{
		{"raw_new", "new ", "memory", "high", "Manual memory management risk",
			"use smart pointers (unique_ptr, shared_ptr)"},
		{"copy_in_loop", "push_back", "cpu", "medium", "Object copy in push_back",
			"use emplace_back or move semantics"},
		{"string_copy", "std::string", "memory", "medium", "String copy overhead",
			"use std::string_view for read-only access"},
		{"vector_reserve", "vector<", "memory", "low", "Vector may reallocate repeatedly",
			"use reserve() to pre-allocate"},
		{"iostream_sync", "std::cout", "cpu", "low", "iostream sync with C stdio",
			"add ios_base::sync_with_stdio(false)"},
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		for _, p := range cppPatterns {
			if strings.Contains(trimmed, p.pattern) {
				issues = append(issues, PerformanceIssue{
					Type:        p.pType,
					Severity:    p.severity,
					Line:        i + 1,
					Description: fmt.Sprintf("Performance issue: %s", p.name),
					Impact:      p.impact,
					Fix:         p.fix,
				})
			}
		}
	}
	return issues
}

func cppProfilingTools() []ProfilingTool {
	return []ProfilingTool{
		{Name: "perf", Purpose: "CPU profiling (Linux)", Language: "c++", Command: "perf record -g -- ./binary && perf report"},
		{Name: "valgrind", Purpose: "Memory leak detection", Language: "c++", Command: "valgrind --leak-check=full ./binary"},
		{Name: "gprof", Purpose: "Function-level profiling", Language: "c++", Command: "g++ -pg -o binary source.cpp && ./binary && gprof binary gmon.out"},
		{Name: "gperftools", Purpose: "CPU + heap profiling", Language: "c++", Command: "LD_PRELOAD=/usr/lib/libprofiler.so CPUPROFILE=prof.out ./binary"},
		{Name: "AddressSanitizer", Purpose: "Memory error detection", Language: "c++", Command: "g++ -fsanitize=address -g source.cpp"},
		{Name: "Infer", Purpose: "Static analysis", Language: "c++", Command: "infer run -- make"},
	}
}

// ── JavaScript/TypeScript Performance Scanning ──

func scanJSPerformance(lines []string) []PerformanceIssue {
	var issues []PerformanceIssue
	jsPatterns := []struct {
		name     string
		pattern  string
		pType    string
		severity string
		impact   string
		fix      string
	}{
		{"dom_query_loop", "querySelector", "cpu", "medium", "DOM query in loop is expensive",
			"cache query results, use DocumentFragment"},
		{"string_concat_loop", "+= ", "cpu", "medium", "String concatenation in loop",
			"use array.join() or template literals"},
		{"json_parse_loop", "JSON.parse", "cpu", "medium", "JSON parsing in loop",
			"batch processing, use streaming parser"},
		{"sync_xhr", "XMLHttpRequest", "io", "high", "Synchronous XHR blocks UI",
			"use fetch() with async/await"},
		{"memory_leak_closure", "function(", "memory", "medium", "Closure may cause memory leak",
			"use WeakRef, WeakMap, or cleanup event listeners"},
		{"array_push_spread", "...spread", "cpu", "low", "Spread operator copies entire array",
			"use push() with individual items for large arrays"},
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		for _, p := range jsPatterns {
			if strings.Contains(trimmed, p.pattern) {
				issues = append(issues, PerformanceIssue{
					Type:        p.pType,
					Severity:    p.severity,
					Line:        i + 1,
					Description: fmt.Sprintf("Performance issue: %s", p.name),
					Impact:      p.impact,
					Fix:         p.fix,
				})
			}
		}
	}
	return issues
}

func jsProfilingTools() []ProfilingTool {
	return []ProfilingTool{
		{Name: "Chrome DevTools", Purpose: "CPU + Memory profiling", Language: "javascript", Command: "Open DevTools → Performance tab → Record"},
		{Name: "node --prof", Purpose: "V8 CPU profiling", Language: "javascript", Command: "node --prof script.js && node --prof-process isolate-*.log"},
		{Name: "clinic.js", Purpose: "Node.js diagnostics", Language: "javascript", Command: "npx clinic doctor -- node script.js"},
		{Name: "0x", Purpose: "Flame graph for Node.js", Language: "javascript", Command: "npx 0x node script.js"},
		{Name: "memwatch-next", Purpose: "Memory leak detection", Language: "javascript", Command: "npm install memwatch-next && require('memwatch-next').on('leak', ...)"},
	}
}

// ── Suggestions Generation ──

func generateProfilingSuggestions(r ProfilingResult) []string {
	suggestions := make([]string, 0)

	hasCPU := false
	hasMemory := false
	hasIO := false
	for _, h := range r.Hotspots {
		switch h.Type {
		case "cpu":
			hasCPU = true
		case "memory":
			hasMemory = true
		case "io", "algorithm":
			hasIO = true
		}
	}

	if hasCPU {
		suggestions = append(suggestions, "Run CPU profiling to identify the hottest functions")
		suggestions = append(suggestions, "Consider algorithmic improvements before micro-optimization")
	}
	if hasMemory {
		suggestions = append(suggestions, "Run memory profiling to find allocation hotspots")
		suggestions = append(suggestions, "Consider object pooling for frequently allocated objects")
	}
	if hasIO {
		suggestions = append(suggestions, "Run I/O profiling to identify blocking operations")
		suggestions = append(suggestions, "Consider async I/O or batching for better throughput")
	}

	if len(r.Hotspots) == 0 {
		suggestions = append(suggestions, "No obvious performance issues detected. Run profiling to establish baseline.")
		suggestions = append(suggestions, "Focus on: algorithmic complexity (Big-O), memory allocation patterns, I/O blocking")
	}

	return suggestions
}

func getSampleProfilingCommand(language, target string) string {
	switch language {
	case "go":
		if target == "memory" {
			return `import _ "net/http/pprof"; go http.ListenAndServe(":6060", nil); // then: go tool pprof http://localhost:6060/debug/pprof/heap`
		}
		return `import _ "net/http/pprof"; go http.ListenAndServe(":6060", nil); // then: go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30`
	case "rust":
		return "cargo install flamegraph && cargo flamegraph"
	case "python":
		if target == "memory" {
			return "python -m memory_profiler your_script.py"
		}
		return "python -m cProfile -s cumtime your_script.py"
	case "shell":
		return "strace -c -e trace=file ./your_script.sh"
	case "c++", "c":
		return "perf record -g -- ./your_binary && perf report"
	case "typescript", "javascript":
		return "node --prof your_script.js && node --prof-process isolate-*.log"
	default:
		return "# No profiling command available for this language"
	}
}

func formatProfilingReport(r ProfilingResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Performance Profiling Report: %s\n", r.FilePath))
	sb.WriteString(fmt.Sprintf("Language: %s | Target: %s | Score: %d/100\n\n", r.Language, r.Target, r.Score))

	if len(r.Hotspots) > 0 {
		sb.WriteString(fmt.Sprintf("🔥 Hotspots Found: %d\n", len(r.Hotspots)))
		for _, h := range r.Hotspots {
			icon := "🟡"
			if h.Severity == "high" {
				icon = "🔴"
			} else if h.Severity == "low" {
				icon = "🟢"
			}
			sb.WriteString(fmt.Sprintf("  %s [%s] Line %d: %s\n", icon, strings.ToUpper(h.Type), h.Line, h.Description))
			sb.WriteString(fmt.Sprintf("     Impact: %s\n", h.Impact))
			sb.WriteString(fmt.Sprintf("     Fix: %s\n", h.Fix))
		}
	} else {
		sb.WriteString("✅ No obvious performance issues detected.\n")
	}

	if len(r.Tools) > 0 {
		sb.WriteString("\n🛠️ Profiling Tools:\n")
		for _, t := range r.Tools {
			sb.WriteString(fmt.Sprintf("  %s (%s)\n", t.Name, t.Purpose))
			sb.WriteString(fmt.Sprintf("    Command: %s\n", t.Command))
		}
	}

	if len(r.Suggestions) > 0 {
		sb.WriteString("\n💡 Suggestions:\n")
		for _, s := range r.Suggestions {
			sb.WriteString(fmt.Sprintf("  - %s\n", s))
		}
	}

	if r.SampleCommand != "" {
		sb.WriteString(fmt.Sprintf("\n📋 Quick Start:\n  %s\n", r.SampleCommand))
	}

	return sb.String()
}
