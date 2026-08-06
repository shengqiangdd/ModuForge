package agent

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// ═══════════════════════════════════════════════════════════════════
// Repo-Map — Global Code Structure Index (inspired by Aider)
//
// Scans the project directory and extracts function/struct/enum signatures
// from source files. Provides a compact summary (~2-4KB) that gives the
// LLM a bird's-eye view of the codebase without reading every file.
// ═══════════════════════════════════════════════════════════════════

type RepoMap struct {
	root      string
	fileIndex map[string]FileInfo // path -> {size, hash, signatures}
	mu        sync.RWMutex
}

// FileInfo holds metadata and signatures for a single file.
type FileInfo struct {
	Size       int64
	Hash       string   // sha256 hex (first 12 chars)
	Signatures []string // function/struct/enum signatures
}

// sourceExtensions lists file extensions to scan for signatures.
var sourceExtensions = map[string]bool{
	".rs": true, ".go": true, ".cpp": true, ".c": true, ".h": true, ".hpp": true,
	".ts": true, ".js": true, ".tsx": true, ".jsx": true,
	".py": true, ".java": true, ".kt": true,
}

// signaturePatterns holds compiled regex patterns per language for extracting signatures.
var signaturePatterns = map[string][]*regexp.Regexp{
	".rs": {
		regexp.MustCompile(`^\s*pub\s+fn\s+(\w+)`),
		regexp.MustCompile(`^\s*fn\s+(\w+)`),
		regexp.MustCompile(`^\s*pub\s+struct\s+(\w+)`),
		regexp.MustCompile(`^\s*struct\s+(\w+)`),
		regexp.MustCompile(`^\s*pub\s+enum\s+(\w+)`),
		regexp.MustCompile(`^\s*enum\s+(\w+)`),
		regexp.MustCompile(`^\s*pub\s+trait\s+(\w+)`),
		regexp.MustCompile(`^\s*impl\s+(\w+)`),
	},
	".go": {
		regexp.MustCompile(`^func\s+(\w+)`),
		regexp.MustCompile(`^func\s*\(\s*\w+\s+\*?\w+\)\s+(\w+)`),
		regexp.MustCompile(`^type\s+(\w+)\s+struct`),
		regexp.MustCompile(`^type\s+(\w+)\s+interface`),
	},
	".cpp": {
		regexp.MustCompile(`^(?:static\s+)?(?:void|int|char|bool|float|double|long|unsigned|auto)\s+(\w+)\s*\(`),
		regexp.MustCompile(`^(?:class|struct|enum)\s+(\w+)`),
	},
	".c": {
		regexp.MustCompile(`^(?:static\s+)?(?:void|int|char|bool|float|double|long|unsigned)\s+(\w+)\s*\(`),
		regexp.MustCompile(`^(?:struct|enum)\s+(\w+)`),
	},
	".h": {
		regexp.MustCompile(`^(?:static\s+)?(?:void|int|char|bool|float|double|long|unsigned)\s+(\w+)\s*\(`),
		regexp.MustCompile(`^(?:struct|enum)\s+(\w+)`),
	},
	".ts": {
		regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+(\w+)`),
		regexp.MustCompile(`^(?:export\s+)?class\s+(\w+)`),
		regexp.MustCompile(`^(?:export\s+)?interface\s+(\w+)`),
		regexp.MustCompile(`^(?:export\s+)?type\s+(\w+)`),
	},
	".js": {
		regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+(\w+)`),
		regexp.MustCompile(`^(?:export\s+)?class\s+(\w+)`),
	},
	".py": {
		regexp.MustCompile(`^def\s+(\w+)`),
		regexp.MustCompile(`^class\s+(\w+)`),
	},
	".java": {
		regexp.MustCompile(`(?:public|private|protected)?\s*(?:static\s+)?(?:void|int|char|boolean|long|double|float|String)\s+(\w+)\s*\(`),
		regexp.MustCompile(`(?:public|private|protected)?\s*(?:static\s+)?class\s+(\w+)`),
		regexp.MustCompile(`(?:public|private|protected)?\s*interface\s+(\w+)`),
	},
	".kt": {
		regexp.MustCompile(`fun\s+(\w+)`),
		regexp.MustCompile(`class\s+(\w+)`),
		regexp.MustCompile(`interface\s+(\w+)`),
	},
}

