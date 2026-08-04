package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type GenerateCodeSkill struct {
	apiKey   string
	endpoint string
	model    string
	client   *http.Client
}

func NewGenerateCodeSkill(apiKey, endpoint, model string) *GenerateCodeSkill {
	return &GenerateCodeSkill{
		apiKey:   apiKey,
		endpoint: endpoint,
		model:    model,
		client:   http.DefaultClient,
	}
}

// NewGenerateCodeSkillWithClient creates a GenerateCodeSkill that reuses a shared
// HTTP client (with connection pooling) instead of allocating a new one per skill.
func NewGenerateCodeSkillWithClient(apiKey, endpoint, model string, client *http.Client) *GenerateCodeSkill {
	if client == nil {
		client = http.DefaultClient
	}
	return &GenerateCodeSkill{
		apiKey:   apiKey,
		endpoint: endpoint,
		model:    model,
		client:   client,
	}
}

func (s *GenerateCodeSkill) Name() string {
	return "generate_code"
}

func (s *GenerateCodeSkill) Description() string {
	return "Generate module code with build system files. Input: {\"description\": \"...\", \"files_spec\": \"...\", \"existing_files\": {\"path\":\"content\"} (optional for incremental edits)}. Returns source code + build configs (CMakeLists.txt/Cargo.toml/Android.mk/go.mod)."
}

func (s *GenerateCodeSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	description, _ := input["description"].(string)
	filesSpec, _ := input["files_spec"].(string)
	existingFiles, _ := input["existing_files"].(map[string]interface{})

	if description == "" {
		return "", fmt.Errorf("description is required")
	}

	isIncremental := len(existingFiles) > 0

	systemPrompt := `You are a senior Android module developer for Magisk/KernelSU/APatch. Generate PRODUCTION-QUALITY code.

## RULES
1. Code must be COMPLETE and compilable — no placeholders, TODOs, or stubs
2. Every function must have error handling
3. Prefer standard library over third-party deps
4. NEVER chmod 777 — use 755/600 as needed
5. NEVER hardcode credentials or API keys
6. ALWAYS include build system files (CMakeLists.txt, Android.mk, Cargo.toml, or go.mod) — code without build config cannot be compiled

## LANGUAGE SELECTION
- Go: daemons, services, data processing, network ops (use for most tasks)
- Rust: memory-safe systems, eBPF, kernel interactions, performance-critical
- C/C++: performance-critical native code, JNI bridges, low-level system ops
- Shell: ONLY for install/boot scripts (customize.sh, service.sh, post-fs-data.sh)
  - Use #!/system/bin/sh (NOT bash), set -euo pipefail, quote "$VAR"

## BUILD SYSTEM FILES (CRITICAL)
When generating compiled code, you MUST include the appropriate build file:

### For Rust modules — Cargo.toml:
    [package]
    name = "module_name"
    version = "0.1.0"
    edition = "2021"

    [lib]
    crate-type = ["cdylib"]

    [dependencies]
    # minimal deps only

### For C/C++ modules — Android.mk:
    LOCAL_PATH := $(call my-dir)
    include $(CLEAR_VARS)
    LOCAL_MODULE := module_name
    LOCAL_SRC_FILES := src/main.cpp
    LOCAL_LDLIBS := -llog -landroid
    include $(BUILD_SHARED_LIBRARY)

### For C/C++ modules — CMakeLists.txt:
    cmake_minimum_required(VERSION 3.18)
    project(module_name)
    add_library(module_name SHARED src/main.cpp)
    target_link_libraries(module_name log android)

### For Go modules — go.mod:
    module module_name
    go 1.21

## MODULE STRUCTURE (Magisk/KernelSU/APatch)
- module.prop: id, name, version, versionCode, author, description
- customize.sh: install script (detect $KSU/$APATCH/Magisk)
- service.sh: late_start background tasks
- META-INF/com/google/android/update-binary: Magisk updater
- META-INF/com/google/android/updater-script: just "#MAGISK"

## OUTPUT FORMAT
Return ONLY a JSON object: {"files":[{"path":"...","content":"..."}]}
Each file must be complete and production-ready. No examples, no demos.`

	userPrompt := fmt.Sprintf("Generate module files for: %s", description)
	if filesSpec != "" {
		userPrompt += fmt.Sprintf("\n\nFile specification: %s", filesSpec)
	}
	if isIncremental {
		userPrompt += "\n\nExisting files (modify these, don't recreate from scratch):\n"
		for path, content := range existingFiles {
			userPrompt += fmt.Sprintf("=== %s ===\n%s\n\n", path, content)
		}
		userPrompt += "Return ONLY the files that need changes or new files."
	} else {
		userPrompt += "\n\nReturn ONLY a JSON object with 'files' array."
	}

	return s.callLLM(ctx, systemPrompt, userPrompt)
}

func (s *GenerateCodeSkill) callLLM(ctx context.Context, system, user string) (string, error) {
	endpoint := s.endpoint
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint = endpoint + "/chat/completions"
	}

	body := map[string]interface{}{
		"model": s.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"stream":     false,
		"max_tokens": 16384,
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("LLM error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse LLM response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	return result.Choices[0].Message.Content, nil
}

func (s *GenerateCodeSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  false,
		Essential: false,
		NeedsDB:   false,
		NeedsLLM:  true,
	}
}
