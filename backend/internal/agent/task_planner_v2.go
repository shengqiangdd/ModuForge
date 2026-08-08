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
// TaskPlannerV2 - 改进的任务分解系统 (参考 OpenHands + MetaGPT + AutoGPT)
// ═══════════════════════════════════════════════════════════════════

// V2Step represents a single executable step
type V2Step struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Files       []string `json:"files,omitempty"`
	Tools       []string `json:"tools,omitempty"`
	Status      string   `json:"status"` // pending, in_progress, completed, failed
	Result      string   `json:"result,omitempty"`
	RetryCount  int      `json:"retry_count,omitempty"`
}

// V2Plan represents the execution plan
type V2Plan struct {
	ID           string    `json:"id"`
	Task         string    `json:"task"`
	Steps        []V2Step  `json:"steps"`
	CurrentIdx   int       `json:"current_idx"`
	CreatedAt    int64     `json:"created_at"`
	Status       string    `json:"status"` // planning, executing, completed, failed
}

// TaskPlannerV2 manages task decomposition
type TaskPlannerV2 struct {
	db *sql.DB
}

// NewTaskPlannerV2 creates a new instance
func NewTaskPlannerV2(db *sql.DB) *TaskPlannerV2 {
	return &TaskPlannerV2{db: db}
}

// ═══════════════════════════════════════════════════════════════════
// 1. Plan Creation - 使用LLM生成详细步骤
// ═══════════════════════════════════════════════════════════════════

// CreatePlan generates an execution plan
func (tp *TaskPlannerV2) CreatePlan(ctx context.Context, task string, projectContext string, r *AgentRunner, cfg RunConfig) (*V2Plan, error) {
	prompt := fmt.Sprintf(`你是一个任务规划专家。请将任务分解为可执行的步骤。

任务: %s

项目上下文:
%s

要求:
1. 分解为3-8个具体步骤
2. 每个步骤描述要具体、可执行
3. 标注涉及的文件和推荐工具
4. 步骤按依赖顺序排列

返回JSON数组，每个元素:
{
  "id": "step_1",
  "description": "具体步骤描述",
  "files": ["涉及的文件"],
  "tools": ["推荐工具"]
}

只返回JSON数组，不要其他内容:`, task, projectContext)

	summaryPrompt := []map[string]string{
		{"role": "system", "content": "你是任务规划助手。只输出有效的JSON数组。"},
		{"role": "user", "content": prompt},
	}

	result, err := r.callLLMSummary(ctx, cfg, summaryPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM planning failed: %w", err)
	}

	// Parse response
	result = strings.TrimSpace(result)
	if idx := strings.Index(result, "```"); idx >= 0 {
		result = strings.TrimPrefix(result[idx:], "```")
		result = strings.TrimPrefix(result, "json\n")
		if endIdx := strings.LastIndex(result, "```"); endIdx >= 0 {
			result = result[:endIdx]
		}
		result = strings.TrimSpace(result)
	}

	var steps []V2Step
	if err := json.Unmarshal([]byte(result), &steps); err != nil {
		log.Printf("[TaskPlannerV2] Failed to parse plan: %v (raw=%s)", err, result)
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}

	if len(steps) == 0 {
		return nil, fmt.Errorf("plan has no steps")
	}

	// Initialize statuses
	for i := range steps {
		steps[i].Status = "pending"
		if steps[i].ID == "" {
			steps[i].ID = fmt.Sprintf("step_%d", i+1)
		}
	}

	plan := &V2Plan{
		ID:        fmt.Sprintf("plan_%d", time.Now().UnixMilli()),
		Task:      task,
		Steps:     steps,
		CurrentIdx: 0,
		CreatedAt: time.Now().UnixMilli(),
		Status:    "executing",
	}

	log.Printf("[TaskPlannerV2] Created plan with %d steps", len(steps))
	return plan, nil
}

// ═══════════════════════════════════════════════════════════════════
// 2. Current Step Management
// ═══════════════════════════════════════════════════════════════════

// GetCurrentStep returns the current step to execute
func (tp *TaskPlannerV2) GetCurrentStep(plan *V2Plan) *V2Step {
	if plan == nil || plan.CurrentIdx >= len(plan.Steps) {
		return nil
	}
	return &plan.Steps[plan.CurrentIdx]
}

// AdvanceToNextStep moves to the next step
func (tp *TaskPlannerV2) AdvanceToNextStep(plan *V2Plan) *V2Step {
	if plan == nil {
		return nil
	}

	// Mark current step as completed if pending/in_progress
	if plan.CurrentIdx < len(plan.Steps) {
		step := &plan.Steps[plan.CurrentIdx]
		if step.Status == "pending" || step.Status == "in_progress" {
			step.Status = "completed"
		}
	}

	// Move to next
	plan.CurrentIdx++
	if plan.CurrentIdx >= len(plan.Steps) {
		plan.Status = "completed"
		return nil
	}

	// Mark new current step as in_progress
	newStep := &plan.Steps[plan.CurrentIdx]
	newStep.Status = "in_progress"
	return newStep
}

