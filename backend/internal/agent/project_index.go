package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ProjectIndex holds a scan summary of the project directory.
type ProjectIndex struct {
	Root             string
	TotalFiles       int
	Dirs             int
	ByExt            map[string]int      // extension → count
	SourceFiles      []string            // key source file paths (up to 50)
	GoFunctions      map[string][]string // file → function names (Go only)
	GoTypes          map[string][]string // file → type definitions (struct/interface/type)
	GoImports        map[string][]string // file → import paths
	FileFingerprints map[string]string   // file → structural fingerprint (~200 chars)
	FileTree         []string            // all relative file paths
	DepGraph         map[string][]string // file → files it depends on (imports)
}

// IndexProject scans projectDir and returns a structured index.
// Skips hidden dirs, node_modules, vendor, __pycache__, .git.
func IndexProject(projectDir string) *ProjectIndex {
	idx := &ProjectIndex{
		Root:             projectDir,
		ByExt:            make(map[string]int),
		GoFunctions:      make(map[string][]string),
		GoTypes:          make(map[string][]string),
		GoImports:        make(map[string][]string),
		FileFingerprints: make(map[string]string),
		FileTree:         make([]string, 0, 200),
		DepGraph:         make(map[string][]string),
	}

	// Skip directories that are never useful for context
	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "vendor": true,
		"__pycache__": true, ".venv": true, "venv": true,
		".idea": true, ".vscode": true, "dist": true,
		"build": true, "target": true,
	}

	err := filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable files
		}
		if info.IsDir() {
			name := info.Name()
			if name != projectDir && (strings.HasPrefix(name, ".") || skipDirs[name]) {
				return filepath.SkipDir
			}
			idx.Dirs++
			return nil
		}
		idx.TotalFiles++
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			ext = "(no ext)"
		}
		idx.ByExt[ext]++

		relPath, _ := filepath.Rel(projectDir, path)
		idx.FileTree = append(idx.FileTree, relPath)

		// Collect source files for context (exclude generated/binary)
		if isSourceFile(ext) && len(idx.SourceFiles) < 50 {
			idx.SourceFiles = append(idx.SourceFiles, relPath)
		}

		// Extract Go function names for smart file selection
		if ext == ".go" && len(idx.GoFunctions) < 200 {
			if funcs := extractGoFunctions(path); len(funcs) > 0 {
				idx.GoFunctions[relPath] = funcs
			}
		}
		// Extract Go type definitions (struct/interface/type)
		if ext == ".go" && len(idx.GoTypes) < 200 {
			if types := extractGoTypes(path); len(types) > 0 {
				idx.GoTypes[relPath] = types
			}
		}
		// Extract Go import paths
		if ext == ".go" && len(idx.GoImports) < 200 {
			if imports := extractGoImports(path); len(imports) > 0 {
				idx.GoImports[relPath] = imports
			}
		}
		// Generate structural fingerprint
		if isSourceFile(ext) && len(idx.FileFingerprints) < 200 {
			idx.FileFingerprints[relPath] = generateFingerprint(path, ext)
		}
		return nil
	})
	if err != nil {
		return idx
	}

	idx.BuildDependencyGraph()
	return idx
}

// Summary returns a concise project overview for injection into the system prompt.
func (idx *ProjectIndex) Summary() string {
	if idx == nil || idx.TotalFiles == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## PROJECT INDEX (auto-generated)\n")
	sb.WriteString(fmt.Sprintf("Root: %s\n", idx.Root))
	sb.WriteString(fmt.Sprintf("Files: %d total, %d directories\n\n", idx.TotalFiles, idx.Dirs))

	// Language breakdown (sorted by count descending)
	type extCount struct {
		Ext   string
		Count int
	}
	var sorted []extCount
	for ext, count := range idx.ByExt {
		sorted = append(sorted, extCount{ext, count})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Count > sorted[j].Count })

	sb.WriteString("### Languages / File Types\n")
	for _, ec := range sorted[:minInt(len(sorted), 10)] { // top 10
		sb.WriteString(fmt.Sprintf("- %s: %d files\n", ec.Ext, ec.Count))
	}

	// Source file listing
	if len(idx.SourceFiles) > 0 {
		sb.WriteString("\n### Key Source Files\n")
		for _, f := range idx.SourceFiles {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	return sb.String()
}

// extractGoFunctions extracts exported function and method names from a Go file.
// Uses simple regex-like string scanning (no AST dependency).
func extractGoFunctions(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)
	var funcs []string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Match: func FuncName( or func (r *Type) MethodName(
		if strings.HasPrefix(line, "func ") && !strings.HasPrefix(line, "func Test") {
			afterFunc := line[5:] // after "func "
			// Simple function: func Name(
			if idx := strings.IndexByte(afterFunc, '('); idx > 0 {
				name := strings.TrimSpace(afterFunc[:idx])
				if name != "" && len(name) < 80 {
					funcs = append(funcs, name)
				}
			}
		}
		if len(funcs) >= 30 {
			break
		}
	}
	return funcs
}

// extractGoTypes extracts type definitions (struct, interface, type alias) from a Go file.
func extractGoTypes(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)
	var types []string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Match: type XXX struct { ... }
		// Match: type XXX interface { ... }
		// Match: type XXX = ...
		if strings.HasPrefix(line, "type ") {
			afterType := line[5:]
			// Extract name: everything before the first space or '='
			var name string
			if idx := strings.IndexByte(afterType, ' '); idx > 0 {
				name = afterType[:idx]
			} else if idx := strings.IndexByte(afterType, '='); idx > 0 {
				name = afterType[:idx]
			} else {
				name = afterType
			}
			name = strings.TrimSpace(name)
			if name != "" && len(name) < 80 {
				types = append(types, name)
			}
		}
		if len(types) >= 30 {
			break
		}
	}
	return types
}

