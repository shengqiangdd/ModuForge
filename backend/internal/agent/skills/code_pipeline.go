package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CodePipelineSkill 编码管道：生成 → 构建验证 → Lint → 修复 → 测试 → 修复 → 输出
// 核心改进：加入真实编译环节，构建失败信息反馈给 LLM 修复
type CodePipelineSkill struct {
	apiKey   string
	endpoint string
	model    string
	client   *http.Client
}

func NewCodePipelineSkill(apiKey, endpoint, model string) *CodePipelineSkill {
	return &CodePipelineSkill{
		apiKey:   apiKey,
		endpoint: endpoint,
		model:    model,
		client:   http.DefaultClient,
	}
}

// NewCodePipelineSkillWithClient creates a CodePipelineSkill that reuses a shared
// HTTP client (with connection pooling) instead of allocating a new one per skill.
func NewCodePipelineSkillWithClient(apiKey, endpoint, model string, client *http.Client) *CodePipelineSkill {
	if client == nil {
		client = http.DefaultClient
	}
	return &CodePipelineSkill{
		apiKey:   apiKey,
		endpoint: endpoint,
		model:    model,
		client:   client,
	}
}

func (s *CodePipelineSkill) Name() string {
	return "code_pipeline"
}

func (s *CodePipelineSkill) Description() string {
	return `Full code pipeline: generate → build verify → lint → fix → test → fix → output.
Input: {"description": "...", "files_spec": "..."}.
Generates code, attempts compilation, and iteratively fixes build errors.
Supports: shell scripts, Rust, C/C++, Go modules for Magisk/KernelSU/APatch.`
}

