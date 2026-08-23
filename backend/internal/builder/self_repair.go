package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultMaxRetries   = 3
	defaultBuildTimeout = 30 * time.Second
)

// GenerateAndRepair implements a self-repair loop:
// 1. Calls LLM to generate code from description
// 2. Writes generated code to a temp directory
// 3. Attempts to build (go build / bash -n)
// 4. On failure, feeds error + original prompt back to LLM for a fix
// 5. Repeats until success or maxRetries reached
func (b *Builder) GenerateAndRepair(
	ctx context.Context,
	projectDir string,
	description string,
	maxRetries int,
	logFn func(string),
) error {
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}
	if logFn == nil {
		logFn = func(string) {}
	}

	endpoint, apiKey, model := b.resolveLLMForFix()
	if endpoint == "" || apiKey == "" {
		return fmt.Errorf("no LLM configured for self-repair")
	}

	// Create isolated temp directory for code generation
	tmpDir, err := os.MkdirTemp("", "self-repair-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	logFn(fmt.Sprintf("[SelfRepair] Using temp dir: %s\n", tmpDir))

	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		logFn(fmt.Sprintf("[SelfRepair] Attempt %d/%d\n", attempt, maxRetries))

		// Step 1: Generate or repair code via LLM
		var code string
		if attempt == 1 {
			code, err = b.generateCode(ctx, endpoint, apiKey, model, description, logFn)
		} else {
			code, err = b.repairCode(ctx, endpoint, apiKey, model, description, lastErr, logFn)
		}
		if err != nil {
			logFn(fmt.Sprintf("[SelfRepair] LLM call failed: %v\n", err))
			lastErr = err
			continue
		}

		// Step 2: Write generated code to temp directory
		if err := writeGeneratedFiles(tmpDir, code); err != nil {
			logFn(fmt.Sprintf("[SelfRepair] Failed to write files: %v\n", err))
			lastErr = err
			continue
		}

		// Step 3: Detect and run appropriate build command
		stdout, stderr, buildErr := b.runBuildCommand(ctx, tmpDir, logFn)
		if buildErr == nil {
			// Build succeeded — copy result to projectDir
			if err := copyDirSelfRepair(tmpDir, projectDir); err != nil {
				logFn(fmt.Sprintf("[SelfRepair] Failed to copy to project dir: %v\n", err))
				lastErr = err
				continue
			}
			logFn(fmt.Sprintf("[SelfRepair] Build succeeded on attempt %d\n", attempt))
			return nil
		}

		// Step 4: Build failed — capture error for next iteration
		errMsg := fmt.Sprintf("stdout:\n%s\nstderr:\n%s\nerror: %v", stdout, stderr, buildErr)
		logFn(fmt.Sprintf("[SelfRepair] Build failed: %s\n", truncate(errMsg, 500)))
		lastErr = fmt.Errorf("%s", errMsg)
	}

	return fmt.Errorf("self-repair failed after %d attempts: %w", maxRetries, lastErr)
}

// generateCode calls the LLM to generate code from a description.
func (b *Builder) generateCode(
	ctx context.Context,
	endpoint, apiKey, model string,
	description string,
	logFn func(string),
) (string, error) {
	prompt := fmt.Sprintf(`You are an expert Go developer for Android Magisk modules.
Generate a complete, compilable Go project based on this description.

## Description
%s

## Rules
- Use only Go standard library (no third-party dependencies)
- Package main with func main()
- All variables must be declared and used
- Handle all errors
- Use only: int, int64, string, bool, float64, []byte, error, map, slice
- Include go.mod with module path "selfrepair" and go version "1.21"
- Output format: JSON array of files
[{"path":"go.mod","content":"..."},{"path":"main.go","content":"..."}]

Return ONLY the JSON array.`, description)

	logFn("[SelfRepair] Generating code from description...\n")
	return callLLMForFix(ctx, endpoint, apiKey, model, prompt)
}

// repairCode calls the LLM to fix code that failed to build.
func (b *Builder) repairCode(
	ctx context.Context,
	endpoint, apiKey, model string,
	description string,
	buildErr error,
	logFn func(string),
) (string, error) {
	errMsg := buildErr.Error()
	if len(errMsg) > 3000 {
		errMsg = errMsg[len(errMsg)-3000:]
	}

	prompt := fmt.Sprintf(`You are an expert Go developer. Your previous code had build errors.
Fix ALL errors and return the COMPLETE corrected code.

## Original Description
%s

## Build Errors
%s

## Rules
- Use only Go standard library (no third-party dependencies)
- Package main with func main()
- All variables must be declared and used
- Handle all errors
- Use only: int, int64, string, bool, float64, []byte, error, map, slice
- Include go.mod with module path "selfrepair" and go version "1.21"
- Output format: JSON array of files
[{"path":"go.mod","content":"..."},{"path":"main.go","content":"..."}]

Return ONLY the JSON array.`, description, errMsg)

	logFn("[SelfRepair] Requesting LLM repair...\n")
	return callLLMForFix(ctx, endpoint, apiKey, model, prompt)
}

// runBuildCommand detects the project type and runs the appropriate build.
func (b *Builder) runBuildCommand(
	ctx context.Context,
	projectDir string,
	logFn func(string),
) (stdout, stderr string, err error) {
	// Detect project type by looking for file extensions
	hasGo := false
	hasShell := false

	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		switch {
		case strings.HasSuffix(path, ".go"):
			hasGo = true
		case strings.HasSuffix(path, ".sh"):
			hasShell = true
		}
		return nil
	})

	if hasGo {
		return b.runGoBuild(ctx, projectDir, logFn)
	}
	if hasShell {
		return b.runBashSyntaxCheck(ctx, projectDir, logFn)
	}

	// Fallback: try go build
	return b.runGoBuild(ctx, projectDir, logFn)
}

