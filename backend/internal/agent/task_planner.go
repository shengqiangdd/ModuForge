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
// TaskPlanner - 改进的任务分解系统
// 参考: OpenHands (PLAN.md持久化) + MetaGPT (角色化SOP) + AutoGPT (循环执行)
// ═══════════════════════════════════════════════════════════════════

// PlanPhase represents a phase in the execution plan
type PlanPhase struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Steps       []PlanStep        `json:"steps"`
	Status      string            `json:"status"` // pending, in_progress, completed, failed
	StartedAt   int64             `json:"started_at,omitempty"`
	CompletedAt int64             `json:"completed_at,omitempty"`
}

// PlanStep represents a single step within a phase
type PlanStep struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Files       []string `json:"files,omitempty"`
	Tools       []string `json:"tools,omitempty"` // recommended tools
	Status      string   `json:"status"`          // pending, in_progress, completed, failed, skipped
	Result      string   `json:"result,omitempty"`
	StartedAt   int64    `json:"started_at,omitempty"`
	CompletedAt int64    `json:"completed_at,omitempty"`
	RetryCount  int      `json:"retry_count,omitempty"`
}

// ExecutionPlan represents the complete task execution plan
type ExecutionPlan struct {
	ID          string      `json:"id"`
	Task        string      `json:"task"`
	Phases      []PlanPhase `json:"phases"`
	CreatedAt   int64       `json:"created_at"`
	UpdatedAt   int64       `json:"updated_at"`
	Status      string      `json:"status"` // planning, executing, completed, failed
	CurrentPhase int        `json:"current_phase"`
	CurrentStep  int        `json:"current_step"`
}

// TaskPlanner manages task decomposition and execution tracking
type TaskPlanner struct {
	db *sql.DB
}

// NewTaskPlanner creates a new TaskPlanner instance
func NewTaskPlanner(db *sql.DB) *TaskPlanner {
	return &TaskPlanner{db: db}
}

// ═══════════════════════════════════════════════════════════════════
// Phase 1: Planning - 使用LLM生成详细执行计划
// 参考: OpenHands Planning Agent
// ═══════════════════════════════════════════════════════════════════

// CreatePlan generates an execution plan using LLM
func (tp *TaskPlanner) CreatePlan(ctx context.Context, task string, projectContext string, r *AgentRunner, cfg RunConfig) (*ExecutionPlan, error) {
	prompt := fmt.Sprintf(`你是一个任务规划专家。请为以下任务创建详细的执行计划。

任务: %s

项目上下文:
%s

要求:
1. 将任务分解为2-5个阶段(Phase)，每个阶段有明确目标
2. 每个阶段包含2-5个具体步骤(Step)
3. 每个步骤描述要具体、可执行
4. 标注每个步骤可能涉及的文件和推荐工具
5. 步骤之间要有依赖关系

返回JSON格式:
{
  "phases": [
    {
      "id": "phase_1",
      "name": "阶段名称",
      "description": "阶段目标描述",
      "steps": [
        {
          "id": "step_1",
          "description": "具体步骤描述",
          "files": ["涉及的文件路径"],
          "tools": ["推荐使用的工具"]
        }
      ]
    }
  ]
}`, task, projectContext)

	summaryPrompt := []map[string]string{
		{"role": "system", "content": "你是一个任务规划助手。输出有效的JSON，不要包含markdown代码块。"},
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

	var planData struct {
		Phases []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Steps       []struct {
				ID          string   `json:"id"`
				Description string   `json:"description"`
				Files       []string `json:"files"`
				Tools       []string `json:"tools"`
			} `json:"steps"`
		} `json:"phases"`
	}

	if err := json.Unmarshal([]byte(result), &planData); err != nil {
		log.Printf("[TaskPlanner] Failed to parse plan JSON: %v (raw=%s)", err, result)
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}

	if len(planData.Phases) == 0 {
		return nil, fmt.Errorf("plan has no phases")
	}

	// Convert to ExecutionPlan
	plan := &ExecutionPlan{
		ID:        fmt.Sprintf("plan_%d", time.Now().UnixMilli()),
		Task:      task,
		Status:    "planning",
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}

	for _, p := range planData.Phases {
		phase := PlanPhase{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Status:      "pending",
		}
		for _, s := range p.Steps {
			step := PlanStep{
				ID:          s.ID,
				Description: s.Description,
				Files:       s.Files,
				Tools:       s.Tools,
				Status:      "pending",
			}
			phase.Steps = append(phase.Steps, step)
		}
		plan.Phases = append(plan.Phases, phase)
	}

	log.Printf("[TaskPlanner] Created plan with %d phases", len(plan.Phases))
	return plan, nil
}

