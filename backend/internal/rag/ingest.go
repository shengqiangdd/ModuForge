package rag

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	ChunkSize    = 512
	ChunkOverlap = 50
	VectordbDir  = "data/vectordb"
)

// IngestKnowledge scans baseDir for example files and _patch directories,
// chunks them, computes TF-IDF vectors, and persists to JSON.
func IngestKnowledge(baseDir string) (*KnowledgeBase, error) {
	kb := NewKnowledgeBase()

	// Collect all scannable files
	var files []string

	// 1. Scan docs/examples/
	examplesDir := filepath.Join(baseDir, "docs", "examples")
	if entries, err := os.ReadDir(examplesDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if isScannable(e.Name()) {
				files = append(files, filepath.Join(examplesDir, e.Name()))
			}
		}
	}

	// 2. Scan all _patch directories recursively
	filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		if info.Name() == "_patch" {
			filepath.Walk(path, func(p string, fi os.FileInfo, e error) error {
				if e != nil || fi.IsDir() {
					return nil
				}
				if isScannable(fi.Name()) {
					files = append(files, p)
				}
				return nil
			})
		}
		return nil
	})

	// 3. Scan the builder prompts (multi_stage.go, auto_fix.go) for embedded knowledge
	builderDir := filepath.Join(baseDir, "backend", "internal", "builder")
	for _, name := range []string{"multi_stage.go", "auto_fix.go", "auto_fix_v2.go"} {
		p := filepath.Join(builderDir, name)
		if _, err := os.Stat(p); err == nil {
			files = append(files, p)
		}
	}

	// 4. If no files found, generate embedded sample knowledge
	if len(files) == 0 {
		files = generateSampleKnowledge(baseDir)
	}

	// Process each file
	for _, filePath := range files {
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		content := string(data)
		relPath, _ := filepath.Rel(baseDir, filePath)

		chunks := chunkText(content, ChunkSize, ChunkOverlap)
		for i, chunk := range chunks {
			id := fmt.Sprintf("%s:%d", relPath, i)
			kb.Chunks = append(kb.Chunks, CodeChunk{
				ID:      id,
				Source:  relPath,
				Content: chunk,
				Metadata: map[string]string{
					"file": relPath,
					"part": fmt.Sprintf("%d/%d", i+1, len(chunks)),
				},
			})
		}
	}

	if len(kb.Chunks) == 0 {
		return kb, nil
	}

	// Compute IDF across all chunks
	kb.IDF = computeIDF(kb.Chunks)

	// Compute TF-IDF vectors for each chunk
	for i := range kb.Chunks {
		kb.Chunks[i].Vector = computeTFIDF(kb.Chunks[i].Content, kb.IDF)
	}

	return kb, nil
}

// Save persists the knowledge base to JSON.
func (kb *KnowledgeBase) Save(dir string) error {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create vectordb dir: %w", err)
	}

	data, err := json.MarshalIndent(kb, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal kb: %w", err)
	}

	path := filepath.Join(dir, "knowledge_base.json")
	return os.WriteFile(path, data, 0644)
}

// LoadKnowledge reads the knowledge base from JSON.
func LoadKnowledge(dir string) (*KnowledgeBase, error) {
	path := filepath.Join(dir, "knowledge_base.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read kb: %w", err)
	}

	kb := NewKnowledgeBase()
	if err := json.Unmarshal(data, kb); err != nil {
		return nil, fmt.Errorf("unmarshal kb: %w", err)
	}

	return kb, nil
}

// chunkText splits text into overlapping chunks.
func chunkText(text string, size, overlap int) []string {
	if len(text) <= size {
		return []string{text}
	}

	runes := []rune(text)
	var chunks []string
	step := size - overlap

	for i := 0; i < len(runes); i += step {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
		if end == len(runes) {
			break
		}
	}

	return chunks
}

// tokenize splits text into lowercase tokens.
func tokenize(text string) []string {
	// Split on non-alphanumeric characters
	re := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	raw := re.Split(strings.ToLower(text), -1)

	// Filter empty and very short tokens
	var tokens []string
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "is": true, "it": true,
		"to": true, "of": true, "in": true, "for": true, "on": true,
		"with": true, "as": true, "by": true, "at": true, "and": true,
		"or": true, "if": true, "not": true, "no": true, "do": true,
		"this": true, "that": true, "be": true, "has": true, "had": true,
	}

	for _, t := range raw {
		if len(t) < 2 || stopWords[t] {
			continue
		}
		tokens = append(tokens, t)
	}
	return tokens
}

