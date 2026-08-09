package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════════════
// Enhanced Plan Mode — File-level granularity task planning
// Inspired by: OpenHands PLAN.md, Claude Code's plan mode, Cline/Roo Code
//
// The enhanced planner:
// 1. Analyzes task complexity and determines planning depth
// 2. Generates file-level plans (which files to create/modify/delete)
// 3. Tracks dependencies between file operations
// 4. Provides progress tracking with file-level granularity
// 5. Supports incremental plan updates as implementation progresses
// ══════════════════════════════════════════════════════════════════?
// PlanDepth determines how detailed the plan should be.
type PlanDepth int

const (
	PlanSimple  PlanDepth = iota // 1-2 steps, no file details
	PlanStandard                 // 3-5 steps with file lists
	PlanDetailed                 // 6+ steps with file-level operations
)

// FileOperation represents a single file-level operation in the plan.
type FileOperation struct {
	Type    string // "create", "modify", "delete", "read", "test"
	Path    string // file path
	Reason  string // why this file needs to change
	Lines   int    // estimated lines of change (0 = unknown)
	DependsOn []string // paths this operation depends on
}

// PlanStep represents a high-level step in the execution plan.
type EnhancedPlanStep struct {
	ID           string          `json:"id"`
	Description  string          `json:"description"`
	Status       string          `json:"status"` // pending, in_progress, completed, failed, skipped
	Files        []FileOperation `json:"files,omitempty"`
	Dependencies []string        `json:"dependencies,omitempty"` // step IDs
	Result       string          `json:"result,omitempty"`
	Error        string          `json:"error,omitempty"`
	StartedAt    int64           `json:"started_at,omitempty"`
	CompletedAt  int64           `json:"completed_at,omitempty"`
}

// EnhancedPlan is a file-level granularity execution plan.
type EnhancedPlan struct {
	TaskID      string     `json:"task_id"`
	Task        string     `json:"task"`
	Depth       PlanDepth  `json:"depth"`
	Steps       []EnhancedPlanStep `json:"steps"`
	CreatedAt   int64      `json:"created_at"`
	UpdatedAt   int64      `json:"updated_at"`
	CurrentStep int        `json:"current_step"`
	Progress    int        `json:"progress"` // 0-100
}

// EnhancedPlanner creates and manages enhanced plans.
type EnhancedPlanner struct {
	db         *sql.DB
	depGraph   *FileDependencyGraph
	taskDecomp *TaskDecomposer
}

// NewEnhancedPlanner creates a new enhanced planner.
func NewEnhancedPlanner(db *sql.DB, projectPath string) *EnhancedPlanner {
	var depGraph *FileDependencyGraph
	if projectPath != "" {
		depGraph = NewFileDependencyGraph(projectPath)
	}
	return &EnhancedPlanner{
		db:         db,
		depGraph:   depGraph,
		taskDecomp: &TaskDecomposer{db: db},
	}
}

