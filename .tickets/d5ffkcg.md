---
schema_version: 1
id: d5ffkcg
status: closed
closed: 2026-01-08T00:32:00Z
blocked-by: []
created: 2026-01-08T00:26:26Z
type: bug
priority: 2
---
# Improve wt info error message for main/regular branches

When running 'wt info' inside the MAIN branch or a branch that's not a worktree, the error message is confusing:

Current message:
```
error: not in a wt-managed worktree (use wt list to find worktrees)
```

Suggested improvement:
```
error: this is a regular branch, not a worktree (use wt list to find worktrees)
```

This makes it clearer that the user is in a valid git repo, just not in a worktree context.

## Acceptance Criteria

- Error message clearly indicates user is in a regular branch, not a worktree
- Tests added for this error case
