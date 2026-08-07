package builder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetArchInfo_Valid(t *testing.T) {
	a, err := GetArchInfo("arm64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Goarch != "arm64" {
		t.Errorf("expected Goarch arm64, got %s", a.Goarch)
	}
	if a.RustTarget != "aarch64-linux-android" {
		t.Errorf("expected RustTarget aarch64-linux-android, got %s", a.RustTarget)
	}
	if a.MinAPI != 21 {
		t.Errorf("expected MinAPI 21, got %d", a.MinAPI)
	}
	if !a.Default {
		t.Error("expected arm64 to be default")
	}
}

func TestGetArchInfo_ARM(t *testing.T) {
	a, err := GetArchInfo("arm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.GOARM != "7" {
		t.Errorf("expected GOARM 7, got %s", a.GOARM)
	}
	if a.MinAPI != 16 {
		t.Errorf("expected MinAPI 16, got %d", a.MinAPI)
	}
}

func TestGetArchInfo_Invalid(t *testing.T) {
	_, err := GetArchInfo("risc-v")
	if err == nil {
		t.Fatal("expected error for unsupported architecture")
	}
}

func TestDefaultArch(t *testing.T) {
	a := DefaultArch()
	if a.ID != "arm64" {
		t.Errorf("expected arm64, got %s", a.ID)
	}
}

func TestValidateArch(t *testing.T) {
	if !ValidateArch("arm64") {
		t.Error("expected arm64 to be valid")
	}
	if !ValidateArch("arm") {
		t.Error("expected arm to be valid")
	}
	if !ValidateArch("x86_64") {
		t.Error("expected x86_64 to be valid")
	}
	if ValidateArch("mips") {
		t.Error("expected mips to be invalid")
	}
}

func TestNormalizeArch(t *testing.T) {
	if got := NormalizeArch("arm64"); got != "arm64" {
		t.Errorf("expected arm64, got %s", got)
	}
	if got := NormalizeArch(""); got != "arm64" {
		t.Errorf("expected arm64 (default), got %s", got)
	}
	if got := NormalizeArch("invalid"); got != "arm64" {
		t.Errorf("expected arm64 (default), got %s", got)
	}
}

func TestParseCMakeArgs(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{input: "target src1.c src2.c", expected: []string{"target", "src1.c", "src2.c"}},
		{input: `target "src with spaces.c"`, expected: []string{"target", "src with spaces.c"}},
		{input: "target", expected: []string{"target"}},
		{input: "", expected: nil},
		{input: "  spaced  args  ", expected: []string{"spaced", "args"}},
		{input: "target 'single quoted.c'", expected: []string{"target", "single quoted.c"}},
	}

	for _, tt := range tests {
		t.Run(tt.input[:min(len(tt.input), 20)], func(t *testing.T) {
			got := parseCMakeArgs(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, got)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected, got)
					return
				}
			}
		})
	}
}

func TestRelativePath(t *testing.T) {
	got := relativePath("/base/sub/file.txt", "/base")
	normalized := filepath.ToSlash(got)
	if normalized != "sub/file.txt" {
		t.Errorf("expected sub/file.txt, got %s", got)
	}

	// When paths are not under the same base, the function should still return a valid path
	got2 := relativePath("/base/file.txt", "/other")
	if got2 == "" {
		t.Error("expected non-empty relative path")
	}
}

func TestIsExcluded_DirectoryPattern(t *testing.T) {
	patterns := []string{"src/", ".git/"}
	if !isExcluded("src/main.go", patterns) {
		t.Error("expected src/main.go to be excluded by src/")
	}
	if !isExcluded(".git/config", patterns) {
		t.Error("expected .git/config to be excluded")
	}
	if isExcluded("README.md", patterns) {
		t.Error("expected README.md to NOT be excluded")
	}
}

func TestIsExcluded_GlobPattern(t *testing.T) {
	patterns := []string{"*.go", "*.md"}
	if !isExcluded("main.go", patterns) {
		t.Error("expected main.go to be excluded by *.go")
	}
	if !isExcluded("sub/dir/test.go", patterns) {
		t.Error("expected sub/dir/test.go to be excluded")
	}
	if !isExcluded("README.md", patterns) {
		t.Error("expected README.md to be excluded by *.md")
	}
	if isExcluded("main.c", patterns) {
		t.Error("expected main.c to NOT be excluded")
	}
}

