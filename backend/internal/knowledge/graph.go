package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// EntityType represents the type of an entity.
type EntityType string

const (
	EntityAPI      EntityType = "api"
	EntityFunction EntityType = "function"
	EntityConfig   EntityType = "config"
	EntityProperty EntityType = "property"
	EntityModule   EntityType = "module"
)

// RelationType represents the type of a relation.
type RelationType string

const (
	RelationCalls     RelationType = "calls"
	RelationDependsOn RelationType = "depends_on"
	RelationConflicts RelationType = "conflicts_with"
	RelationUses      RelationType = "uses"
)

// Entity represents a code entity (function, API, config, etc).
type Entity struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        EntityType `json:"type"`
	Description string     `json:"description,omitempty"`
	Source      string     `json:"source,omitempty"`
}

// Relation represents a relationship between two entities.
type Relation struct {
	From   string       `json:"from"`
	To     string       `json:"to"`
	Type   RelationType `json:"type"`
	Weight float64      `json:"weight"`
}

// KnowledgeGraph stores entities and their relationships.
type KnowledgeGraph struct {
	mu        sync.RWMutex
	dir       string
	entities  map[string]Entity
	relations []Relation
}

// NewKnowledgeGraph creates a graph backed by JSON in dataDir.
func NewKnowledgeGraph(dataDir string) *KnowledgeGraph {
	return &KnowledgeGraph{
		dir:       dataDir,
		entities:  make(map[string]Entity),
		relations: make([]Relation, 0),
	}
}

// AddEntity adds an entity to the graph.
func (kg *KnowledgeGraph) AddEntity(entity Entity) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	if err := kg.load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load: %w", err)
	}

	if entity.ID == "" {
		entity.ID = strings.ToLower(strings.ReplaceAll(entity.Name, " ", "_"))
	}

	kg.entities[entity.ID] = entity
	return kg.save()
}

// AddRelation adds a relation to the graph.
func (kg *KnowledgeGraph) AddRelation(rel Relation) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	if err := kg.load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load: %w", err)
	}

	if rel.Weight == 0 {
		rel.Weight = 1.0
	}

	kg.relations = append(kg.relations, rel)
	return kg.save()
}

// ExtractFromCode parses source code and extracts entities and relations.
func (kg *KnowledgeGraph) ExtractFromCode(filePath, content string) ([]Entity, []Relation) {
	var entities []Entity
	var relations []Relation

	ext := filePath
	if idx := strings.LastIndex(filePath, "."); idx >= 0 {
		ext = filePath[idx:]
	}

	switch ext {
	case ".go":
		entities, relations = extractGo(filePath, content)
	case ".sh":
		entities, relations = extractShell(filePath, content)
	}

	return entities, relations
}

// GetAllEntities returns all entities.
func (kg *KnowledgeGraph) GetAllEntities() []Entity {
	kg.mu.RLock()
	defer kg.mu.RUnlock()
	kg.load()

	var entities []Entity
	for _, e := range kg.entities {
		entities = append(entities, e)
	}
	return entities
}

// GetAllRelations returns all relations.
func (kg *KnowledgeGraph) GetAllRelations() []Relation {
	kg.mu.RLock()
	defer kg.mu.RUnlock()
	kg.load()
	return kg.relations
}

// EntityCount returns the number of entities.
func (kg *KnowledgeGraph) EntityCount() int {
	kg.mu.RLock()
	defer kg.mu.RUnlock()
	kg.load()
	return len(kg.entities)
}

// RelationCount returns the number of relations.
func (kg *KnowledgeGraph) RelationCount() int {
	kg.mu.RLock()
	defer kg.mu.RUnlock()
	kg.load()
	return len(kg.relations)
}

// ═══════════════════════════════════════════════════════
// Go code extraction
// ═══════════════════════════════════════════════════════

var (
	goFuncRe      = regexp.MustCompile(`^func\s+(?:\(\s*\w+\s+\S+\s*\)\s+)?(\w+)\s*\(`)
	goImportRe    = regexp.MustCompile(`^import\s+"([^"]+)"`)
	goImportBlock = regexp.MustCompile(`^import\s+\(`)
	goCallRe      = regexp.MustCompile(`(\w+)\s*\(`)
	goAPIRe       = regexp.MustCompile(`(os\.ReadFile|os\.WriteFile|exec\.Command|fmt\.Print|http\.Get|http\.Post)`)
)

