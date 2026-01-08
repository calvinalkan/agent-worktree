---
schema_version: 1
id: d5ffs4r
status: open
blocked-by: []
created: 2026-01-08T00:38:43Z
type: chore
priority: 3
---
# Add blank line between error message and usage help

When displaying an error followed by usage help, there should be a blank line separating them for better readability.

**Current output:**
```
error: unknown command: ls
wt - git worktree manager
...
```

**Expected output:**
```
error: unknown command: ls

wt - git worktree manager
...
```

This applies to all error messages that are followed by usage information.