// ═══════════════════════════════════════════════════════════════════
// Phase 2: Execution Tracking - 跟踪执行进度
// 参考: MetaGPT QA验证 + AutoGPT循环
// ═══════════════════════════════════════════════════════════════════

// GetCurrentStep returns the current step to execute
func (tp *TaskPlanner) GetCurrentStep(plan *ExecutionPlan) (*PlanPhase, *PlanStep) {
	if plan == nil || len(plan.Phases) == 0 {
		return nil, nil
	}

	for pi := plan.CurrentPhase; pi < len(plan.Phases); pi++ {
		phase := &plan.Phases[pi]
		for si := 0; si < len(phase.Steps); si++ {
			step := &phase.Steps[si]
			if step.Status == "pending" {
				return phase, step
			}
		}
		// All steps in phase completed, move to next phase
		if phase.Status != "completed" {
			phase.Status = "completed"
			phase.CompletedAt = time.Now().UnixMilli()
		}
		plan.CurrentPhase = pi + 1
	}

	return nil, nil // All done
}

// MarkStepStarted marks a step as in progress
func (tp *TaskPlanner) MarkStepStarted(plan *ExecutionPlan, phaseID, stepID string) {
	for pi := range plan.Phases {
		if plan.Phases[pi].ID == phaseID {
			phase := &plan.Phases[pi]
			if phase.Status == "pending" {
				phase.Status = "in_progress"
				phase.StartedAt = time.Now().UnixMilli()
			}
			for si := range phase.Steps {
				if phase.Steps[si].ID == stepID {
					step := &phase.Steps[si]
					step.Status = "in_progress"
					step.StartedAt = time.Now().UnixMilli()
					plan.CurrentPhase = pi
					plan.CurrentStep = si
					plan.UpdatedAt = time.Now().UnixMilli()
					return
				}
			}
		}
	}
}

// MarkStepCompleted marks a step as completed
func (tp *TaskPlanner) MarkStepCompleted(plan *ExecutionPlan, phaseID, stepID, result string) {
	for pi := range plan.Phases {
		if plan.Phases[pi].ID == phaseID {
			for si := range plan.Phases[pi].Steps {
				if plan.Phases[pi].Steps[si].ID == stepID {
					step := &plan.Phases[pi].Steps[si]
					step.Status = "completed"
					step.Result = result
					step.CompletedAt = time.Now().UnixMilli()
					plan.UpdatedAt = time.Now().UnixMilli()
					return
				}
			}
		}
	}
}

// MarkStepFailed marks a step as failed and increments retry count
func (tp *TaskPlanner) MarkStepFailed(plan *ExecutionPlan, phaseID, stepID, error string) {
	for pi := range plan.Phases {
		if plan.Phases[pi].ID == phaseID {
			for si := range plan.Phases[pi].Steps {
				if plan.Phases[pi].Steps[si].ID == stepID {
					step := &plan.Phases[pi].Steps[si]
					step.RetryCount++
					if step.RetryCount >= 3 {
						step.Status = "failed"
						step.Result = error
					} else {
						step.Status = "pending" // Retry
					}
					plan.UpdatedAt = time.Now().UnixMilli()
					return
				}
			}
		}
	}
}

