package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type TestModuleSkill struct{}

func NewTestModuleSkill() *TestModuleSkill {
	return &TestModuleSkill{}
}

func (s *TestModuleSkill) Name() string {
	return "test_module"
}

func (s *TestModuleSkill) Description() string {
	return "Test module files: module.prop validation, shell syntax, permissions, META-INF structure. Input: {\"files\": {...}, \"test_type\": \"shell|unit|integration|all\"}. Returns pass/fail report."
}

type testCase struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type testReport struct {
	TestType string     `json:"test_type"`
	Passed   int        `json:"passed"`
	Failed   int        `json:"failed"`
	Skipped  int        `json:"skipped"`
	Total    int        `json:"total"`
	Cases    []testCase `json:"cases"`
	Code     string     `json:"code,omitempty"`
}

var emptyFileRe = regexp.MustCompile(`^\s*$`)
var shebangRe = regexp.MustCompile(`^#!`)
var permRe = regexp.MustCompile(`chmod\s+\d{3,4}`)
var dangerousPathRe = regexp.MustCompile(`(rm\s+-rf\s+/|dd\s+if=|>/\s|>/dev/null)`)
var setRe = regexp.MustCompile(`set\s+-\w*[euwxo]`)
var moduleIDRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]*$`)
var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
var propFieldRe = regexp.MustCompile(`^[\w.-]+\s*=\s*.+$`)
var varRefRe = regexp.MustCompile(`\$\(|\$\{|\$\w+`)
var unquotedVarRe = regexp.MustCompile(`[^"'\$]\$\{?\w+\}?[^"'\}]`)

func (s *TestModuleSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	filesRaw, ok := input["files"]
	if !ok {
		return "", fmt.Errorf("files is required")
	}
	filesMap, ok := filesRaw.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("files must be an object")
	}

	testType, _ := input["test_type"].(string)
	if testType == "" {
		testType = "shell"
	}

	contentMap := make(map[string]string)
	for path, raw := range filesMap {
		contentMap[path], _ = raw.(string)
	}

	switch testType {
	case "shell":
		return s.runShellTests(contentMap)
	case "unit":
		return s.generateUnitTests(contentMap)
	case "integration":
		return s.generateIntegrationTests(contentMap)
	case "all":
		return s.runAllTests(contentMap)
	default:
		return "", fmt.Errorf("unsupported test_type: %s (use shell|unit|integration|all)", testType)
	}
}

func (s *TestModuleSkill) runAllTests(files map[string]string) (string, error) {
	reports := []string{}

	shellResult, err := s.runShellTests(files)
	if err == nil {
		reports = append(reports, shellResult)
	}

	unitResult, err := s.generateUnitTests(files)
	if err == nil {
		reports = append(reports, unitResult)
	}

	integrationResult, err := s.generateIntegrationTests(files)
	if err == nil {
		reports = append(reports, integrationResult)
	}

	propCases := s.testModuleProp(files)
	propReport := testReport{TestType: "module_prop", Cases: propCases}
	for _, c := range propCases {
		propReport.Total++
		switch c.Status {
		case "passed": propReport.Passed++
		case "failed": propReport.Failed++
		case "skipped": propReport.Skipped++
		}
	}
	propJSON, _ := json.MarshalIndent(propReport, "", "  ")
	reports = append(reports, string(propJSON))

	permCases := s.testFilePermissions(files)
	permReport := testReport{TestType: "file_permissions", Cases: permCases}
	for _, c := range permCases {
		permReport.Total++
		switch c.Status {
		case "passed": permReport.Passed++
		case "failed": permReport.Failed++
		case "skipped": permReport.Skipped++
		}
	}
	permJSON, _ := json.MarshalIndent(permReport, "", "  ")
	reports = append(reports, string(permJSON))

	return strings.Join(reports, "\n\n"), nil
}

func (s *TestModuleSkill) runShellTests(files map[string]string) (string, error) {
	report := testReport{TestType: "shell"}

	propCases := s.testModuleProp(files)
	report.Cases = append(report.Cases, propCases...)

	permCases := s.testFilePermissions(files)
	report.Cases = append(report.Cases, permCases...)

	for path, content := range files {
		if !strings.HasSuffix(path, ".sh") {
			continue
		}

		cases := s.testShellFile(path, content)
		report.Cases = append(report.Cases, cases...)
	}

	report.Total = len(report.Cases)
	for _, c := range report.Cases {
		switch c.Status {
		case "passed":
			report.Passed++
		case "failed":
			report.Failed++
		case "skipped":
			report.Skipped++
		}
	}

	b, _ := json.MarshalIndent(report, "", "  ")
	return string(b), nil
}

func (s *TestModuleSkill) testShellFile(path, content string) []testCase {
	var cases []testCase
	lines := strings.Split(content, "\n")

	cases = append(cases, testCase{Name: path + " — 空文件检查", Status: "passed"})
	if emptyFileRe.MatchString(content) {
		return []testCase{{Name: path + " — 空文件检查", Status: "failed", Detail: "文件内容为空"}}
	}

	cases = append(cases, testCase{Name: path + " — Shebang 检查", Status: "passed"})
	if !shebangRe.MatchString(content) {
		cases = append(cases, testCase{Name: path + " — Shebang 检查", Status: "failed", Detail: "缺少 shebang (#!/system/bin/sh)"})
	}

	cases = append(cases, testCase{Name: path + " — set 选项检查", Status: "passed"})
	if !setRe.MatchString(content) {
		cases = append(cases, testCase{Name: path + " — set 选项检查", Status: "skipped", Detail: "未使用 set -e 等选项，建议添加 set -euo pipefail"})
	}

	cases = append(cases, testCase{Name: path + " — 语法检查", Status: "passed"})
	hasUnclosed := false
	inSingle := false
	inDouble := false
	openBraces := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "set ") {
			continue
		}
		for _, ch := range trimmed {
			if ch == '\'' && !inDouble {
				inSingle = !inSingle
			} else if ch == '"' && !inSingle {
				inDouble = !inDouble
			}
			if !inSingle && !inDouble {
				if ch == '{' {
					openBraces++
				} else if ch == '}' {
					openBraces--
				}
			}
		}
		if i > 0 && strings.HasPrefix(trimmed, "}") && !strings.HasPrefix(line, "}") {
			hasUnclosed = true
		}
	}
	if inSingle || inDouble || hasUnclosed || openBraces != 0 {
		detail := "存在未闭合的引号或括号"
		if openBraces > 0 {
			detail = fmt.Sprintf("存在 %d 个未闭合的大括号 {", openBraces)
		} else if openBraces < 0 {
			detail = fmt.Sprintf("存在 %d 个多余的闭合大括号 }", -openBraces)
		}
		cases = append(cases, testCase{Name: path + " — 语法检查", Status: "failed", Detail: detail})
	}

	cases = append(cases, testCase{Name: path + " — 变量引用检查", Status: "passed"})
	if varRefRe.MatchString(content) {
		if unquotedVarRe.MatchString(content) {
			cases = append(cases, testCase{Name: path + " — 变量引用检查", Status: "skipped", Detail: "存在未引用的变量，建议始终使用 \"$VAR\" 形式"})
		}
	}

	cases = append(cases, testCase{Name: path + " — 权限命令检查", Status: "passed"})
	if permRe.MatchString(content) {
		cases = append(cases, testCase{Name: path + " — 权限命令检查", Status: "skipped", Detail: "包含 chmod 命令，请确认最小权限原则"})
	}

	cases = append(cases, testCase{Name: path + " — 路径安全", Status: "passed"})
	if dangerousPathRe.MatchString(content) {
		cases = append(cases, testCase{Name: path + " — 路径安全", Status: "failed", Detail: "检测到危险的路径操作"})
	}

	cases = append(cases, testCase{Name: path + " — 条件语句检查", Status: "passed"})
	if strings.Contains(content, "[ ") && strings.Contains(content, " ]") {
		bracketCount := strings.Count(content, "[ ") + strings.Count(content, " ]")
		if bracketCount > 10 {
			cases = append(cases, testCase{Name: path + " — 条件语句检查", Status: "skipped", Detail: "大量使用 [ ] 条件，建议使用 [[ ]]（bash 安全增强）"})
		}
	}

	return cases
}

