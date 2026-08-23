package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// IntentCompiler synthesizes Go/C source code from intent descriptions.
// Unlike a static template engine, it combines:
// 1. Pattern Catalog (reusable code patterns)
// 2. Research Context (best practices, API patterns)
// 3. Intent JSON (what the model describes the code should do)
// 4. Self-evolution (new patterns can be added at runtime)
type IntentCompiler struct {
	catalog *PatternCatalog
	builder *Builder
}

// IntentJSON is the structured intent description from the model.
type IntentJSON struct {
	Functions []IntentFunction `json:"functions"`
}

// IntentFunction describes one function/module to generate.
type IntentFunction struct {
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Type           string            `json:"type"`            // "daemon", "tool", "watchdog", "monitor"
	OutputPath     string            `json:"output_path"`     // e.g. "src/main.go"
	Config         map[string]string `json:"config"`          // Key-value config
	Logic          IntentLogic       `json:"logic"`           // What the code should do
	DataStructures []DataStructure   `json:"data_structures"` // Structs to generate
	PatternIDs     []string          `json:"pattern_ids"`     // Specific patterns to use (optional)
}

// IntentLogic describes the algorithmic behavior.
type IntentLogic struct {
	MainLoop     string    `json:"main_loop"`     // What the main loop does
	Triggers     []Trigger `json:"triggers"`      // Event triggers
	InitSteps    []string  `json:"init_steps"`    // Initialization steps
	CleanupSteps []string  `json:"cleanup_steps"` // Cleanup on exit
}

// Trigger describes an event-condition-action rule.
type Trigger struct {
	Condition string `json:"condition"` // e.g. "temperature > 55"
	Action    string `json:"action"`    // e.g. "log warning + reduce CPU"
}

// DataStructure describes a Go struct to generate.
type DataStructure struct {
	Name   string        `json:"name"`   // e.g. "BatteryStatus"
	Fields []StructField `json:"fields"` // Struct fields
}

// StructField is one field in a data structure.
type StructField struct {
	Name string `json:"name"` // e.g. "Temperature"
	Type string `json:"type"` // e.g. "float64"
	Tag  string `json:"tag"`  // e.g. "json:\"temperature\""
}

// NewIntentCompiler creates a new Intent Compiler.
func NewIntentCompiler(b *Builder) *IntentCompiler {
	return &IntentCompiler{
		catalog: NewPatternCatalog(),
		builder: b,
	}
}

// CompileIntent converts intent JSON + research context into source files.
func (ic *IntentCompiler) CompileIntent(
	ctx context.Context,
	intent IntentJSON,
	research *ResearchContext,
	projectDir string,
	logFn func(string) error,
) ([]CompiledFile, error) {
	var files []CompiledFile

	for _, fn := range intent.Functions {
		logFn(fmt.Sprintf("    📦 Synthesizing %s (%s)...\n", fn.Name, fn.Type))

		ext := filepath.Ext(fn.OutputPath)
		var code string
		var err error

		switch ext {
		case ".go":
			code, err = ic.synthesizeGo(ctx, fn, research, logFn)
		case ".c", ".h":
			code, err = ic.synthesizeC(ctx, fn, research, logFn)
		default:
			err = fmt.Errorf("unsupported file type: %s", ext)
		}

		if err != nil {
			logFn(fmt.Sprintf("    ❌ Synthesis failed for %s: %v\n", fn.Name, err))
			continue
		}

		files = append(files, CompiledFile{
			Path:    fn.OutputPath,
			Content: code,
		})
		logFn(fmt.Sprintf("    ✅ Synthesized %s (%d bytes)\n", fn.OutputPath, len(code)))
	}

	return files, nil
}

// CompiledFile is a synthesized source file.
type CompiledFile struct {
	Path    string
	Content string
}

// ═══════════════════════════════════════════════════════
// GO SYNTHESIS
// ═══════════════════════════════════════════════════════

