package builder

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/logger"
	"github.com/moduforge/backend/internal/metrics"
	"golang.org/x/sync/errgroup"
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
	Duration        time.Duration      `json:"duration,omitempty"`
	ArtifactPath    string             `json:"artifact_path"`
	Incremental     *IncrementalResult `json:"incremental,omitempty"`
	RecompiledFiles []string           `json:"recompiled_files,omitempty"`
	CacheHits       int                `json:"cache_hits"`
	CacheMisses     int                `json:"cache_misses"`
	Arch            string             `json:"arch"`
}

// progressAccumulator holds intermediate compilation stats for parallel phases.
type progressAccumulator struct {
	RecompiledFiles []string
	CacheHits       int
	CacheMisses     int
}

type Builder struct {
	cfg             *config.Config
	progBroadcaster func(phase string, detail string) // optional SSE broadcast
}

// NewBuilder creates a Builder without SSE broadcasting.
func NewBuilder(cfg *config.Config) *Builder {
	return &Builder{cfg: cfg}
}

// NewBuilderWithBroadcast creates a Builder wired to an SSE broadcaster.
func NewBuilderWithBroadcast(cfg *config.Config, broadcaster func(phase string, detail string)) *Builder {
	return &Builder{cfg: cfg, progBroadcaster: broadcaster}
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

	logger.Info("Starting build: project=%s arch=%s target=%s", taskID, arch, target)
	startTime := time.Now()
	var buildErr error
	defer func() {
		elapsed := time.Since(startTime).Seconds()
		metrics.BuildDuration.Observe(elapsed)
		metrics.TotalBuilds.Inc()
		metrics.BuildsByTarget.WithLabelValues(target).Inc()
		metrics.BuildsByArch.WithLabelValues(arch).Inc()
		if buildErr == nil {
			metrics.BuildSuccesses.Inc()
			logger.Info("Build completed: project=%s arch=%s duration=%.1fs", taskID, arch, elapsed)
		} else {
			metrics.BuildFailures.Inc()
			logger.Error("Build failed: project=%s error=%v", taskID, buildErr)
		}
	}()

	emitProgress := func(phase, detail string) {
		if onProgress != nil {
			onProgress(phase, detail)
		}
		if b.progBroadcaster != nil {
			b.progBroadcaster(phase, detail)
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

	// ── Compilation Phase ───────────────────────────────────────────────
	// Go + Rust can compile in parallel; Python → C must stay serial.

	goFiles := b.DetectGoFiles(projectDir)
	rustDirs := b.DetectRustProjects(projectDir)
	hasParallelPhase := len(goFiles) > 0 || len(rustDirs) > 0

	if hasParallelPhase {
		var gGAcc, gRAcc progressAccumulator
		g, gctx := errgroup.WithContext(ctx)
		// errgroup no longer supports SetMaxWorkers; concurrency is controlled by GOMAXPROCS

		// Phase 1a: Go (parallel)
		if len(goFiles) > 0 {
			fn := goFiles // capture for closure
			logFn(fmt.Sprintf("  Detected %d Go file(s), cross-compiling for android/%s...\n", len(fn), arch))
			emitProgress("compile", fmt.Sprintf("Go: %d files", len(fn)))
			g.Go(func() error {
				ctx = context.Background() // errgroup ctx is cancelled after first error — use fresh ctx per lang
				return b.doGoCompile(gctx, projectDir, arch, incr, fn, logFn, emitProgress, &gGAcc)
			})
		}

		// Phase 1b: Rust (parallel)
		if len(rustDirs) > 0 {
			rd := rustDirs // capture for closure
			logFn(fmt.Sprintf("  Detected %d Rust project(s)...\n", len(rd)))
			emitProgress("compile", fmt.Sprintf("Rust: %d projects", len(rd)))
			g.Go(func() error {
				ctx = context.Background()
				return b.doRustCompile(gctx, projectDir, rd, arch, incr, logFn, emitProgress, &gRAcc)
			})
		}

		if err := g.Wait(); err != nil {
			return nil, err // propagate first failure
		}
		result.RecompiledFiles = append(result.RecompiledFiles, gGAcc.RecompiledFiles...)
		metrics.CacheHits.Add(float64(gGAcc.CacheHits))
		metrics.CacheMisses.Add(float64(gGAcc.CacheMisses))
		result.CacheHits += gGAcc.CacheHits
		result.CacheMisses += gGAcc.CacheMisses
		result.RecompiledFiles = append(result.RecompiledFiles, gRAcc.RecompiledFiles...)
		metrics.CacheHits.Add(float64(gRAcc.CacheHits))
		metrics.CacheMisses.Add(float64(gRAcc.CacheMisses))
	} else if len(goFiles) == 0 && len(rustDirs) == 0 {
		// Still detect for logging even when no sources exist
		b.DetectGoFiles(projectDir)
		b.DetectRustProjects(projectDir)
	}

	// Phase 2: Python (serial — generates C files)
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
			metrics.CacheHits.Add(float64(pyResult.CacheHits))
			metrics.CacheMisses.Add(float64(pyResult.CacheMisses))
			result.CacheHits += pyResult.CacheHits
			result.CacheMisses += pyResult.CacheMisses
		}
	}

	// Phase 3: C/C++ (serial — depends on Python output)
	var cAcc progressAccumulator
	cFiles := b.DetectCFiles(projectDir)
	if len(cFiles) > 0 {
		logFn(fmt.Sprintf("  Detected %d C/C++ file(s), cross-compiling with NDK...\n", len(cFiles)))
		emitProgress("compile", fmt.Sprintf("C/C++: %d files", len(cFiles)))
		if err := b.doCCCompile(ctx, projectDir, arch, incr, cFiles, logFn, emitProgress, &cAcc); err != nil {
			return nil, err
		}
		result.RecompiledFiles = append(result.RecompiledFiles, cAcc.RecompiledFiles...)
		result.CacheHits += cAcc.CacheHits
		result.CacheMisses += cAcc.CacheMisses
		metrics.CacheHits.Add(float64(cAcc.CacheHits))
		metrics.CacheMisses.Add(float64(cAcc.CacheMisses))
	}

	// Update build cache after successful compilation
	emitProgress("cache", "updating")
	if err := UpdateBuildCacheAfterBuild(projectDir, arch, target); err != nil {
		logFn(fmt.Sprintf("  ⚠️  Failed to update build cache: %v\n", err))
	}

	// Android APK build: if Android project is detected, build APK and skip normal packaging
	androidProjects := b.DetectAndroidProjects(projectDir)
	if len(androidProjects) > 0 {
		emitProgress("android", "building APK")
		logFn("  📱 Building Android APK...\n")
		androidResult, err := b.BuildAndroidAPK(ctx, projectDir, taskID, arch, logFn)
		if err != nil {
			logFn(fmt.Sprintf("  ❌ Android APK build failed: %v\n", err))
			return nil, fmt.Errorf("android APK build failed: %w", err)
		}
		result.ArtifactPath = androidResult.ArtifactPath
		result.Duration = androidResult.Duration
		logFn(fmt.Sprintf("  ✅ Android APK build complete! (arch=%s)\n", arch))
		emitProgress("done", fmt.Sprintf("APK: %s", androidResult.ArtifactPath))
		return result, nil
	}

	// Package (standard Magisk module packaging)
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

// ── Parallel compilation helpers ────────────────────────────────────────

func (b *Builder) doGoCompile(ctx context.Context, projectDir, arch string, incr *IncrementalResult, goFiles []string,
	logFn func(string), emitProgress func(phase, detail string), acc *progressAccumulator) error {

	logFn(fmt.Sprintf("  Detected %d Go file(s), cross-compiling for android/%s...\n", len(goFiles), arch))
	emitProgress("compile", fmt.Sprintf("Go: %d files", len(goFiles)))

	PostProcessSourceFiles(projectDir, goFiles, "go", logFn)

	goProgress := func(phase string, current, total int, detail string) {
		emitProgress(phase, fmt.Sprintf("Go [%d/%d]: %s", current, total, detail))
	}

	var goResult CompileResult
	var err error

	gr1, err := b.CompileGoFilesArchWithProgress(ctx, projectDir, arch, incr, logFn, goProgress)
	goResult = *gr1
	if err != nil {
		logFn("\n🔧 Go compilation failed, applying enhanced post-processing...\n")
		PostProcessSourceFiles(projectDir, goFiles, "go", logFn)
		gr2, err := b.CompileGoFilesArchWithProgress(ctx, projectDir, arch, incr, logFn, goProgress)
		goResult = *gr2
		if err != nil {
			for attempt := 1; attempt <= 3; attempt++ {
				logFn(fmt.Sprintf("\n🤖 Auto-fix attempt %d/3: Attempting LLM-based code repair...\n", attempt))
				if b.AutoFixCompileErrorsV2(ctx, projectDir, err, logFn) {
					PostProcessSourceFiles(projectDir, goFiles, "go", logFn)
					gr3, err := b.CompileGoFilesArchWithProgress(ctx, projectDir, arch, incr, logFn, goProgress)
					goResult = *gr3
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
				return fmt.Errorf("go compilation failed after 3 auto-fix v2 attempts: %w", err)
			}
		} else {
			logFn("✅ Go compilation succeeded after enhanced post-processing!\n")
		}
	}

	acc.RecompiledFiles = append(acc.RecompiledFiles, goResult.Recompiled...)
	acc.CacheHits += goResult.CacheHits
	acc.CacheMisses += goResult.CacheMisses
	return nil
}

func (b *Builder) doRustCompile(ctx context.Context, projectDir string, rustDirs []string, arch string,
	incr *IncrementalResult, logFn func(string), emitProgress func(phase, detail string), acc *progressAccumulator) error {

	if err := InstallRustArch(ctx, arch, logFn); err != nil {
		return fmt.Errorf("rust installation failed: %w", err)
	}

	for _, dir := range rustDirs {
		rustResult, err := CompileRustProjectArch(ctx, projectDir, dir, arch, incr, logFn)
		if err != nil {
			return fmt.Errorf("rust compilation failed for %s: %w", dir, err)
		}
		acc.RecompiledFiles = append(acc.RecompiledFiles, rustResult.Recompiled...)
		acc.CacheHits += rustResult.CacheHits
		acc.CacheMisses += rustResult.CacheMisses
	}
	return nil
}

func (b *Builder) doCCCompile(ctx context.Context, projectDir, arch string, incr *IncrementalResult,
	cFiles []string, logFn func(string), emitProgress func(phase, detail string), acc *progressAccumulator) error {

	logFn(fmt.Sprintf("  Detected %d C/C++ file(s), cross-compiling with NDK...\n", len(cFiles)))
	emitProgress("compile", fmt.Sprintf("C/C++: %d files", len(cFiles)))

	PostProcessSourceFiles(projectDir, cFiles, "c", logFn)

	var cResult CompileResult
	var err error

	cr1, err := b.CompileCFilesArch(ctx, projectDir, arch, incr, logFn)
	cResult = *cr1
	if err != nil {
		for attempt := 1; attempt <= 3; attempt++ {
			logFn(fmt.Sprintf("\n🤖 Auto-fix attempt %d/3: Attempting LLM-based C code repair...\n", attempt))
			if b.AutoFixCompileErrorsV2(ctx, projectDir, err, logFn) {
				PostProcessSourceFiles(projectDir, cFiles, "c", logFn)
				cr2, err := b.CompileCFilesArch(ctx, projectDir, arch, incr, logFn)
				cResult = *cr2
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
			return fmt.Errorf("C/C++ compilation failed after 3 auto-fix v2 attempts: %w", err)
		}
	}

	acc.RecompiledFiles = append(acc.RecompiledFiles, cResult.Recompiled...)
	acc.CacheHits += cResult.CacheHits
	acc.CacheMisses += cResult.CacheMisses
	return nil
}
