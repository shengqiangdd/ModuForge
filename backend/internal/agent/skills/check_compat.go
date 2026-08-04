package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type CheckCompatSkill struct{}

func NewCheckCompatSkill() *CheckCompatSkill {
	return &CheckCompatSkill{}
}

func (s *CheckCompatSkill) Name() string {
	return "check_compat"
}

func (s *CheckCompatSkill) Description() string {
	return "Check Magisk/KernelSU/APatch compatibility. Input: {\"files\": {\"path\": \"content\", ...}}. Returns issues with fix suggestions."
}

type compatResult struct {
	Overall    string              `json:"overall"`
	Breakdown  map[string]string   `json:"breakdown"`
	Issues     []compatIssue       `json:"issues"`
	Matrix     map[string]bool     `json:"compatibility_matrix"`
}

type compatIssue struct {
	Severity  string `json:"severity"`
	Platform  string `json:"platform"`
	Rule      string `json:"rule"`
	Message   string `json:"message"`
	Fix       string `json:"fix,omitempty"`
}

var magiskOnlyPaths = []string{
	"/data/adb/magisk/",
	"/sbin/.magisk/",
	"/magisk/",
	"$MAGISKTMP",
}

var bashOnlyPatterns = []struct {
	pattern *regexp.Regexp
	message string
}{
	{pattern: regexp.MustCompile(`\[\[`), message: "[[ ]] is bash-only, use [ ] for POSIX"},
	{pattern: regexp.MustCompile(`<<<`), message: "<<< here-string is bash-only"},
	{pattern: regexp.MustCompile(`<\(`), message: "<() process substitution is bash-only"},
	{pattern: regexp.MustCompile(`\bdeclare\b`), message: "declare is bash-only"},
	{pattern: regexp.MustCompile(`\blocal\b`), message: "local is not POSIX, use subshell or unique var names"},
	{pattern: regexp.MustCompile(`\btypeset\b`), message: "typeset is bash-only"},
	{pattern: regexp.MustCompile(`\becho\s+-[neE]`), message: "echo with flags is not portable, use printf"},
}

func (s *CheckCompatSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	filesRaw, ok := input["files"]
	if !ok {
		return "", fmt.Errorf("files is required")
	}
	filesMap, ok := filesRaw.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("files must be an object")
	}

	var allIssues []compatIssue
	breakdown := make(map[string]string)
	matrix := map[string]bool{
		"magisk":   true,
		"kernelsu": true,
		"apatch":   true,
	}

	for filePath, contentRaw := range filesMap {
		content, _ := contentRaw.(string)
		if content == "" {
			continue
		}

		if filePath == "module.prop" {
			issues := s.checkModuleProp(content)
			allIssues = append(allIssues, issues...)
			for _, iss := range issues {
				if iss.Platform != "" {
					matrix[iss.Platform] = false
				}
			}
		}

		if strings.HasSuffix(filePath, ".sh") {
			issues := s.checkShellCompat(filePath, content)
			allIssues = append(allIssues, issues...)
		}

		if strings.HasSuffix(filePath, ".rs") || strings.HasSuffix(filePath, ".go") {
			issues := s.checkBinaryCompat(filePath, content)
			allIssues = append(allIssues, issues...)
		}
	}

	overall := "compatible"
	for _, ok := range matrix {
		if !ok {
			overall = "partial"
			break
		}
	}
	if len(allIssues) > 5 {
		overall = "incompatible"
	}

	breakdown["magisk"] = s.platformStatus(matrix["magisk"], allIssues, "magisk")
	breakdown["kernelsu"] = s.platformStatus(matrix["kernelsu"], allIssues, "kernelsu")
	breakdown["apatch"] = s.platformStatus(matrix["apatch"], allIssues, "apatch")

	result := compatResult{
		Overall:   overall,
		Breakdown: breakdown,
		Issues:    allIssues,
		Matrix:    matrix,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *CheckCompatSkill) platformStatus(ok bool, issues []compatIssue, platform string) string {
	if !ok {
		return fmt.Sprintf("Issues found (%d)", countForPlatform(issues, platform))
	}
	info := countForPlatform(issues, platform)
	if info > 0 {
		return fmt.Sprintf("Compatible with %d info items", info)
	}
	return "Fully compatible"
}

func countForPlatform(issues []compatIssue, platform string) int {
	count := 0
	for _, iss := range issues {
		if iss.Platform == platform || iss.Platform == "all" {
			count++
		}
	}
	return count
}

