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

// reorganizeGoModule 在编译前检测 Go 文件的 import 路径，自动创建子目录结构。
// 解决的问题：Go 源文件用模块路径（如 androwui/config）import 内部包，
// 但文件全在同一目录，Go 编译器找不到这些子包。
func reorganizeGoModule(projectDir string, logFn func(string)) error {
	// 1. 收集所有 .go 文件
	var goFiles []string
	_ = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})
	if len(goFiles) == 0 {
		return nil
	}

	// 2. 检测 go.mod 中的模块名
	// Go 文件可能在子目录中（如 src/go/），需要在该目录查找 go.mod
	var moduleName string
	var goModPath string
	var goModDir string

	// 收集 .go 文件所在的目录
	goDirs := make(map[string]bool)
	for _, fpath := range goFiles {
		goDirs[filepath.Dir(fpath)] = true
	}

	// 先在 projectDir 查找，再在包含 .go 文件的子目录中查找
	searchDirs := []string{projectDir}
	for d := range goDirs {
		if d != projectDir {
			searchDirs = append(searchDirs, d)
		}
	}

	for _, dir := range searchDirs {
		p := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(p); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					moduleName = strings.TrimSpace(strings.TrimPrefix(line, "module"))
					goModPath = p
					goModDir = dir
					break
				}
			}
			if moduleName != "" {
				break
			}
		}
	}

	if moduleName == "" {
		// 没有 go.mod 或没有 module 声明，跳过重组
		return nil
	}

	// 3. 扫描所有 .go 文件的 import，找出模块内部的 import 路径
	internalImports := make(map[string]bool) // e.g. "androwui/config" -> true
	for _, fpath := range goFiles {
		data, err := os.ReadFile(fpath)
		if err != nil {
			continue
		}
		content := string(data)
		inImport := false
		for _, line := range strings.Split(content, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "import (" {
				inImport = true
				continue
			}
			if inImport && trimmed == ")" {
				inImport = false
				continue
			}
			if trimmed == "" || strings.HasPrefix(trimmed, "//") {
				continue
			}
			// 提取 import 路径
			importPath := trimmed
			if inImport {
				// 可能是 alias "path" 格式
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					importPath = parts[len(parts)-1]
				}
				importPath = strings.Trim(importPath, "\"")
			} else if strings.HasPrefix(trimmed, "import") {
				// 单行 import: import "path"
				importPath = strings.TrimSpace(strings.TrimPrefix(trimmed, "import"))
				importPath = strings.Trim(importPath, "\"")
			} else {
				continue
			}

			// 检查是否是模块内部的 import（以模块名为前缀）
			if strings.HasPrefix(importPath, moduleName+"/") {
				subPkg := strings.TrimPrefix(importPath, moduleName+"/")
				// 只处理第一级子包（如 androwui/config，不处理 androwui/config/sub）
				if !strings.Contains(subPkg, "/") {
					internalImports[importPath] = true
				}
			}
		}
	}

	if len(internalImports) == 0 {
		return nil // 没有内部 import，不需要重组
	}

	logFn(fmt.Sprintf("  🔀 Found %d internal Go imports, reorganizing module structure...\n", len(internalImports)))

	// 4. 根据文件名推断归属包，创建子目录并移动
	// 当源文件 package 声明有误（如全写成 package main）时，
	// 通过文件名与 import 路径的对应关系推断正确归属
	// 建立 包名 -> import路径 的映射
	pkgToImport := make(map[string]string) // e.g. "config" -> "androwui/config"
	for imp := range internalImports {
		parts := strings.Split(imp, "/")
		if len(parts) > 0 {
			pkgName := parts[len(parts)-1]
			pkgToImport[pkgName] = imp
		}
	}

	for _, fpath := range goFiles {
		baseName := strings.TrimSuffix(filepath.Base(fpath), ".go")
		// 查找匹配的 import 路径（如 config.go -> androwui/config）
		if importPath, ok := pkgToImport[baseName]; ok {
			subDir := strings.TrimPrefix(importPath, moduleName+"/")
			targetDir := filepath.Join(goModDir, subDir)
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				logFn(fmt.Sprintf("  ⚠️  Failed to create dir %s: %v\n", targetDir, err))
				continue
			}
			targetPath := filepath.Join(targetDir, filepath.Base(fpath))
			if fpath == targetPath {
				continue // 已经在正确位置
			}
			if err := os.Rename(fpath, targetPath); err != nil {
				logFn(fmt.Sprintf("  ⚠️  Failed to move %s -> %s: %v\n", fpath, targetPath, err))
				continue
			}
			// 修正 package 声明：如果文件名匹配子包，将 package main 改为正确的包名
			pkgName := baseName
			if data, err := os.ReadFile(targetPath); err == nil {
				content := string(data)
				oldDecl := "package main"
				newDecl := "package " + pkgName
				if strings.Contains(content, oldDecl) {
					content = strings.Replace(content, oldDecl, newDecl, 1)
					_ = os.WriteFile(targetPath, []byte(content), 0644)
					logFn(fmt.Sprintf("  📁 Moved %s -> %s/ (package %s)\n", filepath.Base(fpath), subDir, pkgName))
				}
			}
		}
	}

	// 6. 确保 go.mod 中的模块名与重组后的结构一致（只更新 module 行，保留依赖）
	if data, err := os.ReadFile(goModPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "module ") {
				lines[i] = "module " + moduleName
				break
			}
		}
		if err := os.WriteFile(goModPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
			logFn(fmt.Sprintf("  ⚠️  Failed to update go.mod: %v\n", err))
		}
	}

	logFn(fmt.Sprintf("  ✅ Go module reorganized: %s (go.mod at %s)\n", moduleName, goModDir))

	// 7. 清理未使用的 import（代码生成可能产生未使用的导入）
	cleanUnusedImports(goModDir, logFn)

	return nil
}

