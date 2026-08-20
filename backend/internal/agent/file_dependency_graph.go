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

func (dg *FileDependencyGraph) GetDependencies(path string) []string {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	if node, ok := dg.nodes[path]; ok {
		return node.Imports
	}
	return nil
}

// GetDependents returns the files that depend on a given file.
func (dg *FileDependencyGraph) GetDependents(path string) []string {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	if node, ok := dg.nodes[path]; ok {
		return node.ImportedBy
	}
	return nil
}

// GetImpact returns all files that would be affected if a given file changes.
// This is a transitive closure of GetDependents.
func (dg *FileDependencyGraph) GetImpact(path string) []string {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	visited := make(map[string]bool)
	var result []string

	var dfs func(p string)
	dfs = func(p string) {
		if visited[p] {
			return
		}
		visited[p] = true
		if node, ok := dg.nodes[p]; ok {
			for _, dep := range node.ImportedBy {
				if !visited[dep] {
					result = append(result, dep)
					dfs(dep)
				}
			}
		}
	}

	dfs(path)
	return result
}

// GetSmartReadOrder returns files in optimal reading order for understanding a module.
// Priority: entry points (main, index) > high-fan-in files > other files.
func (dg *FileDependencyGraph) GetSmartReadOrder() []string {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	type fileScore struct {
		path  string
		score int
	}

	scores := make([]fileScore, 0, len(dg.nodes))
	for path, node := range dg.nodes {
		score := 0

		// Entry points get highest priority
		baseName := filepath.Base(path)
		if baseName == "main.go" || baseName == "main.rs" || baseName == "index.ts" ||
			baseName == "index.js" || baseName == "lib.rs" || baseName == "mod.rs" {
			score += 100
		}

		// High fan-in files (imported by many) are important
		score += len(node.ImportedBy) * 10

		// Non-test files are more important than test files
		if !node.IsTest {
			score += 5
		}

		// Smaller files are easier to read first
		if node.Size < 1000 {
			score += 3
		} else if node.Size > 10000 {
			score -= 2
		}

		scores = append(scores, fileScore{path: path, score: score})
	}

	// Sort by score descending
	for i := 1; i < len(scores); i++ {
		for j := i; j > 0 && scores[j].score > scores[j-1].score; j-- {
			scores[j], scores[j-1] = scores[j-1], scores[j]
		}
	}

	result := make([]string, len(scores))
	for i, s := range scores {
		result[i] = s.path
	}
	return result
}

// DetectCircularDependencies finds cycles in the dependency graph.
func (dg *FileDependencyGraph) DetectCircularDependencies() [][]string {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	visited := make(map[string]int) // 0=unvisited, 1=in-progress, 2=done
	var cycles [][]string

	var dfs func(path string, stack []string) []string
	dfs = func(path string, stack []string) []string {
		visited[path] = 1
		stack = append(stack, path)

		if node, ok := dg.nodes[path]; ok {
			for _, imp := range node.Imports {
				if visited[imp] == 1 {
					// Found cycle ?extract cycle path
					cycleStart := -1
					for i, s := range stack {
						if s == imp {
							cycleStart = i
							break
						}
					}
					if cycleStart >= 0 {
						cycle := make([]string, len(stack)-cycleStart)
						copy(cycle, stack[cycleStart:])
						return cycle
					}
				}
				if visited[imp] == 0 {
					if cycle := dfs(imp, stack); cycle != nil {
						return cycle
					}
				}
			}
		}

		visited[path] = 2
		return nil
	}

	for path := range dg.nodes {
		if visited[path] == 0 {
			if cycle := dfs(path, nil); cycle != nil {
				cycles = append(cycles, cycle)
			}
		}
	}

	return cycles
}

// GetStats returns summary statistics about the dependency graph.
func (dg *FileDependencyGraph) GetStats() map[string]interface{} {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_files"] = len(dg.nodes)
	stats["last_build"] = dg.lastBuild

	// Count by language
	langCounts := make(map[string]int)
	testCount := 0
	totalImports := 0
	for _, node := range dg.nodes {
		langCounts[node.Language]++
		if node.IsTest {
			testCount++
		}
		totalImports += len(node.Imports)
	}
	stats["by_language"] = langCounts
	stats["test_files"] = testCount
	stats["total_imports"] = totalImports

	// Find files with most dependents (highest impact)
	type impactFile struct {
		path       string
		dependents int
	}
	impacts := make([]impactFile, 0)
	for _, node := range dg.nodes {
		if len(node.ImportedBy) > 0 {
			impacts = append(impacts, impactFile{path: node.Path, dependents: len(node.ImportedBy)})
		}
	}
	// Sort top 5
	for i := 1; i < len(impacts); i++ {
		for j := i; j > 0 && impacts[j].dependents > impacts[j-1].dependents; j-- {
			impacts[j], impacts[j-1] = impacts[j-1], impacts[j]
		}
		if i >= 5 {
			impacts = impacts[:5]
			break
		}
	}
	stats["highest_impact_files"] = impacts

	// Detect circular dependencies
	cycles := dg.DetectCircularDependencies()
	stats["circular_dependencies"] = len(cycles)

	return stats
}

// UpdateFile incrementally updates the dependency graph for a single file.
func (dg *FileDependencyGraph) UpdateFile(path string) {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	// Remove old edges
	if oldNode, ok := dg.nodes[path]; ok {
		for _, imp := range oldNode.Imports {
			if depNode, ok := dg.nodes[imp]; ok {
				// Remove from ImportedBy
				newBy := make([]string, 0, len(depNode.ImportedBy))
				for _, by := range depNode.ImportedBy {
					if by != path {
						newBy = append(newBy, by)
					}
				}
				depNode.ImportedBy = newBy
			}
		}
	}

	// Re-parse
	node := dg.getOrCreateNode(path)
	node.Imports = dg.parseImports(path)
	node.LastScanned = time.Now()

	// Update reverse dependencies
	for _, imp := range node.Imports {
		if depNode, ok := dg.nodes[imp]; ok {
			depNode.ImportedBy = append(depNode.ImportedBy, path)
		}
	}
}

// isSourceFile returns true if the file extension is a known source file.
func isSourceFile(ext string) bool {
	switch ext {
	case ".go", ".rs", ".js", ".mjs", ".ts", ".mts", ".jsx", ".tsx",
		".py", ".java", ".kt", ".cpp", ".c", ".h", ".hpp",
		".swift", ".rb", ".php", ".lua", ".zig":
		return true
	}
	return false
}