func (s *CodePipelineSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	description, _ := input["description"].(string)
	filesSpec, _ := input["files_spec"].(string)

	if description == "" {
		return "", fmt.Errorf("description is required")
	}

	// Create temp working directory for compilation tests
	tmpDir, err := os.MkdirTemp("", "moduforge-pipeline-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// ========== Stage 1: 生成代码 ==========
	genSkill := NewGenerateCodeSkill(s.apiKey, s.endpoint, s.model)
	genResult, err := s.retryLLM(ctx, func() (string, error) {
		return genSkill.Execute(ctx, map[string]interface{}{
			"description": description,
			"files_spec":  filesSpec,
		})
	})
	if err != nil {
		return "", fmt.Errorf("stage 1 generate: %w", err)
	}

	files, err := parseFiles(genResult)
	if err != nil {
		return genResult, nil
	}

	// ========== Stage 2: 写入临时目录 + 构建验证 ==========
	s.writeFilesToDisk(tmpDir, files)
	buildResult := s.verifyBuild(tmpDir, files)

	// ========== Stage 3: 构建失败 → LLM 修复（最多 3 轮） ==========
	for round := 0; round < 3 && !buildResult.Success; round++ {
		files, err = s.fixBuildErrors(ctx, files, buildResult, description)
		if err != nil {
			break
		}
		s.writeFilesToDisk(tmpDir, files)
		buildResult = s.verifyBuild(tmpDir, files)
	}

	// ========== Stage 4: Lint 检查 ==========
	lintSkill := NewLintCodeSkill()
	lintResult, _ := lintSkill.Execute(ctx, map[string]interface{}{"files": files})
	lintReport := parseLintReport(lintResult)

	// ========== Stage 5: 修复 Lint 问题（最多 2 轮） ==========
	for round := 0; round < 2 && len(lintReport.Issues) > 0; round++ {
		files, err = s.fixIssues(ctx, files, lintReport.Issues, "lint")
		if err != nil {
			break
		}
		s.writeFilesToDisk(tmpDir, files)
		lintResult, _ = lintSkill.Execute(ctx, map[string]interface{}{"files": files})
		lintReport = parseLintReport(lintResult)
	}

	// ========== Stage 6: 测试模块 ==========
	testSkill := NewTestModuleSkill()
	testResult, _ := testSkill.Execute(ctx, map[string]interface{}{
		"files":     files,
		"test_type": "all",
	})
	testReport := parseTestReport(testResult)

	// ========== Stage 7: 修复测试失败（最多 2 轮） ==========
	for round := 0; round < 2 && testReport.Failed > 0; round++ {
		files, err = s.fixIssues(ctx, files, nil, "test")
		if err != nil {
			break
		}
		s.writeFilesToDisk(tmpDir, files)
		testResult, _ = testSkill.Execute(ctx, map[string]interface{}{
			"files":     files,
			"test_type": "all",
		})
		testReport = parseTestReport(testResult)
	}

	// ========== 构建最终输出 ==========
	return s.buildOutput(files, buildResult, lintReport, testReport), nil
}

// ═══════════════════════════════════════════════════════════════
// Build Verification — 真正的编译验证
// ═══════════════════════════════════════════════════════════════

type BuildResult struct {
	Success    bool     `json:"success"`
	Language   string   `json:"language"`
	Compiler   string   `json:"compiler"`
	Command    string   `json:"command"`
	Stdout     string   `json:"stdout,omitempty"`
	Stderr     string   `json:"stderr,omitempty"`
	OutputFile string   `json:"output_file,omitempty"`
	Errors     []string `json:"errors,omitempty"`
}

// verifyBuild 根据项目中的源文件类型，选择合适的编译器进行验证
func (s *CodePipelineSkill) verifyBuild(projectDir string, files map[string]interface{}) *BuildResult {
	// 检测项目语言
	lang := detectProjectLanguage(files)
	if lang == "" || lang == "shell" {
		// Shell 脚本不需要编译，做语法检查即可
		return s.verifyShellSyntax(projectDir, files)
	}

	switch lang {
	case "rust":
		return s.verifyRustBuild(projectDir, files)
	case "go":
		return s.verifyGoBuild(projectDir, files)
	case "cpp", "c":
		return s.verifyCppBuild(projectDir, files)
	default:
		return &BuildResult{Success: true, Language: lang, Compiler: "none", Command: "(no compilation needed)"}
	}
}

// detectProjectLanguage 检测项目主要语言
func detectProjectLanguage(files map[string]interface{}) string {
	counts := map[string]int{}
	for path := range files {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".rs":
			counts["rust"]++
		case ".go":
			counts["go"]++
		case ".cpp", ".cc", ".cxx", ".c++":
			counts["cpp"]++
		case ".c", ".h":
			counts["c"]++
		case ".sh":
			counts["shell"]++
		case ".py":
			counts["python"]++
		}
	}
	// 有 Cargo.toml → Rust
	for path := range files {
		if filepath.Base(path) == "Cargo.toml" {
			return "rust"
		}
	}
	// 有 go.mod → Go
	for path := range files {
		if filepath.Base(path) == "go.mod" {
			return "go"
		}
	}
	// 有 CMakeLists.txt 或 Android.mk → C/C++
	for path := range files {
		base := strings.ToLower(filepath.Base(path))
		if base == "cmakelists.txt" || base == "android.mk" {
			return "cpp"
		}
	}
	// 按文件数量判断
	best := ""
	bestCount := 0
	for lang, count := range counts {
		if count > bestCount {
			best = lang
			bestCount = count
		}
	}
	return best
}

// verifyShellSyntax Shell 脚本语法检查
func (s *CodePipelineSkill) verifyShellSyntax(projectDir string, files map[string]interface{}) *BuildResult {
	result := &BuildResult{Success: true, Language: "shell", Compiler: "bash -n"}
	var errors []string

	for path := range files {
		if !strings.HasSuffix(path, ".sh") {
			continue
		}
		fullPath := filepath.Join(projectDir, path)
		cmd := exec.Command("bash", "-n", fullPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", path, strings.TrimSpace(string(output))))
			result.Success = false
		}
	}

	result.Errors = errors
	if !result.Success {
		result.Stderr = strings.Join(errors, "\n")
	}
	return result
}

