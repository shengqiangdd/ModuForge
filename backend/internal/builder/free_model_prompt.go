package builder

// FreeModelBuildPrompt generates a simplified prompt optimized for free models.
// Key strategy: smaller files, explicit structure, step-by-step guidance.
func FreeModelBuildPrompt(description string, isModify bool) string {
	if isModify {
		return freeModelModifyPrompt(description)
	}
	return freeModelNewPrompt(description)
}

func freeModelNewPrompt(description string) string {
	return `Create an Android Magisk/KernelSU module for: ` + description + `

## CRITICAL RULES (violations = build failure)

### Shell modules (RECOMMENDED for best success rate)
- Only write .sh files — no Go, no C, no Rust
- Use ONLY shell syntax: if/for/case/function
- Variables: always use ${VAR} syntax in double quotes
- Numbers only: sleep 5 not sleep 1.5
- Test brackets: [ "$x" = "yes" ] (spaces inside)
- No ` + "`$1`" + ` or ` + "`$2`" + ` in wrong context

### File structure (EXACTLY these files)
Required:
- module.prop (key=value, no quotes around values)
- customize.sh (main logic)
- META-INF/com/google/android/update-binary (copy verbatim from template below)
- META-INF/com/google/android/updater-script (just "#MAGISK")

Optional:
- service.sh (runs on boot)
- uninstall.sh (cleanup on remove)

### module.prop format (copy exactly)
id=my_module
name=My Module
version=1.0.0
versionCode=1
author=AI
description=Module description
minMagisk=24000

### META-INF/com/google/android/update-binary (copy verbatim)
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

### customize.sh template
#!/system/bin/sh
# Install script
MODDIR=${0%/*}
# Your logic here
set_perm_recursive $MODPATH 0 0 0755 0644

## OUTPUT FORMAT
Return ONLY this JSON, nothing else:
{"files":[{"path":"module.prop","content":"id=my_module\nname=My Module\nversion=1.0.0\nversionCode=1\nauthor=AI\ndescription=Desc\nminMagisk=24000"},{"path":"customize.sh","content":"#!/system/bin/sh\n...your code..."}]}

IMPORTANT: The "content" field must contain the FULL file content with \n for newlines. No markdown, no code fences, just raw JSON.`
}

func freeModelModifyPrompt(description string) string {
	return `Modify existing Magisk module.

Changes needed: ` + description + `

## RULES
1. Only output files that CHANGE
2. Keep all existing files unchanged
3. Shell modules only — no Go/C/Rust
4. Variables: ${VAR} in double quotes
5. Return JSON with "changes" field

## OUTPUT FORMAT
{"files":[{"path":"file.sh","content":"full content"}],"changes":"what changed"}`
}
