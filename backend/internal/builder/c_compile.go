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

// ndkPath returns the Android NDK path from environment or default location.
func ndkPath() string {
	if p := os.Getenv("ANDROID_NDK"); p != "" {
		return p
	}
	return "/opt/android-ndk"
}

// NDKAvailable checks if the Android NDK is installed.
// Supports both full NDK and trimmed NDK layouts.
func NDKAvailable() bool {
	ndk := ndkPath()

	// Trimmed layout: /opt/android-ndk/bin/aarch64-linux-android*-clang
	trimmedBin := filepath.Join(ndk, "bin")
	if entries, err := os.ReadDir(trimmedBin); err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "aarch64-linux-android") && strings.HasSuffix(e.Name(), "-clang") {
				return true
			}
		}
	}

	// Full NDK layout: /opt/android-ndk/toolchains/llvm/prebuilt/linux-x86_64/bin/...
	fullBin := filepath.Join(ndk, "toolchains", "llvm", "prebuilt", "linux-x86_64", "bin")
	if entries, err := os.ReadDir(fullBin); err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "aarch64-linux-android") && strings.HasSuffix(e.Name(), "-clang") {
				return true
			}
		}
	}

	return false
}

// ndkBinDir returns the directory containing clang binaries, handling both layouts.
func ndkBinDir() string {
	ndk := ndkPath()
	// Trimmed layout first
	trimmed := filepath.Join(ndk, "bin")
	if _, err := os.Stat(filepath.Join(trimmed, "llvm-strip")); err == nil {
		return trimmed
	}
	// Full NDK layout
	return filepath.Join(ndk, "toolchains", "llvm", "prebuilt", "linux-x86_64", "bin")
}

// DetectCFiles checks if the project contains C/C++ source files.
func (b *Builder) DetectCFiles(projectDir string) []string {
	var cFiles []string
	allowedExts := map[string]bool{".c": true, ".cpp": true, ".cc": true, ".cxx": true, ".h": true, ".hpp": true}
	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if allowedExts[ext] {
			cFiles = append(cFiles, path)
		}
		return nil
	})
	return cFiles
}

