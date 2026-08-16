package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moduforge/backend/internal/agent/registry"
)

// SmartRefactorSkill analyzes root causes and provides intelligent fixes
type SmartRefactorSkill struct {
	db *sql.DB
}

func init() {
	registry.RegisterFactory("smart_refactor", func(deps *registry.Deps) registry.Skill {
		return &SmartRefactorSkill{db: deps.DB}
	})
}

func (s *SmartRefactorSkill) Name() string {
	return "smart_refactor"
}

func (s *SmartRefactorSkill) Description() string {
	return `Analyze code issues and provide intelligent refactoring suggestions. Input: {"file_path": "...", "error": "...", "context": "...", "issue_type": "compile|runtime|performance|security"}`
}

type RefactorSuggestion struct {
	RootCause   string   `json:"root_cause"`
	Issue       string   `json:"issue"`
	Solution    string   `json:"solution"`
	CodeChanges []Change `json:"code_changes"`
	Risk        string   `json:"risk"` // low, medium, high
	Impact      string   `json:"impact"`
	References  []string `json:"references"`
}

type Change struct {
	File        string `json:"file"`
	OldCode     string `json:"old_code"`
	NewCode     string `json:"new_code"`
	Description string `json:"description"`
}

type RefactorResult struct {
	File        string              `json:"file"`
	IssueType   string              `json:"issue_type"`
	Suggestions []RefactorSuggestion `json:"suggestions"`
	AutoFixable bool                `json:"auto_fixable"`
	Confidence  float64             `json:"confidence"`
}

func (s *SmartRefactorSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	filePath, _ := input["file_path"].(string)
	errorMsg, _ := input["error"].(string)
	context, _ := input["context"].(string)
	issueType, _ := input["issue_type"].(string)

	if filePath == "" || errorMsg == "" {
		return "", fmt.Errorf("file_path and error are required")
	}

	if issueType == "" {
		issueType = "compile"
	}

	// Use rule-based analysis
	result := s.analyzeWithRules(filePath, errorMsg, context, issueType)

	// Format output
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(output), nil
}

func (s *SmartRefactorSkill) analyzeWithRules(filePath, errorMsg, context, issueType string) *RefactorResult {
	// Rule-based analysis
	var suggestions []RefactorSuggestion

	// Analyze based on issue type
	switch issueType {
	case "compile":
		suggestions = s.analyzeCompileErrors(filePath, errorMsg, context)
	case "runtime":
		suggestions = s.analyzeRuntimeErrors(filePath, errorMsg, context)
	case "performance":
		suggestions = s.analyzePerformanceIssues(filePath, errorMsg, context)
	case "security":
		suggestions = s.analyzeSecurityIssues(filePath, errorMsg, context)
	default:
		suggestions = s.createGenericSuggestion(errorMsg)
	}

	// Determine if auto-fixable
	autoFixable := false
	for _, s := range suggestions {
		if len(s.CodeChanges) > 0 {
			autoFixable = true
			break
		}
	}

	return &RefactorResult{
		File:        filePath,
		IssueType:   issueType,
		Suggestions: suggestions,
		AutoFixable: autoFixable,
		Confidence:  0.7,
	}
}

func (s *SmartRefactorSkill) analyzeCompileErrors(filePath, errorMsg, context string) []RefactorSuggestion {
	suggestions := []RefactorSuggestion{}

	// Common compile error patterns
	if strings.Contains(errorMsg, "undefined") {
		suggestions = append(suggestions, RefactorSuggestion{
			RootCause: "Reference to undefined variable, function, or type",
			Issue:     errorMsg,
			Solution:  "Check for typos, missing imports, or incorrect package references",
			CodeChanges: []Change{},
			Risk:      "low",
			Impact:    "Fixes compilation error",
			References: []string{"Go spec: Declarations and scope"},
		})
	} else if strings.Contains(errorMsg, "cannot use") {
		suggestions = append(suggestions, RefactorSuggestion{
			RootCause: "Type mismatch in assignment or function call",
			Issue:     errorMsg,
			Solution:  "Ensure types match or add type conversion",
			CodeChanges: []Change{},
			Risk:      "low",
			Impact:    "Fixes type compatibility",
			References: []string{"Go spec: Types"},
		})
	} else if strings.Contains(errorMsg, "not enough arguments") {
		suggestions = append(suggestions, RefactorSuggestion{
			RootCause: "Missing function arguments",
			Issue:     errorMsg,
			Solution:  "Add missing arguments or check function signature",
			CodeChanges: []Change{},
			Risk:      "low",
			Impact:    "Fixes function call",
			References: []string{"Go spec: Function calls"},
		})
	} else {
		suggestions = append(suggestions, RefactorSuggestion{
			RootCause: "Unknown compile error",
			Issue:     errorMsg,
			Solution:  "Review error message and check Go documentation",
			CodeChanges: []Change{},
			Risk:      "medium",
			Impact:    "Unknown",
			References: []string{},
		})
	}

	return suggestions
}

