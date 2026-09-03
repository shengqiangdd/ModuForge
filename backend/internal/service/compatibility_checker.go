package service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// CompatibilityChecker checks module compatibility across root solutions
type CompatibilityChecker struct{}

func NewCompatibilityChecker() *CompatibilityChecker {
	return &CompatibilityChecker{}
}

// CompatibilityResult represents the compatibility check result
type CompatibilityResult struct {
	ModuleID        string                    `json:"module_id"`
	ModuleName      string                    `json:"module_name"`
	ModuleType      string                    `json:"module_type"`
	Checks          []CompatibilityCheck      `json:"checks"`
	Score           int                       `json:"score"`
	Summary         string                    `json:"summary"`
	Recommendations []string                  `json:"recommendations"`
	Platforms       map[string]PlatformCompat `json:"platforms"`
}

type CompatibilityCheck struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Fix      string `json:"fix"`
}

type PlatformCompat struct {
	Compatible bool     `json:"compatible"`
	Score      int      `json:"score"`
	Issues     []string `json:"issues"`
}

// dangerousPatterns holds regex patterns for dangerous shell commands
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+(-[a-zA-Z]*r[a-zA-Z]*f[a-zA-Z]*\s+/|-\s*rf\s+/)`),
	regexp.MustCompile(`rm\s+(-[a-zA-Z]*f[a-zA-Z]*[a-zA-Z]*r[a-zA-Z]*\s+/|-\s*fr\s+/)`),
	regexp.MustCompile(`dd\s+.*of=/dev/`),
	regexp.MustCompile(`mkfs\.`),
	regexp.MustCompile(`>\s*/dev/null`), // allowed in general, but flagged when combined with rm
}

// dangerousCommands is a simpler string-based check for quick filtering
var dangerousCommands = []string{
	"rm -rf /",
	"rm -fr /",
	"mkfs.",
	"dd of=/dev/",
}

// safeUIDs holds acceptable UID/GID pairs for set_perm
var safeUIDs = []string{"0 0", "0 system", "system system"}

// CheckCompatibility runs all compatibility checks on a module
func (cc *CompatibilityChecker) CheckCompatibility(projectDir string, files map[string]string) (*CompatibilityResult, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("no files provided for compatibility check")
	}

	// Determine module type from file structure
	moduleType := detectModuleType(files)

	// Parse module.prop to get module info
	moduleID, moduleName := parseModuleProp(files)

	// Run all checks
	var allChecks []CompatibilityCheck
	allChecks = append(allChecks, cc.checkStructure(files)...)
	allChecks = append(allChecks, cc.checkScripts(files)...)
	allChecks = append(allChecks, cc.checkConfig(files)...)
	allChecks = append(allChecks, cc.checkSecurity(files)...)
	allChecks = append(allChecks, cc.checkAPICompat(files)...)

	score := cc.calculateScore(allChecks)
	recommendations := cc.generateRecommendations(allChecks, moduleType)
	platforms := cc.checkPlatformCompat(files, moduleType)

	summary := generateSummary(allChecks, score)

	return &CompatibilityResult{
		ModuleID:        moduleID,
		ModuleName:      moduleName,
		ModuleType:      moduleType,
		Checks:          allChecks,
		Score:           score,
		Summary:         summary,
		Recommendations: recommendations,
		Platforms:       platforms,
	}, nil
}

// detectModuleType identifies the root solution type from file structure
func detectModuleType(files map[string]string) string {
	hasMetaInf := false
	hasKsu := false
	hasApatch := false

	for path := range files {
		normalized := filepath.ToSlash(path)
		if strings.HasPrefix(normalized, "META-INF/com/google/android/update-binary") ||
			strings.HasPrefix(normalized, "META-INF/com/google/android/updater-script") {
			hasMetaInf = true
		}
		if normalized == "ksu.sh" || normalized == "webroot/ksu.sh" {
			hasKsu = true
		}
		if normalized == "action.sh" {
			hasApatch = true
		}
	}

	if hasMetaInf && !hasKsu && !hasApatch {
		return "magisk"
	}
	if hasKsu {
		return "ksu"
	}
	if hasApatch {
		return "apatch"
	}
	return "unknown"
}

// parseModuleProp extracts module ID and name from module.prop
func parseModuleProp(files map[string]string) (string, string) {
	content, ok := files["module.prop"]
	if !ok {
		return "", ""
	}

	var id, name string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "id=") {
			id = strings.TrimPrefix(line, "id=")
		} else if strings.HasPrefix(line, "name=") {
			name = strings.TrimPrefix(line, "name=")
		}
	}
	return id, name
}

// checkStructure validates the module file structure
func (cc *CompatibilityChecker) checkStructure(files map[string]string) []CompatibilityCheck {
	var checks []CompatibilityCheck

	// Check module.prop existence and required fields
	if propContent, ok := files["module.prop"]; ok {
		requiredFields := []string{"id", "name", "version", "versionCode", "author", "description"}
		props := parseProperties(propContent)

		for _, field := range requiredFields {
			if val, exists := props[field]; !exists || strings.TrimSpace(val) == "" {
				checks = append(checks, CompatibilityCheck{
					Name:     fmt.Sprintf("module.prop.%s", field),
					Category: "structure",
					Status:   "fail",
					Message:  fmt.Sprintf("module.prop is missing required field: %s", field),
					Fix:      fmt.Sprintf("Add '%s=<value>' to module.prop", field),
				})
			}
		}
	} else {
		checks = append(checks, CompatibilityCheck{
			Name:     "module.prop",
			Category: "structure",
			Status:   "fail",
			Message:  "module.prop file is missing",
			Fix:      "Create a module.prop file with required fields: id, name, version, versionCode, author, description",
		})
	}

	// Platform-specific entry point checks
	hasMagiskEntry := false
	hasKSUEntry := false
	hasApatchEntry := false

	for path := range files {
		normalized := filepath.ToSlash(path)
		if normalized == "META-INF/com/google/android/update-binary" {
			hasMagiskEntry = true
		}
		if normalized == "ksu.sh" || normalized == "webroot/ksu.sh" {
			hasKSUEntry = true
		}
		if normalized == "action.sh" {
			hasApatchEntry = true
		}
	}

	if !hasMagiskEntry && !hasKSUEntry && !hasApatchEntry {
		checks = append(checks, CompatibilityCheck{
			Name:     "entry_point",
			Category: "structure",
			Status:   "warn",
			Message:  "No recognized entry point found (update-binary, ksu.sh, or action.sh)",
			Fix:      "Add META-INF/com/google/android/update-binary for Magisk, ksu.sh for KSU, or action.sh for APatch",
		})
	}

	// Check system/ directory structure
	hasSystemDir := false
	for path := range files {
		if strings.HasPrefix(path, "system/") {
			hasSystemDir = true
			break
		}
	}
	if !hasSystemDir {
		checks = append(checks, CompatibilityCheck{
			Name:     "system_dir",
			Category: "structure",
			Status:   "warn",
			Message:  "No system/ directory found in module",
			Fix:      "Consider adding system/ directory for system file modifications",
		})
	}

	// Check for files outside allowed paths
	allowedPrefixes := []string{"system/", "META-INF/", "common/", "vendor/", "system.prop",
		"module.prop", "service.sh", "post-fs-data.sh", "customize.sh", "action.sh",
		"ksu.sh", "webroot/", "system/", "post-fs-data/", "service/", "late_start/",
		"vendor/", "odm/", "oem/", "product/", "system_ext/", "ap/"}

	for path := range files {
		normalized := filepath.ToSlash(path)
		allowed := false
		for _, prefix := range allowedPrefixes {
			if strings.HasPrefix(normalized, prefix) || normalized == strings.TrimSuffix(prefix, "/") {
				allowed = true
				break
			}
		}
		if !allowed {
			checks = append(checks, CompatibilityCheck{
				Name:     fmt.Sprintf("path_%s", sanitizeName(normalized)),
				Category: "structure",
				Status:   "warn",
				Message:  fmt.Sprintf("File path '%s' is outside standard module directories", normalized),
				Fix:      "Move files to standard directories: system/, META-INF/, common/, vendor/, etc.",
			})
		}
	}

	return checks
}

// checkScripts validates shell scripts in the module
func (cc *CompatibilityChecker) checkScripts(files map[string]string) []CompatibilityCheck {
	var checks []CompatibilityCheck

	shellExtensions := []string{".sh"}
	scriptFiles := make(map[string]string)

	for path, content := range files {
		ext := strings.ToLower(filepath.Ext(path))
		for _, allowedExt := range shellExtensions {
			if ext == allowedExt {
				scriptFiles[path] = content
				break
			}
		}
	}

	for path, content := range scriptFiles {
		lines := strings.Split(content, "\n")

		// Check shebang
		if len(lines) > 0 {
			firstLine := strings.TrimSpace(lines[0])
			if !strings.HasPrefix(firstLine, "#!/system/bin/sh") && !strings.HasPrefix(firstLine, "#!/bin/sh") {
				checks = append(checks, CompatibilityCheck{
					Name:     fmt.Sprintf("shebang_%s", sanitizeName(path)),
					Category: "script",
					Status:   "warn",
					Message:  fmt.Sprintf("Script '%s' is missing proper shebang line", path),
					Fix:      "Add '#!/system/bin/sh' or '#!/bin/sh' as the first line",
				})
			}
		}

		// Check for dangerous commands
		contentLower := strings.ToLower(content)
		for _, cmd := range dangerousCommands {
			if strings.Contains(contentLower, cmd) {
				checks = append(checks, CompatibilityCheck{
					Name:     fmt.Sprintf("dangerous_cmd_%s", sanitizeName(path)),
					Category: "script",
					Status:   "fail",
					Message:  fmt.Sprintf("Script '%s' contains dangerous command: %s", path, cmd),
					Fix:      "Remove or sandbox dangerous commands to prevent system damage",
				})
			}
		}

		// Check variable quoting
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			// Look for unquoted variable references in assignments or commands
			if strings.Contains(line, "$") && !strings.Contains(line, "\"") && !strings.Contains(line, "'") {
				// Simple heuristic: flag lines with $ that don't have any quotes
				// but skip lines that are just variable declarations
				if !strings.Contains(line, "=") || strings.Contains(line, "$(") {
					checks = append(checks, CompatibilityCheck{
						Name:     fmt.Sprintf("quoting_%s_line%d", sanitizeName(path), i+1),
						Category: "script",
						Status:   "warn",
						Message:  fmt.Sprintf("Script '%s' line %d may have unquoted variables", path, i+1),
						Fix:      "Quote variable references with double quotes: \"$variable\"",
					})
				}
			}
		}

		// Check set_perm calls
		permRegex := regexp.MustCompile(`set_perm\s+(\S+)\s+(\S+)\s+(\S+)`)
		matches := permRegex.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) >= 4 {
				uid := match[2]
				gid := match[3]
				found := false
				for _, safePair := range safeUIDs {
					parts := strings.SplitN(safePair, " ", 2)
					if len(parts) == 2 && uid == parts[0] && gid == parts[1] {
						found = true
						break
					}
				}
				if !found {
					checks = append(checks, CompatibilityCheck{
						Name:     fmt.Sprintf("perm_%s_%s", sanitizeName(path), match[1]),
						Category: "script",
						Status:   "warn",
						Message:  fmt.Sprintf("set_perm in '%s' uses non-standard UID/GID: %s %s", path, uid, gid),
						Fix:      "Use standard UIDs: 0 0 (root:root) or 0 system (root:system)",
					})
				}
			}
		}

		// Check service.sh for blocking operations
		if filepath.Base(path) == "service.sh" {
			blockingPatterns := []string{"while true", "sleep 999999", "read -p", "select "}
			for _, pattern := range blockingPatterns {
				if strings.Contains(contentLower, pattern) {
					checks = append(checks, CompatibilityCheck{
						Name:     "service_blocking",
						Category: "script",
						Status:   "fail",
						Message:  fmt.Sprintf("service.sh contains blocking pattern: %s", pattern),
						Fix:      "service.sh must not block boot. Use background processes or short timeouts.",
					})
				}
			}
		}

		// Check for hardcoded paths outside MODPATH
		hardcodedPaths := []string{"/system/", "/vendor/", "/data/", "/cache/"}
		for _, hPath := range hardcodedPaths {
			if strings.Contains(content, hPath) && !strings.Contains(content, "$MODPATH") && !strings.Contains(content, "${MODPATH}") {
				checks = append(checks, CompatibilityCheck{
					Name:     fmt.Sprintf("hardcoded_%s", sanitizeName(path)),
					Category: "script",
					Status:   "warn",
					Message:  fmt.Sprintf("Script '%s' may contain hardcoded path: %s", path, hPath),
					Fix:      "Use $MODPATH variable instead of hardcoded paths",
				})
			}
		}
	}

	return checks
}

// checkConfig validates configuration files in the module
func (cc *CompatibilityChecker) checkConfig(files map[string]string) []CompatibilityCheck {
	var checks []CompatibilityCheck

	// Validate module.prop values
	if propContent, ok := files["module.prop"]; ok {
		props := parseProperties(propContent)

		// Check ID format (no special characters)
		if id, exists := props["id"]; exists {
			idRegex := regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)
			if !idRegex.MatchString(id) {
				checks = append(checks, CompatibilityCheck{
					Name:     "prop_id_format",
					Category: "config",
					Status:   "fail",
					Message:  "module.prop id contains invalid characters",
					Fix:      "Use only alphanumeric characters, hyphens, and underscores for module ID",
				})
			}
		}

		// Check version format
		if version, exists := props["version"]; exists {
			if strings.TrimSpace(version) == "" {
				checks = append(checks, CompatibilityCheck{
					Name:     "prop_version_empty",
					Category: "config",
					Status:   "warn",
					Message:  "module.prop version is empty",
					Fix:      "Provide a meaningful version string",
				})
			}
		}

		// Check versionCode is numeric
		if verCode, exists := props["versionCode"]; exists {
			codeRegex := regexp.MustCompile(`^\d+$`)
			if !codeRegex.MatchString(strings.TrimSpace(verCode)) {
				checks = append(checks, CompatibilityCheck{
					Name:     "prop_versioncode",
					Category: "config",
					Status:   "fail",
					Message:  "module.prop versionCode must be a numeric value",
					Fix:      "Set versionCode to an integer value",
				})
			}
		}
	}

	// Validate system.prop
	if propContent, ok := files["system.prop"]; ok {
		lines := strings.Split(propContent, "\n")
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			// Check for valid property format (key=value)
			if !strings.Contains(line, "=") {
				checks = append(checks, CompatibilityCheck{
					Name:     fmt.Sprintf("sysprop_line%d", i+1),
					Category: "config",
					Status:   "warn",
					Message:  fmt.Sprintf("system.prop line %d has invalid format: %s", i+1, line),
					Fix:      "Use format: property.name=value",
				})
			}
		}
	}

	// Validate JSON files
	jsonExtensions := []string{".json", ".json5"}
	for path, content := range files {
		ext := strings.ToLower(filepath.Ext(path))
		for _, jsonExt := range jsonExtensions {
			if ext == jsonExt {
				var js interface{}
				if err := json.Unmarshal([]byte(content), &js); err != nil {
					checks = append(checks, CompatibilityCheck{
						Name:     fmt.Sprintf("json_%s", sanitizeName(path)),
						Category: "config",
						Status:   "fail",
						Message:  fmt.Sprintf("Invalid JSON in '%s': %s", path, err.Error()),
						Fix:      "Fix the JSON syntax error",
					})
				}
				break
			}
		}
	}

	// Check for duplicate file paths (this is more about the map, but we can check for case conflicts)
	pathMap := make(map[string]string)
	for path := range files {
		normalized := strings.ToLower(filepath.ToSlash(path))
		if existing, exists := pathMap[normalized]; exists && existing != path {
			checks = append(checks, CompatibilityCheck{
				Name:     fmt.Sprintf("dup_path_%s", sanitizeName(path)),
				Category: "config",
				Status:   "warn",
				Message:  fmt.Sprintf("Potential duplicate file paths: '%s' and '%s'", path, existing),
				Fix:      "Remove duplicate files or use distinct paths",
			})
		}
		pathMap[normalized] = path
	}

	return checks
}

// checkSecurity performs security checks on module files
func (cc *CompatibilityChecker) checkSecurity(files map[string]string) []CompatibilityCheck {
	var checks []CompatibilityCheck

	// Patterns for hardcoded credentials
	credentialPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)password\s*=\s*["'][^"']+["']`),
		regexp.MustCompile(`(?i)api[_-]?key\s*=\s*["'][^"']+["']`),
		regexp.MustCompile(`(?i)secret\s*=\s*["'][^"']+["']`),
		regexp.MustCompile(`(?i)token\s*=\s*["'][^"']+["']`),
		regexp.MustCompile(`(?i)aws[_-]?(access|secret)\s*=\s*["'][^"']+["']`),
	}

	// Check for hardcoded credentials
	for path, content := range files {
		for _, pattern := range credentialPatterns {
			if pattern.MatchString(content) {
				checks = append(checks, CompatibilityCheck{
					Name:     fmt.Sprintf("cred_%s", sanitizeName(path)),
					Category: "security",
					Status:   "fail",
					Message:  fmt.Sprintf("Possible hardcoded credential found in '%s'", path),
					Fix:      "Remove hardcoded credentials and use environment variables or secure storage",
				})
			}
		}
	}

	// Check for curl/wget without verification
	downloadPatterns := []*regexp.Regexp{
		regexp.MustCompile(`curl\s+.*\|\s*(sh|bash|busybox\s+sh|busybox\s+bash)`),
		regexp.MustCompile(`wget\s+.*\|\s*(sh|bash|busybox\s+sh|busybox\s+bash)`),
		regexp.MustCompile(`curl\s+.*-o\s+\S+\s+&&\s+(sh|bash|chmod\s+\+x)`),
		regexp.MustCompile(`wget\s+.*-O\s+\S+\s+&&\s+(sh|bash|chmod\s+\+x)`),
	}

	for path, content := range files {
		for _, pattern := range downloadPatterns {
			if pattern.MatchString(content) {
				checks = append(checks, CompatibilityCheck{
					Name:     fmt.Sprintf("dl_exec_%s", sanitizeName(path)),
					Category: "security",
					Status:   "fail",
					Message:  fmt.Sprintf("Script '%s' downloads and executes code without verification", path),
					Fix:      "Verify downloaded files with checksums or signatures before execution",
				})
			}
		}
	}

	// Check for eval usage
	evalPattern := regexp.MustCompile(`\beval\s+`)
	for path, content := range files {
		if evalPattern.MatchString(content) {
			checks = append(checks, CompatibilityCheck{
				Name:     fmt.Sprintf("eval_%s", sanitizeName(path)),
				Category: "security",
				Status:   "warn",
				Message:  fmt.Sprintf("Script '%s' uses 'eval' which may indicate dynamic code execution", path),
				Fix:      "Avoid using eval; use functions or case statements instead",
			})
		}
	}

	// Check for SELinux disabled
	selinuxPatterns := []string{
		"setenforce 0",
		"getenforce | grep -i permissive",
		"enforcing=0",
	}

	for path, content := range files {
		contentLower := strings.ToLower(content)
		for _, pattern := range selinuxPatterns {
			if strings.Contains(contentLower, strings.ToLower(pattern)) {
				checks = append(checks, CompatibilityCheck{
					Name:     fmt.Sprintf("selinux_%s", sanitizeName(path)),
					Category: "security",
					Status:   "fail",
					Message:  fmt.Sprintf("Script '%s' attempts to disable SELinux enforcement", path),
					Fix:      "Do not disable SELinux; work within the security policy",
				})
			}
		}
	}

	// Check for world-readable/writable permissions
	sensitiveFiles := []string{"*.prop", "*.key", "*.pem", "*.cert", "*.p12"}
	for path := range files {
		baseName := filepath.Base(path)
		for _, pattern := range sensitiveFiles {
			matched, _ := filepath.Match(pattern, baseName)
			if matched {
				// Check if there's a chmod making it world-readable
				if content, ok := files[path]; ok {
					if strings.Contains(content, "chmod 777") || strings.Contains(content, "chmod a+r") {
						checks = append(checks, CompatibilityCheck{
							Name:     fmt.Sprintf("perm_%s", sanitizeName(path)),
							Category: "security",
							Status:   "warn",
							Message:  fmt.Sprintf("Sensitive file '%s' may have overly permissive permissions", path),
							Fix:      "Use restrictive permissions: chmod 600 or chmod 644 for sensitive files",
						})
					}
				}
			}
		}
	}

	return checks
}

