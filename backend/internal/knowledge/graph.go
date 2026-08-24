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
	goImportLine  = regexp.MustCompile(`^"([^"]+)"`)
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
			if m := goImportLine.FindStringSubmatch(trimmed); m != nil {
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

// ═══════════════════════════════════════════════════════
// KNOWLEDGE ACCUMULATION — 成功/失败模式记录
// ═══════════════════════════════════════════════════════

// SuccessRecord records a successful module build.
type SuccessRecord struct {
	Timestamp    time.Time `json:"timestamp"`
	ModuleType   string    `json:"module_type"`   // "daemon", "tool", "tweak"
	Languages    []string  `json:"languages"`     // ["go", "shell"]
	Patterns     []string  `json:"patterns"`      // Used code patterns
	QualityScore int       `json:"quality_score"` // 0-100
	FilePaths    []string  `json:"file_paths"`    // Generated files
	Description  string    `json:"description"`   // Module description
}

// FailureRecord records a failed build attempt.
type FailureRecord struct {
	Timestamp    time.Time `json:"timestamp"`
	ModuleType   string    `json:"module_type"`
	ErrorType    string    `json:"error_type"` // "compile", "runtime", "linker"
	ErrorMessage string    `json:"error_message"`
	FixApplied   string    `json:"fix_applied"` // What fixed it
	Patterns     []string  `json:"patterns"`    // Patterns that were used
}

// PatternScore tracks the success rate of a pattern.
type PatternScore struct {
	PatternID  string    `json:"pattern_id"`
	Successes  int       `json:"successes"`
	Failures   int       `json:"failures"`
	LastUsed   time.Time `json:"last_used"`
	AvgQuality float64   `json:"avg_quality"`
}

// KnowledgeHistory stores success and failure records.
type KnowledgeHistory struct {
	Successes []SuccessRecord `json:"successes"`
	Failures  []FailureRecord `json:"failures"`
	Patterns  []PatternScore  `json:"patterns"`
}

// RecordSuccess records a successful module build.
func (kg *KnowledgeGraph) RecordSuccess(moduleType string, patterns []string, qualityScore int, filePaths []string, description string) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	if err := kg.load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load: %w", err)
	}

	// Load history
	history, err := kg.loadHistory()
	if err != nil {
		history = &KnowledgeHistory{}
	}

	// Add success record
	record := SuccessRecord{
		Timestamp:    Now(),
		ModuleType:   moduleType,
		Languages:    detectLanguages(filePaths),
		Patterns:     patterns,
		QualityScore: qualityScore,
		FilePaths:    filePaths,
		Description:  description,
	}
	history.Successes = append(history.Successes, record)

	// Update pattern scores
	for _, patternID := range patterns {
		found := false
		for i := range history.Patterns {
			if history.Patterns[i].PatternID == patternID {
				history.Patterns[i].Successes++
				history.Patterns[i].LastUsed = Now()
				// Update average quality
				total := float64(history.Patterns[i].Successes + history.Patterns[i].Failures)
				history.Patterns[i].AvgQuality = (history.Patterns[i].AvgQuality*(total-1) + float64(qualityScore)) / total
				found = true
				break
			}
		}
		if !found {
			history.Patterns = append(history.Patterns, PatternScore{
				PatternID:  patternID,
				Successes:  1,
				LastUsed:   Now(),
				AvgQuality: float64(qualityScore),
			})
		}
	}

	return kg.saveHistory(history)
}

