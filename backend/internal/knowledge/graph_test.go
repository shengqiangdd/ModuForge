package knowledge

import (
	"testing"
)

func TestNewKnowledgeGraph(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())
	if kg == nil {
		t.Fatal("expected non-nil graph")
	}
}

func TestAddEntity(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())

	entity := Entity{
		ID:          "func_main",
		Name:        "main",
		Type:        EntityFunction,
		Description: "entry point",
		Source:      "main.go",
	}

	if err := kg.AddEntity(entity); err != nil {
		t.Fatalf("AddEntity failed: %v", err)
	}

	if kg.EntityCount() != 1 {
		t.Errorf("expected 1 entity, got %d", kg.EntityCount())
	}
}

func TestAddRelation(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())

	kg.AddEntity(Entity{ID: "a", Name: "A", Type: EntityFunction})
	kg.AddEntity(Entity{ID: "b", Name: "B", Type: EntityFunction})

	rel := Relation{From: "a", To: "b", Type: RelationCalls}
	if err := kg.AddRelation(rel); err != nil {
		t.Fatalf("AddRelation failed: %v", err)
	}

	if kg.RelationCount() != 1 {
		t.Errorf("expected 1 relation, got %d", kg.RelationCount())
	}
}

func TestExtractFromCode_Go(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())

	goCode := `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("hello")
	os.ReadFile("test.txt")
}

func helper() {
	main()
}
`

	entities, relations := kg.ExtractFromCode("main.go", goCode)

	if len(entities) == 0 {
		t.Fatal("expected at least 1 entity")
	}

	// Should find functions
	hasFunc := false
	for _, e := range entities {
		if e.Type == EntityFunction {
			hasFunc = true
			break
		}
	}
	if !hasFunc {
		t.Error("expected function entity")
	}

	// Should find imports
	hasImport := false
	for _, e := range entities {
		if e.Type == EntityModule {
			hasImport = true
			break
		}
	}
	if !hasImport {
		t.Error("expected import entity")
	}

	// Should find relations
	if len(relations) == 0 {
		t.Error("expected at least 1 relation")
	}
}

func TestExtractFromCode_Shell(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())

	shellCode := `#!/system/bin/sh

MODPATH=${0%/*}

install_module() {
  set_perm ${MODPATH}/bin/test 0 0 0755
  ui_print "Installing..."
}

main() {
  install_module
}
`

	entities, relations := kg.ExtractFromCode("customize.sh", shellCode)

	if len(entities) == 0 {
		t.Fatal("expected at least 1 entity")
	}

	// Should find function
	hasFunc := false
	for _, e := range entities {
		if e.Type == EntityFunction {
			hasFunc = true
			break
		}
	}
	if !hasFunc {
		t.Error("expected function entity")
	}

	// Should find API calls
	hasAPI := false
	for _, e := range entities {
		if e.Type == EntityAPI {
			hasAPI = true
			break
		}
	}
	if !hasAPI {
		t.Error("expected API entity")
	}

	// Should find relations
	if len(relations) == 0 {
		t.Error("expected at least 1 relation")
	}
}

func TestGetAllEntities(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())

	kg.AddEntity(Entity{ID: "a", Name: "A", Type: EntityFunction})
	kg.AddEntity(Entity{ID: "b", Name: "B", Type: EntityAPI})

	all := kg.GetAllEntities()
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
}

func TestGetAllRelations(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())

	kg.AddRelation(Relation{From: "a", To: "b", Type: RelationCalls})

	all := kg.GetAllRelations()
	if len(all) != 1 {
		t.Errorf("expected 1, got %d", len(all))
	}
}

func TestEntityCount(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())

	kg.AddEntity(Entity{ID: "a", Name: "A", Type: EntityFunction})

	if kg.EntityCount() != 1 {
		t.Errorf("expected 1, got %d", kg.EntityCount())
	}
}

func TestRelationCount(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())

	kg.AddRelation(Relation{From: "a", To: "b", Type: RelationCalls})

	if kg.RelationCount() != 1 {
		t.Errorf("expected 1, got %d", kg.RelationCount())
	}
}

func TestEntity_Fields(t *testing.T) {
	e := Entity{
		ID:          "test",
		Name:        "Test Entity",
		Type:        EntityConfig,
		Description: "test config",
		Source:      "config.json",
	}

	if e.ID != "test" {
		t.Errorf("expected test, got %s", e.ID)
	}

	if e.Type != EntityConfig {
		t.Errorf("expected config, got %s", e.Type)
	}
}

func TestRelation_Fields(t *testing.T) {
	r := Relation{
		From:   "a",
		To:     "b",
		Type:   RelationDependsOn,
		Weight: 0.8,
	}

	if r.From != "a" {
		t.Errorf("expected a, got %s", r.From)
	}

	if r.Weight != 0.8 {
		t.Errorf("expected 0.8, got %.1f", r.Weight)
	}
}

func TestEntityID(t *testing.T) {
	id := EntityID("My Function")
	if id != "my_function" {
		t.Errorf("expected my_function, got %s", id)
	}
}

func TestExtractFromCode_Empty(t *testing.T) {
	kg := NewKnowledgeGraph(t.TempDir())

	entities, relations := kg.ExtractFromCode("test.go", "")
	if len(entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(entities))
	}
	if len(relations) != 0 {
		t.Errorf("expected 0 relations, got %d", len(relations))
	}
}