func (s *CheckCompatSkill) checkModuleProp(content string) []compatIssue {
	var issues []compatIssue
	lines := strings.Split(content, "\n")
	fields := make(map[string]string)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 {
			fields[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	requiredFields := []string{"id", "name", "version", "author", "description"}
	for _, rf := range requiredFields {
		if _, exists := fields[rf]; !exists {
			issues = append(issues, compatIssue{
				Severity: "critical", Platform: "all",
				Rule:    "missing_field",
				Message: fmt.Sprintf("module.prop missing required field: '%s'", rf),
				Fix:     fmt.Sprintf("Add '%s=<value>' to module.prop", rf),
			})
		}
	}

	// KernelSU 支持标志（正确字段名：ksu.supported）
	if _, exists := fields["ksu.supported"]; !exists {
		if _, exists := fields["ksu"]; !exists {
			issues = append(issues, compatIssue{
				Severity: "warning", Platform: "kernelsu",
				Rule:    "missing_ksu_flag",
				Message: "module.prop missing 'ksu.supported=true' for KernelSU compatibility",
				Fix:     "Add 'ksu.supported=true' to module.prop",
			})
		}
	}

	// APatch 支持标志（正确字段名：apatch.supported）
	if _, exists := fields["apatch.supported"]; !exists {
		if _, exists := fields["apatch"]; !exists {
			issues = append(issues, compatIssue{
				Severity: "warning", Platform: "apatch",
				Rule:    "missing_apatch_flag",
				Message: "module.prop missing 'apatch.supported=true' for APatch compatibility",
				Fix:     "Add 'apatch.supported=true' to module.prop",
			})
		}
	}

	if id, exists := fields["id"]; exists {
		if len(id) > 64 {
			issues = append(issues, compatIssue{
				Severity: "warning", Platform: "all",
				Rule:    "id_too_long",
				Message: fmt.Sprintf("module id '%s' is too long (>64 chars), may cause issues", id),
				Fix:     "Shorten the module id to 64 characters or less",
			})
		}
		if strings.Contains(id, " ") {
			issues = append(issues, compatIssue{
				Severity: "critical", Platform: "all",
				Rule:    "id_has_spaces",
				Message: fmt.Sprintf("module id '%s' contains spaces - use underscores or dots", id),
				Fix:     "Replace spaces with underscores or dots in module id",
			})
		}
	}

	return issues
}

func (s *CheckCompatSkill) checkShellCompat(filePath, content string) []compatIssue {
	var issues []compatIssue
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "#!") {
			if strings.Contains(trimmed, "bash") {
				issues = append(issues, compatIssue{
					Severity: "warning", Platform: "all",
					Rule:    "bash_shebang",
					Message: fmt.Sprintf("%s uses bash shebang - use #!/system/bin/sh for POSIX compatibility", filePath),
					Fix:     "Change shebang to #!/system/bin/sh",
				})
			}
			continue
		}

		for _, bp := range bashOnlyPatterns {
			if bp.pattern.MatchString(trimmed) {
				issues = append(issues, compatIssue{
					Severity: "warning", Platform: "all",
					Rule:    "bash_only_feature",
					Message: fmt.Sprintf("%s line %d: %s", filePath, i+1, bp.message),
					Fix:     "Replace bash-specific syntax with POSIX-compatible alternative",
				})
			}
		}

		for _, magiskPath := range magiskOnlyPaths {
			if strings.Contains(trimmed, magiskPath) && !strings.Contains(trimmed, "#") {
				issues = append(issues, compatIssue{
					Severity: "warning", Platform: "kernelsu",
					Rule:    "magisk_specific_path",
					Message: fmt.Sprintf("%s line %d: Uses Magisk-specific path '%s' - not available on KernelSU/APatch", filePath, i+1, magiskPath),
					Fix:     "Use Magisk module API or detect environment at runtime to support multiple root solutions",
				})
			}
		}
	}

	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		if !strings.HasPrefix(firstLine, "#!") {
			issues = append(issues, compatIssue{
				Severity: "warning", Platform: "all",
				Rule:    "missing_shebang",
				Message: fmt.Sprintf("%s missing shebang line", filePath),
				Fix:     "Add #!/system/bin/sh as the first line",
			})
		}
	}

	return issues
}

func (s *CheckCompatSkill) checkBinaryCompat(filePath, content string) []compatIssue {
	var issues []compatIssue

	if strings.HasSuffix(filePath, ".rs") {
		nonStdLibImports := []string{"std::fs", "std::net", "std::process", "std::thread"}
		for _, imp := range nonStdLibImports {
			if strings.Contains(content, fmt.Sprintf("use %s", imp)) {
				issues = append(issues, compatIssue{
					Severity: "info", Platform: "all",
					Rule:    "binary_feature",
					Message: fmt.Sprintf("%s uses %s - ensure binary is compiled for ARM64 architecture", filePath, imp),
					Fix:     "Build with: cargo build --release --target aarch64-linux-android",
				})
				break
			}
		}
	}

	return issues
}

func (s *CheckCompatSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  true,
		Essential: false,
		NeedsDB:   false,
		NeedsLLM:  true,
	}
}
