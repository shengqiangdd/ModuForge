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

// CompileProgress is called during compilation to report progress.
type CompileProgress func(phase string, current, total int, detail string)

// CompileGoFilesArch compiles Go files with architecture support and binary caching.
func (b *Builder) CompileGoFilesArch(ctx context.Context, projectDir, arch string, incr *IncrementalResult, logFn func(string)) (*CompileResult, error) {
	return b.CompileGoFilesArchWithProgress(ctx, projectDir, arch, incr, logFn, nil)
}

// CompileGoFilesArchWithProgress compiles Go files with progress reporting.
func (b *Builder) CompileGoFilesArchWithProgress(ctx context.Context, projectDir, arch string, incr *IncrementalResult, logFn func(string), onProgress CompileProgress) (*CompileResult, error) {
	result := &CompileResult{}

	emitProgress := func(phase string, current, total int, detail string) {
		if onProgress != nil {
			onProgress(phase, current, total, detail)
		}
	}

	// 编译前重组 Go 模块结构（解决内部 import 路径问题）
	if err := reorganizeGoModule(projectDir, logFn); err != nil {
		logFn(fmt.Sprintf("  ⚠️  Go module reorganization failed: %v\n", err))
	}

	// 收集所有 .go 文件
	var goFiles []string
	err := filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(goFiles) == 0 {
		return result, nil
	}

	// 按目录分组
	dirSet := make(map[string]bool)
	for _, f := range goFiles {
		dirSet[filepath.Dir(f)] = true
	}

	// 转为切片并按路径长度排序（长路径优先，深层目录先处理）
	var dirs []string
	for d := range dirSet {
		dirs = append(dirs, d)
	}
	for i := 0; i < len(dirs); i++ {
		for j := i + 1; j < len(dirs); j++ {
			if len(dirs[j]) > len(dirs[i]) {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			}
		}
	}

	// 去掉被其他目录包含的目录
	var filteredDirs []string
	for _, d := range dirs {
		contained := false
		for _, fd := range filteredDirs {
			if strings.HasPrefix(d, fd+string(os.PathSeparator)) {
				contained = true
				break
			}
		}
		if !contained {
			filteredDirs = append(filteredDirs, d)
		}
	}

	// 反转，从最浅到最深编译
	for i, j := 0, len(filteredDirs)-1; i < j; i, j = i+1, j-1 {
		filteredDirs[i], filteredDirs[j] = filteredDirs[j], filteredDirs[i]
	}

	// 检查 go 是否可用（检查多个可能的位置）
	var goPath string
	for _, p := range []string{"go", "/usr/local/go/bin/go", "/usr/bin/go"} {
		if path, err := exec.LookPath(p); err == nil {
			goPath = path
			break
		}
	}
	if goPath == "" {
		return nil, fmt.Errorf("go compiler not found in PATH or /usr/local/go/bin")
	}

	// 确定二进制输出目录
	binDir := filepath.Join(projectDir, "system", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return nil, fmt.Errorf("create system/bin: %w", err)
	}

	// 为每个编译目录确保 go.mod 存在
	for _, dir := range filteredDirs {
		ensureGoMod(goPath, dir, dirBaseName(dir))
	}

	archInfo, _ := GetArchInfo(arch)
	goarch := archInfo.Goarch
	goarm := archInfo.GOARM

	// Build the list of packages to compile (filter by incremental)
	var packagesToCompile []string
	for _, dir := range filteredDirs {
		if !hasMainPackage(dir) {
			logFn(fmt.Sprintf("  Skip %s: no main package\n", filepath.Base(dir)))
			continue
		}

		name := filepath.Base(dir)
		if name == "." || name == "go" {
			name = "daemon"
		}
		binPath := filepath.Join(binDir, name)

		// Determine if this package needs compilation
		needsCompile := true

		if incr != nil && !incr.NeedsRebuild {
			// No rebuild needed globally - check binary cache
			mainFile := findMainFile(dir)
			if mainFile != "" {
				if cached := CheckBinaryCache(projectDir, mainFile); cached != nil {
					if input, err := os.ReadFile(*cached); err == nil {
						if err := os.WriteFile(binPath, input, 0755); err == nil {
							logCompileSkip(logFn, name, "binary cache hit")
							result.CacheHits++
							result.Recompiled = append(result.Recompiled, "system/bin/"+name)
							needsCompile = false
						}
					}
				}
			}
		} else if incr != nil && len(incr.ChangedFiles) > 0 {
			// Rebuild needed - check if this specific dir has changes
			// Use ChangedDirs map for O(1) lookup instead of iterating ChangedFiles
			if !incr.ChangedDirs[dir] {
				// Dir unchanged - try binary cache
				mainFile := findMainFile(dir)
				if mainFile != "" {
					if cached := CheckBinaryCache(projectDir, mainFile); cached != nil {
						if input, err := os.ReadFile(*cached); err == nil {
							if err := os.WriteFile(binPath, input, 0755); err == nil {
								logCompileSkip(logFn, name, "binary cache hit (dir unchanged)")
								result.CacheHits++
								result.Recompiled = append(result.Recompiled, "system/bin/"+name)
								needsCompile = false
							}
						}
					}
				}
			}
		}

		if needsCompile {
			packagesToCompile = append(packagesToCompile, dir)
		}
	}

	if len(packagesToCompile) == 0 {
		logFn("  ✅ All packages cached, nothing to compile\n")
		return result, nil
	}

	logFn(fmt.Sprintf("  📦 %d package(s) to compile\n", len(packagesToCompile)))
	emitProgress("compile", 0, len(packagesToCompile), "starting")

	// Ensure Go module cache directories exist
	ensureGoCacheDirs(logFn)

	envExtra := goBuildEnv(goarch, goarm)

	for i, dir := range packagesToCompile {
		name := filepath.Base(dir)
		if name == "." || name == "go" {
			name = "daemon"
		}
		binPath := filepath.Join(binDir, name)

		emitProgress("compile", i+1, len(packagesToCompile), name)
		logFn(fmt.Sprintf("  🔨 Compiling %s → system/bin/%s (android/%s) [%d/%d]...\n", name, name, goarch, i+1, len(packagesToCompile)))

		// Phase 1: Download dependencies with longer timeout
		dlCtx, dlCancel := context.WithTimeout(ctx, 180*time.Second)
		dlCmd := exec.CommandContext(dlCtx, goPath, "mod", "download")
		dlCmd.Dir = dir
		dlCmd.Env = envExtra
		setupProcessGroup(dlCmd)

		logFn(fmt.Sprintf("  ⬇️  %s: downloading dependencies (timeout 180s)...\n", name))
		dlOut, dlErr := dlCmd.CombinedOutput()
		dlCancel()

		if dlErr != nil {
			if dlCtx.Err() == context.DeadlineExceeded {
				logFn(fmt.Sprintf("  ⚠️  %s: dependency download timed out, trying build anyway\n", name))
				// Kill any remaining go processes
				killProcessGroup(dlCmd)
			} else {
				logFn(fmt.Sprintf("  ⚠️  go mod download failed: %s\n%s\n", dlErr, string(dlOut)))
			}
		} else {
			logFn(fmt.Sprintf("  ⬇️  %s: dependencies ready\n", name))
		}

		// Phase 2: Build with timeout and process group cleanup
		buildCtx, buildCancel := context.WithTimeout(ctx, 180*time.Second)
		logFn(fmt.Sprintf("  🔨 %s: building binary (timeout 180s)...\n", name))
		cmd := exec.CommandContext(buildCtx, goPath, "build", "-trimpath", "-ldflags=-s -w", "-o", binPath, ".")
		cmd.Dir = dir
		cmd.Env = envExtra
		setupProcessGroup(cmd)

		out, err := cmd.CombinedOutput()
		buildCancel()

		if err != nil {
			// Ensure the process is killed on timeout
			killProcessGroup(cmd)

			if buildCtx.Err() == context.DeadlineExceeded {
				logFn(fmt.Sprintf("  ❌ %s: build timed out (180s)\n", name))
				return result, fmt.Errorf("go build %s: timed out after 180s", dir)
			}
			logFn(fmt.Sprintf("  ❌ Compile failed: %s\n%s\n", err, string(out)))
			return result, fmt.Errorf("go build %s: %w\n%s", dir, err, string(out))
		}

		if info, err := os.Stat(binPath); err != nil || info.Size() == 0 {
			return result, fmt.Errorf("compiled binary is empty: %s", binPath)
		}

		logFn(fmt.Sprintf("  ✅ %s (%d KB)\n", name, fileSizeKB(binPath)))
		result.Recompiled = append(result.Recompiled, "system/bin/"+name)
		result.CacheMisses++

		// Store in binary cache
		mainFile := findMainFile(dir)
		if mainFile != "" {
			if err := StoreBinaryCache(projectDir, mainFile, binPath); err != nil {
				logFn(fmt.Sprintf("  ⚠️  Failed to cache binary: %v\n", err))
			}
		}
	}

	emitProgress("compile", len(packagesToCompile), len(packagesToCompile), "done")
	return result, nil
}

