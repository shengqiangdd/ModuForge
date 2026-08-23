package executor

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestExecute_Linear(t *testing.T) {
	pe := New()
	pe.TaskTimeout = 5 * time.Second

	tasks := []Task{
		&SimpleTask{ID: "T1"},
		&SimpleTask{ID: "T2", Deps: []string{"T1"}},
		&SimpleTask{ID: "T3", Deps: []string{"T2"}},
	}

	var order []string
	executor := func(taskID string) ([]byte, error) {
		order = append(order, taskID)
		return []byte("done"), nil
	}

	results, err := pe.Execute(context.Background(), tasks, executor, 2)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify execution order respects dependencies
	idx := make(map[string]int)
	for i, r := range results {
		idx[r.TaskID] = i
	}

	if idx["T1"] >= idx["T2"] {
		t.Error("T1 should execute before T2")
	}
	if idx["T2"] >= idx["T3"] {
		t.Error("T2 should execute before T3")
	}
}

func TestExecute_Parallel(t *testing.T) {
	pe := New()
	pe.TaskTimeout = 5 * time.Second

	// T1 has no deps, T2 and T3 depend on T1
	tasks := []Task{
		&SimpleTask{ID: "T1"},
		&SimpleTask{ID: "T2", Deps: []string{"T1"}},
		&SimpleTask{ID: "T3", Deps: []string{"T1"}},
	}

	var maxConcurrent int64
	var current int64

	executor := func(taskID string) ([]byte, error) {
		c := atomic.AddInt64(&current, 1)
		// Track max concurrency
		for {
			old := atomic.LoadInt64(&maxConcurrent)
			if c <= old || atomic.CompareAndSwapInt64(&maxConcurrent, old, c) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt64(&current, -1)
		return []byte("done"), nil
	}

	_, err := pe.Execute(context.Background(), tasks, executor, 4)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// T2 and T3 should run concurrently (maxConcurrent >= 2)
	if maxConcurrent < 2 {
		t.Errorf("expected at least 2 concurrent executions, got %d", maxConcurrent)
	}
}

func TestExecute_WithErrors(t *testing.T) {
	pe := New()
	pe.TaskTimeout = 5 * time.Second

	tasks := []Task{
		&SimpleTask{ID: "T1"},
		&SimpleTask{ID: "T2", Deps: []string{"T1"}},
	}

	executor := func(taskID string) ([]byte, error) {
		if taskID == "T2" {
			return nil, fmt.Errorf("build failed")
		}
		return []byte("ok"), nil
	}

	results, err := pe.Execute(context.Background(), tasks, executor, 2)
	// err should be the first error encountered
	if err == nil {
		t.Fatal("expected error from executor")
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// T2 should have error
	for _, r := range results {
		if r.TaskID == "T2" && r.Error == "" {
			t.Error("expected error for T2")
		}
	}
}

func TestExecute_Timeout(t *testing.T) {
	pe := New()
	pe.TaskTimeout = 100 * time.Millisecond

	tasks := []Task{
		&SimpleTask{ID: "slow"},
	}

	executor := func(taskID string) ([]byte, error) {
		time.Sleep(500 * time.Millisecond)
		return []byte("done"), nil
	}

	results, err := pe.Execute(context.Background(), tasks, executor, 1)
	if err != nil {
		t.Logf("Execute returned error (expected): %v", err)
	}

	for _, r := range results {
		if r.TaskID == "slow" && r.Error == "" {
			t.Error("expected timeout error for slow task")
		}
	}
}

func TestTopologicalSort(t *testing.T) {
	tasks := []Task{
		&SimpleTask{ID: "T1"},
		&SimpleTask{ID: "T2", Deps: []string{"T1"}},
		&SimpleTask{ID: "T3", Deps: []string{"T1"}},
		&SimpleTask{ID: "T4", Deps: []string{"T2", "T3"}},
	}

	sorted, err := TopologicalSort(tasks)
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}

	if len(sorted) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(sorted))
	}

	idx := make(map[string]int)
	for i, task := range sorted {
		idx[task.GetID()] = i
	}

	// Verify dependencies
	if idx["T1"] >= idx["T2"] {
		t.Error("T1 before T2")
	}
	if idx["T1"] >= idx["T3"] {
		t.Error("T1 before T3")
	}
	if idx["T2"] >= idx["T4"] {
		t.Error("T2 before T4")
	}
	if idx["T3"] >= idx["T4"] {
		t.Error("T3 before T4")
	}
}

func TestTopologicalSort_Cycle(t *testing.T) {
	tasks := []Task{
		&SimpleTask{ID: "T1", Deps: []string{"T2"}},
		&SimpleTask{ID: "T2", Deps: []string{"T1"}},
	}

	_, err := TopologicalSort(tasks)
	if err == nil {
		t.Fatal("expected error for cycle")
	}
}

func TestSimpleTask(t *testing.T) {
	task := &SimpleTask{ID: "T1", Deps: []string{"T0"}}

	if task.GetID() != "T1" {
		t.Errorf("expected ID T1, got %s", task.GetID())
	}

	deps := task.GetDependencies()
	if len(deps) != 1 || deps[0] != "T0" {
		t.Errorf("expected deps [T0], got %v", deps)
	}
}