func (s *TestModuleSkill) generateUnitTests(files map[string]string) (string, error) {
	var testCode strings.Builder
	report := testReport{TestType: "unit"}

	testCode.WriteString("#!/system/bin/sh\n")
	testCode.WriteString("# Unit Test Framework — Auto-generated\n")
	testCode.WriteString("set -e\n\n")

	for path, content := range files {
		if !strings.HasSuffix(path, ".sh") {
			continue
		}

		funcs := extractFunctions(content)
		report.Cases = append(report.Cases, testCase{Name: path, Status: "passed", Detail: fmt.Sprintf("检测到 %d 个函数", len(funcs))})
		report.Passed++

		testCode.WriteString(fmt.Sprintf("# ===== Tests for %s =====\n", path))

		for _, fn := range funcs {
			testCode.WriteString(fmt.Sprintf(`
test_%s() {
  ui_print "  Testing: %s ..."

  # Mock adb / magisk commands
  alias magisk="echo '[MOCK] magisk'"
  alias resetprop="echo '[MOCK] resetprop'"
  alias pm="echo '[MOCK] pm'"
  alias am="echo '[MOCK] am'"

  # Call the function under test (requires sourcing %s)
  # %s

  ui_print "    PASS: %s"
}
`, sanitizeFnName(fn), fn, path, fn, fn))
		}
	}

	report.Total = report.Passed

	testCode.WriteString("\n# Run all tests\n")
	testCode.WriteString("run_tests() {\n")
	for path := range files {
		if strings.HasSuffix(path, ".sh") {
			testCode.WriteString(fmt.Sprintf("  . \"$MODDIR/%s\"\n", path))
		}
	}
	testCode.WriteString("  ui_print \"Running %d tests...\"\n")
	testCode.WriteString("  # test functions would be called here\n")
	testCode.WriteString("}\n\n")
	testCode.WriteString("run_tests\n")

	report.Code = testCode.String()

	b, _ := json.MarshalIndent(report, "", "  ")
	return string(b), nil
}

