package versioning

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SnapshotID is a unique identifier for a snapshot.
type SnapshotID string

// SnapshotInfo contains metadata about a snapshot.
type SnapshotInfo struct {
	ID        SnapshotID `json:"id"`
	Label     string     `json:"label"`
	Timestamp time.Time  `json:"timestamp"`
	FileCount int        `json:"file_count"`
	TotalSize int64      `json:"total_size"`
}

// SnapshotManager manages code snapshots for a project.
type SnapshotManager struct {
	snapshotDir string
}

// NewSnapshotManager creates a manager for the given project directory.
func NewSnapshotManager(projectDir string) *SnapshotManager {
	return &SnapshotManager{
		snapshotDir: filepath.Join(projectDir, ".moduforge", "snapshots"),
	}
}

// TakeSnapshot creates a compressed snapshot of the project directory.
func (sm *SnapshotManager) TakeSnapshot(projectDir, label string) (SnapshotID, error) {
	if err := os.MkdirAll(sm.snapshotDir, 0755); err != nil {
		return "", fmt.Errorf("create snapshot dir: %w", err)
	}

	id := SnapshotID(fmt.Sprintf("snap_%d", time.Now().UnixNano()))
	snapFile := filepath.Join(sm.snapshotDir, string(id)+".zip")

	// Create zip archive
	if err := sm.createZip(projectDir, snapFile); err != nil {
		return "", fmt.Errorf("create zip: %w", err)
	}

	// Count files and total size
	fileCount, totalSize := sm.countProjectFiles(projectDir)

	// Save metadata
	info := SnapshotInfo{
		ID:        id,
		Label:     label,
		Timestamp: time.Now(),
		FileCount: fileCount,
		TotalSize: totalSize,
	}

	if err := sm.saveMetadata(id, info); err != nil {
		return "", fmt.Errorf("save metadata: %w", err)
	}

	return id, nil
}

// RestoreSnapshot restores the project directory from a snapshot.
func (sm *SnapshotManager) RestoreSnapshot(projectDir, snapshotID string) error {
	snapFile := filepath.Join(sm.snapshotDir, snapshotID+".zip")
	if _, err := os.Stat(snapFile); os.IsNotExist(err) {
		return fmt.Errorf("snapshot not found: %s", snapshotID)
	}

	// Remove existing project files (except .moduforge)
	sm.cleanProjectDir(projectDir)

	// Extract zip
	if err := sm.extractZip(snapFile, projectDir); err != nil {
		return fmt.Errorf("extract zip: %w", err)
	}

	return nil
}

// ListSnapshots returns all snapshots for the project.
func (sm *SnapshotManager) ListSnapshots(projectDir string) []SnapshotInfo {
	metaDir := filepath.Join(sm.snapshotDir, "meta")
	entries, err := os.ReadDir(metaDir)
	if err != nil {
		return nil
	}

	var snapshots []SnapshotInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(metaDir, entry.Name()))
		if err != nil {
			continue
		}

		var info SnapshotInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}

		snapshots = append(snapshots, info)
	}

	// Sort by timestamp descending
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.After(snapshots[j].Timestamp)
	})

	return snapshots
}

// DeleteSnapshot removes a snapshot.
func (sm *SnapshotManager) DeleteSnapshot(snapshotID string) error {
	snapFile := filepath.Join(sm.snapshotDir, string(snapshotID)+".zip")
	metaFile := filepath.Join(sm.snapshotDir, "meta", string(snapshotID)+".json")

	os.Remove(snapFile)
	os.Remove(metaFile)
	return nil
}

// ═══════════════════════════════════════════════════════
// Internal helpers
// ═══════════════════════════════════════════════════════

func (sm *SnapshotManager) createZip(sourceDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		// Skip .moduforge directory
		rel, _ := filepath.Rel(sourceDir, path)
		if strings.HasPrefix(rel, ".moduforge") {
			return nil
		}

		// Skip very large files (>10MB)
		if info.Size() > 10*1024*1024 {
			return nil
		}

		f, err := w.Create(rel)
		if err != nil {
			return err
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()

		_, err = io.Copy(f, src)
		return err
	})
}

func (sm *SnapshotManager) extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)

		// Prevent zip slip
		if !strings.HasPrefix(filepath.Clean(fpath), filepath.Clean(destDir)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}

		outFile, err := os.Create(fpath)
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func (sm *SnapshotManager) saveMetadata(id SnapshotID, info SnapshotInfo) error {
	metaDir := filepath.Join(sm.snapshotDir, "meta")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(metaDir, string(id)+".json")
	return os.WriteFile(path, data, 0644)
}

func (sm *SnapshotManager) countProjectFiles(projectDir string) (int, int64) {
	var count int
	var totalSize int64

	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(projectDir, path)
		if strings.HasPrefix(rel, ".moduforge") {
			return nil
		}
		count++
		totalSize += info.Size()
		return nil
	})

	return count, totalSize
}

func (sm *SnapshotManager) cleanProjectDir(projectDir string) {
	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(projectDir, path)
		if rel == "." || strings.HasPrefix(rel, ".moduforge") {
			return nil
		}
		os.RemoveAll(path)
		return nil
	})
}

// CreateTGZ creates a tar.gz archive (alternative format).
func (sm *SnapshotManager) CreateTGZ(sourceDir, destPath string) error {
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		rel, _ := filepath.Rel(sourceDir, path)
		if strings.HasPrefix(rel, ".moduforge") {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()

		_, err = io.Copy(tw, src)
		return err
	})
}

// ExtractTGZ extracts a tar.gz archive.
func (sm *SnapshotManager) ExtractTGZ(srcPath, destDir string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		fpath := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(filepath.Clean(fpath), filepath.Clean(destDir)) {
			continue
		}

		if header.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}

		outFile, err := os.Create(fpath)
		if err != nil {
			return err
		}

		_, err = io.Copy(outFile, tr)
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