// findMainFile finds a .go file with package main in the directory.
func findMainFile(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.SplitN(string(data), "\n", 10) {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "package ") {
				if line == "package main" {
					return path
				}
				break
			}
		}
	}
	return ""
}

// ensureGoMod 确保目录中有 go.mod 文件
// 只在主模块根目录创建，不在子目录中创建（否则会破坏模块内包解析）
func ensureGoMod(goPath, dir, moduleName string) {
	goModPath := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		return
	}
	// 检查父目录是否有 go.mod（如果是子目录，不创建新的 go.mod）
	if parent := filepath.Dir(dir); parent != dir {
		if _, err := os.Stat(filepath.Join(parent, "go.mod")); err == nil {
			return // 父目录已有 go.mod，这是模块内的子目录，跳过
		}
	}
	cmd := exec.Command(goPath, "mod", "init", moduleName)
	cmd.Dir = dir
	cmd.Run()
}

// dirBaseName 从路径中提取目录名作为模块名
func dirBaseName(dir string) string {
	name := filepath.Base(dir)
	if name == "." || name == "go" || name == "src" {
		name = "module"
	}
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, name)
}

// hasMainPackage 检查目录中是否有 package main 的 Go 文件
func hasMainPackage(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for _, line := range strings.SplitN(string(data), "\n", 10) {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "package ") {
				return line == "package main"
			}
		}
	}
	return false
}

func fileSizeKB(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size() / 1024
}