// ══════════════════════════════════════════════════════════════════?// Plan Generation ?Create enhanced plans with file-level detail
// ══════════════════════════════════════════════════════════════════?
// GeneratePlan creates an enhanced plan for a task.
func (ep *EnhancedPlanner) GeneratePlan(ctx context.Context, task string, projectContext string, r *AgentRunner, cfg RunConfig) (*EnhancedPlan, error) {
	start := time.Now()

	// Determine plan depth based on task complexity
	depth := ep.determineDepth(task)
	log.Printf("[EnhancedPlanner] task depth: %d for: %s", depth, task)

	plan := &EnhancedPlan{
		Task:      task,
		Depth:     depth,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	switch depth {
	case PlanSimple:
		plan.Steps = ep.generateSimplePlan(task, projectContext)
	case PlanStandard:
		plan.Steps = ep.generateStandardPlan(ctx, task, projectContext, r, cfg)
	case PlanDetailed:
		plan.Steps = ep.generateDetailedPlan(ctx, task, projectContext, r, cfg)
	}

	// Build dependency graph if available and needed
	if ep.depGraph != nil && depth >= PlanStandard {
		ep.depGraph.BuildGraph()
		ep.enrichWithDependencies(plan)
	}

	// Calculate initial progress
	plan.Progress = 0
	log.Printf("[EnhancedPlanner] generated plan with %d steps in %v", len(plan.Steps), time.Since(start))
	return plan, nil
}

// determineDepth analyzes the task and returns the appropriate planning depth.
func (ep *EnhancedPlanner) determineDepth(task string) PlanDepth {
	taskLower := strings.ToLower(task)
	taskLen := len(task)

	// Simple tasks: short, single action
	if taskLen < 30 && !strings.Contains(taskLower, "and") && !strings.Contains(taskLower, "同时") {
		return PlanSimple
	}

	// Count action indicators
	actionCount := 0
	for _, kw := range []string{
		"and", "then", "also", "同时", "并且", "然后", "接着",
		"create", "implement", "fix", "refactor", "add", "build", "optimize", "migrate", "delete", "remove",
		"实现", "创建", "修复", "重构", "优化", "迁移", "添加", "构建", "删除", "移除",
	} {
		if strings.Contains(taskLower, kw) {
			actionCount++
		}
	}

	// File count indicators
	fileIndicators := 0
	for _, kw := range []string{"file", "files", "module", "package", "文件", "模块", "包"} {
		if strings.Contains(taskLower, kw) {
			fileIndicators++
		}
	}

	// Complex tasks: multiple actions, file mentions, or long descriptions
	if actionCount >= 3 || fileIndicators >= 2 || taskLen > 150 {
		return PlanDetailed
	}

	if actionCount >= 2 || fileIndicators >= 1 || taskLen > 60 {
		return PlanStandard
	}

	return PlanSimple
}

// generateSimplePlan creates a minimal plan for simple tasks.
func (ep *EnhancedPlanner) generateSimplePlan(task string, projectContext string) []EnhancedPlanStep {
	taskLower := strings.ToLower(task)

	switch {
	case strings.Contains(taskLower, "fix") || strings.Contains(taskLower, "修复") || strings.Contains(taskLower, "debug"):
		return []EnhancedPlanStep{
			{ID: "diagnose", Description: "诊断问题原因", Status: "pending"},
			{ID: "fix", Description: "实施修复", Status: "pending", Dependencies: []string{"diagnose"}},
			{ID: "verify", Description: "验证修复结果", Status: "pending", Dependencies: []string{"fix"}},
		}
	case strings.Contains(taskLower, "add") || strings.Contains(taskLower, "添加") || strings.Contains(taskLower, "create"):
		return []EnhancedPlanStep{
			{ID: "analyze", Description: "分析需求", Status: "pending"},
			{ID: "implement", Description: "实现功能", Status: "pending", Dependencies: []string{"analyze"}},
			{ID: "verify", Description: "验证编译", Status: "pending", Dependencies: []string{"implement"}},
		}
	default:
		return []EnhancedPlanStep{
			{ID: "execute", Description: task, Status: "pending"},
		}
	}
}

// generateStandardPlan creates a plan with file-level awareness.
func (ep *EnhancedPlanner) generateStandardPlan(ctx context.Context, task string, projectContext string, r *AgentRunner, cfg RunConfig) []EnhancedPlanStep {
	// Use LLM to generate a detailed plan with file-level operations
	prompt := fmt.Sprintf(`Analyze this task and create a structured execution plan.

Task: %s

Project context: %s

Return a JSON array of steps. Each step must have:
- "id": short lowercase snake_case identifier
- "description": one-line Chinese description
- "files": array of objects with "type" (create/modify/delete/read) and "path" (file path)
- "dependencies": array of step IDs that must complete first (empty if none)

Rules:
1. Maximum 5 steps
2. Each step should involve 1-3 files
3. Order steps logically (analyze before implement, implement before verify)
4. For "fix" tasks: diagnose ?fix ?verify
5. For "create" tasks: analyze ?implement ?test ?verify

Return ONLY the JSON array, no explanation.`,
		truncateString(task, 500),
		truncateString(projectContext, 2000))

	summaryPrompt := []map[string]string{
		{"role": "system", "content": "You are a software engineering planner. Output only valid JSON arrays."},
		{"role": "user", "content": prompt},
	}

	result, err := r.callLLMSummary(ctx, cfg, summaryPrompt)
	if err != nil {
		log.Printf("[EnhancedPlanner] LLM planning failed: %v, using fallback", err)
		return ep.generateFallbackPlan(task)
	}

	// Parse LLM response
	steps := ep.parsePlanSteps(result)
	if len(steps) == 0 {
		return ep.generateFallbackPlan(task)
	}

	return steps
}

// generateDetailedPlan creates a comprehensive plan with deep file analysis.
func (ep *EnhancedPlanner) generateDetailedPlan(ctx context.Context, task string, projectContext string, r *AgentRunner, cfg RunConfig) []EnhancedPlanStep {
	// For detailed plans, we first analyze the codebase structure
	var codebaseInfo string
	if ep.depGraph != nil {
		stats := ep.depGraph.GetStats()
		statsJSON, _ := json.Marshal(stats)
		codebaseInfo = fmt.Sprintf("Codebase stats: %s", string(statsJSON))
	}

	prompt := fmt.Sprintf(`Analyze this complex task and create a detailed execution plan with file-level granularity.

Task: %s

Project context: %s

%s

Return a JSON array of steps. Each step must have:
- "id": short lowercase snake_case identifier
- "description": one-line Chinese description
- "files": array of objects with:
  - "type": "create"/"modify"/"delete"/"read"/"test"
  - "path": file path
  - "reason": why this file needs to change (one line)
- "dependencies": array of step IDs that must complete first

Rules:
1. Maximum 8 steps
2. Each step should focus on a logical unit of work
3. Include explicit "read" steps for understanding existing code
4. Include a "test" step before final verification
5. Order: analyze ?plan ?implement core ?implement details ?test ?verify
6. Group related file changes in the same step

Return ONLY the JSON array, no explanation.`,
		truncateString(task, 500),
		truncateString(projectContext, 2000),
		codebaseInfo)

	summaryPrompt := []map[string]string{
		{"role": "system", "content": "You are a senior software architect. Output only valid JSON arrays."},
		{"role": "user", "content": prompt},
	}

	result, err := r.callLLMSummary(ctx, cfg, summaryPrompt)
	if err != nil {
		log.Printf("[EnhancedPlanner] LLM detailed planning failed: %v, using standard plan", err)
		return ep.generateStandardPlan(ctx, task, projectContext, r, cfg)
	}

	steps := ep.parsePlanSteps(result)
	if len(steps) == 0 {
		return ep.generateStandardPlan(ctx, task, projectContext, r, cfg)
	}

	return steps
}

// generateFallbackPlan creates a plan when LLM is unavailable.
func (ep *EnhancedPlanner) generateFallbackPlan(task string) []EnhancedPlanStep {
	taskLower := strings.ToLower(task)

	if strings.Contains(taskLower, "fix") || strings.Contains(taskLower, "修复") {
		return []EnhancedPlanStep{
			{ID: "diagnose", Description: "诊断问题原因", Status: "pending",
				Files: []FileOperation{{Type: "read", Reason: "分析错误日志和相关代码"}}},
			{ID: "fix", Description: "实施修复", Status: "pending", Dependencies: []string{"diagnose"},
				Files: []FileOperation{{Type: "modify", Reason: "修复问题代码"}}},
			{ID: "verify", Description: "验证修复", Status: "pending", Dependencies: []string{"fix"},
				Files: []FileOperation{{Type: "test", Reason: "验证修复结果"}}},
		}
	}

	if strings.Contains(taskLower, "create") || strings.Contains(taskLower, "implement") || strings.Contains(taskLower, "实现") {
		return []EnhancedPlanStep{
			{ID: "analyze", Description: "分析需求和现有代码结构", Status: "pending",
				Files: []FileOperation{{Type: "read", Reason: "了解现有架构"}}},
			{ID: "implement", Description: "实现核心功能", Status: "pending", Dependencies: []string{"analyze"},
				Files: []FileOperation{{Type: "create", Reason: "创建新功能文件"}}},
			{ID: "integrate", Description: "集成到现有系统", Status: "pending", Dependencies: []string{"implement"},
				Files: []FileOperation{{Type: "modify", Reason: "更新入口点和配置"}}},
			{ID: "verify", Description: "验证编译和功能", Status: "pending", Dependencies: []string{"integrate"},
				Files: []FileOperation{{Type: "test", Reason: "运行测试验证"}}},
		}
	}

	if strings.Contains(taskLower, "refactor") || strings.Contains(taskLower, "重构") || strings.Contains(taskLower, "optimize") {
		return []EnhancedPlanStep{
			{ID: "analyze", Description: "分析当前代码", Status: "pending",
				Files: []FileOperation{{Type: "read", Reason: "理解现有实现"}}},
			{ID: "plan", Description: "制定重构计划", Status: "pending", Dependencies: []string{"analyze"}},
			{ID: "execute", Description: "执行重构", Status: "pending", Dependencies: []string{"plan"},
				Files: []FileOperation{{Type: "modify", Reason: "重构代码结构"}}},
			{ID: "verify", Description: "验证重构结果", Status: "pending", Dependencies: []string{"execute"},
				Files: []FileOperation{{Type: "test", Reason: "确保功能不变"}}},
		}
	}

	return []EnhancedPlanStep{
		{ID: "execute", Description: task, Status: "pending"},
	}
}

// ══════════════════════════════════════════════════════════════════?// Plan Parsing ?Parse LLM plan output
// ══════════════════════════════════════════════════════════════════?
// parsePlanSteps parses LLM output into structured plan steps.
func (ep *EnhancedPlanner) parsePlanSteps(llmOutput string) []EnhancedPlanStep {
	// Clean up the output
	result := strings.TrimSpace(llmOutput)

	// Strip markdown code fences
	if idx := strings.Index(result, "```"); idx >= 0 {
		result = strings.TrimPrefix(result[idx:], "```")
		result = strings.TrimPrefix(result, "json\n")
		result = strings.TrimPrefix(result, "json\r\n")
		if endIdx := strings.LastIndex(result, "```"); endIdx >= 0 {
			result = result[:endIdx]
		}
		result = strings.TrimSpace(result)
	}

	// Try to parse as JSON array
	var rawSteps []struct {
		ID           string `json:"id"`
		Description  string `json:"description"`
		Files        []struct {
			Type   string `json:"type"`
			Path   string `json:"path"`
			Reason string `json:"reason"`
		} `json:"files"`
		Dependencies []string `json:"dependencies"`
	}

	if err := json.Unmarshal([]byte(result), &rawSteps); err != nil {
		log.Printf("[EnhancedPlanner] failed to parse plan JSON: %v", err)
		return nil
	}

	// Convert to PlanStep
	steps := make([]EnhancedPlanStep, 0, len(rawSteps))
	for i, raw := range rawSteps {
		id := raw.ID
		if id == "" {
			id = fmt.Sprintf("step_%d", i)
		}

		files := make([]FileOperation, 0, len(raw.Files))
		for _, f := range raw.Files {
			files = append(files, FileOperation{
				Type:   f.Type,
				Path:   f.Path,
				Reason: f.Reason,
			})
		}

		steps = append(steps, EnhancedPlanStep{
			ID:           id,
			Description:  raw.Description,
			Status:       "pending",
			Files:        files,
			Dependencies: raw.Dependencies,
		})
	}

	// Validate dependencies reference existing IDs
	idSet := make(map[string]bool)
	for _, s := range steps {
		idSet[s.ID] = true
	}
	for i := range steps {
		validDeps := steps[i].Dependencies[:0]
		for _, dep := range steps[i].Dependencies {
			if idSet[dep] {
				validDeps = append(validDeps, dep)
			}
		}
		steps[i].Dependencies = validDeps
	}

	return steps
}

// ══════════════════════════════════════════════════════════════════?// Dependency Enrichment ?Add dependency info from graph
// ══════════════════════════════════════════════════════════════════?
// enrichWithDependencies adds dependency information to plan steps based on the dependency graph.
func (ep *EnhancedPlanner) enrichWithDependencies(plan *EnhancedPlan) {
	if ep.depGraph == nil {
		return
	}

	for i := range plan.Steps {
		step := &plan.Steps[i]
		for j := range step.Files {
			fileOp := &step.Files[j]
			if fileOp.Type == "modify" || fileOp.Type == "read" {
				// Get dependencies for this file
				deps := ep.depGraph.GetDependencies(fileOp.Path)
				if len(deps) > 0 {
					fileOp.DependsOn = deps
				}
			}
		}
	}
}

// ══════════════════════════════════════════════════════════════════?// Plan Management ?Track and update plan progress
// ══════════════════════════════════════════════════════════════════?
// GetCurrentStep returns the next step to execute.
func (ep *EnhancedPlanner) GetCurrentStep(plan *EnhancedPlan) *EnhancedPlanStep {
	completed := make(map[string]bool)
	for _, s := range plan.Steps {
		if s.Status == "completed" {
			completed[s.ID] = true
		}
	}

	for i := range plan.Steps {
		step := &plan.Steps[i]
		if step.Status != "pending" {
			continue
		}
		// Check if all dependencies are completed
		allDepsMet := true
		for _, dep := range step.Dependencies {
			if !completed[dep] {
				allDepsMet = false
				break
			}
		}
		if allDepsMet {
			return step
		}
	}

	return nil // all done or blocked
}

// AdvanceToNextStep marks the current step as completed and returns the next one.
func (ep *EnhancedPlanner) AdvanceToNextStep(plan *EnhancedPlan) *EnhancedPlanStep {
	current := ep.GetCurrentStep(plan)
	if current == nil {
		return nil
	}

	current.Status = "completed"
	current.CompletedAt = time.Now().Unix()
	ep.updateProgress(plan)

	return ep.GetCurrentStep(plan)
}

// MarkStepFailed marks the current step as failed.
func (ep *EnhancedPlanner) MarkStepFailed(plan *EnhancedPlan, errorMsg string) {
	current := ep.GetCurrentStep(plan)
	if current != nil {
		current.Status = "failed"
		current.Error = errorMsg
		current.CompletedAt = time.Now().Unix()
		ep.updateProgress(plan)
	}
}

// GetProgress returns the current progress percentage.
func (ep *EnhancedPlanner) GetProgress(plan *EnhancedPlan) int {
	ep.updateProgress(plan)
	return plan.Progress
}

// updateProgress recalculates the progress percentage.
func (ep *EnhancedPlanner) updateProgress(plan *EnhancedPlan) {
	if len(plan.Steps) == 0 {
		plan.Progress = 100
		return
	}

	completed := 0
	for _, s := range plan.Steps {
		if s.Status == "completed" {
			completed++
		}
	}

	plan.Progress = completed * 100 / len(plan.Steps)
	plan.UpdatedAt = time.Now().Unix()
}

// BuildContextMessage creates a system message with the current plan context.
func (ep *EnhancedPlanner) BuildContextMessage(plan *EnhancedPlan) string {
	if plan == nil {
		return ""
	}

	currentStep := ep.GetCurrentStep(plan)
	if currentStep == nil {
		return fmt.Sprintf("[Task Plan] All %d steps completed. Progress: %d%%", len(plan.Steps), plan.Progress)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[Task Plan ?Step %d/%d, Progress: %d%%]\n", plan.CurrentStep+1, len(plan.Steps), plan.Progress))

	sb.WriteString(fmt.Sprintf("Current step: %s\n", currentStep.Description))
	if len(currentStep.Files) > 0 {
		sb.WriteString("Files to work on:\n")
		for _, f := range currentStep.Files {
			sb.WriteString(fmt.Sprintf("  - %s %s: %s\n", f.Type, f.Path, f.Reason))
		}
	}

	sb.WriteString("\nCompleted steps:\n")
	for _, s := range plan.Steps {
		if s.Status == "completed" {
			sb.WriteString(fmt.Sprintf("  ?%s\n", s.Description))
		} else if s.Status == "failed" {
			sb.WriteString(fmt.Sprintf("  ?%s (failed: %s)\n", s.Description, s.Error))
		}
	}

	return sb.String()
}

// IsStepDone checks if a step's result indicates completion.
func (ep *EnhancedPlanner) IsStepDone(result string) bool {
	resultLower := strings.ToLower(result)
	return strings.Contains(resultLower, "完成") || strings.Contains(resultLower, "done") ||
		strings.Contains(resultLower, "successfully") || strings.Contains(resultLower, "verified") ||
		strings.Contains(resultLower, "passed") || strings.Contains(resultLower, "✅")
}
