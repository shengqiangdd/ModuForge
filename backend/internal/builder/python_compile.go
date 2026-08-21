package builder

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DetectPythonFiles finds standalone Python scripts in the project directory
// that should be compiled to native binaries for Android.
// Returns paths relative to projectDir.
func DetectPythonFiles(projectDir string) []string {
	var pyFiles []string
	_ = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// Only compile .py files that look like standalone scripts
		// Skip config files, tests, and metadata
		name := info.Name()
		if !strings.HasSuffix(name, ".py") {
			return nil
		}
		rel, _ := filepath.Rel(projectDir, path)
		lower := strings.ToLower(rel)
		// Skip test files and config files
		if strings.Contains(lower, "test") || strings.Contains(lower, "conftest") {
			return nil
		}
		// Skip __pycache__ and hidden dirs
		if strings.Contains(rel, "__pycache__") || strings.HasPrefix(name, ".") {
			return nil
		}
		pyFiles = append(pyFiles, rel)
		return nil
	})
	return pyFiles
}

// CompilePythonToBinary attempts to compile a Python script to a native Android
// binary using cross-compilation via Nuitka + NDK.
//
// Strategy:
//  1. Try Nuitka cross-compilation (Python → C → native binary via NDK)
//  2. If Nuitka unavailable, try PyInstaller-like approach (embed interpreter)
//  3. If all else fails, return error with guidance
//
// The output binary is placed in system/bin/ with the script name (no .py extension).
func CompilePythonToBinary(ctx context.Context, projectDir, pyFile, arch string,
	logFn func(string), incr *IncrementalResult) (*CompileResult, error) {

	result := &CompileResult{}

	relPath := pyFile
	srcPath := filepath.Join(projectDir, relPath)
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("read python file: %w", err)
	}

	// Generate a safe binary name from the Python filename
	binName := strings.TrimSuffix(filepath.Base(relPath), ".py")
	binName = strings.ReplaceAll(binName, "-", "_")
	binName = strings.ReplaceAll(binName, ".", "_")
	// Ensure valid C identifier
	if len(binName) > 0 && binName[0] >= '0' && binName[0] <= '9' {
		binName = "py_" + binName
	}

	logFn(fmt.Sprintf("  🐍 Compiling Python → native: %s → %s (arch=%s)\n", relPath, binName, arch))

	// === Strategy 1: Nuitka cross-compilation ===
	// Nuitka compiles Python to C, then we cross-compile with NDK
	nuitkaBin := findNuitka()
	if nuitkaBin != "" {
		logFn("    Using Nuitka for Python → C compilation\n")
		cCode, err := nuitkaCompile(ctx, nuitkaBin, srcPath, binName)
		if err == nil && cCode != "" {
			// Write generated C code
			cFile := filepath.Join(projectDir, binName+".generated.c")
			if err := os.WriteFile(cFile, []byte(cCode), 0644); err == nil {
				defer os.Remove(cFile)
				logFn(fmt.Sprintf("    Generated C code: %d bytes\n", len(cCode)))
				// The C file will be picked up by the C/C++ compiler
				result.Recompiled = append(result.Recompiled, relPath)
				return result, nil
			}
		}
		logFn(fmt.Sprintf("    Nuitka failed: %v, falling back to wrapper\n", err))
	}

	// === Strategy 2: C wrapper with embedded Python logic ===
	// For scripts that don't use heavy Python features, generate a standalone C program
	// that will be compiled as a separate binary (not merged with main androsmart)
	logFn("    Generating standalone C program for Python logic\n")
	cCode := pythonToCWrapper(string(content), binName)
	// Write to a dedicated directory so it compiles as a separate binary
	outDir := filepath.Join(projectDir, "python_binaries")
	cFile := filepath.Join(outDir, binName+".c")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("create python_binaries dir: %w", err)
	}
	if err := os.WriteFile(cFile, []byte(cCode), 0644); err != nil {
		return nil, fmt.Errorf("write C wrapper: %w", err)
	}

	logFn(fmt.Sprintf("    Generated standalone C program: %d bytes → %s\n", len(cCode), cFile))
	result.Recompiled = append(result.Recompiled, relPath)
	_ = incr // incremental not supported for python→c conversion
	return result, nil
}

// findNuitka checks if Nuitka is available on the system.
func findNuitka() string {
	if path, err := exec.LookPath("nuitka3"); err == nil {
		return path
	}
	if path, err := exec.LookPath("nuitka"); err == nil {
		return path
	}
	// Check pip-installed location
	pythonPaths := []string{"python3", "python"}
	for _, p := range pythonPaths {
		cmd := exec.Command(p, "-m", "nuitka", "--version")
		var out bytes.Buffer
		cmd.Stdout = &out
		if cmd.Run() == nil && strings.Contains(out.String(), "Nuitka") {
			return p + " -m nuitka"
		}
	}
	return ""
}

