package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type MatchTemplateSkill struct{}

func NewMatchTemplateSkill() *MatchTemplateSkill {
	return &MatchTemplateSkill{}
}

func (s *MatchTemplateSkill) Name() string {
	return "match_template"
}

func (s *MatchTemplateSkill) Description() string {
	return "Match module description to best template. Input: {\"description\": \"...\", \"type\": \"...\", \"existing_style\": \"...\" (optional, e.g. 'shell' or 'go')}. Returns ranked templates with file structures."
}

type moduleTemplate struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	FileList    []string `json:"file_list"`
	Keywords    []string `json:"keywords"`
}

var templates = []moduleTemplate{
	{
		Name: "Systemless Root Module",
		Description: "Magisk-style systemless module that overlays files without modifying system partition. Suitable for creating, modifying, or replacing system files seamlessly.",
		FileList: []string{
			"module.prop",
			"customize.sh",
			"service.sh",
			"post-fs-data.sh",
			"bin/placeholder",
			"README.md",
		},
		Keywords: []string{"systemless", "overlay", "magisk", "root", "system", "module", "modify", "replace", "file"},
	},
	{
		Name: "Kernel Module Loader",
		Description: "Loads kernel modules (.ko files) at boot. Useful for adding kernel-level features like custom drivers, filesystem support, or hardware acceleration.",
		FileList: []string{
			"module.prop",
			"customize.sh",
			"service.sh",
			"kernel_modules/module.ko",
			"README.md",
		},
		Keywords: []string{"kernel", "module", ".ko", "driver", "kernelsu", "kernelsu", "boot", "load"},
	},
	{
		Name: "System Tweak / Optimization",
		Description: "Applies system-level tweaks for performance, battery life, or thermal management. Includes build.prop modifications, governor tuning, and scheduler optimization.",
		FileList: []string{
			"module.prop",
			"customize.sh",
			"service.sh",
			"system/system.prop",
			"README.md",
		},
		Keywords: []string{"tweak", "optimize", "performance", "battery", "thermal", "governor", "prop", "build.prop"},
	},
	{
		Name: "Debloater / App Remover",
		Description: "Removes or disables system applications without modifying the system partition. Uses Magisk mount mechanism or KSU's overlay system to hide unwanted apps.",
		FileList: []string{
			"module.prop",
			"customize.sh",
			"remove.sh",
			"system/app/placeholder",
			"README.md",
		},
		Keywords: []string{"debloat", "remove", "uninstall", "app", "package", "disable", "hide", "bloatware"},
	},
	{
		Name: "Backup & Restore Tool",
		Description: "Backs up and restores system configurations, apps, or data. Includes scheduling and selective backup of partitions or app data.",
		FileList: []string{
			"module.prop",
			"customize.sh",
			"service.sh",
			"backup.sh",
			"restore.sh",
			"config.json",
			"README.md",
		},
		Keywords: []string{"backup", "restore", "data", "config", "save", "recover", "schedule"},
	},
	{
		Name: "Ad Blocker (Hosts-based)",
		Description: "Blocks ads by modifying the system hosts file. Updates regularly from ad-blocking sources and supports custom blacklists/whitelists.",
		FileList: []string{
			"module.prop",
			"customize.sh",
			"service.sh",
			"system/etc/hosts",
			"blacklist.txt",
			"whitelist.txt",
			"README.md",
		},
		Keywords: []string{"ad", "block", "hosts", "adblock", "ad-block", "dns", "filter", "tracker"},
	},
	{
		Name: "Font Installer / Changer",
		Description: "Installs custom fonts system-wide by replacing the system font files. Supports multiple font formats and includes font preview functionality.",
		FileList: []string{
			"module.prop",
			"customize.sh",
			"service.sh",
			"system/fonts/",
			"README.md",
		},
		Keywords: []string{"font", "fonts", "typeface", "text", "customize", "change", "install", "ttf"},
	},
	{
		Name: "Boot Animation Changer",
		Description: "Replaces the boot animation with custom animations. Supports bootanimation.zip format and includes preview/fallback mechanisms.",
		FileList: []string{
			"module.prop",
			"customize.sh",
			"system/media/bootanimation.zip",
			"system/media/shutdownanimation.zip",
			"README.md",
		},
		Keywords: []string{"boot", "animation", "bootanimation", "boot", "anim", "splash", "logo", "startup"},
	},
}

func (s *MatchTemplateSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	description, _ := input["description"].(string)
	moduleType, _ := input["type"].(string)

	if description == "" && moduleType == "" {
		return "", fmt.Errorf("description or type is required")
	}

	query := strings.ToLower(description + " " + moduleType)
	queryWords := strings.Fields(query)

	scored := make([]struct {
		template moduleTemplate
		score    int
	}, len(templates))

	for i, tmpl := range templates {
		score := 0

		if moduleType != "" {
			mlower := strings.ToLower(tmpl.Name)
			tlower := strings.ToLower(moduleType)
			if strings.Contains(mlower, tlower) || strings.Contains(tlower, mlower) {
				score += 50
			}
		}

		for _, qw := range queryWords {
			for _, kw := range tmpl.Keywords {
				if kw == qw {
					score += 20
				} else if strings.Contains(kw, qw) || strings.Contains(qw, kw) {
					score += 10
				}
			}

			for _, fn := range tmpl.FileList {
				if strings.Contains(strings.ToLower(fn), qw) {
					score += 5
				}
			}
		}

		scored[i].template = tmpl
		scored[i].score = score
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	var results []map[string]interface{}
	for _, s := range scored {
		if s.score > 0 {
			results = append(results, map[string]interface{}{
				"name":        s.template.Name,
				"description": s.template.Description,
				"files":       s.template.FileList,
				"score":       s.score,
				"match":       matchLabel(s.score),
			})
		}
	}

	if len(results) == 0 {
		results = []map[string]interface{}{
			{
				"name":        "Custom Module",
				"description": "No existing template matches your description. A custom module structure is recommended.",
				"files":       []string{"module.prop", "customize.sh", "README.md"},
				"score":       0,
				"match":       "custom",
			},
		}
	}

	output := map[string]interface{}{
		"best_match": results[0],
		"all_matches": results,
	}

	b, _ := json.MarshalIndent(output, "", "  ")
	return string(b), nil
}

func matchLabel(score int) string {
	switch {
	case score >= 60:
		return "strong"
	case score >= 30:
		return "moderate"
	case score > 0:
		return "weak"
	default:
		return "none"
	}
}

func (s *MatchTemplateSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  true,
		Essential: false,
		NeedsDB:   false,
		NeedsLLM:  true,
	}
}