func extractGo(filePath, content string) ([]Entity, []Relation) {
	var entities []Entity
	var relations []Relation
	lines := strings.Split(content, "\n")

	inImportBlock := false
	funcName := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Import block
		if goImportBlock.MatchString(trimmed) {
			inImportBlock = true
			continue
		}
		if inImportBlock {
			if trimmed == ")" {
				inImportBlock = false
				continue
			}
			if m := goImportRe.FindStringSubmatch(trimmed); m != nil {
				eid := "pkg_" + strings.ReplaceAll(m[1], "/", "_")
				entities = append(entities, Entity{
					ID:     eid,
					Name:   m[1],
					Type:   EntityModule,
					Source: filePath,
				})
				relations = append(relations, Relation{
					From: funcName, To: eid, Type: RelationUses, Weight: 0.8,
				})
			}
			continue
		}

		// Single import
		if m := goImportRe.FindStringSubmatch(trimmed); m != nil {
			eid := "pkg_" + strings.ReplaceAll(m[1], "/", "_")
			entities = append(entities, Entity{
				ID:     eid,
				Name:   m[1],
				Type:   EntityModule,
				Source: filePath,
			})
		}

		// Function definition
		if m := goFuncRe.FindStringSubmatch(trimmed); m != nil {
			funcName = "func_" + m[1]
			entities = append(entities, Entity{
				ID:     funcName,
				Name:   m[1],
				Type:   EntityFunction,
				Source: filePath,
			})
		}

		// API calls
		if m := goAPIRe.FindStringSubmatch(trimmed); m != nil {
			apiID := "api_" + strings.ReplaceAll(m[1], ".", "_")
			entities = append(entities, Entity{
				ID:   apiID,
				Name: m[1],
				Type: EntityAPI,
			})
			if funcName != "" {
				relations = append(relations, Relation{
					From: funcName, To: apiID, Type: RelationCalls, Weight: 1.0,
				})
			}
		}

		// Function calls
		if funcName != "" {
			calls := goCallRe.FindAllStringSubmatch(trimmed, -1)
			for _, call := range calls {
				if call[1] != "func" && call[1] != "if" && call[1] != "for" && call[1] != "switch" {
					calleeID := "func_" + call[1]
					if calleeID != funcName {
						relations = append(relations, Relation{
							From: funcName, To: calleeID, Type: RelationCalls, Weight: 0.9,
						})
					}
				}
			}
		}
	}

	return entities, relations
}

// ═══════════════════════════════════════════════════════
// Shell code extraction
// ═══════════════════════════════════════════════════════

var (
	shellFuncRe = regexp.MustCompile(`^(\w+)\s*\(\)\s*\{`)
	shellVarRe  = regexp.MustCompile(`^(\w+)=`)
	shellAPIRe  = regexp.MustCompile(`(set_perm|set_perm_recursive|ui_print|abort)`)
)

func extractShell(filePath, content string) ([]Entity, []Relation) {
	var entities []Entity
	var relations []Relation
	lines := strings.Split(content, "\n")

	currentFunc := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Function definition
		if m := shellFuncRe.FindStringSubmatch(trimmed); m != nil {
			currentFunc = "func_" + m[1]
			entities = append(entities, Entity{
				ID:     currentFunc,
				Name:   m[1],
				Type:   EntityFunction,
				Source: filePath,
			})
		}

		// Variable definition
		if m := shellVarRe.FindStringSubmatch(trimmed); m != nil {
			vid := "var_" + m[1]
			entities = append(entities, Entity{
				ID:     vid,
				Name:   m[1],
				Type:   EntityConfig,
				Source: filePath,
			})
		}

		// Magisk API calls
		if m := shellAPIRe.FindStringSubmatch(trimmed); m != nil {
			apiID := "api_" + m[1]
			entities = append(entities, Entity{
				ID:   apiID,
				Name: m[1],
				Type: EntityAPI,
			})
			if currentFunc != "" {
				relations = append(relations, Relation{
					From: currentFunc, To: apiID, Type: RelationCalls, Weight: 1.0,
				})
			}
		}
	}

	return entities, relations
}

// ═══════════════════════════════════════════════════════
// Persistence
// ═══════════════════════════════════════════════════════

type graphData struct {
	Entities  []Entity   `json:"entities"`
	Relations []Relation `json:"relations"`
}

func (kg *KnowledgeGraph) load() error {
	path := filepath.Join(kg.dir, "knowledge_graph.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var gd graphData
	if err := json.Unmarshal(data, &gd); err != nil {
		return err
	}

	kg.entities = make(map[string]Entity, len(gd.Entities))
	for _, e := range gd.Entities {
		kg.entities[e.ID] = e
	}
	kg.relations = gd.Relations
	return nil
}

func (kg *KnowledgeGraph) save() error {
	if err := os.MkdirAll(kg.dir, 0755); err != nil {
		return err
	}

	entities := make([]Entity, 0, len(kg.entities))
	for _, e := range kg.entities {
		entities = append(entities, e)
	}

	gd := graphData{
		Entities:  entities,
		Relations: kg.relations,
	}

	data, err := json.MarshalIndent(gd, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(kg.dir, "knowledge_graph.json"), data, 0644)
}

// EntityID generates a normalized entity ID.
func EntityID(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", "_"))
}

// Now returns current time (for testing).
func Now() time.Time {
	return time.Now()
}