func (ic *IntentCompiler) synthesizeGo(ctx context.Context, fn IntentFunction, research *ResearchContext, logFn func(string) error) (string, error) {
	var sb strings.Builder

	// 1. Package declaration
	sb.WriteString("package main\n\n")

	// 2. Imports - collect from patterns + research
	imports := ic.collectGoImports(fn, research)
	if len(imports) > 0 {
		sb.WriteString("import (\n")
		for _, imp := range imports {
			sb.WriteString(fmt.Sprintf("\t\"%s\"\n", imp))
		}
		sb.WriteString(")\n\n")
	}

	// 3. Data structures
	for _, ds := range fn.DataStructures {
		code := ic.generateGoStruct(ds)
		sb.WriteString(code)
		sb.WriteString("\n")
	}

	// 4. Helper functions - from pattern catalog
	helpers := ic.selectGoHelpers(fn, research)
	for _, h := range helpers {
		sb.WriteString(h)
		sb.WriteString("\n\n")
	}

	// 5. Main function
	mainCode := ic.generateGoMain(fn, research)
	sb.WriteString(mainCode)

	return sb.String(), nil
}

func (ic *IntentCompiler) collectGoImports(fn IntentFunction, research *ResearchContext) []string {
	importSet := make(map[string]bool)
	importOrder := []string{}

	addImport := func(imp string) {
		if !importSet[imp] {
			importSet[imp] = true
			importOrder = append(importOrder, imp)
		}
	}

	// Always needed for daemons
	if fn.Type == "daemon" || fn.Type == "monitor" || fn.Type == "service" {
		addImport("context")
		addImport("log")
		addImport("os")
		addImport("os/signal")
		addImport("syscall")
		addImport("time")
	}

	// Config loading
	if len(fn.Config) > 0 {
		addImport("encoding/json")
	}

	// Sysfs reading
	if strings.Contains(fn.Description, "sysfs") || strings.Contains(fn.Description, "/sys/") ||
		strings.Contains(fn.Description, "thermal") || strings.Contains(fn.Description, "battery") ||
		strings.Contains(fn.Description, "cpu") || strings.Contains(fn.Description, "温度") ||
		strings.Contains(fn.Description, "电池") {
		addImport("os")
		addImport("strconv")
		addImport("strings")
	}

	// File operations
	if strings.Contains(fn.Description, "file") || strings.Contains(fn.Description, "log") ||
		strings.Contains(fn.Description, "文件") || strings.Contains(fn.Description, "日志") {
		addImport("fmt")
		addImport("os")
	}

	// Math
	if strings.Contains(fn.Description, "math") || strings.Contains(fn.Description, "计算") {
		addImport("math")
	}

	return importOrder
}

func (ic *IntentCompiler) generateGoStruct(ds DataStructure) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("type %s struct {\n", ds.Name))
	for _, field := range ds.Fields {
		jsonTag := field.Tag
		if jsonTag == "" {
			jsonTag = fmt.Sprintf("`json:\"%s\"`", toSnakeCase(field.Name))
		}
		sb.WriteString(fmt.Sprintf("\t%s %s %s\n", field.Name, field.Type, jsonTag))
	}
	sb.WriteString("}\n")
	return sb.String()
}

func (ic *IntentCompiler) selectGoHelpers(fn IntentFunction, research *ResearchContext) []string {
	var helpers []string

	// Always add sysfs reader if reading system files
	if needsSysfs(fn) {
		helpers = append(helpers, `// Read integer value from sysfs
func readSysfsInt(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	val, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return val, nil
}

// Read string value from sysfs
func readSysfsString(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}`)
	}

	// Add config loader if config is specified
	if len(fn.Config) > 0 {
		helpers = append(helpers, fmt.Sprintf(`// Default configuration
var defaultConfig = %s{
	%s
}

func loadConfig(path string) (%s, error) {
	cfg := defaultConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}`, fn.DataStructures[0].Name, generateDefaultValues(fn), fn.DataStructures[0].Name))
	}

	return helpers
}

