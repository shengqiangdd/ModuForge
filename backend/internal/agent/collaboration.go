package agent

import (
	"fmt"
	"sync"
)

// CollaborativeAgent manages multi-agent collaboration.
type CollaborativeAgent struct {
	mu        sync.Mutex
	agents    map[string]*AgentRunner
	taskQueue chan CollaborativeTask
	results   map[string]CollaborativeTaskResult
}

// CollaborativeTask represents a task to be assigned to an agent.
type CollaborativeTask struct {
	ID          string
	Description string
	AssignedTo  string
	Status      string
}

// CollaborativeTaskResult represents the result of a task.
type CollaborativeTaskResult struct {
	TaskID  string
	Success bool
	Output  string
	Errors  []string
}

// NewCollaborativeAgent creates a new collaborative agent.
func NewCollaborativeAgent() *CollaborativeAgent {
	return &CollaborativeAgent{
		agents:    make(map[string]*AgentRunner),
		taskQueue: make(chan CollaborativeTask, 100),
		results:   make(map[string]CollaborativeTaskResult),
	}
}

// RegisterAgent registers an agent for collaboration.
func (ca *CollaborativeAgent) RegisterAgent(name string, runner *AgentRunner) {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	ca.agents[name] = runner
}

// AssignTask assigns a task to an agent.
func (ca *CollaborativeAgent) AssignTask(task CollaborativeTask) error {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	if _, ok := ca.agents[task.AssignedTo]; !ok {
		return fmt.Errorf("agent %s not found", task.AssignedTo)
	}

	task.Status = "assigned"
	ca.taskQueue <- task
	return nil
}

// GetResult returns the result of a task.
func (ca *CollaborativeAgent) GetResult(taskID string) (CollaborativeTaskResult, bool) {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	result, ok := ca.results[taskID]
	return result, ok
}

// ExecuteTask executes a task using the assigned agent.
func (ca *CollaborativeAgent) ExecuteTask(task CollaborativeTask) CollaborativeTaskResult {
	ca.mu.Lock()
	runner, ok := ca.agents[task.AssignedTo]
	ca.mu.Unlock()

	if !ok {
		return CollaborativeTaskResult{
			TaskID:  task.ID,
			Success: false,
			Errors:  []string{"agent not found"},
		}
	}

	// Execute task using the agent
	// This is a simplified version - in production, you'd use the runner's Run method
	result := CollaborativeTaskResult{
		TaskID:  task.ID,
		Success: true,
		Output:  fmt.Sprintf("Task %s executed by %s", task.ID, task.AssignedTo),
	}

	ca.mu.Lock()
	ca.results[task.ID] = result
	ca.mu.Unlock()

	_ = runner // reserved for future runner integration
	return result
}