// verifyRustBuild Rust 编译验证
func (s *CodePipelineSkill) verifyRustBuild(projectDir string, files map[string]interface{}) *BuildResult {
	result := &BuildResult{Language: "rust"}

	// 检查 rustc 是否可用
	rustcPath := findExecutable("rustc")
	if rustcPath == "" {
		// 无 rustc，跳过编译但标记为成功（不阻塞）
		result.Success = true
		result.Compiler = "rustc (not available, skipped)"
		result.Command = "(rustc not found, syntax check only)"
		return result
	}

	// 尝试 cargo check（比 cargo build 快）
	cargoPath := findExecutable("cargo")
	if cargoPath != "" && fileExists(filepath.Join(projectDir, "Cargo.toml")) {
		cmd := exec.Command("cargo", "check", "--message-format=short")
		cmd.Dir = projectDir
		cmd.Env = append(os.Environ(), "CARGO_INCREMENTAL=1")
		output, err := cmd.CombinedOutput()
		result.Command = "cargo check --message-format=short"
		result.Compiler = "cargo"
		result.Stdout = string(output)
		if err != nil {
			result.Success = false
			result.Stderr = extractCompileErrors(string(output))
			result.Errors = parseCargoErrors(string(output))
		} else {
			result.Success = true
		}
		return result
	}

	// Fallback: 逐文件 rustc --edition 2021 --crate-type lib
	var errors []string
	for path := range files {
		if !strings.HasSuffix(path, ".rs") {
			continue
		}
		fullPath := filepath.Join(projectDir, path)
		cmd := exec.Command(rustcPath, "--edition", "2021", "--crate-type", "lib", "--emit=metadata", "-o", "/dev/null", fullPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s:\n%s", path, strings.TrimSpace(string(output))))
		}
	}

	result.Command = "rustc --edition 2021 --crate-type lib --emit=metadata"
	result.Compiler = "rustc"
	if len(errors) > 0 {
		result.Success = false
		result.Stderr = strings.Join(errors, "\n\n")
		result.Errors = errors
	} else {
		result.Success = true
	}
	return result
}

// verifyGoBuild Go 编译验证
func (s *CodePipelineSkill) verifyGoBuild(projectDir string, files map[string]interface{}) *BuildResult {
	result := &BuildResult{Language: "go"}

	goPath := findExecutable("go")
	if goPath == "" {
		result.Success = true
		result.Compiler = "go (not available, skipped)"
		return result
	}

	// 逐文件 go vet
	var errors []string
	for path := range files {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		fullPath := filepath.Join(projectDir, path)
		cmd := exec.Command("go", "vet", fullPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", path, strings.TrimSpace(string(output))))
		}
	}

	result.Command = "go vet"
	result.Compiler = "go"
	if len(errors) > 0 {
		result.Success = false
		result.Stderr = strings.Join(errors, "\n\n")
		result.Errors = errors
	} else {
		result.Success = true
	}
	return result
}

// verifyCppBuild C/C++ 编译验证
func (s *CodePipelineSkill) verifyCppBuild(projectDir string, files map[string]interface{}) *BuildResult {
	result := &BuildResult{Language: "cpp"}

	// 查找编译器（优先 NDK clang，fallback 到系统 gcc/g++）
	compiler := findCppCompilerForPipeline()
	if compiler == "" {
		result.Success = true
		result.Compiler = "g++ (not available, skipped)"
		return result
	}

	// 收集所有 C++ 源文件
	var srcFiles []string
	for path := range files {
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".cpp" || ext == ".cc" || ext == ".cxx" || ext == ".c" || ext == ".h" || ext == ".hpp" {
			srcFiles = append(srcFiles, filepath.Join(projectDir, path))
		}
	}

	if len(srcFiles) == 0 {
		result.Success = true
		return result
	}

	// 编译到 .o（不链接，只验证语法和类型）
	outputFile := filepath.Join(projectDir, ".build_verify.o")
	args := append([]string{"-std=c++17", "-fsyntax-only", "-Wall", "-Wextra"}, srcFiles...)
	cmd := exec.Command(compiler, args...)
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()

	result.Command = compiler + " " + strings.Join(args, " ")
	result.Compiler = compiler
	result.Stdout = string(output)

	if err != nil {
		result.Success = false
		result.Stderr = string(output)
		result.Errors = parseGccErrors(string(output))
	} else {
		result.Success = true
		os.Remove(outputFile)
	}
	return result
}

