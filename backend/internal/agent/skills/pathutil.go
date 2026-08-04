package skills

import (
	"database/sql"
	"path/filepath"
	"strings"
)

// ResolveProjectPath resolves the actual filesystem path for a project.
// Checks the database for storage_path, falls back to projectPath/projectID.
func ResolveProjectPath(db *sql.DB, projectPath, projectID string) string {
	if projectID == "" {
		return projectPath
	}
	if db != nil {
		var storagePath string
		err := db.QueryRow(`SELECT COALESCE(storage_path,'') FROM projects WHERE id=?`, projectID).Scan(&storagePath)
		if err == nil && storagePath != "" {
			return storagePath
		}
	}
	return filepath.Join(projectPath, projectID)
}

// isPathWithin reports whether path is equal to base or nested inside it.
// Replaces the deprecated filepath.HasPrefix, which does not respect path
// boundaries (e.g. "/foo/bar" would falsely match base "/foo/b").
func isPathWithin(base, path string) bool {
	base = filepath.Clean(base)
	path = filepath.Clean(path)
	if path == base {
		return true
	}
	return strings.HasPrefix(path, base+string(filepath.Separator))
}

// detectLanguage detects the programming language of a file based on its extension.
func detectLanguage(path string) string {
	path = strings.ToLower(path)

	// Shell scripts
	if strings.HasSuffix(path, ".sh") || strings.HasSuffix(path, ".bash") || strings.HasSuffix(path, ".zsh") {
		return "shell"
	}
	// Rust
	if strings.HasSuffix(path, ".rs") {
		return "rust"
	}
	// Go
	if strings.HasSuffix(path, ".go") {
		return "go"
	}
	// Python
	if strings.HasSuffix(path, ".py") || strings.HasSuffix(path, ".pyw") {
		return "python"
	}
	// JavaScript/TypeScript
	if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".jsx") || strings.HasSuffix(path, ".mjs") {
		return "javascript"
	}
	if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx") {
		return "typescript"
	}
	// C/C++
	if strings.HasSuffix(path, ".c") || strings.HasSuffix(path, ".h") {
		return "c"
	}
	if strings.HasSuffix(path, ".cpp") || strings.HasSuffix(path, ".cc") || strings.HasSuffix(path, ".cxx") || strings.HasSuffix(path, ".hpp") || strings.HasSuffix(path, ".hxx") {
		return "cpp"
	}
	// Web
	if strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".htm") {
		return "html"
	}
	if strings.HasSuffix(path, ".css") || strings.HasSuffix(path, ".scss") || strings.HasSuffix(path, ".less") {
		return "css"
	}
	// Data/Config
	if strings.HasSuffix(path, ".json") {
		return "json"
	}
	if strings.HasSuffix(path, ".xml") {
		return "xml"
	}
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		return "yaml"
	}
	if strings.HasSuffix(path, ".toml") {
		return "toml"
	}
	// Android-specific
	if strings.HasSuffix(path, ".prop") || strings.HasSuffix(path, ".properties") {
		return "properties"
	}
	if strings.HasSuffix(path, ".kt") || strings.HasSuffix(path, ".kts") {
		return "kotlin"
	}
	if strings.HasSuffix(path, ".java") {
		return "java"
	}
	// Markdown
	if strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".markdown") {
		return "markdown"
	}
	// Dockerfile
	if strings.Contains(path, "dockerfile") || strings.HasSuffix(path, ".dockerfile") {
		return "dockerfile"
	}
	// Makefile
	if strings.Contains(path, "makefile") || strings.HasSuffix(path, ".mk") {
		return "makefile"
	}

	return "unknown"
}

// detectModuleType detects the module type from module.prop content.
func detectModuleType(propContent string) string {
	lower := strings.ToLower(propContent)
	if strings.Contains(lower, "kernelsu") || strings.Contains(lower, "ksu") {
		return "kernelsu"
	}
	if strings.Contains(lower, "apatch") {
		return "apatch"
	}
	if strings.Contains(lower, "magisk") {
		return "magisk"
	}
	return "universal"
}
