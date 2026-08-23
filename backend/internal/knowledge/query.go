package knowledge

import (
	"fmt"
	"strings"
)

// GraphResult is a query result with path and distance.
type GraphResult struct {
	Entity   Entity   `json:"entity"`
	Path     []string `json:"path"`
	Distance int      `json:"distance"`
}

// Recommendation suggests an approach for a requirement.
type Recommendation struct {
	Steps      []string `json:"steps"`
	Reason     string   `json:"reason"`
	Confidence float64  `json:"confidence"`
}

// GraphQuery provides query operations on the knowledge graph.
type GraphQuery struct {
	graph *KnowledgeGraph
}

// NewGraphQuery creates a query interface for the given graph.
func NewGraphQuery(graph *KnowledgeGraph) *GraphQuery {
	return &GraphQuery{graph: graph}
}

// FindRelated finds all entities related to the given entity within maxDepth.
func (gq *GraphQuery) FindRelated(entityName string, maxDepth int) []GraphResult {
	gq.graph.mu.RLock()
	defer gq.graph.mu.RUnlock()
	gq.graph.load()

	if maxDepth <= 0 {
		maxDepth = 3
	}

	entityID := EntityID(entityName)
	visited := make(map[string]bool)
	var results []GraphResult

	type queueItem struct {
		id    string
		path  []string
		depth int
	}

	queue := []queueItem{{id: entityID, path: []string{entityID}, depth: 0}}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if visited[item.id] {
			continue
		}
		visited[item.id] = true

		entity, ok := gq.graph.entities[item.id]
		if !ok {
			continue
		}

		results = append(results, GraphResult{
			Entity:   entity,
			Path:     item.path,
			Distance: item.depth,
		})

		if item.depth >= maxDepth {
			continue
		}

		// Find neighbors
		neighbors := gq.getNeighbors(item.id)
		for _, neighbor := range neighbors {
			if !visited[neighbor] {
				newPath := make([]string, len(item.path))
				copy(newPath, item.path)
				newPath = append(newPath, neighbor)
				queue = append(queue, queueItem{
					id:    neighbor,
					path:  newPath,
					depth: item.depth + 1,
				})
			}
		}
	}

	return results
}

// FindPath finds the shortest path between two entities using BFS.
func (gq *GraphQuery) FindPath(from, to string) []string {
	gq.graph.mu.RLock()
	defer gq.graph.mu.RUnlock()
	gq.graph.load()

	fromID := EntityID(from)
	toID := EntityID(to)

	if fromID == toID {
		return []string{fromID}
	}

	visited := make(map[string]bool)
	parent := make(map[string]string)

	type queueItem struct {
		id   string
		path []string
	}

	queue := []queueItem{{id: fromID, path: []string{fromID}}}
	visited[fromID] = true

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		neighbors := gq.getNeighbors(item.id)
		for _, neighbor := range neighbors {
			if visited[neighbor] {
				continue
			}

			visited[neighbor] = true
			parent[neighbor] = item.id

			if neighbor == toID {
				// Reconstruct path
				return gq.reconstructPath(parent, fromID, toID)
			}

			newPath := make([]string, len(item.path))
			copy(newPath, item.path)
			newPath = append(newPath, neighbor)
			queue = append(queue, queueItem{id: neighbor, path: newPath})
		}
	}

	return nil // No path found
}

// GetEntitiesByType returns all entities of a specific type.
func (gq *GraphQuery) GetEntitiesByType(entityType EntityType) []Entity {
	gq.graph.mu.RLock()
	defer gq.graph.mu.RUnlock()
	gq.graph.load()

	var entities []Entity
	for _, e := range gq.graph.entities {
		if e.Type == entityType {
			entities = append(entities, e)
		}
	}
	return entities
}

// GetRelations returns all relations involving the given entity.
func (gq *GraphQuery) GetRelations(entityName string) []Relation {
	gq.graph.mu.RLock()
	defer gq.graph.mu.RUnlock()
	gq.graph.load()

	entityID := EntityID(entityName)
	var relations []Relation

	for _, rel := range gq.graph.relations {
		if rel.From == entityID || rel.To == entityID {
			relations = append(relations, rel)
		}
	}

	return relations
}

