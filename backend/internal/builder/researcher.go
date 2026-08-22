package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Researcher performs technical research before code generation.
// It uses LLM knowledge + structured best practices to gather
// authoritative patterns, API conventions, and anti-patterns.
type Researcher struct {
	builder *Builder
}

// ResearchContext holds research findings for a specific requirement.
type ResearchContext struct {
	Requirement    string            `json:"requirement"`
	BestPractices  []string          `json:"best_practices"`   // 权威最佳实践
	APIPatterns     []APIPattern      `json:"api_patterns"`     // API 使用模式
	AntiPatterns    []string          `json:"anti_patterns"`    // 反模式（避免的写法）
	CodeExamples    []CodeExample     `json:"code_examples"`    // 参考代码片段
	DesignPatterns  []DesignPattern   `json:"design_patterns"`  // 设计模式建议
	Dependencies    []Dependency      `json:"dependencies"`     // 推荐依赖
}

// APIPattern describes how to use a specific API.
type APIPattern struct {
	Name        string `json:"name"`        // e.g. "read thermal zone"
	API         string `json:"api"`         // e.g. "os.ReadFile('/sys/class/thermal/...')"
	Description string `json:"description"` // 说明
	GoSnippet   string `json:"go_snippet"`  // Go 代码片段
}

// CodeExample is a reference code snippet from authoritative sources.
type CodeExample struct {
	Source      string `json:"source"`       // e.g. "effective_go", "magisk_wiki"
	Purpose     string `json:"purpose"`      // 用途说明
	Code        string `json:"code"`         // 代码片段
}

// DesignPattern is a suggested design pattern for the module.
type DesignPattern struct {
	Name        string `json:"name"`        // e.g. "graceful shutdown"
	Description string `json:"description"` // 何时使用
	Template    string `json:"template"`    // 模式代码骨架
}

// Dependency describes an external dependency recommendation.
type Dependency struct {
	Name    string `json:"name"`    // e.g. "stdlib only"
	Reason  string `json:"reason"`  // 为什么用/不用
}

// NewResearcher creates a new Researcher instance.
func NewResearcher(b *Builder) *Researcher {
	return &Researcher{builder: b}
}

// Research performs technical research for a given requirement.
// Returns a ResearchContext with best practices, API patterns, and code examples.
func (r *Researcher) Research(ctx context.Context, description string, planJSON string, logFn func(string)) (*ResearchContext, error) {
	logFn("  🔍 Research: analyzing requirement and gathering best practices...\n")

	rc := &ResearchContext{
		Requirement: description,
	}

	// Step 1: Use LLM to identify relevant technologies and patterns
	researchJSON, err := r.llmResearch(ctx, description, planJSON, logFn)
	if err != nil {
		logFn(fmt.Sprintf("  ⚠️  LLM research failed: %v, using defaults\n", err))
		rc = r.defaultResearch(description)
		return rc, nil
	}

	// Step 2: Merge with authoritative knowledge base
	rc.BestPractices = researchJSON.BestPractices
	rc.APIPatterns = researchJSON.APIPatterns
	rc.AntiPatterns = researchJSON.AntiPatterns
	rc.DesignPatterns = researchJSON.DesignPatterns
	rc.Dependencies = researchJSON.Dependencies

	// Step 3: Enrich with built-in authoritative patterns
	r.enrichWithAuthoritativeKnowledge(rc)

	logFn(fmt.Sprintf("  ✅ Research complete: %d best practices, %d API patterns, %d anti-patterns\n",
		len(rc.BestPractices), len(rc.APIPatterns), len(rc.AntiPatterns)))

	return rc, nil
}

// llmResearch uses LLM to identify relevant technologies and patterns.
func (r *Researcher) llmResearch(ctx context.Context, description, planJSON string, logFn func(string)) (*ResearchContext, error) {
	prompt := `You are a senior Android/Go/C engineer researching best practices for a Magisk module.

## Module Requirement
` + description + `

## Architecture Plan
` + planJSON + `

## Task
Research and recommend the BEST technical approach for this module. Output a JSON object:

{
  "best_practices": [
    "Go daemon should use signal.NotifyContext for graceful shutdown",
    "Use log/slog (Go 1.21+ stdlib) for structured logging, no third-party logger",
    "Read sysfs values with os.ReadFile + strconv.Atoi, no cgo needed"
  ],
  "api_patterns": [
    {
      "name": "read thermal zone temperature",
      "api": "os.ReadFile('/sys/class/thermal/thermal_zone0/temp')",
      "description": "Read temperature from sysfs, value is millidegrees",
      "go_snippet": "data, err := os.ReadFile(\"/sys/class/thermal/thermal_zone0/temp\")\nif err != nil { return err }\ntemp, _ := strconv.Atoi(strings.TrimSpace(string(data)))\ntempC := float64(temp) / 1000.0"
    }
  ],
  "anti_patterns": [
    "DO NOT use cgo — NDK cross-compilation is complex and error-prone",
    "DO NOT use os/exec to call shell commands — use Go stdlib directly",
    "DO NOT use global variables for state — use struct methods",
    "DO NOT use viper or other heavy config libraries — use json.Unmarshal"
  ],
  "design_patterns": [
    {
      "name": "graceful shutdown",
      "description": "Catch SIGTERM/SIGINT, cleanup resources, exit cleanly",
      "template": "ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)\ndefer stop()\n<-ctx.Done()\n// cleanup here"
    }
  ],
  "dependencies": [
    {"name": "stdlib only", "reason": "Avoid external deps for Magisk modules — keep binary small"}
  ]
}

## CRITICAL RULES
- Focus on Go 1.21+ standard library features
- Reference Android sysfs interfaces when relevant
- Prefer POSIX-compatible approaches
- Output ONLY valid JSON, nothing else.`

	endpoint, apiKey, model := r.builder.resolveLLMForFix()
	if endpoint == "" || apiKey == "" {
		return nil, fmt.Errorf("no LLM configured")
	}

	response, err := callLLMForFix(ctx, endpoint, apiKey, model, prompt)
	if err != nil {
		return nil, err
	}

	// Parse response
	response = extractJSONFromResponse(response)
	var rc ResearchContext
	if err := json.Unmarshal([]byte(response), &rc); err != nil {
		return nil, fmt.Errorf("failed to parse research response: %w", err)
	}

	return &rc, nil
}

