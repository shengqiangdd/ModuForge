package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FuncSpec describes a function signature in the spec.
type FuncSpec struct {
	Name        string   `json:"name"`
	Params      []string `json:"params,omitempty"`
	ReturnType  string   `json:"return_type,omitempty"`
	Description string   `json:"description,omitempty"`
}

// SpecFile describes a single file in the module spec.
type SpecFile struct {
	Path         string     `json:"path"`
	Purpose      string     `json:"purpose"`
	Language     string     `json:"language"`
	RequiredVars []string   `json:"required_vars,omitempty"`
	Functions    []FuncSpec `json:"functions,omitempty"`
}

// TestCase describes a test case for the module.
type TestCase struct {
	Name           string `json:"name"`
	Input          string `json:"input,omitempty"`
	ExpectedOutput string `json:"expected_output,omitempty"`
	Preconditions  string `json:"preconditions,omitempty"`
}

// Spec is the complete specification for a Magisk module.
type Spec struct {
	Name               string     `json:"name"`
	Version            string     `json:"version"`
	TargetSystem       string     `json:"target_system"`
	Description        string     `json:"description"`
	Author             string     `json:"author,omitempty"`
	Files              []SpecFile `json:"files"`
	BoundaryConditions []string   `json:"boundary_conditions,omitempty"`
	TestCases          []TestCase `json:"test_cases,omitempty"`
}

// SpecGenerator creates structured specifications from requirements.
type SpecGenerator struct {
	caller llmCaller
}

// NewSpecGenerator creates a SpecGenerator with resolved LLM configuration.
func NewSpecGenerator() *SpecGenerator {
	endpoint, apiKey, model := resolveAgentLLM()
	if endpoint == "" || apiKey == "" {
		return nil
	}
	return &SpecGenerator{
		caller: &builderLLMCaller{
			endpoint: endpoint,
			apiKey:   apiKey,
			model:    model,
		},
	}
}

// GenerateSpec analyzes a requirement and produces a structured Spec.
func (sg *SpecGenerator) GenerateSpec(ctx context.Context, requirement string) (Spec, error) {
	if sg == nil || sg.caller == nil {
		return Spec{}, fmt.Errorf("SpecGenerator not initialized: no LLM configured")
	}

	prompt := fmt.Sprintf(`You are a Software Specification Expert for Android Magisk modules.
Analyze this requirement and produce a detailed, structured specification.

## Requirement
%s

## Output format (valid JSON only):
{
  "name": "module-id",
  "version": "1.0",
  "target_system": "Magisk|KernelSU|Both",
  "description": "detailed description",
  "author": "ModuForge",
  "files": [
    {
      "path": "module.prop",
      "purpose": "module metadata",
      "language": "prop",
      "required_vars": ["MODID", "MODVER"],
      "functions": []
    },
    {
      "path": "customize.sh",
      "purpose": "installer script",
      "language": "shell",
      "required_vars": ["MODPATH", "MODID"],
      "functions": [
        {"name": "install_module", "params": [], "return_type": "void", "description": "main installer logic"}
      ]
    },
    {
      "path": "src/main.go",
      "purpose": "daemon logic",
      "language": "go",
      "required_vars": ["configPath"],
      "functions": [
        {"name": "main", "params": [], "return_type": "void", "description": "entry point"},
        {"name": "monitor", "params": ["interval int"], "return_type": "error", "description": "monitoring loop"}
      ]
    }
  ],
  "boundary_conditions": [
    "Must handle missing /sys file gracefully",
    "Must not consume more than 5%% CPU",
    "Must work on Android 8.0+ (API 26+)"
  ],
  "test_cases": [
    {
      "name": "daemon starts successfully",
      "input": "start daemon with valid config",
      "expected_output": "daemon running, PID file created",
      "preconditions": "config.json exists in module dir"
    },
    {
      "name": "handles missing config",
      "input": "start daemon without config.json",
      "expected_output": "graceful error, default config used",
      "preconditions": "no config.json in module dir"
    }
  ]
}

## Rules
- Each file must list required_vars and functions
- boundary_conditions: runtime constraints, compatibility, resource limits
- test_cases: cover happy path, error paths, edge cases
- target_system: Magisk (most common), KernelSU, or Both
- Output ONLY the JSON specification, nothing else.`, requirement)

	ctx, cancel := context.WithTimeout(ctx, teamLLMTimeout)
	defer cancel()

	resp, err := sg.caller.CallLLM(ctx, prompt)
	if err != nil {
		return Spec{}, fmt.Errorf("LLM call failed: %w", err)
	}

	resp = extractJSON(resp)

	var spec Spec
	if err := json.Unmarshal([]byte(resp), &spec); err != nil {
		return Spec{}, fmt.Errorf("parse spec JSON: %w\nresponse: %s", err, truncateStr(resp, 500))
	}

	return spec, nil
}

// SaveSpec persists the specification as JSON.
func (s *Spec) SaveSpec(dir string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal spec: %w", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	path := filepath.Join(dir, "module_spec.json")
	return os.WriteFile(path, data, 0644)
}

// LoadSpec reads a specification from JSON.
func LoadSpec(dir string) (*Spec, error) {
	path := filepath.Join(dir, "module_spec.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("unmarshal spec: %w", err)
	}
	return &spec, nil
}
