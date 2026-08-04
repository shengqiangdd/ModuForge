package builder

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ValidationResult holds the result of a project integrity check.
type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Warnings []ValidationIssue `json:"warnings,omitempty"`
	Errors   []ValidationIssue `json:"errors,omitempty"`
}

// ValidationIssue describes a single missing file or config problem.
type ValidationIssue struct {
	File    string `json:"file"`              // config file that references the missing source
	Line    int    `json:"line,omitempty"`    // line number in config
	Message string `json:"message"`           // human-readable description
}

// ValidateProjectIntegrity scans build configuration files in projectDir
// and checks that all referenced source files exist on disk.
// It supports CMakeLists.txt, Android.mk, Makefile, and Cargo.toml.
func ValidateProjectIntegrity(projectDir string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Walk the project directory for build config files
	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		base := strings.ToLower(info.Name())

		switch {
		case base == "cmakelists.txt":
			validateCMakeLists(path, projectDir, result)
		case base == "android.mk" || base == "application.mk":
			validateAndroidMK(path, projectDir, result)
		case base == "makefile" || strings.HasSuffix(base, ".mk"):
			if base != "android.mk" && base != "application.mk" {
				validateMakefile(path, projectDir, result)
			}
		case base == "cargo.toml" && path == filepath.Join(projectDir, "Cargo.toml"):
			validateCargoToml(path, projectDir, result)
		}
		return nil
	})

	if len(result.Errors) > 0 {
		result.Valid = false
	}
	return result
}

// ─── CMakeLists.txt ─────────────────────────────────────────────

// Matches: add_executable(name src1 src2 ...) / add_library(name src1 src2 ...)
var cmakeSourceRe = regexp.MustCompile(`(?i)^\s*(add_executable|add_library)\s*\(([^)]+)\)`)

func validateCMakeLists(cmakePath, projectDir string, result *ValidationResult) {
	relConfig := relativePath(cmakePath, projectDir)
	f, err := os.Open(cmakePath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Collect multi-line directives: if line has an opening paren but no closing, read next lines
		fullLine := line
		if strings.Contains(fullLine, "(") && !strings.Contains(fullLine, ")") {
			for scanner.Scan() {
				lineNo++
				next := strings.TrimSpace(scanner.Text())
				fullLine += " " + next
				if strings.Contains(next, ")") {
					break
				}
			}
		}

		matches := cmakeSourceRe.FindStringSubmatch(fullLine)
		if matches == nil {
			continue
		}

		// Parse source files from the directive
		args := parseCMakeArgs(matches[2])
		if len(args) < 2 {
			continue
		}
		// First arg is target name, rest are sources
		for _, src := range args[1:] {
			src = strings.Trim(src, "\"'")
			// Skip generator expressions, variables, and source generators
			if strings.HasPrefix(src, "$") || strings.HasPrefix(src, "${") || strings.HasPrefix(src, "<") {
				continue
			}
			// Resolve relative to CMakeLists.txt directory
			cmakeDir := filepath.Dir(cmakePath)
			fullSrc := filepath.Join(cmakeDir, src)
			if _, err := os.Stat(fullSrc); os.IsNotExist(err) {
				result.Errors = append(result.Errors, ValidationIssue{
					File:    relConfig,
					Line:    lineNo,
					Message: fmt.Sprintf("源文件不存在: %s (引用自 %s)", src, relConfig),
				})
			}
		}
	}
}

