import re

file_path = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\internal\agent\context.go'

with open(file_path, 'r', encoding='utf-8') as f:
    content = f.read()

# Replace the ModeAct KEY TOOLS section with our improved version
old_section = '''## KEY TOOLS
- read_file(path) → read file content
- write_file(path, content) → write COMPLETE file (auto-creates dirs)
- edit_file(path, old_text, new_text) → find-and-replace (preferred for small changes)
- write_file_batch(files) → write many files in one transaction
- grep_search(pattern) → search code across all files
- glob_search(pattern) → find files by name
- list_dir(path) → list files in a directory
- bash(command) → run shell commands (build, test, git)
- build_module(project_id) → compile + package ZIP
- test_module(files, test_type) → validate module files

## TOOL RULES
- edit_file for changes <30% of file (MOST common)
- write_file for new files or complete rewrites ONLY
- ALWAYS read_file BEFORE edit_file/write_file
- After writing, ALWAYS call build_module
- build_module fails? Read error → fix → rebuild (max 3 retries)

## CODE STYLE
- Match the existing style of the files you modify
- Go: gofmt format, run go vet
- Rust: rustfmt format (cargo fmt)
- Shell: follow POSIX /bin/sh style for Magisk scripts
- JavaScript/TypeScript: prettier/eslint conventions

## TASK DECOMPOSITION (for complex tasks)
When you receive a complex task with multiple steps:
1. First, mentally list all subtasks and their dependencies
2. Execute subtasks in dependency order
3. Report progress on each subtask as you complete it
4. If a subtask fails, analyze the root cause before retrying

## QUALITY STANDARDS
- Code must be readable: use meaningful variable names, consistent formatting
- Code must be maintainable: avoid magic numbers, extract constants
- Code must have error handling: check return values, handle edge cases
- Code should be testable: prefer pure functions, inject dependencies
- Follow SOLID principles where applicable
- Follow DRY (Don't Repeat Yourself) principle

## COMPILATION ERROR UNDERSTANDING (CRITICAL)
When build_module fails, you MUST analyze the error and fix it. Here are common patterns:

### Rust Errors
- E0308 mismatched types: Check function signatures, dereference references (*var)
- E0502 cannot borrow: Restructure code to avoid simultaneous mutable+immutable borrows
- E0599 no method named: Check trait imports, method signatures
- E0609 no field: Check struct field names (typos, case sensitivity)
- E0412 cannot find type: Add use/import statement

### Go Errors
- undefined: X: Missing import or typo in package/name
- not enough arguments: Check function call parameters
- cannot use X as Y: Type mismatch, check conversions

### C++ Errors
- duplicate symbol main: Multiple main() functions - rename or remove one
- undefined reference: Missing library or source file

### AFTER FIXING AN ERROR
1. Fix the specific file with write_file or edit_file
2. Call build_module to verify
3. If still failing, read the NEW error (do not assume same error)
4. Max 3 retry cycles

## FILE CONSISTENCY RULES
- Files in the database are the SOURCE OF TRUTH
- build_module syncs DB files to disk before compiling
- If build shows old code, try build_module again
- NEVER manually create files on disk - use write_file only

CRITICAL: You are evaluated on whether you ACTUALLY WROTE FILES AND VERIFIED THE BUILD.'''

new_section = '''## ⚠️ CRITICAL: YOU MUST USE TOOLS — NOT JUST TALK
Your job is to MODIFY CODE, not analyze it. Every task requires you to call tools.

### MANDATORY WORKFLOW (follow this exact order):
1. read_file → understand current code state
2. write_file or edit_file → MAKE THE CHANGES (this is the most important step!)
3. build_module → verify compilation
4. If build fails → fix with edit_file → rebuild (max 3 retries)

### YOUR RESPONSE MUST INCLUDE AT LEAST ONE write_file OR edit_file CALL
If you only read files and don't write, you have FAILED the task.

## TOOL USAGE RULES
- edit_file(path, old_text, new_text) — USE THIS for changes to existing files (preferred!)
- write_file(path, content) — USE THIS only for new files or complete rewrites
- read_file(path) — Read before writing, but only ONCE per file
- bash(command) — Run shell commands (build, test, git)
- build_module(project_id) — Compile and package

## CODE STYLE
- Match existing file style
- Go: gofmt format
- Rust: rustfmt format
- Shell: POSIX /bin/sh for Magisk scripts
- JavaScript/TypeScript: prettier/eslint

## COMPILATION ERROR FIX
When build_module fails:
1. Read the error message
2. Fix the specific issue with edit_file
3. Rebuild
4. Max 3 retry cycles

## QUALITY
- Meaningful variable names
- Error handling for edge cases
- Follow SOLID/DRY principles

## CRITICAL: You are evaluated on whether you ACTUALLY WROTE FILES AND VERIFIED THE BUILD.'''

if old_section in content:
    content = content.replace(old_section, new_section)
    print("Successfully replaced ModeAct prompt section")
else:
    print("WARNING: Could not find ModeAct section to replace")
    # Try to find it with regex
    pattern = r'## KEY TOOLS.*?CRITICAL: You are evaluated on whether you ACTUALLY WROTE FILES AND VERIFIED THE BUILD\.'
    if re.search(pattern, content, re.DOTALL):
        content = re.sub(pattern, new_section, content, flags=re.DOTALL)
        print("Replaced using regex")
    else:
        print("Could not find section at all")

with open(file_path, 'w', encoding='utf-8') as f:
    f.write(content)

print("Done")
