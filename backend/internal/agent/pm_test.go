package agent

import (
	"context"
	"testing"
)

func TestNewPM_NoLLM(t *testing.T) {
	pm := NewPM()
	if pm != nil {
		t.Log("LLM configured, skipping no-LLM test")
		return
	}
}

func TestDecomposeRequirement_NilPM(t *testing.T) {
	var pm *PM
	_, err := pm.DecomposeRequirement(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for nil PM")
	}
}

func TestTaskGraph_GetReadyTasks(t *testing.T) {
	graph := TaskGraph{
		Tasks: []Task{
			{ID: "T1", Name: "module.prop", Status: StatusPending},
			{ID: "T2", Name: "customize.sh", Status: StatusPending},
			{ID: "T3", Name: "main.go", Status: StatusPending},
		},
		Dependencies: map[string][]string{
			"T2": {"T1"},
			"T3": {"T1", "T2"},
		},
	}

	// Initially only T1 should be ready (no dependencies)
	ready := graph.GetReadyTasks()
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task, got %d", len(ready))
	}
	if ready[0].ID != "T1" {
		t.Errorf("expected T1 ready, got %s", ready[0].ID)
	}

	// Mark T1 done — T2 should become ready
	graph.MarkDone("T1")
	ready = graph.GetReadyTasks()
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task after T1 done, got %d", len(ready))
	}
	if ready[0].ID != "T2" {
		t.Errorf("expected T2 ready, got %s", ready[0].ID)
	}

	// Mark T2 done — T3 should become ready
	graph.MarkDone("T2")
	ready = graph.GetReadyTasks()
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task after T2 done, got %d", len(ready))
	}
	if ready[0].ID != "T3" {
		t.Errorf("expected T3 ready, got %s", ready[0].ID)
	}
}

func TestTaskGraph_AllDone(t *testing.T) {
	graph := TaskGraph{
		Tasks: []Task{
			{ID: "T1", Status: StatusDone},
			{ID: "T2", Status: StatusDone},
		},
		Dependencies: map[string][]string{},
	}

	if !graph.AllDone() {
		t.Error("expected all done")
	}

	graph.Tasks = append(graph.Tasks, Task{ID: "T3", Status: StatusPending})
	if graph.AllDone() {
		t.Error("expected not all done")
	}
}

func TestTaskGraph_MarkFailed(t *testing.T) {
	graph := TaskGraph{
		Tasks: []Task{
			{ID: "T1", Status: StatusPending},
		},
		Dependencies: map[string][]string{},
	}

	graph.MarkFailed("T1")
	if graph.Tasks[0].Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %s", graph.Tasks[0].Status)
	}
}

func TestTopologicalSort_Linear(t *testing.T) {
	graph := TaskGraph{
		Tasks: []Task{
			{ID: "T1", Name: "prop"},
			{ID: "T2", Name: "shell"},
			{ID: "T3", Name: "go"},
		},
		Dependencies: map[string][]string{
			"T2": {"T1"},
			"T3": {"T2"},
		},
	}

	sorted, err := graph.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}

	if len(sorted) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(sorted))
	}

	// T1 must come before T2, T2 before T3
	idx := make(map[string]int)
	for i, task := range sorted {
		idx[task.ID] = i
	}
	if idx["T1"] >= idx["T2"] {
		t.Error("T1 should come before T2")
	}
	if idx["T2"] >= idx["T3"] {
		t.Error("T2 should come before T3")
	}
}

func TestTopologicalSort_Parallel(t *testing.T) {
	graph := TaskGraph{
		Tasks: []Task{
			{ID: "T1", Name: "prop"},
			{ID: "T2", Name: "shell1"},
			{ID: "T3", Name: "shell2"},
		},
		Dependencies: map[string][]string{
			"T2": {"T1"},
			"T3": {"T1"},
		},
	}

	sorted, err := graph.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}

	if len(sorted) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(sorted))
	}

	// T1 must be first
	if sorted[0].ID != "T1" {
		t.Errorf("expected T1 first, got %s", sorted[0].ID)
	}
}

func TestTopologicalSort_Cycle(t *testing.T) {
	graph := TaskGraph{
		Tasks: []Task{
			{ID: "T1"},
			{ID: "T2"},
		},
		Dependencies: map[string][]string{
			"T1": {"T2"},
			"T2": {"T1"},
		},
	}

	_, err := graph.TopologicalSort()
	if err == nil {
		t.Fatal("expected error for cycle")
	}
}

func TestTaskGraph_GetReadyTasks_Empty(t *testing.T) {
	graph := TaskGraph{
		Tasks:        []Task{},
		Dependencies: map[string][]string{},
	}

	ready := graph.GetReadyTasks()
	if len(ready) != 0 {
		t.Errorf("expected 0 ready tasks, got %d", len(ready))
	}
}
