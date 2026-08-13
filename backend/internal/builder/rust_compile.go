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

// rustInstallDir Rust 工具链安装目录
const rustInstallDir = "/usr/local/cargo"
const rustupHome = "/usr/local/rustup"

// RustAvailable 检查 Rust 工具链是否已安装
func RustAvailable() bool {
	rustc := filepath.Join(rustInstallDir, "bin", "rustc")
	_, err := os.Stat(rustc)
	return err == nil
}

// InstallRust installs Rust toolchain with default ARM64 target.
func InstallRust(ctx context.Context, logFn func(string)) error {
	return InstallRustArch(ctx, "arm64", logFn)
}

// InstallRustArch installs Rust toolchain and adds the target for the given architecture.
func InstallRustArch(ctx context.Context, arch string, logFn func(string)) error {
	if RustAvailable() {
		logFn("  Rust already installed, checking target...\n")
	} else {
		logFn("  Installing Rust toolchain...\n")

		cmd := exec.CommandContext(ctx, "sh", "-c",
			`curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable --profile minimal`)
		cmd.Env = append(os.Environ(),
			"HOME=/root",
			"CARGO_HOME="+rustInstallDir,
			"RUSTUP_HOME="+rustupHome,
		)

		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("rustup install failed: %w\n%s", err, string(out))
		}
		logFn("  ✅ Rust toolchain installed\n")
	}

	// Get the target triple for the architecture
	archInfo, err := GetArchInfo(arch)
	if err != nil {
		return fmt.Errorf("unsupported arch for rust: %w", err)
	}
	rustTarget := archInfo.RustTarget

	// Check if target is already added
	rustup := filepath.Join(rustInstallDir, "bin", "rustup")
	checkCmd := exec.CommandContext(ctx, rustup, "target", "list", "--installed")
	checkCmd.Env = append(os.Environ(),
		"HOME=/root",
		"CARGO_HOME="+rustInstallDir,
		"RUSTUP_HOME="+rustupHome,
	)
	out, _ := checkCmd.CombinedOutput()
	if strings.Contains(string(out), rustTarget) {
		logFn(fmt.Sprintf("  ✅ %s target already installed\n", rustTarget))
		return nil
	}

	logFn(fmt.Sprintf("  Adding %s target...\n", rustTarget))
	cmd := exec.CommandContext(ctx, rustup, "target", "add", rustTarget)
	cmd.Env = append(os.Environ(),
		"HOME=/root",
		"CARGO_HOME="+rustInstallDir,
		"RUSTUP_HOME="+rustupHome,
	)

	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rustup target add failed: %w\n%s", err, string(out))
	}
	logFn(fmt.Sprintf("  ✅ %s target added\n", rustTarget))

	return nil
}

// CompileRustProject compiles a Rust project with default ARM64 target.
func CompileRustProject(ctx context.Context, projectDir, cargoDir string, logFn func(string)) (string, error) {
	result, err := CompileRustProjectArch(ctx, projectDir, cargoDir, "arm64", nil, logFn)
	if err != nil {
		return "", err
	}
	if len(result.Recompiled) > 0 {
		return result.Recompiled[0], nil
	}
	return "", nil
}