// ═══════════════════════════════════════════════════════════════
// Build Error Fix — LLM 修复编译错误
// ═══════════════════════════════════════════════════════════════

func (s *CodePipelineSkill) fixBuildErrors(ctx context.Context, files map[string]interface{}, buildResult *BuildResult, description string) (map[string]interface{}, error) {
	var fixPrompt strings.Builder
	fixPrompt.WriteString(fmt.Sprintf("The code failed to compile with %s. Fix ALL compilation errors.\n\n", buildResult.Compiler))
	fixPrompt.WriteString(fmt.Sprintf("Original requirement: %s\n\n", description))

	if buildResult.Stderr != "" {
		fixPrompt.WriteString("Compilation errors:\n```\n")
		fixPrompt.WriteString(buildResult.Stderr)
		fixPrompt.WriteString("\n```\n\n")
	}
	if buildResult.Stdout != "" && buildResult.Stdout != buildResult.Stderr {
		fixPrompt.WriteString("Compiler output:\n```\n")
		fixPrompt.WriteString(buildResult.Stdout)
		fixPrompt.WriteString("\n```\n\n")
	}

	fixPrompt.WriteString("Current files:\n")
	for path, content := range files {
		fixPrompt.WriteString(fmt.Sprintf("\n=== %s ===\n%s\n", path, content))
	}
	fixPrompt.WriteString("\nReturn ONLY a JSON object: {\"files\":[{\"path\":\"...\",\"content\":\"...\"}]}")

	systemPrompt := fmt.Sprintf(`You are an expert %s developer fixing compilation errors.
The code failed to compile. Analyze the error messages and fix ALL issues.
Return ONLY a JSON object with "files" array (each with "path" and "content").
Keep all files that don't need changes. Only modify files with compilation errors.

## KEY RULES:
- Fix the ROOT CAUSE of each error, not just symptoms
- If a function signature changed, update ALL callers
- If a type is wrong, fix the type — don't cast
- If an import is missing, add it
- If an undefined symbol appears, check if it's from a library that needs linking
- For Rust: check lifetime annotations, trait bounds, and move semantics
- For Go: check imports, unexported fields, and interface satisfaction
- For C++: check includes, namespaces, and template parameters`, buildResult.Language)

	result, err := s.retryLLM(ctx, func() (string, error) {
		return s.callLLM(ctx, systemPrompt, fixPrompt.String())
	})
	if err != nil {
		return files, err
	}

	fixedFiles, err := parseFiles(result)
	if err != nil {
		return files, err
	}

	// Merge: keep unchanged files, replace changed ones
	merged := make(map[string]interface{})
	for k, v := range files {
		merged[k] = v
	}
	for k, v := range fixedFiles {
		merged[k] = v
	}

	return merged, nil
}

// ═══════════════════════════════════════════════════════════════
// Lint Fix — LLM 修复 lint 问题
// ═══════════════════════════════════════════════════════════════

