package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/moduforge/backend/internal/config"
)

// targetToImage maps build targets to their container image names.
var targetToImage = map[string]string{
	"magisk":  "moduforge/builder-magisk:latest",
	"ksu":     "moduforge/builder-ksu:latest",
	"apatch":  "moduforge/builder-apatch:latest",
}

type Builder struct {
	cfg *config.Config
}

func NewBuilder(cfg *config.Config) *Builder {
	return &Builder{cfg: cfg}
}

// Build 主入口，检查 Docker 可用性，有 Docker 则用容器，否则回退到本地 zip
func (b *Builder) Build(ctx context.Context, projectDir, target, taskID string) (string, error) {
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		return "", fmt.Errorf("project dir not found: %s", projectDir)
	}

	artifactDir := filepath.Join(b.cfg.StoragePath, "artifacts", taskID)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return "", fmt.Errorf("create artifact dir: %w", err)
	}

	if b.dockerAvailable(ctx) {
		return b.buildWithDocker(ctx, projectDir, target, artifactDir)
	}
	return b.buildNative(ctx, projectDir, target, artifactDir)
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
	return outputZip, nil
}

// buildNative 本地 zip 打包（无 Docker 回退方案）
func (b *Builder) buildNative(ctx context.Context, sourceDir, target, artifactDir string) (string, error) {
	outputZip := filepath.Join(artifactDir, "module.zip")

	cmd := exec.CommandContext(ctx, "zip", "-r", outputZip, ".")
	cmd.Dir = sourceDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("zip failed: %s: %w", string(out), err)
	}
	return outputZip, nil
}
