package service

import (
	"fmt"
	"strings"
)

// ModuleTemplate 表示一个模板
type ModuleTemplate struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"` // system, module, ui
	Tags        []string       `json:"tags"`
	Files       []TemplateFile `json:"files"`
}

type TemplateFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type TemplateService struct {
	templates map[string]*ModuleTemplate
}

func NewTemplateService() *TemplateService {
	ts := &TemplateService{
		templates: make(map[string]*ModuleTemplate),
	}
	ts.registerDefaults()
	return ts
}

func (s *TemplateService) registerDefaults() {
	s.templates["system.prop"] = &ModuleTemplate{
		Name:        "System.prop 模块",
		Description: "通过 system.prop 修改系统属性的 Magisk/KSU 模块",
		Category:    "system",
		Tags:        []string{"magisk", "ksu", "system", "prop"},
		Files: []TemplateFile{
			{Path: "module.prop", Content: "id=system_prop_mod\nname=System Prop Mod\nversion=v1.0\nversionCode=1\nauthor=ModuForge\ndescription=Custom system property modification module"},
			{Path: "system.prop", Content: "# System Properties\n# Add your properties below\n# Example:\n# debug.hwui.renderer=opengl\n"},
			{Path: "customize.sh", Content: "#!/system/bin/sh\n\nui_print \"- Installing system properties...\"\n"},
		},
	}
	s.templates["boot_animation"] = &ModuleTemplate{
		Name:        "开机动画模块",
		Description: "自定义开机动画的 Magisk 模块",
		Category:    "ui",
		Tags:        []string{"magisk", "boot", "animation", "ui"},
		Files: []TemplateFile{
			{Path: "module.prop", Content: "id=boot_animation\nname=Boot Animation\nversion=v1.0\nversionCode=1\nauthor=ModuForge\ndescription=Custom boot animation"},
			{Path: "customize.sh", Content: "#!/system/bin/sh\n\nui_print \"- Installing boot animation...\"\n"},
			{Path: "post-fs-data.sh", Content: "#!/system/bin/sh\n# Boot animation customization\n"},
		},
	}
	s.templates["audio_tweaks"] = &ModuleTemplate{
		Name:        "音频优化模块",
		Description: "音频参数优化的 Magisk/KSU 模块",
		Category:    "module",
		Tags:        []string{"magisk", "ksu", "audio", "tweaks"},
		Files: []TemplateFile{
			{Path: "module.prop", Content: "id=audio_tweaks\nname=Audio Tweaks\nversion=v1.0\nversionCode=1\nauthor=ModuForge\ndescription=Audio parameter optimization module"},
			{Path: "system/etc/audio_parameters.conf", Content: "# Audio Parameters\n"},
		},
	}
}

// ListTemplates 返回所有模板
func (s *TemplateService) ListTemplates() []*ModuleTemplate {
	result := make([]*ModuleTemplate, 0, len(s.templates))
	for _, t := range s.templates {
		result = append(result, t)
	}
	return result
}

// RecommendByDescription 根据用户描述推荐模板
func (s *TemplateService) RecommendByDescription(description string) []*ModuleTemplate {
	desc := strings.ToLower(description)
	var scored []struct {
		template *ModuleTemplate
		score    int
	}

	for _, t := range s.templates {
		score := 0
		// Check name
		if strings.Contains(strings.ToLower(t.Name), desc) {
			score += 3
		}
		// Check tags
		for _, tag := range t.Tags {
			if strings.Contains(desc, tag) {
				score += 2
			}
		}
		// Check description
		if strings.Contains(strings.ToLower(t.Description), desc) {
			score += 1
		}
		if score > 0 {
			scored = append(scored, struct {
				template *ModuleTemplate
				score    int
			}{t, score})
		}
	}

	// Sort by score desc
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	result := make([]*ModuleTemplate, 0, len(scored))
	for _, s := range scored {
		result = append(result, s.template)
	}
	return result
}

// GetTemplate 按名称获取模板
func (s *TemplateService) GetTemplate(name string) (*ModuleTemplate, error) {
	if t, ok := s.templates[name]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("template not found: %s", name)
}
