package skills

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// compileRust compiles Rust code in the project.
func (s *BuildModuleSkill) compileRust(projectPath string, cargoDir string) string {
	cargoPath := findExec("cargo")
	if cargoPath == "" {
		return "  ⚠️ cargo not found, skipping Rust compilation\n"
	}

	// Try cross-compilation for Android first
	// Find NDK clang - trimmed NDK uses /opt/android-ndk/bin/ directly
	ndkClang := ""
	candidates := []string{
		"/opt/android-ndk/bin/aarch64-linux-android21-clang",
		"/opt/android-ndk/bin/aarch64-linux-android-clang",
		filepath.Join(os.Getenv("ANDROID_NDK"), "bin", "aarch64-linux-android21-clang"),
		filepath.Join(os.Getenv("ANDROID_NDK"), "bin", "aarch64-linux-android-clang"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			ndkClang = c
			break
		}
	}

	// Create .cargo/config.toml with correct linker if NDK found
	cargoConfig := filepath.Join(cargoDir, ".cargo", "config.toml")
	if ndkClang != "" {
		ndkDir := filepath.Dir(ndkClang)
		os.MkdirAll(filepath.Dir(cargoConfig), 0755)
		configContent := fmt.Sprintf(`[target.aarch64-linux-android]
linker = "%s"
`, filepath.Join(ndkDir, "aarch64-linux-android21-clang"))
		os.WriteFile(cargoConfig, []byte(configContent), 0644)
	}

	// Use context with timeout to prevent hanging compilations
	ctx, cancel := context.WithTimeout(context.Background(), compileTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "cargo", "build", "--release", "--target", "aarch64-linux-android")
	cmd.Dir = cargoDir
	cmd.Env = append(os.Environ(),
		"CARGO_INCREMENTAL=1",
		"CC_aarch64_linux_android="+ndkClang,
		"CXX_aarch64_linux_android="+strings.Replace(ndkClang, "-clang", "-clang++", 1),
		"CARGO_TARGET_AARCH64_LINUX_ANDROID_LINKER="+ndkClang,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)

		// Cross-compilation failed - classify the failure type
		isLinkerError := strings.Contains(outputStr, "linker") || strings.Contains(outputStr, "ld: ") || strings.Contains(outputStr, "cannot find -l")
		isNDKMissing := ndkClang == "" || strings.Contains(outputStr, "not found")

		if isLinkerError || isNDKMissing {
			// Phase A: Linker/NDK issue - try host compilation to validate code quality
			ctxHost, cancelHost := context.WithTimeout(context.Background(), compileTimeout)
			defer cancelHost()
			cmdHost := exec.CommandContext(ctxHost, "cargo", "build", "--release")
			cmdHost.Dir = cargoDir
			outputHost, errHost := cmdHost.CombinedOutput()

			if errHost != nil {
				// Host compilation also failed - real code errors
				hostErrors := parseCompileErrors("rust", string(outputHost))
				return fmt.Sprintf(
					"  ❌ Rust build failed (code errors, not cross-compile):\n"+
						"  Cross-compile issue: %s\n"+
						"  Code errors found:\n%s\n"+
						"  💡 Fix the code errors above, then rebuild. The cross-compile linker issue is secondary.\n",
					classifyNDKError(outputStr),
					formatCompileErrors(hostErrors))
			}
			// Host compilation succeeded - code is valid, cross-compile is secondary
			return fmt.Sprintf(
				"  ✅ Rust code validated (host target OK)\n"+
					"  ⚠️ Cross-compile skipped: %s\n"+
					"  💡 Code compiles successfully for host. Cross-compilation will work when NDK is properly configured.\n",
				classifyNDKError(outputStr))
		}

		// Real compilation errors (syntax, type, etc.)
		compileErrors := parseCompileErrors("rust", outputStr)
		return fmt.Sprintf(
			"  ❌ Rust build failed:\n%s\n"+
				"  💡 Found %d error(s). Analyze the errors above and use edit_file to fix them.\n",
			outputStr, len(compileErrors))
	}

	// Copy compiled binary to system/bin/
	// Dynamic: read [[bin]] name from Cargo.toml, fallback to "androst"
	binName := "androst"
	cargoToml := filepath.Join(cargoDir, "Cargo.toml")
	if data, err := os.ReadFile(cargoToml); err == nil {
		content := string(data)
		// Try [[bin]] name = "xxx"
		if m := regexp.MustCompile(`(?m)^\[\[bin\]\]\s*\n\s*name\s*=\s*"([^"]+)"`).FindStringSubmatch(content); len(m) > 1 {
			binName = m[1]
		} else if m := regexp.MustCompile(`(?m)^name\s*=\s*"([^"]+)"`).FindStringSubmatch(content); len(m) > 1 {
			binName = m[1]
		}
	}
	// Also copy ALL binaries from target/release to system/bin/
	releaseDir := filepath.Join(cargoDir, "target", "aarch64-linux-android", "release")
	binDst := filepath.Join(projectPath, "system", "bin")
	os.MkdirAll(binDst, 0755)
	if entries, err := os.ReadDir(releaseDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil || info.Mode().Perm()&0111 == 0 {
				continue // skip non-executable files
			}
			src := filepath.Join(releaseDir, e.Name())
			dst := filepath.Join(binDst, e.Name())
			if data, err := os.ReadFile(src); err == nil {
				os.WriteFile(dst, data, 0755)
			}
		}
	}
	// Fallback: if specific binName not in system/bin, copy from target
	specificBin := filepath.Join(releaseDir, binName)
	if _, err := os.Stat(specificBin); err == nil {
		dst := filepath.Join(binDst, binName)
		if data, err := os.ReadFile(specificBin); err == nil {
			os.WriteFile(dst, data, 0755)
		}
	}

	return "  ✅ Rust build succeeded (binaries copied to system/bin/)\n"
}

