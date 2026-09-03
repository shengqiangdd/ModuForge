package builder

import (
	"context"
	"strings"
	"testing"
)

func TestSynthesizeGoV2_BasicDaemon(t *testing.T) {
	ic := &IntentCompiler{}

	fn := IntentFunction{
		Name:        "cpu-monitor",
		Description: "Monitor CPU temperature and log warnings when overheating",
		OutputPath:  "daemon/main.go",
		Config: map[string]string{
			"check_interval": "10",
			"threshold":      "55",
		},
		DataStructures: []DataStructure{
			{
				Name: "Config",
				Fields: []StructField{
					{Name: "CheckInterval", Type: "int"},
					{Name: "Threshold", Type: "int"},
				},
			},
		},
		Logic: IntentLogic{
			MainLoop: "Read CPU temperature every 10 seconds",
			InitSteps: []string{
				"Load configuration",
				"Initialize logging",
			},
			CleanupSteps: []string{
				"Close log file",
			},
			Triggers: []Trigger{
				{
					Condition: "temperature > threshold",
					Action:    "log warning about overheating",
				},
			},
		},
	}

	code, err := ic.synthesizeGoV2(context.Background(), fn, nil, func(msg string) error {
		t.Log(msg)
		return nil
	})
	if err != nil {
		t.Fatalf("synthesizeGoV2 failed: %v", err)
	}

	// Verify: must have package declaration
	if !strings.Contains(code, "package main") {
		t.Error("missing package main")
	}

	// Verify: must have func main
	if !strings.Contains(code, "func main()") {
		t.Error("missing func main()")
	}

	// Verify: must have signal handling
	if !strings.Contains(code, "signal.NotifyContext") && !strings.Contains(code, "signal.Notify") {
		t.Error("missing signal handling")
	}

	// Verify: must have config constants (interval=10, threshold=55)
	if !strings.Contains(code, "defaultInterval") {
		t.Error("missing defaultInterval constant")
	}

	// Verify: checkOnce has actual logic (not just comments)
	if strings.Contains(code, "checkOnce") {
		if !strings.Contains(code, "func checkOnce") {
			t.Error("missing checkOnce function definition")
		}
	}

	// Verify: has sysfs reading
	if !strings.Contains(code, "readSysfsInt") {
		t.Error("missing readSysfsInt helper")
	}

	// Verify: has time ticker
	if !strings.Contains(code, "time.NewTicker") {
		t.Error("missing time.NewTicker")
	}

	// Verify: has graceful shutdown
	if !strings.Contains(code, "ctx.Done()") {
		t.Error("missing graceful shutdown (ctx.Done)")
	}

	t.Logf("Generated code length: %d bytes", len(code))
	t.Logf("=== Generated Go Code ===\n%s", code)
}

func TestSynthesizeGoV2_WithTriggers(t *testing.T) {
	ic := &IntentCompiler{}

	fn := IntentFunction{
		Name:        "battery-guard",
		Description: "Monitor battery health and manage charging",
		OutputPath:  "daemon/main.go",
		Config: map[string]string{
			"check_interval": "30",
		},
		DataStructures: []DataStructure{
			{
				Name: "BatteryConfig",
				Fields: []StructField{
					{Name: "CheckInterval", Type: "int"},
					{Name: "LowThreshold", Type: "int"},
					{Name: "HighThreshold", Type: "int"},
				},
			},
		},
		Logic: IntentLogic{
			MainLoop: "Monitor battery level and temperature",
			Triggers: []Trigger{
				{
					Condition: "battery level < 20",
					Action:    "log warning about low battery",
				},
				{
					Condition: "temperature > 45",
					Action:    "log alert about battery overheating",
				},
				{
					Condition: "battery level > 90",
					Action:    "log info about full charge",
				},
			},
		},
	}

	code, err := ic.synthesizeGoV2(context.Background(), fn, nil, func(msg string) error {
		return nil
	})
	if err != nil {
		t.Fatalf("synthesizeGoV2 failed: %v", err)
	}

	// Verify triggers generate real if-conditions
	if !strings.Contains(code, "if ") {
		t.Error("no if-conditions generated from triggers")
	}

	// Verify: at least 3 trigger blocks
	ifCount := strings.Count(code, "if ")
	if ifCount < 3 {
		t.Errorf("expected at least 3 if-blocks for triggers, got %d", ifCount)
	}

	t.Logf("Generated code: %d bytes, %d if-blocks", len(code), ifCount)
	t.Logf("=== Generated Go Code ===\n%s", code)
}

