package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/builder"
)

const (
	maxReviewRounds = 2
	teamLLMTimeout  = 120 * time.Second
)

// GeneratedFile represents a single generated file.
type GeneratedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// PlanResult is the output from the Planner agent.
type PlanResult struct {
	ModuleID string        `json:"module_id"`
	Files    []PlannedFile `json:"files"`
}

// PlannedFile describes a single file to generate.
type PlannedFile struct {
	Path        string `json:"path"`
	Description string `json:"description"`
	Language    string `json:"language"`
}

// ReviewResult is the output from the Reviewer agent.
type ReviewResult struct {
	Passed bool          `json:"passed"`
	Issues []ReviewIssue `json:"issues,omitempty"`
}

// ReviewIssue describes a single problem found by the reviewer.
type ReviewIssue struct {
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Level   string `json:"level"` // error, warning
	Message string `json:"message"`
}

// llmCaller is an interface for making LLM calls.
// Implemented by the builder package's callLLMForFix.
type llmCaller interface {
	CallLLM(ctx context.Context, prompt string) (string, error)
}

// builderLLMCaller wraps the builder package's LLM functions.
type builderLLMCaller struct {
	endpoint string
	apiKey   string
	model    string
}

func (c *builderLLMCaller) CallLLM(ctx context.Context, prompt string) (string, error) {
	return builder.CallLLMForFix(ctx, c.endpoint, c.apiKey, c.model, prompt)
}

// Team coordinates Planner → Coder → Reviewer collaboration.
type Team struct {
	caller llmCaller
}

// NewTeam creates a Team with resolved LLM configuration.
func NewTeam() *Team {
	endpoint, apiKey, model := builder.ResolveLLMForFix()
	if endpoint == "" || apiKey == "" {
		return nil
	}
	return &Team{
		caller: &builderLLMCaller{
			endpoint: endpoint,
			apiKey:   apiKey,
			model:    model,
		},
	}
}

// GenerateWithReview runs the full Planner → Coder → Reviewer loop.
func (t *Team) GenerateWithReview(
	ctx context.Context,
	description string,
	logFn func(string),
) ([]GeneratedFile, error) {
	if t == nil || t.caller == nil {
		return nil, fmt.Errorf("team not initialized: no LLM configured")
	}
	if logFn == nil {
		logFn = func(string) {}
	}

	// ═══════════════════════════════════════════════════════
	// Phase 1: Planner
	// ═══════════════════════════════════════════════════════
	logFn("[Team] Phase 1: Planning...\n")
	plan, err := t.plan(ctx, description, logFn)
	if err != nil {
		return nil, fmt.Errorf("planning failed: %w", err)
	}
	logFn(fmt.Sprintf("[Team] Plan: %d files to generate\n", len(plan.Files)))

	// ═══════════════════════════════════════════════════════
	// Phase 2: Coder
	// ═══════════════════════════════════════════════════════
	logFn("[Team] Phase 2: Coding...\n")
	files, err := t.code(ctx, description, plan, logFn)
	if err != nil {
		return nil, fmt.Errorf("coding failed: %w", err)
	}
	logFn(fmt.Sprintf("[Team] Generated %d files\n", len(files)))

	// ═══════════════════════════════════════════════════════
	// Phase 3: Review loop (max 2 rounds)
	// ═══════════════════════════════════════════════════════
	for round := 1; round <= maxReviewRounds; round++ {
		logFn(fmt.Sprintf("[Team] Phase 3: Review round %d/%d...\n", round, maxReviewRounds))

		review, err := t.review(ctx, description, files, logFn)
		if err != nil {
			logFn(fmt.Sprintf("[Team] Review error: %v (accepting current code)\n", err))
			break
		}

		if review.Passed {
			logFn("[Team] Review passed!\n")
			return files, nil
		}

		logFn(fmt.Sprintf("[Team] Review found %d issues, requesting fixes...\n", len(review.Issues)))
		for _, issue := range review.Issues {
			logFn(fmt.Sprintf("  - [%s] %s: %s\n", issue.Level, issue.File, issue.Message))
		}

		// ═══════════════════════════════════════════════════════
		// Phase 4: Fix (Coder + Reviewer feedback)
		// ═══════════════════════════════════════════════════════
		logFn(fmt.Sprintf("[Team] Phase 4: Fixing (round %d)...\n", round))
		files, err = t.fix(ctx, description, files, review, logFn)
		if err != nil {
			logFn(fmt.Sprintf("[Team] Fix error: %v\n", err))
			break
		}
	}

	return files, nil
}