func (s *CodePipelineSkill) fixIssues(ctx context.Context, files map[string]interface{}, issues []lintIssue, fixType string) (map[string]interface{}, error) {
	var fixPrompt strings.Builder
	fixPrompt.WriteString("Fix the following issues in the code files.\n\n")

	if fixType == "lint" && len(issues) > 0 {
		fixPrompt.WriteString("Lint issues found:\n")
		for _, issue := range issues {
			fixPrompt.WriteString(fmt.Sprintf("- [%s] %s:%d: %s\n", issue.Severity, issue.File, issue.Line, issue.Message))
		}
		fixPrompt.WriteString("\nFix ALL issues above. Return the corrected files.\n\n")
	} else if fixType == "test" {
		fixPrompt.WriteString("Tests failed. Review the code and fix any bugs that could cause test failures.\n\n")
	}

	fixPrompt.WriteString("Current files:\n")
	for path, content := range files {
		fixPrompt.WriteString(fmt.Sprintf("\n=== %s ===\n%s\n", path, content))
	}
	fixPrompt.WriteString("\nReturn ONLY a JSON object: {\"files\":[{\"path\":\"...\",\"content\":\"...\"}]}")

	systemPrompt := `You are an expert code fixer for Android Magisk/KernelSU/APatch modules.
Fix the issues described in the user message.
Return ONLY a JSON object with "files" array (each with "path" and "content").
Keep all files that don't need changes. Only modify files with issues.
Ensure all code is production-ready with proper error handling.

## RUST FIX GUIDELINES
When fixing Rust code, check for:
1. AtomicU32/AtomicU64 misuse - plain u32/u64 cannot use .load()/.store()
2. Move-out-of-shared-reference - use "ref mut" in match patterns
3. Missing error handling - replace .unwrap() with ? operator
4. Unsafe blocks - ensure they're justified and properly documented

## C/C++ FIX GUIDELINES
When fixing C/C++ code, check for:
1. Raw new/delete - replace with smart pointers
2. C-style arrays - replace with std::array or std::vector
3. sprintf() - replace with snprintf()
4. Uninitialized variables - add proper initialization
5. Memory leaks - use RAII patterns`

	result, err := s.retryLLM(ctx, func() (string, error) {
		return s.callLLM(ctx, systemPrompt, fixPrompt.String())
	})
	if err != nil {
		return files, err
	}

	fixedFiles, err := parseFiles(result)
	if err != nil {
		return files, err
	}

	return fixedFiles, nil
}

// ═══════════════════════════════════════════════════════════════
// Output Builder — 带构建报告的最终输出
// ═══════════════════════════════════════════════════════════════

func (s *CodePipelineSkill) buildOutput(files map[string]interface{}, buildResult *BuildResult, lr lintReport, tr testReport) string {
	type PipelineReport struct {
		Files  map[string]interface{} `json:"files"`
		Build  *BuildResult           `json:"build,omitempty"`
		Lint   *lintReport            `json:"lint,omitempty"`
		Test   *testReport            `json:"test,omitempty"`
		Summary string                `json:"summary"`
	}

	report := PipelineReport{
		Files: files,
		Build: buildResult,
		Lint:  &lr,
		Test:  &tr,
	}

	// Build summary
	var summary []string
	if buildResult.Success {
		summary = append(summary, fmt.Sprintf("✅ 编译通过 (%s)", buildResult.Language))
	} else {
		summary = append(summary, fmt.Sprintf("❌ 编译失败 (%s): %d 个错误", buildResult.Language, len(buildResult.Errors)))
	}
	if lr.Safe {
		summary = append(summary, fmt.Sprintf("✅ Lint 通过 (score: %d)", lr.Score))
	} else {
		summary = append(summary, fmt.Sprintf("⚠️ Lint: %d 个问题 (score: %d)", len(lr.Issues), lr.Score))
	}
	if tr.Failed == 0 {
		summary = append(summary, fmt.Sprintf("✅ 测试通过 (%d/%d)", tr.Passed, tr.Total))
	} else {
		summary = append(summary, fmt.Sprintf("❌ 测试失败: %d/%d", tr.Failed, tr.Total))
	}
	report.Summary = strings.Join(summary, " | ")

	out, _ := json.MarshalIndent(report, "", "  ")
	return string(out)
}

// ═══════════════════════════════════════════════════════════════
// Helpers
// ═══════════════════════════════════════════════════════════════

// writeFilesToDisk 将内存中的文件写入磁盘
func (s *CodePipelineSkill) writeFilesToDisk(dir string, files map[string]interface{}) {
	for path, content := range files {
		fullPath := filepath.Join(dir, filepath.Clean(path))
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		if contentStr, ok := content.(string); ok {
			os.WriteFile(fullPath, []byte(contentStr), 0644)
		}
	}
}

