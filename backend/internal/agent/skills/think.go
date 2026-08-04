package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type ThinkSkill struct{}

func NewThinkSkill() *ThinkSkill {
	return &ThinkSkill{}
}

func (s *ThinkSkill) Name() string {
	return "think"
}

func (s *ThinkSkill) Description() string {
	return "Reasoning and analysis. Input: {\"thought\": \"...\"}. Use to plan approach, analyze requirements, or break down complex tasks before taking action."
}

func (s *ThinkSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	thought, _ := input["thought"].(string)
	if thought == "" {
		return "", fmt.Errorf("thought is required")
	}

	// Sanitize: strip control chars, collapse whitespace, remove garbled text
	thought = sanitizeText(thought)

	analysis := analyzeThought(thought)

	result := map[string]interface{}{
		"thought":  thought,
		"analysis": analysis,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

// sanitizeText cleans up garbled/encoded text from LLM output.
func sanitizeText(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	prevWasSpace := false
	for _, r := range text {
		if r < 32 && r != '\n' && r != '\t' {
			continue
		}
		if (r >= 0x200B && r <= 0x200F) || (r >= 0x2028 && r <= 0x202E) ||
			(r >= 0x2060 && r <= 0x2069) || (r >= 0xFFF0 && r <= 0xFFFF) ||
			(r >= 0xE000 && r <= 0xF8FF) {
			continue
		}
		if r == ' ' || r == '\t' {
			if prevWasSpace { continue }
			prevWasSpace = true
			b.WriteRune(' ')
			continue
		}
		if r == '\n' {
			if prevWasSpace { continue }
			prevWasSpace = true
			b.WriteRune('\n')
			continue
		}
		prevWasSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// analyzeThought provides structured analysis of the thought content.
func analyzeThought(thought string) map[string]interface{} {
	lower := strings.ToLower(thought)
	analysis := make(map[string]interface{})

	// Detect task type
	taskType := "general"
	switch {
	case strings.Contains(lower, "create") || strings.Contains(lower, "generate") || strings.Contains(lower, "build") || strings.Contains(lower, "implement"):
		taskType = "creation"
	case strings.Contains(lower, "fix") || strings.Contains(lower, "repair") || strings.Contains(lower, "debug") || strings.Contains(lower, "error"):
		taskType = "repair"
	case strings.Contains(lower, "review") || strings.Contains(lower, "analyze") || strings.Contains(lower, "check") || strings.Contains(lower, "audit"):
		taskType = "analysis"
	case strings.Contains(lower, "optimize") || strings.Contains(lower, "improve") || strings.Contains(lower, "performance") || strings.Contains(lower, "refactor"):
		taskType = "optimization"
	case strings.Contains(lower, "explain") || strings.Contains(lower, "how") || strings.Contains(lower, "what is"):
		taskType = "explanation"
	}
	analysis["task_type"] = taskType

	// Detect complexity
	complexityIndicators := 0
	complexWords := []string{"multiple", "complex", "advanced", "full", "complete", "comprehensive", "detailed", "integrate", "system", "architecture", "several", "all files", "entire", "redesign"}
	for _, w := range complexWords {
		if strings.Contains(lower, w) {
			complexityIndicators++
		}
	}
	// File count signals complexity
	if strings.Contains(lower, "module") || strings.Contains(lower, "project") {
		complexityIndicators++
	}
	switch {
	case complexityIndicators >= 3:
		analysis["complexity"] = "high"
	case complexityIndicators >= 1:
		analysis["complexity"] = "medium"
	default:
		analysis["complexity"] = "low"
	}

	// Suggest relevant tools based on task keywords
	var suggestedTools []string
	toolSuggestions := map[string][]string{
		"read_file":    {"read", "show", "display", "open", "file", "source", "code"},
		"write_file":   {"write", "create", "save", "update", "modify", "change", "edit"},
		"lint_code":    {"lint", "syntax", "style", "format"},
		"validate":     {"validate", "verify", "check", "security", "safe"},
		"review_code":  {"review", "audit", "improve", "quality"},
		"test_module":  {"test", "verify", "check", "assert"},
		"build_module": {"build", "compile", "package", "zip"},
		"gen_docs":     {"document", "readme", "docs"},
		"match_template": {"template", "pattern", "scaffold"},
		"check_compat": {"compatibility", "compat", "magisk", "ksu", "apatch"},
		"profile_code": {"performance", "profile", "optimize", "slow"},
	}
	for tool, keywords := range toolSuggestions {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				suggestedTools = append(suggestedTools, tool)
				break
			}
		}
	}
	if len(suggestedTools) > 0 {
		analysis["suggested_tools"] = suggestedTools
	}

	// Extract key files mentioned
	var files []string
	words := strings.Fields(thought)
	for _, w := range words {
		clean := strings.Trim(w, ".,;:()[]{}!\"'")
		if strings.Contains(clean, ".") && len(clean) > 3 &&
			(strings.HasSuffix(clean, ".go") || strings.HasSuffix(clean, ".rs") || strings.HasSuffix(clean, ".sh") ||
				strings.HasSuffix(clean, ".py") || strings.HasSuffix(clean, ".java") || strings.HasSuffix(clean, ".kt") ||
				strings.HasSuffix(clean, ".xml") || strings.HasSuffix(clean, ".json") || strings.HasSuffix(clean, ".prop") ||
				strings.HasSuffix(clean, ".cpp") || strings.HasSuffix(clean, ".c") || strings.HasSuffix(clean, ".h")) {
			files = append(files, clean)
		}
	}
	if len(files) > 0 {
		analysis["files_mentioned"] = files
	}

	// Generate approach suggestion
	approach := generateApproach(taskType, analysis["complexity"].(string), suggestedTools)
	analysis["suggested_approach"] = approach

	return analysis
}

// generateApproach suggests a step-by-step approach based on analysis
func generateApproach(taskType, complexity string, tools []string) string {
	switch taskType {
	case "creation":
		if complexity == "high" {
			return "Use write_file to create each file directly (parent dirs are auto-created, do NOT use create_dir first). Plan: 1) Create module.prop and metadata, 2) Create main source files, 3) Add error handling, 4) Validate with lint/validate"
		}
		return "Use write_file to create each file directly (parent dirs are auto-created). Steps: 1) Create required files, 2) Validate the result"
	case "repair":
		return "1) Read the failing code, 2) Identify root cause, 3) Apply minimal fix, 4) Verify the fix works"
	case "analysis":
		return "1) Read relevant files, 2) Analyze structure/patterns, 3) Summarize findings"
	case "optimization":
		return "1) Profile current code, 2) Identify bottlenecks, 3) Apply optimizations, 4) Verify improvements"
	case "explanation":
		return "1) Read the relevant code, 2) Explain how it works, 3) Highlight key patterns"
	default:
		if complexity == "high" {
			return "Break down into smaller subtasks, tackle each systematically"
		}
		return "Assess the request, use appropriate tools, provide clear answer"
	}
}

func (s *ThinkSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  true,
		Essential: true,
		NeedsDB:   false,
		NeedsLLM:  false,
	}
}
