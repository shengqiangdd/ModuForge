# Tool Reference

## File Operations

| Tool | Description | Parameters |
|------|-------------|------------|
| `read_file` | Read file contents | `path`, `start_line?`, `end_line?` |
| `write_file` | Create/overwrite file | `path`, `content` |
| `edit_file` | Find and replace text | `path`, `old_text`, `new_text` |
| `delete_file` | Delete a file | `path` |
| `delete_dir` | Delete directory | `path` |
| `move_file` | Move/rename file | `from`, `to` |
| `list_dir` | List directory contents | `path` |

## Search Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `glob_search` | Find files by pattern | `pattern`, `path?` |
| `grep_search` | Search file contents | `pattern`, `path?`, `is_regex?` |

## Build & Test

| Tool | Description | Parameters |
|------|-------------|------------|
| `build_module` | Compile project | `project_id` |
| `test_module` | Run tests | `project_id` |
| `syntax_checker` | Check syntax | `path`, `language?` |

## Git Operations

| Tool | Description | Parameters |
|------|-------------|------------|
| `git_ops` | Git commands | `action`, `args?` |

## Usage Tips

1. **Always read before writing** — Understand current state first
2. **Use edit_file for small changes** — More efficient than full rewrite
3. **Batch related changes** — Write all files, then build once
4. **Check build output** — Fix errors immediately, don't accumulate
