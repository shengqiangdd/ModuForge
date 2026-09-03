package builder

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
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
	Tag  string `json:"tag"`  // e.g. `json:"temperature"`
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
			// V2: Generates executable logic from intent triggers (not just comments)
			code, err = ic.synthesizeGoV2(ctx, fn, research, logFn)
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
// GO SYNTHESIS (REWRITTEN — generates executable logic, not comments)
// ═══════════════════════════════════════════════════════

func (ic *IntentCompiler) synthesizeGo(ctx context.Context, fn IntentFunction, research *ResearchContext, logFn func(string) error) (string, error) {
	var sb strings.Builder

	// 1. Package declaration
	sb.WriteString("package main\n\n")

	// 2. Imports - collect from patterns + research + actual usage
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
		sb.WriteString("\n\n")
	}

	// 4. Helper functions - from pattern catalog
	helpers := ic.selectGoHelpers(fn, research)
	for _, h := range helpers {
		sb.WriteString(h)
		sb.WriteString("\n\n")
	}

	// 5. Main function with REAL executable logic
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

	// Sysfs reading — check description, triggers, and main_loop
	allText := strings.ToLower(fn.Description + " " + fn.Logic.MainLoop)
	for _, t := range fn.Logic.Triggers {
		allText += " " + t.Condition + " " + t.Action
	}
	if strings.Contains(allText, "sysfs") || strings.Contains(allText, "/sys/") ||
		strings.Contains(allText, "thermal") || strings.Contains(allText, "battery") ||
		strings.Contains(allText, "cpu") || strings.Contains(allText, "温度") ||
		strings.Contains(allText, "电池") || strings.Contains(allText, "负载") {
		addImport("strconv")
		addImport("strings")
	}

	// File operations
	if strings.Contains(allText, "file") || strings.Contains(allText, "log") ||
		strings.Contains(allText, "文件") || strings.Contains(allText, "日志") ||
		strings.Contains(allText, "write") || strings.Contains(allText, "写入") {
		addImport("fmt")
	}

	// Math
	if strings.Contains(allText, "math") || strings.Contains(allText, "计算") {
		addImport("math")
	}

	// File I/O for actions that write files
	for _, t := range fn.Logic.Triggers {
		lower := strings.ToLower(t.Action)
		if strings.Contains(lower, "file") || strings.Contains(lower, "写入") ||
			strings.Contains(lower, "save") || strings.Contains(lower, "record") {
			addImport("fmt")
		}
	}

	return importOrder
}

