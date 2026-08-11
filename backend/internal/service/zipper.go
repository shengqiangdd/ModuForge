package service

import (
	"archive/zip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ModuleFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	IsDir   bool   `json:"is_dir,omitempty"`
}

type ZipperService struct {
	outputDir string
	db        *sql.DB
}

func NewZipperService(outputDir string, db *sql.DB) *ZipperService {
	os.MkdirAll(outputDir, 0755)
	return &ZipperService{outputDir: outputDir, db: db}
}

var excludedPatterns = []string{
	// Source code (do NOT include in module zip)
	"src/",
	"*.go",
	"*.rs",
	"*.c",
	"*.h",
	"*.cpp",
	"*.py",
	"*.java",
	"*.kt",
	// Build system
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
	".build_status/",
	".build_status.json",
	"build-cache/",
	// Dev files
	"node_modules/",
	".git/",
	".DS_Store",
	"__pycache__/",
	"*.tmp",
	"*.xml",
	"*.gradle",
	// Documentation
	"README*",
	"LICENSE*",
	"CHANGELOG*",
	"CONTRIBUTING*",
	"*.md",
	"docs/",
	"doc/",
	// Test files
	"test/",
	"tests/",
	"*_test.go",
	"*_test.rs",
	"*_test.cpp",
	"test_*",
	// Temp and debug files
	"tmp/",
	"upload",
	// Build artifacts
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
	"Thumbs.db",
	"desktop.ini",
	// Version control
	".gitignore",
	".gitattributes",
	".svn/",
	".hg/",
	// Config files that shouldn't be in module
	".env",
	".env.*",
	"docker-compose*.yml",
	"Dockerfile*",
	// Shell/Editor temp & config files
	".es*",
	".shell*",
	// Cargo lock files
	".cargo-*",
	// Debug symbols
	"*.d",
	// Backend source (should be excluded)
	"app/backend/",
}

// Frontend file patterns that should be wrapped in webroot/
var frontendPatterns = []string{
	"index.html",
	"css/",
	"js/",
	"app/",
	"fonts/",
	"assets/",
	"manifest.json",
	"sw.js",
	"icon-*.svg",
}

func isExcluded(path string) bool {
	lower := strings.ToLower(path)
	for _, pat := range excludedPatterns {
		if strings.HasSuffix(pat, "/") {
			if strings.HasPrefix(lower, strings.ToLower(pat)) {
				return true
			}
		} else if strings.Contains(pat, "*") {
			suffix := strings.TrimPrefix(pat, "*")
			if strings.HasSuffix(lower, strings.ToLower(suffix)) {
				return true
			}
		} else {
			if lower == strings.ToLower(pat) || strings.HasSuffix(lower, "/"+strings.ToLower(pat)) {
				return true
			}
		}
	}
	return false
}

func isFrontendFile(path string) bool {
	lower := strings.ToLower(path)
	for _, pat := range frontendPatterns {
		if strings.HasSuffix(pat, "/") {
			if strings.HasPrefix(lower, strings.ToLower(pat)) {
				return true
			}
		} else if strings.Contains(pat, "*") {
			suffix := strings.TrimPrefix(pat, "*")
			if strings.HasSuffix(lower, strings.ToLower(suffix)) {
				return true
			}
		} else {
			if lower == strings.ToLower(pat) {
				return true
			}
		}
	}
	return false
}

