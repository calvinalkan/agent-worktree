---
schema_version: 1
id: d5ffr78
status: closed
closed: 2026-01-08T00:41:55Z
blocked-by: []
created: 2026-01-08T00:36:45Z
type: feature
priority: 2
---
# Improve wt delete UX: show deletion first, then prompt for branch cleanup

Current flow asks about branch deletion before showing what happened, which is confusing.

**Current behavior:**
1. Asks 'Delete branch?' before any action
2. Deletes worktree
3. Shows 'Deleted worktree: name'

**Proposed behavior:**
1. Delete the worktree first
2. Show confirmation: 'Deleted worktree directory: /path/to/worktree'
3. Explain the branch still exists with commits intact
4. Ask if user also wants to delete the branch

**Example output:**
```
Deleted worktree directory: /home/user/worktrees/repo/lean-puma

Branch 'lean-puma' still contains all your commits.
Also delete the branch? (y/N)
```

This makes the flow less scary and clarifies that deleting a worktree doesn't lose any code.