// runGoBuild runs "go build" in the given directory.
func (b *Builder) runGoBuild(
	ctx context.Context,
	projectDir string,
	logFn func(string),
) (stdout, stderr string, err error) {
	// Find go binary
	goPath, lookErr := exec.LookPath("go")
	if lookErr != nil {
		for _, p := range []string{"/usr/local/go/bin/go", "/usr/bin/go"} {
			if _, statErr := os.Stat(p); statErr == nil {
				goPath = p
				break
			}
		}
	}
	if goPath == "" {
		return "", "", fmt.Errorf("go compiler not found")
	}

	ctx, cancel := context.WithTimeout(ctx, defaultBuildTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, goPath, "build", "-o", "/dev/null", ".")
	cmd.Dir = projectDir
	cmd.Env = goBuildEnv("arm64", "")

	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	if ctx.Err() == context.DeadlineExceeded {
		return stdout, stderr, fmt.Errorf("go build timed out after %s", defaultBuildTimeout)
	}

	return stdout, stderr, err
}

// runBashSyntaxCheck runs "bash -n" for shell script syntax validation.
func (b *Builder) runBashSyntaxCheck(
	ctx context.Context,
	projectDir string,
	logFn func(string),
) (stdout, stderr string, err error) {
	var allErr []string

	err = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".sh") {
			return nil
		}

		ctx, cancel := context.WithTimeout(ctx, defaultBuildTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, "bash", "-n", path)
		var stderrBuf strings.Builder
		cmd.Stderr = &stderrBuf

		if cmdErr := cmd.Run(); cmdErr != nil {
			allErr = append(allErr, fmt.Sprintf("%s: %s", path, stderrBuf.String()))
		}
		return nil
	})

	if len(allErr) > 0 {
		return "", strings.Join(allErr, "\n"), fmt.Errorf("bash syntax check failed")
	}
	return "", "", nil
}

// writeGeneratedFiles parses the LLM JSON response and writes files to dir.
func writeGeneratedFiles(dir string, llmResponse string) error {
	code := extractCodeFromResponse(llmResponse)

	// Try to parse as JSON array of {path, content} objects
	type fileEntry struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	var files []fileEntry

	// Try standard JSON unmarshal
	if err := json.Unmarshal([]byte(code), &files); err != nil || len(files) == 0 {
		// Fallback: try parseJSONArray, then treat as single file
		var parsed []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if parseErr := parseJSONArray(code, &parsed); parseErr == nil && len(parsed) > 0 {
			for _, p := range parsed {
				files = append(files, fileEntry{Path: p.Path, Content: p.Content})
			}
		} else {
			files = []fileEntry{{Path: "main.go", Content: code}}
		}
	}

	for _, f := range files {
		if f.Path == "" {
			continue
		}
		target := filepath.Join(dir, f.Path)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("create dir for %s: %w", f.Path, err)
		}
		if err := os.WriteFile(target, []byte(f.Content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", f.Path, err)
		}
	}

	return nil
}

// parseFileEntries tries to parse JSON file entries from LLM response.
func parseFileEntries(code string, files *[]struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}) error {
	// Find JSON array boundaries
	start := strings.Index(code, "[")
	end := strings.LastIndex(code, "]")
	if start < 0 || end < 0 || end <= start {
		return fmt.Errorf("no JSON array found")
	}
	return parseJSONArray(code[start:end+1], files)
}

// parseJSONArray does minimal JSON parsing for [{"path":"...","content":"..."}].
func parseJSONArray(jsonStr string, files *[]struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}) error {
	// Find each object
	pos := 0
	for pos < len(jsonStr) {
		start := strings.Index(jsonStr[pos:], "{")
		if start < 0 {
			break
		}
		pos += start
		end := findMatchingBrace(jsonStr, pos)
		if end < 0 {
			break
		}
		obj := jsonStr[pos : end+1]
		pos = end + 1

		path := extractJSONString(obj, "path")
		content := extractJSONString(obj, "content")
		if path != "" {
			*files = append(*files, struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}{Path: path, Content: content})
		}
	}
	return nil
}

// findMatchingBrace finds the closing brace matching the opening brace at pos.
func findMatchingBrace(s string, pos int) int {
	if pos >= len(s) || s[pos] != '{' {
		return -1
	}
	depth := 0
	inString := false
	escape := false
	for i := pos; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' && inString {
			escape = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// extractJSONString extracts the value of a key from a JSON object string.
func extractJSONString(obj, key string) string {
	search := `"` + key + `":`
	idx := strings.Index(obj, search)
	if idx < 0 {
		return ""
	}
	valStart := idx + len(search)
	// Skip whitespace
	for valStart < len(obj) && obj[valStart] == ' ' {
		valStart++
	}
	if valStart >= len(obj) || obj[valStart] != '"' {
		return ""
	}
	valStart++ // skip opening quote
	valEnd := valStart
	for valEnd < len(obj) {
		if obj[valEnd] == '\\' {
			valEnd += 2
			continue
		}
		if obj[valEnd] == '"' {
			break
		}
		valEnd++
	}
	if valEnd >= len(obj) {
		return ""
	}
	val := obj[valStart:valEnd]
	// Unescape \n to actual newlines
	val = strings.ReplaceAll(val, `\n`, "\n")
	val = strings.ReplaceAll(val, `\"`, `"`)
	val = strings.ReplaceAll(val, `\\`, `\`)
	return val
}

// copyDirSelfRepair copies src directory contents into dst recursively.
// Named to avoid collision with other copyDir definitions in the package.
func copyDirSelfRepair(src, dst string) error {
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
