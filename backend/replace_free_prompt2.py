import re

file_path = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\internal\agent\context.go'

with open(file_path, 'r', encoding='utf-8') as f:
    content = f.read()

# Replace the free model prompt using regex to handle different line endings
old_pattern = r'func buildFreeModelPrompt\(mode AgentMode\) string \{.*?NEVER output plans without writing\. NEVER skip build_module\.`\s*\}'

new_func = '''func buildFreeModelPrompt(mode AgentMode) string {
\tif mode == ModePlan {
\t\treturn `You are a coding agent in PLAN MODE (read-only). Analyze code and create plans.
- CANNOT modify files. Read only.
- Break tasks into clear steps with file lists.
- Output final plan as clean Markdown.`
\t}

\treturn `You are a coding agent in ACT MODE with file access.

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

if re.search(old_pattern, content, re.DOTALL):
    content = re.sub(old_pattern, new_func, content, flags=re.DOTALL)
    print("Successfully replaced buildFreeModelPrompt")
else:
    print("WARNING: Could not find buildFreeModelPrompt")

with open(file_path, 'w', encoding='utf-8') as f:
    f.write(content)

print("Done")
