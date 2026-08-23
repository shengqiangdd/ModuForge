package builder

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/moduforge/backend/internal/config"
)

// targetToImage maps build targets to their container image names.
var targetToImage = map[string]string{
	"magisk":    "moduforge/builder-magisk:latest",
	"ksu":       "moduforge/builder-ksu:latest",
	"apatch":    "moduforge/builder-apatch:latest",
	"universal": "moduforge/builder-magisk:latest",
}

// BuildResult holds the full result of a build operation.
type BuildResult struct {
	Success         bool               `json:"success"`
	Stdout          string             `json:"stdout,omitempty"`
	Stderr          string             `json:"stderr,omitempty"`
	ArtifactPath    string             `json:"artifact_path"`
	Incremental     *IncrementalResult `json:"incremental,omitempty"`
	RecompiledFiles []string           `json:"recompiled_files,omitempty"`
	CacheHits       int                `json:"cache_hits"`
	CacheMisses     int                `json:"cache_misses"`
	Arch            string             `json:"arch"`
}

type Builder struct {
	cfg *config.Config
}

func NewBuilder(cfg *config.Config) *Builder {
	return &Builder{cfg: cfg}
}

// Build 主入口，检查 Docker 可用性，有 Docker 则用容器，否则回退到本地 zip
func (b *Builder) Build(ctx context.Context, projectDir, target, taskID string) (string, error) {
	r, err := b.BuildWithResult(ctx, projectDir, target, taskID, "arm64", func(string) {})
	if err != nil {
		return "", err
	}
	return r.ArtifactPath, nil
}

// BuildWithLog 带日志回调的构建入口
func (b *Builder) BuildWithLog(ctx context.Context, projectDir, target, taskID string, logFn func(string)) (string, error) {
	r, err := b.BuildWithResult(ctx, projectDir, target, taskID, "arm64", logFn)
	if err != nil {
		return "", err
	}
	return r.ArtifactPath, nil
}

// BuildWithArch 带架构参数的构建入口
func (b *Builder) BuildWithArch(ctx context.Context, projectDir, target, taskID, arch string, logFn func(string)) (string, error) {
	r, err := b.BuildWithResult(ctx, projectDir, target, taskID, arch, logFn)
	if err != nil {
		return "", err
	}
	return r.ArtifactPath, nil
}

// BuildWithResult is the full build entry point with incremental + cache support.
func (b *Builder) BuildWithResult(ctx context.Context, projectDir, target, taskID, arch string, logFn func(string)) (*BuildResult, error) {
	return b.BuildWithResultAndProgress(ctx, projectDir, target, taskID, arch, logFn, nil)
}

// BuildProgressFunc is called during build to report progress.
type BuildProgressFunc func(phase string, detail string)

