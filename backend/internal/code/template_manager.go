package code

import (
	"fmt"
	"strings"
	"time"
)

// TemplateManager 项目模板管理器
type TemplateManager struct {
	templates map[string]*ProjectTemplate
}

// NewTemplateManager 创建模板管理器
func NewTemplateManager() *TemplateManager {
	mgr := &TemplateManager{
		templates: make(map[string]*ProjectTemplate),
	}
	mgr.loadDefaultTemplates()
	return mgr
}

// ProjectTemplate 项目模板
type ProjectTemplate struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Language    string             `json:"language"`
	Category    string             `json:"category"`
	Files       []TemplateFile     `json:"files"`
	Variables   []TemplateVariable `json:"variables"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

// TemplateFile 模板文件
type TemplateFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	IsDir   bool   `json:"is_dir"`
}

// TemplateVariable 模板变量
type TemplateVariable struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Default     string `json:"default"`
	Required    bool   `json:"required"`
}

// ListTemplates 列出所有模板
func (m *TemplateManager) ListTemplates() []*ProjectTemplate {
	templates := make([]*ProjectTemplate, 0)
	for _, t := range m.templates {
		templates = append(templates, t)
	}
	return templates
}

// GetTemplate 获取模板
func (m *TemplateManager) GetTemplate(id string) (*ProjectTemplate, bool) {
	t, ok := m.templates[id]
	return t, ok
}

// CreateTemplate 创建模板
func (m *TemplateManager) CreateTemplate(t *ProjectTemplate) {
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	m.templates[t.ID] = t
}

// GenerateProject 从模板生成项目
func (m *TemplateManager) GenerateProject(templateID string, variables map[string]string) ([]TemplateFile, error) {
	t, ok := m.templates[templateID]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	files := make([]TemplateFile, 0)
	for _, f := range t.Files {
		content := f.Content
		for k, v := range variables {
			content = strings.ReplaceAll(content, "{{"+k+"}}", v)
		}
		files = append(files, TemplateFile{
			Path:    f.Path,
			Content: content,
			IsDir:   f.IsDir,
		})
	}

	return files, nil
}

func (m *TemplateManager) loadDefaultTemplates() {
	m.CreateTemplate(&ProjectTemplate{
		ID:          "go-web-api",
		Name:        "Go Web API",
		Description: "使用 Fiber 框架的 Go Web API 项目",
		Language:    "go",
		Category:    "web",
		Files: []TemplateFile{
			{Path: "main.go", Content: `package main

import (
	"log"
	"github.com/gofiber/fiber/v3"
)

func main() {
	app := fiber.New()

	app.Get("/", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Hello, {{project_name}}!",
		})
	})

	log.Fatal(app.Listen(":3000"))
}`},
			{Path: "go.mod", Content: `module {{module_name}}

go 1.21

require github.com/gofiber/fiber/v3 v3.0.0`},
			{Path: "README.md", Content: `# {{project_name}}

{{description}}

## 运行

` + "```bash" + `
go run main.go
` + "```" + `
`},
		},
		Variables: []TemplateVariable{
			{Name: "project_name", Description: "项目名称", Default: "my-api", Required: true},
			{Name: "module_name", Description: "Go模块名", Default: "github.com/user/my-api", Required: true},
			{Name: "description", Description: "项目描述", Default: "A Go web API", Required: false},
		},
	})

	m.CreateTemplate(&ProjectTemplate{
		ID:          "python-cli",
		Name:        "Python CLI",
		Description: "Python 命令行工具项目",
		Language:    "python",
		Category:    "cli",
		Files: []TemplateFile{
			{Path: "main.py", Content: `#!/usr/bin/env python3
"""{{project_name}} - {{description}}"""

import argparse
import sys

def main():
    parser = argparse.ArgumentParser(description="{{description}}")
    parser.add_argument("command", help="Command to execute")
    args = parser.parse_args()

    print(f"Running {args.command}...")

if __name__ == "__main__":
    main()`},
			{Path: "requirements.txt", Content: `# {{project_name}} dependencies
`},
			{Path: "setup.py", Content: `from setuptools import setup, find_packages

setup(
    name="{{project_name}}",
    version="0.1.0",
    packages=find_packages(),
    entry_points={
        "console_scripts": [
            "{{project_name}}=main:main",
        ],
    },
)`},
		},
		Variables: []TemplateVariable{
			{Name: "project_name", Description: "项目名称", Default: "my-cli", Required: true},
			{Name: "description", Description: "项目描述", Default: "A Python CLI tool", Required: false},
		},
	})

	m.CreateTemplate(&ProjectTemplate{
		ID:          "js-react",
		Name:        "JavaScript React",
		Description: "React 前端项目模板",
		Language:    "javascript",
		Category:    "frontend",
		Files: []TemplateFile{
			{Path: "package.json", Content: `{
  "name": "{{project_name}}",
  "version": "0.1.0",
  "description": "{{description}}",
  "main": "index.js",
  "scripts": {
    "start": "react-scripts start",
    "build": "react-scripts build"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0"
  }
}`},
			{Path: "src/App.js", Content: `import React from 'react';

function App() {
  return (
    <div className="App">
      <h1>{{project_name}}</h1>
      <p>{{description}}</p>
    </div>
  );
}

export default App;`},
			{Path: "src/index.js", Content: `import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';

const root = ReactDOM.createRoot(document.getElementById('root'));
root.render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);`},
		},
		Variables: []TemplateVariable{
			{Name: "project_name", Description: "项目名称", Default: "my-app", Required: true},
			{Name: "description", Description: "项目描述", Default: "A React application", Required: false},
		},
	})
}
