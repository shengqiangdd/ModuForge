#!/usr/bin/env python3
"""Fix zipper.go to add webroot wrapper and proper exclusions."""

import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

# New zipper.go with fixes
new_zipper_go = '''package service

import (
\t"archive/zip"
\t"context"
\t"database/sql"
\t"fmt"
\t"io"
\t"os"
\t"path/filepath"
\t"strings"
\t"time"
)

type ModuleFile struct {
\tPath    string `json:"path"`
\tContent string `json:"content"`
\tIsDir   bool   `json:"is_dir,omitempty"`
}

type ZipperService struct {
\toutputDir string
\tdb        *sql.DB
}

func NewZipperService(outputDir string, db *sql.DB) *ZipperService {
\tos.MkdirAll(outputDir, 0755)
\treturn &ZipperService{outputDir: outputDir, db: db}
}

// excludedPatterns defines files/directories to exclude from module zip.
// Only runtime files (compiled binaries, shell scripts, module metadata) are included.
var excludedPatterns = []string{
\t// Source code
\t"src/",
\t"*.go",
\t"*.rs",
\t"*.c",
\t"*.h",
\t"*.cpp",
\t"*.py",
\t"*.java",
\t"*.kt",
\t// Build system
\t"build.sh",
\t"compile.sh",
\t"go.mod",
\t"go.sum",
\t"Cargo.toml",
\t"Cargo.lock",
\t"target/",
\t"Makefile",
\t// Build cache & artifacts
\t".build_cache/",
\t".build_cache.json",
\t".build_status/",
\t".build_status.json",
\t"build-cache/",
\t".cargo-artifact-lock",
\t".cargo-build-lock",
\t".cargo-lock",
\t// Debug symbols
\t"*.d",
\t"*.o",
\t"*.a",
\t// Dev files
\t"node_modules/",
\t".git/",
\t".DS_Store",
\t"__pycache__/",
\t"*.tmp",
\t"*.xml",
\t"*.gradle",
\t// Documentation
\t"*.md",
\t"docs/",
\t"doc/",
\t// IDE
\t".idea/",
\t".vscode/",
\t".vs/",
\t// Temp and misc
\t"tmp/",
\t"upload",
\t"DESIGN_DOC.md",
\t"rust_files_list.txt",
\t// Backend source (should NOT be in module)
\t"app/backend/",
}

func isExcluded(path string) bool {
\tfor _, pat := range excludedPatterns {
\t\tif strings.HasSuffix(pat, "/") {
\t\t\tif strings.Contains(path, pat) {
\t\t\t\treturn true
\t\t\t}
\t\t} else if strings.HasPrefix(pat, "*") {
\t\t\tsuffix := pat[1:]
\t\t\tif strings.HasSuffix(path, suffix) {
\t\t\t\treturn true
\t\t\t}
\t\t} else {
\t\t\tif path == pat || strings.HasSuffix(path, "/"+pat) {
\t\t\t\treturn true
\t\t\t}
\t\t}
\t}
\treturn false
}

func (s *ZipperService) BuildModuleZip(_ context.Context, _ string, files []ModuleFile) (string, error) {
\ttimestamp := time.Now().UnixMilli()
\tzipName := fmt.Sprintf("moduforge_module_%d.zip", timestamp)
\tzipPath := filepath.Join(s.outputDir, zipName)

\tzipFile, err := os.Create(zipPath)
\tif err != nil {
\t\treturn "", fmt.Errorf("create zip file: %w", err)
\t}
\tdefer zipFile.Close()

\tzw := zip.NewWriter(zipFile)
\tdefer zw.Close()

\tif err := addMetaInf(zw); err != nil {
\t\treturn "", fmt.Errorf("add META-INF: %w", err)
\t}

\t// Collect frontend files for webroot wrapper
\tvar frontendFiles []ModuleFile
\tvar otherFiles []ModuleFile

\tfor _, f := range files {
\t\tif isExcluded(f.Path) {
\t\t\tcontinue
\t\t}

\t\t// Sanitize path to prevent path traversal
\t\tcleanPath := filepath.Clean(f.Path)
\t\tif strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
\t\t\tcontinue // skip malicious paths
\t\t}

\t\t// Detect frontend files (HTML, CSS, JS, images, etc.)
\t\tif isFrontendFile(cleanPath) {
\t\t\tfrontendFiles = append(frontendFiles, ModuleFile{Path: cleanPath, Content: f.Content, IsDir: f.IsDir})
\t\t} else {
\t\t\totherFiles = append(otherFiles, ModuleFile{Path: cleanPath, Content: f.Content, IsDir: f.IsDir})
\t\t}
\t}

\t// Write frontend files under webroot/
\tfor _, f := range frontendFiles {
\t\twebrootPath := "webroot/" + f.Path
\t\tif f.IsDir {
\t\t\tif _, err := zw.Create(webrootPath + "/"); err != nil {
\t\t\t\treturn "", fmt.Errorf("create dir %s: %w", webrootPath, err)
\t\t\t}
\t\t\tcontinue
\t\t}

\t\theader := &zip.FileHeader{
\t\t\tName:   webrootPath,
\t\t\tMethod: zip.Deflate,
\t\t}
\t\theader.SetMode(0644)
\t\theader.Modified = time.Now()

\t\tw, err := zw.CreateHeader(header)
\t\tif err != nil {
\t\t\treturn "", fmt.Errorf("create file %s: %w", webrootPath, err)
\t\t}

\t\tif _, err := io.WriteString(w, f.Content); err != nil {
\t\t\treturn "", fmt.Errorf("write file %s: %w", webrootPath, err)
\t\t}
\t}

\t// Write other files at root
\tfor _, f := range otherFiles {
\t\tif f.IsDir {
\t\t\tif _, err := zw.Create(f.Path + "/"); err != nil {
\t\t\t\treturn "", fmt.Errorf("create dir %s: %w", f.Path, err)
\t\t\t}
\t\t\tcontinue
\t\t}

\t\theader := &zip.FileHeader{
\t\t\tName:   f.Path,
\t\t\tMethod: zip.Deflate,
\t\t}
\t\tif strings.HasSuffix(f.Path, ".sh") || f.Path == "META-INF/com/google/android/update-binary" {
\t\t\theader.SetMode(0755)
\t\t} else {
\t\t\theader.SetMode(0644)
\t\t}
\t\theader.Modified = time.Now()

\t\tw, err := zw.CreateHeader(header)
\t\tif err != nil {
\t\t\treturn "", fmt.Errorf("create file %s: %w", f.Path, err)
\t\t}

\t\tif _, err := io.WriteString(w, f.Content); err != nil {
\t\t\treturn "", fmt.Errorf("write file %s: %w", f.Path, err)
\t\t}
\t}

\treturn zipPath, nil
}

// isFrontendFile checks if a file path is a frontend asset
func isFrontendFile(path string) bool {
\tlower := strings.ToLower(path)
\texts := []string{".html", ".css", ".js", ".json", ".svg", ".png", ".jpg", ".jpeg", ".gif", ".ico", ".woff", ".woff2", ".ttf", ".eot"}
\tfor _, ext := range exts {
\t\tif strings.HasSuffix(lower, ext) {
\t\t\treturn true
\t\t}
\t}
\t// Directories that are frontend assets
\tfrontendDirs := []string{"css/", "js/", "app/", "assets/", "images/", "fonts/"}
\tfor _, dir := range frontendDirs {
\t\tif strings.HasPrefix(lower, dir) {
\t\t\treturn true
\t\t}
\t}
\treturn false
}

func addMetaInf(zw *zip.Writer) error {
\tfor _, dir := range []string{"META-INF/", "META-INF/com/", "META-INF/com/google/", "META-INF/com/google/android/"} {
\t\tif _, err := zw.Create(dir); err != nil {
\t\t\treturn err
\t\t}
\t}

\tubHeader := &zip.FileHeader{
\t\tName:   "META-INF/com/google/android/update-binary",
\t\tMethod: zip.Deflate,
\t}
\tubHeader.SetMode(0755)
\tubHeader.Modified = time.Now()

\tw, err := zw.CreateHeader(ubHeader)
\tif err != nil {
\t\treturn err
\t}

\tupdateBinary := `#!/sbin/sh

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
\tif _, err := io.WriteString(w, updateBinary); err != nil {
\t\treturn err
\t}

\tusHeader := &zip.FileHeader{
\t\tName:   "META-INF/com/google/android/updater-script",
\t\tMethod: zip.Deflate,
\t}
\tusHeader.SetMode(0644)
\tusHeader.Modified = time.Now()

\tw2, err := zw.CreateHeader(usHeader)
\tif err != nil {
\t\treturn err
\t}
\tif _, err := io.WriteString(w2, "#MAGISK\\n"); err != nil {
\t\treturn err
\t}

\treturn nil
}

func (s *ZipperService) ExportModuleZip(projectID string) (string, error) {
\trows, err := s.db.Query(
\t\t"SELECT path, content FROM project_files WHERE project_id = ? ORDER BY path",
\t\tprojectID,
\t)
\tif err != nil {
\t\treturn "", fmt.Errorf("read project files: %w", err)
\t}
\tdefer rows.Close()

\tvar files []ModuleFile
\thasModuleProp := false
\tfor rows.Next() {
\t\tvar path, content string
\t\tif err := rows.Scan(&path, &content); err != nil {
\t\t\tcontinue
\t\t}
\t\tif path == "module.prop" {
\t\t\thasModuleProp = true
\t\t\tif !strings.Contains(content, "id=") || !strings.Contains(content, "name=") || !strings.Contains(content, "version=") {
\t\t\t\treturn "", fmt.Errorf("module.prop must contain id, name, and version fields")
\t\t\t}
\t\t}
\t\tfiles = append(files, ModuleFile{Path: path, Content: content})
\t}

\tif !hasModuleProp {
\t\treturn "", fmt.Errorf("project must contain a module.prop file")
\t}

\treturn s.BuildModuleZip(context.Background(), projectID, files)
}

func (s *ZipperService) GetAvailableDownloads() []string {
\tentries, err := os.ReadDir(s.outputDir)
\tif err != nil {
\t\treturn nil
\t}
\tvar zips []string
\tfor _, e := range entries {
\t\tif !e.IsDir() && strings.HasSuffix(e.Name(), ".zip") {
\t\t\tzips = append(zips, e.Name())
\t\t}
\t}
\treturn zips
}
'''

# Write the new zipper.go to the server
print("Writing new zipper.go to server...")
sftp = ssh.open_sftp()
with sftp.open("/tmp/zipper.go", "w") as f:
    f.write(new_zipper_go)
sftp.close()

# Copy to container and recompile
print("Copying to container and recompiling...")
commands = [
    f"docker cp /tmp/zipper.go {CONTAINER}:/tmp/zipper.go",
    f"docker exec {CONTAINER} cp /tmp/zipper.go /app/backend/internal/service/zipper.go",
    f"docker exec {CONTAINER} sh -c 'cd /app/backend && go build -o /tmp/server ./cmd/moduforge'",
    f"docker cp {CONTAINER}:/tmp/server /tmp/server",
    f"docker exec {CONTAINER} cp /tmp/server /app/server",
    f"docker exec {CONTAINER} chmod +x /app/server",
    f"docker restart {CONTAINER}",
]

for cmd in commands:
    print(f"Running: {cmd}")
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode('utf-8', errors='replace')
    err = stderr.read().decode('utf-8', errors='replace')
    if out: print(f"OUT: {out}")
    if err: print(f"ERR: {err}")

ssh.close()
print("\nDone!")
