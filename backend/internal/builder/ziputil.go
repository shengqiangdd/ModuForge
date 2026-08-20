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
	".cargo-artifact-lock",
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
	// Shell/Editor temp & config files (e.g. .eslintrc, .shellcheckrc)
	".es*",
	".shell*",
	"*.bak",
	"*.tmp",
	"*.orig",
	// Empty / leftover component directories
	"native_component/",
	"rust_component/",
	// ModuForge platform assets (leaked into project storage)
	"assets/",
	"fonts/",
	// PWA files (ModuForge platform, not module)
	"sw.js",
	"manifest.json",
	"icon-*.png",
	"icon-*.svg",
	"icon.svg",
	// ModuForge build binary (should not be in module)
	"moduforge-build-*",
	"moduforge-build",
}

// ZipModuleForBuild zips a module directory for build output,
// excluding source code and build artifacts — only runtime files are included.
func ZipModuleForBuild(sourceDir, outputZip string) error {
	return ZipDirExcluding(sourceDir, outputZip, ModuleExcludePatterns)
}

// WrapWebroot rearranges a module zip so that webui files (HTML/CSS/JS)
// at the root level are moved into a webroot/ directory, which is required
// by KernelSU and APatch WebUI. Files already in webroot/ or in other
// subdirectories are left untouched.
//
// Standard Magisk/KernelSU/APatch module zip structure:
//
//	module.zip/
//	├── META-INF/com/google/android/update-binary
//	├── META-INF/com/google/android/updater-script
//	├── customize.sh
//	├── module.prop
//	├── service.sh
//	├── system/bin/...
//	└── webroot/          ← WebUI files go here (KernelSU/APatch)
//		├── index.html
//		├── styles.css
//		└── app.js
func WrapWebroot(zipPath string) error {
	// Patterns that identify webui files at root level
	webuiExts := map[string]bool{
		".html": true, ".htm": true,
		".css":  true,
		".js":   true,
		".json": true,
		".svg":  true,
		".png":  true, ".jpg": true, ".ico": true,
	}

	// Step 1: Read zip into memory and classify files
	type zipEntry struct {
		Name    string
		Content []byte
		Header  os.FileInfo
	}

	var webuiEntries []zipEntry
	var otherEntries []zipEntry

	// Track files already in webroot/ to avoid duplicates
	webrootFiles := make(map[string]bool)

	func() error {
		reader, err := zip.OpenReader(zipPath)
		if err != nil {
			return fmt.Errorf("open zip: %w", err)
		}
		defer reader.Close()

		// First pass: collect files already in webroot/
		for _, f := range reader.File {
			nameNorm := filepath.ToSlash(f.Name)
			if strings.HasPrefix(nameNorm, "webroot/") {
				webrootFiles[strings.TrimPrefix(nameNorm, "webroot/")] = true
			}
		}

		for _, f := range reader.File {
			nameNorm := filepath.ToSlash(f.Name)

			// Read file content
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("open entry %s: %w", f.Name, err)
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return fmt.Errorf("read entry %s: %w", f.Name, err)
			}

			fi := f.FileInfo()
			entry := zipEntry{Name: f.Name, Content: content, Header: fi}

			// Already in webroot/ — keep as is
			if strings.HasPrefix(nameNorm, "webroot/") {
				otherEntries = append(otherEntries, entry)
				continue
			}

			// Check if it's a root-level webui file
			base := filepath.Base(nameNorm)
			dir := filepath.ToSlash(filepath.Dir(nameNorm))
			ext := strings.ToLower(filepath.Ext(base))

			if (dir == "." || dir == "") && webuiExts[ext] {
				// Skip if this file already exists in webroot/ to avoid overwriting
				if webrootFiles[base] {
					otherEntries = append(otherEntries, entry)
					continue
				}
				webuiEntries = append(webuiEntries, entry)
			} else {
				otherEntries = append(otherEntries, entry)
			}
		}
		return nil
	}()

	// If no webui files to wrap, skip
	if len(webuiEntries) == 0 {
		return nil
	}

	// Step 2: Create new zip with wrapped webui files
	tmpZip := zipPath + ".tmp"
	outFile, err := os.Create(tmpZip)
	if err != nil {
		return fmt.Errorf("create temp zip: %w", err)
	}

	w := zip.NewWriter(outFile)

	// Copy non-webui files as-is
	for _, entry := range otherEntries {
		fw, err := w.Create(entry.Name)
		if err != nil {
			w.Close()
			outFile.Close()
			os.Remove(tmpZip)
			return fmt.Errorf("create entry: %w", err)
		}
		fw.Write(entry.Content)
	}

	// Add webui files under webroot/
	for _, entry := range webuiEntries {
		newName := "webroot/" + filepath.Base(entry.Name)
		fw, err := w.Create(newName)
		if err != nil {
			w.Close()
			outFile.Close()
			os.Remove(tmpZip)
			return fmt.Errorf("create webroot entry: %w", err)
		}
		fw.Write(entry.Content)
	}

	if err := w.Close(); err != nil {
		outFile.Close()
		os.Remove(tmpZip)
		return fmt.Errorf("close zip writer: %w", err)
	}
	outFile.Close()

	// Atomically replace original zip
	if err := os.Rename(tmpZip, zipPath); err != nil {
		os.Remove(tmpZip)
		return fmt.Errorf("rename temp zip: %w", err)
	}
	return nil
}

