package agent

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type ToolDef struct {
	Type     string           `json:"type"`
	Function ToolFunctionDef `json:"function"`
}

type ToolFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

func (r *AgentRunner) buildToolDefinitionsForMode(mode AgentMode, modelName string) []ToolDef {
	skills := r.registry.List()
	defs := make([]ToolDef, 0, len(skills))

	// Derive skill sets from metadata (no hardcoded maps)
	readOnlySkills := r.registry.ReadOnlySkills()
	essentialToolsFree := r.registry.EssentialToolsForFree()

	// For free models, only expose essential tools (if any are marked)
	// If no skills implement MetadataProvider, expose all tools for free models too
	isFree := resolveModelTier(modelName) == TierFree
	hasEssentialMetadata := len(essentialToolsFree) > 0

	for _, s := range skills {
		// In Plan mode, only expose read-only tools
		if mode == ModePlan && !readOnlySkills[s.Name()] {
			continue
		}

		// For free models with essential metadata, skip non-essential tools
		// For free models without essential metadata, expose all tools
		if isFree && hasEssentialMetadata && !essentialToolsFree[s.Name()] {
			continue
		}

		def := ToolDef{
			Type: "function",
			Function: ToolFunctionDef{
				Name:        s.Name(),
				Description: s.Description(),
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		}

		switch s.Name() {
		case "read_file":
			def.Function.Parameters["properties"] = map[string]interface{}{
				"path":       map[string]interface{}{"type": "string", "description": "File path to read"},
				"start_line": map[string]interface{}{"type": "integer", "description": "First line (1-based, optional)"},
				"end_line":   map[string]interface{}{"type": "integer", "description": "Last line (1-based, optional)"},
			}
			def.Function.Parameters["required"] = []string{"path"}
		case "write_file":
			def.Function.Parameters["properties"] = map[string]interface{}{
				"path":    map[string]interface{}{"type": "string", "description": "File path to write"},
				"content": map[string]interface{}{"type": "string", "description": "Complete file content"},
			}
			def.Function.Parameters["required"] = []string{"path", "content"}
		case "write_file_batch":
			def.Function.Parameters["properties"] = map[string]interface{}{
				"files": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "object"},
					"description": "Array of {path, content} objects to write in one transaction",
				},
				"project_id": map[string]interface{}{"type": "string", "description": "Project ID (auto-created if omitted)"},
			}
			def.Function.Parameters["required"] = []string{"files"}
		case "think":
			def.Function.Parameters["properties"] = map[string]interface{}{
				"thought": map[string]interface{}{"type": "string", "description": "Your reasoning, analysis, or plan"},
			}
			def.Function.Parameters["required"] = []string{"thought"}
		case "gather_requirements":
			def.Function.Parameters["properties"] = map[string]interface{}{
				"description": map[string]interface{}{"type": "string", "description": "Project/module description to analyze"},
				"answers":     map[string]interface{}{"type": "object", "description": "Optional Q&A answers {q1: answer, ...}"},
			}
			def.Function.Parameters["required"] = []string{"description"}
		case "match_template":
			def.Function.Parameters["properties"] = map[string]interface{}{
				"description": map[string]interface{}{"type": "string", "description": "Project description to match against templates"},
				"type":        map[string]interface{}{"type": "string", "description": "Module type (e.g. magisk_module, android_app, library)"},
			}
			def.Function.Parameters["required"] = []string{"description", "type"}
		case "generate_code", "code_pipeline":
			def.Function.Parameters["properties"] = map[string]interface{}{
				"description":  map[string]interface{}{"type": "string", "description": "What to generate or build"},
				"files_spec":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": "File specifications to generate"},
				"pipeline":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Pipeline stages (detect/lint/test only)"},
				"project_id":   map[string]interface{}{"type": "string", "description": "Project ID for file context"},
			}
			def.Function.Parameters["required"] = []string{"description"}
		case "create_dir":
			def.Function.Parameters["properties"] = map[string]interface{}{
				"path": map[string]interface{}{"type": "string", "description": "Directory path to create"},
			}
			def.Function.Parameters["required"] = []string{"path"}
		case "detect":
			def.Function.Parameters["properties"] = map[string]interface{}{
				"path": map[string]interface{}{"type": "string", "description": "File path to analyze"},
			}
			def.Function.Parameters["required"] = []string{"path"}
		case "lint_code":
			def.Function.Parameters["properties"] = map[string]interface{}{
				"files": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "File paths to lint"},
			}
			def.Function.Parameters["required"] = []string{"files"}
		case "validate":
			def.Function.Parameters["properties"] = map[string]interface{}{
				"files": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "File paths to validate"},
			}
			def.Function.Parameters["required"] = []string{"files"}
		case "web_search":
			def.Function.Parameters["properties"] = map[string]interface{}{
				"query": map[string]interface{}{"type": "string", "description": "Search query"},
			}
			def.Function.Parameters["required"] = []string{"query"}
		case "memory_manager":
			def.Function.Parameters["properties"] = map[string]interface{}{
				"action": map[string]interface{}{"type": "string", "description": "Action: list/get/update/delete/search"},
				"key":    map[string]interface{}{"type": "string", "description": "Memory key"},
				"value":  map[string]interface{}{"type": "string", "description": "Memory value (for update)"},
				"query":  map[string]interface{}{"type": "string", "description": "Search query (for search)"},
			}
			def.Function.Parameters["required"] = []string{"action"}
		default:
			def.Function.Parameters["properties"] = map[string]interface{}{
				"input": map[string]interface{}{"type": "string", "description": "Input for the skill"},
			}
		}

		defs = append(defs, def)
	}

	return defs
}