// CompileRustProjectArch compiles a Rust project with arch support and binary caching.
func CompileRustProjectArch(ctx context.Context, projectDir, cargoDir, arch string, incr *IncrementalResult, logFn func(string)) (*CompileResult, error) {
	result := &CompileResult{}

	cargoPath := filepath.Join(rustInstallDir, "bin", "cargo")
	if _, err := os.Stat(cargoPath); err != nil {
		return nil, fmt.Errorf("cargo not found at %s", cargoPath)
	}

	archInfo, err := GetArchInfo(arch)
	if err != nil {
		return nil, fmt.Errorf("unsupported arch: %w", err)
	}
	rustTarget := archInfo.RustTarget

	pkgName := readCargoPackageName(cargoDir)
	if pkgName == "" {
		pkgName = filepath.Base(cargoDir)
	}

	binDir := filepath.Join(projectDir, "system", "bin")
	os.MkdirAll(binDir, 0755)
	binPath := filepath.Join(binDir, pkgName)

	// Check if we need to recompile
	if incr != nil && !incr.NeedsRebuild {
		cargoToml := filepath.Join(cargoDir, "Cargo.toml")
		if cached := CheckBinaryCache(projectDir, cargoToml); cached != nil {
			if input, err := os.ReadFile(*cached); err == nil {
				if err := os.WriteFile(binPath, input, 0755); err == nil {
					logCompileSkip(logFn, pkgName, "binary cache hit")
					result.CacheHits++
					result.Recompiled = append(result.Recompiled, "system/bin/"+pkgName)
					return result, nil
				}
			}
		}
	} else if incr != nil {
		// Check if this project's files changed
		changed := false
		for _, cf := range incr.ChangedFiles {
			if strings.HasPrefix(cf, cargoDir) {
				changed = true
				break
			}
		}
		if !changed {
			cargoToml := filepath.Join(cargoDir, "Cargo.toml")
			if cached := CheckBinaryCache(projectDir, cargoToml); cached != nil {
				if input, err := os.ReadFile(*cached); err == nil {
					if err := os.WriteFile(binPath, input, 0755); err == nil {
						logCompileSkip(logFn, pkgName, "binary cache hit (dir unchanged)")
						result.CacheHits++
						result.Recompiled = append(result.Recompiled, "system/bin/"+pkgName)
						return result, nil
					}
				}
			}
		}
	}

	logFn(fmt.Sprintf("  🔨 Compiling %s → system/bin/%s (%s)...\n", filepath.Base(cargoDir), pkgName, rustTarget))
	logFn(fmt.Sprintf("  🔨 %s: cargo building...\n", pkgName))

	// Set up Android NDK linker for cross-compilation
	ndkBin := ndkBinDir()
	linkerPath := filepath.Join(ndkBin, rustTarget+"21-clang")
	if _, err := os.Stat(linkerPath); err != nil {
		// Try without API level
		linkerPath = filepath.Join(ndkBin, rustTarget+"-clang")
	}

	// Create .cargo/config.toml for linker configuration
	cargoConfigDir := filepath.Join(cargoDir, ".cargo")
	os.MkdirAll(cargoConfigDir, 0755)
	cargoConfigPath := filepath.Join(cargoConfigDir, "config.toml")

	var configContent string
	if _, err := os.Stat(linkerPath); err == nil {
		configContent = fmt.Sprintf(`[target.%s]
linker = "%s"
`, rustTarget, linkerPath)
	} else {
		// Fallback: use cc from PATH (needs gcc installed)
		configContent = fmt.Sprintf(`[target.%s]
linker = "cc"
`, rustTarget)
	}

	if err := os.WriteFile(cargoConfigPath, []byte(configContent), 0644); err != nil {
		logFn(fmt.Sprintf("  ⚠️  Failed to write cargo config: %v\n", err))
	} else {
		logFn(fmt.Sprintf("  📝 Cargo config: linker=%s\n", linkerPath))
	}

	buildCtx, buildCancel := context.WithTimeout(ctx, 180*time.Second)
	cmd := exec.CommandContext(buildCtx, cargoPath, "build", "--release", "--target", rustTarget)
	cmd.Dir = cargoDir
	// Compute env-var key fragments from the rust target triple
	// e.g. "aarch64-linux-android" → "AARCH64_LINUX_ANDROID"
	targetEnvKey := strings.ReplaceAll(strings.ToUpper(rustTarget), "-", "_")

	// Derive the clang and llvm-ar paths from the linker path
	// linkerPath is typically ".../bin/aarch64-linux-android21-clang"
	ndkBinDir := filepath.Dir(linkerPath)
	ndkClang := linkerPath // e.g. ".../bin/aarch64-linux-android21-clang"
	ndkAr := filepath.Join(ndkBinDir, "llvm-ar")

	// cc crate uses lowercase target triple (hyphens→underscores, NOT uppercased)
	// e.g. "aarch64-linux-android" → "aarch64_linux_android"
	targetEnvKeyLower := strings.ReplaceAll(rustTarget, "-", "_")

	cmd.Env = append(os.Environ(),
		"HOME=/root",
		"CARGO_HOME="+rustInstallDir,
		"RUSTUP_HOME="+rustupHome,
		"PATH="+rustInstallDir+"/bin:"+os.Getenv("PATH"),
		// Cargo env vars use UPPERCASE target triple
		"CARGO_TARGET_"+targetEnvKey+"_LINKER="+ndkClang,
		"CARGO_TARGET_"+targetEnvKey+"_AR="+ndkAr,
		// cc crate env vars use lowercase target triple (case-sensitive!)
		"CC_"+targetEnvKeyLower+"="+ndkClang,
		"CXX_"+targetEnvKeyLower+"="+strings.Replace(ndkClang, "-clang", "-clang++", 1),
		"AR_"+targetEnvKeyLower+"="+ndkAr,
		// Host linker for build scripts (Alpine musl)
		"CC="+ndkClang,
		"CXX="+strings.Replace(ndkClang, "-clang", "-clang++", 1),
	)

	out, err := cmd.CombinedOutput()
	buildCancel()
	if err != nil {
		if buildCtx.Err() == context.DeadlineExceeded {
			logFn(fmt.Sprintf("  ❌ %s: cargo build timed out (180s)\n", pkgName))
			return nil, fmt.Errorf("cargo build %s: timed out after 180s", pkgName)
		}
		logFn(fmt.Sprintf("  ❌ Cargo build failed: %s\n%s\n", err, string(out)))
		return nil, fmt.Errorf("cargo build: %w\n%s", err, string(out))
	}

	// Cargo output path
	cargoBin := filepath.Join(cargoDir, "target", rustTarget, "release", pkgName)
	if info, err := os.Stat(cargoBin); err != nil || info.Size() == 0 {
		entries, _ := os.ReadDir(filepath.Join(cargoDir, "target", rustTarget, "release"))
		for _, e := range entries {
			if !e.IsDir() && e.Name() != pkgName {
				cargoBin = filepath.Join(cargoDir, "target", rustTarget, "release", e.Name())
				pkgName = e.Name()
				binPath = filepath.Join(binDir, pkgName)
				break
			}
		}
	}

	input, err := os.ReadFile(cargoBin)
	if err != nil {
		return nil, fmt.Errorf("read compiled binary: %w", err)
	}
	if err := os.WriteFile(binPath, input, 0755); err != nil {
		return nil, fmt.Errorf("write binary: %w", err)
	}

	// strip symbols
	stripPath := filepath.Join(rustInstallDir, "bin", "llvm-strip")
	if _, err := os.Stat(stripPath); err == nil {
		cmd = exec.CommandContext(ctx, stripPath, binPath)
		cmd.Run()
	}

	info, _ := os.Stat(binPath)
	sizeKB := int64(0)
	if info != nil {
		sizeKB = info.Size() / 1024
	}
	logFn(fmt.Sprintf("  ✅ %s (%d KB)\n", pkgName, sizeKB))

	result.Recompiled = append(result.Recompiled, "system/bin/"+pkgName)
	result.CacheMisses++

	// Store in binary cache
	cargoToml := filepath.Join(cargoDir, "Cargo.toml")
	if err := StoreBinaryCache(projectDir, cargoToml, binPath); err != nil {
		logFn(fmt.Sprintf("  ⚠️  Failed to cache binary: %v\n", err))
	}

	return result, nil
}

// readCargoPackageName reads the package name from Cargo.toml
func readCargoPackageName(cargoDir string) string {
	data, err := os.ReadFile(filepath.Join(cargoDir, "Cargo.toml"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name") && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.Trim(strings.TrimSpace(parts[1]), "\"' ")
				if name != "" {
					return name
				}
			}
		}
	}
	return ""
}