func TestGenerateConditionCode(t *testing.T) {
	fn := IntentFunction{
		Config: map[string]string{
			"threshold": "55",
		},
		DataStructures: []DataStructure{
			{
				Name: "Config",
				Fields: []StructField{
					{Name: "Threshold", Type: "int"},
				},
			},
		},
	}

	tests := []struct {
		name       string
		condition  string
		mustHave   []string
	}{
		{
			name:      "temperature > threshold",
			condition: "temperature > threshold",
			mustHave:  []string{">", "readSysfsInt"},
		},
		{
			name:      "battery level < 20",
			condition: "battery level < 20",
			mustHave:  []string{"<", "20", "readSysfsInt"},
		},
		{
			name:      "cpu load > 80",
			condition: "cpu load > 80",
			mustHave:  []string{">", "80", "readCPUUsage"},
		},
		{
			name:      "memory > 90",
			condition: "memory > 90",
			mustHave:  []string{">", "90", "readMemUsage"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := generateConditionCode(tt.condition, fn)
			for _, must := range tt.mustHave {
				if !strings.Contains(code, must) {
					t.Errorf("condition code %q missing %q\nGot: %s", tt.condition, must, code)
				}
			}
			t.Logf("condition=%q → code=%q", tt.condition, code)
		})
	}
}

func TestGenerateMainV2(t *testing.T) {
	fn := IntentFunction{
		Name: "test-daemon",
		Config: map[string]string{
			"check_interval": "60",
		},
		DataStructures: []DataStructure{
			{
				Name: "Config",
				Fields: []StructField{
					{Name: "CheckInterval", Type: "int"},
				},
			},
		},
		Logic: IntentLogic{
			Triggers:     []Trigger{{Condition: "temp > 50", Action: "log warning"}},
			CleanupSteps: []string{"Close files"},
		},
	}

	code := generateMainV2(fn)

	// Verify essential daemon structure
	checks := []struct {
		name string
		want string
	}{
		{"func main", "func main()"},
		{"signal", "signal.NotifyContext"},
		{"ticker", "time.NewTicker"},
		{"shutdown", "ctx.Done()"},
		{"config loading", "loadConfig"},
		{"checkOnce call", "checkOnce"},
	}

	for _, c := range checks {
		if !strings.Contains(code, c.want) {
			t.Errorf("generateMainV2 missing %s: expected %q", c.name, c.want)
		}
	}

	t.Logf("=== Generated main.go ===\n%s", code)
}

func TestGenerateCheckOnceV2_MultipleTriggers(t *testing.T) {
	fn := IntentFunction{
		DataStructures: []DataStructure{
			{
				Name: "Config",
				Fields: []StructField{
					{Name: "Threshold", Type: "int"},
				},
			},
		},
		Logic: IntentLogic{
			Triggers: []Trigger{
				{Condition: "temperature > 55", Action: "log warning"},
				{Condition: "battery < 20", Action: "log alert"},
			},
		},
	}

	code := generateCheckOnceV2(fn)

	// Must have type assertion for config
	if !strings.Contains(code, "cfg.(Config)") {
		t.Error("missing config type assertion")
	}

	// Must have trigger if-blocks (the type assertion also has an if, so expect >= 2+1=3)
	ifCount := strings.Count(code, "if ")
	if ifCount < 3 {
		t.Errorf("expected at least 3 if-blocks (2 triggers + type assert), got %d", ifCount)
	}

	// Must have log.Printf calls for actions
	logCount := strings.Count(code, "log.Printf")
	if logCount < 2 {
		t.Errorf("expected at least 2 log.Printf calls, got %d", logCount)
	}

	t.Logf("=== Generated checkOnce ===\n%s", code)
}

func TestGenerateConfigConstants(t *testing.T) {
	fn := IntentFunction{
		Config: map[string]string{
			"check_interval": "30",
			"threshold":      "55",
			"max_cpu":        "80",
		},
	}

	code := generateConfigConstants(fn)

	if !strings.Contains(code, "defaultInterval") {
		t.Error("missing defaultInterval")
	}
	if !strings.Contains(code, "30") {
		t.Error("missing interval value 30")
	}
	if !strings.Contains(code, "defaultThreshold") {
		t.Error("missing defaultThreshold")
	}
	if !strings.Contains(code, "55") {
		t.Error("missing threshold value 55")
	}

	t.Logf("=== Generated constants ===\n%s", code)
}