// MarkStepFailed marks current step as failed
func (tp *TaskPlannerV2) MarkStepFailed(plan *V2Plan, error string) {
	if plan == nil || plan.CurrentIdx >= len(plan.Steps) {
		return
	}

	step := &plan.Steps[plan.CurrentIdx]
	step.RetryCount++
	step.Result = error

	if step.RetryCount >= 3 {
		step.Status = "failed"
		// Try next step
		plan.CurrentIdx++
		if plan.CurrentIdx < len(plan.Steps) {
			plan.Steps[plan.CurrentIdx].Status = "in_progress"
		} else {
			plan.Status = "failed"
		}
	} else {
		// Retry same step
		step.Status = "pending"
	}
}

// ═══════════════════════════════════════════════════════════════════
// 3. Context Injection - 关键: 让LLM知道当前任务
// ═══════════════════════════════════════════════════════════════════

// BuildContextMessage builds a context message for the LLM
func (tp *TaskPlannerV2) BuildContextMessage(plan *V2Plan) string {
	if plan == nil || plan.CurrentIdx >= len(plan.Steps) {
		return ""
	}

	step := &plan.Steps[plan.CurrentIdx]
	completedCount := 0
	for _, s := range plan.Steps {
		if s.Status == "completed" {
			completedCount++
		}
	}

	var sb strings.Builder
	sb.WriteString("═══ 任务执行计划 ═══\n")
	sb.WriteString(fmt.Sprintf("总进度: %d/%d 步骤完成\n\n", completedCount, len(plan.Steps)))

	// Show all steps with status
	for i, s := range plan.Steps {
		icon := "⬜"
		switch s.Status {
		case "in_progress":
			icon = "🔄"
		case "completed":
			icon = "✅"
		case "failed":
			icon = "❌"
		}

		if i == plan.CurrentIdx {
			sb.WriteString(fmt.Sprintf("%s **[当前]** %s\n", icon, s.Description))
		} else {
			sb.WriteString(fmt.Sprintf("%s %s\n", icon, s.Description))
		}
	}

	sb.WriteString("\n═══ 当前任务 ═══\n")
	sb.WriteString(fmt.Sprintf("请完成: %s\n", step.Description))

	if len(step.Files) > 0 {
		sb.WriteString(fmt.Sprintf("涉及文件: %s\n", strings.Join(step.Files, ", ")))
	}
	if len(step.Tools) > 0 {
		sb.WriteString(fmt.Sprintf("推荐工具: %s\n", strings.Join(step.Tools, ", ")))
	}

	sb.WriteString("\n完成后请回复 DONE，系统会自动进入下一步。")
	return sb.String()
}

// ═══════════════════════════════════════════════════════════════════
// 4. Completion Detection
// ═══════════════════════════════════════════════════════════════════

// IsStepDone checks if the agent's response indicates step completion
func (tp *TaskPlannerV2) IsStepDone(response string) bool {
	lower := strings.ToLower(response)
	// Check for completion signals
	doneSignals := []string{"done", "完成", "已完成", "finished", "completed", "next"}
	for _, signal := range doneSignals {
		if strings.Contains(lower, signal) {
			return true
		}
	}
	return false
}

// ═══════════════════════════════════════════════════════════════════
// 5. Progress Tracking
// ═══════════════════════════════════════════════════════════════════

// GetProgress returns progress percentage
func (tp *TaskPlannerV2) GetProgress(plan *V2Plan) float64 {
	if plan == nil || len(plan.Steps) == 0 {
		return 0
	}

	completed := 0
	for _, s := range plan.Steps {
		if s.Status == "completed" {
			completed++
		}
	}
	return float64(completed) / float64(len(plan.Steps)) * 100
}

// FormatAsMarkdown formats plan as markdown
func (tp *TaskPlannerV2) FormatAsMarkdown(plan *V2Plan) string {
	if plan == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# 任务执行计划\n\n")
	sb.WriteString(fmt.Sprintf("任务: %s\n\n", plan.Task))
	sb.WriteString(fmt.Sprintf("状态: %s | 进度: %.0f%%\n\n", plan.Status, tp.GetProgress(plan)))

	for i, step := range plan.Steps {
		icon := "⬜"
		switch step.Status {
		case "in_progress":
			icon = "🔄"
		case "completed":
			icon = "✅"
		case "failed":
			icon = "❌"
		}

		marker := ""
		if i == plan.CurrentIdx {
			marker = " **[当前]**"
		}

		sb.WriteString(fmt.Sprintf("%s Step %d: %s%s\n", icon, i+1, step.Description, marker))
		if step.Result != "" {
			sb.WriteString(fmt.Sprintf("   结果: %s\n", step.Result))
		}
	}

	return sb.String()
}
