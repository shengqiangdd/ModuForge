package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moduforge/backend/internal/agent/registry"
)

// TaskDecomposerSkill breaks down complex requirements into manageable subtasks
type TaskDecomposerSkill struct {
	db *sql.DB
}

func init() {
	registry.RegisterFactory("task_decomposer", func(deps *registry.Deps) registry.Skill {
		return &TaskDecomposerSkill{db: deps.DB}
	})
}

func (s *TaskDecomposerSkill) Name() string {
	return "task_decomposer"
}

func (s *TaskDecomposerSkill) Description() string {
	return `Decompose complex requirements into manageable subtasks. Input: {"requirement": "...", "context": "...", "max_tasks": 10}`
}

type DecompositionTask struct {
	ID          int      `json:"id"`
	Task        string   `json:"task"`
	Description string   `json:"description"`
	Depends     []int    `json:"depends"`
	Files       []string `json:"files"`
	Priority    string   `json:"priority"`  // P0, P1, P2
	Estimated   string   `json:"estimated"` // e.g., "5 min", "30 min"
}

type DecompositionResult struct {
	Requirement string              `json:"requirement"`
	Tasks       []DecompositionTask `json:"tasks"`
	TotalTasks  int                 `json:"total_tasks"`
	Estimated   string              `json:"estimated_total"`
	Complexity  string              `json:"complexity"` // simple, medium, complex
}

func (s *TaskDecomposerSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	requirement, _ := input["requirement"].(string)
	context, _ := input["context"].(string)
	maxTasks := 10
	if m, ok := input["max_tasks"].(float64); ok {
		maxTasks = int(m)
	}

	if requirement == "" {
		return "", fmt.Errorf("requirement is required")
	}

	// Use heuristics to decompose the requirement
	result := s.decomposeWithHeuristics(requirement, context, maxTasks)

	// Format output
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(output), nil
}

func (s *TaskDecomposerSkill) decomposeWithHeuristics(requirement, context string, maxTasks int) *DecompositionResult {
	// Simple heuristic-based decomposition
	// Analyze the requirement to determine complexity
	complexity := "simple"

	// Check for keywords that indicate complexity
	complexKeywords := []string{"complex", "multiple", "integrate", "system", "architecture", "distributed"}
	for _, keyword := range complexKeywords {
		if strings.Contains(strings.ToLower(requirement), keyword) {
			complexity = "complex"
			break
		}
	}

	mediumKeywords := []string{"add", "implement", "create", "modify", "update", "enhance"}
	for _, keyword := range mediumKeywords {
		if strings.Contains(strings.ToLower(requirement), keyword) {
			complexity = "medium"
			break
		}
	}

	// Create tasks based on complexity
	var tasks []DecompositionTask

	// Task 1: Always analyze requirements
	tasks = append(tasks, DecompositionTask{
		ID:          1,
		Task:        "Analyze requirements",
		Description: "Understand the requirement and identify key components",
		Depends:     []int{},
		Files:       []string{},
		Priority:    "P0",
		Estimated:   "5 min",
	})

	// Task 2: Design/architecture (for medium/complex)
	if complexity != "simple" {
		tasks = append(tasks, DecompositionTask{
			ID:          2,
			Task:        "Design solution",
			Description: "Create high-level design and identify components",
			Depends:     []int{1},
			Files:       []string{},
			Priority:    "P0",
			Estimated:   "10 min",
		})
	}

	// Task 3: Implement core logic
	tasks = append(tasks, DecompositionTask{
		ID:          len(tasks) + 1,
		Task:        "Implement core logic",
		Description: "Write the main implementation code",
		Depends:     []int{len(tasks)},
		Files:       []string{},
		Priority:    "P0",
		Estimated:   "20 min",
	})

	// Task 4: Add error handling (for medium/complex)
	if complexity != "simple" {
		tasks = append(tasks, DecompositionTask{
			ID:          len(tasks) + 1,
			Task:        "Add error handling",
			Description: "Implement proper error handling and edge cases",
			Depends:     []int{len(tasks)},
			Files:       []string{},
			Priority:    "P1",
			Estimated:   "10 min",
		})
	}

	// Task 5: Write tests
	tasks = append(tasks, DecompositionTask{
		ID:          len(tasks) + 1,
		Task:        "Write tests",
		Description: "Create unit tests and integration tests",
		Depends:     []int{len(tasks)},
		Files:       []string{},
		Priority:    "P1",
		Estimated:   "15 min",
	})

	// Task 6: Documentation (for complex)
	if complexity == "complex" {
		tasks = append(tasks, DecompositionTask{
			ID:          len(tasks) + 1,
			Task:        "Write documentation",
			Description: "Create API documentation and usage examples",
			Depends:     []int{len(tasks)},
			Files:       []string{},
			Priority:    "P2",
			Estimated:   "10 min",
		})
	}

	// Calculate total estimated time
	totalMinutes := 0
	for _, task := range tasks {
		if strings.Contains(task.Estimated, "min") {
			var mins int
			fmt.Sscanf(task.Estimated, "%d min", &mins)
			totalMinutes += mins
		}
	}

	estimatedTotal := fmt.Sprintf("%d min", totalMinutes)
	if totalMinutes >= 60 {
		estimatedTotal = fmt.Sprintf("%.1f hour", float64(totalMinutes)/60)
	}

	return &DecompositionResult{
		Requirement: requirement,
		Tasks:       tasks,
		TotalTasks:  len(tasks),
		Estimated:   estimatedTotal,
		Complexity:  complexity,
	}
}

func (s *TaskDecomposerSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  true,
		Essential: false,
		NeedsDB:   false,
		NeedsLLM:  true,
	}
}
