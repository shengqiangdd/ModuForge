package builder

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moduforge/backend/internal/rag"
)

// Multi-stage generation engine for free models.
// Architecture:
//   Stage 0: Architecture Planning (unchanged)
//   Stage 1.5: Technical Research (NEW — gather best practices, API patterns)
//   Stage 1: Shell Generation (unchanged, 95%+ success rate)
//   Stage 2: Intent Generation → Code Synthesis (REPLACED — model outputs intent JSON, synthesizer generates code)
//   Stage 3: Build System Generation (unchanged)
//   Stage 4: Compilation + AutoFix (unchanged)

// StagePlan holds the architecture plan determined in Stage 0.
type StagePlan struct {
	ID         string          `json:"id"`          // e.g. "battery-monitor"
	Name       string          `json:"name"`        // Human-readable name
	ModuleType string          `json:"module_type"` // service, tool, tweak
	Languages  []string        `json:"languages"`   // ["shell"], ["go","shell"], etc.
	ShellFiles []StageFileInfo `json:"shell_files"` // module.prop, customize.sh, etc.
	GoFiles    []StageFileInfo `json:"go_files"`    // src/main.go, etc.
	CFiles     []StageFileInfo `json:"c_files"`     // src/main.c, etc.
	BuildFiles []StageFileInfo `json:"build_files"` // build.sh, go.mod, etc.
	ExtraFiles []StageFileInfo `json:"extra_files"` // service.sh, uninstall.sh, config, etc.
}

type StageFileInfo struct {
	Path        string `json:"path"`
	Description string `json:"description"` // What this file does
}

// MultiStageBuildPrompt returns the Stage 0 architecture planning prompt.
func MultiStageBuildPrompt(description string) string {
	return `You are an Android module architect. Analyze the requirement and output a JSON plan.

## Requirement
` + description + `

## Output a JSON architecture plan:
{
  "id": "short-kebab-case-id",
  "name": "Human Readable Name",
  "module_type": "service|tool|tweak",
  "languages": ["shell"] | ["go","shell"] | ["c","shell"] | ["go","c","shell"],
  "shell_files": [
    {"path": "module.prop", "description": "module metadata"},
    {"path": "customize.sh", "description": "installer script"},
    {"path": "META-INF/com/google/android/update-binary", "description": "magisk binary template"},
    {"path": "META-INF/com/google/android/updater-script", "description": "just #MAGISK"}
  ],
  "go_files": [
    {"path": "src/main.go", "description": "main daemon logic"}
  ],
  "c_files": [],
  "build_files": [
    {"path": "go.mod", "description": "go module definition"},
    {"path": "build.sh", "description": "cross-compilation script"}
  ],
  "extra_files": [
    {"path": "service.sh", "description": "runs on boot"},
    {"path": "uninstall.sh", "description": "cleanup on remove"}
  ]
}

## Rules:
- Prefer Shell-only when possible (highest success rate)
- Use Go only for: daemon, network, data processing, complex logic
- Use C only for: system calls, performance-critical, hardware access
- NEVER mix Go and C unless absolutely necessary
- Output ONLY the JSON plan, nothing else.
- Do NOT generate source code yet — just the file structure.`
}

