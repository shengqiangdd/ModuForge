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

// ndkSysrootPath returns the NDK sysroot path containing headers and libraries.
// Handles both trimmed layout (Dockerfile copies toolchain root to /opt/android-ndk)
// and full NDK layout (toolchains/llvm/prebuilt/linux-x86_64/sysroot).
func ndkSysrootPath() string {
	ndk := ndkPath()
	// Trimmed layout: /opt/android-ndk/sysroot/ (has include/ and lib/ directly)
	trimmed := filepath.Join(ndk, "sysroot")
	if _, err := os.Stat(filepath.Join(trimmed, "include")); err == nil {
		return trimmed
	}
	// Full NDK layout
	full := filepath.Join(ndk, "toolchains", "llvm", "prebuilt", "linux-x86_64", "sysroot")
	if _, err := os.Stat(full); err == nil {
		return full
	}
	// Fallback
	return trimmed
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

// DetectCFilesInDir finds C/C++ source files in a specific directory (non-recursive).
func (b *Builder) DetectCFilesInDir(dir string) []string {
	var cFiles []string
	allowedExts := map[string]bool{".c": true, ".cpp": true, ".cc": true, ".cxx": true}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if allowedExts[ext] {
			cFiles = append(cFiles, entry.Name())
		}
	}
	return cFiles
}

// DetectProjectBinaryName looks for a primary source file to derive the output binary name.
// Priority: main.c/main.cpp > top-level .c/.cpp in system/bin/ or daemon/src/ > directory name.
func DetectProjectBinaryName(projectDir string, srcFiles []string) string {
	// Check for main.c / main.cpp
	for _, f := range srcFiles {
		base := strings.ToLower(filepath.Base(f))
		if base == "main.c" || base == "main.cpp" || base == "main.cc" || base == "main.cxx" {
			return "main"
		}
	}

	// Check for common daemon names in project structure
	candidates := []string{
		"androboostd", "androsmart", "daemon", "core", "native",
	}
	for _, name := range candidates {
		for _, f := range srcFiles {
			base := filepath.Base(f)
			if strings.HasPrefix(base, name) {
				return name
			}
		}
	}

	// Fallback: use the first source file's directory name
	if len(srcFiles) > 0 {
		dir := filepath.Base(filepath.Dir(srcFiles[0]))
		if dir != "." && dir != "src" && dir != "c" && dir != "bin" {
			return dir
		}
	}

	return "native"
}

// isHeaderFile returns true for .h and .hpp files.
func isHeaderFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".h" || ext == ".hpp"
}

