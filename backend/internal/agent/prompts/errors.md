# Error Handling Reference

## Common Errors & Solutions

### Rust Compilation Errors

| Error | Solution |
|-------|----------|
| `cannot find value` | Check variable name, scope, import |
| `mismatched types` | Verify type compatibility, add conversion |
| `no method named` | Check trait import, method signature |
| `unused variable` | Prefix with `_` or remove |
| `lifetime mismatch` | Add explicit lifetime annotations |

### Go Compilation Errors

| Error | Solution |
|-------|----------|
| `undefined: X` | Check import, package name |
| `cannot use X as Y` | Type assertion or conversion needed |
| `implicit assignment to nil` | Initialize variable before use |
| `redeclared in this block` | Rename or check scope |

### Build Failures

1. **Read the error message** — It tells you exactly what's wrong
2. **Identify the file and line** — Focus on that specific location
3. **Fix the root cause** — Don't just suppress warnings
4. **Rebuild to verify** — One fix at a time

### Recovery Strategy

```
IF build fails:
  1. Parse error message
  2. Locate file:line
  3. Understand the issue
  4. Apply minimal fix
  5. Rebuild
  6. IF still fails after 3 attempts:
     - Rollback to last working state
     - Ask for help with specific error
```

## Preventing Errors

- Match existing code style exactly
- Use type annotations for complex types
- Handle all error cases (no `_ = err`)
- Test edge cases mentally before coding
