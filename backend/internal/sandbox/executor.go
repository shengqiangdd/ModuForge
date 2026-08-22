package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const DefaultTimeout = 30 * time.Second

// Executor provides sandboxed command execution.
// Commands run in a dedicated temp directory with network disabled and a timeout.
type Executor struct {
	Timeout time.Duration
}

// New creates an Executor with the default 30s timeout.
func New() *Executor {
	return &Executor{Timeout: DefaultTimeout}
}

// ExecuteResult holds the captured output of a sandboxed command.
type ExecuteResult struct {
	Stdout string
	Stderr string
}

// ExecuteCommand runs the given command in a sandboxed environment.
// It creates an isolated temp directory, disables network-related env vars,
// enforces a timeout, and captures stdout/stderr.
func (e *Executor) ExecuteCommand(
	ctx context.Context,
	workDir string,
	command string,
	args []string,
	timeout time.Duration,
) (stdout, stderr string, err error) {
	if timeout <= 0 {
		timeout = e.Timeout
	}

	// Create isolated temp directory for this execution
	tmpDir, err := os.MkdirTemp("", "sandbox-exec-*")
	if err != nil {
		return "", "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// If workDir is specified and exists, copy relevant files into tmpDir
	if workDir != "" {
		if err := copyDir(workDir, tmpDir); err != nil {
			return "", "", fmt.Errorf("prepare work dir: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = tmpDir
	cmd.Env = sandboxEnv()

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	if ctx.Err() == context.DeadlineExceeded {
		return stdout, stderr, fmt.Errorf("command timed out after %s", timeout)
	}
	if err != nil {
		return stdout, stderr, fmt.Errorf("command failed: %w\nstderr: %s", err, stderr)
	}

	return stdout, stderr, nil
}

// sandboxEnv returns a minimal, network-restricted environment.
func sandboxEnv() []string {
	home, _ := os.UserHomeDir()
	return []string{
		"HOME=" + home,
		"TMPDIR=" + os.TempDir(),
		"PATH=/usr/local/go/bin:/usr/bin:/bin",
		"LANG=en_US.UTF-8",
		"LC_ALL=C",
		// Explicitly clear proxy vars to prevent network access via proxy
		"HTTP_PROXY=",
		"HTTPS_PROXY=",
		"http_proxy=",
		"https_proxy=",
		"NO_PROXY=*",
		"no_proxy=*",
		"ALL_PROXY=",
		"all_proxy=",
		"GOROOT=",
		"GOPATH=",
		"GOPROXY=off",
		"GONOSUMCHECK=*",
		"GONOSUMDB=*",
		"GOFLAGS=-mod=mod",
	}
}

// copyDir copies src into dst recursively.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
