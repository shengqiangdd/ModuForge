package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// TaskStatus represents the state of a task.
type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
	StatusFailed     TaskStatus = "failed"
)

// Task represents a single unit of work in the decomposition.
type Task struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Language     string     `json:"language"`
	Dependencies []string   `json:"dependencies,omitempty"`
	Status       TaskStatus `json:"status"`
}

// TaskGraph is a DAG of tasks with dependency information.
type TaskGraph struct {
	Tasks        []Task              `json:"tasks"`
	Dependencies map[string][]string `json:"dependencies"`
}

// PM is the Project Manager agent that decomposes requirements into task graphs.
type PM struct {
	caller llmCaller
}

// NewPM creates a PM with resolved LLM configuration.
func NewPM() *PM {
	endpoint, apiKey, model := resolveAgentLLM()
	if endpoint == "" || apiKey == "" {
		return nil
	}
	return &PM{
		caller: &builderLLMCaller{
			endpoint: endpoint,
			apiKey:   apiKey,
			model:    model,
		},
	}
}

// DecomposeRequirement analyzes a high-level requirement and produces a TaskGraph.
func (pm *PM) DecomposeRequirement(ctx context.Context, requirement string) (TaskGraph, error) {
	if pm == nil || pm.caller == nil {
		return TaskGraph{}, fmt.Errorf("PM not initialized: no LLM configured")
	}

	prompt := fmt.Sprintf(`You are a Project Manager for Android Magisk module development.
Analyze this requirement and decompose it into a task dependency graph.

## Requirement
%s

## Output format (valid JSON only):
{
  "tasks": [
    {
      "id": "T1",
      "name": "task name",
      "description": "what this task does",
      "language": "shell|go|c|prop",
      "dependencies": ["T0"]
    }
  ],
  "dependencies": {
    "T1": ["T0"]
  }
}

## Rules
- Tasks should be granular (one file or one logical unit per task)
- module.prop (T1) is always the first task with no dependencies
- customize.sh depends on module.prop
- service.sh depends on customize.sh
- Go/C source files depend on module.prop and customize.sh
- uninstall.sh depends on module.prop
- build.sh depends on all source files
- Use sequential IDs: T1, T2, T3...
- Output ONLY the JSON task graph, nothing else.`, requirement)

	ctx, cancel := context.WithTimeout(ctx, teamLLMTimeout)
	defer cancel()

	resp, err := pm.caller.CallLLM(ctx, prompt)
	if err != nil {
		return TaskGraph{}, fmt.Errorf("LLM call failed: %w", err)
	}

	resp = extractJSON(resp)

	var graph TaskGraph
	if err := json.Unmarshal([]byte(resp), &graph); err != nil {
		return TaskGraph{}, fmt.Errorf("parse task graph JSON: %w\nresponse: %s", err, truncateStr(resp, 500))
	}

	// Initialize dependencies map if nil
	if graph.Dependencies == nil {
		graph.Dependencies = make(map[string][]string)
	}

	// Set default statuses and sync dependencies
	for i := range graph.Tasks {
		graph.Tasks[i].Status = StatusPending
		if deps, ok := graph.Dependencies[graph.Tasks[i].ID]; ok {
			graph.Tasks[i].Dependencies = deps
		}
	}

	return graph, nil
}

// GetReadyTasks returns tasks whose dependencies are all satisfied.
func (g *TaskGraph) GetReadyTasks() []Task {
	var ready []Task
	for _, t := range g.Tasks {
		if t.Status != StatusPending {
			continue
		}
		deps := g.Dependencies[t.ID]
		allDone := true
		for _, dep := range deps {
			if g.getTaskStatus(dep) != StatusDone {
				allDone = false
				break
			}
		}
		if allDone {
			ready = append(ready, t)
		}
	}
	return ready
}

// MarkDone marks a task as completed.
func (g *TaskGraph) MarkDone(taskID string) {
	for i := range g.Tasks {
		if g.Tasks[i].ID == taskID {
			g.Tasks[i].Status = StatusDone
			return
		}
	}
}

// MarkFailed marks a task as failed.
func (g *TaskGraph) MarkFailed(taskID string) {
	for i := range g.Tasks {
		if g.Tasks[i].ID == taskID {
			g.Tasks[i].Status = StatusFailed
			return
		}
	}
}

// AllDone returns true if all tasks are completed.
func (g *TaskGraph) AllDone() bool {
	for _, t := range g.Tasks {
		if t.Status != StatusDone {
			return false
		}
	}
	return true
}

// getTaskStatus returns the status of a task by ID.
func (g *TaskGraph) getTaskStatus(taskID string) TaskStatus {
	for _, t := range g.Tasks {
		if t.ID == taskID {
			return t.Status
		}
	}
	return StatusPending
}

// TopologicalSort returns tasks in dependency order (Kahn's algorithm).
func (g *TaskGraph) TopologicalSort() ([]Task, error) {
	inDegree := make(map[string]int)
	adj := make(map[string][]string)

	for _, t := range g.Tasks {
		if _, ok := inDegree[t.ID]; !ok {
			inDegree[t.ID] = 0
		}
		for _, dep := range g.Dependencies[t.ID] {
			adj[dep] = append(adj[dep], t.ID)
			inDegree[t.ID]++
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var sorted []Task
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		for _, t := range g.Tasks {
			if t.ID == id {
				sorted = append(sorted, t)
				break
			}
		}

		for _, neighbor := range adj[id] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(sorted) != len(g.Tasks) {
		return nil, fmt.Errorf("cycle detected in task graph")
	}

	return sorted, nil
}

// resolveAgentLLM reads LLM config from environment variables.
func resolveAgentLLM() (endpoint, apiKey, model string) {
	endpoint = os.Getenv("LLM_ENDPOINT")
	apiKey = os.Getenv("LLM_API_KEY")
	model = os.Getenv("LLM_MODEL")

	endpoint = strings.TrimSpace(endpoint)
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)

	if endpoint == "" {
		endpoint = "https://api.commandcode.ai/provider/v1"
	}
	if model == "" {
		model = "poolside/laguna-s-2.1-free"
	}

	return
}
