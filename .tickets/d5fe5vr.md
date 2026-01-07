---
schema_version: 1
id: d5fe5vr
status: closed
closed: 2026-01-07T23:02:23Z
blocked-by: []
created: 2026-01-07T22:49:19Z
type: chore
priority: 2
---
# Improve list command help and empty output

Make list command help clearer and add message for empty list.

Changes to list.go:

1. Update Long description to explain:
   - Only shows wt-managed worktrees (those with .wt/worktree.json)
   - Output columns: NAME, PATH, CREATED (age)
   - Use --json for machine-readable output

2. In execList, add message to stderr when no worktrees found (table mode only):
   - Print to stderr: 'No worktrees found. Create one with: wt create'
   - JSON mode should still output empty [] to stdout (for script parsing)