// cleanUnusedImports 移除未使用的 import
func cleanUnusedImports(dir string, logFn func(string)) {
	// 收集所有 .go 文件
	var goFiles []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		goFiles = append(goFiles, path)
		return nil
	})

	for _, fpath := range goFiles {
		data, err := os.ReadFile(fpath)
		if err != nil {
			continue
		}
		content := string(data)

		// 提取 import 块中的包名
		importBlock := extractImportBlock(content)
		if importBlock == "" {
			continue
		}
		importedPkgs := parseImportNames(importBlock)
		if len(importedPkgs) == 0 {
			continue
		}

		// 检查每个导入的包是否在代码中被使用
		// 移除 import 块后的代码部分
		codeAfterImports := content
		if idx := strings.Index(content, ")"); idx != -1 {
			// 找到 import 块的结束括号
			importEnd := strings.Index(content, "import")
			if importEnd != -1 {
				parenEnd := strings.Index(content[importEnd:], ")")
				if parenEnd != -1 {
					codeAfterImports = content[importEnd+parenEnd+1:]
				}
			}
		}

		var unused []string
		for _, pkg := range importedPkgs {
			// 检查包名是否在代码中被使用（简单文本匹配）
			if !strings.Contains(codeAfterImports, pkg+".") && !strings.Contains(codeAfterImports, pkg+"{") && !strings.Contains(codeAfterImports, pkg+"(") {
				unused = append(unused, pkg)
			}
		}

		if len(unused) == 0 {
			continue
		}

		// 移除未使用的 import
		lines := strings.Split(content, "\n")
		var newLines []string
		inImport := false
		importStarted := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "import (" {
				inImport = true
				importStarted = true
				newLines = append(newLines, line)
				continue
			}
			if inImport && trimmed == ")" {
				inImport = false
				newLines = append(newLines, line)
				continue
			}
			if inImport {
				// 检查这一行是否是未使用的 import
				skip := false
				for _, pkg := range unused {
					if strings.Contains(line, pkg) && (strings.Contains(line, "\"") || strings.Contains(line, pkg)) {
						skip = true
						break
					}
				}
				if skip {
					continue
				}
			}
			newLines = append(newLines, line)
		}

		if importStarted {
			newContent := strings.Join(newLines, "\n")
			if newContent != content {
				_ = os.WriteFile(fpath, []byte(newContent), 0644)
				logFn(fmt.Sprintf("  🧹 Cleaned unused imports in %s: removed %v\n", filepath.Base(fpath), unused))
			}
		}
	}
}

// extractImportBlock 提取 import (...) 块的内容
func extractImportBlock(content string) string {
	start := strings.Index(content, "import (")
	if start == -1 {
		return ""
	}
	start += len("import (")
	end := strings.Index(content[start:], ")")
	if end == -1 {
		return ""
	}
	return content[start : start+end]
}

// parseImportNames 从 import 块中解析出包名
func parseImportNames(block string) []string {
	var names []string
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		// 提取包名：可能是 "pkg" 或 alias "pkg"
		parts := strings.Fields(line)
		if len(parts) == 1 {
			// "pkg" 或 "pkg"
			pkg := strings.Trim(parts[0], "\"")
			if idx := strings.LastIndex(pkg, "/"); idx != -1 {
				pkg = pkg[idx+1:]
			}
			names = append(names, pkg)
		} else if len(parts) == 2 {
			// alias "pkg"
			names = append(names, parts[0])
		}
	}
	return names
}

// goBuildEnv returns the environment variables needed for Go cross-compilation.
func goBuildEnv(goarch, goarm string) []string {
	env := append(os.Environ(),
		"GOOS=android",
		fmt.Sprintf("GOARCH=%s", goarch),
		"CGO_ENABLED=0",
		// Use persistent GOMODCACHE for dependency caching
		"GOMODCACHE="+getGoModCache(),
		"GOPATH="+getGoPath(),
	)
	if goarm != "" {
		env = append(env, fmt.Sprintf("GOARM=%s", goarm))
	}
	return env
}

// getGoModCache returns the persistent GOMODCACHE path.
func getGoModCache() string {
	if v := os.Getenv("GOMODCACHE"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}
	return filepath.Join(home, ".cache", "moduforge", "gomodcache")
}

// getGoPath returns the persistent GOPATH.
func getGoPath() string {
	if v := os.Getenv("GOPATH"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}
	return filepath.Join(home, ".cache", "moduforge", "gopath")
}

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
	binDir := filepath.Join(projectDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return nil, fmt.Errorf("create bin: %w", err)
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
							result.Recompiled = append(result.Recompiled, "bin/"+name)
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
								result.Recompiled = append(result.Recompiled, "bin/"+name)
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
		logFn(fmt.Sprintf("  🔨 Compiling %s → bin/%s (android/%s) [%d/%d]...\n", name, name, goarch, i+1, len(packagesToCompile)))

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
		result.Recompiled = append(result.Recompiled, "bin/"+name)
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

// ensureGoCacheDirs creates the Go module and build cache directories.
func ensureGoCacheDirs(logFn func(string)) {
	dirs := []string{getGoModCache(), getGoPath()}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			logFn(fmt.Sprintf("  ⚠️  Failed to create cache dir %s: %v\n", d, err))
		}
	}
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