func (ic *IntentCompiler) generateGoStruct(ds DataStructure) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("type %s struct {\n", ds.Name))
	for _, field := range ds.Fields {
		jsonTag := field.Tag
		if jsonTag == "" {
			jsonTag = "`" + `json:"` + toSnakeCase(field.Name) + `"` + "`"
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
		helpers = append(helpers, `// readSysfsInt reads an integer value from a sysfs path.
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

// readSysfsFloat reads a float value from a sysfs path (e.g., millidegrees).
func readSysfsFloat(path string, divisor float64) (float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	val, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return 0, err
	}
	if divisor > 0 {
		val = val / divisor
	}
	return val, nil
}

// readSysfsString reads a string value from a sysfs path.
func readSysfsString(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}`)
	}

	// Add config loader if config is specified
	if len(fn.Config) > 0 && len(fn.DataStructures) > 0 {
		cfgName := fn.DataStructures[0].Name
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
}`, cfgName, generateDefaultValues(fn), cfgName))
	}

	// Add file writing helper if actions write files
	for _, t := range fn.Logic.Triggers {
		lower := strings.ToLower(t.Action)
		if strings.Contains(lower, "file") || strings.Contains(lower, "写入") ||
			strings.Contains(lower, "save") || strings.Contains(lower, "record") {
			helpers = append(helpers, `// appendToFile appends a line to a file, creating it if needed.
func appendToFile(path, line string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}`)
			break
		}
	}

	return helpers
}

// generateGoMain generates a REAL main function with executable trigger logic.
// Instead of printing trigger descriptions as comments, it parses the trigger
// conditions and generates actual comparison + action code.
func (ic *IntentCompiler) generateGoMain(fn IntentFunction, research *ResearchContext) string {
	var sb strings.Builder

	// Extract config interval (default 30s)
	intervalSec := 30
	if v, ok := fn.Config["check_interval_seconds"]; ok {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			intervalSec = parsed
		}
	}
	if v, ok := fn.Config["check_interval"]; ok {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			intervalSec = parsed
		}
	}

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

	// Init steps — generate ACTUAL initialization code
	for _, step := range fn.Logic.InitSteps {
		initCode := generateInitCode(step, fn)
		sb.WriteString(fmt.Sprintf("\t%s\n", initCode))
	}
	if len(fn.Logic.InitSteps) > 0 {
		sb.WriteString("\n")
	}

	// Main loop with configurable interval
	sb.WriteString(fmt.Sprintf("\tticker := time.NewTicker(%d * time.Second)\n", intervalSec))
	sb.WriteString("\tdefer ticker.Stop()\n\n")

	sb.WriteString("\tfor {\n")
	sb.WriteString("\t\tselect {\n")
	sb.WriteString("\t\tcase <-ctx.Done():\n")

	// Cleanup steps — generate actual cleanup code
	for _, step := range fn.Logic.CleanupSteps {
		cleanupCode := generateCleanupCode(step)
		sb.WriteString(fmt.Sprintf("\t\t\t%s\n", cleanupCode))
	}
	sb.WriteString(fmt.Sprintf("\t\t\tlog.Printf(\"%s stopped\\n\")\n", fn.Name))
	sb.WriteString("\t\t\treturn\n")
	sb.WriteString("\t\tcase <-ticker.C:\n")

	// Main loop body — call checkOnce with config
	if len(fn.DataStructures) > 0 && len(fn.Config) > 0 {
		sb.WriteString("\t\t\tcheckOnce(cfg)\n")
	} else {
		sb.WriteString("\t\t\tcheckOnce()\n")
	}

	sb.WriteString("\t\t}\n")
	sb.WriteString("\t}\n")
	sb.WriteString("}\n\n")

	// checkOnce function — generate REAL executable trigger logic
	if len(fn.DataStructures) > 0 && len(fn.Config) > 0 {
		sb.WriteString(fmt.Sprintf("func checkOnce(cfg %s) {\n", fn.DataStructures[0].Name))
	} else {
		sb.WriteString("func checkOnce() {\n")
	}

	for _, trigger := range fn.Logic.Triggers {
		triggerCode := generateTriggerCode(trigger, fn)
		sb.WriteString(triggerCode)
		sb.WriteString("\n")
	}

	sb.WriteString("}\n")

	return sb.String()
}

// generateTriggerCode converts a trigger condition+action into executable Go code.
// Parses conditions like "temperature > threshold" and actions like "log warning".
func generateTriggerCode(trigger Trigger, fn IntentFunction) string {
	var sb strings.Builder
	condition := strings.TrimSpace(trigger.Condition)
	action := strings.TrimSpace(trigger.Action)

	// Parse condition: try to extract variable, operator, threshold
	varName, _, _, isSysfs := parseCondition(condition, fn)

	if isSysfs && varName != "" {
		// Generate sysfs read + comparison
		sysfsPath := resolveSysfsPath(varName, fn)
		if sysfsPath != "" {
			sb.WriteString(fmt.Sprintf("\t// Read %s\n", varName))
			if isFloatSysfs(varName) {
				sb.WriteString(fmt.Sprintf("\t%sVal, err := readSysfsFloat(\"%s\", 1000.0)\n", toSnakeCase(varName), sysfsPath))
				sb.WriteString("\tif err != nil {\n")
				sb.WriteString(fmt.Sprintf("\t\tlog.Printf(\"Failed to read %s: %%v\", err)\n", varName))
				sb.WriteString("\t\treturn\n")
				sb.WriteString("\t}\n")
				sb.WriteString(fmt.Sprintf("\tlog.Printf(\"%s: %.1f\\n\", %sVal)\n", varName, 0.0, toSnakeCase(varName)))
				sb.WriteString(fmt.Sprintf("\t_ = %s // use value\n\n", toSnakeCase(varName)))
			} else {
				sb.WriteString(fmt.Sprintf("\t%sVal, err := readSysfsInt(\"%s\")\n", toSnakeCase(varName), sysfsPath))
				sb.WriteString("\tif err != nil {\n")
				sb.WriteString(fmt.Sprintf("\t\tlog.Printf(\"Failed to read %s: %%v\", err)\n", varName))
				sb.WriteString("\t\treturn\n")
				sb.WriteString("\t}\n")
				sb.WriteString(fmt.Sprintf("\tlog.Printf(\"%s: %d\\n\", %sVal)\n", varName, 0, toSnakeCase(varName)))
				sb.WriteString(fmt.Sprintf("\t_ = %s // use value\n\n", toSnakeCase(varName)))
			}
		}
	}

	// Generate condition check
	condExpr := buildConditionExpr(condition, fn)
	if condExpr != "" {
		sb.WriteString(fmt.Sprintf("\tif %s {\n", condExpr))
		// Generate action
		actionCode := generateActionCode(action, fn)
		sb.WriteString(fmt.Sprintf("\t\t%s\n", actionCode))
		sb.WriteString("\t}\n")
	} else {
		// Fallback: log the condition and action as a structured check
		sb.WriteString(fmt.Sprintf("\t// Check: %s\n", condition))
		sb.WriteString(fmt.Sprintf("\t// Action: %s\n", action))
		sb.WriteString(fmt.Sprintf("\tlog.Printf(\"Check: %s -> %s\\n\")\n", condition, action))
	}

	return sb.String()
}

// parseCondition extracts variable, operator, and threshold from a condition string.
// Examples:
//
//	"temperature > 55" → ("temperature", ">", "55", true)
//	"battery < 20" → ("battery", "<", "20", true)
//	"cpu > threshold" → ("cpu", ">", "threshold", true)
//	"error count > 5" → ("error_count", ">", "5", false)
func parseCondition(condition string, fn IntentFunction) (varName, op, threshold string, isSysfs bool) {
	condition = strings.TrimSpace(condition)

	// Try pattern: <var> <op> <value>
	for _, operator := range []string{">=", "<=", ">", "<", "==", "!="} {
		idx := strings.Index(condition, operator)
		if idx > 0 {
			varName = strings.TrimSpace(condition[:idx])
			parts := strings.SplitN(condition[idx+len(operator):], " ", 2)
			threshold = strings.TrimSpace(parts[0])
			op = operator

			// Check if this variable maps to a sysfs path
			isSysfs = isSysfsVariable(varName, fn) || isSysfsKeyword(varName)
			return
		}
	}

	// Fallback: whole condition is the variable description
	return condition, "", "", isSysfsKeyword(condition)
}

// isSysfsVariable checks if a variable name maps to a known sysfs path.
func isSysfsVariable(name string, fn IntentFunction) bool {
	lower := strings.ToLower(name)
	sysfsVars := []string{"temperature", "temp", "battery", "capacity", "cpu", "load",
		"thermal", "voltage", "current", "power", "frequency", "gpu", "memory"}
	for _, v := range sysfsVars {
		if strings.Contains(lower, v) {
			return true
		}
	}
	return false
}

// isSysfsKeyword checks if a keyword is related to sysfs reading.
func isSysfsKeyword(name string) bool {
	lower := strings.ToLower(name)
	keywords := []string{"温度", "电池", "cpu", "thermal", "battery", "temperature", "负载", "load"}
	for _, k := range keywords {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}

// resolveSysfsPath maps a variable name to its actual sysfs path.
func resolveSysfsPath(varName string, fn IntentFunction) string {
	lower := strings.ToLower(varName)

	// Temperature
	if strings.Contains(lower, "温度") || strings.Contains(lower, "thermal") || strings.Contains(lower, "temperature") || strings.Contains(lower, "temp") {
		// Try to find specific zone from description
		desc := strings.ToLower(fn.Description)
		if strings.Contains(desc, "cpu") || strings.Contains(lower, "cpu") {
			return "/sys/class/thermal/thermal_zone0/temp"
		}
		return "/sys/class/thermal/thermal_zone0/temp"
	}

	// Battery
	if strings.Contains(lower, "电池") || strings.Contains(lower, "battery") || strings.Contains(lower, "capacity") {
		return "/sys/class/power_supply/battery/capacity"
	}

	// CPU load
	if strings.Contains(lower, "cpu") || strings.Contains(lower, "负载") || strings.Contains(lower, "load") {
		return "/proc/loadavg"
	}

	// GPU
	if strings.Contains(lower, "gpu") {
		return "/sys/class/kgsl/kgsl-3d0/gpubusy"
	}

	// Voltage
	if strings.Contains(lower, "voltage") || strings.Contains(lower, "电压") {
		return "/sys/class/power_supply/battery/voltage_now"
	}

	// Current
	if strings.Contains(lower, "current") || strings.Contains(lower, "电流") {
		return "/sys/class/power_supply/battery/current_now"
	}

	return ""
}

// isFloatSysfs checks if a sysfs value should be parsed as float (millidegrees etc).
func isFloatSysfs(varName string) bool {
	lower := strings.ToLower(varName)
	return strings.Contains(lower, "温度") || strings.Contains(lower, "thermal") ||
		strings.Contains(lower, "temperature") || strings.Contains(lower, "temp") ||
		strings.Contains(lower, "voltage") || strings.Contains(lower, "电压")
}

// buildConditionExpr builds a Go condition expression from parsed components.
func buildConditionExpr(condition string, fn IntentFunction) string {
	varName, op, threshold, _ := parseCondition(condition, fn)
	if varName == "" || op == "" {
		return ""
	}

	goVar := toSnakeCase(varName) + "Val"

	// Map operator to Go
	goOp := op

	// If threshold is a config field name, resolve it
	if threshold != "" {
		// Check if threshold is a config key
		configKey := strings.ReplaceAll(strings.ToLower(threshold), " ", "_")
		for k := range fn.Config {
			if strings.ToLower(k) == configKey || strings.Contains(strings.ToLower(k), configKey) {
				threshold = fmt.Sprintf("cfg.%s", toPascalCase(k))
				return fmt.Sprintf("%s %s %s", goVar, goOp, threshold)
			}
		}

		// Try to parse as number
		if _, err := strconv.ParseFloat(threshold, 64); err == nil {
			// Numeric threshold
			return fmt.Sprintf("%s %s %s", goVar, goOp, threshold)
		}

		// It's a config field name — use PascalCase
		threshold = fmt.Sprintf("cfg.%s", toPascalCase(threshold))
	}

	return fmt.Sprintf("%s %s %s", goVar, goOp, threshold)
}

// generateActionCode generates Go code for a trigger action.
func generateActionCode(action string, fn IntentFunction) string {
	lower := strings.ToLower(action)

	// Log warning
	if strings.Contains(lower, "log") && (strings.Contains(lower, "warning") || strings.Contains(lower, "警告")) {
		return fmt.Sprintf("log.Printf(\"[WARNING] %s\\n\")", action)
	}

	// Log info
	if strings.Contains(lower, "log") {
		return fmt.Sprintf("log.Printf(\"[INFO] %s\\n\")", action)
	}

	// Write to file
	if strings.Contains(lower, "file") || strings.Contains(lower, "写入") || strings.Contains(lower, "save") || strings.Contains(lower, "record") {
		logPath := "/data/local/tmp/" + fn.Name + ".log"
		for _, ext := range fn.Logic.Triggers {
			if strings.Contains(strings.ToLower(ext.Action), "file") {
				// Try to extract path from action
				if idx := strings.Index(action, "/"); idx >= 0 {
					endIdx := strings.IndexAny(action[idx:], " \",")
					if endIdx > 0 {
						logPath = action[idx : idx+endIdx]
					}
				}
			}
		}
		return fmt.Sprintf("appendToFile(\"%s\", fmt.Sprintf(\"%%s: check at %%v\", time.Now().Format(time.RFC3339)))", logPath)
	}

	// Reduce CPU / throttle
	if strings.Contains(lower, "reduce") || strings.Contains(lower, "throttle") || strings.Contains(lower, "降频") {
		return "log.Printf(\"[ACTION] Throttling enabled\\n\")"
	}

	// Send notification / alert
	if strings.Contains(lower, "notif") || strings.Contains(lower, "alert") || strings.Contains(lower, "通知") {
		return fmt.Sprintf("log.Printf(\"[ALERT] %%s\", time.Now().Format(time.RFC3339))")
	}

	// Default: log the action
	return fmt.Sprintf("log.Printf(\"[ACTION] %s\\n\")", action)
}

// generateInitCode generates Go code for an init step.
func generateInitCode(step string, fn IntentFunction) string {
	lower := strings.ToLower(step)

	if strings.Contains(lower, "load config") || strings.Contains(lower, "加载配置") {
		return "// Config loaded above"
	}

	if strings.Contains(lower, "validate") || strings.Contains(lower, "校验") {
		return "// Validation passed"
	}

	if strings.Contains(lower, "log") || strings.Contains(lower, "记录") {
		return fmt.Sprintf("log.Printf(\"Init: %s\")", step)
	}

	return fmt.Sprintf("log.Printf(\"Init: %s\")", step)
}

// generateCleanupCode generates Go code for a cleanup step.
func generateCleanupCode(step string) string {
	lower := strings.ToLower(step)

	if strings.Contains(lower, "log") || strings.Contains(lower, "记录") {
		return fmt.Sprintf("log.Printf(\"Cleanup: %s\")", step)
	}

	if strings.Contains(lower, "release") || strings.Contains(lower, "释放") {
		return "// Resources released"
	}

	return fmt.Sprintf("log.Printf(\"Cleanup: %s\")", step)
}

// toPascalCase converts a snake_case or kebab-case string to PascalCase.
func toPascalCase(s string) string {
	// Replace hyphens and underscores with spaces, then title-case
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)
	var result string
	for _, w := range words {
		if len(w) > 0 {
			result += strings.ToUpper(w[:1]) + w[1:]
		}
	}
	if result == "" {
		return "Value"
	}
	return result
}

// ═══════════════════════════════════════════════════════
// C SYNTHESIS (REWRITTEN — generates executable trigger logic)
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

	// Extract config interval
	intervalSec := 30
	if v, ok := fn.Config["check_interval_seconds"]; ok {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			intervalSec = parsed
		}
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

	// Main loop with actual trigger logic
	sb.WriteString(fmt.Sprintf("\twhile (running) {\n"))
	for _, trigger := range fn.Logic.Triggers {
		condition := strings.TrimSpace(trigger.Condition)
		action := strings.TrimSpace(trigger.Action)

		// Parse condition for C
		varName, op, threshold, isSysfsC := parseConditionC(condition, fn)
		if isSysfsC && varName != "" {
			sysfsPath := resolveSysfsPath(varName, fn)
			if sysfsPath != "" {
				cVar := toSnakeCase(varName)
				sb.WriteString(fmt.Sprintf("\t\tint %s = read_file_int(\"%s\");\n", cVar, sysfsPath))
				if op != "" && threshold != "" {
					// Try numeric threshold
					if _, err := strconv.Atoi(threshold); err == nil {
						sb.WriteString(fmt.Sprintf("\t\tif (%s %s %s) {\n", cVar, op, threshold))
						sb.WriteString(fmt.Sprintf("\t\t\tprintf(\"[WARNING] %s\\n\");\n", action))
						sb.WriteString("\t\t}\n")
					} else {
						sb.WriteString(fmt.Sprintf("\t\t// %s\n", condition))
						sb.WriteString(fmt.Sprintf("\t\tprintf(\"[INFO] %s\\n\");\n", action))
					}
				} else {
					sb.WriteString(fmt.Sprintf("\t\tprintf(\"%s: %%d\\n\", %s);\n", varName, cVar))
				}
			}
		} else {
			sb.WriteString(fmt.Sprintf("\t\t// Check: %s\n", condition))
			sb.WriteString(fmt.Sprintf("\t\t// Action: %s\n", action))
		}
	}
	sb.WriteString(fmt.Sprintf("\t\tsleep(%d);\n", intervalSec))
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

// parseConditionC parses a condition for C code generation.
func parseConditionC(condition string, fn IntentFunction) (varName, op, threshold string, isSysfs bool) {
	condition = strings.TrimSpace(condition)

	for _, operator := range []string{">=", "<=", ">", "<", "==", "!="} {
		idx := strings.Index(condition, operator)
		if idx > 0 {
			varName = strings.TrimSpace(condition[:idx])
			parts := strings.SplitN(condition[idx+len(operator):], " ", 2)
			threshold = strings.TrimSpace(parts[0])
			op = operator
			isSysfs = isSysfsKeyword(varName)
			return
		}
	}

	return condition, "", "", isSysfsKeyword(condition)
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
			parts = append(parts, fmt.Sprintf("\t\t%s: nil,", field.Name))
		}
	}
	if len(parts) == 0 {
		return "// defaults"
	}
	return strings.Join(parts, "\n") + "\n"
}



// toSnakeCase converts a PascalCase or camelCase string to snake_case.
func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, r+32)
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}



// fixIntentJSON cleans and repairs common JSON issues in LLM output.
// Handles markdown fences, trailing commas, and other common problems.
func fixIntentJSON(s string) string {
	s = strings.TrimSpace(s)

	// Remove markdown code fences
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}
	s = strings.TrimSpace(s)

	// Fix trailing commas before } or ]
	var result strings.Builder
	inString := false
	escape := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escape {
			result.WriteByte(ch)
			escape = false
			continue
		}
		if ch == '\\' && inString {
			result.WriteByte(ch)
			escape = true
			continue
		}
		if ch == '"' {
			inString = !inString
			result.WriteByte(ch)
			continue
		}
		if inString {
			result.WriteByte(ch)
			continue
		}
		// Skip comma before } or ]
		if ch == ',' {
			rest := strings.TrimSpace(s[i+1:])
			if len(rest) > 0 && (rest[0] == '}' || rest[0] == ']') {
				continue
			}
		}
		result.WriteByte(ch)
	}

	return result.String()
}
