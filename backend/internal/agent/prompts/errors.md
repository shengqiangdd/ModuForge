<error_reference>
<rust_errors>
| Error | Solution |
|-------|----------|
| `cannot find value` | Check variable name, scope, import |
| `mismatched types` | Verify type compatibility, add conversion |
| `no method named` | Check trait import, method signature |
| `unused variable` | Prefix with `_` or remove |
| `lifetime mismatch` | Add explicit lifetime annotations |
| `expected one of` | Check syntax, missing comma/semicolon |
| `unreachable pattern` | Check match arms, remove duplicate |
</rust_errors>

<go_errors>
| Error | Solution |
|-------|----------|
| `undefined: X` | Check import, package name |
| `cannot use X as Y` | Type assertion or conversion needed |
| `implicit assignment to nil` | Initialize variable before use |
| `redeclared in this block` | Rename or check scope |
| `imported and not used` | Remove unused import or use it |
| `syntax error` | Check brackets, semicolons |
</go_errors>

<shell_errors>
| Error | Solution |
|-------|----------|
| `command not found` | Check PATH, use full path |
| `permission denied` | Check file permissions (0755 for scripts) |
| `syntax error near` | Check quotes, brackets, semicolons |
| `unexpected end of file` | Check matching quotes/brackets |
| `no such file or directory` | Check file path, use quotes |
</shell_errors>

<build_failures>
**Strategy:**
1. **Read the error message** — It tells you exactly what's wrong
2. **Identify the file and line** — Focus on that specific location
3. **Fix the root cause** — Don't just suppress warnings
4. **Rebuild to verify** — One fix at a time
</build_failures>

<prevention>
- Match existing code style exactly
- Use type annotations for complex types
- Handle all error cases (no `_ = err`)
- Test edge cases mentally before coding
</prevention>
</error_reference>