// computeIDF calculates inverse document frequency for all terms.
func computeIDF(chunks []CodeChunk) map[string]float64 {
	df := make(map[string]float64)
	n := float64(len(chunks))

	for _, chunk := range chunks {
		// Unique terms in this chunk
		seen := make(map[string]bool)
		for _, token := range tokenize(chunk.Content) {
			if !seen[token] {
				df[token]++
				seen[token] = true
			}
		}
	}

	idf := make(map[string]float64)
	for term, freq := range df {
		idf[term] = math.Log((n+1)/(freq+1)) + 1
	}

	return idf
}

// computeTFIDF computes TF-IDF vector for a text chunk.
func computeTFIDF(text string, idf map[string]float64) map[string]float64 {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return nil
	}

	// Term frequency
	tf := make(map[string]float64)
	for _, t := range tokens {
		tf[t]++
	}

	// Normalize TF
	maxTF := 0.0
	for _, v := range tf {
		if v > maxTF {
			maxTF = v
		}
	}

	vector := make(map[string]float64)
	for term, freq := range tf {
		// Augmented TF: 0.5 + 0.5 * (freq / maxFreq)
		normalizedTF := 0.5 + 0.5*(freq/maxTF)
		if idfVal, ok := idf[term]; ok {
			vector[term] = normalizedTF * idfVal
		}
	}

	return vector
}