// getToolDefinitions returns cached tool definitions for the given mode+tier.
// Tool definitions don't change between iterations, so caching avoids rebuilding.
// Optimization 31: Cache key uses tier only (not model name) — tier determines
// which tools are exposed, and all models within a tier see the same tools.
func (r *AgentRunner) getToolDefinitions(mode AgentMode, modelName string) []ToolDef {
	tier := resolveModelTier(modelName)
	// Cache key: "mode:tier" (e.g., "act:0" for free act mode)
	cacheKey := string(mode) + ":" + strconv.Itoa(int(tier))

	// Fast path: read lock
	r.toolDefCacheMu.RLock()
	if defs, ok := r.toolDefCache[cacheKey]; ok {
		r.toolDefCacheMu.RUnlock()
		return defs
	}
	r.toolDefCacheMu.RUnlock()

	// Slow path: build and cache
	defs := r.buildToolDefinitionsForMode(mode, modelName)
	r.toolDefCacheMu.Lock()
	r.toolDefCache[cacheKey] = defs
	r.toolDefCacheMu.Unlock()
	return defs
}

// validateRequiredParams checks that all required parameters are present for a given skill.
// Returns a comma-separated list of missing param names, or "" if all OK.
func validateRequiredParams(skillName string, input map[string]interface{}) string {
	requiredMap := map[string][]string{
		"write_file":        {"path", "content"},
		"write_file_batch":  {"files"},
		"read_file":         {"path"},
		"create_dir":        {"path"},
		"delete_file":       {"path"},
		"delete_dir":        {"path"},
		"move_file":         {"source", "destination"},
		"list_dir":          {"path"},
		"detect":            {"path"},
		"web_search":        {"query"},
		"think":             {"thought"},
		"gather_requirements": {"description"},
		"match_template":    {"description", "type"},
		"generate_code":     {"description"},
		"code_pipeline":     {"description"},
	}
	required, ok := requiredMap[skillName]
	if !ok {
		return "" // unknown skill, skip validation
	}
	var missing []string
	for _, param := range required {
		val, exists := input[param]
		if !exists || val == nil {
			missing = append(missing, param)
		} else if s, ok := val.(string); ok && s == "" {
			missing = append(missing, param)
		}
	}
	if len(missing) > 0 {
		return strings.Join(missing, ", ")
	}
	return ""
}