// CollectIncludeDirs finds all unique directories containing source or header files.
// This ensures that #include "file.h" works for files in the same directory.
func CollectIncludeDirs(srcFiles []string) []string {
	seen := make(map[string]bool)
	var dirs []string
	for _, f := range srcFiles {
		dir := filepath.Dir(f)
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// CompileCFilesArch compiles C/C++ source files into a single executable using Android NDK.
//
// Key changes from previous version:
//   - Removed -shared flag: produces executables, not .so libraries
//   - All source files linked into ONE binary (default name: "androsmart" or derived from project)
//   - API level raised to 26 (Android 8.0 minimum for module compatibility)
//   - Static linking of C++ stdlib for portability across Android 8-17
//   - Single output: bin/<project_name>
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
		if !isHeaderFile(f) {
			srcFiles = append(srcFiles, f)
		}
	}

	if len(srcFiles) == 0 {
		return result, nil
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

	// Android 8.0 = API 26 (minimum for Magisk/KernelSU module compatibility)
	// This also enables better NDK support and modern libc features
	apiLevel := "26"
	if archInfo.Goarch == "arm" {
		apiLevel = "26" // API 26 works for both arm and arm64
	}

	// Find the right clang binary
	clangBin := ""
	clangAPILevel := apiLevel

	// 1. Try exact name (full NDK layout): aarch64-linux-android-clang
	exactPath := filepath.Join(ndkBin, targetTriple+"-clang")
	if _, err := os.Stat(exactPath); err == nil {
		clangBin = exactPath
	} else {
		// 2. Trimmed layout: scan for targetTriple + "<digits>-clang"
		bestLevel := -1
		entries, err := os.ReadDir(ndkBin)
		if err == nil {
			for _, e := range entries {
				name := e.Name()
				if !strings.HasSuffix(name, "-clang") || strings.HasSuffix(name, "-clang++") {
					continue
				}

				var level int
				var valid bool

				// Pattern 1: targetTriple + "<digits>-clang" (e.g., "aarch64-linux-android26-clang")
				prefix1 := targetTriple
				if strings.HasPrefix(name, prefix1) {
					mid := name[len(prefix1) : len(name)-len("-clang")]
					level, valid = parseAPILevel(mid)
				}

				// Pattern 2: targetTriple + "-<digits>-clang" (e.g., "aarch64-linux-android-26-clang")
				if !valid {
					prefix2 := targetTriple + "-"
					if strings.HasPrefix(name, prefix2) {
						mid := name[len(prefix2) : len(name)-len("-clang")]
						level, valid = parseAPILevel(mid)
					}
				}

				if !valid || level < 0 {
					continue
				}

				// Pick the lowest API level >= 26 (Android 8.0)
				if level >= 26 && (bestLevel < 0 || level < bestLevel) {
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

	binDir := filepath.Join(projectDir, "system", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return nil, fmt.Errorf("create system/bin: %w", err)
	}

	// Output binary is always "androsmart" — consistent name for Magisk/KernelSU modules
	binaryName := "androsmart"
	binPath := filepath.Join(binDir, binaryName)

	// Separate Python-generated C files from main C source files
	// Python-generated files are in python_binaries/ directory and need separate compilation
	pythonBinDir := filepath.Join(projectDir, "python_binaries")
	var mainSrcFiles []string
	var pythonCFiles []string
	for _, f := range srcFiles {
		if strings.HasPrefix(f, pythonBinDir+string(os.PathSeparator)) || strings.HasPrefix(f, pythonBinDir+"/") {
			pythonCFiles = append(pythonCFiles, f)
		} else {
			mainSrcFiles = append(mainSrcFiles, f)
		}
	}

	// Check incremental: skip if no source files changed
	if incr != nil && !incr.NeedsRebuild {
		changed := false
		for _, cf := range incr.ChangedFiles {
			for _, sf := range mainSrcFiles {
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
			logCompileSkip(logFn, binaryName, "no changes")
			return result, nil
		}
	}

	logFn(fmt.Sprintf("  🔨 Compiling C/C++ → system/bin/%s (%s, API %s)...\n", binaryName, targetTriple, clangAPILevel))

	// ─── Strategy 1: CMake build (preferred when CMakeLists.txt exists) ───
	cmakeLists := filepath.Join(projectDir, "CMakeLists.txt")
	if _, err := os.Stat(cmakeLists); err == nil {
		if ok := b.tryCmakeBuild(ctx, projectDir, binPath, targetTriple, clangBin, clangAPILevel, logFn); ok {
			if info, err := os.Stat(binPath); err == nil && info.Size() > 0 {
				sizeKB := fileSizeKB(binPath)
				logFn(fmt.Sprintf("  ✅ system/bin/%s (%d KB) — cmake static build\n", binaryName, sizeKB))
				result.Recompiled = append(result.Recompiled, "system/bin/"+binaryName)
				result.CacheMisses++
				return result, nil
			}
			logFn("  ⚠️  CMake build produced no binary, falling back to direct compilation\n")
		}
	}

	// ─── Strategy 2: Direct clang — all sources into ONE executable ──────
	includeDirs := CollectIncludeDirs(cFiles)
	ndkSysroot := ndkSysrootPath()

	args := []string{
		"--target=" + targetTriple + clangAPILevel,

		// NDK sysroot for headers and libraries
		"--sysroot=" + ndkSysroot,

		// Optimization & stripping
		"-O2",
		"-s", // strip symbols for smaller binary

		// Linker library search paths
		"-L" + filepath.Join(ndkSysroot, "lib", targetTriple),
		"-L" + filepath.Join(ndkSysroot, "lib", targetTriple, clangAPILevel),

		// Output
		"-o", binPath,

		// C++ standard library (NDK libc++ static) + Android system libs
		// NOTE: Do NOT use -static globally — Android system libs (liblog, libandroid)
		// only have .so versions in the NDK. Link libc++_static.a for portability.
		"-lc++_static",
		"-lc++abi",
		"-llog",
		"-landroid",
		"-lm",
		"-lc",
	}

	// Add include paths for header files
	for _, dir := range includeDirs {
		args = append(args, "-I", dir)
	}

	// Also add common include directories for the project
	for _, subDir := range []string{"include", "common", "daemon/include", "src/include"} {
		absDir := filepath.Join(projectDir, subDir)
		if info, err := os.Stat(absDir); err == nil && info.IsDir() {
			args = append(args, "-I", absDir)
		}
	}

	// Add main source files (Python-generated files compiled separately)
	args = append(args, mainSrcFiles...)

	compileCtx, compileCancel := context.WithTimeout(ctx, 120*time.Second)
	cmd := exec.CommandContext(compileCtx, clangBin, args...)
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	compileCancel()

	if err != nil {
		if compileCtx.Err() == context.DeadlineExceeded {
			logFn(fmt.Sprintf("  ❌ C/C++ compile timed out (120s): %s\n", binaryName))
			return result, nil
		}
		logFn(fmt.Sprintf("  ❌ C/C++ compile failed: %s\n%s\n", err, string(out)))
		return result, nil
	}

	if info, err := os.Stat(binPath); err != nil || info.Size() == 0 {
		logFn(fmt.Sprintf("  ❌ Compiled binary is empty: %s\n", binPath))
		return result, nil
	}

	sizeKB := fileSizeKB(binPath)
	logFn(fmt.Sprintf("  ✅ system/bin/%s (%d KB) — statically linked executable\n", binaryName, sizeKB))

	if sizeKB > 500 {
		logFn(fmt.Sprintf("  ⚠️  Binary size %d KB exceeds 500 KB target\n", sizeKB))
	}

	result.Recompiled = append(result.Recompiled, "system/bin/"+binaryName)
	result.CacheMisses++

	// Compile Python-generated C files as separate binaries
	for _, pyCFile := range pythonCFiles {
		pyBinName := strings.TrimSuffix(filepath.Base(pyCFile), ".c")
		pyBinName = strings.TrimSuffix(pyBinName, ".cpp")
		pyBinPath := filepath.Join(binDir, pyBinName)

		logFn(fmt.Sprintf("  🔨 Compiling Python binary: %s → system/bin/%s\n", filepath.Base(pyCFile), pyBinName))

		pyArgs := []string{
			"--target=" + targetTriple + clangAPILevel,
			"--sysroot=" + ndkSysroot,
			"-static",
			"-O2",
			"-Wall",
			"-o", pyBinPath,
			pyCFile,
		}

		pyCtx, pyCancel := context.WithTimeout(ctx, 60*time.Second)
		pyCmd := exec.CommandContext(pyCtx, clangBin, pyArgs...)
		pyCmd.Dir = projectDir
		pyOut, pyErr := pyCmd.CombinedOutput()
		pyCancel()

		if pyErr != nil {
			logFn(fmt.Sprintf("  ⚠️  Python binary compilation failed: %s\n", string(pyOut)))
			continue
		}

		if info, statErr := os.Stat(pyBinPath); statErr == nil && info.Size() > 0 {
			sizeKB := fileSizeKB(pyBinPath)
			logFn(fmt.Sprintf("  ✅ system/bin/%s (%d KB) — Python → native binary\n", pyBinName, sizeKB))
			result.Recompiled = append(result.Recompiled, "system/bin/"+pyBinName)
			result.CacheMisses++
		}
	}

	return result, nil
}

// parseAPILevel extracts API level digits from a string.
// Returns the level and true if valid, or -1 and false if invalid.
func parseAPILevel(s string) (int, bool) {
	if s == "" {
		return -1, false
	}
	level := 0
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			level = level*10 + int(ch-'0')
		} else {
			return -1, false
		}
	}
	return level, level > 0
}

// tryCmakeBuild attempts to build the project using CMake with the Android NDK toolchain.
// Returns true if cmake was found and the build succeeded.
func (b *Builder) tryCmakeBuild(ctx context.Context, projectDir, binPath, targetTriple, clangBin, apiLevel string, logFn func(string)) bool {
	// Check if cmake is available
	cmakePath, err := exec.LookPath("cmake")
	if err != nil {
		return false
	}
	logFn("  📦 Found CMakeLists.txt, attempting cmake build...\n")

	// Find NDK root from clang binary
	ndkRoot := ndkPath()

	// Build directory
	buildDir := filepath.Join(projectDir, "build")
	os.MkdirAll(buildDir, 0755)

	// Map target triple to CMake ANDROID_ABI
	androidABI := "arm64-v8a"
	if targetTriple == "armv7a-linux-androideabi" {
		androidABI = "armeabi-v7a"
	}

	// Step 1: cmake configure
	ndkToolchain := filepath.Join(ndkRoot, "build", "cmake", "android.toolchain.cmake")
	configureArgs := []string{
		"-S", projectDir,
		"-B", buildDir,
		"-DCMAKE_TOOLCHAIN_FILE=" + ndkToolchain,
		"-DANDROID_ABI=" + androidABI,
		"-DANDROID_PLATFORM=android-" + apiLevel,
		"-DANDROID_STL=c++_static", // NDK static libc++
		"-DCMAKE_BUILD_TYPE=Release",
	}

	// Only set toolchain file if it exists; let cmake find NDK via ANDROID_NDK env
	if _, err := os.Stat(ndkToolchain); err != nil {
		// No bundled toolchain, rely on env
		configureArgs = configureArgs[:len(configureArgs)-1] // remove last flag
	}

	cfgCtx, cfgCancel := context.WithTimeout(ctx, 60*time.Second)
	cfgCmd := exec.CommandContext(cfgCtx, cmakePath, configureArgs...)
	cfgCmd.Dir = projectDir
	cfgOut, cfgErr := cfgCmd.CombinedOutput()
	cfgCancel()

	if cfgErr != nil {
		logFn(fmt.Sprintf("  ⚠️  CMake configure failed: %s\n%s\n", cfgErr, string(cfgOut)))
		return false
	}

	// Step 2: cmake build
	buildArgs := []string{
		"--build", buildDir,
		"--config", "Release",
		"-j", "4",
	}

	buildCtx, buildCancel := context.WithTimeout(ctx, 120*time.Second)
	buildCmd := exec.CommandContext(buildCtx, cmakePath, buildArgs...)
	buildCmd.Dir = projectDir
	buildOut, buildErr := buildCmd.CombinedOutput()
	buildCancel()

	if buildErr != nil {
		logFn(fmt.Sprintf("  ⚠️  CMake build failed: %s\n%s\n", buildErr, string(buildOut)))
		return false
	}

	// Step 3: Find the built binary and copy it to bin/androsmart
	var foundBinary string
	filepath.Walk(buildDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// Look for ELF executables (not .so, not CMake artifacts)
		if !strings.HasSuffix(path, ".so") && !strings.HasSuffix(path, ".a") &&
			!strings.HasSuffix(path, ".cmake") && !strings.Contains(path, "CMakeFiles") {
			// Check if it's executable-like (no extension or common binary extensions)
			ext := strings.ToLower(filepath.Ext(path))
			if ext == "" || ext == ".out" || ext == ".elf" {
				foundBinary = path
			}
		}
		return nil
	})

	if foundBinary == "" {
		logFn("  ⚠️  CMake build succeeded but no binary found in build directory\n")
		return false
	}

	// Copy to target binPath using cp to preserve permissions
	cpCtx, cpCancel := context.WithTimeout(ctx, 10*time.Second)
	cpCmd := exec.CommandContext(cpCtx, "cp", foundBinary, binPath)
	cpOut, cpErr := cpCmd.CombinedOutput()
	cpCancel()
	if cpErr != nil {
		logFn(fmt.Sprintf("  ⚠️  Failed to copy cmake binary: %s %s\n", cpErr, string(cpOut)))
		return false
	}

	os.Chmod(binPath, 0755)
	return true
}