// isScannable checks if a filename is a type we want to ingest.
func isScannable(name string) bool {
	scannable := []string{".sh", ".prop", ".go", ".c", ".md", ".txt"}
	for _, ext := range scannable {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	special := []string{"module.prop", "service.sh", "customize.sh", "uninstall.sh", "build.sh"}
	for _, s := range special {
		if name == s {
			return true
		}
	}
	return false
}

// generateSampleKnowledge creates embedded example files for the knowledge base.
// Used when no external examples are found.
func generateSampleKnowledge(baseDir string) []string {
	examplesDir := filepath.Join(baseDir, "docs", "examples")
	os.MkdirAll(examplesDir, 0755)

	samples := map[string]string{
		"service_daemon.sh": `#!/system/bin/sh
# Magisk module service daemon example
MODDIR=${0%/*}
LOGFILE=/data/local/tmp/module_daemon.log

log_msg() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "${LOGFILE}"
}

while true; do
  # Read battery level
  BATTERY=$(cat /sys/class/power_supply/battery/capacity 2>/dev/null)
  if [ -n "${BATTERY}" ]; then
    log_msg "Battery: ${BATTERY}%"
    if [ "${BATTERY}" -lt 15 ]; then
      log_msg "Low battery warning"
    fi
  fi
  sleep 300
done`,

		"customize_example.sh": `#!/system/bin/sh
# Magisk module installer example
MODID=battery_monitor
MODVER=1.0

ui_print "========================================="
ui_print "  Battery Monitor Module v${MODVER}"
ui_print "========================================="

# Check Android version
SDK=$(getprop ro.build.version.sdk)
if [ "${SDK}" -lt 26 ]; then
  ui_print "  Error: Requires Android 8.0+ (API 26+)"
  abort
fi

# Create directories
mkdir -p "${MODPATH}/system/bin"
mkdir -p "${MODPATH}/system/etc"

# Set permissions
set_perm_recursive "${MODPATH}" 0 0 0755 0644
set_perm "${MODPATH}/system/bin/androsmart" 0 0 0755

ui_print "  Installation complete!"
ui_print "========================================="`,

		"module_prop_example.txt": `id=battery_monitor
name=Battery Monitor
version=1.0
versionCode=1
author=ModuForge
description=Monitors battery level and logs charging status

# module.prop key=value format
# - No quotes around values
# - One key=value per line
# - id: lowercase alphanumeric with underscores`,

		"go_daemon_example.go": `package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	moduleDir := "/data/adb/modules/battery_monitor"
	logFile := moduleDir + "/daemon.log"

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log: %v\\n", err)
		os.Exit(1)
	}
	defer f.Close()

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	fmt.Fprintln(f, "Daemon started")

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(f, "Daemon shutting down")
			return
		case <-ticker.C:
			data, err := os.ReadFile("/sys/class/power_supply/battery/capacity")
			if err != nil {
				continue
			}
			capStr := strings.TrimSpace(string(data))
			capacity, err := strconv.Atoi(capStr)
			if err != nil {
				continue
			}
			fmt.Fprintf(f, "Battery: %d%%\\n", capacity)
		}
	}
}`,

		"uninstall_safe.sh": `#!/system/bin/sh
# Safe uninstall template for Magisk modules
# NEVER use "rm -rf /" — only remove module-specific files

MODDIR=${0%/*}
MODID=$(basename "${MODDIR}")

# Remove module data directory
rm -rf "/data/adb/modules/${MODID}"

# Remove any created symlinks
rm -f /system/bin/androsmart

# Remove log files
rm -f /data/local/tmp/module_daemon.log

echo "Module ${MODID} uninstalled safely"`,

		"service_sh_example.sh": `#!/system/bin/sh
# service.sh - runs on every boot
MODDIR=${0%/*}

# Wait for system to fully boot
while [ "$(getprop sys.boot_completed)" != "1" ]; do
  sleep 1
done

# Start daemon
nohup "${MODDIR}/system/bin/androsmart" >> /dev/null 2>&1 &
echo $! > "${MODDIR}/daemon.pid"`,

		"go_compile_patterns.md": `# Go Compilation Patterns for Magisk Modules

## Cross-compilation command
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ./bin/androsmart ./src/

## Common errors and fixes

### 1. "declared but not used"
Fix: Remove unused variable or use _ = varName

### 2. "cannot use _ as value"
Fix: Replace _ with nil for interfaces, 0 for ints, "" for strings

### 3. "undefined reference"
Fix: Ensure all imported packages are used, check go.mod

### 4. Syntax error: unexpected
Fix: Check for unclosed braces, unterminated strings, missing commas

## Best practices
- Use signal.NotifyContext for graceful shutdown (Go 1.16+)
- Use log/slog for structured logging (Go 1.21+)
- Read sysfs with os.ReadFile + strconv.Atoi
- All config in /data/adb/modules/<id>/
- Cross-compile: GOOS=android GOARCH=arm64 CGO_ENABLED=0`,

		"shell_patterns.md": `# Shell Script Patterns for Magisk Modules

## Critical rules
1. ALWAYS use ${VAR} syntax, NEVER bare $VAR
2. All variables in double quotes: "${VAR}"
3. sleep takes INTEGER seconds only: sleep 5
4. Test syntax: [ "$x" = "yes" ] (spaces inside brackets)
5. Shebang: #!/system/bin/sh

## Module.prop format
key=value pairs, NO quotes around values

## Common patterns

### Reading battery level
BATTERY=$(cat /sys/class/power_supply/battery/capacity 2>/dev/null)

### Checking Android version
SDK=$(getprop ro.build.version.sdk)

### Waiting for boot
while [ "$(getprop sys.boot_completed)" != "1" ]; do
  sleep 1
done

### Safe file operations
mkdir -p "${MODPATH}/system/bin"
set_perm_recursive "${MODPATH}" 0 0 0755 0644`,

		"error_recovery_patterns.md": `# Error Recovery Patterns

## Auto-fix strategy
1. Parse compilation errors (file:line:col: message)
2. Categorize: syntax, link, type, other
3. Try post-processing fixes (fast, no LLM):
   - "declared but not used" -> prefix with _ =
   - "cannot use _ as value" -> replace with nil
   - "syntax error: unexpected" -> add missing braces
4. Fall back to LLM-based repair (full file context)

## Build retry logic
- Max 3 attempts per compilation
- Each attempt: fix -> rebuild -> check
- Timeout per attempt: 30 seconds
- Preserve error context across retries

## Go build environment
- GOOS=android GOARCH=arm64
- CGO_ENABLED=0 (no cgo)
- -trimpath -ldflags="-s -w"
- GOPROXY=off (no network in sandbox)`,
	}

	var paths []string
	for name, content := range samples {
		path := filepath.Join(examplesDir, name)
		os.WriteFile(path, []byte(content), 0644)
		paths = append(paths, path)
	}

	sort.Strings(paths)
	return paths
}