// checkAPICompat validates API compatibility across platforms
func (cc *CompatibilityChecker) checkAPICompat(files map[string]string) []CompatibilityCheck {
	var checks []CompatibilityCheck

	// Deprecated API patterns
	deprecatedAPIs := []string{
		"su -mm",
		"su -cn",
		"su -zb",
	}

	// Check for deprecated API usage
	for path, content := range files {
		for _, api := range deprecatedAPIs {
			if strings.Contains(content, api) {
				checks = append(checks, CompatibilityCheck{
					Name:     fmt.Sprintf("deprecated_%s", sanitizeName(path)),
					Category: "api",
					Status:   "warn",
					Message:  fmt.Sprintf("Script '%s' uses deprecated Magisk API: %s", path, api),
					Fix:      "Update to use current Magisk API functions",
				})
			}
		}
	}

	// Check for architecture-specific binaries in correct paths
	archPaths := map[string][]string{
		"arm64":  {"system/lib64/", "system/lib/arm64/"},
		"arm":    {"system/lib/", "system/lib/arm/"},
		"x86_64": {"system/lib64/", "system/lib/x86_64/"},
		"x86":    {"system/lib/", "system/lib/x86/"},
	}

	for path := range files {
		lowerPath := strings.ToLower(path)
		for arch, correctPaths := range archPaths {
			if strings.Contains(lowerPath, arch) {
				inCorrectPath := false
				for _, correctPath := range correctPaths {
					if strings.HasPrefix(filepath.ToSlash(path), correctPath) {
						inCorrectPath = true
						break
					}
				}
				if !inCorrectPath {
					checks = append(checks, CompatibilityCheck{
						Name:     fmt.Sprintf("arch_path_%s", sanitizeName(path)),
						Category: "api",
						Status:   "warn",
						Message:  fmt.Sprintf("Architecture-specific binary '%s' may be in incorrect path for %s", path, arch),
						Fix:      fmt.Sprintf("Move %s binaries to: %s", arch, strings.Join(correctPaths, " or ")),
					})
				}
			}
		}
	}

	// Check for Android API level compatibility (API 26+)
	// This is a heuristic check based on available system properties
	apiLevelIndicators := []string{
		"ro.build.version.sdk",
		"ro.build.version.release",
	}

	for path, content := range files {
		for _, indicator := range apiLevelIndicators {
			if strings.Contains(content, indicator) {
				// Check if there's an explicit API level check
				apiCheckPattern := regexp.MustCompile(`(?:if|elif|case)\s+.*\b(?:2[0-5]|1[0-9]|[0-9])\b`)
				if !apiCheckPattern.MatchString(content) {
					checks = append(checks, CompatibilityCheck{
						Name:     fmt.Sprintf("api_level_%s", sanitizeName(path)),
						Category: "api",
						Status:   "warn",
						Message:  fmt.Sprintf("Script '%s' references Android version without explicit API level check", path),
						Fix:      "Add explicit API level checks for Android version compatibility",
					})
				}
			}
		}
	}

	return checks
}