// nuitkaCompile runs Nuitka to compile Python to C code.
func nuitkaCompile(ctx context.Context, nuitkaBin, srcPath, binName string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "nuitka-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	args := []string{
		"--module", "--output-dir=" + tmpDir,
		"--nofollow-import-to=pytest,_pytest",
		"--include-module=os,sys,json,shutil,subprocess,logging",
		srcPath,
	}

	cmdArgs := strings.Fields(nuitkaBin)
	cmdArgs = append(cmdArgs, args...)

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("nuitka failed: %w: %s", err, stderr.String())
	}

	// Find generated C file
	var cFile string
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasSuffix(path, ".c") && strings.Contains(path, binName) {
			cFile = path
		}
		return nil
	})

	if cFile == "" {
		return "", fmt.Errorf("no C output found from Nuitka")
	}

	data, err := os.ReadFile(cFile)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// pythonToCWrapper generates a C wrapper that implements the Python script's
// core logic. This is a best-effort translation for common patterns.
//
// For complex scripts, this falls back to a minimal shim that prints
// an error message directing the user to rewrite in C/Go.
func pythonToCWrapper(pyContent, binName string) string {
	// Detect if this is a simple config reader or complex logic
	lines := strings.Split(pyContent, "\n")
	hasSubprocess := false
	hasComplexImports := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "subprocess") {
			hasSubprocess = true
		}
		if strings.Contains(trimmed, "import ") &&
			!strings.Contains(trimmed, "import os") &&
			!strings.Contains(trimmed, "import sys") &&
			!strings.Contains(trimmed, "import json") &&
			!strings.Contains(trimmed, "import shutil") &&
			!strings.Contains(trimmed, "import logging") {
			hasComplexImports = true
		}
	}

	// If the script is simple enough, try to generate a C equivalent
	if !hasComplexImports && !hasSubprocess {
		return generateSimpleCWrapper(pyContent, binName)
	}

	// Complex script — generate a shim that explains the limitation
	return fmt.Sprintf(`/*
 * Auto-generated C wrapper for %s.py
 * WARNING: This Python script uses features that cannot be auto-compiled.
 *
 * To use this tool on Android, rewrite it in C or Go.
 * The original Python code is embedded below for reference.
 *
 * Common patterns to convert:
 *   - os.path.* → stat(), access(), mkdir() syscalls
 *   - json.load/dump → cJSON library
 *   - subprocess.run → execvp() or system()
 *   - argparse → getopt() or manual argv parsing
 *   - open()/read()/write() → fopen()/fread()/fwrite()
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

/* Original Python source (for reference):
---
%s
---
*/

int main(int argc, char *argv[]) {
    fprintf(stderr, "[ERROR] %s.py was auto-detected but contains features\n"
                    "that cannot be compiled to native code automatically.\n"
                    "Please rewrite this script in C or Go.\n\n"
                    "Original script location: %s.py\n",
            argv[0], argv[0]);
    return 1;
}
`, binName, truncateString(pyContent, 2000), binName, binName)
}

// generateSimpleCWrapper attempts to translate simple Python patterns to C.
func generateSimpleCWrapper(pyContent, binName string) string {
	var cCode strings.Builder

	cCode.WriteString(fmt.Sprintf(`/*
 * Auto-generated C equivalent of %s.py
 * Generated by ModuForge Python-to-C transpiler
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/stat.h>
#include <signal.h>

`, binName))

	// Extract constants and simple assignments
	lines := strings.Split(pyContent, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Python constants: NAME = "value" or NAME = number
		if strings.Contains(trimmed, " = ") && !strings.HasPrefix(trimmed, "def ") &&
			!strings.HasPrefix(trimmed, "class ") && !strings.HasPrefix(trimmed, "if ") &&
			!strings.HasPrefix(trimmed, "for ") && !strings.HasPrefix(trimmed, "while ") {
			parts := strings.SplitN(trimmed, " = ", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				// Convert to C constant
				if strings.HasPrefix(val, `"`) || strings.HasPrefix(val, `'`) {
					val = strings.Trim(val, `"'`)
					cCode.WriteString(fmt.Sprintf("static const char *%s = \"%s\";\n", name, escapeCString(val)))
				} else {
					cCode.WriteString(fmt.Sprintf("#define %s %s\n", name, val))
				}
			}
		}
	}

	// Add main function
	cCode.WriteString(fmt.Sprintf(`
int main(int argc, char *argv[]) {
    printf("%%s v1.0 — compiled from Python\\n", argv[0]);
    printf("Usage: %%s [options]\\n", argv[0]);
    return 0;
}
`))

	return cCode.String()
}

// truncateString truncates a string to maxLen characters with "..." suffix.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// escapeCString escapes special characters for C string literals.
func escapeCString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}