// parseCMakeArgs splits a CMake argument list, handling quoted strings.
func parseCMakeArgs(s string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQuote:
			if c == quoteChar {
				inQuote = false
			} else {
				current.WriteByte(c)
			}
		case c == '"' || c == '\'':
			inQuote = true
			quoteChar = c
		case c == ' ' || c == '\t' || c == '\n':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

// ─── Android.mk ─────────────────────────────────────────────────

var androidMkSourceRe = regexp.MustCompile(`(?i)^\s*LOCAL_SRC_FILES\s*[+:]?=\s*(.*)`)

func validateAndroidMK(mkPath, projectDir string, result *ValidationResult) {
	relConfig := relativePath(mkPath, projectDir)
	f, err := os.Open(mkPath)
	if err != nil {
		return
	}
	defer f.Close()

	mkDir := filepath.Dir(mkPath)
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())

		// Skip comments
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Handle line continuation
		fullLine := line
		for strings.HasSuffix(fullLine, "\\") && scanner.Scan() {
			lineNo++
			fullLine = strings.TrimSuffix(fullLine, "\\") + " " + strings.TrimSpace(scanner.Text())
		}

		matches := androidMkSourceRe.FindStringSubmatch(fullLine)
		if matches == nil {
			continue
		}

		// Split sources by whitespace
		sources := strings.Fields(matches[1])
		for _, src := range sources {
			src = strings.Trim(src, "\"'")
			// Skip variables
			if strings.HasPrefix(src, "$") {
				continue
			}
			fullSrc := filepath.Join(mkDir, src)
			if _, err := os.Stat(fullSrc); os.IsNotExist(err) {
				result.Errors = append(result.Errors, ValidationIssue{
					File:    relConfig,
					Line:    lineNo,
					Message: fmt.Sprintf("源文件不存在: %s (引用自 %s)", src, relConfig),
				})
			}
		}
	}
}

// ─── Makefile (basic) ───────────────────────────────────────────

func validateMakefile(mkPath, projectDir string, result *ValidationResult) {
	// Only do basic checks for Makefiles — too complex for full parsing
	// Just check that file references in obvious patterns exist
	relConfig := relativePath(mkPath, projectDir)
	f, err := os.Open(mkPath)
	if err != nil {
		return
	}
	defer f.Close()

	mkDir := filepath.Dir(mkPath)
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Check for obvious source file references in dependency lines
		// e.g., "target: file1.o file2.o"
		if strings.Contains(line, ".o") || strings.Contains(line, ".c") || strings.Contains(line, ".cpp") {
			parts := strings.Fields(line)
			for _, part := range parts {
				// Skip targets, variables, colons, and special characters
				if strings.HasPrefix(part, "$") || strings.HasSuffix(part, ":") ||
					strings.HasPrefix(part, "-") || part == "" {
					continue
				}
				// Check .o -> .c/.cpp mapping
				if strings.HasSuffix(part, ".o") {
					srcC := strings.TrimSuffix(part, ".o") + ".c"
					srcCpp := strings.TrimSuffix(part, ".o") + ".cpp"
					fullSrcC := filepath.Join(mkDir, srcC)
					fullSrcCpp := filepath.Join(mkDir, srcCpp)
					if _, errC := os.Stat(fullSrcC); os.IsNotExist(errC) {
						if _, errCpp := os.Stat(fullSrcCpp); os.IsNotExist(errCpp) {
							result.Warnings = append(result.Warnings, ValidationIssue{
								File:    relConfig,
								Line:    lineNo,
								Message: fmt.Sprintf("目标文件 %s 对应的源文件不存在 (%s 或 %s)", part, srcC, srcCpp),
							})
						}
					}
				}
			}
		}
	}
}

// ─── Cargo.toml (basic) ─────────────────────────────────────────

func validateCargoToml(tomlPath, projectDir string, result *ValidationResult) {
	relConfig := relativePath(tomlPath, projectDir)
	f, err := os.Open(tomlPath)
	if err != nil {
		return
	}
	defer f.Close()

	tomlDir := filepath.Dir(tomlPath)
	scanner := bufio.NewScanner(f)
	lineNo := 0
	inBinSection := false

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())

		// Track [[bin]] sections
		if strings.HasPrefix(line, "[[bin]]") {
			inBinSection = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inBinSection = false
			continue
		}

		// In [[bin]] sections, check path = "..."
		if inBinSection && strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "path") {
			eqIdx := strings.Index(line, "=")
			if eqIdx < 0 {
				continue
			}
			val := strings.TrimSpace(line[eqIdx+1:])
			val = strings.Trim(val, "\"'")
			if val == "" {
				continue
			}
			fullSrc := filepath.Join(tomlDir, val)
			if _, err := os.Stat(fullSrc); os.IsNotExist(err) {
				result.Errors = append(result.Errors, ValidationIssue{
					File:    relConfig,
					Line:    lineNo,
					Message: fmt.Sprintf("源文件不存在: %s (引用自 %s)", val, relConfig),
				})
			}
		}
	}
}

// ─── Helpers ────────────────────────────────────────────────────

func relativePath(path, base string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return filepath.Base(path)
	}
	return rel
}