// defaultResearch provides fallback research when LLM is unavailable.
func (r *Researcher) defaultResearch(description string) *ResearchContext {
	rc := &ResearchContext{
		Requirement: description,
		BestPractices: []string{
			"Use Go 1.21+ standard library (log/slog, signal.NotifyContext)",
			"Cross-compile with GOOS=android GOARCH=arm64 CGO_ENABLED=0",
			"Keep dependencies minimal — stdlib only for Magisk modules",
			"Handle errors at every level — no unchecked err returns",
			"Use context.Context for cancellation and timeouts",
		},
		AntiPatterns: []string{
			"DO NOT use cgo — complicates cross-compilation",
			"DO NOT use os/exec — use Go stdlib instead",
			"DO NOT use global variables — use struct methods",
			"DO NOT use third-party libraries — keep binary small",
		},
		DesignPatterns: []DesignPattern{
			{
				Name:        "graceful shutdown",
				Description: "Catch SIGTERM/SIGINT for clean exit",
				Template:    "ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)\ndefer stop()\n<-ctx.Done()",
			},
			{
				Name:        "periodic check loop",
				Description: "Run a check at fixed intervals with context support",
				Template:    "ticker := time.NewTicker(interval)\ndefer ticker.Stop()\nfor {\n  select {\n  case <-ctx.Done(): return\n  case <-ticker.C: check()\n  }\n}",
			},
		},
	}

	// Detect sysfs paths from description
	lower := strings.ToLower(description)
	if strings.Contains(lower, "温度") || strings.Contains(lower, "thermal") || strings.Contains(lower, "temperature") {
		rc.APIPatterns = append(rc.APIPatterns, APIPattern{
			Name:        "read thermal zone",
			API:         "os.ReadFile('/sys/class/thermal/thermal_zone0/temp')",
			Description: "Read temperature (millidegrees Celsius)",
			GoSnippet:   `data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")`,
		})
	}
	if strings.Contains(lower, "电池") || strings.Contains(lower, "battery") {
		rc.APIPatterns = append(rc.APIPatterns, APIPattern{
			Name:        "read battery level",
			API:         "os.ReadFile('/sys/class/power_supply/battery/capacity')",
			Description: "Read battery percentage (0-100)",
			GoSnippet:   `data, err := os.ReadFile("/sys/class/power_supply/battery/capacity")`,
		})
	}
	if strings.Contains(lower, "cpu") || strings.Contains(lower, "负载") {
		rc.APIPatterns = append(rc.APIPatterns, APIPattern{
			Name:        "read CPU load",
			API:         "os.ReadFile('/proc/loadavg')",
			Description: "Read system load average",
			GoSnippet:   `data, err := os.ReadFile("/proc/loadavg")`,
		})
	}

	return rc
}

// enrichWithAuthoritativeKnowledge adds built-in authoritative patterns.
func (r *Researcher) enrichWithAuthoritativeKnowledge(rc *ResearchContext) {
	// Go standard library best practices (Effective Go, Go Proverbs)
	goBestPractices := map[string]bool{}
	for _, bp := range rc.BestPractices {
		goBestPractices[bp] = true
	}

	addIfNew := func(bp string) {
		if !goBestPractices[bp] {
			rc.BestPractices = append(rc.BestPractices, bp)
			goBestPractices[bp] = true
		}
	}

	// Always add these universal best practices
	addIfNew("Handle every error — no unchecked err returns")
	addIfNew("Use strconv.Atoi/strconv.Itoa instead of fmt.Sprintf for number conversion")
	addIfNew("Close all open resources (files, connections) with defer")
	addIfNew("Use strings.TrimSpace on values read from sysfs files")
	addIfNew("Return error up the call stack, don't panic in library code")
}

// extractJSONFromResponse extracts JSON from LLM response that might contain markdown fences.
func extractJSONFromResponse(response string) string {
	// Try to find JSON block
	if idx := strings.Index(response, "```json"); idx >= 0 {
		start := idx + 7
		if end := strings.Index(response[start:], "```"); end > 0 {
			return strings.TrimSpace(response[start : start+end])
		}
	}
	if idx := strings.Index(response, "```"); idx >= 0 {
		start := idx + 3
		// Skip language identifier on same line
		if nl := strings.Index(response[start:], "\n"); nl >= 0 {
			start = start + nl + 1
		}
		if end := strings.Index(response[start:], "```"); end > 0 {
			return strings.TrimSpace(response[start : start+end])
		}
	}

	// Try to find raw JSON (starts with {, ends with })
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start >= 0 && end > start {
		return response[start : end+1]
	}

	return response
}