// BuildWithResultAndProgress is the full build entry point with progress reporting.
func (b *Builder) BuildWithResultAndProgress(ctx context.Context, projectDir, target, taskID, arch string, logFn func(string), onProgress BuildProgressFunc) (*BuildResult, error) {
	arch = NormalizeArch(arch)
	result := &BuildResult{Arch: arch}

	emitProgress := func(phase, detail string) {
		if onProgress != nil {
			onProgress(phase, detail)
		}
	}

	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("project dir not found: %s", projectDir)
	}

	artifactDir := filepath.Join(b.cfg.StoragePath, "artifacts", taskID)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return nil, fmt.Errorf("create artifact dir: %w", err)
	}

	// Cleanup expired binary cache
	emitProgress("cache", "cleaning")
	if cleaned, err := CleanupExpiredCache(projectDir); err == nil && cleaned > 0 {
		logFn(fmt.Sprintf("  🧹 Cleaned %d expired cache entries\n", cleaned))
	}

	// Check incremental build status
	emitProgress("incremental", "checking")
	logFn(fmt.Sprintf("  📋 Checking incremental build (arch=%s)...\n", arch))
	incr := CheckIncremental(projectDir, arch)
	result.Incremental = incr

	if incr.NeedsRebuild {
		logFn(fmt.Sprintf("  📝 %s\n", incr.Reason))
		if len(incr.ChangedFiles) > 0 {
			logFn(fmt.Sprintf("  📄 Changed: %d file(s)\n", len(incr.ChangedFiles)))
		}
		if len(incr.NewFiles) > 0 {
			logFn(fmt.Sprintf("  🆕 New: %d file(s)\n", len(incr.NewFiles)))
		}
		emitProgress("incremental", fmt.Sprintf("changes detected: %s", incr.Reason))
	} else {
		logFn("  ✅ No changes detected, using cached binaries\n")
		emitProgress("incremental", "no changes")
	}

	// Compile Go files with arch support + binary cache
	// Post-process first to fix truncation issues from free models
	goFiles := b.DetectGoFiles(projectDir)
	if len(goFiles) > 0 {
		logFn(fmt.Sprintf("  Detected %d Go file(s), cross-compiling for android/%s...\n", len(goFiles), arch))
		emitProgress("compile", fmt.Sprintf("Go: %d files", len(goFiles)))

		// Post-process Go files to fix truncation issues
		PostProcessSourceFiles(projectDir, goFiles, "go", logFn)

		goProgress := func(phase string, current, total int, detail string) {
			emitProgress(phase, fmt.Sprintf("Go [%d/%d]: %s", current, total, detail))
		}
		goResult, err := b.CompileGoFilesArchWithProgress(ctx, projectDir, arch, incr, logFn, goProgress)
		if err != nil {
			// Retry with enhanced post-processing
			logFn(fmt.Sprintf("\n🔧 Go compilation failed, applying enhanced post-processing...\n"))
			PostProcessSourceFiles(projectDir, goFiles, "go", logFn)
			goResult, err = b.CompileGoFilesArchWithProgress(ctx, projectDir, arch, incr, logFn, goProgress)
			if err != nil {
				// Multi-pass auto-fix with LLM (up to 3 attempts)
				for attempt := 1; attempt <= 3; attempt++ {
					logFn(fmt.Sprintf("\n🤖 Auto-fix attempt %d/3: Attempting LLM-based code repair...\n", attempt))
					if b.AutoFixCompileErrorsV2(ctx, projectDir, err, logFn) {
						// Retry compilation after auto-fix
						PostProcessSourceFiles(projectDir, goFiles, "go", logFn)
						goResult, err = b.CompileGoFilesArchWithProgress(ctx, projectDir, arch, incr, logFn, goProgress)
						if err == nil {
							logFn(fmt.Sprintf("✅ Go compilation succeeded after auto-fix v2 attempt %d!\n", attempt))
							break
						}
						logFn(fmt.Sprintf("  ⚠️  Auto-fix attempt %d failed, trying again...\n", attempt))
					} else {
						logFn("  ⚠️  Auto-fix v2 could not parse/fix errors\n")
						break
					}
				}
				if err != nil {
					return nil, fmt.Errorf("go compilation failed after 3 auto-fix v2 attempts: %w", err)
				}
			} else {
				logFn("✅ Go compilation succeeded after enhanced post-processing!\n")
			}
		}
		result.RecompiledFiles = append(result.RecompiledFiles, goResult.Recompiled...)
		result.CacheHits += goResult.CacheHits
		result.CacheMisses += goResult.CacheMisses
	}

	// Compile Rust projects with arch support + binary cache
	rustDirs := b.DetectRustProjects(projectDir)
	if len(rustDirs) > 0 {
		logFn(fmt.Sprintf("  Detected %d Rust project(s)...\n", len(rustDirs)))
		emitProgress("compile", fmt.Sprintf("Rust: %d projects", len(rustDirs)))
		if err := InstallRustArch(ctx, arch, logFn); err != nil {
			return nil, fmt.Errorf("rust installation failed: %w", err)
		}
		for _, dir := range rustDirs {
			rustResult, err := CompileRustProjectArch(ctx, projectDir, dir, arch, incr, logFn)
			if err != nil {
				return nil, fmt.Errorf("rust compilation failed for %s: %w", dir, err)
			}
			result.RecompiledFiles = append(result.RecompiledFiles, rustResult.Recompiled...)
			result.CacheHits += rustResult.CacheHits
			result.CacheMisses += rustResult.CacheMisses
		}
	}

	// Compile Python scripts to native binaries via C wrapper + NDK
	// IMPORTANT: Python compilation MUST happen before C compilation
	// because it generates C files that need to be compiled as separate binaries
	pyFiles := DetectPythonFiles(projectDir)
	if len(pyFiles) > 0 {
		logFn(fmt.Sprintf("  Detected %d Python file(s), compiling to native binaries...\n", len(pyFiles)))
		emitProgress("compile", fmt.Sprintf("Python: %d files", len(pyFiles)))
		for _, pyFile := range pyFiles {
			pyResult, err := CompilePythonToBinary(ctx, projectDir, pyFile, arch, logFn, incr)
			if err != nil {
				logFn(fmt.Sprintf("  ⚠️  Python compilation failed for %s: %v\n", pyFile, err))
				continue
			}
			result.RecompiledFiles = append(result.RecompiledFiles, pyResult.Recompiled...)
			result.CacheHits += pyResult.CacheHits
			result.CacheMisses += pyResult.CacheMisses
		}
		// Python-generated C files are now in python_binaries/ directory
		// They will be compiled as separate binaries in CompileCFilesArch
	}

	// Compile C/C++ files with NDK + arch support
	// Post-process first to fix truncation issues from free models
	cFiles := b.DetectCFiles(projectDir)
	if len(cFiles) > 0 {
		logFn(fmt.Sprintf("  Detected %d C/C++ file(s), cross-compiling with NDK...\n", len(cFiles)))
		emitProgress("compile", fmt.Sprintf("C/C++: %d files", len(cFiles)))

		// Post-process C/C++ files to fix truncation issues
		PostProcessSourceFiles(projectDir, cFiles, "c", logFn)

		cResult, err := b.CompileCFilesArch(ctx, projectDir, arch, incr, logFn)
		if err != nil {
			// Multi-pass auto-fix with LLM (up to 3 attempts)
			for attempt := 1; attempt <= 3; attempt++ {
				logFn(fmt.Sprintf("\n🤖 Auto-fix attempt %d/3: Attempting LLM-based C code repair...\n", attempt))
				if b.AutoFixCompileErrorsV2(ctx, projectDir, err, logFn) {
					// Retry compilation after auto-fix
					PostProcessSourceFiles(projectDir, cFiles, "c", logFn)
					cResult, err = b.CompileCFilesArch(ctx, projectDir, arch, incr, logFn)
					if err == nil {
						logFn(fmt.Sprintf("✅ C/C++ compilation succeeded after auto-fix v2 attempt %d!\n", attempt))
						break
					}
					logFn(fmt.Sprintf("  ⚠️  Auto-fix attempt %d failed, trying again...\n", attempt))
				} else {
					logFn("  ⚠️  Auto-fix v2 could not parse/fix errors\n")
					break
				}
			}
			if err != nil {
				return nil, fmt.Errorf("C/C++ compilation failed after 3 auto-fix v2 attempts: %w", err)
			}
		}
		result.RecompiledFiles = append(result.RecompiledFiles, cResult.Recompiled...)
		result.CacheHits += cResult.CacheHits
		result.CacheMisses += cResult.CacheMisses
	}

	// Update build cache after successful compilation
	emitProgress("cache", "updating")
	if err := UpdateBuildCacheAfterBuild(projectDir, arch, target); err != nil {
		logFn(fmt.Sprintf("  ⚠️  Failed to update build cache: %v\n", err))
	}

	// Package
	emitProgress("package", "starting")
	logFn("  📦 Packaging module...\n")
	var artifactPath string
	var err error
	if b.dockerAvailable(ctx) {
		artifactPath, err = b.buildWithDocker(ctx, projectDir, target, artifactDir)
	} else {
		artifactPath, err = b.buildNative(ctx, projectDir, target, artifactDir)
	}
	if err != nil {
		return nil, err
	}

	result.ArtifactPath = artifactPath
	logFn(fmt.Sprintf("  📦 Cache stats: %d hits, %d misses\n", result.CacheHits, result.CacheMisses))

	// Post-build validation: check binary name matches script references
	emitProgress("validate", "checking build output")
	validateBuildOutput(projectDir, logFn)

	// Shell syntax validation on module scripts
	ValidateShellScripts(projectDir, logFn)
	emitProgress("done", fmt.Sprintf("artifact: %s", artifactPath))

	return result, nil
}

