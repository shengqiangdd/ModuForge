package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/agent/registry"
	"github.com/moduforge/backend/internal/builder"
	"github.com/moduforge/backend/internal/storage"
)

// BuildModuleSkill implements the build_module skill for compiling and packaging modules.
type BuildModuleSkill struct {
	projectPath string
	db          *sql.DB
	storage     storage.StorageAdapter // optional S3 storage backend
}

// NewBuildModuleSkillWithDB creates a new BuildModuleSkill with database support.
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

// Name returns the skill name.
func (s *BuildModuleSkill) Name() string {
	return "build_module"
}

// Description returns the skill description.
func (s *BuildModuleSkill) Description() string {
	return `Build the module: validate → compile → package.
Input: {} or {"project_id": "..."}.
Compiles source code (Rust/C/C++/Go), validates structure, then creates ZIP.
Returns build log with success/failure status.`
}

// Execute runs the build process: validate → compile → package.
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

	// ========== Phase 1: Sync source files from DB/S3 to disk ==========
	// Sync MUST happen before validation so that files are on disk for stat checks.
	log.WriteString("\n── Syncing source files to disk... ──\n")
	if s.db != nil && projectID != "" {
		// First pass: collect all relative paths and detect common prefix
		type fileEntry struct {
			path    string
			relPath string
		}
		var files []fileEntry
		rows, err := s.db.Query(`SELECT path FROM project_files WHERE project_id=?`, projectID)
		if err == nil {
			for rows.Next() {
				var path string
				if err := rows.Scan(&path); err != nil {
					continue
				}
				relPath := strings.TrimPrefix(path, "/")
				files = append(files, fileEntry{path: path, relPath: relPath})
			}
			rows.Close()
		}

		// Detect common top-level directory (e.g., all files under "hello-world/")
		// Strategy: find the deepest common ancestor of all file paths, which is the module root.
		effectivePath := projectPath
		commonPrefix := ""
		if len(files) > 1 {
			// Split all paths and find common prefix components
			splitPaths := make([][]string, len(files))
			for i, fe := range files {
				splitPaths[i] = strings.Split(fe.relPath, "/")
			}
			minLen := len(splitPaths[0])
			for _, sp := range splitPaths {
				if len(sp) < minLen {
					minLen = len(sp)
				}
			}
			commonComponents := 0
			for ci := 0; ci < minLen; ci++ {
				val := splitPaths[0][ci]
				allMatch := true
				for _, sp := range splitPaths {
					if sp[ci] != val {
						allMatch = false
						break
					}
				}
				if !allMatch {
					break
				}
				commonComponents = ci + 1
			}
			if commonComponents > 0 {
				prefixParts := splitPaths[0][:commonComponents]
				commonPrefix = strings.Join(prefixParts, "/")
				effectivePath = filepath.Join(projectPath, commonPrefix)
				log.WriteString(fmt.Sprintf("  📁 Detected module root: %s/\n", commonPrefix))
				os.MkdirAll(effectivePath, 0755)
			}
		} else if len(files) == 1 {
			// Single file: use its directory as root
			dir := filepath.Dir(files[0].relPath)
			if dir != "" && dir != "." {
				commonPrefix = dir
				effectivePath = filepath.Join(projectPath, commonPrefix)
				log.WriteString(fmt.Sprintf("  📁 Detected module root: %s/\n", commonPrefix))
				os.MkdirAll(effectivePath, 0755)
			}
		}

		// Second pass: write files, stripping the common prefix if detected
		synced := 0
		for _, fe := range files {
			content, err := readFileContent(ctx, s.storage, s.db, projectID, fe.path)
			if err != nil {
				continue
			}
			// Strip common prefix from relPath if detected
			writeRelPath := fe.relPath
			if commonPrefix != "" {
				writeRelPath = strings.TrimPrefix(fe.relPath, commonPrefix+"/")
			}
			fullPath := filepath.Join(effectivePath, writeRelPath)
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
		// Use effectivePath for subsequent phases
		projectPath = effectivePath
	}

	// ========== Phase 2: Structure Validation ==========
	log.WriteString("\n── Phase 2: Structure Validation ──\n")
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

	// ========== Phase 3: Source Compilation ==========
	log.WriteString("\n── Phase 3: Source Compilation ──\n")
	log.WriteString("[BUILD_PROGRESS] phase=compile status=starting\n")
	compileResult := s.compileSources(projectPath)
	log.WriteString(compileResult.log)
	if compileResult.buildSuccess {
		log.WriteString("[BUILD_PROGRESS] phase=compile status=done\n")
	} else {
		log.WriteString(fmt.Sprintf("[BUILD_PROGRESS] phase=compile status=failed errors=%d\n", len(compileResult.errors)))
	}

	// ========== Phase 4: Shell Script Validation ==========
	log.WriteString("\n── Phase 4: Shell Script Validation ──\n")
	log.WriteString("[BUILD_PROGRESS] phase=shellcheck status=starting\n")
	shellValid := s.validateShellScripts(projectPath)
	if shellValid {
		log.WriteString("  ✅ All shell scripts passed syntax check\n")
	} else {
		log.WriteString("  ⚠️ Some shell scripts have syntax issues (see above)\n")
	}
	log.WriteString("[BUILD_PROGRESS] phase=shellcheck status=done\n")

	// ========== Phase 5: Package ==========
	log.WriteString("\n── Phase 5: Package ──\n")
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

// compileSources compiles all source code in the project.
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

	// Compile Android APP (APK) if app/build.gradle.kts exists
	apkGradleFile := filepath.Join(projectPath, "app", "build.gradle.kts")
	if _, err := os.Stat(apkGradleFile); err == nil {
		hasSources = true
		log.WriteString("  📱 Compiling Android APP (APK)...\n")
		res := s.compileAndroidApp(projectPath)
		log.WriteString(res)
		result.sourceResults["android_app"] = res
		if strings.Contains(res, "❌") {
			result.buildSuccess = false
			result.errors = append(result.errors, CompileError{
				SourceType: "android_app",
				Message:    "Android APP build failed",
				ErrorType:  "unknown",
			})
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

// validateShellScripts validates shell script syntax and security in the project.
func (s *BuildModuleSkill) validateShellScripts(projectPath string) bool {
	allPass := true
	scripts := []string{"customize.sh", "service.sh", "post-fs-data.sh", "uninstall.sh", "action.sh"}

	for _, script := range scripts {
		fullPath := filepath.Join(projectPath, script)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		content := string(data)
		originalContent := content

		// Syntax check
		cmd := exec.Command("bash", "-n", fullPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("  ❌ %s: %s\n", script, strings.TrimSpace(string(output)))
			allPass = false
		}

		// Security fixes for service.sh
		if script == "service.sh" {
			// Fix chmod 777 -> 755 (directory permissions)
			if strings.Contains(content, "chmod 777") {
				content = strings.ReplaceAll(content, "chmod 777", "chmod 755")
				fmt.Printf("  🔧 %s: fixed chmod 777 -> 755\n", script)
			}

			// Fix chmod 666 -> 644 (file permissions)
			if strings.Contains(content, "chmod 666") {
				content = strings.ReplaceAll(content, "chmod 666", "chmod 644")
				fmt.Printf("  🔧 %s: fixed chmod 666 -> 644\n", script)
			}

			// Write back if changed
			if content != originalContent {
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					fmt.Printf("  ⚠️ %s: failed to write security fixes: %v\n", script, err)
				}
			}

			// Check for include_prop usage
			if !strings.Contains(content, "include_prop") {
				fmt.Printf("  ⚠️ %s: 未使用 include_prop (建议使用 Magisk 标准方式读取配置)\n", script)
			}
		}
	}

	return allPass
}

// hasFile checks if a file exists in the project path.
func (s *BuildModuleSkill) hasFile(projectPath, name string) bool {
	_, err := os.Stat(filepath.Join(projectPath, name))
	return err == nil
}

// compileAndroidApp builds an Android APK from the app/ subdirectory.
func (s *BuildModuleSkill) compileAndroidApp(projectPath string) string {
	// Check if APK already exists — skip rebuild if so
	apkDst := filepath.Join(projectPath, "app", "app.apk")
	if _, err := os.Stat(apkDst); err == nil {
		apkInfo, _ := os.Stat(apkDst)
		apkSizeMB := float64(apkInfo.Size()) / 1024 / 1024
		return fmt.Sprintf("  ✅ Android APP already built (APK: %.1f MB) — skipping rebuild\n", apkSizeMB)
	}

	buildCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	gradleProjectDir := filepath.Join(projectPath, "app")

	// Try using full Gradle installation first, fall back to gradlew
	gradleBin := "/opt/gradle/gradle-8.7/bin/gradle"
	if _, err := os.Stat(gradleBin); os.IsNotExist(err) {
		// Fall back to gradlew
		gradlew := filepath.Join(gradleProjectDir, "gradlew")
		if _, err := os.Stat(gradlew); os.IsNotExist(err) {
			return "  ⚠️ Neither gradle nor gradlew found — run android_app skill first\n"
		}
		gradleBin = gradlew
	}

	cmd := exec.CommandContext(buildCtx, gradleBin, "assembleDebug", "--no-daemon")
	cmd.Dir = gradleProjectDir
	cmd.Env = append(os.Environ(),
		"ANDROID_HOME=/opt/android-sdk",
		"ANDROID_SDK_ROOT=/opt/android-sdk",
		"JAVA_HOME=/usr/lib/jvm/java-17-openjdk",
		"GRADLE_OPTS=-Xmx1536m",
	)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		if buildCtx.Err() == context.DeadlineExceeded {
			return fmt.Sprintf("  ❌ Android APP build timed out after %v\n", 10*time.Minute)
		}
		return fmt.Sprintf("  ❌ Android APP build failed:\n%s\n", outputStr)
	}

	// Copy APK to module root/app/
	apkSrc := filepath.Join(gradleProjectDir, "app", "build", "outputs", "apk", "debug", "app-debug.apk")
	apkDst = filepath.Join(projectPath, "app", "app.apk")

	if _, err := os.Stat(apkSrc); os.IsNotExist(err) {
		return "  ⚠️ APK output not found (build succeeded but APK missing)\n"
	}

	apkData, err := os.ReadFile(apkSrc)
	if err != nil {
		return fmt.Sprintf("  ❌ Failed to read APK: %v\n", err)
	}
	os.MkdirAll(filepath.Dir(apkDst), 0755)
	if err := os.WriteFile(apkDst, apkData, 0644); err != nil {
		return fmt.Sprintf("  ❌ Failed to copy APK to module: %v\n", err)
	}

	apkSizeMB := float64(len(apkData)) / 1024 / 1024
	return fmt.Sprintf("  ✅ Android APP built successfully (APK: %.1f MB)\n", apkSizeMB)
}

// removeExisting deletes an existing file.
func (s *BuildModuleSkill) removeExisting(path string) error {
	if _, err := os.Stat(path); err == nil {
		return os.Remove(path)
	}
	return nil
}

// Metadata returns the skill metadata.
func (s *BuildModuleSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: true,
		Core:      true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