// findExecutable 查找可执行文件
func findExecutable(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	// 常见备选路径
	candidates := []string{
		"/usr/bin/" + name,
		"/usr/local/bin/" + name,
		"/opt/android-ndk/bin/" + name,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// findCppCompiler 查找 C++ 编译器
func findCppCompilerForPipeline() string {
	// 优先 NDK clang++
	ndkBase := os.Getenv("ANDROID_NDK")
	if ndkBase == "" {
		ndkBase = "/opt/android-ndk"
	}
	ndkClang := filepath.Join(ndkBase, "bin", "clang++")
	if _, err := os.Stat(ndkClang); err == nil {
		return ndkClang
	}
	// Fallback: 系统编译器
	for _, name := range []string{"g++", "clang++", "gcc", "cc"} {
		if p := findExecutable(name); p != "" {
			return p
		}
	}
	return ""
}

// fileExists 检查文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseCargoErrors 从 cargo 输出中提取错误信息
func parseCargoErrors(output string) []string {
	var errors []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "error[") || strings.Contains(line, "error:") {
			errors = append(errors, strings.TrimSpace(line))
		}
	}
	if len(errors) == 0 && strings.Contains(output, "error") {
		// Fallback: 取最后 20 行
		start := len(lines) - 20
		if start < 0 {
			start = 0
		}
		errors = append(errors, strings.Join(lines[start:], "\n"))
	}
	return errors
}

// parseGccErrors 从 gcc/g++ 输出中提取错误信息
func parseGccErrors(output string) []string {
	var errors []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "error:") || strings.Contains(line, "error:") {
			errors = append(errors, strings.TrimSpace(line))
		}
	}
	if len(errors) == 0 && output != "" {
		errors = append(errors, strings.TrimSpace(output))
	}
	return errors
}

// extractCompileErrors 从编译输出中提取错误摘要
func extractCompileErrors(output string) string {
	var errs []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "error") || strings.Contains(line, "warning") {
			errs = append(errs, line)
		}
	}
	if len(errs) == 0 {
		return output
	}
	return strings.Join(errs, "\n")
}

// retryLLM 带重试的 LLM 调用
func (s *CodePipelineSkill) retryLLM(ctx context.Context, fn func() (string, error)) (string, error) {
	var lastErr error
	var lastResult string
	for i := 0; i < 3; i++ {
		result, err := fn()
		if err == nil {
			if strings.Contains(result, `"files"`) || strings.Contains(result, `"path"`) {
				return result, nil
			}
			lastResult = result
			lastErr = fmt.Errorf("result missing expected 'files' structure")
		} else {
			lastErr = err
		}
		if i < 2 {
			backoff := time.Duration(i+1) * 2 * time.Second
			time.Sleep(backoff)
		}
	}
	if lastResult != "" {
		return lastResult + "\n⚠️ Warning: Output may be incomplete", nil
	}
	return "", fmt.Errorf("failed after 3 attempts: %w", lastErr)
}

// callLLM 调用 LLM API
func (s *CodePipelineSkill) callLLM(ctx context.Context, system, user string) (string, error) {
	endpoint := s.endpoint
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint = endpoint + "/chat/completions"
	}

	body := map[string]interface{}{
		"model": s.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"stream":     false,
		"max_tokens": 16384,
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("LLM error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse LLM response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	return result.Choices[0].Message.Content, nil
}

// ═══════════════════════════════════════════════════════════════
// Shared Helpers — used by code_pipeline and other skills
// ═══════════════════════════════════════════════════════════════

// parseFiles 解析 LLM 返回的 JSON 文件列表
func parseFiles(raw string) (map[string]interface{}, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 {
		return nil, fmt.Errorf("no JSON found in response")
	}
	jsonStr := raw[start : end+1]

	var result struct {
		Files []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("parse files JSON: %w", err)
	}

	files := make(map[string]interface{})
	for _, f := range result.Files {
		if f.Path != "" && strings.TrimSpace(f.Content) != "" {
			files[f.Path] = f.Content
		}
	}
	return files, nil
}

// parseLintReport 解析 lint 结果
func parseLintReport(raw string) lintReport {
	var report lintReport
	json.Unmarshal([]byte(raw), &report)
	return report
}

// parseTestReport 解析测试结果
func parseTestReport(raw string) testReport {
	var report testReport
	json.Unmarshal([]byte(raw), &report)
	return report
}

func (s *CodePipelineSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  false,
		Essential: false,
		NeedsDB:   false,
		NeedsLLM:  true,
	}
}