// NewRepoMap creates a new RepoMap for the given project root.
func NewRepoMap(root string) *RepoMap {
	return &RepoMap{
		root:      root,
		fileIndex: make(map[string]FileInfo),
	}
}

// GenerateRepoMap scans the project directory and builds the code structure index.
func (rm *RepoMap) GenerateRepoMap(root string) string {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if root == "" {
		root = rm.root
	}
	rm.root = root

	// Clear old index
	rm.fileIndex = make(map[string]FileInfo)

	// Walk the directory tree
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !sourceExtensions[ext] {
			return nil
		}
		// Skip hidden dirs and build artifacts
		relPath, _ := filepath.Rel(root, path)
		if strings.HasPrefix(relPath, ".") || strings.Contains(relPath, "node_modules") ||
			strings.Contains(relPath, "target") || strings.Contains(relPath, "vendor") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		hash := fmt.Sprintf("%x", sha256.Sum256(data))
		if len(hash) > 12 {
			hash = hash[:12]
		}

		sigs := extractSignatures(string(data), ext)

		rm.fileIndex[relPath] = FileInfo{
			Size:       info.Size(),
			Hash:       hash,
			Signatures: sigs,
		}
		return nil
	})

	return rm.generateOutput()
}

// UpdateFile incrementally updates a single file's index entry.
func (rm *RepoMap) UpdateFile(relPath, content string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	ext := strings.ToLower(filepath.Ext(relPath))
	if !sourceExtensions[ext] {
		return
	}

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	if len(hash) > 12 {
		hash = hash[:12]
	}

	sigs := extractSignatures(content, ext)

	rm.fileIndex[relPath] = FileInfo{
		Size:       int64(len(content)),
		Hash:       hash,
		Signatures: sigs,
	}
}

// RemoveFile removes a file from the index.
func (rm *RepoMap) RemoveFile(relPath string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.fileIndex, relPath)
}

// GetRepoMapSummary returns a compact repo-map summary (limited to ~4KB).
func (rm *RepoMap) GetRepoMapSummary() string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.generateOutput()
}

// generateOutput builds the formatted repo-map string (must hold lock).
func (rm *RepoMap) generateOutput() string {
	if len(rm.fileIndex) == 0 {
		return "[RepoMap] No source files indexed."
	}

	// Sort paths for deterministic output
	paths := make([]string, 0, len(rm.fileIndex))
	for p := range rm.fileIndex {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Repo Map (%d files)\n\n", len(paths)))

	totalSize := 0
	for _, p := range paths {
		fi := rm.fileIndex[p]
		totalSize += int(fi.Size)

		line := fmt.Sprintf("%s (%d bytes, hash=%s):\n", p, fi.Size, fi.Hash)
		if len(line) > 200 {
			line = line[:200] + "...\n"
		}
		sb.WriteString(line)

		for _, sig := range fi.Signatures {
			sb.WriteString(fmt.Sprintf("  %s\n", sig))
		}
		if len(fi.Signatures) == 0 {
			sb.WriteString("  (no signatures)\n")
		}
	}

	// Truncate to 4KB max
	result := sb.String()
	if len(result) > 4096 {
		result = result[:4096] + "\n... [truncated]"
	}

	return result
}

// extractSignatures extracts function/struct/enum signatures from file content.
func extractSignatures(content, ext string) []string {
	patterns, ok := signaturePatterns[ext]
	if !ok {
		return nil
	}

	lines := strings.Split(content, "\n")
	var sigs []string
	seen := make(map[string]bool)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip comments
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		for _, pat := range patterns {
			if m := pat.FindStringSubmatch(line); len(m) > 1 {
				name := m[1]
				if !seen[name] {
					seen[name] = true
					// Clean up the signature line
					sigLine := trimmed
					if len(sigLine) > 80 {
						sigLine = sigLine[:80] + "..."
					}
					sigs = append(sigs, sigLine)
				}
				break
			}
		}
		// Limit to 20 signatures per file
		if len(sigs) >= 20 {
			break
		}
	}

	return sigs
}
