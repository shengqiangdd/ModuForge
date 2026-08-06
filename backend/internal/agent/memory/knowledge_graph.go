package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
)

// KnowledgeGraph manages cross-project knowledge sharing and entity relationships.
type KnowledgeGraph struct {
	db        *sql.DB
	mu        sync.RWMutex
	entities  map[string]*Entity  // id -> entity
	relations map[string][]Relation // entityID -> relations
}

// Entity represents a technical entity extracted from memories.
type Entity struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"` // language, framework, pattern, concept
	Properties map[string]string `json:"properties"`
	ProjectIDs []string          `json:"project_ids"`
}

// Relation represents a relationship between two entities.
type Relation struct {
	SourceID   string `json:"source_id"`
	TargetID   string `json:"target_id"`
	Type       string `json:"type"` // uses, depends_on, similar_to, conflicts_with
	Weight     float64 `json:"weight"`
	ProjectID  string `json:"project_id"`
}

// NewKnowledgeGraph creates a new knowledge graph.
func NewKnowledgeGraph(db *sql.DB) *KnowledgeGraph {
	kg := &KnowledgeGraph{
		db:        db,
		entities:  make(map[string]*Entity),
		relations: make(map[string][]Relation),
	}
	kg.ensureTables()
	return kg
}

func (kg *KnowledgeGraph) ensureTables() {
	kg.db.Exec(`CREATE TABLE IF NOT EXISTS kg_entities (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		properties TEXT DEFAULT '{}',
		project_ids TEXT DEFAULT '[]',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	kg.db.Exec(`CREATE TABLE IF NOT EXISTS kg_relations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_id TEXT NOT NULL,
		target_id TEXT NOT NULL,
		type TEXT NOT NULL,
		weight REAL DEFAULT 1.0,
		project_id TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(source_id, target_id, type, project_id)
	)`)
	kg.db.Exec(`CREATE INDEX IF NOT EXISTS idx_kg_entity_name ON kg_entities(name)`)
	kg.db.Exec(`CREATE INDEX IF NOT EXISTS idx_kg_relation_source ON kg_relations(source_id)`)
	kg.db.Exec(`CREATE INDEX IF NOT EXISTS idx_kg_relation_target ON kg_relations(target_id)`)
}

// ExtractEntities extracts technical entities from text.
func (kg *KnowledgeGraph) ExtractEntities(text string) []Entity {
	var entities []Entity
	lower := strings.ToLower(text)

	// Language detection
	languages := map[string]string{
		"rust": "language", "go": "language", "python": "language",
		"javascript": "language", "typescript": "language", "c++": "language",
		"c": "language", "java": "language", "kotlin": "language",
	}
	for lang, typ := range languages {
		if strings.Contains(lower, lang) {
			entities = append(entities, Entity{
				Name: lang,
				Type: typ,
				Properties: map[string]string{"source": "extracted"},
			})
		}
	}

	// Framework detection
	frameworks := map[string]string{
		"kernelSU": "framework", "apatch": "framework", "magisk": "framework",
		"android": "platform", "linux": "platform",
		"react": "framework", "vue": "framework", "svelte": "framework",
		"fiber": "framework", "gin": "framework", "echo": "framework",
	}
	for fw, typ := range frameworks {
		if strings.Contains(lower, strings.ToLower(fw)) {
			entities = append(entities, Entity{
				Name: fw,
				Type: typ,
				Properties: map[string]string{"source": "extracted"},
			})
		}
	}

	// Pattern detection
	patterns := []string{
		"linucb", "thermal", "power", "energy", "scheduler",
		"ipc", "shared_memory", "bpf", "ebpf", "syscall",
	}
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			entities = append(entities, Entity{
				Name: pattern,
				Type: "pattern",
				Properties: map[string]string{"source": "extracted"},
			})
		}
	}

	return entities
}

// AddEntity adds an entity to the knowledge graph.
func (kg *KnowledgeGraph) AddEntity(entity Entity, projectID string) {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	// Generate ID if not provided
	if entity.ID == "" {
		entity.ID = fmt.Sprintf("entity_%s_%s", entity.Type, entity.Name)
	}

	// Add project to entity
	if projectID != "" {
		found := false
		for _, pid := range entity.ProjectIDs {
			if pid == projectID {
				found = true
				break
			}
		}
		if !found {
			entity.ProjectIDs = append(entity.ProjectIDs, projectID)
		}
	}

	// Store in memory
	kg.entities[entity.ID] = &entity

	// Store in database
	propsJSON, _ := json.Marshal(entity.Properties)
	projsJSON, _ := json.Marshal(entity.ProjectIDs)

	kg.db.Exec(`
		INSERT INTO kg_entities (id, name, type, properties, project_ids)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET properties=?, project_ids=?
	`, entity.ID, entity.Name, entity.Type, string(propsJSON), string(projsJSON),
		string(propsJSON), string(projsJSON))
}

// AddRelation adds a relation between two entities.
func (kg *KnowledgeGraph) AddRelation(sourceID, targetID, relationType, projectID string, weight float64) {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	relation := Relation{
		SourceID:  sourceID,
		TargetID:  targetID,
		Type:      relationType,
		Weight:    weight,
		ProjectID: projectID,
	}

	// Store in memory
	kg.relations[sourceID] = append(kg.relations[sourceID], relation)

	// Store in database
	kg.db.Exec(`
		INSERT INTO kg_relations (source_id, target_id, type, weight, project_id)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(source_id, target_id, type, project_id) DO UPDATE SET weight=?
	`, sourceID, targetID, relationType, weight, projectID, weight)
}

// FindSimilarProjects finds projects similar to the given project.
func (kg *KnowledgeGraph) FindSimilarProjects(projectID string, limit int) []string {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	// Get entities for this project
	projectEntities := make(map[string]bool)
	for _, entity := range kg.entities {
		for _, pid := range entity.ProjectIDs {
			if pid == projectID {
				projectEntities[entity.ID] = true
				break
			}
		}
	}

	// Find other projects with similar entities
	projectScores := make(map[string]float64)
	for entityID := range projectEntities {
		for _, entity := range kg.entities {
			if entity.ID == entityID {
				continue
			}
			// Check if entities are related
			for _, rel := range kg.relations[entityID] {
				if rel.TargetID == entity.ID && rel.Type == "similar_to" {
					for _, pid := range entity.ProjectIDs {
						if pid != projectID {
							projectScores[pid] += rel.Weight
						}
					}
				}
			}
		}
	}

	// Sort by score
	type projectScore struct {
		projectID string
		score     float64
	}
	var scores []projectScore
	for pid, score := range projectScores {
		scores = append(scores, projectScore{pid, score})
	}
	for i := 0; i < len(scores)-1; i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].score > scores[i].score {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}

	// Return top N
	var result []string
	for i := 0; i < len(scores) && i < limit; i++ {
		result = append(result, scores[i].projectID)
	}

	return result
}

// GetProjectKnowledge returns knowledge from similar projects.
func (kg *KnowledgeGraph) GetProjectKnowledge(projectID string) map[string]interface{} {
	similarProjects := kg.FindSimilarProjects(projectID, 5)

	result := map[string]interface{}{
		"similar_projects": similarProjects,
		"entities":         make([]Entity, 0),
		"relations":        make([]Relation, 0),
	}

	// Get entities from similar projects
	seen := make(map[string]bool)
	for _, pid := range similarProjects {
		for _, entity := range kg.entities {
			if seen[entity.ID] {
				continue
			}
			for _, epid := range entity.ProjectIDs {
				if epid == pid {
					result["entities"] = append(result["entities"].([]Entity), *entity)
					seen[entity.ID] = true
					break
				}
			}
		}
	}

	return result
}

// SuggestKnowledgeTransfer suggests knowledge transfer from similar projects.
func (kg *KnowledgeGraph) SuggestKnowledgeTransfer(projectID string, projectType string) []string {
	var suggestions []string

	// Find similar projects
	similarProjects := kg.FindSimilarProjects(projectID, 5)

	for _, pid := range similarProjects {
		// Get entities from this similar project
		for _, entity := range kg.entities {
			for _, epid := range entity.ProjectIDs {
				if epid == pid && entity.Type == "pattern" {
					suggestions = append(suggestions, fmt.Sprintf(
						"Project %s uses pattern '%s' which may be applicable here",
						pid, entity.Name))
				}
			}
		}
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "No similar projects found with transferable knowledge")
	}

	return suggestions
}

// BuildFromMemories builds the knowledge graph from existing memories.
func (kg *KnowledgeGraph) BuildFromMemories(userID string) error {
	rows, err := kg.db.Query(`
		SELECT content, tags FROM memory_v2
		WHERE user_id = ? AND (expires_at IS NULL OR expires_at > datetime('now'))
	`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var content, tagsJSON string
		if err := rows.Scan(&content, &tagsJSON); err == nil {
			// Extract entities from content
			entities := kg.ExtractEntities(content)

			// Add entities
			for _, entity := range entities {
				kg.AddEntity(entity, "")
			}

			// Extract relations from tags
			var tags []string
			json.Unmarshal([]byte(tagsJSON), &tags)
			if len(entities) > 1 {
				for i := 0; i < len(entities)-1; i++ {
					for j := i + 1; j < len(entities); j++ {
						kg.AddRelation(entities[i].ID, entities[j].ID, "co_occurs", "", 1.0)
					}
				}
			}
		}
	}

	log.Printf("[KnowledgeGraph] built graph with %d entities", len(kg.entities))
	return nil
}