// detectGoFiles 检查项目中是否有 .go 文件
func (b *Builder) DetectGoFiles(projectDir string) []string {
	var goFiles []string
	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})
	return goFiles
}

// detectRustProjects 检查项目中是否有 Cargo.toml（Rust 项目）
func (b *Builder) DetectRustProjects(projectDir string) []string {
	var dirs []string
	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && info.Name() == "Cargo.toml" {
			dirs = append(dirs, filepath.Dir(path))
		}
		return nil
	})
	return dirs
}

// dockerAvailable 检查 Docker daemon 是否可访问
func (b *Builder) dockerAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "info", "--format", "{{.ServerVersion}}")
	cmd.Env = append(os.Environ(), "DOCKER_HOST="+b.cfg.DockerEndpoint)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	_ = out
	return true
}

// buildWithDocker 运行 Docker 构建容器
func (b *Builder) buildWithDocker(ctx context.Context, sourceDir, target, artifactDir string) (string, error) {
	image, ok := targetToImage[target]
	if !ok {
		return "", fmt.Errorf("unknown target: %s", target)
	}

	outputZip := filepath.Join(artifactDir, "module.zip")

	args := []string{
		"run", "--rm",
		"--network", "none",
		"--memory", "256m",
		"--cpus", "1",
		"--read-only",
		"--tmpfs", "/tmp:size=64m",
		"-v", sourceDir + ":/workspace:ro",
		"-v", artifactDir + ":/output",
		image,
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = append(os.Environ(), "DOCKER_HOST="+b.cfg.DockerEndpoint)

	var stderr strings.Builder
	cmd.Stderr = &stderr
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("docker build failed: %s: %w", string(out)+stderr.String(), err)
	}

	if _, err := os.Stat(outputZip); os.IsNotExist(err) {
		return "", fmt.Errorf("build container did not produce output zip")
	}

	// Wrap webui files into webroot/ directory
	if err := WrapWebroot(outputZip); err != nil {
		log.Printf("[Builder] docker webroot wrap skipped: %v", err)
	}

	// P1: Ensure META-INF/com/google/android/{update-binary, updater-script} exists
	if err := EnsureMetaInf(outputZip); err != nil {
		log.Printf("[Builder] docker metainf ensure skipped: %v", err)
	}

	return outputZip, nil
}

