## Build Failure Recovery
1. Read the error message — identify file:line
2. Fix the root cause (don't suppress warnings)
3. Rebuild; one fix at a time
4. If still failing after 3 attempts: rollback to last working state, ask for help

## Prevention
- Match existing code style
- Handle all error cases
- Test edge cases before declaring done
