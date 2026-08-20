package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
