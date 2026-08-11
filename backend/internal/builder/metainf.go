package builder

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Standard update-binary for Magisk/KernelSU/APatch module installation.
// This is the minimal bootstrap that extracts the module and runs customize.sh.
var metaInfUpdateBinary = []byte(`#!/sbin/sh

#################
# Initialization
#################

umask 022

# echo before loading util_functions
ui_print() { echo "$1"; }

require_new_magisk() {
  ui_print "*******************************"
  ui_print " Please install Magisk v20.4+! "
  ui_print "*******************************"
  exit 1
}

#########################
# Load util_functions.sh
#########################

OUTFD=$2
ZIPFILE=$3

[ -f /data/adb/magisk/util_functions.sh ] || require_new_magisk
. /data/adb/magisk/util_functions.sh
[ $MAGISK_VER_CODE -lt 20400 ] && require_new_magisk

install_module
exit 0
`)

// updater-script is always empty for Magisk/KernelSU/APatch modules.
var metaInfUpdaterScript = []byte(`#MAGISK
`)

// EnsureMetaInf checks if the zip contains META-INF/com/google/android/{update-binary, updater-script}.
// If either is missing, it creates them inside the zip. This ensures compatibility
// with Magisk, KernelSU, and APatch, which all require META-INF for module installation.
func EnsureMetaInf(zipPath string) error {
	// Step 1: Read zip into memory and check for META-INF
	type zipEntry struct {
		Name    string
		Content []byte
		Header  os.FileInfo
	}

	var entries []zipEntry
	hasUpdateBinary := false
	hasUpdaterScript := false

	func() error {
		reader, err := zip.OpenReader(zipPath)
		if err != nil {
			return fmt.Errorf("open zip: %w", err)
		}
		defer reader.Close()

		for _, f := range reader.File {
			nameNorm := filepath.ToSlash(f.Name)

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
			entries = append(entries, zipEntry{Name: f.Name, Content: content, Header: fi})

			base := filepath.Base(nameNorm)
			dir := filepath.ToSlash(filepath.Dir(nameNorm))

			if dir == "META-INF/com/google/android" {
				if base == "update-binary" {
					hasUpdateBinary = true
				}
				if base == "updater-script" {
					hasUpdaterScript = true
				}
			}
		}
		return nil
	}()

	// If both exist, nothing to do
	if hasUpdateBinary && hasUpdaterScript {
		return nil
	}

	// Step 2: Create new zip with injected META-INF entries
	tmpZip := zipPath + ".tmp"
	outFile, err := os.Create(tmpZip)
	if err != nil {
		return fmt.Errorf("create temp zip: %w", err)
	}

	w := zip.NewWriter(outFile)

	// Copy existing entries
	for _, entry := range entries {
		fw, err := w.Create(entry.Name)
		if err != nil {
			w.Close()
			outFile.Close()
			os.Remove(tmpZip)
			return fmt.Errorf("create entry: %w", err)
		}
		fw.Write(entry.Content)
	}

	// Inject missing META-INF files
	if !hasUpdateBinary {
		fw, err := w.Create("META-INF/com/google/android/update-binary")
		if err != nil {
			w.Close()
			outFile.Close()
			os.Remove(tmpZip)
			return fmt.Errorf("create update-binary: %w", err)
		}
		fw.Write(metaInfUpdateBinary)
	}

	if !hasUpdaterScript {
		fw, err := w.Create("META-INF/com/google/android/updater-script")
		if err != nil {
			w.Close()
			outFile.Close()
			os.Remove(tmpZip)
			return fmt.Errorf("create updater-script: %w", err)
		}
		fw.Write(metaInfUpdaterScript)
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

	if !hasUpdateBinary {
		fmt.Printf("[Builder] Auto-generated META-INF/com/google/android/update-binary\n")
	}
	if !hasUpdaterScript {
		fmt.Printf("[Builder] Auto-generated META-INF/com/google/android/updater-script\n")
	}

	return nil
}

// InjectMetaInfToBytes does the same as EnsureMetaInf but operates on a byte slice
// (for the Docker build path where the zip is returned as bytes).
func InjectMetaInfToBytes(zipData []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("open zip reader: %w", err)
	}

	type zipEntry struct {
		Name    string
		Content []byte
		Header  *zip.FileHeader
	}

	var entries []zipEntry
	hasUpdateBinary := false
	hasUpdaterScript := false

	for _, f := range reader.File {
		nameNorm := filepath.ToSlash(f.Name)

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open entry %s: %w", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read entry %s: %w", f.Name, err)
		}

		entries = append(entries, zipEntry{Name: f.Name, Content: content, Header: &f.FileHeader})

		base := filepath.Base(nameNorm)
		dir := filepath.ToSlash(filepath.Dir(nameNorm))

		if dir == "META-INF/com/google/android" {
			if base == "update-binary" {
				hasUpdateBinary = true
			}
			if base == "updater-script" {
				hasUpdaterScript = true
			}
		}
	}

	if hasUpdateBinary && hasUpdaterScript {
		return zipData, nil
	}

	// Rebuild zip with injected entries
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	for _, entry := range entries {
		fw, err := w.Create(entry.Name)
		if err != nil {
			return nil, fmt.Errorf("create entry: %w", err)
		}
		fw.Write(entry.Content)
	}

	if !hasUpdateBinary {
		fw, err := w.Create("META-INF/com/google/android/update-binary")
		if err != nil {
			return nil, fmt.Errorf("create update-binary: %w", err)
		}
		fw.Write(metaInfUpdateBinary)
	}

	if !hasUpdaterScript {
		fw, err := w.Create("META-INF/com/google/android/updater-script")
		if err != nil {
			return nil, fmt.Errorf("create updater-script: %w", err)
		}
		fw.Write(metaInfUpdaterScript)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// HasMetaInf checks if zip data contains META-INF structure
func HasMetaInf(zipData []byte) bool {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return false
	}

	hasUpdateBinary := false
	hasUpdaterScript := false

	for _, f := range reader.File {
		nameNorm := filepath.ToSlash(f.Name)
		base := filepath.Base(nameNorm)
		dir := filepath.ToSlash(filepath.Dir(nameNorm))

		if dir == "META-INF/com/google/android" {
			if base == "update-binary" {
				hasUpdateBinary = true
			}
			if base == "updater-script" {
				hasUpdaterScript = true
			}
		}
	}

	return hasUpdateBinary && hasUpdaterScript
}
