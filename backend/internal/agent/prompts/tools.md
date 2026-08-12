<tool_reference>
<file_operations>
| Tool | Description | When to Use | Parameters |
|------|-------------|-------------|------------|
| `read_file` | Read file contents | Before writing, to understand current state | `path`, `start_line?`, `end_line?` |
| `write_file` | Create/overwrite file | New files or complete rewrites | `path`, `content` |
| `edit_file` | Find and replace text | Small, targeted changes (PREFERRED) | `path`, `old_text`, `new_text` |
| `delete_file` | Delete a file | Removing unnecessary files | `path` |
| `list_dir` | List directory contents | Understanding project structure | `path` |
</file_operations>

<search_tools>
| Tool | Description | When to Use | Parameters |
|------|-------------|-------------|------------|
| `glob_search` | Find files by pattern | Finding specific file types | `pattern`, `path?` |
| `grep_search` | Search file contents | Finding code patterns, functions | `pattern`, `path?`, `is_regex?` |
</search_tools>

<build_test>
| Tool | Description | When to Use | Parameters |
|------|-------------|-------------|------------|
| `build_module` | Compile project | **MANDATORY** after writing code | `project_id` |
| `test_module` | Run tests | After build passes | `project_id` |
| `syntax_checker` | Check syntax | Before building | `path`, `language?` |
</build_test>

<git_operations>
| Tool | Description | When to Use | Parameters |
|------|-------------|-------------|------------|
| `git_ops` | Git commands | Version control operations | `action`, `args?` |
</git_operations>

<tool_usage_rules>
1. **Read before write** — Always understand current state first
2. **Edit preferred** — Use edit_file for small changes, not write_file
3. **Batch writes** — Write all files, then build once
4. **Check build output** — Fix errors immediately
5. **Parallel reads** — Read multiple files simultaneously when possible
</tool_usage_rules>

<examples>
<example>
Task: Add error handling to a function
1. read_file(path="main.go") — understand current code
2. edit_file(path="main.go", old_text="...", new_text="...") — add error handling
3. build_module(project_id="my_module") — verify compilation
</example>

<example>
Task: Create new module
1. write_file(path="module.prop", content="...") — create metadata
2. write_file(path="customize.sh", content="...") — create install script
3. write_file(path="service.sh", content="...") — create service script
4. build_module(project_id="new_module") — compile and package
</example>
</examples>
</tool_reference>