// CompileCFilesArch compiles C/C++ source files using the Android NDK with arch support.
func (b *Builder) CompileCFilesArch(ctx context.Context, projectDir string, arch string, incr *IncrementalResult, logFn func(string)) (*CompileResult, error) {
	result := &CompileResult{}

	if !NDKAvailable() {
		logFn("  ⚠️  Android NDK not found, skipping C/C++ compilation\n")
		return result, nil
	}

	cFiles := b.DetectCFiles(projectDir)
	if len(cFiles) == 0 {
		return result, nil
	}

	// Separate header files from source files
	var srcFiles []string
	for _, f := range cFiles {
		ext := strings.ToLower(filepath.Ext(f))
		if ext != ".h" && ext != ".hpp" {
			srcFiles = append(srcFiles, f)
		}
	}

	if len(srcFiles) == 0 {
		return result, nil
	}

	// Group source files by directory
	dirFiles := make(map[string][]string)
	for _, f := range srcFiles {
		dir := filepath.Dir(f)
		dirFiles[dir] = append(dirFiles[dir], f)
	}

	archInfo, _ := GetArchInfo(arch)
	ndkBin := ndkBinDir()

	var targetTriple string
	switch archInfo.Goarch {
	case "arm64":
		targetTriple = "aarch64-linux-android"
	case "arm":
		targetTriple = "armv7a-linux-androideabi"
	default:
		targetTriple = "aarch64-linux-android"
	}

	// API level 21 is minimum for arm64
	apiLevel := "21"
	if archInfo.Goarch == "arm" {
		apiLevel = "19"
	}

	// Find the right clang binary
	// 1. Try exact name (full NDK layout): aarch64-linux-android-clang
	// 2. Try with API level (trimmed layout): aarch64-linux-android21-clang
	// 3. Scan directory for any matching prefix, pick lowest API level
	clangBin := ""
	clangAPILevel := apiLevel

	exactPath := filepath.Join(ndkBin, targetTriple+"-clang")
	if _, err := os.Stat(exactPath); err == nil {
		clangBin = exactPath
	} else {
		// Trimmed layout: scan for targetTriple + "<digits>-clang"
		prefix := targetTriple + "-"
		bestLevel := -1
		entries, err := os.ReadDir(ndkBin)
		if err == nil {
			for _, e := range entries {
				name := e.Name()
				if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, "-clang") || strings.HasSuffix(name, "-clang++") {
					continue
				}
				// Extract API level digits between prefix and "-clang"
				mid := name[len(prefix) : len(name)-len("-clang")]
				level := 0
				for _, ch := range mid {
					if ch >= '0' && ch <= '9' {
						level = level*10 + int(ch-'0')
					} else {
						level = -1
						break
					}
				}
				if level < 0 {
					continue
				}
				// Pick the lowest API level >= target minimum
				minLevel := 21
				if archInfo.Goarch == "arm" {
					minLevel = 19
				}
				if level >= minLevel && (bestLevel < 0 || level < bestLevel) {
					bestLevel = level
					clangBin = filepath.Join(ndkBin, name)
				}
			}
		}
		if bestLevel > 0 {
			clangAPILevel = fmt.Sprintf("%d", bestLevel)
		}
	}

	if clangBin == "" {
		logFn("  ⚠️  NDK clang not found for target: " + targetTriple + "\n")
		return result, nil
	}

	logFn(fmt.Sprintf("  📎 Using NDK clang: %s (API %s)\n", filepath.Base(clangBin), clangAPILevel))

	binDir := filepath.Join(projectDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return nil, fmt.Errorf("create bin: %w", err)
	}

	for dir, files := range dirFiles {
		// Determine binary name from directory
		name := filepath.Base(dir)
		if name == "." || name == "src" || name == "c" {
			name = "native"
		}
		binPath := filepath.Join(binDir, name)

		// Check incremental: skip if no changes
		if incr != nil && !incr.NeedsRebuild {
			changed := false
			for _, cf := range incr.ChangedFiles {
				for _, sf := range files {
					if strings.HasPrefix(cf, sf) || cf == sf {
						changed = true
						break
					}
				}
				if changed {
					break
				}
			}
			if !changed {
				logCompileSkip(logFn, name+".c", "no changes")
				continue
			}
		}

		logFn(fmt.Sprintf("  🔨 Compiling C/C++ → bin/%s (%s)...\n", name, targetTriple))

		// Build compile command
		args := []string{
			"--target=" + targetTriple + clangAPILevel,
			"-O2",
			"-s", // strip symbols
			"-o", binPath,
		}

		// Add include paths for headers in the same directory
		for _, f := range files {
			ext := strings.ToLower(filepath.Ext(f))
			if ext == ".h" || ext == ".hpp" {
				args = append(args, "-I", filepath.Dir(f))
			}
		}

		// Add source files
		for _, f := range files {
			args = append(args, f)
		}

		compileCtx, compileCancel := context.WithTimeout(ctx, 60*time.Second)
		cmd := exec.CommandContext(compileCtx, clangBin, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		compileCancel()
		if err != nil {
			if compileCtx.Err() == context.DeadlineExceeded {
				logFn(fmt.Sprintf("  ❌ C/C++ compile timed out (60s): %s\n", name))
				continue
			}
			logFn(fmt.Sprintf("  ❌ C/C++ compile failed: %s\n%s\n", err, string(out)))
			continue
		}

		if info, err := os.Stat(binPath); err != nil || info.Size() == 0 {
			logFn(fmt.Sprintf("  ❌ Compiled binary is empty: %s\n", binPath))
			continue
		}

		logFn(fmt.Sprintf("  ✅ %s.c (%d KB)\n", name, fileSizeKB(binPath)))
		result.Recompiled = append(result.Recompiled, "bin/"+name)
		result.CacheMisses++
	}

	return result, nil
}
