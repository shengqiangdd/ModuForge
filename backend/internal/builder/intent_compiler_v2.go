package builder

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ═══════════════════════════════════════════════════════
// V2 SYNTHESIS — Generates executable logic from intent
// ═══════════════════════════════════════════════════════

// synthesizeGoV2 replaces synthesizeGo with a version that generates
// ACTUAL executable code from intent triggers, config, and data structures.
// V1 only produced comments; V2 generates real condition-action logic.
func (ic *IntentCompiler) synthesizeGoV2(ctx context.Context, fn IntentFunction, research *ResearchContext, logFn func(string) error) (string, error) {
	var sb strings.Builder

	// 1. Package declaration
	sb.WriteString("package main\n\n")

	// 2. Imports — collect from intent analysis + research
	imports := ic.collectGoImportsV2(fn, research)
	sb.WriteString("import (\n")
	for _, imp := range imports {
		sb.WriteString(fmt.Sprintf("\t\"%s\"\n", imp))
	}
	sb.WriteString(")\n\n")

	// 3. Constants from config (interval, thresholds)
	if len(fn.Config) > 0 {
		sb.WriteString(generateConfigConstants(fn))
		sb.WriteString("\n")
	}

	// 4. Data structures
	for _, ds := range fn.DataStructures {
		sb.WriteString(ic.generateGoStruct(ds))
		sb.WriteString("\n")
	}

	// 5. Helper functions — sysfs reader, file writer, etc.
	helpers := ic.selectGoHelpersV2(fn, research)
	for _, h := range helpers {
		sb.WriteString(h)
		sb.WriteString("\n\n")
	}

	// 6. Global state (if needed)
	if needsLogging(fn) || needsFileIO(fn) {
		sb.WriteString(generateGlobalState(fn))
		sb.WriteString("\n")
	}

	// 7. Config loader
	if len(fn.Config) > 0 && len(fn.DataStructures) > 0 {
		sb.WriteString(generateConfigLoader(fn))
		sb.WriteString("\n")
	}

	// 8. checkOnce — THE CRITICAL PART: generates executable trigger logic
	sb.WriteString(generateCheckOnceV2(fn))
	sb.WriteString("\n")

	// 9. Main function with config-aware interval
	sb.WriteString(generateMainV2(fn))

	return sb.String(), nil
}

// ═══════════════════════════════════════════════════════
// CONFIG CONSTANTS
// ═══════════════════════════════════════════════════════

func generateConfigConstants(fn IntentFunction) string {
	var sb strings.Builder
	sb.WriteString("// Configuration defaults (overridden by config.json)\n")
	sb.WriteString("const (\n")

	// Extract interval from config
	interval := "30" // default 30 seconds
	if v, ok := fn.Config["check_interval_seconds"]; ok {
		if _, err := strconv.Atoi(v); err == nil {
			interval = v
		}
	}
	if v, ok := fn.Config["check_interval"]; ok {
		if _, err := strconv.Atoi(v); err == nil {
			interval = v
		}
	}
	sb.WriteString(fmt.Sprintf("\tdefaultInterval = %s\n", interval))

	// Extract thresholds from config
	for k, v := range fn.Config {
		if k == "config_path" || k == "check_interval_seconds" || k == "check_interval" {
			continue
		}
		// Try to parse as number for constants
		if intVal, err := strconv.Atoi(v); err == nil {
			constName := toConstName(k)
			sb.WriteString(fmt.Sprintf("\tdefault%s = %d\n", constName, intVal))
		} else if floatVal, err := strconv.ParseFloat(v, 64); err == nil {
			constName := toConstName(k)
			sb.WriteString(fmt.Sprintf("\tdefault%s = %.1f\n", constName, floatVal))
		}
	}

	sb.WriteString(")\n")
	return sb.String()
}

