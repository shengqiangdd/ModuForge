package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type CreateDirSkill struct {
	projectPath string
}

func NewCreateDirSkill(projectPath string) *CreateDirSkill {
	return &CreateDirSkill{projectPath: projectPath}
}

func (s *CreateDirSkill) Name() string {
	return "create_dir"
}

func (s *CreateDirSkill) Description() string {
	return "Create an empty directory. NOTE: write_file auto-creates parent directories, so you rarely need this. Only use for creating empty dirs without files. Input: {\"path\": \"...\"}"
}

func (s *CreateDirSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, _ := input["path"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	fullPath := filepath.Join(s.projectPath, path)

	// Security: prevent path traversal
	if !filepath.HasPrefix(fullPath, filepath.Clean(s.projectPath)) {
		return "", fmt.Errorf("path traversal not allowed: %s", path)
	}

	err := os.MkdirAll(fullPath, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory %s: %v", path, err)
	}

	// Verify it actually exists
	info, err := os.Stat(fullPath)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("directory %s was not created successfully", path)
	}

	return fmt.Sprintf("Directory created: %s", path), nil
}

func (s *CreateDirSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  false,
		Essential: true,
		NeedsDB:   false,
		NeedsLLM:  false,
	}
}
