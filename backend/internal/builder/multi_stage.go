package builder

// Multi-stage generation engine for free models.
// Core insight: split one 30K-token generation into 3-4 focused stages,
// each generating 1-2 files (~5K tokens), staying within free model limits.

// StagePlan holds the architecture plan determined in Stage 0.
type StagePlan struct {
	ID          string            `json:"id"`          // e.g. "battery-monitor"
	Name        string            `json:"name"`        // Human-readable name
	ModuleType  string            `json:"module_type"` // service, tool, tweak
	Languages   []string          `json:"languages"`   // ["shell"], ["go","shell"], etc.
	ShellFiles  []StageFileInfo   `json:"shell_files"` // module.prop, customize.sh, etc.
	GoFiles     []StageFileInfo   `json:"go_files"`    // src/main.go, etc.
	CFiles      []StageFileInfo   `json:"c_files"`     // src/main.c, etc.
	BuildFiles  []StageFileInfo   `json:"build_files"` // build.sh, go.mod, etc.
	ExtraFiles  []StageFileInfo   `json:"extra_files"` // service.sh, uninstall.sh, config, etc.
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
	return `Generate Shell scripts for this Android Magisk module.

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

## OUTPUT FORMAT
{"files":[{"path":"module.prop","content":"..."},{"path":"customize.sh","content":"..."}]}

Return ONLY valid JSON. Full file contents with \\n for newlines in content field.`
}

// GoStagePrompt generates a Go source file (Stage 2).
// One file at a time with full project context to avoid truncation.
func GoStagePrompt(planJSON, shellFilesJSON, description string, fileInfo StageFileInfo) string {
	return `Generate a complete Go source file for an Android Magisk module.

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
}

// CStagePrompt generates a C source file (Stage 2).
func CStagePrompt(planJSON, shellFilesJSON, description string, fileInfo StageFileInfo) string {
	return `Generate a complete C source file for an Android Magisk module.

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
}

// BuildSystemPrompt generates build scripts (Stage 3).
func BuildSystemPrompt(planJSON, sourceFilesJSON, description string) string {
	return `Generate build scripts and config files for this Android Magisk module.

## Architecture Plan
` + planJSON + `

## Generated Source Files (for reference — DO NOT regenerate)
` + sourceFilesJSON + `

## Requirement
` + description + `

## Files to generate:
1. go.mod (if Go source exists):
   - Module path: github.com/moduforge/module/<id>
   - Go version: go 1.21
   - NO external dependencies — stdlib only
2. build.sh:
   - #!/bin/sh
   - Set up NDK/Go cross-compilation environment
   - Compile each source file to binary in ./bin/
   - Go: GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ./bin/<name> ./src/
   - C: $CC -static -o ./bin/<name> src/main.c (use NDK compiler)
   - Make ./bin/ directory if needed
   - NO mv/cp commands — just compile to ./bin/
3. Makefile (if C source exists):
   - Cross-compiler: aarch64-linux-android-gcc or clang
   - Target: arm64-v8a
   - Flags: -static -O2 -Wall

## OUTPUT FORMAT
{"files":[{"path":"build.sh","content":"..."},{"path":"go.mod","content":"..."}]}

Return ONLY valid JSON. Full file contents with \\n for newlines.`
}
