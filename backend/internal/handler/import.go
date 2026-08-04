package handler

import (
	"archive/zip"
	"io"
	"strings"

	"github.com/gofiber/fiber/v3"
)

type ModuleInfo struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	VersionCode     string `json:"versionCode"`
	Author          string `json:"author"`
	Description     string `json:"description"`
	KSUSupported    bool   `json:"ksu_supported"`
	APatchSupported bool   `json:"apatch_supported"`
}

type ZipFileInfo struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
}

type ParseZipResponse struct {
	Module ModuleInfo    `json:"module"`
	Files  []ZipFileInfo `json:"files"`
}

type ImportZipResponse struct {
	Module   ModuleInfo            `json:"module"`
	Files    []ZipFileContent      `json:"files"`
	FileList []ZipFileInfo         `json:"file_list"`
}

type ZipFileContent struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	IsDir   bool   `json:"is_dir"`
}

const (
	maxZipUploadSize   = 50 * 1024 * 1024 // 50MB max upload
	maxZipEntries      = 1000             // max files in zip
	maxZipEntrySize    = 5 * 1024 * 1024  // 5MB per file entry
	maxTotalUncompressed = 100 * 1024 * 1024 // 100MB total uncompressed
)

func ParseModuleZip(c fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "missing file"})
	}

	if !strings.HasSuffix(strings.ToLower(file.Filename), ".zip") {
		return c.Status(400).JSON(fiber.Map{"error": "file must be a .zip archive"})
	}

	// Enforce upload size limit
	if file.Size > maxZipUploadSize {
		return c.Status(400).JSON(fiber.Map{"error": "file too large, max 50MB"})
	}

	fh, err := file.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to open uploaded file"})
	}
	defer fh.Close()

	// Read entire file into memory (bounded by maxZipUploadSize)
	raw := make([]byte, 0, file.Size)
	buf := make([]byte, 32768)
	for {
		n, err := fh.Read(buf)
		if n > 0 {
			raw = append(raw, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to read uploaded file"})
		}
	}

	reader, err := zip.NewReader(io.NewSectionReader(readerAt(raw), 0, int64(len(raw))), int64(len(raw)))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid zip file"})
	}

	var module ModuleInfo
	var files []ZipFileInfo
	totalUncompressed := uint64(0)

	for i, zf := range reader.File {
		// Limit number of entries
		if i >= maxZipEntries {
			return c.Status(400).JSON(fiber.Map{"error": "too many files in zip (max 1000)"})
		}
		// Track total uncompressed size to prevent zip bomb
		totalUncompressed += zf.UncompressedSize64
		if totalUncompressed > maxTotalUncompressed {
			return c.Status(400).JSON(fiber.Map{"error": "uncompressed zip too large (max 100MB)"})
		}

		info := ZipFileInfo{
			Path:  zf.Name,
			Size:  int64(zf.UncompressedSize64),
			IsDir: zf.FileInfo().IsDir(),
		}
		files = append(files, info)

		if normalizePath(zf.Name) == "module.prop" && !zf.FileInfo().IsDir() {
			// Limit individual file size
			if zf.UncompressedSize64 > maxZipEntrySize {
				continue
			}
			rc, err := zf.Open()
			if err != nil {
				continue
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			module = parseModuleProp(string(content))
		}
	}

	return c.JSON(ParseZipResponse{
		Module: module,
		Files:  files,
	})
}

func ImportModuleZip(c fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "missing file"})
	}

	if !strings.HasSuffix(strings.ToLower(file.Filename), ".zip") {
		return c.Status(400).JSON(fiber.Map{"error": "file must be a .zip archive"})
	}

	// Enforce upload size limit
	if file.Size > maxZipUploadSize {
		return c.Status(400).JSON(fiber.Map{"error": "file too large, max 50MB"})
	}

	fh, err := file.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to open uploaded file"})
	}
	defer fh.Close()

	raw := make([]byte, 0, file.Size)
	buf := make([]byte, 32768)
	for {
		n, err := fh.Read(buf)
		if n > 0 {
			raw = append(raw, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to read uploaded file"})
		}
	}

	reader, err := zip.NewReader(io.NewSectionReader(readerAt(raw), 0, int64(len(raw))), int64(len(raw)))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid zip file"})
	}

	var module ModuleInfo
	var contents []ZipFileContent
	var fileList []ZipFileInfo
	totalUncompressed := uint64(0)

	for i, zf := range reader.File {
		// Limit number of entries
		if i >= maxZipEntries {
			return c.Status(400).JSON(fiber.Map{"error": "too many files in zip (max 1000)"})
		}
		// Track total uncompressed size to prevent zip bomb
		totalUncompressed += zf.UncompressedSize64
		if totalUncompressed > maxTotalUncompressed {
			return c.Status(400).JSON(fiber.Map{"error": "uncompressed zip too large (max 100MB)"})
		}

		info := ZipFileInfo{
			Path:  zf.Name,
			Size:  int64(zf.UncompressedSize64),
			IsDir: zf.FileInfo().IsDir(),
		}
		fileList = append(fileList, info)

		if zf.FileInfo().IsDir() {
			contents = append(contents, ZipFileContent{Path: zf.Name, Content: "", IsDir: true})
			continue
		}

		// Limit individual file size
		if zf.UncompressedSize64 > maxZipEntrySize {
			contents = append(contents, ZipFileContent{Path: zf.Name, Content: "[file too large, skipped]", IsDir: false})
			continue
		}

		rc, err := zf.Open()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		contents = append(contents, ZipFileContent{
			Path:    zf.Name,
			Content: string(content),
			IsDir:   false,
		})

		if normalizePath(zf.Name) == "module.prop" && module.ID == "" {
			module = parseModuleProp(string(content))
		}
	}

	return c.JSON(ImportZipResponse{
		Module:   module,
		Files:    contents,
		FileList: fileList,
	})
}

func normalizePath(p string) string {
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimSuffix(p, "/")
	return p
}

func parseModuleProp(content string) ModuleInfo {
	var m ModuleInfo
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "id":
			m.ID = val
		case "name":
			m.Name = val
		case "version":
			m.Version = val
		case "versionCode":
			m.VersionCode = val
		case "author":
			m.Author = val
		case "description":
			m.Description = val
		case "ksu.supported":
			m.KSUSupported = val == "true"
		case "apatch.supported":
			m.APatchSupported = val == "true"
		}
	}
	return m
}

type readerAt []byte

func (b readerAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(b)) {
		return 0, io.EOF
	}
	n := copy(p, b[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