// normalizeSkillInput parses the LLM's "input" JSON string field and merges its contents into top-level params.
// This allows LLM to send {"input": "{\"thought\":\"...\"}"} and have it work the same as {"thought":"..."}.
// Optimization 20: Also coerces common type mismatches (string→int for line numbers, etc.)
func normalizeSkillInput(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return input
	}
	rawInput, ok := input["input"].(string)
	if !ok || rawInput == "" {
		// Still apply type coercion even without input wrapper
		coerceTypes(input)
		return input
	}
	// Try JSON parse
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(rawInput), &parsed); err != nil {
		coerceTypes(input)
		return input // Not JSON, return as-is
	}
	// Merge parsed fields into top-level (don't overwrite existing)
	for k, v := range parsed {
		if _, exists := input[k]; !exists {
			input[k] = v
		}
	}
	// Remove the raw "input" field to avoid confusion
	delete(input, "input")
	// Apply type coercion
	coerceTypes(input)
	return input
}

// coerceTypes fixes common LLM type mismatches.
// Free models often send integer parameters as strings (e.g., "start_line": "10" instead of 10).
func coerceTypes(input map[string]interface{}) {
	intFields := []string{"start_line", "end_line", "limit", "offset", "port"}
	for _, field := range intFields {
		if v, ok := input[field]; ok {
			if s, ok := v.(string); ok && s != "" {
				if n, err := strconv.Atoi(s); err == nil {
					input[field] = n
					debugLog("type coercion: %s string(%q) → int(%d)", field, s, n)
				}
			}
		}
	}
}

func (r *AgentRunner) executeSkill(ctx context.Context, name string, input map[string]interface{}) (string, error) {
	start := time.Now()
	result, err := r.registry.Execute(ctx, name, input)
	duration := time.Since(start).Milliseconds()
	if r.db != nil {
		go r.recordExecution(name, input, result, err, duration)
	}
	if err != nil {
		return "", err
	}
	return result, nil
}

func (r *AgentRunner) recordExecution(name string, input map[string]interface{}, result string, err error, durationMs int64) {
	if r.db == nil {
		return
	}
	inputJSON, _ := json.Marshal(input)
	status := "success"
	errMsg := ""
	if err != nil {
		status = "error"
		errMsg = err.Error()
	}
	// Optimization 38: Batch DB writes in a single transaction (2 inserts → 1 transaction)
	inputSize := len(inputJSON)
	resultSize := len(result)
	if resultSize > 2000 {
		result = result[:2000] + "...(truncated)"
		resultSize = 2000
	}
	tx, txErr := r.db.Begin()
	if txErr != nil {
		// Fallback: individual inserts if transaction fails
		r.db.Exec(
			`INSERT INTO skill_executions (skill_name, input, result, error_msg, duration_ms, status, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, datetime('now'))`,
			name, string(inputJSON), result, errMsg, durationMs, status,
		)
		r.db.Exec(
			`INSERT INTO skill_metrics (skill_name, input_size, result_size, duration_ms, status, created_at)
			 VALUES (?, ?, ?, ?, ?, datetime('now'))`,
			name, inputSize, resultSize, durationMs, status,
		)
		return
	}
	tx.Exec(
		`INSERT INTO skill_executions (skill_name, input, result, error_msg, duration_ms, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, datetime('now'))`,
		name, string(inputJSON), result, errMsg, durationMs, status,
	)
	tx.Exec(
		`INSERT INTO skill_metrics (skill_name, input_size, result_size, duration_ms, status, created_at)
		 VALUES (?, ?, ?, ?, ?, datetime('now'))`,
		name, inputSize, resultSize, durationMs, status,
	)
	tx.Commit()
}

func (r *AgentRunner) ListSkills() []Skill {
	return r.registry.List()
}
