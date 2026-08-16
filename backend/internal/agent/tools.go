package agent

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/agent/registry"
)

type ToolDef struct {
	Type     string          `json:"type"`
	Function ToolFunctionDef `json:"function"`
}

type ToolFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// toolParams builds a JSON-schema object parameter map. "required" is only
// included when non-empty.
func toolParams(props map[string]interface{}, required []string) map[string]interface{} {
	p := map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		p["required"] = required
	}
	return p
}

func (r *AgentRunner) buildToolDefinitionsForMode(mode AgentMode, modelName string) []ToolDef {
	skills := r.registry.List()
	defs := make([]ToolDef, 0, len(skills))

	// Derive skill sets from metadata (no hardcoded maps)
	readOnlySkills := r.registry.ReadOnlySkills()
	essentialToolsFree := r.registry.EssentialToolsForFree()
	coreTools := r.registry.CoreTools()

	// For free models, only expose essential tools (if any are marked)
	// If no skills implement MetadataProvider, expose all tools for free models too
	isFree := resolveModelTier(modelName) == TierFree
	hasEssentialMetadata := len(essentialToolsFree) > 0
	hasCoreTools := len(coreTools) > 0

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

		// Core tool filtering: if core tools are defined, only expose core + read-only tools
		// This reduces tool bloat for ALL models (not just free tier)
		if hasCoreTools && !coreTools[s.Name()] && !readOnlySkills[s.Name()] {
			continue
		}

		def := ToolDef{
			Type: "function",
			Function: ToolFunctionDef{
				Name:        s.Name(),
				Description: s.Description(),
			},
		}

		switch s.Name() {
		case "read_file":
			def.Function.Description = "Read file content. For large codebases, consider grep_search/glob_search first to locate relevant code, but reading a known file directly is fine."
			def.Function.Parameters = toolParams(map[string]interface{}{
				"path":       map[string]interface{}{"type": "string", "description": "File path to read"},
				"start_line": map[string]interface{}{"type": "integer", "description": "First line (1-based, optional)"},
				"end_line":   map[string]interface{}{"type": "integer", "description": "Last line (1-based, optional)"},
			}, []string{"path"})
		case "write_file":
			def.Function.Description = "Write COMPLETE file content. Use ONLY for new files or complete rewrites. For small changes, prefer edit_file."
			def.Function.Parameters = toolParams(map[string]interface{}{
				"path":    map[string]interface{}{"type": "string", "description": "File path to write"},
				"content": map[string]interface{}{"type": "string", "description": "Complete file content"},
			}, []string{"path", "content"})
		case "edit_file":
			def.Function.Description = "Make targeted edits to an existing file. Use this for MOST changes (preferred over write_file for modifications <30% of file). Specify exact old_text to find and new_text to replace."
			def.Function.Parameters = toolParams(map[string]interface{}{
				"path":     map[string]interface{}{"type": "string", "description": "File path to edit"},
				"old_text": map[string]interface{}{"type": "string", "description": "Exact text to find (must match exactly)"},
				"new_text": map[string]interface{}{"type": "string", "description": "Replacement text"},
			}, []string{"path", "old_text", "new_text"})
		case "grep_search":
			def.Function.Description = "Search code patterns across all project files (like grep -rn). Use BEFORE read_file to find relevant code."
			def.Function.Parameters = toolParams(map[string]interface{}{
				"pattern":         map[string]interface{}{"type": "string", "description": "Search pattern (regex supported)"},
				"include_pattern": map[string]interface{}{"type": "string", "description": "File filter glob (e.g. *.go)"},
				"context_lines":   map[string]interface{}{"type": "integer", "description": "Context lines before/after match"},
				"is_regex":        map[string]interface{}{"type": "boolean", "description": "Treat pattern as regex"},
			}, []string{"pattern"})
		case "glob_search":
			def.Function.Description = "Find files by name pattern (e.g., *.go, **/*.rs). Use to discover project structure."
			def.Function.Parameters = toolParams(map[string]interface{}{
				"pattern": map[string]interface{}{"type": "string", "description": "Glob pattern (use ** for recursive)"},
			}, []string{"pattern"})
		case "bash":
			def.Function.Description = "Execute shell commands. Use for: build (go build, cargo build), test (go test, cargo test), git, file inspection (cat, ls, find)."
			def.Function.Parameters = toolParams(map[string]interface{}{
				"command": map[string]interface{}{"type": "string", "description": "Shell command to execute"},
			}, []string{"command"})
		case "build_module":
			def.Function.Description = "Compile and package the project into a ZIP. ALWAYS call after writing code to verify it compiles."
			def.Function.Parameters = toolParams(map[string]interface{}{
				"project_id": map[string]interface{}{"type": "string", "description": "Project ID to build"},
			}, []string{"project_id"})
		case "syntax_checker":
			def.Function.Description = "Pre-build syntax validation for Go, Rust, C/C++. Run AFTER writing code but BEFORE build_module to catch errors early. Returns structured errors with line numbers and fix hints."
			def.Function.Parameters = toolParams(map[string]interface{}{
				"project_id": map[string]interface{}{"type": "string", "description": "Project ID"},
				"language":   map[string]interface{}{"type": "string", "enum": []interface{}{"auto", "go", "rust", "cpp"}, "description": "Language to check (default: auto-detect)"},
			}, nil)
		case "test_module":
			def.Function.Description = "Validate Magisk module files (module.prop, shell script syntax, permissions, META-INF). Use after build_module to catch issues compilers miss."
			def.Function.Parameters = toolParams(map[string]interface{}{
				"files": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Files to validate (optional; defaults to all project files)"},
				"test_type": map[string]interface{}{"type": "string", "enum": []interface{}{"shell", "unit", "integration", "all"}, "description": "Validation type (default all)"},
			}, []string{})
		case "list_dir":
			def.Function.Description = "List files and directories in the project (non-recursive by default). Use to discover project structure."
			def.Function.Parameters = toolParams(map[string]interface{}{
				"path":       map[string]interface{}{"type": "string", "description": "Directory path (default '.')"},
				"recursive":  map[string]interface{}{"type": "boolean", "description": "List recursively"},
				"project_id": map[string]interface{}{"type": "string", "description": "Project ID"},
			}, nil)
		case "delete_file":
			def.Function.Description = "Delete a single file from the project. Use when a file is no longer needed."
			def.Function.Parameters = toolParams(map[string]interface{}{
				"path":       map[string]interface{}{"type": "string", "description": "File path to delete"},
				"project_id": map[string]interface{}{"type": "string", "description": "Project ID"},
			}, []string{"path"})
		case "delete_dir":
			def.Function.Description = "Delete a directory and its contents. Set confirm=true when the path is '.' (project root)."
			def.Function.Parameters = toolParams(map[string]interface{}{
				"path":       map[string]interface{}{"type": "string", "description": "Directory path to delete"},
				"confirm":    map[string]interface{}{"type": "boolean", "description": "Confirmation flag (required when deleting project root)"},
				"project_id": map[string]interface{}{"type": "string", "description": "Project ID"},
			}, []string{"path"})
		case "move_file":
			def.Function.Description = "Move or rename a file within the project (from → to)."
			def.Function.Parameters = toolParams(map[string]interface{}{
				"from":       map[string]interface{}{"type": "string", "description": "Source path"},
				"to":         map[string]interface{}{"type": "string", "description": "Destination path"},
				"project_id": map[string]interface{}{"type": "string", "description": "Project ID"},
			}, []string{"from", "to"})
		case "write_file_batch":
			def.Function.Description = "Write multiple files atomically in one call. Prefer for multi-file changes over repeated write_file calls."
			def.Function.Parameters = toolParams(map[string]interface{}{
				"files": map[string]interface{}{"type": "array", "items": map[string]interface{}{
					"type": "object", "properties": map[string]interface{}{
						"path":    map[string]interface{}{"type": "string", "description": "File path"},
						"content": map[string]interface{}{"type": "string", "description": "File content"},
					}, "required": []interface{}{"path", "content"},
				}, "description": "List of files to write"},
				"project_id": map[string]interface{}{"type": "string", "description": "Project ID"},
			}, []string{"files"})
		default:
			// MCP-backed and other parameterized skills expose their own
			// JSON-Schema definition; fall back to a generic input wrapper.
			if pp, ok := s.(registry.ParameterProvider); ok {
				if params := pp.Parameters(); params != nil {
					def.Function.Parameters = params
				} else {
					def.Function.Parameters = toolParams(map[string]interface{}{
						"input": map[string]interface{}{"type": "string", "description": "Input for the skill"},
					}, nil)
				}
			} else {
				def.Function.Parameters = toolParams(map[string]interface{}{
					"input": map[string]interface{}{"type": "string", "description": "Input for the skill"},
				}, nil)
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
		"write_file":       {"path", "content"},
		"write_file_batch": {"files"},
		"read_file":        {"path"},
		"delete_file":      {"path"},
		"delete_dir":       {"path"},
		"move_file":        {"source", "destination"},
		"list_dir":         {"path"},
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
