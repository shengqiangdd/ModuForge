package builder

import (
	"fmt"
	"os"
	"path/filepath"
)

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

// ensureGoCacheDirs creates the Go module and build cache directories.
func ensureGoCacheDirs(logFn func(string)) {
	dirs := []string{getGoModCache(), getGoPath()}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			logFn(fmt.Sprintf("  ⚠️  Failed to create cache dir %s: %v\n", d, err))
		}
	}
}