func (s *ZipperService) BuildModuleZip(_ context.Context, _ string, files []ModuleFile) (string, error) {
	timestamp := time.Now().UnixMilli()
	zipName := fmt.Sprintf("moduforge_module_%d.zip", timestamp)
	zipPath := filepath.Join(s.outputDir, zipName)

	zipFile, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("create zip file: %w", err)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	if err := addMetaInf(zw); err != nil {
		return "", fmt.Errorf("add META-INF: %w", err)
	}

	for _, f := range files {
		if isExcluded(f.Path) {
			continue
		}

		// Sanitize path to prevent path traversal
		cleanPath := filepath.Clean(f.Path)
		if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
			continue // skip malicious paths
		}

		// Wrap frontend files in webroot/
		zipPath := cleanPath
		if isFrontendFile(cleanPath) {
			zipPath = "webroot/" + cleanPath
		}

		if f.IsDir {
			if _, err := zw.Create(zipPath + "/"); err != nil {
				return "", fmt.Errorf("create dir %s: %w", zipPath, err)
			}
			continue
		}

		header := &zip.FileHeader{
			Name:   zipPath,
			Method: zip.Deflate,
		}
		if strings.HasSuffix(zipPath, ".sh") || zipPath == "META-INF/com/google/android/update-binary" {
			header.SetMode(0755)
		} else {
			header.SetMode(0644)
		}
		header.Modified = time.Now()

		w, err := zw.CreateHeader(header)
		if err != nil {
			return "", fmt.Errorf("create file %s: %w", zipPath, err)
		}

		if _, err := io.WriteString(w, f.Content); err != nil {
			return "", fmt.Errorf("write file %s: %w", zipPath, err)
		}
	}

	return zipPath, nil
}

func addMetaInf(zw *zip.Writer) error {
	for _, dir := range []string{"META-INF/", "META-INF/com/", "META-INF/com/google/", "META-INF/com/google/android/"} {
		if _, err := zw.Create(dir); err != nil {
			return err
		}
	}

	ubHeader := &zip.FileHeader{
		Name:   "META-INF/com/google/android/update-binary",
		Method: zip.Deflate,
	}
	ubHeader.SetMode(0755)
	ubHeader.Modified = time.Now()

	w, err := zw.CreateHeader(ubHeader)
	if err != nil {
		return err
	}

	updateBinary := `#!/sbin/sh

###############
# Initialization
###############
umask 022

# echo before loading util_functions
ui_print() { echo "$1"; }

require_new_android() {
  ui_print "******************************"
  ui_print " Please install Magisk v20.4+! "
  ui_print "******************************"
  exit 1
}

#########################
# Load util_functions.sh
#########################
OUTFD=$2
ZIPFILE=$3

mount /data 2>/dev/null
mount /data 2>/dev/null

if [ -f /data/adb/magisk/util_functions.sh ]; then
  . /data/adb/magisk/util_functions.sh
elif [ -f /data/adb/ksu/util_functions.sh ]; then
  . /data/adb/ksu/util_functions.sh
elif [ -f /data/adb/ap/util_functions.sh ]; then
  . /data/adb/ap/util_functions.sh
else
  require_new_android
fi

[ $MAGISK_VER_CODE -gt 20000 ] || require_new_android

install_module
exit 0
`
	if _, err := io.WriteString(w, updateBinary); err != nil {
		return err
	}

	usHeader := &zip.FileHeader{
		Name:   "META-INF/com/google/android/updater-script",
		Method: zip.Deflate,
	}
	usHeader.SetMode(0644)
	usHeader.Modified = time.Now()

	w2, err := zw.CreateHeader(usHeader)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w2, "#MAGISK\n"); err != nil {
		return err
	}

	return nil
}

func (s *ZipperService) ExportModuleZip(projectID string) (string, error) {
	rows, err := s.db.Query(
		"SELECT path, content FROM project_files WHERE project_id = ? ORDER BY path",
		projectID,
	)
	if err != nil {
		return "", fmt.Errorf("read project files: %w", err)
	}
	defer rows.Close()

	var files []ModuleFile
	hasModuleProp := false
	for rows.Next() {
		var path, content string
		if err := rows.Scan(&path, &content); err != nil {
			continue
		}
		if path == "module.prop" {
			hasModuleProp = true
			if !strings.Contains(content, "id=") || !strings.Contains(content, "name=") || !strings.Contains(content, "version=") {
				return "", fmt.Errorf("module.prop must contain id, name, and version fields")
			}
		}
		files = append(files, ModuleFile{Path: path, Content: content})
	}

	if !hasModuleProp {
		return "", fmt.Errorf("project must contain a module.prop file")
	}

	return s.BuildModuleZip(context.Background(), projectID, files)
}

func (s *ZipperService) GetAvailableDownloads() []string {
	entries, err := os.ReadDir(s.outputDir)
	if err != nil {
		return nil
	}
	var zips []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".zip") {
			zips = append(zips, e.Name())
		}
	}
	return zips
}
