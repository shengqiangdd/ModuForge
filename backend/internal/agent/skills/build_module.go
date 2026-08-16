package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"github.com/moduforge/backend/internal/builder"
	"github.com/moduforge/backend/internal/agent/registry"
	"github.com/moduforge/backend/internal/storage"
)

type BuildModuleSkill struct {
	projectPath string
	db          *sql.DB
	storage     storage.StorageAdapter // optional S3 storage backend
}

func NewBuildModuleSkillWithDB(projectPath string, db *sql.DB) *BuildModuleSkill {
	return &BuildModuleSkill{projectPath: projectPath, db: db}
}

// WithStorage sets the S3 storage adapter. When set, files are loaded from S3.
func (s *BuildModuleSkill) WithStorage(st storage.StorageAdapter) *BuildModuleSkill {
	s.storage = st
	return s
}

// compileTimeout is the maximum time allowed for a single compilation command.
const compileTimeout = 5 * time.Minute

// sourceInfo holds the results of a single-pass source detection walk.
type sourceInfo struct {
	hasCargo bool
	cargoDir string
	hasCpp   bool
	hasGo    bool
	goModDir string
}

// BuildResult holds structured build information for the runner to consume.
type BuildResult struct {
	BuildReady    bool              `json:"build_ready"`
	Success       bool              `json:"success"`
	SourceResults map[string]string `json:"source_results"` // "rust"|"cpp"|"go" -> result message
	Errors        []CompileError    `json:"errors"`
	Warnings      []string          `json:"warnings"`
}