// ShellStagePrompt generates Shell files (Stage 1).
// Shell has ~100% success rate with free models.
func ShellStagePrompt(planJSON, description string) string {
	prompt := `Generate Shell scripts for this Android Magisk module.

## Architecture Plan
` + planJSON + `

## Requirement
` + description + `

## CRITICAL RULES (violations = build failure)
1. Variables: ALWAYS use ${VAR} syntax, NEVER bare $VAR
2. All variables in double quotes: "${VAR}"
3. sleep takes INTEGER seconds only: sleep 5 (not sleep 1.5)
4. Test syntax: [ "$x" = "yes" ] (spaces inside brackets)
5. Shebang: #!/system/bin/sh
6. module.prop: key=value pairs, NO quotes around values
7. Do NOT use $1 $2 $3 except where they are actual script arguments

## Files to generate (in this exact order):
1. module.prop (key=value format)
2. META-INF/com/google/android/update-binary (MAGISK TEMPLATE — copy exactly):
   #!/sbin/sh
   umask 022
   ui_print() { echo "$1"; }
   require_new_magisk() { ui_print "Install Magisk v20.4+!"; exit 1; }
   OUTFD=$2
   ZIPFILE=$3
   [ -f /data/adb/magisk/util_functions.sh ] || require_new_magisk
   . /data/adb/magisk/util_functions.sh
   [ $MAGISK_VER_CODE -lt 20400 ] && require_new_magisk
   install_module
   exit 0
3. META-INF/com/google/android/updater-script: just "#MAGISK"
4. customize.sh (main installer logic)
5. service.sh (if needed — runs on boot)
6. uninstall.sh (if needed — cleanup)

## SECURITY RULES for uninstall.sh:
- NEVER use "rm -rf /" or "rm -rf /*" — this is EXTREMELY DANGEROUS and will fail security scan
- Only remove files YOUR module created (in /data/adb/modules/<module_id>/)
- Use specific paths: rm -rf /data/adb/modules/$MODPATH
- Safe uninstall template:
  #!/system/bin/sh
  MODDIR=\${0%/*}
  # Remove module data
  rm -rf /data/adb/modules/\$(basename \$MODDIR)

## OUTPUT FORMAT
{"files":[{"path":"module.prop","content":"..."},{"path":"customize.sh","content":"..."}]}

Return ONLY valid JSON. Full file contents with \\n for newlines in content field.`
	return InjectRAGContext(prompt, description, 2)
}

// ═══════════════════════════════════════════════════════
// STAGE 1.5: Technical Research Prompt
// ═══════════════════════════════════════════════════════

// ResearchStagePrompt generates the research prompt for Stage 1.5.
// The LLM analyzes the requirement and produces best practices + API patterns.
func ResearchStagePrompt(planJSON, description string) string {
	return `You are a senior Android/Go/C engineer. Before writing code, RESEARCH the best approach.

## Module Requirement
` + description + `

## Architecture Plan
` + planJSON + `

## Task
Analyze this requirement and output a JSON object with your technical research:

{
  "best_practices": [
    "Go daemon should use signal.NotifyContext for graceful shutdown (Go 1.16+)",
    "Use log/slog for structured logging (Go 1.21+ stdlib, no third-party needed)",
    "Read sysfs values with os.ReadFile + strconv.Atoi, no cgo needed",
    "All Magisk module config in /data/adb/modules/<id>/",
    "Cross-compile: GOOS=android GOARCH=arm64 CGO_ENABLED=0"
  ],
  "api_patterns": [
    {
      "name": "read thermal zone temperature",
      "api": "os.ReadFile('/sys/class/thermal/thermal_zone0/temp')",
      "description": "Read temperature from sysfs, value is millidegrees Celsius",
      "go_snippet": "data, err := os.ReadFile(\"/sys/class/thermal/thermal_zone0/temp\")\nif err != nil { return err }\ntemp, _ := strconv.Atoi(strings.TrimSpace(string(data)))\ntempC := float64(temp) / 1000.0"
    }
  ],
  "anti_patterns": [
    "DO NOT use cgo — NDK cross-compilation is complex",
    "DO NOT use os/exec — use Go stdlib directly",
    "DO NOT use third-party libraries — keep binary small"
  ],
  "design_patterns": [
    {
      "name": "periodic check with graceful exit",
      "description": "Use time.Ticker + signal.NotifyContext for clean daemon loop"
    }
  ],
  "dependencies": [
    {"name": "stdlib only", "reason": "Minimal binary for Magisk modules"}
  ]
}

## RULES
- Be specific to THIS requirement (not generic advice)
- Reference actual sysfs paths when reading system values
- Use Go 1.21+ features (slog, signal.NotifyContext)
- Output ONLY valid JSON, nothing else.`
}

// ═══════════════════════════════════════════════════════
// STAGE 2: Intent Generation Prompt (REPLACES direct code gen)
// ═══════════════════════════════════════════════════════

