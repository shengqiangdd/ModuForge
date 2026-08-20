package agent

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════
// File Dependency Graph — Track file dependencies for smart reads
// Inspired by: Rust's cargo dependency graph, TypeScript's project refs
//
// Builds an in-memory directed graph of file dependencies:
// - Go: import statements
// - Rust: use/mod statements
// - JS/TS: import/require statements
// - Generic: grep for cross-file references
//
// Enables:
// - Smart file selection (read most impactful files first)
// - Incremental builds (only rebuild affected files)
// - Impact analysis (what breaks when a file changes)
// - Circular dependency detection
// ══════════════════════════════════════════════════════════════════?
// DependencyNode represents a single file in the dependency graph.
type FileDependencyNode struct {
	Path        string   // file path (project-relative)
	Imports     []string // files this node imports/depends on
	ImportedBy  []string // files that import this node
	LastScanned time.Time
	IsTest      bool     // is this a test file?
	Language    string   // "go", "rust", "js", "ts", "python", etc.
	Size        int64    // file size in bytes
}

// DependencyGraph tracks file dependencies for a project.
type FileDependencyGraph struct {
	projectPath string
	nodes       map[string]*FileDependencyNode // path -> node
	mu          sync.RWMutex
	lastBuild   time.Time
	fileIndex   map[string]bool // all known files
}

// NewFileDependencyGraph creates a new file dependency graph for a project.
func NewFileDependencyGraph(projectPath string) *FileDependencyGraph {
	return &FileDependencyGraph{
		projectPath: projectPath,
		nodes:       make(map[string]*FileDependencyNode),
		fileIndex:   make(map[string]bool),
	}
}

// ══════════════════════════════════════════════════════════════════?// Scanning ?Build the dependency graph from source files
// ══════════════════════════════════════════════════════════════════?
// BuildGraph scans all source files and builds the dependency graph.
// This is an O(n * k) operation where n = number of files, k = avg lines per file.
func (dg *FileDependencyGraph) BuildGraph() error {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	start := time.Now()

	// Step 1: Index all source files
	err := filepath.Walk(dg.projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			// Skip common non-source directories
			name := info.Name()
			if name == "node_modules" || name == ".git" || name == "vendor" ||
				name == "target" || name == "dist" || name == "build" ||
				name == ".mf_backups" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if isSourceFile(ext) {
			relPath, _ := filepath.Rel(dg.projectPath, path)
			dg.fileIndex[relPath] = true
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk project: %v", err)
	}

	// Step 2: Parse dependencies for each file
	for relPath := range dg.fileIndex {
		node := dg.getOrCreateNode(relPath)
		node.Imports = dg.parseImports(relPath)
		node.LastScanned = time.Now()

		// Detect language
		ext := filepath.Ext(relPath)
		switch ext {
		case ".go":
			node.Language = "go"
		case ".rs":
			node.Language = "rust"
		case ".js", ".mjs":
			node.Language = "javascript"
		case ".ts", ".mts":
			node.Language = "typescript"
		case ".py":
			node.Language = "python"
		case ".java":
			node.Language = "java"
		case ".kt":
			node.Language = "kotlin"
		default:
			node.Language = "other"
		}

		// Detect test files
		baseName := filepath.Base(relPath)
		node.IsTest = strings.HasSuffix(baseName, "_test.go") ||
			strings.HasSuffix(baseName, "_test.rs") ||
			strings.HasSuffix(baseName, ".test.js") ||
			strings.HasSuffix(baseName, ".test.ts") ||
			strings.HasSuffix(baseName, ".spec.js") ||
			strings.HasSuffix(baseName, ".spec.ts") ||
			strings.Contains(baseName, "test_") ||
			strings.Contains(baseName, "_test.")

		// Get file size
		absPath := filepath.Join(dg.projectPath, relPath)
		if info, err := os.Stat(absPath); err == nil {
			node.Size = info.Size()
		}
	}

	// Step 3: Build reverse dependencies (ImportedBy)
	for _, node := range dg.nodes {
		for _, imp := range node.Imports {
			if depNode, ok := dg.nodes[imp]; ok {
				depNode.ImportedBy = append(depNode.ImportedBy, node.Path)
			}
		}
	}

	dg.lastBuild = time.Now()
	log.Printf("[DepGraph] Built dependency graph: %d files, %d nodes in %v",
		len(dg.fileIndex), len(dg.nodes), time.Since(start))
	return nil
}

// getOrCreateNode returns the node for a path, creating it if needed.
func (dg *FileDependencyGraph) getOrCreateNode(path string) *FileDependencyNode {
	if node, ok := dg.nodes[path]; ok {
		return node
	}
	node := &FileDependencyNode{
		Path:     path,
		Imports:  make([]string, 0),
		ImportedBy: make([]string, 0),
	}
	dg.nodes[path] = node
	return node
}

// parseImports extracts import statements from a source file.

func (dg *FileDependencyGraph) parseImports(relPath string) []string {
	absPath := filepath.Join(dg.projectPath, relPath)
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil
	}

	ext := filepath.Ext(relPath)
	text := string(content)
	imports := make([]string, 0)

	switch ext {
	case ".go":
		imports = dg.parseGoImports(text)
	case ".rs":
		imports = dg.parseRustImports(text)
	case ".js", ".mjs", ".ts", ".mts":
		imports = dg.parseJSImports(text)
	case ".py":
		imports = dg.parsePythonImports(text)
	}

	// Resolve to project-relative paths
	resolved := make([]string, 0, len(imports))
	for _, imp := range imports {
		if rel := dg.resolveImport(relPath, imp); rel != "" {
			resolved = append(resolved, rel)
		}
	}

	return resolved
}

