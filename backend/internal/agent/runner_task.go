package agent

import (
	"database/sql"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════
// P2-1: TaskDecomposer — Break complex tasks into subtasks
// ═══════════════════════════════════════════════════════════════════

// TaskDecomposer analyzes a task and breaks it into manageable subtasks.
type TaskDecomposer struct {
	db *sql.DB
}

// Subtask represents a piece of a larger task.
type Subtask struct {
	ID           string   `json:"id"`
	Description  string   `json:"description"`
	Status       string   `json:"status"` // pending, in_progress, completed, failed
	Dependencies []string `json:"dependencies"`
	Files        []string `json:"files,omitempty"`    // involved files
	Progress     int      `json:"progress,omitempty"` // 0-100
	StartedAt    int64    `json:"started_at,omitempty"`
	CompletedAt  int64    `json:"completed_at,omitempty"`
	RetryCount   int      `json:"retry_count,omitempty"`
}

// isComplexTask determines whether a task warrants LLM decomposition.
// Simple tasks (single action, short) are handled by keyword fallback.
func isComplexTask(task string) bool {
	taskLower := strings.ToLower(task)
	// Short tasks are not complex
	if len(task) < 15 {
		return false
	}
	// Tasks with multiple action verbs are complex
	actionCount := 0
	for _, kw := range []string{"and", "then", "also", "additionally", "同时", "并且", "然后", "接着",
		"create", "implement", "fix", "refactor", "add", "build", "optimize", "migrate",
		"实现", "创建", "修复", "重构", "优化", "迁移", "添加", "构建"} {
		if strings.Contains(taskLower, kw) {
			actionCount++
		}
	}
	if actionCount >= 2 {
		return true
	}
	// Tasks with enumeration markers (、) suggesting multiple items
	if strings.Count(task, "\u3001") >= 2 {
		return true
	}
	// Tasks mentioning "包含" (containing) with multiple features
	if strings.Contains(taskLower, "\u5305\u542b") || strings.Contains(taskLower, "include") {
		return true
	}
	// Long tasks are likely complex
	if len(task) > 80 {
		return true
	}
	return false
}

// DecomposeTask breaks a complex task into subtasks.
// Optimization 50: Enhanced task decomposition with more patterns and dependency tracking
func (td *TaskDecomposer) DecomposeTask(task string, projectContext string) []Subtask {
	subtasks := make([]Subtask, 0)

	// Simple heuristic: detect common patterns
	taskLower := strings.ToLower(task)

	// Pattern: "create X" -> analyze, implement, test
	if strings.Contains(taskLower, "create") || strings.Contains(taskLower, "implement") || strings.Contains(taskLower, "add") {
		subtasks = append(subtasks, Subtask{
			ID:          "analyze",
			Description: "分析需求和现有代码结构",
			Status:      "pending",
		})
		subtasks = append(subtasks, Subtask{
			ID:           "implement",
			Description:  "实现功能代码",
			Status:       "pending",
			Dependencies: []string{"analyze"},
		})
		subtasks = append(subtasks, Subtask{
			ID:           "verify",
			Description:  "验证编译和功能",
			Status:       "pending",
			Dependencies: []string{"implement"},
		})
	}

	// Pattern: "fix X" -> diagnose, fix, test
	if strings.Contains(taskLower, "fix") || strings.Contains(taskLower, "repair") || strings.Contains(taskLower, "debug") {
		subtasks = append(subtasks, Subtask{
			ID:          "diagnose",
			Description: "诊断问题原因",
			Status:      "pending",
		})
		subtasks = append(subtasks, Subtask{
			ID:           "fix",
			Description:  "修复问题",
			Status:       "pending",
			Dependencies: []string{"diagnose"},
		})
		subtasks = append(subtasks, Subtask{
			ID:           "verify",
			Description:  "验证修复",
			Status:       "pending",
			Dependencies: []string{"fix"},
		})
	}

	// Pattern: "refactor X" -> analyze, plan, execute, verify
	if strings.Contains(taskLower, "refactor") || strings.Contains(taskLower, "optimize") || strings.Contains(taskLower, "improve") {
		subtasks = append(subtasks, Subtask{
			ID:          "analyze",
			Description: "分析当前代码",
			Status:      "pending",
		})
		subtasks = append(subtasks, Subtask{
			ID:           "plan",
			Description:  "制定重构计划",
			Status:       "pending",
			Dependencies: []string{"analyze"},
		})
		subtasks = append(subtasks, Subtask{
			ID:           "execute",
			Description:  "执行重构",
			Status:       "pending",
			Dependencies: []string{"plan"},
		})
		subtasks = append(subtasks, Subtask{
			ID:           "verify",
			Description:  "验证重构结果",
			Status:       "pending",
			Dependencies: []string{"execute"},
		})
	}

	// Optimization 50: New patterns for common tasks
	// Pattern: "test X" -> analyze, write tests, run tests
	if strings.Contains(taskLower, "test") || strings.Contains(taskLower, "测试") {
		subtasks = append(subtasks, Subtask{
			ID:          "analyze",
			Description: "分析需要测试的代码",
			Status:      "pending",
		})
		subtasks = append(subtasks, Subtask{
			ID:           "write_tests",
			Description:  "编写测试用例",
			Status:       "pending",
			Dependencies: []string{"analyze"},
		})
		subtasks = append(subtasks, Subtask{
			ID:           "run_tests",
			Description:  "运行测试并验证",
			Status:       "pending",
			Dependencies: []string{"write_tests"},
		})
	}

	// Pattern: "document X" -> analyze, write docs, review
	if strings.Contains(taskLower, "document") || strings.Contains(taskLower, "文档") || strings.Contains(taskLower, "readme") {
		subtasks = append(subtasks, Subtask{
			ID:          "analyze",
			Description: "分析代码结构和功能",
			Status:      "pending",
		})
		subtasks = append(subtasks, Subtask{
			ID:           "write_docs",
			Description:  "编写文档",
			Status:       "pending",
			Dependencies: []string{"analyze"},
		})
		subtasks = append(subtasks, Subtask{
			ID:           "review",
			Description:  "审查文档质量",
			Status:       "pending",
			Dependencies: []string{"write_docs"},
		})
	}

	// Pattern: "migrate X" -> analyze, plan, execute, verify
	if strings.Contains(taskLower, "migrate") || strings.Contains(taskLower, "迁移") || strings.Contains(taskLower, "升级") {
		subtasks = append(subtasks, Subtask{
			ID:          "analyze",
			Description: "分析迁移需求和影响",
			Status:      "pending",
		})
		subtasks = append(subtasks, Subtask{
			ID:           "plan",
			Description:  "制定迁移计划",
			Status:       "pending",
			Dependencies: []string{"analyze"},
		})
		subtasks = append(subtasks, Subtask{
			ID:           "execute",
			Description:  "执行迁移",
			Status:       "pending",
			Dependencies: []string{"plan"},
		})
		subtasks = append(subtasks, Subtask{
			ID:           "verify",
			Description:  "验证迁移结果",
			Status:       "pending",
			Dependencies: []string{"execute"},
		})
	}

	// If no pattern matched, treat as single task
	if len(subtasks) == 0 {
		subtasks = append(subtasks, Subtask{
			ID:          "complete",
			Description: task,
			Status:      "pending",
		})
	}

	return subtasks
}

// GetNextSubtask returns the next subtask to execute based on dependencies.
func (td *TaskDecomposer) GetNextSubtask(subtasks []Subtask) *Subtask {
	completed := make(map[string]bool)
	for _, st := range subtasks {
		if st.Status == "completed" {
			completed[st.ID] = true
		}
	}

	for i := range subtasks {
		if subtasks[i].Status != "pending" {
			continue
		}
		// Check if all dependencies are completed
		allDepsMet := true
		for _, dep := range subtasks[i].Dependencies {
			if !completed[dep] {
				allDepsMet = false
				break
			}
		}
		if allDepsMet {
			return &subtasks[i]
		}
	}

	return nil // all done or blocked
}
