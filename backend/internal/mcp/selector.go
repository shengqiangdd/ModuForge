package mcp

import (
	"strings"
)

// PipelineStep describes a single step in a tool pipeline.
type PipelineStep struct {
	ToolName      string   `json:"tool_name"`
	InputMapping  string   `json:"input_mapping,omitempty"`
	OutputMapping string   `json:"output_mapping,omitempty"`
	DependsOn     []string `json:"depends_on,omitempty"`
}

// Pipeline is an ordered sequence of tool steps.
type Pipeline struct {
	Steps []PipelineStep `json:"steps"`
}

// ToolSelector picks the best tool for a given task.
type ToolSelector struct{}

// NewToolSelector creates a new selector.
func NewToolSelector() *ToolSelector {
	return &ToolSelector{}
}

// SelectTool picks the most suitable tool for a task description.
func (ts *ToolSelector) SelectTool(task string, availableTools []ToolDefinition) ToolDefinition {
	if len(availableTools) == 0 {
		return ToolDefinition{}
	}

	taskLower := strings.ToLower(task)
	bestScore := -1.0
	bestTool := availableTools[0]

	for _, tool := range availableTools {
		score := ts.scoreTool(taskLower, tool)
		if score > bestScore {
			bestScore = score
			bestTool = tool
		}
	}

	return bestTool
}

// BuildPipeline creates an ordered pipeline from a list of tasks.
func (ts *ToolSelector) BuildPipeline(tasks []string, tools []ToolDefinition) Pipeline {
	var steps []PipelineStep
	toolMap := make(map[string]ToolDefinition)
	for _, t := range tools {
		toolMap[strings.ToLower(t.Name)] = t
	}

	for i, task := range tasks {
		selected := ts.SelectTool(task, tools)

		step := PipelineStep{
			ToolName:      selected.Name,
			InputMapping:  "step_input",
			OutputMapping: "step_output",
		}

		if i > 0 {
			step.DependsOn = []string{steps[i-1].ToolName}
		}

		steps = append(steps, step)
	}

	return Pipeline{Steps: steps}
}

// scoreTool calculates how well a tool matches a task.
func (ts *ToolSelector) scoreTool(task string, tool ToolDefinition) float64 {
	score := 0.0
	toolText := strings.ToLower(tool.Name + " " + tool.Description)

	// Keyword matching
	taskWords := strings.Fields(task)
	for _, word := range taskWords {
		if len(word) < 3 {
			continue
		}
		if strings.Contains(toolText, word) {
			score += 2.0
		}
	}

	// Exact name match bonus
	if strings.Contains(task, strings.ToLower(tool.Name)) {
		score += 5.0
	}

	// Parameter count penalty (prefer simpler tools)
	score -= float64(len(tool.Parameters)) * 0.1

	return score
}