// ZipDir recursively zips a directory into a .zip file using Go standard library.
// No external `zip` binary needed.
func ZipDir(sourceDir, outputZip string) error {
	return ZipDirExcluding(sourceDir, outputZip, nil)
}

// ZipProgressFunc is called during zip creation to report progress.
// current/total represent files processed/total files to zip.
type ZipProgressFunc func(current, total int, currentFile string)

// ZipDirExcluding zips a directory, excluding paths that match any of the given patterns.
// If patterns is nil, everything is included (original behavior).
func ZipDirExcluding(sourceDir, outputZip string, excludePatterns []string) error {
	return ZipDirExcludingWithProgress(sourceDir, outputZip, excludePatterns, nil)
}

// ZipDirExcludingWithProgress zips a directory with progress reporting.
func ZipDirExcludingWithProgress(sourceDir, outputZip string, excludePatterns []string, onProgress ZipProgressFunc) error {
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

	// First pass: count files to zip for progress reporting
	totalFiles := 0
	if onProgress != nil {
		filepath.Walk(absSource, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			relPath, _ := filepath.Rel(absSource, path)
			relPath = filepath.ToSlash(relPath)
			if !IsExcluded(relPath, excludePatterns) {
				totalFiles++
			}
			return nil
		})
	}

	currentFile := 0
	return filepath.Walk(absSource, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("create zip file: %w", err)
		}

		// Build the relative name inside the zip
		relPath, err := filepath.Rel(absSource, path)
		if err != nil {
			return fmt.Errorf("write zip: %w", err)
		}
		// Normalize to forward slashes for zip
		relPath = filepath.ToSlash(relPath)
		if relPath == "." || relPath == "" {
			return nil
		}

		// Check exclusion patterns
		if excludePatterns != nil && IsExcluded(relPath, excludePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories — they'll be created implicitly by files
		if info.IsDir() {
			return nil
		}

		// Create zip entry
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("close zip writer: %w", err)
		}
		header.Name = relPath

		writer, err := w.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("open zip file: %w", err)
		}

		// Copy file content
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)

		// Report progress
		if onProgress != nil {
			currentFile++
			onProgress(currentFile, totalFiles, relPath)
		}

		return err
	})
}

// IsExcluded checks if a path matches any exclusion pattern.
func IsExcluded(relPath string, excludePatterns []string) bool {
	if excludePatterns == nil {
		return false
	}
	lower := strings.ToLower(relPath)
	// Extract base filename for filename-level pattern matching
	base := strings.ToLower(filepath.Base(relPath))
	for _, pat := range excludePatterns {
		patLower := strings.ToLower(pat)
		if strings.HasSuffix(pat, "/") {
			// Directory pattern: match dir name exactly or as a path prefix
			patDir := patLower
			if lower == strings.TrimSuffix(patDir, "/") || strings.HasPrefix(lower+"/", patDir) {
				return true
			}
		} else if strings.Contains(pat, "*") {
			// Glob pattern handling — check both full path and base filename
			trimLeft := strings.TrimPrefix(patLower, "*")
			trimRight := strings.TrimSuffix(patLower, "*")
			if strings.HasPrefix(patLower, "*") && strings.HasSuffix(patLower, "*") {
				// *.txt* → contains check on full path or base
				if strings.Contains(lower, trimLeft) || strings.Contains(base, trimLeft) {
					return true
				}
			} else if strings.HasPrefix(patLower, "*") {
				// *.txt → suffix check on full path or base
				if strings.HasSuffix(lower, trimLeft) || strings.HasSuffix(base, trimLeft) {
					return true
				}
			} else if strings.HasSuffix(patLower, "*") {
				// txt* → prefix check on full path or base
				if strings.HasPrefix(lower, trimRight) || strings.HasPrefix(base, trimRight) {
					return true
				}
			} else {
				// txt*txt → contains check on full path or base
				if strings.Contains(lower, patLower) || strings.Contains(base, patLower) {
					return true
				}
			}
		} else {
			// Exact filename match
			if lower == patLower || base == patLower || strings.HasSuffix(lower, "/"+patLower) {
				return true
			}
		}
	}
	return false
}