// GoIntentPrompt generates the intent prompt for Go files (Stage 2).
// Instead of generating code, the model describes WHAT the code should do.
func GoIntentPrompt(planJSON, shellFilesJSON, description string, fileInfo StageFileInfo, researchStr string) string {
	return fmt.Sprintf(`You are a code architect. Convert this requirement into a STRUCTURED INTENT description.

DO NOT write actual source code. Describe WHAT the code should do in structured JSON.

## Architecture Plan
%s

## Existing Shell Files (context only)
%s

## Requirement
%s

## File to generate
Path: %s
Purpose: %s

%s

## OUTPUT FORMAT (valid JSON only)
{
  "functions": [
    {
      "name": "module_daemon",
      "description": "what this does",
      "type": "daemon",
      "output_path": "%s",
      "config": {
        "config_path": "/data/adb/modules/<id>/config.json",
        "check_interval": "300"
      },
      "data_structures": [
        {
          "name": "ModuleConfig",
          "fields": [
            {"name": "CheckInterval", "type": "int"},
            {"name": "Threshold", "type": "float64"}
          ]
        }
      ],
      "logic": {
        "init_steps": ["Load config", "Validate thresholds"],
        "main_loop": "Read sensor, compare thresholds, trigger actions",
        "triggers": [
          {"condition": "temperature > threshold", "action": "log warning and take protective action"},
          {"condition": "value normal", "action": "reset counters"}
        ],
        "cleanup_steps": ["Log shutdown", "Release resources"]
      }
    }
  ]
}

## RULES
- Describe logic in PLAIN ENGLISH, not code
- Config values as strings (synthesizer handles types)
- Data structures use Go types (int, float64, string, bool)
- Triggers: clear condition-action pairs
- Follow the best practices from research
- Output ONLY valid JSON, nothing else.`,
		planJSON, shellFilesJSON, description,
		fileInfo.Path, fileInfo.Description, researchStr,
		fileInfo.Path,
	)
}

// CIntentPrompt generates the intent prompt for C files (Stage 2).
func CIntentPrompt(planJSON, shellFilesJSON, description string, fileInfo StageFileInfo, researchStr string) string {
	return fmt.Sprintf(`You are a code architect. Convert this requirement into a STRUCTURED INTENT description.

DO NOT write actual source code. Describe WHAT the code should do in structured JSON.

## Architecture Plan
%s

## Existing Shell Files (context only)
%s

## Requirement
%s

## File to generate
Path: %s
Purpose: %s

%s

## OUTPUT FORMAT (valid JSON only)
{
  "functions": [
    {
      "name": "system_watchdog",
      "description": "what this does",
      "type": "watchdog",
      "output_path": "%s",
      "config": {
        "interval_seconds": "30"
      },
      "data_structures": [],
      "logic": {
        "init_steps": ["Set up signal handlers"],
        "main_loop": "Check system condition, take action if needed",
        "triggers": [
          {"condition": "condition detected", "action": "perform system action"}
        ],
        "cleanup_steps": ["Log shutdown"]
      }
    }
  ]
}

## RULES
- Describe logic in PLAIN ENGLISH
- Use POSIX API (no Android-specific headers)
- C89 style: declare variables before use
- Output ONLY valid JSON, nothing else.`,
		planJSON, shellFilesJSON, description,
		fileInfo.Path, fileInfo.Description, researchStr,
		fileInfo.Path,
	)
}

// GoStagePrompt generates a Go source file (Stage 2).
// One file at a time with full project context to avoid truncation.
func GoStagePrompt(planJSON, shellFilesJSON, description string, fileInfo StageFileInfo) string {
	prompt := `Generate a complete Go source file for an Android Magisk module.

## Architecture Plan
` + planJSON + `

## Existing Shell Files (DO NOT regenerate, these are for context only)
` + shellFilesJSON + `

## Requirement
` + description + `

## File to generate
Path: ` + fileInfo.Path + `
Purpose: ` + fileInfo.Description + `

## CRITICAL GO RULES (violations = compile failure)
1. Package: package main (for main.go) or package <name>
2. Import only standard library — NO cgo, NO unsafe, NO third-party
3. Use ONLY: int, int64, string, bool, float64, []byte, error, map, slice
4. Initialize ALL variables: x := 0 or var x int = 0
5. Handle ALL errors: if err != nil { return err }
6. NO $digit in strings — use fmt.Sprintf("value=%d", val) instead of "value=$val"
7. Graceful shutdown: signal.Notify + os.Signal channel
8. File paths: /data/adb/modules/<module-id>/ for config, /data/local/tmp/ for output
9. Logging: log.Println / log.Printf to stdout (Magisk captures it)
10. Close ALL open resources (files, connections)
11. Each function MUST be complete — no TODO, no placeholders, no "..."
12. Minimum 100 lines of real business logic

## OUTPUT FORMAT
{"files":[{"path":"` + fileInfo.Path + `","content":"..."}]}

Return ONLY valid JSON. Full Go source code in content field with \\n for newlines.`
	return InjectRAGContext(prompt, description, 2)
}