func (ic *IntentCompiler) generateGoMain(fn IntentFunction, research *ResearchContext) string {
	var sb strings.Builder

	sb.WriteString("func main() {\n")
	sb.WriteString("\tlog.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)\n")
	sb.WriteString(fmt.Sprintf("\tlog.Printf(\"%s starting...\\n\")\n", fn.Name))

	// Graceful shutdown
	sb.WriteString("\tctx, stop := signal.NotifyContext(context.Background(),\n")
	sb.WriteString("\t\tsyscall.SIGTERM, syscall.SIGINT)\n")
	sb.WriteString("\tdefer stop()\n\n")

	// Config loading
	if len(fn.Config) > 0 && len(fn.DataStructures) > 0 {
		cfgPath := fn.Config["config_path"]
		if cfgPath == "" {
			cfgPath = "/data/adb/modules/" + fn.Name + "/config.json"
		}
		sb.WriteString(fmt.Sprintf("\tcfg, err := loadConfig(\"%s\")\n", cfgPath))
		sb.WriteString("\tif err != nil {\n")
		sb.WriteString("\t\tlog.Printf(\"Warning: using default config (%v)\", err)\n")
		sb.WriteString("\t}\n\n")
	}

	// Init steps
	for _, step := range fn.Logic.InitSteps {
		sb.WriteString(fmt.Sprintf("\t// %s\n", step))
	}
	if len(fn.Logic.InitSteps) > 0 {
		sb.WriteString("\n")
	}

	// Main loop (periodic check)
	sb.WriteString("\tticker := time.NewTicker(30 * time.Second)\n")
	sb.WriteString("\tdefer ticker.Stop()\n\n")

	sb.WriteString("\tfor {\n")
	sb.WriteString("\t\tselect {\n")
	sb.WriteString("\t\tcase <-ctx.Done():\n")

	// Cleanup steps
	for _, step := range fn.Logic.CleanupSteps {
		sb.WriteString(fmt.Sprintf("\t\t\t// %s\n", step))
	}
	sb.WriteString(fmt.Sprintf("\t\t\tlog.Printf(\"%s stopped\\n\")\n", fn.Name))
	sb.WriteString("\t\t\treturn\n")
	sb.WriteString("\t\tcase <-ticker.C:\n")

	// Main loop body
	sb.WriteString(fmt.Sprintf("\t\t\t// %s\n", fn.Logic.MainLoop))
	sb.WriteString("\t\t\tcheckOnce()\n")

	sb.WriteString("\t\t}\n")
	sb.WriteString("\t}\n")
	sb.WriteString("}\n\n")

	// checkOnce function
	sb.WriteString("func checkOnce() {\n")
	for _, trigger := range fn.Logic.Triggers {
		sb.WriteString(fmt.Sprintf("\t// %s\n", trigger.Condition))
		sb.WriteString(fmt.Sprintf("\t// Action: %s\n", trigger.Action))
	}
	sb.WriteString("}\n")

	return sb.String()
}

// ═══════════════════════════════════════════════════════
// C SYNTHESIS
// ═══════════════════════════════════════════════════════

func (ic *IntentCompiler) synthesizeC(ctx context.Context, fn IntentFunction, research *ResearchContext, logFn func(string) error) (string, error) {
	var sb strings.Builder

	// Standard C includes for Android/Magisk modules
	sb.WriteString(`#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <signal.h>
#include <time.h>
`)

	// Conditional includes
	if needsFileIO(fn) {
		sb.WriteString("#include <fcntl.h>\n")
	}
	sb.WriteString("\n")

	// Global state
	sb.WriteString("static volatile int running = 1;\n\n")

	// Signal handler
	sb.WriteString("static void signal_handler(int sig) {\n")
	sb.WriteString("\t(void)sig;\n")
	sb.WriteString("\trunning = 0;\n")
	sb.WriteString("}\n\n")

	// Helper functions
	if needsSysfs(fn) {
		sb.WriteString(`static int read_file_int(const char *path) {
	FILE *f = fopen(path, "r");
	if (!f) return -1;
	int val = 0;
	fscanf(f, "%d", &val);
	fclose(f);
	return val;
}

`)
	}

	// Main function
	sb.WriteString("int main(int argc, char *argv[]) {\n")
	sb.WriteString("\t(void)argc;\n")
	sb.WriteString("\t(void)argv;\n\n")
	sb.WriteString("\tsignal(SIGTERM, signal_handler);\n")
	sb.WriteString("\tsignal(SIGINT, signal_handler);\n\n")
	sb.WriteString(fmt.Sprintf("\tprintf(\"%s started\\n\");\n", fn.Name))

	// Init
	for _, step := range fn.Logic.InitSteps {
		sb.WriteString(fmt.Sprintf("\t// %s\n", step))
	}
	sb.WriteString("\n")

	// Main loop
	sb.WriteString("\twhile (running) {\n")
	for _, trigger := range fn.Logic.Triggers {
		sb.WriteString(fmt.Sprintf("\t\t// %s\n", trigger.Condition))
		sb.WriteString(fmt.Sprintf("\t\t// Action: %s\n", trigger.Action))
	}
	sb.WriteString("\t\tsleep(30);\n")
	sb.WriteString("\t}\n\n")

	// Cleanup
	for _, step := range fn.Logic.CleanupSteps {
		sb.WriteString(fmt.Sprintf("\t// %s\n", step))
	}
	sb.WriteString(fmt.Sprintf("\tprintf(\"%s stopped\\n\");\n", fn.Name))
	sb.WriteString("\treturn 0;\n")
	sb.WriteString("}\n")

	return sb.String(), nil
}