func TestSelectGoHelpersV2(t *testing.T) {
	ic := &IntentCompiler{}

	tests := []struct {
		name     string
		fn       IntentFunction
		mustHave []string
	}{
		{
			name: "sysfs reader",
			fn: IntentFunction{
				Description: "Read thermal zone temperature from /sys/class/thermal",
			},
			mustHave: []string{"readSysfsInt", "readSysfsString"},
		},
		{
			name: "cpu reader",
			fn: IntentFunction{
				Description: "Monitor CPU load and frequency",
			},
			mustHave: []string{"readCPUUsage"},
		},
		{
			name: "memory reader",
			fn: IntentFunction{
				Description: "Check memory usage percentage",
			},
			mustHave: []string{"readMemUsage"},
		},
		{
			name: "network reader",
			fn: IntentFunction{
				Description: "Track network traffic bytes",
			},
			mustHave: []string{"readNetBytes"},
		},
		{
			name: "file I/O",
			fn: IntentFunction{
				Description: "Log metrics to activity file",
			},
			mustHave: []string{"writeMetricToFile"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helpers := ic.selectGoHelpersV2(tt.fn, nil)
			combined := strings.Join(helpers, "\n")
			for _, must := range tt.mustHave {
				if !strings.Contains(combined, must) {
					t.Errorf("missing helper %q in %d helper functions", must, len(helpers))
				}
			}
		})
	}
}

func TestIntentCompilerV2_FullPipeline(t *testing.T) {
	ic := &IntentCompiler{}

	fn := IntentFunction{
		Name:        "thermal-guard",
		Description: "Monitor device thermal status and protect against overheating",
		OutputPath:  "daemon/main.go",
		Config: map[string]string{
			"config_path": "/data/adb/modules/thermal-guard/config.json",
		},
		DataStructures: []DataStructure{
			{
				Name: "ThermalConfig",
				Fields: []StructField{
					{Name: "CheckInterval", Type: "int"},
					{Name: "WarnTemp", Type: "int"},
					{Name: "CriticalTemp", Type: "int"},
					{Name: "LogPath", Type: "string"},
				},
			},
		},
		Logic: IntentLogic{
			MainLoop: "Monitor thermal zones and manage cooling",
			InitSteps: []string{
				"Load configuration from config.json",
				"Validate temperature thresholds",
				"Open log file for writing",
			},
			CleanupSteps: []string{
				"Close log file",
				"Release thermal resources",
			},
			Triggers: []Trigger{
				{
					Condition: "temperature > critical",
					Action:    "log critical warning and activate emergency cooling",
				},
				{
					Condition: "temperature > warn",
					Action:    "log warning about high temperature",
				},
				{
					Condition: "cpu load > 90",
					Action:    "log warning about CPU overload",
				},
			},
		},
	}

	code, err := ic.synthesizeGoV2(context.Background(), fn, nil, func(msg string) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Full pipeline failed: %v", err)
	}

	// Comprehensive checks
	requirements := []struct {
		name string
		desc string
	}{
		{"package main", "package declaration"},
		{"func main()", "main function"},
		{"signal.NotifyContext", "signal handling for graceful shutdown"},
		{"time.NewTicker", "periodic timer"},
		{"ctx.Done()", "graceful shutdown on context cancel"},
		{"func checkOnce", "checkOnce function definition"},
		{"loadConfig", "config loading"},
		{"ThermalConfig", "config struct"},
		{"readSysfsInt", "sysfs reading helper"},
		{"defaultInterval", "interval constant"},
		{"// Trigger 1", "trigger 1 comment"},
		{"// Trigger 2", "trigger 2 comment"},
		{"// Trigger 3", "trigger 3 comment"},
	}

	for _, req := range requirements {
		if !strings.Contains(code, req.name) {
			t.Errorf("FULL PIPELINE missing %s: %s", req.name, req.desc)
		}
	}

	// Must have at least 3 if-blocks for 3 triggers
	ifCount := strings.Count(code, "if ")
	if ifCount < 3 {
		t.Errorf("expected >= 3 if-blocks for 3 triggers, got %d", ifCount)
	}

	t.Logf("Full pipeline output: %d bytes, %d if-blocks, %d helpers",
		len(code), ifCount, strings.Count(code, "func "))
	t.Logf("=== Full Pipeline Output ===\n%s", code)
}