// CStagePrompt generates a C source file (Stage 2).
func CStagePrompt(planJSON, shellFilesJSON, description string, fileInfo StageFileInfo) string {
	prompt := `Generate a complete C source file for an Android Magisk module.

## Architecture Plan
` + planJSON + `

## Existing Shell Files (context only, DO NOT regenerate)
` + shellFilesJSON + `

## Requirement
` + description + `

## File to generate
Path: ` + fileInfo.Path + `
Purpose: ` + fileInfo.Description + `

## CRITICAL C RULES (violations = compile failure)
1. Declare ALL variables before use (C89 style)
2. Initialize ALL variables: int x = 0; not int x;
3. Include headers: <stdio.h>, <stdlib.h>, <string.h>, <unistd.h>, <signal.h>, <sys/stat.h>
4. Use POSIX API only (no Android-specific headers like <android/log.h>)
5. For strings: use char buf[256]; snprintf(buf, sizeof(buf), "fmt %d", val);
6. NO $digit in strings
7. Main: int main(int argc, char *argv[])
8. Return 0 on success, non-zero on failure
9. Signal handling: signal(SIGTERM, handler); signal(SIGINT, handler);
10. Each function MUST be complete — no TODO, no placeholders
11. Minimum 100 lines of real business logic
12. Proper resource cleanup before exit

## OUTPUT FORMAT
{"files":[{"path":"` + fileInfo.Path + `","content":"..."}]}

Return ONLY valid JSON. Full C source code in content field with \\n for newlines.`
	return InjectRAGContext(prompt, description, 2)
}

// BuildSystemPrompt generates build scripts (Stage 3).
func BuildSystemPrompt(planJSON, sourceFilesJSON, description string) string {
	// Parse plan to determine which build files are needed
	var plan struct {
		GoFiles    int `json:"go_files"`
		CFiles     int `json:"c_files"`
		ShellFiles int `json:"shell_files"`
	}
	json.Unmarshal([]byte(planJSON), &plan)

	filesToGenerate := []string{"build.sh"}

	requirement := `## Requirement
` + description + `
`
	if plan.GoFiles > 0 {
		filesToGenerate = append(filesToGenerate, "go.mod")
		requirement += `
## Go Compilation
- Module path: github.com/moduforge/module
- Go version: go 1.21
- NO external dependencies — stdlib only
- GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"
`
	}
	if plan.CFiles > 0 {
		filesToGenerate = append(filesToGenerate, "Makefile")
		requirement += `
## C Compilation
- Cross-compiler: aarch64-linux-android-gcc or clang
- Target: arm64-v8a, Flags: -static -O2 -Wall
`
	}
	if plan.ShellFiles == 0 && plan.GoFiles == 0 && plan.CFiles == 0 {
		// All shell, no compilation needed
		filesToGenerate = []string{"build.sh"}
		requirement += `
## Shell-Only Module
No Go or C source files needed. build.sh should only package the shell scripts into the module zip.
`
	}

	prompt := `Generate build scripts and config files for this Android Magisk module.

## Architecture Plan
` + planJSON + `

## Generated Source Files (for reference — DO NOT regenerate)
` + sourceFilesJSON + `

` + requirement + `
## Files to generate:
` + strings.Join(filesToGenerate, "\n") + `
1. build.sh:
   - #!/bin/sh
   - Create ./bin/ directory
   - For Go: GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ./bin/<name> ./src/
   - For C: $CC -static -o ./bin/<name> src/main.c
   - For Shell-only: just echo "Shell-only module, no compilation needed"
   - NO mv/cp commands — just compile to ./bin/

## OUTPUT FORMAT
{"files":[{"path":"build.sh","content":"..."},{"path":"go.mod","content":"..."}]}

Return ONLY valid JSON. Full file contents with \\n for newlines.`
	return InjectRAGContext(prompt, description, 2)
}