func TestIsExcluded_ExactPattern(t *testing.T) {
	patterns := []string{"Makefile", "go.mod"}
	if !isExcluded("Makefile", patterns) {
		t.Error("expected Makefile to be excluded")
	}
	if !isExcluded("sub/Makefile", patterns) {
		t.Error("expected sub/Makefile to be excluded")
	}
	if !isExcluded("go.mod", patterns) {
		t.Error("expected go.mod to be excluded")
	}
	if isExcluded("main.go", patterns) {
		t.Error("expected main.go to NOT be excluded")
	}
}

func TestIsExcluded_NilPatterns(t *testing.T) {
	if isExcluded("anything", nil) {
		t.Error("expected nil patterns to not exclude anything")
	}
}

func TestIsExcluded_CaseInsensitive(t *testing.T) {
	patterns := []string{"makefile"}
	if !isExcluded("Makefile", patterns) {
		t.Error("expected case-insensitive matching for Makefile")
	}
	if !isExcluded("MAKEFILE", patterns) {
		t.Error("expected case-insensitive matching for MAKEFILE")
	}
}

func TestValidateProjectIntegrity_NoBuildFiles(t *testing.T) {
	dir := t.TempDir()
	result := ValidateProjectIntegrity(dir)
	if !result.Valid {
		t.Error("expected valid for empty directory")
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(result.Errors))
	}
}

func TestValidateProjectIntegrity_CMakeLists(t *testing.T) {
	dir := t.TempDir()
	// Create a source file and a CMakeLists.txt referencing it
	srcFile := filepath.Join(dir, "main.c")
	os.WriteFile(srcFile, []byte("int main() {}"), 0644)

	cmake := filepath.Join(dir, "CMakeLists.txt")
	content := "add_executable(myapp main.c)"
	os.WriteFile(cmake, []byte(content), 0644)

	result := ValidateProjectIntegrity(dir)
	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}
}

func TestValidateProjectIntegrity_CMakeLists_MissingFile(t *testing.T) {
	dir := t.TempDir()
	cmake := filepath.Join(dir, "CMakeLists.txt")
	content := "add_executable(myapp missing.c)"
	os.WriteFile(cmake, []byte(content), 0644)

	result := ValidateProjectIntegrity(dir)
	if result.Valid {
		t.Error("expected invalid due to missing source file")
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors[0].File != "CMakeLists.txt" {
		t.Errorf("expected error in CMakeLists.txt, got %s", result.Errors[0].File)
	}
}

func TestBuildCacheJSON_LoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	cache := loadBuildCache(dir)
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
	if cache.Files == nil {
		t.Error("expected non-nil Files map")
	}
	if cache.FullRebuild {
		t.Error("expected FullRebuild to be false for new cache")
	}
}

func TestBuildCacheJSON_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	cache := &BuildCacheJSON{
		Files: map[string]FileMtimeRecord{
			"main.c": {Path: "main.c", Hash: "abc123", Size: 100},
		},
		BuildTime:   "2024-01-01T00:00:00Z",
		Arch:        "arm64",
		Target:      "magisk",
		FullRebuild: false,
	}

	if err := saveBuildCache(dir, cache); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded := loadBuildCache(dir)
	if loaded.BuildTime != cache.BuildTime {
		t.Errorf("expected %s, got %s", cache.BuildTime, loaded.BuildTime)
	}
	if loaded.Files["main.c"].Hash != "abc123" {
		t.Errorf("expected hash abc123, got %s", loaded.Files["main.c"].Hash)
	}
}

func TestZipModuleExcludePatterns(t *testing.T) {
	// Verify that the exclusion list is non-empty
	if len(ModuleExcludePatterns) == 0 {
		t.Error("expected non-empty exclusion patterns")
	}

	// Source code patterns should be present
	foundGo := false
	for _, p := range ModuleExcludePatterns {
		if p == "*.go" {
			foundGo = true
			break
		}
	}
	if !foundGo {
		t.Error("expected *.go in exclusion patterns")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}