// RecordFailure records a failed build attempt.
func (kg *KnowledgeGraph) RecordFailure(moduleType string, errorType string, errorMessage string, fixApplied string, patterns []string) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	if err := kg.load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load: %w", err)
	}

	history, err := kg.loadHistory()
	if err != nil {
		history = &KnowledgeHistory{}
	}

	record := FailureRecord{
		Timestamp:    Now(),
		ModuleType:   moduleType,
		ErrorType:    errorType,
		ErrorMessage: errorMessage,
		FixApplied:   fixApplied,
		Patterns:     patterns,
	}
	history.Failures = append(history.Failures, record)

	// Update pattern scores
	for _, patternID := range patterns {
		for i := range history.Patterns {
			if history.Patterns[i].PatternID == patternID {
				history.Patterns[i].Failures++
				history.Patterns[i].LastUsed = Now()
				break
			}
		}
	}

	return kg.saveHistory(history)
}

// RecommendPatterns recommends patterns based on historical success rates.
func (kg *KnowledgeGraph) RecommendPatterns(moduleType string, requirements []string) []string {
	kg.mu.RLock()
	defer kg.mu.RUnlock()
	kg.load()

	history, err := kg.loadHistory()
	if err != nil {
		return nil
	}

	// Find patterns with high success rates
	type scoredPattern struct {
		pattern string
		score   float64
	}

	var candidates []scoredPattern
	for _, ps := range history.Patterns {
		total := ps.Successes + ps.Failures
		if total == 0 {
			continue
		}
		successRate := float64(ps.Successes) / float64(total)
		// Boost recently used patterns
		daysSinceUse := Now().Sub(ps.LastUsed).Hours() / 24
		recencyBoost := 1.0 / (1.0 + daysSinceUse*0.1)
		score := successRate * recencyBoost * ps.AvgQuality / 100
		candidates = append(candidates, scoredPattern{pattern: ps.PatternID, score: score})
	}

	// Sort by score (simple selection sort for small N)
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Return top 5 patterns
	var result []string
	limit := 5
	if len(candidates) < limit {
		limit = len(candidates)
	}
	for i := 0; i < limit; i++ {
		result = append(result, candidates[i].pattern)
	}

	return result
}

// GetSuccessRate returns the overall success rate.
func (kg *KnowledgeGraph) GetSuccessRate() float64 {
	kg.mu.RLock()
	defer kg.mu.RUnlock()
	kg.load()

	history, err := kg.loadHistory()
	if err != nil {
		return 0
	}

	total := len(history.Successes) + len(history.Failures)
	if total == 0 {
		return 0
	}
	return float64(len(history.Successes)) / float64(total)
}

// GetRecentFailures returns recent failure records for analysis.
func (kg *KnowledgeGraph) GetRecentFailures(limit int) []FailureRecord {
	kg.mu.RLock()
	defer kg.mu.RUnlock()
	kg.load()

	history, err := kg.loadHistory()
	if err != nil {
		return nil
	}

	// Return most recent failures
	if limit <= 0 {
		limit = 10
	}
	start := len(history.Failures) - limit
	if start < 0 {
		start = 0
	}
	return history.Failures[start:]
}

// detectLanguages detects programming languages from file paths.
func detectLanguages(filePaths []string) []string {
	langSet := make(map[string]bool)
	for _, fp := range filePaths {
		ext := filepath.Ext(fp)
		switch ext {
		case ".go":
			langSet["go"] = true
		case ".c", ".cpp", ".h":
			langSet["c"] = true
		case ".rs":
			langSet["rust"] = true
		case ".sh":
			langSet["shell"] = true
		case ".py":
			langSet["python"] = true
		case ".js", ".ts":
			langSet["javascript"] = true
		}
	}
	var langs []string
	for lang := range langSet {
		langs = append(langs, lang)
	}
	return langs
}

// ═══════════════════════════════════════════════════════
// History Persistence
// ═══════════════════════════════════════════════════════

func (kg *KnowledgeGraph) loadHistory() (*KnowledgeHistory, error) {
	path := filepath.Join(kg.dir, "knowledge_history.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var history KnowledgeHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}
	return &history, nil
}

func (kg *KnowledgeGraph) saveHistory(history *KnowledgeHistory) error {
	if err := os.MkdirAll(kg.dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(kg.dir, "knowledge_history.json"), data, 0644)
}