func toConstName(s string) string {
	// Convert snake_case to PascalCase
	parts := strings.Split(s, "_")
	var result strings.Builder
	for _, p := range parts {
		if len(p) > 0 {
			result.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return result.String()
}

// ═══════════════════════════════════════════════════════
// GLOBAL STATE
// ═══════════════════════════════════════════════════════

func generateGlobalState(fn IntentFunction) string {
	var sb strings.Builder

	if needsFileIO(fn) {
		logPath := "/data/adb/modules/" + fn.Name + "/activity.log"
		// Check config for custom log path
		if v, ok := fn.Config["log_path"]; ok && v != "" {
			logPath = v
		}
		sb.WriteString(fmt.Sprintf(`var logFile *os.File

func initLogging() error {
	var err error
	logFile, err = os.OpenFile("%s", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Warning: cannot open log file: %%v", err)
		return err
	}
	return nil
}

func logActivity(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Println(msg)
	if logFile != nil {
		logFile.WriteString(fmt.Sprintf("[%%s] %%s\\n", time.Now().Format("2006-01-02 15:04:05"), msg))
	}
}
`, logPath))
	}

	return sb.String()
}

// ═══════════════════════════════════════════════════════
// CONFIG LOADER
// ═══════════════════════════════════════════════════════

func generateConfigLoader(fn IntentFunction) string {
	if len(fn.DataStructures) == 0 {
		return ""
	}
	ds := fn.DataStructures[0]

	cfgPath := fn.Config["config_path"]
	if cfgPath == "" {
		cfgPath = "/data/adb/modules/" + fn.Name + "/config.json"
	}

	return fmt.Sprintf(`var defaultConfig = %s{
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
}

// Config path (can be overridden by first argument)
var configPath = "%s"
`, ds.Name, generateDefaultValues(fn), ds.Name, cfgPath)
}

// ═══════════════════════════════════════════════════════
// CHECK ONCE — Executable trigger logic (V2)
// ═══════════════════════════════════════════════════════

func generateCheckOnceV2(fn IntentFunction) string {
	var sb strings.Builder

	sb.WriteString("func checkOnce(cfg interface{}) {\n")

	// Type-assert config if we have data structures
	if len(fn.DataStructures) > 0 {
		sb.WriteString(fmt.Sprintf("\tc, ok := cfg.(%s)\n", fn.DataStructures[0].Name))
		sb.WriteString("\tif !ok {\n")
		sb.WriteString("\t\tlog.Printf(\"Warning: invalid config type\\n\")\n")
		sb.WriteString("\t\treturn\n")
		sb.WriteString("\t}\n")
		sb.WriteString("\t_ = c\n") // Will be used by generated trigger code
	}

	// Generate executable trigger logic
	for i, trigger := range fn.Logic.Triggers {
		conditionCode := generateConditionCode(trigger.Condition, fn)
		actionCode := generateActionCodeV2(trigger.Action, fn, i)

		sb.WriteString(fmt.Sprintf("\n\t// Trigger %d: %s\n", i+1, trigger.Condition))
		sb.WriteString(fmt.Sprintf("\tif %s {\n", conditionCode))
		sb.WriteString(fmt.Sprintf("\t\t%s\n", actionCode))
		sb.WriteString("\t}\n")
	}

	sb.WriteString("}\n")
	return sb.String()
}

// generateConditionCode converts a natural language condition to Go code.
// Examples:
//   "temperature > threshold" → "tempC > float64(c.Threshold)"
//   "battery level < 20" → "batteryLevel < 20"
//   "cpu load > 80" → "cpuLoad > 80"
func generateConditionCode(condition string, fn IntentFunction) string {
	// Pattern: X > threshold / X < threshold / X >= Y / X <= Y
	re := regexp.MustCompile(`(\w+)\s*(>|<|>=|<=|==|!=)\s*(\w+)`)
	matches := re.FindStringSubmatch(condition)

	if len(matches) == 4 {
		left := matches[1]
		op := matches[2]
		right := matches[3]

		// Map left side to Go variable
		leftVar := mapConditionVar(left, fn)
		// Map right side to Go expression
		rightExpr := mapConditionExpr(right, fn)

		return fmt.Sprintf("%s %s %s", leftVar, op, rightExpr)
	}

	// Fallback: use the condition as a comment and return true
	return fmt.Sprintf("/* %s */ true", condition)
}

// mapConditionVar maps a condition variable name to Go code.
func mapConditionVar(name string, fn IntentFunction) string {
	lower := strings.ToLower(name)

	// Temperature patterns
	if strings.Contains(lower, "temp") || strings.Contains(lower, "温度") {
		return `readSysfsInt("/sys/class/thermal/thermal_zone0/temp") / 1000`
	}

	// Battery patterns
	if strings.Contains(lower, "batter") || strings.Contains(lower, "电池") || strings.Contains(lower, "capacity") {
		return `readSysfsInt("/sys/class/power_supply/battery/capacity")`
	}

	// CPU patterns
	if strings.Contains(lower, "cpu") || strings.Contains(lower, "负载") || strings.Contains(lower, "load") {
		return `readCPUUsage()`
	}

	// Memory patterns
	if strings.Contains(lower, "mem") || strings.Contains(lower, "内存") {
		return `readMemUsage()`
	}

	// Network patterns
	if strings.Contains(lower, "net") || strings.Contains(lower, "网络") || strings.Contains(lower, "traffic") {
		return `readNetBytes()`
	}

	// Generic: try to read from sysfs
	return fmt.Sprintf(`readSysfsInt("/sys/class/thermal/thermal_zone0/temp")`)
}

// mapConditionExpr maps a condition expression to Go code.
func mapConditionExpr(name string, fn IntentFunction) string {
	lower := strings.ToLower(name)

	// If it's a number, return as-is
	if _, err := strconv.Atoi(name); err == nil {
		return name
	}
	if _, err := strconv.ParseFloat(name, 64); err == nil {
		return name
	}

	// Map to config field
	configFields := map[string]string{
		"threshold":  "c.Threshold",
		"limit":      "c.Limit",
		"max":        "c.Max",
		"min":        "c.Min",
		"warn":       "c.WarningThreshold",
		"warning":    "c.WarningThreshold",
		"critical":   "c.CriticalThreshold",
		"interval":   "c.CheckInterval",
	}

	for key, val := range configFields {
		if strings.Contains(lower, key) {
			return val
		}
	}

	// Fallback: return the name as a numeric literal
	if _, err := strconv.Atoi(name); err == nil {
		return name
	}
	return "0"
}

// generateActionCode converts a natural language action to Go code.
func generateActionCodeV2(action string, fn IntentFunction, index int) string {
	lower := strings.ToLower(action)
	var actions []string

	// Log action
	if strings.Contains(lower, "log") || strings.Contains(lower, "记录") || strings.Contains(lower, "日志") {
		if needsFileIO(fn) {
			actions = append(actions, fmt.Sprintf(`logActivity("Trigger %d: %s")`, index+1, escapeForGo(action)))
		} else {
			actions = append(actions, fmt.Sprintf(`log.Printf("Trigger %d: %s")`, index+1, escapeForGo(action)))
		}
	}

	// Warning/alert action
	if strings.Contains(lower, "warn") || strings.Contains(lower, "alert") || strings.Contains(lower, "告警") || strings.Contains(lower, "警告") {
		actions = append(actions, fmt.Sprintf(`log.Printf("⚠️  WARNING: %s")`, escapeForGo(action)))
	}

	// Write to file
	if strings.Contains(lower, "write") || strings.Contains(lower, "save") || strings.Contains(lower, "写入") || strings.Contains(lower, "保存") {
		actions = append(actions, fmt.Sprintf(`writeMetricToFile(%d)`, index+1))
	}

	// Reduce/throttle
	if strings.Contains(lower, "reduce") || strings.Contains(lower, "throttle") || strings.Contains(lower, "降低") || strings.Contains(lower, "限制") {
		actions = append(actions, `log.Printf("⚡ Applying throttle action")`)
	}

	// Reset counters
	if strings.Contains(lower, "reset") || strings.Contains(lower, "重置") {
		actions = append(actions, `log.Printf("🔄 Resetting counters")`)
	}

	// Notification
	if strings.Contains(lower, "notif") || strings.Contains(lower, "notify") || strings.Contains(lower, "通知") {
		actions = append(actions, `sendNotification()`)
	}

	// Default: at least log the action
	if len(actions) == 0 {
		actions = append(actions, fmt.Sprintf(`log.Printf("Action %d: %s")`, index+1, escapeForGo(action)))
	}

	return strings.Join(actions, "\n\t\t")
}

func escapeForGo(s string) string {
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, `\n`, ` `)
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

// ═══════════════════════════════════════════════════════
// MAIN FUNCTION (V2) — Config-aware interval
// ═══════════════════════════════════════════════════════

func generateMainV2(fn IntentFunction) string {
	var sb strings.Builder

	cfgPath := fn.Config["config_path"]
	if cfgPath == "" {
		cfgPath = "/data/adb/modules/" + fn.Name + "/config.json"
	}

	sb.WriteString("func main() {\n")
	sb.WriteString("\tlog.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)\n")
	sb.WriteString(fmt.Sprintf("\tlog.Printf(\"%s starting...\\n\")\n", fn.Name))

	// Init logging if needed
	if needsFileIO(fn) {
		sb.WriteString("\tif err := initLogging(); err != nil {\n")
		sb.WriteString("\t\tlog.Printf(\"Warning: logging init failed: %v\", err)\n")
		sb.WriteString("\t}\n")
		sb.WriteString("\tif logFile != nil {\n")
		sb.WriteString("\t\tdefer logFile.Close()\n")
		sb.WriteString("\t}\n\n")
	}

	// Graceful shutdown
	sb.WriteString("\tctx, stop := signal.NotifyContext(context.Background(),\n")
	sb.WriteString("\t\tsyscall.SIGTERM, syscall.SIGINT)\n")
	sb.WriteString("\tdefer stop()\n\n")

	// Load config
	if len(fn.Config) > 0 {
		sb.WriteString("\t// Load config (override path via first argument)\n")
			sb.WriteString("\tif len(os.Args) > 1 {\n")
		sb.WriteString("\t\tconfigPath = os.Args[1]\n")
		sb.WriteString("\t}\n")
		sb.WriteString("\tcfg, err := loadConfig(configPath)\n")
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

	// Determine interval from config
	interval := "30"
	if v, ok := fn.Config["check_interval_seconds"]; ok {
		if _, err := strconv.Atoi(v); err == nil {
			interval = v
		}
	}
	if v, ok := fn.Config["check_interval"]; ok {
		if _, err := strconv.Atoi(v); err == nil {
			interval = v
		}
	}

	// Main loop with config-aware interval
	sb.WriteString(fmt.Sprintf("\t// Main loop — check every %s seconds\n", interval))
	if len(fn.Config) > 0 && len(fn.DataStructures) > 0 {
		sb.WriteString(fmt.Sprintf("\tticker := time.NewTicker(time.Duration(cfg.CheckInterval) * time.Second)\n"))
	} else {
		sb.WriteString(fmt.Sprintf("\tticker := time.NewTicker(%s * time.Second)\n", interval))
	}
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

	// Call checkOnce with config
	if len(fn.Config) > 0 && len(fn.DataStructures) > 0 {
		sb.WriteString("\t\t\tcheckOnce(cfg)\n")
	} else {
		sb.WriteString("\t\t\tcheckOnce(nil)\n")
	}

	sb.WriteString("\t\t}\n")
	sb.WriteString("\t}\n")
	sb.WriteString("}\n")

	return sb.String()
}

// ═══════════════════════════════════════════════════════
// V2 IMPORTS — Smarter import collection
// ═══════════════════════════════════════════════════════

func (ic *IntentCompiler) collectGoImportsV2(fn IntentFunction, research *ResearchContext) []string {
	importSet := make(map[string]bool)
	importOrder := []string{}

	addImport := func(imp string) {
		if !importSet[imp] {
			importSet[imp] = true
			importOrder = append(importOrder, imp)
		}
	}

	// Always needed for daemons
	addImport("context")
	addImport("log")
	addImport("os")
	addImport("os/signal")
	addImport("syscall")
	addImport("time")

	// Config loading
	if len(fn.Config) > 0 {
		addImport("encoding/json")
		addImport("strconv")
	}

	// Sysfs / file reading
	if needsSysfs(fn) || needsFileIO(fn) {
		addImport("strconv")
		addImport("strings")
		addImport("fmt")
	}

	// File I/O
	if needsFileIO(fn) {
		// fmt already added above
	}

	return importOrder
}

// ═══════════════════════════════════════════════════════
// V2 HELPERS — More complete helper functions
// ═══════════════════════════════════════════════════════

func (ic *IntentCompiler) selectGoHelpersV2(fn IntentFunction, research *ResearchContext) []string {
	var helpers []string

	// Sysfs reader
	if needsSysfs(fn) {
		helpers = append(helpers, `// readSysfsInt reads an integer from a sysfs file.
// Returns 0 on error (safe for threshold comparisons).
func readSysfsInt(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	val, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return val
}

// readSysfsString reads a string from a sysfs file.
func readSysfsString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}`)
	}

	// CPU usage reader
	if needsCPUReading(fn) {
		helpers = append(helpers, `// readCPUUsage reads overall CPU usage percentage from /proc/stat.
// Returns a value 0-100 (approximate).
func readCPUUsage() int {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	lines := strings.SplitN(string(data), "\n", 2)
	if len(lines) == 0 {
		return 0
	}
	// First line: cpu user nice system idle iowait irq softirq steal
	fields := strings.Fields(lines[0])
	if len(fields) < 5 {
		return 0
	}
	user, _ := strconv.Atoi(fields[1])
	nice, _ := strconv.Atoi(fields[2])
	system, _ := strconv.Atoi(fields[3])
	idle, _ := strconv.Atoi(fields[4])
	total := user + nice + system + idle
	if total == 0 {
		return 0
	}
	return (total - idle) * 100 / total
}`)
	}

	// Memory usage reader
	if needsMemReading(fn) {
		helpers = append(helpers, `// readMemUsage reads memory usage percentage from /proc/meminfo.
func readMemUsage() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	var total, available int
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fmt.Sscanf(line, "MemTotal: %d kB", &total)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			fmt.Sscanf(line, "MemAvailable: %d kB", &available)
		}
	}
	if total == 0 {
		return 0
	}
	return (total - available) * 100 / total
}`)
	}

	// Network bytes reader
	if needsNetReading(fn) {
		helpers = append(helpers, `// readNetBytes reads total network bytes from /proc/net/dev.
func readNetBytes() int64 {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0
	}
	var total int64
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, ":"); idx > 0 {
			fields := strings.Fields(line[idx+1:])
			if len(fields) >= 1 {
				rx, _ := strconv.ParseInt(fields[0], 10, 64)
				total += rx
			}
			if len(fields) >= 9 {
				tx, _ := strconv.ParseInt(fields[8], 10, 64)
				total += tx
			}
		}
	}
	return total
}`)
	}

	// File writer
	if needsFileIO(fn) {
		helpers = append(helpers, `// writeMetricToFile appends a timestamped metric line to the activity log.
func writeMetricToFile(triggerIndex int) {
	if logFile == nil {
		return
	}
	line := fmt.Sprintf("[%%s] Metric recorded (trigger %%d)\\n",
		time.Now().Format("2006-01-02 15:04:05"), triggerIndex)
	logFile.WriteString(line)
}

// sendNotification sends a notification (placeholder — implement per-device).
func sendNotification() {
	log.Printf("📱 Notification sent")
}`)
	}

	return helpers
}

// ═══════════════════════════════════════════════════════
// INTENT ANALYSIS HELPERS
// ═══════════════════════════════════════════════════════

func needsCPUReading(fn IntentFunction) bool {
	desc := strings.ToLower(fn.Description + " " + fn.Logic.MainLoop)
	for _, t := range fn.Logic.Triggers {
		desc += " " + t.Condition + " " + t.Action
	}
	return strings.Contains(desc, "cpu") || strings.Contains(desc, "负载") || strings.Contains(desc, "load")
}

func needsMemReading(fn IntentFunction) bool {
	desc := strings.ToLower(fn.Description + " " + fn.Logic.MainLoop)
	for _, t := range fn.Logic.Triggers {
		desc += " " + t.Condition + " " + t.Action
	}
	return strings.Contains(desc, "mem") || strings.Contains(desc, "内存") || strings.Contains(desc, "memory")
}

func needsNetReading(fn IntentFunction) bool {
	desc := strings.ToLower(fn.Description + " " + fn.Logic.MainLoop)
	for _, t := range fn.Logic.Triggers {
		desc += " " + t.Condition + " " + t.Action
	}
	return strings.Contains(desc, "net") || strings.Contains(desc, "网络") || strings.Contains(desc, "traffic") || strings.Contains(desc, "bandwidth")
}

func needsLogging(fn IntentFunction) bool {
	desc := strings.ToLower(fn.Description + " " + fn.Logic.MainLoop)
	for _, t := range fn.Logic.Triggers {
		desc += " " + t.Condition + " " + t.Action
	}
	return strings.Contains(desc, "log") || strings.Contains(desc, "记录") || strings.Contains(desc, "日志") || strings.Contains(desc, "audit")
}