// CompileError holds parsed compilation error details.
type CompileError struct {
	SourceType string `json:"source_type"` // "rust", "cpp", "go"
	File       string `json:"file"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Message    string `json:"message"`
	ErrorType  string `json:"error_type"` // "syntax", "undefined", "type_mismatch", "linker", "missing_import", "timeout", "unknown"
}

// parseCompileErrors extracts structured error information from compiler output.
func parseCompileErrors(sourceType, output string) []CompileError {
	var errors []CompileError
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		ce := CompileError{SourceType: sourceType}

		switch sourceType {
		case "rust":
			// Rust error format: error[E0XXX]: message\n  --> file:line:col
			if m := regexp.MustCompile(`error\[(E\d+)\]:\s*(.+)`).FindStringSubmatch(line); len(m) > 2 {
				ce.ErrorType = classifyRustError(m[1])
				ce.Message = m[2]
			} else if m := regexp.MustCompile(`--> (.+):(\d+):(\d+)`).FindStringSubmatch(line); len(m) > 3 {
				ce.File = m[1]
				ce.Line = atoi(m[2])
				ce.Column = atoi(m[3])
			} else if strings.HasPrefix(line, "error") {
				ce.Message = strings.TrimPrefix(line, "error: ")
				ce.ErrorType = classifyRustError("")
			}

		case "go":
			// Go error format: file:line:col: message
			if m := regexp.MustCompile(`(.+):(\d+):(\d+):\s*(.+)`).FindStringSubmatch(line); len(m) > 4 {
				ce.File = m[1]
				ce.Line = atoi(m[2])
				ce.Column = atoi(m[3])
				ce.Message = m[4]
				ce.ErrorType = classifyGoError(m[4])
			} else if strings.Contains(line, "undefined") || strings.Contains(line, "undeclared") {
				ce.Message = line
				ce.ErrorType = "undefined"
			}

		case "cpp":
			// C++ error format: file:line:col: error: message
			if m := regexp.MustCompile(`(.+):(\d+):(\d+):\s*(?:error|warning):\s*(.+)`).FindStringSubmatch(line); len(m) > 4 {
				ce.File = m[1]
				ce.Line = atoi(m[2])
				ce.Column = atoi(m[3])
				ce.Message = m[4]
				ce.ErrorType = classifyCppError(m[4])
			} else if strings.Contains(line, "error:") {
				ce.Message = line
				ce.ErrorType = "unknown"
			}
		}

		if ce.Message != "" {
			errors = append(errors, ce)
		}
	}
	return errors
}

// atoi is a simple string-to-int converter for error parsing.
func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}

// classifyRustError categorizes Rust compiler error codes.
func classifyRustError(code string) string {
	switch {
	case strings.HasPrefix(code, "E0"):
		if strings.Contains(code, "0412") || strings.Contains(code, "0432") || strings.Contains(code, "0433") {
			return "missing_import"
		}
		if strings.Contains(code, "0596") || strings.Contains(code, "0599") || strings.Contains(code, "0609") || strings.Contains(code, "0603") {
			return "undefined"
		}
		if strings.Contains(code, "0308") || strings.Contains(code, "0305") || strings.Contains(code, "0382") {
			return "type_mismatch"
		}
		if strings.Contains(code, "0001") || strings.Contains(code, "0002") || strings.Contains(code, "0003") || strings.Contains(code, "0004") {
			return "syntax"
		}
		if strings.Contains(code, "0277") || strings.Contains(code, "0282") {
			return "type_mismatch"
		}
		if strings.Contains(code, "0583") || strings.Contains(code, "0584") {
			return "undefined"
		}
	case strings.Contains(code, "linker"):
		return "linker"
	}
	return "unknown"
}

// classifyGoError categorizes Go compiler error messages.
func classifyGoError(msg string) string {
	switch {
	case strings.Contains(msg, "undefined") || strings.Contains(msg, "undeclared"):
		return "undefined"
	case strings.Contains(msg, "cannot use") || strings.Contains(msg, "type mismatch"):
		return "type_mismatch"
	case strings.Contains(msg, "syntax error"):
		return "syntax"
	case strings.Contains(msg, "cannot find package") || strings.Contains(msg, "no required module"):
		return "missing_import"
	case strings.Contains(msg, "imported and not used"):
		return "unused_import"
	case strings.Contains(msg, "not enough arguments") || strings.Contains(msg, "too many arguments"):
		return "type_mismatch"
	case strings.Contains(msg, "cannot call") || strings.Contains(msg, "has no field") || strings.Contains(msg, "has no method"):
		return "undefined"
	case strings.Contains(msg, "missing return"):
		return "syntax"
	case strings.Contains(msg, "expected") || strings.Contains(msg, "unexpected"):
		return "syntax"
	case strings.Contains(msg, "go.mod"):
		return "missing_import"
	case strings.Contains(msg, "module declared multiple times") || strings.Contains(msg, "non-Go module"):
		return "missing_import"
	}
	return "unknown"
}

// classifyCppError categorizes C++ compiler error messages.
func classifyCppError(msg string) string {
	switch {
	case strings.Contains(msg, "undeclared") || strings.Contains(msg, "was not declared"):
		return "undefined"
	case strings.Contains(msg, "no matching function") || strings.Contains(msg, "cannot convert"):
		return "type_mismatch"
	case strings.Contains(msg, "expected") || strings.Contains(msg, "unexpected"):
		return "syntax"
	case strings.Contains(msg, "fatal error: ") || strings.Contains(msg, "No such file"):
		return "missing_import"
	case strings.Contains(msg, "undefined reference"):
		return "linker"
	}
	return "unknown"
}

// resolvePath 根据 project_id 解析实际文件路径（与 write_file 保持一致）
func (s *BuildModuleSkill) Name() string {
	return "build_module"
}

func (s *BuildModuleSkill) Description() string {
	return `Build the module: validate → compile → package.
Input: {} or {"project_id": "..."}.
Compiles source code (Rust/C/C++/Go), validates structure, then creates ZIP.
Returns build log with success/failure status.`
}

func (s *BuildModuleSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	projectID, _ := input["project_id"].(string)
	projectPath := ResolveProjectPath(s.db, s.projectPath, projectID)
	enableIncremental, _ := input["incremental"].(bool)

	var log strings.Builder
	log.WriteString("🔨 Build Module\n")
	log.WriteString(fmt.Sprintf("📂 Project: %s\n\n", projectPath))

	// Emit progress marker for frontend
	log.WriteString("[BUILD_PROGRESS] phase=init status=starting\n")

	// ========== Phase 0: Incremental Build Check ==========
	if enableIncremental {
		log.WriteString("\n── Phase 0: Incremental Check ──\n")
		log.WriteString("[BUILD_PROGRESS] phase=incremental status=checking\n")
		incResult := builder.CheckIncremental(projectPath, "arm64")
		if !incResult.NeedsRebuild {
			log.WriteString("  ✅ No changes detected since last build — skipping compilation\n")
			log.WriteString("  ℹ️ Use incremental=false to force full rebuild\n")
			log.WriteString("[BUILD_PROGRESS] phase=incremental status=skipped reason=no_changes\n")
			// Still need to package
		} else {
			log.WriteString(fmt.Sprintf("  📝 Changes: %s\n", incResult.Reason))
			if len(incResult.ChangedFiles) > 0 {
				log.WriteString(fmt.Sprintf("     Changed: %d files\n", len(incResult.ChangedFiles)))
			}
			if len(incResult.NewFiles) > 0 {
				log.WriteString(fmt.Sprintf("     New: %d files\n", len(incResult.NewFiles)))
			}
			if len(incResult.RemovedFiles) > 0 {
				log.WriteString(fmt.Sprintf("     Removed: %d files\n", len(incResult.RemovedFiles)))
			}
			log.WriteString("[BUILD_PROGRESS] phase=incremental status=changes_detected\n")
		}
	}

	// ========== Phase 1: Structure Validation ==========
	log.WriteString("\n── Phase 1: Structure Validation ──\n")
	log.WriteString("[BUILD_PROGRESS] phase=validate status=starting\n")
	requiredFiles := []string{"module.prop", "META-INF/com/google/android/update-binary"}
	var missingFiles []string
	for _, f := range requiredFiles {
		path := filepath.Join(projectPath, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missingFiles = append(missingFiles, f)
		} else {
			log.WriteString(fmt.Sprintf("  ✅ %s\n", f))
		}
	}

	if len(missingFiles) > 0 {
		for _, f := range missingFiles {
			log.WriteString(fmt.Sprintf("  ❌ %s — missing\n", f))
		}
		log.WriteString(fmt.Sprintf("\n❌ Build failed: %d required files missing\n", len(missingFiles)))
		log.WriteString("[BUILD_PROGRESS] phase=validate status=failed\n")
		return log.String(), fmt.Errorf("missing required files: %s", strings.Join(missingFiles, ", "))
	}

	// Check recommended files
	if _, err := os.Stat(filepath.Join(projectPath, "customize.sh")); os.IsNotExist(err) {
		log.WriteString("  ⚠️ customize.sh not found (recommended)\n")
	} else {
		log.WriteString("  ✅ customize.sh\n")
	}
	log.WriteString("[BUILD_PROGRESS] phase=validate status=done\n")

	// ========== Phase 1.5: Sync source files from DB/S3 to disk ==========
	log.WriteString("\n── Syncing source files to disk... ──\n")
	if s.db != nil && projectID != "" {
		rows, err := s.db.Query(`SELECT path FROM project_files WHERE project_id=?`, projectID)
		if err == nil {
			defer rows.Close()
			synced := 0
			for rows.Next() {
				var path string
				if err := rows.Scan(&path); err != nil {
					continue
				}
				content, err := readFileContent(context.Background(), s.storage, s.db, projectID, path)
				if err != nil {
					continue
				}
				fullPath := filepath.Join(projectPath, path)
				// DB/S3 is source of truth - always overwrite disk with content
				dir := filepath.Dir(fullPath)
				if err := os.MkdirAll(dir, 0755); err != nil {
					continue
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					continue
				}
				synced++
			}
			if synced > 0 {
				log.WriteString(fmt.Sprintf("  ✅ Synced %d files from %s to disk\n", synced, storageLabel(s.storage)))
			} else {
				log.WriteString("  ✅ All files already present on disk\n")
			}
		}
	}

	// ========== Phase 2: Source Compilation ==========
	log.WriteString("\n── Phase 2: Source Compilation ──\n")
	log.WriteString("[BUILD_PROGRESS] phase=compile status=starting\n")
	compileResult := s.compileSources(projectPath)
	log.WriteString(compileResult.log)
	if compileResult.buildSuccess {
		log.WriteString("[BUILD_PROGRESS] phase=compile status=done\n")
	} else {
		log.WriteString(fmt.Sprintf("[BUILD_PROGRESS] phase=compile status=failed errors=%d\n", len(compileResult.errors)))
	}

	// ========== Phase 3: Shell Script Validation ==========
	log.WriteString("\n── Phase 3: Shell Script Validation ──\n")
	log.WriteString("[BUILD_PROGRESS] phase=shellcheck status=starting\n")
	shellValid := s.validateShellScripts(projectPath)
	if shellValid {
		log.WriteString("  ✅ All shell scripts passed syntax check\n")
	} else {
		log.WriteString("  ⚠️ Some shell scripts have syntax issues (see above)\n")
	}
	log.WriteString("[BUILD_PROGRESS] phase=shellcheck status=done\n")

	// ========== Phase 4: Package ==========
	log.WriteString("\n── Phase 4: Package ──\n")
	log.WriteString("[BUILD_PROGRESS] phase=package status=starting\n")
	outputZIP := filepath.Join(filepath.Dir(projectPath), "output.zip")
	if err := s.removeExisting(outputZIP); err != nil {
		log.WriteString(fmt.Sprintf("  ⚠️ Could not remove old output: %v\n", err))
	}

	// Use progress-aware zip
	zipErr := builder.ZipDirExcludingWithProgress(projectPath, outputZIP, builder.ModuleExcludePatterns,
		func(current, total int, currentFile string) {
			if current%10 == 0 || current == total {
				pct := 0
				if total > 0 {
					pct = current * 100 / total
				}
				log.WriteString(fmt.Sprintf("[BUILD_PROGRESS] phase=package status=zipping current=%d total=%d pct=%d file=%s\n",
					current, total, pct, currentFile))
			}
		})

	if zipErr != nil {
		log.WriteString(fmt.Sprintf("  ❌ ZIP creation failed: %v\n", zipErr))
		log.WriteString("[BUILD_PROGRESS] phase=package status=failed\n")
		return log.String(), fmt.Errorf("build failed: %v", zipErr)
	}

	// Get zip size
	if info, err := os.Stat(outputZIP); err == nil {
		sizeMB := float64(info.Size()) / 1024 / 1024
		log.WriteString(fmt.Sprintf("  ✅ %s (%.1f MB)\n", outputZIP, sizeMB))
	} else {
		log.WriteString(fmt.Sprintf("  ✅ %s\n", outputZIP))
	}
	log.WriteString("[BUILD_PROGRESS] phase=package status=done\n")

	// ========== Update Build Cache ==========
	if compileResult.buildSuccess {
		if err := builder.UpdateBuildCacheAfterBuild(projectPath, "arm64", "android"); err != nil {
			log.WriteString(fmt.Sprintf("  ⚠️ Could not update build cache: %v\n", err))
		}
	}

	// ========== Build Result ==========
	buildReady := compileResult.buildSuccess
	result := BuildResult{
		BuildReady:    buildReady,
		Success:       buildReady && len(missingFiles) == 0,
		SourceResults: compileResult.sourceResults,
		Errors:        compileResult.errors,
		Warnings:      compileResult.warnings,
	}

	resultJSON, _ := json.Marshal(result)
	log.WriteString(fmt.Sprintf("\n[BUILD_RESULT] %s\n", string(resultJSON)))

	if buildReady {
		log.WriteString("\n✅ Build complete! Module is ready for packaging and testing.\n")
	} else {
		log.WriteString("\n⚠️ Build completed with errors. Please fix the issues above.\n")
	}

	return log.String(), nil
}

// detectSources performs a single filepath.Walk to detect all source types
// in the project, replacing multiple separate walks for Rust/C++/Go detection.
func (s *BuildModuleSkill) detectSources(projectPath string) sourceInfo {
	var info sourceInfo
	_ = filepath.Walk(projectPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		name := fi.Name()
		ext := strings.ToLower(filepath.Ext(path))

		// Rust: look for Cargo.toml
		if !info.hasCargo && name == "Cargo.toml" {
			info.hasCargo = true
			info.cargoDir = filepath.Dir(path)
		}

		// C/C++: source files (.cpp, .c, .cc, .cxx)
		if !info.hasCpp {
			switch ext {
			case ".cpp", ".c", ".cc", ".cxx":
				info.hasCpp = true
			}
		}
		// C/C++: build manifests at project root (Android.mk, CMakeLists.txt)
		if !info.hasCpp && (name == "Android.mk" || name == "CMakeLists.txt") {
			if rel, err := filepath.Rel(projectPath, filepath.Dir(path)); err == nil && rel == "." {
				info.hasCpp = true
			}
		}

		// Go: go.mod or .go files
		if !info.hasGo {
			if name == "go.mod" {
				info.hasGo = true
				info.goModDir = filepath.Dir(path)
			} else if strings.HasSuffix(path, ".go") {
				info.hasGo = true
			}
		}

		return nil
	})
	return info
}

// compileResult holds aggregated compilation results.
type compileResult struct {
	buildSuccess  bool
	sourceResults map[string]string // "rust"|"cpp"|"go" -> result message
	errors        []CompileError
	warnings      []string
	log           string // human-readable compilation log
}

// compileSources 编译项目中的源代码
func (s *BuildModuleSkill) compileSources(projectPath string) compileResult {
	var log strings.Builder
	hasSources := false
	result := compileResult{
		buildSuccess:  true,
		sourceResults: make(map[string]string),
	}

	// Single walk pass to detect all source types
	sources := s.detectSources(projectPath)

	// Compile Rust
	if sources.hasCargo {
		hasSources = true
		log.WriteString("  🔧 Compiling Rust...\n")
		res := s.compileRust(projectPath, sources.cargoDir)
		log.WriteString(res)
		result.sourceResults["rust"] = res
		if strings.Contains(res, "❌") {
			result.buildSuccess = false
			result.errors = append(result.errors, parseCompileErrors("rust", res)...)
		} else if strings.Contains(res, "⚠️") {
			result.warnings = append(result.warnings, res)
		}
	}

	// Compile C/C++
	if sources.hasCpp {
		hasSources = true
		log.WriteString("  🔧 Compiling C/C++...\n")
		res := s.compileCpp(projectPath)
		log.WriteString(res)
		result.sourceResults["cpp"] = res
		if strings.Contains(res, "❌") {
			result.buildSuccess = false
			result.errors = append(result.errors, parseCompileErrors("cpp", res)...)
		} else if strings.Contains(res, "⚠️") {
			result.warnings = append(result.warnings, res)
		}
	}

	// Compile Go
	if sources.hasGo {
		hasSources = true
		log.WriteString("  🔧 Compiling Go...\n")
		res := s.compileGo(projectPath)
		log.WriteString(res)
		result.sourceResults["go"] = res
		if strings.Contains(res, "❌") {
			result.buildSuccess = false
			result.errors = append(result.errors, parseCompileErrors("go", res)...)
		} else if strings.Contains(res, "⚠️") {
			result.warnings = append(result.warnings, res)
		}
	}

	if !hasSources {
		log.WriteString("  ℹ️ No compiled sources found (shell-only module)\n")
	}

	result.log = log.String()
	return result
}

// compileRust 编译 Rust 代码
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

// compileCpp 编译 C/C++ 代码
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

// compileGo 编译 Go 代码
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

// validateShellScripts 验证 shell 脚本语法
func (s *BuildModuleSkill) validateShellScripts(projectPath string) bool {
	allPass := true
	scripts := []string{"customize.sh", "service.sh", "post-fs-data.sh", "uninstall.sh", "action.sh"}

	for _, script := range scripts {
		fullPath := filepath.Join(projectPath, script)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			continue
		}

		cmd := exec.Command("bash", "-n", fullPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("  ❌ %s: %s\n", script, strings.TrimSpace(string(output)))
			allPass = false
		}
	}

	return allPass
}

// hasFile 检查文件是否存在
func (s *BuildModuleSkill) hasFile(projectPath, name string) bool {
	_, err := os.Stat(filepath.Join(projectPath, name))
	return err == nil
}

// removeExisting 删除已有文件
func (s *BuildModuleSkill) removeExisting(path string) error {
	if _, err := os.Stat(path); err == nil {
		return os.Remove(path)
	}
	return nil
}

// findExec 查找可执行文件
func findExec(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return ""
}

// findCppCompiler 查找 C++ 编译器
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

// classifyNDKError describes the NDK/cross-compile issue in human-readable form.
func classifyNDKError(output string) string {
	switch {
	case strings.Contains(output, "linker") && strings.Contains(output, "not found"):
		return "NDK linker not found - NDK may not be installed at /opt/android-ndk"
	case strings.Contains(output, "cannot find -l"):
		return "NDK linker cannot find system libraries - NDK sysroot may be incomplete"
	case strings.Contains(output, "aarch64-linux-android"):
		return "Android cross-compile toolchain not fully configured"
	default:
		return "NDK cross-compilation environment issue"
	}
}

// formatCompileErrors formats a slice of CompileErrors into a readable string with fix hints.
func formatCompileErrors(errors []CompileError) string {
	var b strings.Builder
	for _, e := range errors {
		loc := ""
		if e.File != "" {
			loc = fmt.Sprintf("%s:%d", e.File, e.Line)
			if e.Column > 0 {
				loc = fmt.Sprintf("%s:%d", loc, e.Column)
			}
		}
		if loc != "" {
			b.WriteString(fmt.Sprintf("    [%s] %s: %s\n", e.ErrorType, loc, e.Message))
		} else {
			b.WriteString(fmt.Sprintf("    [%s] %s\n", e.ErrorType, e.Message))
		}
		// Append fix hint for each error
		if hint := generateBuildFixHint(e); hint != "" {
			b.WriteString(fmt.Sprintf("    💡 %s\n", hint))
		}
	}
	return b.String()
}

// generateBuildFixHint creates an actionable fix suggestion for a compile error.
func generateBuildFixHint(e CompileError) string {
	msgLower := strings.ToLower(e.Message)
	switch e.SourceType {
	case "go":
		switch e.ErrorType {
		case "undefined":
			return fmt.Sprintf("Check if the identifier is declared. If it's from another package, add the correct import. File: %s:%d", e.File, e.Line)
		case "missing_import":
			if strings.Contains(msgLower, "cannot find package") {
				return "Run 'go mod tidy' to add missing dependencies, or check the package path spelling."
			}
			if strings.Contains(msgLower, "go.mod") {
				return "Ensure go.mod exists in the project root with 'module <name>' declaration."
			}
			return "Check import paths and go.mod dependencies."
		case "type_mismatch":
			if strings.Contains(msgLower, "not enough arguments") || strings.Contains(msgLower, "too many arguments") {
				return "Check function signature and provide the correct number of arguments."
			}
			return "Check types match at the assignment or function call site."
		case "unused_import":
			return "Remove the unused import, or use the imported package in your code."
		case "syntax":
			if strings.Contains(msgLower, "missing return") {
				return "Add a return statement to the function. All non-void functions must return a value."
			}
			return "Fix syntax: check brackets, semicolons, and Go syntax rules."
		}
	case "rust":
		switch e.ErrorType {
		case "missing_import":
			return "Add the missing crate to Cargo.toml [dependencies] and add 'use' statement."
		case "undefined":
			return "Check if the identifier is declared and in scope. Check import paths."
		case "type_mismatch":
			return "Check expected vs actual types. Use .into() or explicit type conversion if needed."
		case "syntax":
			return "Fix Rust syntax: check semicolons, braces, and match arms."
		}
	case "cpp":
		switch e.ErrorType {
		case "missing_import":
			if strings.Contains(msgLower, "fatal error:") || strings.Contains(msgLower, "no such file") {
				return "Add the missing #include directive or check header file paths."
			}
			return "Check #include paths and header file existence."
		case "undefined":
			return "Check if the identifier is declared. Add missing #include or forward declaration."
		case "type_mismatch":
			return "Check argument types match the function parameter types."
		case "syntax":
			return "Fix C++ syntax: check semicolons, braces, and statement structure."
		case "linker":
			return "Undefined reference: ensure all declared functions have implementations."
		}
	}
	return ""
}

// generateBuildErrorSummary creates a concise summary of all build errors for the agent.
func generateBuildErrorSummary(errors []CompileError) string {
	if len(errors) == 0 {
		return ""
	}

	// Count by type
	typeCounts := make(map[string]int)
	for _, e := range errors {
		typeCounts[e.ErrorType]++
	}

	var parts []string
	for errType, count := range typeCounts {
		parts = append(parts, fmt.Sprintf("%d %s error(s)", count, errType))
	}

	return fmt.Sprintf("Build failed with %d error(s): %s", len(errors), strings.Join(parts, ", "))
}

func (s *BuildModuleSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: true,
		Core:      true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
