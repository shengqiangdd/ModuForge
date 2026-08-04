package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type GenDocsSkill struct{}

func NewGenDocsSkill() *GenDocsSkill {
	return &GenDocsSkill{}
}

func (s *GenDocsSkill) Name() string {
	return "gen_docs"
}

func (s *GenDocsSkill) Description() string {
	return "Generate README and validate module.prop. Input: {\"files\": {\"path\": \"content\", ...}, \"module_name\": \"...\", \"description\": \"...\"}. Returns practical docs."
}

type docsOutput struct {
	Readme      string            `json:"readme"`
	ModuleProp  *modulePropEval   `json:"module_prop,omitempty"`
	GeneratedBy string            `json:"generated_by"`
	GeneratedAt string            `json:"generated_at"`
}

type modulePropEval struct {
	Valid   bool     `json:"valid"`
	Fields  []string `json:"fields_found"`
	Missing []string `json:"fields_missing"`
	Issues  []string `json:"issues"`
}

func (s *GenDocsSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	moduleName, _ := input["module_name"].(string)
	description, _ := input["description"].(string)

	filesRaw, _ := input["files"]
	filesMap, _ := filesRaw.(map[string]interface{})

	if moduleName == "" {
		moduleName = "MyModule"
	}
	if description == "" {
		description = "Android module for Magisk/KernelSU/APatch"
	}

	var propContent string
	var propEval *modulePropEval
	var hasShell, hasRust, hasGo, hasPython, hasWeb bool
	var fileList []string

	for path, contentRaw := range filesMap {
		content, _ := contentRaw.(string)
		fileList = append(fileList, path)

		if path == "module.prop" {
			propContent = content
		}

		switch detectLanguage(path) {
		case "shell":
			hasShell = true
		case "rust":
			hasRust = true
		case "go":
			hasGo = true
		case "python":
			hasPython = true
		case "html", "javascript", "css":
			hasWeb = true
		}
	}

	if propContent != "" {
		propEval = evaluateModuleProp(propContent)
	}

	sort.Strings(fileList)

	readme := generateReadme(moduleName, description, fileList, hasShell, hasRust, hasGo, hasPython, hasWeb, propEval)

	output := docsOutput{
		Readme:      readme,
		ModuleProp:  propEval,
		GeneratedBy: "ModuForge GenDocs Skill",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	b, _ := json.MarshalIndent(output, "", "  ")
	return string(b), nil
}

func evaluateModuleProp(content string) *modulePropEval {
	requiredFields := []string{"id", "name", "version", "author", "description"}
	knownFields := []string{"id", "name", "version", "versionCode", "author", "description", "minMagisk", "maxMagisk", "minKernelSU", "requireUtils", "support", "donate", "config"}
	var found, missing []string
	var issues []string

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 {
			field := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if value == "" {
				issues = append(issues, fmt.Sprintf("Field '%s' is empty", field))
			}
			for _, rf := range requiredFields {
				if field == rf {
					found = append(found, field)
				}
			}
			found = append(found, field)
		}
	}

	for _, rf := range requiredFields {
		rfFound := false
		for _, f := range found {
			if f == rf {
				rfFound = true
				break
			}
		}
		if !rfFound {
			missing = append(missing, rf)
		}
	}

	knownFound := make(map[string]bool)
	for _, f := range found {
		isKnown := false
		for _, kf := range knownFields {
			if f == kf {
				isKnown = true
				break
			}
		}
		if !isKnown && f != "" {
			knownFound[f] = true
		}
	}

	return &modulePropEval{
		Valid:   len(missing) == 0,
		Fields:  found,
		Missing: missing,
		Issues:  issues,
	}
}

func generateReadme(name, description string, files []string, hasShell, hasRust, hasGo, hasPython, hasWeb bool, prop *modulePropEval) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", name))
	sb.WriteString(fmt.Sprintf("%s\n\n", description))

	sb.WriteString("## Features\n\n")
	if hasShell {
		sb.WriteString("- Shell script integration for system-level operations\n")
	}
	if hasRust {
		sb.WriteString("- Rust-based components for high-performance operations\n")
	}
	if hasGo {
		sb.WriteString("- Go-based tools for concurrent tasks\n")
	}
	if hasPython {
		sb.WriteString("- Python scripting for flexible automation\n")
	}
	if hasWeb {
		sb.WriteString("- Web UI for interactive configuration\n")
	}
	sb.WriteString("- Compatible with Magisk, KernelSU, and APatch\n\n")

	sb.WriteString("## Installation\n\n")
	sb.WriteString("1. Download the latest release from the Releases page\n")
	sb.WriteString("2. Open your root manager app (Magisk/KernelSU/APatch)\n")
	sb.WriteString("3. Go to Modules section\n")
	sb.WriteString("4. Tap \"Install from storage\" and select the downloaded ZIP\n")
	sb.WriteString("5. Reboot your device\n\n")

	sb.WriteString("## Usage\n\n")
	if hasShell {
		sb.WriteString("After installation, the module activates automatically on boot.\n")
		sb.WriteString("Use `su -c \"module_name control\"` for management commands.\n\n")
	} else {
		sb.WriteString("After installation, reboot your device to activate the module.\n\n")
	}

	sb.WriteString("## Files\n\n")
	sb.WriteString("```\n")
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("├── %s\n", f))
	}
	sb.WriteString("```\n\n")

	if hasRust || hasGo {
		sb.WriteString("## Building from Source\n\n")
		if hasRust {
			sb.WriteString("### Rust Components\n")
			sb.WriteString("```bash\ncd rust_component\ncargo build --release\n```\n\n")
		}
		if hasGo {
			sb.WriteString("### Go Components\n")
			sb.WriteString("```bash\ncd go_component\ngo build -o binary_name\n```\n\n")
		}
	}

	sb.WriteString("## Compatibility\n\n")
	sb.WriteString("| Platform | Status |\n")
	sb.WriteString("|----------|--------|\n")
	sb.WriteString("| Magisk   | ✅ Tested |\n")
	sb.WriteString("| KernelSU | ✅ Compatible |\n")
	sb.WriteString("| APatch   | ✅ Compatible |\n\n")

	sb.WriteString("## Changelog\n\n")
	currentDate := time.Now().UTC().Format("2006-01-02")
	sb.WriteString(fmt.Sprintf("### v1.0.0 (%s)\n", currentDate))
	sb.WriteString("- Initial release\n")

	return sb.String()
}

func (s *GenDocsSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  true,
		Essential: false,
		NeedsDB:   false,
		NeedsLLM:  true,
	}
}