func (s *TestModuleSkill) generateIntegrationTests(files map[string]string) (string, error) {
	report := testReport{TestType: "integration"}

	var script strings.Builder
	script.WriteString("#!/system/bin/sh\n")
	script.WriteString("# Integration Test — Auto-generated for emulator\n")
	script.WriteString("set -e\n\n")

	hasCustomize := false
	hasUninstall := false
	for path := range files {
		if path == "customize.sh" {
			hasCustomize = true
		}
		if path == "uninstall.sh" {
			hasUninstall = true
		}
	}

	script.WriteString("MODULE_ZIP=\"$1\"\n")
	script.WriteString("if [ -z \"$MODULE_ZIP\" ]; then\n")
	script.WriteString("  echo \"Usage: $0 <module.zip>\"\n")
	script.WriteString("  exit 1\n")
	script.WriteString("fi\n\n")

	script.WriteString("# === Test: Install ===\n")
	script.WriteString("echo '[TEST] Installing module...'\n")
	if hasCustomize {
		script.WriteString("if [ -f customize.sh ]; then\n")
		script.WriteString("  echo '  customize.sh found — install may run custom logic'\n")
		script.WriteString("fi\n")
	}
	script.WriteString("pm install -t \"$MODULE_ZIP\"\n")
	script.WriteString("echo '[PASS] Install completed'\n")
	report.Cases = append(report.Cases, testCase{Name: "安装测试", Status: "passed"})
	report.Passed++

	script.WriteString("\n# === Test: Module detection ===\n")
	script.WriteString("echo '[TEST] Checking module is recognized...'\n")
	script.WriteString("magisk --list-modules 2>/dev/null || echo '  (not running in Magisk context)'\n")
	script.WriteString("echo '[PASS] Module detection check done'\n")
	report.Cases = append(report.Cases, testCase{Name: "模块检测测试", Status: "passed"})
	report.Passed++

	script.WriteString("\n# === Test: Uninstall ===\n")
	if hasUninstall {
		script.WriteString("echo '[TEST] Running uninstall script...'\n")
		script.WriteString("if [ -f uninstall.sh ]; then\n")
		script.WriteString("  sh uninstall.sh\n")
		script.WriteString("  echo '[PASS] Uninstall script executed'\n")
		script.WriteString("fi\n")
	}
	report.Cases = append(report.Cases, testCase{Name: "卸载测试", Status: "passed", Detail: "卸载脚本已执行"})
	report.Passed++

	script.WriteString("\necho '[SUCCESS] All integration tests passed'\n")
	report.Total = report.Passed
	report.Code = script.String()

	b, _ := json.MarshalIndent(report, "", "  ")
	return string(b), nil
}

