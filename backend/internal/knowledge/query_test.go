package knowledge

import (
	"testing"
)

func TestNewGraphQuery(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())
	gq := NewGraphQuery(kg)
	if gq == nil {
		t.Fatal("expected non-nil query")
	}
}

func TestFindRelated(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())

	kg.AddEntity(Entity{ID: "a", Name: "A", Type: EntityFunction})
	kg.AddEntity(Entity{ID: "b", Name: "B", Type: EntityFunction})
	kg.AddEntity(Entity{ID: "c", Name: "C", Type: EntityFunction})

	kg.AddRelation(Relation{From: "a", To: "b", Type: RelationCalls})
	kg.AddRelation(Relation{From: "b", To: "c", Type: RelationCalls})

	gq := NewGraphQuery(kg)
	results := gq.FindRelated("a", 2)

	if len(results) < 2 {
		t.Errorf("expected at least 2 results, got %d", len(results))
	}

	// Should find b (distance 1) and c (distance 2)
	foundB := false
	foundC := false
	for _, r := range results {
		if r.Entity.ID == "b" && r.Distance == 1 {
			foundB = true
		}
		if r.Entity.ID == "c" && r.Distance == 2 {
			foundC = true
		}
	}

	if !foundB {
		t.Error("expected to find b at distance 1")
	}

	if !foundC {
		t.Error("expected to find c at distance 2")
	}
}

func TestFindRelated_NotFound(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())
	gq := NewGraphQuery(kg)

	results := gq.FindRelated("nonexistent", 3)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFindPath(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())

	kg.AddEntity(Entity{ID: "a", Name: "A", Type: EntityFunction})
	kg.AddEntity(Entity{ID: "b", Name: "B", Type: EntityFunction})
	kg.AddEntity(Entity{ID: "c", Name: "C", Type: EntityFunction})

	kg.AddRelation(Relation{From: "a", To: "b", Type: RelationCalls})
	kg.AddRelation(Relation{From: "b", To: "c", Type: RelationCalls})

	gq := NewGraphQuery(kg)
	path := gq.FindPath("a", "c")

	if path == nil {
		t.Fatal("expected path")
	}

	if len(path) != 3 {
		t.Errorf("expected path length 3, got %d: %v", len(path), path)
	}

	if path[0] != "a" || path[1] != "b" || path[2] != "c" {
		t.Errorf("expected [a b c], got %v", path)
	}
}

func TestFindPath_NoPath(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())

	kg.AddEntity(Entity{ID: "a", Name: "A", Type: EntityFunction})
	kg.AddEntity(Entity{ID: "b", Name: "B", Type: EntityFunction})

	// No relation between a and b
	gq := NewGraphQuery(kg)
	path := gq.FindPath("a", "b")

	if path != nil {
		t.Errorf("expected nil path, got %v", path)
	}
}

func TestFindPath_SameEntity(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())
	kg.AddEntity(Entity{ID: "a", Name: "A", Type: EntityFunction})

	gq := NewGraphQuery(kg)
	path := gq.FindPath("a", "a")

	if path == nil || len(path) != 1 {
		t.Errorf("expected [a], got %v", path)
	}
}

func TestGetEntitiesByType(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())

	kg.AddEntity(Entity{ID: "f1", Name: "F1", Type: EntityFunction})
	kg.AddEntity(Entity{ID: "f2", Name: "F2", Type: EntityFunction})
	kg.AddEntity(Entity{ID: "a1", Name: "A1", Type: EntityAPI})

	gq := NewGraphQuery(kg)
	funcs := gq.GetEntitiesByType(EntityFunction)

	if len(funcs) != 2 {
		t.Errorf("expected 2 functions, got %d", len(funcs))
	}
}

func TestGetRelations(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())

	kg.AddRelation(Relation{From: "a", To: "b", Type: RelationCalls})
	kg.AddRelation(Relation{From: "a", To: "c", Type: RelationUses})
	kg.AddRelation(Relation{From: "b", To: "c", Type: RelationCalls})

	gq := NewGraphQuery(kg)
	rels := gq.GetRelations("a")

	if len(rels) != 2 {
		t.Errorf("expected 2 relations for a, got %d", len(rels))
	}
}

func TestGetRelations_None(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())
	gq := NewGraphQuery(kg)

	rels := gq.GetRelations("nonexistent")
	if len(rels) != 0 {
		t.Errorf("expected 0 relations, got %d", len(rels))
	}
}

func TestRecommendApproach(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())

	kg.AddEntity(Entity{ID: "monitor", Name: "battery_monitor", Type: EntityFunction, Description: "battery monitoring"})
	kg.AddEntity(Entity{ID: "read_file", Name: "os.ReadFile", Type: EntityAPI, Description: "read sysfs"})

	gq := NewGraphQuery(kg)
	recs := gq.RecommendApproach("创建电池监控模块")

	if len(recs) == 0 {
		t.Error("expected at least 1 recommendation")
	}

	// Should have confidence > 0
	for _, r := range recs {
		if r.Confidence <= 0 {
			t.Errorf("expected positive confidence, got %.1f", r.Confidence)
		}
	}
}

func TestRecommendApproach_Root(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())
	gq := NewGraphQuery(kg)

	recs := gq.RecommendApproach("检测Root权限")

	found := false
	for _, r := range recs {
		for _, step := range r.Steps {
			if len(step) > 0 {
				found = true
			}
		}
	}

	if !found {
		t.Error("expected recommendation steps")
	}
}

func TestGraphResult_Fields(t *testing.T) {
	r := GraphResult{
		Entity:   Entity{ID: "test", Name: "Test"},
		Path:     []string{"a", "b", "test"},
		Distance: 2,
	}

	if r.Distance != 2 {
		t.Errorf("expected 2, got %d", r.Distance)
	}

	if len(r.Path) != 3 {
		t.Errorf("expected 3 path elements, got %d", len(r.Path))
	}
}

func TestRecommendation_Fields(t *testing.T) {
	r := Recommendation{
		Steps:      []string{"step1", "step2"},
		Reason:     "because",
		Confidence: 0.85,
	}

	if r.Confidence != 0.85 {
		t.Errorf("expected 0.85, got %.2f", r.Confidence)
	}

	if len(r.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(r.Steps))
	}
}

func TestFindRelated_MaxDepth(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())

	kg.AddEntity(Entity{ID: "a", Name: "A", Type: EntityFunction})
	kg.AddEntity(Entity{ID: "b", Name: "B", Type: EntityFunction})
	kg.AddEntity(Entity{ID: "c", Name: "C", Type: EntityFunction})

	kg.AddRelation(Relation{From: "a", To: "b", Type: RelationCalls})
	kg.AddRelation(Relation{From: "b", To: "c", Type: RelationCalls})

	gq := NewGraphQuery(kg)

	// Depth 1 should only find a and b
	results := gq.FindRelated("a", 1)
	if len(results) > 2 {
		t.Errorf("expected at most 2 results with depth 1, got %d", len(results))
	}
}