func (s *SmartRefactorSkill) analyzeRuntimeErrors(filePath, errorMsg, context string) []RefactorSuggestion {
	suggestions := []RefactorSuggestion{}

	if strings.Contains(errorMsg, "nil pointer") {
		suggestions = append(suggestions, RefactorSuggestion{
			RootCause: "Dereferencing nil pointer",
			Issue:     errorMsg,
			Solution:  "Add nil check before dereferencing",
			CodeChanges: []Change{},
			Risk:      "low",
			Impact:    "Prevents panic",
			References: []string{"Go wiki: Nil pointer dereference"},
		})
	} else if strings.Contains(errorMsg, "index out of range") {
		suggestions = append(suggestions, RefactorSuggestion{
			RootCause: "Array/slice index out of bounds",
			Issue:     errorMsg,
			Solution:  "Add bounds checking before access",
			CodeChanges: []Change{},
			Risk:      "low",
			Impact:    "Prevents panic",
			References: []string{"Go spec: Indexes"},
		})
	} else if strings.Contains(errorMsg, "goroutine") {
		suggestions = append(suggestions, RefactorSuggestion{
			RootCause: "Goroutine leak or race condition",
			Issue:     errorMsg,
			Solution:  "Review goroutine lifecycle and synchronization",
			CodeChanges: []Change{},
			Risk:      "medium",
			Impact:    "Fixes concurrency issue",
			References: []string{"Go wiki: Race detector"},
		})
	} else {
		suggestions = append(suggestions, RefactorSuggestion{
			RootCause: "Unknown runtime error",
			Issue:     errorMsg,
			Solution:  "Add logging and error handling",
			CodeChanges: []Change{},
			Risk:      "medium",
			Impact:    "Unknown",
			References: []string{},
		})
	}

	return suggestions
}

func (s *SmartRefactorSkill) analyzePerformanceIssues(filePath, errorMsg, context string) []RefactorSuggestion {
	suggestions := []RefactorSuggestion{}

	suggestions = append(suggestions, RefactorSuggestion{
		RootCause: "Performance bottleneck identified",
		Issue:     errorMsg,
		Solution:  "Profile code and optimize hot paths",
		CodeChanges: []Change{},
		Risk:      "medium",
		Impact:    "Improves performance",
		References: []string{"Go wiki: Profiling"},
	})

	return suggestions
}

func (s *SmartRefactorSkill) analyzeSecurityIssues(filePath, errorMsg, context string) []RefactorSuggestion {
	suggestions := []RefactorSuggestion{}

	if strings.Contains(errorMsg, "injection") {
		suggestions = append(suggestions, RefactorSuggestion{
			RootCause: "Potential injection vulnerability",
			Issue:     errorMsg,
			Solution:  "Validate and sanitize user input",
			CodeChanges: []Change{},
			Risk:      "high",
			Impact:    "Prevents security vulnerability",
			References: []string{"OWASP Top 10"},
		})
	} else if strings.Contains(errorMsg, "hardcoded") {
		suggestions = append(suggestions, RefactorSuggestion{
			RootCause: "Hardcoded sensitive data",
			Issue:     errorMsg,
			Solution:  "Move secrets to environment variables or secure storage",
			CodeChanges: []Change{},
			Risk:      "high",
			Impact:    "Improves security",
			References: []string{"OWASP: Secrets Management"},
		})
	} else {
		suggestions = append(suggestions, RefactorSuggestion{
			RootCause: "Potential security issue",
			Issue:     errorMsg,
			Solution:  "Review code for security best practices",
			CodeChanges: []Change{},
			Risk:      "medium",
			Impact:    "Improves security",
			References: []string{"OWASP Top 10"},
		})
	}

	return suggestions
}

func (s *SmartRefactorSkill) createGenericSuggestion(errorMsg string) []RefactorSuggestion {
	return []RefactorSuggestion{
		{
			RootCause: "Unable to determine root cause automatically",
			Issue:     errorMsg,
			Solution:  "Manual review required",
			CodeChanges: []Change{},
			Risk:      "medium",
			Impact:    "Unknown",
			References: []string{},
		},
	}
}

func (s *SmartRefactorSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  true,
		Essential: false,
		NeedsDB:   false,
		NeedsLLM:  true,
	}
}