// extractGoImports extracts import paths from a Go file.
func extractGoImports(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)
	var imports []string

	// Single-line import: import "path"
	lines := strings.Split(content, "\n")
	inImportBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Single-line import
		if strings.HasPrefix(trimmed, "import ") && !strings.HasPrefix(trimmed, "import (") {
			if path := extractImportPath(trimmed[7:]); path != "" {
				imports = append(imports, path)
			}
			continue
		}

		// Start of import block
		if trimmed == "import (" {
			inImportBlock = true
			continue
		}
		// End of import block
		if inImportBlock && trimmed == ")" {
			inImportBlock = false
			continue
		}
		// Inside import block
		if inImportBlock && trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			if path := extractImportPath(trimmed); path != "" {
				imports = append(imports, path)
			}
		}
	}
	return imports
}

// extractImportPath pulls the quoted path from an import line like `alias "path"`.
func extractImportPath(s string) string {
	s = strings.TrimSpace(s)
	start := strings.IndexByte(s, '"')
	if start < 0 {
		return ""
	}
	end := strings.IndexByte(s[start+1:], '"')
	if end < 0 {
		return ""
	}
	return s[start+1 : start+1+end]
}

// generateFingerprint creates a structural summary of a file (~200 chars).
func generateFingerprint(path, ext string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := string(data)
	lines := strings.Split(content, "\n")

	// Count structural elements
	funcCount := 0
	typeCount := 0
	importCount := 0
	commentLines := 0
	totalLines := len(lines)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "func ") {
			funcCount++
		}
		if strings.HasPrefix(trimmed, "type ") {
			typeCount++
		}
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "\"") {
			importCount++
		}
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			commentLines++
		}
	}

	// Determine language from extension
	lang := "unknown"
	switch ext {
	case ".go":
		lang = "Go"
	case ".js", ".mjs":
		lang = "JavaScript"
	case ".ts", ".tsx":
		lang = "TypeScript"
	case ".py":
		lang = "Python"
	case ".rs":
		lang = "Rust"
	case ".java":
		lang = "Java"
	case ".yaml", ".yml":
		lang = "YAML"
	case ".json":
		lang = "JSON"
	case ".md":
		lang = "Markdown"
	case ".html", ".htm":
		lang = "HTML"
	case ".css":
		lang = "CSS"
	case ".sh":
		lang = "Shell"
	}

	// Extract first meaningful comment or doc string
	var firstComment string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "// ") && len(trimmed) > 4 {
			firstComment = trimmed[3:]
			if len(firstComment) > 60 {
				firstComment = firstComment[:60]
			}
			break
		}
	}

	fingerprint := fmt.Sprintf("[%s] lines=%d funcs=%d types=%d imports=%d comments=%d",
		lang, totalLines, funcCount, typeCount, importCount, commentLines)
	if firstComment != "" {
		fingerprint += " doc=\"" + firstComment + "\""
	}
	return fingerprint
}

// BuildDependencyGraph builds a file → dependencies mapping from GoImports.
// Import paths are resolved to project-relative absolute paths.
func (idx *ProjectIndex) BuildDependencyGraph() {
	if idx.GoImports == nil {
		return
	}

	// Build a map from package paths to source files
	// e.g., "internal/agent" → "backend/internal/agent/project_index.go"
	pkgToFile := map[string][]string{}
	for file := range idx.GoFunctions {
		dir := filepath.Dir(file)
		pkgToFile[dir] = append(pkgToFile[dir], file)
	}
	// Also include GoTypes files (they may not be in GoFunctions)
	for file := range idx.GoTypes {
		dir := filepath.Dir(file)
		pkgToFile[dir] = append(pkgToFile[dir], file)
	}

	for file, imports := range idx.GoImports {
		var deps []string
		seen := map[string]bool{}
		for _, imp := range imports {
			// Skip stdlib (no dot in first path element)
			parts := strings.Split(imp, "/")
			if len(parts) == 0 || !strings.Contains(parts[0], ".") {
				continue
			}
			// Try to find files in the project that belong to this package
			// Strip the module prefix (assumes module name from root)
			// e.g., "github.com/user/project/internal/agent" → "internal/agent"
			for pkgDir, files := range pkgToFile {
				// Match if the import path ends with the local package dir
				if strings.HasSuffix(imp, pkgDir) || strings.HasSuffix(imp, "/"+pkgDir) {
					for _, f := range files {
						if f != file && !seen[f] {
							seen[f] = true
							deps = append(deps, f)
						}
					}
				}
			}
		}
		if len(deps) > 0 {
			idx.DepGraph[file] = deps
		}
	}
}
