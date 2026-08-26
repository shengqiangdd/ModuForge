package code

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PackageManager 包管理器
type PackageManager struct {
	RootDir string
}

// NewPackageManager 创建包管理器
func NewPackageManager(rootDir string) *PackageManager {
	return &PackageManager{RootDir: rootDir}
}

// Dependency 依赖项
type Dependency struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"` // direct, indirect, dev
}

// PackageInfo 包信息
type PackageInfo struct {
	Language    string       `json:"language"`
	FileName    string       `json:"file_name"`
	Dependencies []Dependency `json:"dependencies"`
}

// AnalyzeDependencies 分析依赖
func (pm *PackageManager) AnalyzeDependencies() (*PackageInfo, error) {
	// 尝试查找 go.mod
	goModPath := filepath.Join(pm.RootDir, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		return pm.parseGoMod(goModPath)
	}

	// 尝试查找 package.json
	pkgJsonPath := filepath.Join(pm.RootDir, "package.json")
	if _, err := os.Stat(pkgJsonPath); err == nil {
		return pm.parsePackageJson(pkgJsonPath)
	}

	return nil, fmt.Errorf("no supported package file found")
}

func (pm *PackageManager) parseGoMod(path string) (*PackageInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	info := &PackageInfo{
		Language: "go",
		FileName: "go.mod",
		Dependencies: make([]Dependency, 0),
	}

	inRequireBlock := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "require (" {
			inRequireBlock = true
			continue
		}
		if line == ")" {
			inRequireBlock = false
			continue
		}

		if strings.HasPrefix(line, "require ") || inRequireBlock {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				name := parts[0]
				version := parts[1]
				info.Dependencies = append(info.Dependencies, Dependency{
					Name:    name,
					Version: version,
					Type:    "direct",
				})
			}
		}
	}

	return info, nil
}

func (pm *PackageManager) parsePackageJson(path string) (*PackageInfo, error) {
	// 简化的 JSON 解析，实际项目中应使用 encoding/json
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	info := &PackageInfo{
		Language: "javascript",
		FileName: "package.json",
		Dependencies: make([]Dependency, 0),
	}

	// 提取 dependencies
	if idx := strings.Index(content, "\"dependencies\""); idx != -1 {
		start := strings.Index(content[idx:], "{") + idx
		end := strings.Index(content[start:], "}") + start
		if end > start {
			depBlock := content[start+1 : end]
			lines := strings.Split(depBlock, ",")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "\"") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						name := strings.Trim(parts[0], "\" ")
						version := strings.Trim(parts[1], "\" ,")
						info.Dependencies = append(info.Dependencies, Dependency{
							Name:    name,
							Version: version,
							Type:    "direct",
						})
					}
				}
			}
		}
	}

	return info, nil
}
