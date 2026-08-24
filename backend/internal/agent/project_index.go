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
	Root        string
	TotalFiles  int
	Dirs        int
	ByExt       map[string]int      // extension → count
	SourceFiles []string            // key source file paths (up to 50)
	GoFunctions map[string][]string // file → function names (Go only)
	FileTree    []string            // all relative file paths
}

// IndexProject scans projectDir and returns a structured index.
// Skips hidden dirs, node_modules, vendor, __pycache__, .git.
func IndexProject(projectDir string) *ProjectIndex {
	idx := &ProjectIndex{
		Root:        projectDir,
		ByExt:       make(map[string]int),
		GoFunctions: make(map[string][]string),
		FileTree:    make([]string, 0, 200),
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
		return nil
	})
	if err != nil {
		return idx
	}

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
