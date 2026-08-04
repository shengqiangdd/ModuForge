package builder

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ModuleExcludePatterns defines directories and files to exclude from module zip.
// Source code, build artifacts, and development files are excluded — only
// runtime files (compiled binaries, shell scripts, module metadata) are included.
var ModuleExcludePatterns = []string{
	// Source code
	"src/",
	"*.go",
	"*.rs",
	"*.c",
	"*.h",
	"*.cpp",
	"*.py",
	"*.java",
	"*.kt",
	// Build system (note: do NOT exclude "bin/" — system/bin/ contains compiled binaries)
	"build.sh",
	"compile.sh",
	"build_module.sh",
	"build_binary.sh",
	"Makefile",
	"makefile",
	"go.mod",
	"go.sum",
	"Cargo.toml",
	"Cargo.lock",
	"target/",
	// Build cache & artifacts
	".build_cache/",
	".build_cache.json",
	"*.o",
	"*.a",
	"*.so",
	"*.dylib",
	"*.class",
	"*.jar",
	"*.apk",
	"*.dex",
	// IDE & editor
	".idea/",
	".vscode/",
	".vs/",
	"*.iml",
	"*.ipr",
	"*.iws",
	".project",
	".classpath",
	"*.swp",
	"*.swo",
	"*~",
	// OS files
	".DS_Store",
	"Thumbs.db",
	"desktop.ini",
	// Version control
	".git/",
	".gitignore",
	".gitattributes",
	".svn/",
	".hg/",
	// Documentation (only in root, not in data/)
	"README*",
	"LICENSE*",
	"CHANGELOG*",
	"CONTRIBUTING*",
	"*.md",
	"docs/",
	"doc/",
	// Test
	"test/",
	"tests/",
	"*_test.go",
	"*_test.rs",
	"*_test.cpp",
	"test_*",
	// Config files that shouldn't be in module
	".env",
	".env.*",
	"docker-compose*.yml",
	"Dockerfile*",
	"*.gradle",
}

// shouldExcludePath checks if a relative path matches any exclusion pattern.
func shouldExcludePath(relPath string) bool {
	lower := strings.ToLower(relPath)
	for _, pat := range ModuleExcludePatterns {
		if strings.HasSuffix(pat, "/") {
			// Directory pattern: path starts with it
			if strings.HasPrefix(lower, pat) {
				return true
			}
		} else if strings.Contains(pat, "*") {
			// Glob pattern: match filename suffix
			suffix := strings.TrimPrefix(pat, "*")
			if strings.HasSuffix(lower, suffix) {
				return true
			}
		} else {
			// Exact filename match
			if lower == strings.ToLower(pat) || strings.HasSuffix(lower, "/"+strings.ToLower(pat)) {
				return true
			}
		}
	}
	return false
}

// ZipModuleForBuild zips a module directory for build output,
// excluding source code and build artifacts — only runtime files are included.
func ZipModuleForBuild(sourceDir, outputZip string) error {
	return ZipDirExcluding(sourceDir, outputZip, ModuleExcludePatterns)
}

// ZipDir recursively zips a directory into a .zip file using Go standard library.
// No external `zip` binary needed.
func ZipDir(sourceDir, outputZip string) error {
	return ZipDirExcluding(sourceDir, outputZip, nil)
}

// ZipDirExcluding zips a directory, excluding paths that match any of the given patterns.
// If patterns is nil, everything is included (original behavior).
func ZipDirExcluding(sourceDir, outputZip string, excludePatterns []string) error {
	zipFile, err := os.Create(outputZip)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	// Ensure sourceDir is absolute for clean paths
	absSource, err := filepath.Abs(sourceDir)
	if err != nil {
		return fmt.Errorf("abs path: %w", err)
	}

	return filepath.Walk(absSource, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Build the relative name inside the zip
		relPath, err := filepath.Rel(absSource, path)
		if err != nil {
			return err
		}
		// Normalize to forward slashes for zip
		relPath = filepath.ToSlash(relPath)
		if relPath == "." || relPath == "" {
			return nil
		}

		// Check exclusion patterns
		if excludePatterns != nil {
			lower := strings.ToLower(relPath)
			for _, pat := range excludePatterns {
				if strings.HasSuffix(pat, "/") {
					// Directory pattern: path starts with it
					if strings.HasPrefix(lower, strings.ToLower(pat)) {
						if info.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
				} else if strings.Contains(pat, "*") {
					// Glob pattern: match filename
					suffix := strings.TrimPrefix(pat, "*")
					if strings.HasSuffix(lower, strings.ToLower(suffix)) {
						return nil
					}
				} else {
					// Exact filename match
					if lower == strings.ToLower(pat) || strings.HasSuffix(lower, "/"+strings.ToLower(pat)) {
						return nil
					}
				}
			}
		}

		// Create zip entry
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relPath
		if info.IsDir() {
			header.Name += "/"
		}

		writer, err := w.CreateHeader(header)
		if err != nil {
			return err
		}

		// Skip directories (already handled by name)
		if info.IsDir() {
			return nil
		}

		// Copy file content
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}