// compileCpp compiles C/C++ code in the project.
func (s *BuildModuleSkill) compileCpp(projectPath string) string {
	// 查找 Android.mk 或 CMakeLists.txt
	if s.hasFile(projectPath, "Android.mk") {
		// Android.mk 需要 ndk-build
		ndkBuild := findExec("ndk-build")
		if ndkBuild == "" {
			ndkBase := os.Getenv("ANDROID_NDK")
			if ndkBase != "" {
				ndkBuild = filepath.Join(ndkBase, "ndk-build")
				if _, err := os.Stat(ndkBuild); err != nil {
					ndkBuild = ""
				}
			}
		}
		if ndkBuild == "" {
			return "  ⚠️ ndk-build not found, skipping Android.mk compilation\n"
		}

		cmd := exec.Command(ndkBuild, "-C", projectPath, "NDK_PROJECT_PATH="+projectPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("  ❌ ndk-build failed:\n%s\n", string(output))
		}
		return "  ✅ ndk-build succeeded\n"
	}

	// Fallback: 直接用 g++/clang++ 做语法检查
	compiler := findCppCompiler()
	if compiler == "" {
		return "  ⚠️ No C++ compiler found, skipping compilation check\n"
	}

	var srcFiles []string
	filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".cpp" || ext == ".c" || ext == ".cc" {
			srcFiles = append(srcFiles, path)
		}
		return nil
	})

	if len(srcFiles) == 0 {
		return "  ℹ️ No C/C++ source files found\n"
	}

	args := append([]string{"-std=c++17", "-fsyntax-only", "-Wall"}, srcFiles...)
	cmd := exec.Command(compiler, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("  ❌ C++ syntax check failed:\n%s\n", string(output))
	}

	// Try to compile with NDK for Android ARM64
	ndkClangpp := ""
	candidates := []string{
		"/opt/android-ndk/bin/aarch64-linux-android21-clang++",
		"/opt/android-ndk/bin/aarch64-linux-android-clang++",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			ndkClangpp = c
			break
		}
	}

	if ndkClangpp != "" {
		// projectPath is like .../1785249992652501794-1864, need .../1785249992652501794-1864/system/bin/
		binDst := filepath.Join(projectPath, "system", "bin", "andromon")
		os.MkdirAll(filepath.Dir(binDst), 0755)

		compileArgs := append([]string{"-std=c++17", "-O2", "-o", binDst}, srcFiles...)
		cmdCompile := exec.Command(ndkClangpp, compileArgs...)
		cmdCompile.Env = append(os.Environ(),
			"SYSROOT=/opt/android-ndk/sysroot",
		)
		if output, err := cmdCompile.CombinedOutput(); err != nil {
			return fmt.Sprintf("  ⚠️ C++ NDK compile failed: %s\n", string(output))
		}
		return "  ✅ C/C++ compiled and installed\n"
	}

	return "  ✅ C/C++ syntax check passed\n"
}