// buildNative 本地 zip 打包（无 Docker 回退方案，排除源码只保留运行时文件）
func (b *Builder) buildNative(ctx context.Context, sourceDir, target, artifactDir string) (string, error) {
	outputZip := filepath.Join(artifactDir, "module.zip")

	// 使用 ZipModuleForBuild 排除源码目录，只打包编译后的二进制和shell脚本
	if err := ZipModuleForBuild(sourceDir, outputZip); err != nil {
		return "", fmt.Errorf("zip failed: %w", err)
	}

	// Wrap webui files (HTML/CSS/JS at root level) into webroot/ directory
	if err := WrapWebroot(outputZip); err != nil {
		// Non-fatal: log but don't fail the build
		log.Printf("[Builder] webroot wrap skipped: %v", err)
	}

	// P1: Ensure META-INF/com/google/android/{update-binary, updater-script} exists
	if err := EnsureMetaInf(outputZip); err != nil {
		log.Printf("[Builder] metainf ensure skipped: %v", err)
	}

	return outputZip, nil
}

// GetSupportedArchitectures returns the list of supported build architectures.
func GetSupportedArchitectures() []ArchInfo {
	return SupportedArchitectures
}

// CompileResult holds compilation statistics.
type CompileResult struct {
	Recompiled  []string
	CacheHits   int
	CacheMisses int
}

func logCompileSkip(logFn func(string), name, reason string) {
	logFn(fmt.Sprintf("  ⏭️  %s: %s (using cached binary)\n", name, reason))
}

// validateBuildOutput checks that compiled binaries and script references are consistent.
// Returns warnings for mismatches (e.g., script references wrong binary name).
func validateBuildOutput(moduleDir string, logFn func(string)) []string {
	var warnings []string

	// Find compiled binary — builder always produces "androsmart"
	var binaryFound bool
	filepath.Walk(moduleDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.Name() == "androsmart" {
			binaryFound = true
		}
		return nil
	})

	if !binaryFound {
		warnings = append(warnings, "⚠️  compiled binary 'androsmart' not found in module directory")
	}

	// Check scripts reference the correct binary name
	scripts := []string{"customize.sh", "service.sh", "uninstall.sh"}
	wrongNames := []string{"perf_tuner", "daemon", "tuner", "my_daemon", "moduforge"}
	for _, script := range scripts {
		content, err := os.ReadFile(filepath.Join(moduleDir, script))
		if err != nil {
			continue
		}
		contentStr := string(content)
		for _, wrong := range wrongNames {
			// Only warn if the wrong name appears and androsmart is NOT also referenced
			if strings.Contains(contentStr, wrong) && !strings.Contains(contentStr, "androsmart") {
				msg := fmt.Sprintf("⚠️  %s references '%s' but compiled binary is 'androsmart'", script, wrong)
				warnings = append(warnings, msg)
			}
		}
	}

	if logFn != nil {
		for _, w := range warnings {
			logFn("  " + w + "\n")
		}
	}
	return warnings
}
