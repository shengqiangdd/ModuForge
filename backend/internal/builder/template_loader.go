package builder

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/*
var templateFS embed.FS

// TemplateType represents the type of module template.
type TemplateType string

const (
	TemplateGoDaemon    TemplateType = "go_daemon"
	TemplateCModule     TemplateType = "c_kernel"
	TemplateShellModule TemplateType = "shell_module"
)

// TemplateInfo contains metadata about a template.
type TemplateInfo struct {
	Type        TemplateType `json:"type"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Language    string       `json:"language"`
	Files       []string     `json:"files"`
}

// GetAvailableTemplates returns all available templates.
func GetAvailableTemplates() []TemplateInfo {
	return []TemplateInfo{
		{
			Type:        TemplateGoDaemon,
			Name:        "Go Daemon",
			Description: "A complete Go daemon with signal handling, config loading, and periodic tasks",
			Language:    "go",
			Files:       []string{"main.go", "build.sh"},
		},
		{
			Type:        TemplateCModule,
			Name:        "C Module",
			Description: "A C module with signal handling, logging, and system calls",
			Language:    "c",
			Files:       []string{"main.c"},
		},
		{
			Type:        TemplateShellModule,
			Name:        "Shell Module",
			Description: "A Magisk module with install, service, and uninstall scripts",
			Language:    "shell",
			Files:       []string{"module.prop", "customize.sh", "service.sh", "uninstall.sh"},
		},
	}
}

// LoadTemplate loads a template and returns its files with variable substitution.
func LoadTemplate(templateType TemplateType, variables map[string]string) (map[string]string, error) {
	files := make(map[string]string)

	// Get template directory
	templateDir := filepath.Join("templates", string(templateType))

	// Read all files from embedded FS
	entries, err := templateFS.ReadDir(templateDir)
	if err != nil {
		return nil, fmt.Errorf("template not found: %s", templateType)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Read file content
		path := filepath.Join(templateDir, entry.Name())
		content, err := templateFS.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read template file %s: %w", path, err)
		}

		// Apply variable substitution
		processed := string(content)
		for key, value := range variables {
			placeholder := "{{" + strings.ToUpper(key) + "}}"
			processed = strings.ReplaceAll(processed, placeholder, value)
		}

		files[entry.Name()] = processed
	}

	return files, nil
}

// ApplyTemplateToProject applies a template to a project directory.
func ApplyTemplateToProject(projectDir string, templateType TemplateType, variables map[string]string) error {
	files, err := LoadTemplate(templateType, variables)
	if err != nil {
		return err
	}

	for filename, content := range files {
		path := filepath.Join(projectDir, filename)

		// Create directory if needed
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}

		// Write file
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("write file %s: %w", path, err)
		}
	}

	return nil
}