// compileGo compiles Go code in the project.
func (s *BuildModuleSkill) compileGo(projectPath string) string {
	// Use /usr/local/go/bin/go if available (container environment)
	goBin := "/usr/local/go/bin/go"
	if _, err := os.Stat(goBin); os.IsNotExist(err) {
		goBin = findExec("go")
	}
	if goBin == "" {
		return "  ⚠️ go not found, skipping Go compilation\n"
	}

	// Find the directory containing go.mod (may be in a subdirectory like src/go/)
	goModDir := projectPath
	if !s.hasFile(projectPath, "go.mod") {
		filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || goModDir != projectPath {
				return nil
			}
			if info.Name() == "go.mod" {
				goModDir = filepath.Dir(path)
			}
			return nil
		})
	}

	// projectPath is always the project root — use it directly instead of
	// computing from goModDir (which breaks when goModDir nesting depth varies)
	binDst := filepath.Join(projectPath, "system", "bin", "androwui")
	os.MkdirAll(filepath.Dir(binDst), 0755)

	// Use context with timeout to prevent hanging compilations
	ctx, cancel := context.WithTimeout(context.Background(), compileTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, "build", "-o", binDst, ".")
	cmd.Dir = goModDir

	// Auto-detect CGO: check for .c files or cgo imports
	needsCGO := false
	filepath.Walk(goModDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || needsCGO {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".c" || ext == ".h" {
			needsCGO = true
			return filepath.SkipDir
		}
		return nil
	})

	cgoEnabled := "0"
	if needsCGO {
		cgoEnabled = "1"
	}
	cmd.Env = append(os.Environ(),
		"GOOS=android",
		"GOARCH=arm64",
		"CGO_ENABLED="+cgoEnabled,
	)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Sprintf(
			"  ❌ Go build timed out after %v\n"+
				"  💡 The build is taking too long. Check for infinite loops or very large dependencies.\n",
			compileTimeout)
	}
	if err != nil {
		outputStr := string(output)
		compileErrors := parseCompileErrors("go", outputStr)

		// Format errors with per-error fix hints
		errorDetails := formatCompileErrors(compileErrors)
		summary := generateBuildErrorSummary(compileErrors)

		return fmt.Sprintf(
			"  ❌ Go build failed (dir=%s):\n%s\n"+
				"  %s\n"+
				"%s\n"+
				"  Use syntax_checker or edit_file to fix the errors above, then rebuild.\n",
			goModDir, outputStr, summary, errorDetails)
	}
	return fmt.Sprintf("  ✅ Go build succeeded (dir=%s)\n", goModDir)
}

// findExec finds an executable in PATH.
func findExec(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return ""
}

// findCppCompiler finds a C++ compiler.
func findCppCompiler() string {
	for _, name := range []string{"g++", "clang++", "gcc", "cc"} {
		if p := findExec(name); p != "" {
			return p
		}
	}
	// NDK
	ndkBase := os.Getenv("ANDROID_NDK")
	if ndkBase != "" {
		candidates := []string{
			filepath.Join(ndkBase, "bin", "clang++"),
			filepath.Join(ndkBase, "bin", "clang"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}
	return ""
}
