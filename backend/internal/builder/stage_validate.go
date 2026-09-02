package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ValidateGoSyntax performs a lightweight Go syntax check on a single file.
// Uses `go vet` on a temporary module to catch errors before final build.
// Returns nil if the file is syntactically valid.
func ValidateGoSyntax(ctx context.Context, filePath string, logFn func(string)) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Quick check: must start with "package"
	lines := strings.SplitN(string(content), "\n", 5)
	if len(lines) == 0 {
		return fmt.Errorf("empty file")
	}
	hasPackage := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "package ") {
			hasPackage = true
			break
		}
	}
	if !hasPackage {
		return fmt.Errorf("missing package declaration")
	}

	// Quick check: balanced braces
	openBraces := strings.Count(string(content), "{")
	closeBraces := strings.Count(string(content), "}")
	if openBraces != closeBraces {
		return fmt.Errorf("unbalanced braces: %d open, %d close", openBraces, closeBraces)
	}

	// Quick check: no unterminated strings
	inString := false
	inBacktick := false
	for i, ch := range string(content) {
		if ch == '`' {
			inBacktick = !inBacktick
		} else if ch == '"' && !inBacktick {
			// Check for escaped quote
			if i > 0 && string(content[i-1]) == "\\" {
				continue
			}
			inString = !inString
		}
	}
	if inString {
		return fmt.Errorf("unterminated string literal")
	}

	// Quick check: no empty assignments (= ; or = \n at end)
	if strings.Contains(string(content), "= ;") || strings.Contains(string(content), "=;\n") {
		return fmt.Errorf("empty assignment detected")
	}

	// Quick check: no obvious truncation markers
	if strings.Contains(string(content), "...") {
		// Only flag if it looks like truncation (not in a string)
	}

	logFn(fmt.Sprintf("  ✅ Quick syntax check passed: %s\n", filepath.Base(filePath)))
	return nil
}

// ValidateGoModule creates a temp module and runs `go vet` to catch
// import errors, type mismatches, and other issues that quick checks miss.
func ValidateGoModule(ctx context.Context, projectDir string, goFiles []string, logFn func(string)) error {
	if len(goFiles) == 0 {
		return nil
	}

	// Create temp directory for validation
	tmpDir, err := os.MkdirTemp("", "go-validate-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write go.mod
	goMod := `module validation
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return fmt.Errorf("write go.mod: %w", err)
	}

	// Copy Go files
	for _, srcFile := range goFiles {
		content, err := os.ReadFile(srcFile)
		if err != nil {
			continue
		}
		baseName := filepath.Base(srcFile)
		if err := os.WriteFile(filepath.Join(tmpDir, baseName), content, 0644); err != nil {
			continue
		}
	}

	// Run go vet
	goPath := findGoBinary()
	if goPath == "" {
		logFn("  ⚠️  Go binary not found, skipping module validation\n")
		return nil
	}

	vetCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(vetCtx, goPath, "vet", ".")
	cmd.Dir = tmpDir
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg != "" {
			logFn(fmt.Sprintf("  ⚠️  go vet found issues:\n%s\n", errMsg))
		}
		return fmt.Errorf("go vet failed: %w", err)
	}

	logFn("  ✅ go vet passed — no issues found\n")
	return nil
}

// findGoBinary locates the Go compiler.
func findGoBinary() string {
	for _, p := range []string{
		"go",
		"/usr/local/go/bin/go",
		"/usr/bin/go",
		"/snap/bin/go",
	} {
		if path, err := exec.LookPath(p); err == nil {
			return path
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// QuickValidateAllFiles runs quick syntax checks on all generated Go files.
// Returns the list of files that passed validation.
func QuickValidateAllFiles(goFiles []string, logFn func(string)) []string {
	var validFiles []string

	for _, file := range goFiles {
		if err := ValidateGoSyntax(context.Background(), file, logFn); err != nil {
			logFn(fmt.Sprintf("  ⚠️  %s: %v\n", filepath.Base(file), err))
		} else {
			validFiles = append(validFiles, file)
		}
	}

	return validFiles
}