// parseGoImports extracts Go import paths.
func (dg *FileDependencyGraph) parseGoImports(text string) []string {
	imports := make([]string, 0)
	lines := strings.Split(text, "\n")
	inImport := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "import (" {
			inImport = true
			continue
		}
		if inImport && trimmed == ")" {
			inImport = false
			continue
		}

		if inImport || strings.HasPrefix(trimmed, "import ") {
			// Extract import path
			start := strings.Index(trimmed, "\"")
			end := strings.LastIndex(trimmed, "\"")
			if start >= 0 && end > start {
				impPath := trimmed[start+1 : end]
				// Only track local imports (starting with . or ./ or ../)
				if strings.HasPrefix(impPath, ".") {
					imports = append(imports, impPath)
				}
			}
		}
	}
	return imports
}

// parseRustImports extracts Rust use/mod statements.
func (dg *FileDependencyGraph) parseRustImports(text string) []string {
	imports := make([]string, 0)
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip comments
		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		// mod statement: mod foo;
		if strings.HasPrefix(trimmed, "mod ") && strings.HasSuffix(trimmed, ";") {
			modName := strings.TrimPrefix(trimmed, "mod ")
			modName = strings.TrimSuffix(modName, ";")
			modName = strings.TrimSpace(modName)
			imports = append(imports, modName+".rs")
			continue
		}

		// use statement with self:: or super:: or crate::
		if strings.HasPrefix(trimmed, "use ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				usePath := strings.TrimSuffix(parts[1], ";")
				if strings.HasPrefix(usePath, "self::") || strings.HasPrefix(usePath, "super::") {
					imports = append(imports, usePath)
				}
			}
		}
	}
	return imports
}

// parseJSImports extracts JavaScript/TypeScript import statements.
func (dg *FileDependencyGraph) parseJSImports(text string) []string {
	imports := make([]string, 0)
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip comments
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		// import ... from 'path'
		if strings.Contains(trimmed, "from ") {
			start := strings.LastIndex(trimmed, "'")
			if start < 0 {
				start = strings.LastIndex(trimmed, "\"")
			}
			end := strings.LastIndex(trimmed, "'")
			if end < 0 {
				end = strings.LastIndex(trimmed, "\"")
			}
			if start >= 0 && end > start {
				impPath := trimmed[start+1 : end]
				if strings.HasPrefix(impPath, ".") {
					imports = append(imports, impPath)
				}
			}
		}

		// require('path')
		if strings.Contains(trimmed, "require(") {
			start := strings.Index(trimmed, "require('")
			if start < 0 {
				start = strings.Index(trimmed, "require(\"")
			}
			if start >= 0 {
				start = strings.Index(trimmed[start:], "'")
				if start < 0 {
					start = strings.Index(trimmed[start:], "\"")
				}
				end := strings.Index(trimmed[start+1:], "'")
				if end < 0 {
					end = strings.Index(trimmed[start+1:], "\"")
				}
				if start >= 0 && end >= 0 {
					impPath := trimmed[start+1 : start+1+end]
					if strings.HasPrefix(impPath, ".") {
						imports = append(imports, impPath)
					}
				}
			}
		}

		// import 'path' (side effect)
		if strings.HasPrefix(trimmed, "import ") && !strings.Contains(trimmed, "from ") {
			start := strings.Index(trimmed, "'")
			if start < 0 {
				start = strings.Index(trimmed, "\"")
			}
			end := strings.LastIndex(trimmed, "'")
			if end < 0 {
				end = strings.LastIndex(trimmed, "\"")
			}
			if start >= 0 && end > start {
				impPath := trimmed[start+1 : end]
				if strings.HasPrefix(impPath, ".") {
					imports = append(imports, impPath)
				}
			}
		}
	}
	return imports
}

// parsePythonImports extracts Python import statements.
func (dg *FileDependencyGraph) parsePythonImports(text string) []string {
	imports := make([]string, 0)
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip comments
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// from . import foo / from .. import bar
		if strings.HasPrefix(trimmed, "from ") && strings.Contains(trimmed, " import ") {
			parts := strings.SplitN(trimmed, " import ", 2)
			if len(parts) == 2 {
				modulePath := strings.TrimSpace(parts[0])
				modulePath = strings.TrimPrefix(modulePath, "from ")
				if strings.HasPrefix(modulePath, ".") {
					imports = append(imports, modulePath)
				}
			}
		}
	}
	return imports
}

// resolveImport resolves an import path to a project-relative file path.
func (dg *FileDependencyGraph) resolveImport(fromFile, importPath string) string {
	if !strings.HasPrefix(importPath, ".") {
		return "" // external import
	}

	dir := filepath.Dir(fromFile)
	resolved := filepath.Clean(filepath.Join(dir, importPath))

	// Try exact path
	for _, ext := range []string{".go", ".rs", ".js", ".ts", ".py", ".java", ".kt", ""} {
		candidate := resolved + ext
		if dg.fileIndex[candidate] {
			return candidate
		}
		// Try index files
		candidate = resolved + "/index" + ext
		if dg.fileIndex[candidate] {
			return candidate
		}
		// Try mod.rs for Rust
		if ext == ".rs" {
			candidate = resolved + "/mod.rs"
			if dg.fileIndex[candidate] {
				return candidate
			}
		}
	}

	return ""
}

// ══════════════════════════════════════════════════════════════════?// Query API ?Query the dependency graph
// ══════════════════════════════════════════════════════════════════?
// GetDependencies returns the files that a given file depends on.