// ═══════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════

func needsSysfs(fn IntentFunction) bool {
	desc := strings.ToLower(fn.Description + " " + fn.Logic.MainLoop)
	for _, t := range fn.Logic.Triggers {
		desc += " " + t.Condition + " " + t.Action
	}
	return strings.Contains(desc, "/sys/") || strings.Contains(desc, "thermal") ||
		strings.Contains(desc, "battery") || strings.Contains(desc, "cpu") ||
		strings.Contains(desc, "温度") || strings.Contains(desc, "电池") ||
		strings.Contains(desc, "sysfs")
}

func needsFileIO(fn IntentFunction) bool {
	desc := strings.ToLower(fn.Description + " " + fn.Logic.MainLoop)
	return strings.Contains(desc, "file") || strings.Contains(desc, "log") ||
		strings.Contains(desc, "文件") || strings.Contains(desc, "日志")
}

func generateDefaultValues(fn IntentFunction) string {
	if len(fn.DataStructures) == 0 {
		return ""
	}
	var parts []string
	for _, field := range fn.DataStructures[0].Fields {
		switch field.Type {
		case "int":
			parts = append(parts, fmt.Sprintf("\t\t%s: 0,", field.Name))
		case "float64":
			parts = append(parts, fmt.Sprintf("\t\t%s: 0.0,", field.Name))
		case "string":
			parts = append(parts, fmt.Sprintf("\t\t%s: \"\",", field.Name))
		case "bool":
			parts = append(parts, fmt.Sprintf("\t\t%s: false,", field.Name))
		default:
			parts = append(parts, fmt.Sprintf("\t\t%s: 0,", field.Name))
		}
	}
	return strings.Join(parts, "\n")
}

func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}

// ═══════════════════════════════════════════════════════
// LLM-BASED INTENT GENERATION (Stage 2)
// ═══════════════════════════════════════════════════════

