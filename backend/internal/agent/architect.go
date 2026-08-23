package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FuncSignature describes a function to generate.
type FuncSignature struct {
	Name       string   `json:"name"`
	Signature  string   `json:"signature"`
	Doc        string   `json:"doc,omitempty"`
	Parameters []string `json:"parameters,omitempty"`
	Returns    []string `json:"returns,omitempty"`
}

// VarDef describes a variable to declare.
type VarDef struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Default string `json:"default,omitempty"`
	Doc     string `json:"doc,omitempty"`
}

// FileDesign describes how a single file should be structured.
type FileDesign struct {
	Path      string          `json:"path"`
	Purpose   string          `json:"purpose"`
	Language  string          `json:"language"`
	Functions []FuncSignature `json:"functions,omitempty"`
	Variables []VarDef        `json:"variables,omitempty"`
}

// ModuleDesign is the complete architectural design for a Magisk module.
type ModuleDesign struct {
	ModuleName   string       `json:"module_name"`
	Version      string       `json:"version"`
	Description  string       `json:"description"`
	TargetSystem string       `json:"target_system"`
	Author       string       `json:"author,omitempty"`
	Files        []FileDesign `json:"files"`
}

// Architect designs the module structure based on PM's task graph.
type Architect struct {
	caller llmCaller
}

// NewArchitect creates an Architect with resolved LLM configuration.
func NewArchitect() *Architect {
	endpoint, apiKey, model := resolveAgentLLM()
	if endpoint == "" || apiKey == "" {
		return nil
	}
	return &Architect{
		caller: &builderLLMCaller{
			endpoint: endpoint,
			apiKey:   apiKey,
			model:    model,
		},
	}
}

// DesignModule analyzes the task graph and requirement, producing a ModuleDesign.
func (a *Architect) DesignModule(ctx context.Context, graph TaskGraph, requirement string) (ModuleDesign, error) {
	if a == nil || a.caller == nil {
		return ModuleDesign{}, fmt.Errorf("Architect not initialized: no LLM configured")
	}

	graphJSON, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return ModuleDesign{}, fmt.Errorf("marshal task graph: %w", err)
	}

	prompt := fmt.Sprintf(`You are a Software Architect for Android Magisk modules.
Design the module structure based on this task graph and requirement.

## Requirement
%s

## Task Graph
%s

## Output format (valid JSON only):
{
  "module_name": "module-id",
  "version": "1.0",
  "description": "module description",
  "target_system": "Android 8.0+ (API 26+)",
  "author": "ModuForge",
  "files": [
    {
      "path": "module.prop",
      "purpose": "module metadata",
      "language": "prop",
      "functions": [],
      "variables": [
        {"name": "MODID", "type": "string", "default": "module_id", "doc": "module identifier"}
      ]
    },
    {
      "path": "customize.sh",
      "purpose": "installer script",
      "language": "shell",
      "functions": [
        {"name": "install_module", "signature": "install_module()", "doc": "main installer logic"}
      ],
      "variables": [
        {"name": "MODPATH", "type": "string", "doc": "module installation path"}
      ]
    }
  ]
}

## Rules
- Each file design must include all functions and key variables
- module.prop: id, name, version, versionCode, author, description fields
- customize.sh: installation logic, permissions setup
- service.sh: daemon lifecycle (if needed)
- Go files: daemon struct, init, main loop, signal handling
- All Magisk conventions: ${MODPATH}, set_perm, set_perm_recursive
- Output ONLY the JSON design, nothing else.`, requirement, string(graphJSON))

	ctx, cancel := context.WithTimeout(ctx, teamLLMTimeout)
	defer cancel()

	resp, err := a.caller.CallLLM(ctx, prompt)
	if err != nil {
		return ModuleDesign{}, fmt.Errorf("LLM call failed: %w", err)
	}

	resp = extractJSON(resp)

	var design ModuleDesign
	if err := json.Unmarshal([]byte(resp), &design); err != nil {
		return ModuleDesign{}, fmt.Errorf("parse module design JSON: %w\nresponse: %s", err, truncateStr(resp, 500))
	}

	return design, nil
}

// SaveDesign persists the module design as JSON.
func (d *ModuleDesign) SaveDesign(dir string) error {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal design: %w", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	path := filepath.Join(dir, "module_design.json")
	return os.WriteFile(path, data, 0644)
}

// LoadDesign reads a module design from JSON.
func LoadDesign(dir string) (*ModuleDesign, error) {
	path := filepath.Join(dir, "module_design.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read design: %w", err)
	}
	var design ModuleDesign
	if err := json.Unmarshal(data, &design); err != nil {
		return nil, fmt.Errorf("unmarshal design: %w", err)
	}
	return &design, nil
}
