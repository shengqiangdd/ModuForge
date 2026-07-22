package service

import (
	"archive/zip"
	"context"
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
}

func NewZipperService(outputDir string) *ZipperService {
	os.MkdirAll(outputDir, 0755)
	return &ZipperService{outputDir: outputDir}
}

var excludedPatterns = []string{
	"node_modules/",
	".git/",
	".DS_Store",
	"__pycache__/",
	"*.tmp",
}

func isExcluded(path string) bool {
	for _, pat := range excludedPatterns {
		if strings.HasSuffix(pat, "/") {
			if strings.Contains(path, pat) {
				return true
			}
		} else if strings.HasPrefix(pat, "*") {
			suffix := pat[1:]
			if strings.HasSuffix(path, suffix) {
				return true
			}
		} else if path == pat {
			return true
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

		if f.IsDir {
			if _, err := zw.Create(f.Path + "/"); err != nil {
				return "", fmt.Errorf("create dir %s: %w", f.Path, err)
			}
			continue
		}

		header := &zip.FileHeader{
			Name:   f.Path,
			Method: zip.Deflate,
		}
		if strings.HasSuffix(f.Path, ".sh") || f.Path == "META-INF/com/google/android/update-binary" {
			header.SetMode(0755)
		} else {
			header.SetMode(0644)
		}
		header.Modified = time.Now()

		w, err := zw.CreateHeader(header)
		if err != nil {
			return "", fmt.Errorf("create file %s: %w", f.Path, err)
		}

		if _, err := io.WriteString(w, f.Content); err != nil {
			return "", fmt.Errorf("write file %s: %w", f.Path, err)
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