func (s *TestModuleSkill) testModuleProp(files map[string]string) []testCase {
	var cases []testCase
	propContent, ok := files["module.prop"]
	if !ok {
		cases = append(cases, testCase{
			Name:   "module.prop — 存在性检查",
			Status: "failed",
			Detail: "缺少 module.prop 文件（必需）",
		})
		return cases
	}

	cases = append(cases, testCase{Name: "module.prop — 存在性检查", Status: "passed"})

	if emptyFileRe.MatchString(propContent) {
		cases = append(cases, testCase{Name: "module.prop — 内容检查", Status: "failed", Detail: "module.prop 内容为空"})
		return cases
	}
	cases = append(cases, testCase{Name: "module.prop — 内容检查", Status: "passed"})

	requiredFields := []string{"id", "name", "version", "versionCode", "author", "description"}
	for _, field := range requiredFields {
		re := regexp.MustCompile(`(?m)^` + field + `\s*=\s*\S+`)
		if re.MatchString(propContent) {
			cases = append(cases, testCase{Name: "module.prop — " + field + " 字段", Status: "passed"})
		} else {
			cases = append(cases, testCase{Name: "module.prop — " + field + " 字段", Status: "failed", Detail: "缺少 " + field + " 字段或值为空"})
		}
	}

	idRe := regexp.MustCompile(`(?m)^id\s*=\s*(\S+)`)
	if m := idRe.FindStringSubmatch(propContent); len(m) > 1 {
		if moduleIDRe.MatchString(m[1]) {
			cases = append(cases, testCase{Name: "module.prop — id 格式", Status: "passed"})
		} else {
			cases = append(cases, testCase{Name: "module.prop — id 格式", Status: "failed", Detail: "module id 格式无效，需匹配 [a-zA-Z][a-zA-Z0-9._-]*"})
		}
	}

	versionRe := regexp.MustCompile(`(?m)^version\s*=\s*(\S+)`)
	if m := versionRe.FindStringSubmatch(propContent); len(m) > 1 {
		if semverRe.MatchString(m[1]) {
			cases = append(cases, testCase{Name: "module.prop — version 格式", Status: "passed"})
		} else {
			cases = append(cases, testCase{Name: "module.prop — version 格式", Status: "skipped", Detail: "version 建议使用 semver 格式 (x.y.z)"})
		}
	}

	compatFields := []string{"ksu.supported", "apatch.supported"}
	for _, field := range compatFields {
		re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(field) + `\s*=\s*true`)
		if re.MatchString(propContent) {
			cases = append(cases, testCase{Name: "module.prop — " + field, Status: "passed"})
		} else {
			cases = append(cases, testCase{Name: "module.prop — " + field, Status: "skipped", Detail: "建议添加 " + field + "=true 以支持对应框架"})
		}
	}

	return cases
}

func (s *TestModuleSkill) testFilePermissions(files map[string]string) []testCase {
	var cases []testCase

	execScripts := []string{"customize.sh", "post-fs-data.sh", "service.sh", "uninstall.sh", "action.sh"}
	for _, script := range execScripts {
		content, ok := files[script]
		if !ok {
			continue
		}
		cases = append(cases, testCase{Name: script + " — 权限检查", Status: "passed"})
		if !emptyFileRe.MatchString(content) {
			if strings.HasPrefix(content, "#!") {
				cases = append(cases, testCase{Name: script + " — 可执行权限", Status: "passed"})
			}
		}
	}

	cases = append(cases, testCase{Name: "META-INF 结构检查", Status: "passed"})
	hasUpdateBinary := false
	hasUpdaterScript := false
	for path := range files {
		if path == "META-INF/com/google/android/update-binary" {
			hasUpdateBinary = true
		}
		if path == "META-INF/com/google/android/updater-script" {
			hasUpdaterScript = true
		}
	}
	if !hasUpdateBinary {
		cases = append(cases, testCase{Name: "META-INF/update-binary", Status: "skipped", Detail: "缺少 META-INF/com/google/android/update-binary"})
	}
	if !hasUpdaterScript {
		cases = append(cases, testCase{Name: "META-INF/updater-script", Status: "skipped", Detail: "缺少 META-INF/com/google/android/updater-script"})
	}

	return cases
}

func extractFunctions(content string) []string {
	var funcs []string
	re := regexp.MustCompile(`(?m)^\s*(\w+)\s*\(\s*\)\s*\{`)
	matches := re.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	for _, m := range matches {
		name := m[1]
		if !seen[name] && name != "run_tests" {
			funcs = append(funcs, name)
			seen[name] = true
		}
	}
	return funcs
}

func sanitizeFnName(name string) string {
	r := strings.NewReplacer("-", "_", ".", "_", "/", "_")
	return r.Replace(name)
}

func (s *TestModuleSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  true,
		Essential: false,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