// plan calls the LLM to analyze the requirement and output a file plan.
func (t *Team) plan(ctx context.Context, description string, logFn func(string)) (*PlanResult, error) {
	prompt := fmt.Sprintf(`You are a Magisk module architect. Analyze this requirement and output a JSON file plan.

## Requirement
%s

## Output format (valid JSON only):
{
  "module_id": "short-kebab-case-id",
  "files": [
    {"path": "module.prop", "description": "module metadata", "language": "prop"},
    {"path": "customize.sh", "description": "installer script", "language": "shell"},
    {"path": "service.sh", "description": "runs on boot", "language": "shell"},
    {"path": "src/main.go", "description": "main daemon", "language": "go"},
    {"path": "uninstall.sh", "description": "cleanup on remove", "language": "shell"}
  ]
}

## Rules
- module.prop is always required
- customize.sh is always required (Magisk installer)
- service.sh only if daemon/boot behavior needed
- Go files only for complex logic (daemon, data processing)
- Shell for everything else
- Include uninstall.sh if module creates files outside its directory
- Output ONLY the JSON plan, nothing else.`, description)

	ctx, cancel := context.WithTimeout(ctx, teamLLMTimeout)
	defer cancel()

	resp, err := t.caller.CallLLM(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Extract JSON from response
	resp = extractJSON(resp)

	var plan PlanResult
	if err := json.Unmarshal([]byte(resp), &plan); err != nil {
		return nil, fmt.Errorf("parse plan JSON: %w\nresponse: %s", err, truncateStr(resp, 500))
	}

	return &plan, nil
}

// code calls the LLM to generate code for each planned file.
func (t *Team) code(ctx context.Context, description string, plan *PlanResult, logFn func(string)) ([]GeneratedFile, error) {
	var files []GeneratedFile

	for _, pf := range plan.Files {
		logFn(fmt.Sprintf("  Generating %s (%s)...\n", pf.Path, pf.Language))

		prompt := buildCodePrompt(description, plan, pf)

		ctx, cancel := context.WithTimeout(ctx, teamLLMTimeout)
		resp, err := t.caller.CallLLM(ctx, prompt)
		cancel()

		if err != nil {
			logFn(fmt.Sprintf("  ⚠️  Failed to generate %s: %v\n", pf.Path, err))
			continue
		}

		content := extractCodeFromLLM(resp, pf.Language)
		if content == "" {
			content = resp
		}

		files = append(files, GeneratedFile{
			Path:    pf.Path,
			Content: content,
		})
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("failed to generate any files")
	}

	return files, nil
}

// review calls the LLM to check generated code for Magisk-specific issues.
func (t *Team) review(ctx context.Context, description string, files []GeneratedFile, logFn func(string)) (*ReviewResult, error) {
	// Build a summary of all generated files
	var codeSummary strings.Builder
	for _, f := range files {
		codeSummary.WriteString(fmt.Sprintf("### %s\n```\n%s\n```\n\n", f.Path, truncateStr(f.Content, 2000)))
	}

	prompt := fmt.Sprintf(`You are a Magisk module code reviewer. Check these generated files for issues.

## Original Requirement
%s

## Generated Files
%s

## Review checklist
1. Shell scripts: ${VAR} syntax (never bare $VAR), shebang #!/system/bin/sh
2. module.prop: key=value format, no quotes around values
3. set_perm / set_perm_recursive usage for binary files
4. No "rm -rf /" or "rm -rf /*" in any script
5. Go code: package main, all variables used, proper error handling
6. customize.sh: must call set_perm for binaries
7. service.sh: proper daemon lifecycle (PID file, nohup)
8. All scripts have proper shebang lines
9. No hardcoded paths outside /data/adb/modules/
10. uninstall.sh: only removes module-specific files

## Output format (valid JSON only):
{
  "passed": true/false,
  "issues": [
    {"file": "path", "line": 0, "level": "error|warning", "message": "description"}
  ]
}

If everything looks correct, set "passed": true and "issues": [].
Output ONLY the JSON.`, description, codeSummary.String())

	ctx, cancel := context.WithTimeout(ctx, teamLLMTimeout)
	defer cancel()

	resp, err := t.caller.CallLLM(ctx, prompt)
	if err != nil {
		return nil, err
	}

	resp = extractJSON(resp)

	var result ReviewResult
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return nil, fmt.Errorf("parse review JSON: %w", err)
	}

	return &result, nil
}