// GenerateIntentFromLLM uses LLM to generate intent JSON from description + research.
// This is the key function that replaces direct code generation.
func (ic *IntentCompiler) GenerateIntentFromLLM(
	ctx context.Context,
	description string,
	planJSON string,
	research *ResearchContext,
	fileInfo StageFileInfo,
	logFn func(string),
) (IntentJSON, error) {

	// Build research context string
	researchStr := ""
	if research != nil {
		researchStr = fmt.Sprintf(`

## Best Practices (FOLLOW THESE)
%s

## API Patterns (USE THESE)
%s

## Anti-Patterns (AVOID THESE)
%s

## Design Patterns (APPLY THESE)
%s`,
			strings.Join(research.BestPractices, "\n- "),
			formatAPIPatterns(research.APIPatterns),
			strings.Join(research.AntiPatterns, "\n- "),
			formatDesignPatterns(research.DesignPatterns),
		)
	}

	// Detect what type of code to generate
	isGo := strings.HasSuffix(fileInfo.Path, ".go")
	isC := strings.HasSuffix(fileInfo.Path, ".c") || strings.HasSuffix(fileInfo.Path, ".h")

	languageGuide := ""
	structGuide := ""
	if isGo {
		languageGuide = `Generate Go code intent. Use ONLY standard library (no cgo, no third-party).
Types: int, int64, string, bool, float64, []byte, error, map[string]interface{}, []StructType.
All variables must be initialized. All errors must be handled.`
		structGuide = `Define data structures as JSON objects with "name", "type" (Go type), "tag" (json tag).`
	} else if isC {
		languageGuide = `Generate C code intent. Use POSIX API only (no Android-specific headers).
Declare ALL variables before use (C89 style). Initialize ALL variables.`
		structGuide = `C structs not needed for simple cases — describe fields in config instead.`
	}

	prompt := fmt.Sprintf(`You are a code architect. Convert a module requirement into a STRUCTURED INTENT description.

DO NOT write actual source code. Instead, describe WHAT the code should do in structured JSON.

## Module Requirement
%s

## File to generate
Path: %s
Purpose: %s
%s
%s

%s

## OUTPUT FORMAT (valid JSON only, no markdown fences)
{
  "functions": [
    {
      "name": "module_daemon",
      "description": "what this function/module does",
      "type": "daemon|tool|watchdog|monitor",
      "output_path": "%s",
      "config": {
        "config_path": "/data/adb/modules/<id>/config.json",
        "check_interval_seconds": "300"
      },
      "data_structures": [
        {
          "name": "ModuleConfig",
          "fields": [
            {"name": "Interval", "type": "int", "tag": "`+"`"+`json:\"interval\"`+"`"+`"},
            {"name": "Threshold", "type": "float64", "tag": "`+"`"+`json:\"threshold\"`+"`"+`"}
          ]
        }
      ],
      "logic": {
        "init_steps": ["Load config from file", "Validate thresholds"],
        "main_loop": "Read sensor values, compare with thresholds, trigger actions",
        "triggers": [
          {"condition": "temperature > threshold", "action": "log warning and take action"},
          {"condition": "value normal", "action": "reset counters"}
        ],
        "cleanup_steps": ["Log shutdown", "Release resources"]
      }
    }
  ]
}

## RULES
- Describe logic in PLAIN ENGLISH, not code
- Config values as strings (the synthesizer handles type conversion)
- Data structures use Go types (int, float64, string, bool)
- Triggers should be clear condition-action pairs
- Output ONLY valid JSON, nothing else
- Do NOT include code snippets in the JSON`,
		description, fileInfo.Path, fileInfo.Description,
		languageGuide, structGuide, researchStr, fileInfo.Path,
	)

	endpoint, apiKey, model := ic.builder.resolveLLMForFix()
	if endpoint == "" || apiKey == "" {
		return IntentJSON{}, fmt.Errorf("no LLM configured for intent generation")
	}

	response, err := callLLMForFix(ctx, endpoint, apiKey, model, prompt)
	if err != nil {
		return IntentJSON{}, fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse intent JSON
	response = extractJSONFromResponse(response)
	var intent IntentJSON
	if err := json.Unmarshal([]byte(response), &intent); err != nil {
		// Try to fix common JSON issues
		response = fixIntentJSON(response)
		if err2 := json.Unmarshal([]byte(response), &intent); err2 != nil {
			return IntentJSON{}, fmt.Errorf("failed to parse intent JSON: %w (response: %s)", err2, truncate(response, 200))
		}
	}

	return intent, nil
}

// formatAPIPatterns formats API patterns for the prompt.
func formatAPIPatterns(patterns []APIPattern) string {
	var sb strings.Builder
	for _, p := range patterns {
		sb.WriteString(fmt.Sprintf("- %s: %s\n  Go: %s\n", p.Name, p.Description, p.GoSnippet))
	}
	return sb.String()
}

// formatDesignPatterns formats design patterns for the prompt.
func formatDesignPatterns(patterns []DesignPattern) string {
	var sb strings.Builder
	for _, p := range patterns {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", p.Name, p.Description))
	}
	return sb.String()
}

// fixIntentJSON attempts to fix common JSON issues from LLM output.
func fixIntentJSON(s string) string {
	// Remove markdown fences
	s = strings.TrimPrefix(s, "```json\n")
	s = strings.TrimPrefix(s, "```\n")
	s = strings.TrimSuffix(s, "\n```")
	s = strings.TrimSpace(s)

	// Fix trailing commas before }
	re := regexp.MustCompile(`,\s*}`)
	s = re.ReplaceAllString(s, "}")

	// Fix trailing commas before ]
	re2 := regexp.MustCompile(`,\s*\]`)
	s = re2.ReplaceAllString(s, "]")

	return s
}

// truncate truncates a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// LoadIntentJSON loads intent JSON from a file.
func LoadIntentJSON(path string) (IntentJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return IntentJSON{}, err
	}
	var intent IntentJSON
	if err := json.Unmarshal(data, &intent); err != nil {
		return IntentJSON{}, err
	}
	return intent, nil
}
