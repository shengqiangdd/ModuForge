package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type GatherRequirementsSkill struct{}

func NewGatherRequirementsSkill() *GatherRequirementsSkill {
	return &GatherRequirementsSkill{}
}

func (s *GatherRequirementsSkill) Name() string {
	return "gather_requirements"
}

func (s *GatherRequirementsSkill) Description() string {
	return "Collect module requirements through structured Q&A. Input: {\"description\": \"...\", \"answers\": {\"q1\": \"...\"}} or {\"description\": \"...\"}. Returns structured spec."
}

type requirementDoc struct {
	ModuleName          string   `json:"module_name"`
	Description         string   `json:"description"`
	TargetFrameworks    []string `json:"target_frameworks"`
	Features            []string `json:"features"`
	UIRequired          bool     `json:"ui_required"`
	SpecialRequirements string   `json:"special_requirements"`
	Language            string   `json:"language,omitempty"`
	Architecture        string   `json:"architecture,omitempty"`
}

var gatherQuestions = []struct {
	Key         string
	Question    string
	Required    bool
	DefaultHint string
}{
	{Key: "q1", Question: "目标设备和 Android 版本？（如 Android 12-15, ARM64）", Required: true, DefaultHint: "默认: Android 12-15, ARM64"},
	{Key: "q2", Question: "是否需要 WebUI 界面？", Required: false, DefaultHint: "默认: 否"},
	{Key: "q3", Question: "兼容哪些框架？（Magisk/KernelSU/APatch/全部）", Required: false, DefaultHint: "默认: 全部兼容"},
	{Key: "q4", Question: "核心功能列表？（每行一个功能点）", Required: true, DefaultHint: ""},
	{Key: "q5", Question: "特殊需求？（性能/依赖/权限等）", Required: false, DefaultHint: "默认: 无"},
}

func (s *GatherRequirementsSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	description, _ := input["description"].(string)
	if description == "" {
		return "", fmt.Errorf("description is required")
	}

	answersRaw, hasAnswers := input["answers"]
	answersMap, _ := answersRaw.(map[string]interface{})

	if hasAnswers && len(answersMap) > 0 {
		return s.buildRequirementDoc(description, answersMap)
	}

	return s.askQuestions(description)
}

func (s *GatherRequirementsSkill) askQuestions(description string) (string, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("收到需求：%s\n\n", description))
	sb.WriteString("请回答以下问题以生成完整的模块规格：\n\n")
	for _, q := range gatherQuestions {
		req := ""
		if q.Required {
			req = " [必填]"
		}
		sb.WriteString(fmt.Sprintf("%s%s\n", q.Question, req))
		if q.DefaultHint != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", q.DefaultHint))
		}
	}
	sb.WriteString("\n回答格式: {\"action\": {\"skill\": \"gather_requirements\", \"input\": {\"description\": \"...\", \"answers\": {\"q1\": \"...\", \"q2\": \"...\", \"q3\": \"...\", \"q4\": \"...\", \"q5\": \"...\"}}}}")

	var qList []map[string]interface{}
	for _, q := range gatherQuestions {
		qList = append(qList, map[string]interface{}{
			"key":      q.Key,
			"question": q.Question,
			"required": q.Required,
			"default":  q.DefaultHint,
		})
	}

	result := map[string]interface{}{
		"phase":     "questioning",
		"prompt":    sb.String(),
		"questions": qList,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *GatherRequirementsSkill) buildRequirementDoc(description string, answers map[string]interface{}) (string, error) {
	qaText := ""
	for _, q := range gatherQuestions {
		ans, _ := answers[q.Key].(string)
		if ans == "" {
			// Try numeric key fallback
			for k, v := range answers {
				if strings.Contains(k, q.Key[1:]) {
					ans, _ = v.(string)
					break
				}
			}
		}
		if ans == "" && q.Required && q.DefaultHint != "" {
			ans = q.DefaultHint
		}
		qaText += fmt.Sprintf("Q: %s\nA: %s\n\n", q.Question, ans)
	}

	return s.generateSpec(description, qaText)
}

func (s *GatherRequirementsSkill) generateSpec(description, qaText string) (string, error) {
	spec := requirementDoc{
		ModuleName:         extractModuleName(description),
		Description:        description,
		TargetFrameworks:   detectFrameworks(qaText),
		Features:           extractFeatures(qaText),
		UIRequired:         strings.Contains(strings.ToLower(qaText), "需要") && (strings.Contains(strings.ToLower(qaText), "ui") || strings.Contains(strings.ToLower(qaText), "界面") || strings.Contains(strings.ToLower(qaText), "webui")),
		SpecialRequirements: extractSpecial(qaText),
	}

	result := map[string]interface{}{
		"phase":     "completed",
		"spec":      spec,
		"qa_summary": qaText,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func extractModuleName(description string) string {
	desc := strings.ToLower(description)
	prefixes := []string{"模块叫", "模块名称", "名称", "name is", "called", "名为"}
	for _, p := range prefixes {
		idx := strings.Index(desc, p)
		if idx >= 0 {
			remain := desc[idx+len(p):]
			remain = strings.TrimSpace(remain)
			if end := strings.IndexAny(remain, "，。,.;;"); end >= 0 {
				return strings.TrimSpace(remain[:end])
			}
			return remain
		}
	}
	return ""
}

func detectFrameworks(qaText string) []string {
	var frameworks []string
	lower := strings.ToLower(qaText)
	if strings.Contains(lower, "magisk") {
		frameworks = append(frameworks, "Magisk")
	}
	if strings.Contains(lower, "kernelsu") || strings.Contains(lower, "ksu") {
		frameworks = append(frameworks, "KernelSU")
	}
	if strings.Contains(lower, "apatch") {
		frameworks = append(frameworks, "APatch")
	}
	if len(frameworks) == 0 {
		frameworks = append(frameworks, "Magisk", "KernelSU", "APatch")
	}
	return frameworks
}

func extractFeatures(qaText string) []string {
	var features []string
	lines := strings.Split(qaText, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "•") {
			feature := strings.TrimSpace(trimmed[1:])
			if feature != "" && len(feature) < 100 {
				features = append(features, feature)
			}
		}
	}
	return features
}

func extractSpecial(qaText string) string {
	lines := strings.Split(qaText, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(strings.ToLower(trimmed), "特殊") || strings.Contains(strings.ToLower(trimmed), "注意") || strings.Contains(strings.ToLower(trimmed), "其他") {
			if idx := strings.Index(trimmed, "："); idx >= 0 {
				return strings.TrimSpace(trimmed[idx+3:])
			}
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return strings.TrimSpace(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func (s *GatherRequirementsSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  true,
		Essential: false,
		NeedsDB:   false,
		NeedsLLM:  true,
	}
}