// calculateScore computes the compatibility score from checks
func (cc *CompatibilityChecker) calculateScore(checks []CompatibilityCheck) int {
	score := 50 // Base score

	for _, check := range checks {
		switch check.Status {
		case "pass":
			score += 10
		case "warn":
			score -= 5
		case "fail":
			score -= 20
		}
	}

	// Cap between 0 and 100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// generateRecommendations creates fix recommendations based on checks
func (cc *CompatibilityChecker) generateRecommendations(checks []CompatibilityCheck, moduleType string) []string {
	recommendations := make(map[string]bool)

	for _, check := range checks {
		if check.Status == "fail" && check.Fix != "" {
			recommendations[check.Fix] = true
		}
	}

	// Add platform-specific recommendations
	switch moduleType {
	case "magisk":
		recommendations["Ensure META-INF/com/google/android/update-binary is properly configured for Magisk"] = true
	case "ksu":
		recommendations["Ensure ksu.sh or webroot/ksu.sh exists for KernelSU support"] = true
	case "apatch":
		recommendations["Ensure action.sh exists for APatch support"] = true
	}

	result := make([]string, 0, len(recommendations))
	for rec := range recommendations {
		result = append(result, rec)
	}

	return result
}

// checkPlatformCompat generates per-platform compatibility information
func (cc *CompatibilityChecker) checkPlatformCompat(files map[string]string, moduleType string) map[string]PlatformCompat {
	platforms := make(map[string]PlatformCompat)

	// Magisk compatibility
	magiskScore := 100
	var magiskIssues []string

	if moduleType != "magisk" && moduleType != "unknown" {
		magiskScore -= 10
		magiskIssues = append(magiskIssues, "Module designed for different root solution")
	}

	hasMetaInf := false
	for path := range files {
		if strings.HasPrefix(filepath.ToSlash(path), "META-INF/com/google/android/update-binary") {
			hasMetaInf = true
			break
		}
	}
	if !hasMetaInf {
		magiskScore -= 20
		magiskIssues = append(magiskIssues, "Missing META-INF/com/google/android/update-binary")
	}

	platforms["magisk"] = PlatformCompat{
		Compatible: magiskScore >= 50,
		Score:      magiskScore,
		Issues:     magiskIssues,
	}

	// KernelSU compatibility
	ksuScore := 100
	var ksuIssues []string

	if moduleType != "ksu" && moduleType != "unknown" {
		ksuScore -= 10
		ksuIssues = append(ksuIssues, "Module designed for different root solution")
	}

	hasKSU := false
	for path := range files {
		normalized := filepath.ToSlash(path)
		if normalized == "ksu.sh" || normalized == "webroot/ksu.sh" {
			hasKSU = true
			break
		}
	}
	if !hasKSU {
		ksuScore -= 30
		ksuIssues = append(ksuIssues, "Missing ksu.sh or webroot/ksu.sh")
	}

	platforms["ksu"] = PlatformCompat{
		Compatible: ksuScore >= 50,
		Score:      ksuScore,
		Issues:     ksuIssues,
	}

	// APatch compatibility
	apatchScore := 100
	var apatchIssues []string

	if moduleType != "apatch" && moduleType != "unknown" {
		apatchScore -= 10
		apatchIssues = append(apatchIssues, "Module designed for different root solution")
	}

	hasApatch := false
	for path := range files {
		if filepath.ToSlash(path) == "action.sh" {
			hasApatch = true
			break
		}
	}
	if !hasApatch {
		apatchScore -= 30
		apatchIssues = append(apatchIssues, "Missing action.sh")
	}

	platforms["apatch"] = PlatformCompat{
		Compatible: apatchScore >= 50,
		Score:      apatchScore,
		Issues:     apatchIssues,
	}

	return platforms
}

// parseProperties parses a property file (key=value format)
func parseProperties(content string) map[string]string {
	props := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			props[key] = value
		}
	}
	return props
}

// sanitizeName creates a safe identifier from a file path
func sanitizeName(path string) string {
	replacer := strings.NewReplacer("/", "_", ".", "_", "-", "_", " ", "_")
	sanitized := replacer.Replace(path)
	// Remove any non-alphanumeric characters except underscore
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	sanitized = reg.ReplaceAllString(sanitized, "")
	return sanitized
}

// generateSummary creates a human-readable summary of the check results
func generateSummary(checks []CompatibilityCheck, score int) string {
	passCount := 0
	warnCount := 0
	failCount := 0

	for _, check := range checks {
		switch check.Status {
		case "pass":
			passCount++
		case "warn":
			warnCount++
		case "fail":
			failCount++
		}
	}

	var quality string
	switch {
	case score >= 80:
		quality = "excellent"
	case score >= 60:
		quality = "good"
	case score >= 40:
		quality = "moderate"
	default:
		quality = "poor"
	}

	return fmt.Sprintf("Module compatibility is %s (score: %d/100). Passed: %d, Warnings: %d, Failures: %d",
		quality, score, passCount, warnCount, failCount)
}