// RecommendApproach analyzes a requirement and recommends implementation steps.
func (gq *GraphQuery) RecommendApproach(requirement string) []Recommendation {
	gq.graph.mu.RLock()
	defer gq.graph.mu.RUnlock()
	gq.graph.load()

	reqLower := strings.ToLower(requirement)
	var recommendations []Recommendation

	// Find relevant entities by keyword matching
	relevantEntities := gq.findRelevantEntities(reqLower)

	// Build recommendations based on entity types and relations
	if len(relevantEntities) > 0 {
		rec := gq.buildRecommendation(reqLower, relevantEntities)
		if rec != nil {
			recommendations = append(recommendations, *rec)
		}
	}

	// Add default recommendations based on keywords
	recommendations = append(recommendations, gq.keywordBasedRecommendations(reqLower)...)

	return recommendations
}

// ═══════════════════════════════════════════════════════
// Internal helpers
// ═══════════════════════════════════════════════════════

func (gq *GraphQuery) getNeighbors(entityID string) []string {
	neighborSet := make(map[string]bool)

	for _, rel := range gq.graph.relations {
		if rel.From == entityID {
			neighborSet[rel.To] = true
		}
		if rel.To == entityID {
			neighborSet[rel.From] = true
		}
	}

	var neighbors []string
	for n := range neighborSet {
		neighbors = append(neighbors, n)
	}
	return neighbors
}

func (gq *GraphQuery) reconstructPath(parent map[string]string, from, to string) []string {
	path := []string{to}
	current := to
	for current != from {
		p, ok := parent[current]
		if !ok {
			return nil
		}
		path = append([]string{p}, path...)
		current = p
	}
	return path
}

func (gq *GraphQuery) findRelevantEntities(query string) []Entity {
	var entities []Entity
	queryWords := strings.Fields(query)

	for _, e := range gq.graph.entities {
		score := 0
		text := strings.ToLower(e.Name + " " + e.Description + " " + string(e.Type))
		for _, word := range queryWords {
			if len(word) >= 3 && strings.Contains(text, word) {
				score++
			}
		}
		if score > 0 {
			entities = append(entities, e)
		}
	}

	return entities
}

func (gq *GraphQuery) buildRecommendation(query string, entities []Entity) *Recommendation {
	if len(entities) == 0 {
		return nil
	}

	var steps []string
	steps = append(steps, fmt.Sprintf("分析需求: %s", query))

	for _, e := range entities {
		switch e.Type {
		case EntityFunction:
			steps = append(steps, fmt.Sprintf("使用函数: %s (%s)", e.Name, e.Description))
		case EntityAPI:
			steps = append(steps, fmt.Sprintf("调用API: %s", e.Name))
		case EntityConfig:
			steps = append(steps, fmt.Sprintf("配置参数: %s", e.Name))
		}
	}

	steps = append(steps, "验证实现并测试")

	return &Recommendation{
		Steps:      steps,
		Reason:     fmt.Sprintf("基于知识库中 %d 个相关实体的分析", len(entities)),
		Confidence: 0.7,
	}
}

func (gq *GraphQuery) keywordBasedRecommendations(query string) []Recommendation {
	var recs []Recommendation

	if strings.Contains(query, "root") || strings.Contains(query, "权限") {
		recs = append(recs, Recommendation{
			Steps:      []string{"检测设备Root状态", "检查Magisk安装", "验证SU权限"},
			Reason:     "Root相关需求通常需要先验证权限",
			Confidence: 0.8,
		})
	}

	if strings.Contains(query, "性能") || strings.Contains(query, "优化") {
		recs = append(recs, Recommendation{
			Steps:      []string{"分析当前性能瓶颈", "选择优化策略", "实现并测试", "监控效果"},
			Reason:     "性能优化需要系统性分析和迭代",
			Confidence: 0.75,
		})
	}

	if strings.Contains(query, "电池") || strings.Contains(query, "省电") {
		recs = append(recs, Recommendation{
			Steps:      []string{"监控电池状态", "分析耗电应用", "实现省电策略", "验证效果"},
			Reason:     "电池管理需要监控和策略结合",
			Confidence: 0.75,
		})
	}

	return recs
}
