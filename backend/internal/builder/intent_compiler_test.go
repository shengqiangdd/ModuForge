package builder

import (
	"encoding/json"
	"testing"
)

func TestPatternCatalog(t *testing.T) {
	catalog := NewPatternCatalog()

	// Test: find daemon patterns
	matches := catalog.FindMatchingPatterns("daemon", []string{"daemon", "service"})
	if len(matches) == 0 {
		t.Fatal("Expected at least one daemon pattern")
	}
	t.Logf("Found %d daemon patterns", len(matches))
	for _, m := range matches {
		t.Logf("  - %s (%s): %s", m.ID, m.Name, m.Description)
	}

	// Test: find sysfs patterns
	sysfsMatches := catalog.FindMatchingPatterns("", []string{"sysfs", "thermal"})
	if len(sysfsMatches) == 0 {
		t.Fatal("Expected at least one sysfs pattern")
	}
	t.Logf("Found %d sysfs patterns", len(sysfsMatches))

	// Test: get pattern by ID
	p := catalog.GetPattern("go_daemon")
	if p == nil {
		t.Fatal("Expected to find go_daemon pattern")
	}
	t.Logf("go_daemon has %d imports", len(p.Imports))
}

func TestIntentCompiler_SynthesizeGo(t *testing.T) {
	b := &Builder{}
	compiler := NewIntentCompiler(b)

	intent := IntentJSON{
		Functions: []IntentFunction{
			{
				Name:        "battery_monitor",
				Description: "Monitor battery temperature and level",
				Type:        "daemon",
				OutputPath:  "src/main.go",
				Config: map[string]string{
					"config_path": "/data/adb/modules/battery-monitor/config.json",
				},
				DataStructures: []DataStructure{
					{
						Name: "Config",
						Fields: []StructField{
							{Name: "Interval", Type: "int", Tag: "`json:\"interval\"`"},
							{Name: "Threshold", Type: "float64", Tag: "`json:\"threshold\"`"},
						},
					},
				},
				Logic: IntentLogic{
					MainLoop: "Read battery temperature and level, compare with thresholds",
					Triggers: []Trigger{
						{Condition: "temperature > 55", Action: "log critical warning"},
						{Condition: "battery < 15", Action: "enable power saving"},
					},
					InitSteps:    []string{"Load configuration", "Validate thresholds"},
					CleanupSteps: []string{"Log shutdown message"},
				},
			},
		},
	}

	logFn := func(s string) error { t.Log(s); return nil }
	files, err := compiler.CompileIntent(nil, intent, nil, "", logFn)
	if err != nil {
		t.Fatalf("CompileIntent failed: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("Expected at least one compiled file")
	}

	for _, f := range files {
		t.Logf("Generated %s (%d bytes)", f.Path, len(f.Content))
		if len(f.Content) < 100 {
			t.Errorf("File %s too short (%d bytes)", f.Path, len(f.Content))
		}
	}
}

func TestFixIntentJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name:  "valid json",
			input: `{"functions":[{"name":"test"}]}`,
			valid: true,
		},
		{
			name:  "trailing comma",
			input: `{"functions":[{"name":"test",}]}`,
			valid: true,
		},
		{
			name:  "markdown fences",
			input: "```json\n{\"functions\":[{\"name\":\"test\"}]}\n```",
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixed := fixIntentJSON(tt.input)
			var result map[string]interface{}
			err := json.Unmarshal([]byte(fixed), &result)
			if tt.valid && err != nil {
				t.Errorf("Expected valid JSON, got error: %v\nInput: %s\nFixed: %s", err, tt.input, fixed)
			}
		})
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct{ in, out string }{
		{"CheckInterval", "check_interval"},
		{"Temperature", "temperature"},
		{"ID", "i_d"},
		{"HTTPClient", "h_t_t_p_client"},
	}
	for _, tt := range tests {
		result := toSnakeCase(tt.in)
		if result != tt.out {
			t.Errorf("toSnakeCase(%q) = %q, want %q", tt.in, result, tt.out)
		}
	}
}

func TestExtractJSONFromResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "raw json",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "markdown fenced",
			input:    "Here is the result:\n```json\n{\"key\": \"value\"}\n```\nDone.",
			expected: `{"key": "value"}`,
		},
		{
			name:     "plain fenced",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSONFromResponse(tt.input)
			if result != tt.expected {
				t.Errorf("extractJSONFromResponse() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestResearcher_DefaultResearch(t *testing.T) {
	b := &Builder{}
	r := NewResearcher(b)

	rc := r.defaultResearch("开发一个电池温度监控 Magisk 模块，当温度超过阈值时关闭设备")
	if rc == nil {
		t.Fatal("Expected non-nil research context")
	}

	t.Logf("Best practices: %d", len(rc.BestPractices))
	t.Logf("API patterns: %d", len(rc.APIPatterns))
	t.Logf("Anti-patterns: %d", len(rc.AntiPatterns))
	t.Logf("Design patterns: %d", len(rc.DesignPatterns))

	// Should detect thermal and battery patterns
	if len(rc.APIPatterns) < 2 {
		t.Errorf("Expected at least 2 API patterns for thermal+battery, got %d", len(rc.APIPatterns))
	}
}
