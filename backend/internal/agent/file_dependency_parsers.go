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

// resolveImport resolves an import path relative to the importing file.
func (dg *FileDependencyGraph) resolveImport(fromFile, importPath string) string {
	// Handle relative imports
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		fromDir := filepath.Dir(fromFile)
		resolved := filepath.Join(fromDir, importPath)
		return filepath.Clean(resolved)
	}
	
	// For external imports, return the import path as-is
	return importPath
}