// InjectRAGContext retrieves relevant code examples from the knowledge base
// and appends them to a prompt as few-shot examples.
// If the RAG system is not initialized or no relevant chunks are found,
// the original prompt is returned unchanged.
func InjectRAGContext(prompt string, description string, topK int) string {
	chunks, err := rag.SearchRelevant(description, topK)
	if err != nil || len(chunks) == 0 {
		return prompt
	}

	var sb strings.Builder
	sb.WriteString(prompt)
	sb.WriteString("\n\n## Relevant examples from knowledge base\n")
	sb.WriteString("Use these as reference patterns, but adapt them to the specific requirement:\n\n")

	for i, chunk := range chunks {
		source := chunk.Source
		if meta, ok := chunk.Metadata["file"]; ok && meta != "" {
			source = meta
		}
		sb.WriteString(fmt.Sprintf("### Example %d (from %s)\n", i+1, source))
		sb.WriteString("```\n")
		sb.WriteString(chunk.Content)
		sb.WriteString("\n```\n\n")
	}

	return sb.String()
}
// BuildSystemPromptWithLang generates build scripts (Stage 3) with explicit language awareness.
// Unlike BuildSystemPrompt which parses the plan JSON to guess languages, this function
// receives the actual detected language profile from analyzing generated files.
func BuildSystemPromptWithLang(planJSON, sourceFilesJSON, description string, hasGo, hasC, hasRust, shellOnly bool, langProfile string) string {
	var filesToGenerate []string
	var requirement string

	switch langProfile {
	case "PURE_GO":
		filesToGenerate = []string{"build.sh", "go.mod"}
		requirement = `## Go Module Build
- Module path: github.com/moduforge/module
- Go version: 1.21+
- NO external dependencies — stdlib only
- build.sh: GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ./bin/<module_name> ./src/
- go.mod: standard Go module definition
`

	case "PURE_C":
		filesToGenerate = []string{"build.sh", "Makefile"}
		requirement = `## C Build (Makefile + Cross-Compilation)
- Cross-compiler: aarch64-linux-android-gcc or clang (from NDK)
- Makefile targets: clean, build, install
- CFLAGS: -static -O2 -Wall -Werror -Wextra -pedantic
- build.sh: wrapper that invokes make
- Target architecture: arm64-v8a (aarch64)
`

	case "PURE_RUST":
		filesToGenerate = []string{"build.sh", "Cargo.toml"}
		requirement = `## Rust Build (Cargo)
- Target: aarch64-linux-android
- Use cross or cargo-ndk for Android cross-compilation
- build.sh: cross build --target aarch64-linux-android --release
- Cargo.toml: [package] + [[bin]] section
- NO external crates unless absolutely necessary — prefer stdlib
`

	case "MIXED_GO_C":
		filesToGenerate = []string{"build.sh"}
		requirement = `## Mixed Go + C Build
- build.sh must handle BOTH Go and C compilation
- Go: GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"
- C: aarch64-linux-android-gcc -static -O2 -Wall -Werror -o ./bin/<name> src/main.c
- Order: compile C first, then Go (Go may reference C objects if CGO is used)
- If CGO_ENABLED=1 is needed, set CC=aarch64-linux-android-gcc
`

	case "SHELL_ONLY":
		filesToGenerate = []string{"build.sh"}
		requirement = `## Shell-Only Module
- No Go or C source files to compile
- build.sh should only package shell scripts into the module zip
- Just echo "Shell-only module, no compilation needed" and create a basic zip
`

	default:
		filesToGenerate = []string{"build.sh"}
		requirement = `## Build System
- Generate a build.sh appropriate for the source files present
`
	}

	prompt := `Generate build scripts and config files for this Android Magisk module.

## Language Profile
` + langProfile + `

## Architecture Plan
` + planJSON + `

## Generated Source Files (for reference — DO NOT regenerate)
` + sourceFilesJSON + `

## Requirement
` + description + `

` + requirement + `
## Files to generate:
` + strings.Join(filesToGenerate, "\n") + `

## build.sh Rules:
- #!/bin/sh (not bash)
- Create ./bin/ directory: mkdir -p ./bin
- NO mv/cp commands — just compile directly to ./bin/
- Include error handling: set -e
- For Go: GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ./bin/<name> ./src/
- For C: $CC -static -O2 -Wall -o ./bin/<name> src/main.c
- For Shell-only: just echo "Shell-only module, no compilation needed"
- Print success message on completion

## OUTPUT FORMAT
{"files":[{"path":"build.sh","content":"..."},{"path":"go.mod","content":"..."}]}

Return ONLY valid JSON. Full file contents with \\n for newlines.`
	return InjectRAGContext(prompt, description, 2)
}