// fix calls the LLM to fix issues found by the reviewer.
func (t *Team) fix(ctx context.Context, description string, files []GeneratedFile, review *ReviewResult, logFn func(string)) ([]GeneratedFile, error) {
	// Build issue summary
	var issueSummary strings.Builder
	for _, issue := range review.Issues {
		issueSummary.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Level, issue.File, issue.Message))
	}

	// Build current code summary
	var codeSummary strings.Builder
	for _, f := range files {
		codeSummary.WriteString(fmt.Sprintf("### %s\n```\n%s\n```\n\n", f.Path, truncateStr(f.Content, 2000)))
	}

	prompt := fmt.Sprintf(`You are a Magisk module developer. Fix the issues found during code review.

## Original Requirement
%s

## Issues to fix
%s

## Current code
%s

## Instructions
- Fix ALL issues listed above
- Return the COMPLETE fixed files as a JSON array
- Do NOT change files that have no issues (unless needed for consistency)

## Output format (valid JSON only):
[{"path":"file/path","content":"complete fixed content"}]

Return ONLY the JSON array.`, description, issueSummary.String(), codeSummary.String())

	ctx, cancel := context.WithTimeout(ctx, teamLLMTimeout)
	defer cancel()

	resp, err := t.caller.CallLLM(ctx, prompt)
	if err != nil {
		return nil, err
	}

	resp = extractJSON(resp)

	var fixed []GeneratedFile
	if err := json.Unmarshal([]byte(resp), &fixed); err != nil {
		// Try to parse as single file
		var single GeneratedFile
		if err2 := json.Unmarshal([]byte(resp), &single); err2 == nil && single.Path != "" {
			fixed = []GeneratedFile{single}
		} else {
			return nil, fmt.Errorf("parse fix JSON: %w", err)
		}
	}

	// Merge: use fixed versions where available, keep originals otherwise
	fixedMap := make(map[string]GeneratedFile)
	for _, f := range fixed {
		fixedMap[f.Path] = f
	}

	result := make([]GeneratedFile, len(files))
	for i, f := range files {
		if fixed, ok := fixedMap[f.Path]; ok {
			result[i] = fixed
		} else {
			result[i] = f
		}
	}

	return result, nil
}

// buildCodePrompt creates the code generation prompt for a specific file.
func buildCodePrompt(description string, plan *PlanResult, pf PlannedFile) string {
	// Gather context from other planned files
	var otherFiles strings.Builder
	for _, of := range plan.Files {
		if of.Path != pf.Path {
			otherFiles.WriteString(fmt.Sprintf("- %s: %s\n", of.Path, of.Description))
		}
	}

	switch pf.Language {
	case "go":
		return fmt.Sprintf(`Generate a complete Go source file for a Magisk module.

## Requirement
%s

## File to generate
Path: %s
Purpose: %s

## Other files in module
%s

## CRITICAL GO RULES
1. Package main with func main()
2. Use only Go standard library
3. All variables must be declared and used
4. Handle all errors
5. Use only: int, int64, string, bool, float64, []byte, error, map, slice
6. Graceful shutdown: signal.NotifyContext
7. File paths: /data/adb/modules/<module-id>/
8. Logging: log.Println to stdout
9. Each function MUST be complete — no TODO, no placeholders

## OUTPUT FORMAT
{"path":"%s","content":"...full Go source code..."}

Return ONLY the JSON object.`, description, pf.Path, pf.Description, otherFiles.String(), pf.Path)

	case "shell":
		return fmt.Sprintf(`Generate a complete Shell script for a Magisk module.

## Requirement
%s

## File to generate
Path: %s
Purpose: %s

## Other files in module
%s

## CRITICAL SHELL RULES
1. ALWAYS use ${VAR} syntax, NEVER bare $VAR
2. All variables in double quotes: "${VAR}"
3. sleep takes INTEGER seconds only: sleep 5
4. Shebang: #!/system/bin/sh
5. Test syntax: [ "$x" = "yes" ] (spaces inside brackets)
6. NEVER use "rm -rf /" or "rm -rf /*"
7. Use set_perm / set_perm_recursive for binary files
8. module.prop: key=value format, no quotes

## OUTPUT FORMAT
{"path":"%s","content":"...full shell script..."}

Return ONLY the JSON object.`, description, pf.Path, pf.Description, otherFiles.String(), pf.Path)

	default:
		return fmt.Sprintf(`Generate the content for this file in a Magisk module.

## Requirement
%s

## File to generate
Path: %s
Purpose: %s

Return the complete file content.`, description, pf.Path, pf.Description)
	}
}

// extractJSON extracts the first JSON object or array from LLM response.
func extractJSON(resp string) string {
	// Try to find JSON block
	if idx := strings.Index(resp, "```json"); idx >= 0 {
		start := idx + 7
		end := strings.Index(resp[start:], "```")
		if end > 0 {
			return strings.TrimSpace(resp[start : start+end])
		}
	}
	if idx := strings.Index(resp, "```\n"); idx >= 0 {
		start := idx + 4
		end := strings.Index(resp[start:], "```")
		if end > 0 {
			return strings.TrimSpace(resp[start : start+end])
		}
	}

	// Try to find raw JSON (starts with { or [)
	trimmed := strings.TrimSpace(resp)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return trimmed
	}

	// Find first { or [
	for i, c := range trimmed {
		if c == '{' || c == '[' {
			return trimmed[i:]
		}
	}

	return resp
}

// extractCodeFromLLM extracts code content from LLM response based on language.
func extractCodeFromLLM(resp, lang string) string {
	fences := []string{
		"```" + lang,
		"```",
	}

	for _, fence := range fences {
		idx := strings.Index(resp, fence)
		if idx < 0 {
			continue
		}
		start := idx + len(fence)
		// Skip optional newline after fence
		if start < len(resp) && resp[start] == '\n' {
			start++
		}
		end := strings.Index(resp[start:], "```")
		if end > 0 {
			return strings.TrimSpace(resp[start : start+end])
		}
	}

	return ""
}

// truncateStr shortens a string to maxLen characters.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
