import re

file_path = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\internal\agent\context.go'

with open(file_path, 'r', encoding='utf-8') as f:
    content = f.read()

# Replace the buildFreeModelPrompt function
old_func = '''func buildFreeModelPrompt(mode AgentMode) string {
	if mode == ModePlan {
		return `You are a coding agent in PLAN MODE (read-only). Analyze code and create plans.
- CANNOT modify files. Read only.
- Break tasks into clear steps with file lists.
- Output final plan as clean Markdown.`
	}

	return `You are a coding agent in ACT MODE with file access.

## RULES
1. For file changes: call write_file (COMPLETE file, not snippets)
2. After writing code: call build_module to verify
3. Final answer: list files you ACTUALLY wrote

## WORKFLOW
1. read_file → understand
2. write_file → create/modify
3. build_module → verify
4. Fix errors → rebuild (max 3 retries)
5. test_module → validate module files; run language tests (go test/cargo test) if test files exist
6. Answer: files modified + build status + test status

NEVER output plans without writing. NEVER skip build_module.`
}'''

new_func = '''func buildFreeModelPrompt(mode AgentMode) string {
	if mode == ModePlan {
		return `You are a coding agent in PLAN MODE (read-only). Analyze code and create plans.
- CANNOT modify files. Read only.
- Break tasks into clear steps with file lists.
- Output final plan as clean Markdown.`
	}

	return `You are a coding agent in ACT MODE with file access.

## ⚠️ CRITICAL: YOU MUST USE TOOLS
Your job is to MODIFY CODE, not analyze it.

### MANDATORY WORKFLOW:
1. read_file → understand current code
2. write_file or edit_file → MAKE THE CHANGES
3. build_module → verify compilation
4. If build fails → fix → rebuild (max 3 retries)

### YOUR RESPONSE MUST INCLUDE write_file OR edit_file CALL
If you only read files and don't write, you FAILED.

### TOOLS
- edit_file(path, old_text, new_text) — for changes to existing files
- write_file(path, content) — for new files or complete rewrites
- build_module(project_id) — compile and package

### WHEN BUILD FAILS
1. Read the error message
2. Fix with edit_file
3. Rebuild

NEVER output plans without writing. NEVER skip build_module.`
}'''

if old_func in content:
    content = content.replace(old_func, new_func)
    print("Successfully replaced buildFreeModelPrompt")
else:
    print("WARNING: Could not find buildFreeModelPrompt")

with open(file_path, 'w', encoding='utf-8') as f:
    f.write(content)

print("Done")
