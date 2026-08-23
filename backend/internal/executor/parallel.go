package executor

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const DefaultTaskTimeout = 60 * time.Second

// TaskResult holds the outcome of a single task execution.
type TaskResult struct {
	TaskID   string        `json:"task_id"`
	Output   []byte        `json:"output,omitempty"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
}

// Task is the minimal interface for executable tasks.
type Task interface {
	GetID() string
	GetDependencies() []string
}

// ParallelExecutor runs tasks concurrently with dependency ordering.
type ParallelExecutor struct {
	MaxConcurrency int
	TaskTimeout    time.Duration
}

// New creates a ParallelExecutor with default settings.
func New() *ParallelExecutor {
	return &ParallelExecutor{
		MaxConcurrency: 4,
		TaskTimeout:    DefaultTaskTimeout,
	}
}

// Execute runs all tasks respecting dependencies, with bounded concurrency.
// The executor function is called for each task; it receives the task ID and
// should return the output or an error.
func (pe *ParallelExecutor) Execute(
	ctx context.Context,
	tasks []Task,
	executor func(taskID string) ([]byte, error),
	maxConcurrency int,
) ([]TaskResult, error) {
	if maxConcurrency <= 0 {
		maxConcurrency = pe.MaxConcurrency
	}
	if pe.TaskTimeout <= 0 {
		pe.TaskTimeout = DefaultTaskTimeout
	}

	// Build dependency graph
	inDegree := make(map[string]int)
	adj := make(map[string][]string)
	taskMap := make(map[string]Task)

	for _, t := range tasks {
		id := t.GetID()
		taskMap[id] = t
		if _, ok := inDegree[id]; !ok {
			inDegree[id] = 0
		}
		for _, dep := range t.GetDependencies() {
			adj[dep] = append(adj[dep], id)
			inDegree[id]++
		}
	}

	// Track results
	results := make([]TaskResult, len(tasks))
	resultIdx := make(map[string]int)
	for i, t := range tasks {
		resultIdx[t.GetID()] = i
	}

	// Execute in waves (BFS topological)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrency)

	completed := make(map[string]bool)
	totalExecuted := 0
	var execErr error

	for totalExecuted < len(tasks) {
		// Find ready tasks
		var ready []string
		for id, deg := range inDegree {
			if deg == 0 && !completed[id] {
				ready = append(ready, id)
			}
		}

		if len(ready) == 0 && totalExecuted < len(tasks) {
			return results, fmt.Errorf("deadlock: no ready tasks, %d/%d completed", totalExecuted, len(tasks))
		}

		// Execute ready tasks concurrently
		var batchWg sync.WaitGroup
		for _, id := range ready {
			mu.Lock()
			if completed[id] {
				mu.Unlock()
				continue
			}
			completed[id] = true
			totalExecuted++
			mu.Unlock()

			batchWg.Add(1)
			wg.Add(1)
			sem <- struct{}{}

			go func(taskID string) {
				defer batchWg.Done()
				defer wg.Done()
				defer func() { <-sem }()

				// Execute with timeout
				taskCtx, cancel := context.WithTimeout(ctx, pe.TaskTimeout)
				defer cancel()

				type execResult struct {
					output []byte
					err    error
				}
				ch := make(chan execResult, 1)

				go func() {
					out, err := executor(taskID)
					ch <- execResult{out, err}
				}()

				select {
				case <-taskCtx.Done():
					mu.Lock()
					results[resultIdx[taskID]] = TaskResult{
						TaskID:   taskID,
						Error:    fmt.Sprintf("task timed out after %s", pe.TaskTimeout),
						Duration: pe.TaskTimeout,
					}
					mu.Unlock()
				case res := <-ch:
					dur := time.Duration(0) // approximate
					mu.Lock()
					if res.err != nil {
						results[resultIdx[taskID]] = TaskResult{
							TaskID:   taskID,
							Error:    res.err.Error(),
							Duration: dur,
						}
						if execErr == nil {
							execErr = res.err
						}
					} else {
						results[resultIdx[taskID]] = TaskResult{
							TaskID:   taskID,
							Output:   res.output,
							Duration: dur,
						}
					}
					mu.Unlock()
				}

				// Update in-degree for dependents
				mu.Lock()
				for _, neighbor := range adj[taskID] {
					inDegree[neighbor]--
				}
				mu.Unlock()
			}(id)
		}

		batchWg.Wait()
	}

	wg.Wait()

	return results, execErr
}

// TopologicalSort orders tasks by dependency (Kahn's algorithm).
func TopologicalSort(tasks []Task) ([]Task, error) {
	inDegree := make(map[string]int)
	adj := make(map[string][]string)
	taskMap := make(map[string]Task)

	for _, t := range tasks {
		id := t.GetID()
		taskMap[id] = t
		if _, ok := inDegree[id]; !ok {
			inDegree[id] = 0
		}
		for _, dep := range t.GetDependencies() {
			adj[dep] = append(adj[dep], id)
			inDegree[id]++
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
		sorted = append(sorted, taskMap[id])

		for _, neighbor := range adj[id] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(sorted) != len(tasks) {
		return nil, fmt.Errorf("cycle detected in task dependencies")
	}

	return sorted, nil
}

// SimpleTask is a concrete implementation of the Task interface.
type SimpleTask struct {
	ID   string   `json:"id"`
	Deps []string `json:"dependencies,omitempty"`
}

func (t *SimpleTask) GetID() string             { return t.ID }
func (t *SimpleTask) GetDependencies() []string { return t.Deps }