// ═══════════════════════════════════════════════════════════════════
// Phase 3: Context Injection - 将当前任务注入到对话中
// 关键改进: 让LLM知道当前应该做什么
// ═══════════════════════════════════════════════════════════════════

// BuildStepContext builds context message for current step
func (tp *TaskPlanner) BuildStepContext(plan *ExecutionPlan, phase *PlanPhase, step *PlanStep) string {
	var sb strings.Builder

	sb.WriteString("📋 当前任务进度:\n")
	sb.WriteString(fmt.Sprintf("- 阶段: %s (%s)\n", phase.Name, phase.Description))
	sb.WriteString(fmt.Sprintf("- 步骤: %s\n", step.Description))
	sb.WriteString(fmt.Sprintf("- 进度: %d/%d 阶段, %d/%d 步骤\n",
		plan.CurrentPhase+1, len(plan.Phases),
		plan.CurrentStep+1, len(phase.Steps)))

	if len(step.Files) > 0 {
		sb.WriteString(fmt.Sprintf("- 涉及文件: %s\n", strings.Join(step.Files, ", ")))
	}
	if len(step.Tools) > 0 {
		sb.WriteString(fmt.Sprintf("- 推荐工具: %s\n", strings.Join(step.Tools, ", ")))
	}

	// Add completed context
	sb.WriteString("\n✅ 已完成的步骤:\n")
	for _, p := range plan.Phases {
		for _, s := range p.Steps {
			if s.Status == "completed" && s.Result != "" {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", s.Description, truncateString(s.Result, 100)))
			}
		}
	}

	sb.WriteString("\n请完成当前步骤，完成后回复 'DONE'。")
	return sb.String()
}

// ═══════════════════════════════════════════════════════════════════
// Phase 4: Plan Persistence - 保存计划到文件
// 参考: OpenHands PLAN.md
// ═══════════════════════════════════════════════════════════════════

// FormatPlanAsMarkdown formats the plan as markdown for saving
func (tp *TaskPlanner) FormatPlanAsMarkdown(plan *ExecutionPlan) string {
	var sb strings.Builder

	sb.WriteString("# 任务执行计划\n\n")
	sb.WriteString(fmt.Sprintf("## 任务\n%s\n\n", plan.Task))
	sb.WriteString(fmt.Sprintf("## 状态: %s\n\n", plan.Status))

	for _, phase := range plan.Phases {
		statusIcon := "⬜"
		switch phase.Status {
		case "in_progress":
			statusIcon = "🔄"
		case "completed":
			statusIcon = "✅"
		case "failed":
			statusIcon = "❌"
		}

		sb.WriteString(fmt.Sprintf("### %s %s\n\n", statusIcon, phase.Name))
		sb.WriteString(fmt.Sprintf("%s\n\n", phase.Description))

		for _, step := range phase.Steps {
			stepIcon := "⬜"
			switch step.Status {
			case "in_progress":
				stepIcon = "🔄"
			case "completed":
				stepIcon = "✅"
			case "failed":
				stepIcon = "❌"
			case "skipped":
				stepIcon = "⏭️"
			}

			sb.WriteString(fmt.Sprintf("- %s %s\n", stepIcon, step.Description))
			if step.Result != "" {
				sb.WriteString(fmt.Sprintf("  - 结果: %s\n", truncateString(step.Result, 80)))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// ═══════════════════════════════════════════════════════════════════
// Helper Functions
// ═══════════════════════════════════════════════════════════════════

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// GetProgress returns the overall progress percentage
func (tp *TaskPlanner) GetProgress(plan *ExecutionPlan) float64 {
	if plan == nil || len(plan.Phases) == 0 {
		return 0
	}

	totalSteps := 0
	completedSteps := 0
	for _, phase := range plan.Phases {
		totalSteps += len(phase.Steps)
		for _, step := range phase.Steps {
			if step.Status == "completed" || step.Status == "skipped" {
				completedSteps++
			}
		}
	}

	if totalSteps == 0 {
		return 0
	}
	return float64(completedSteps) / float64(totalSteps) * 100
}
