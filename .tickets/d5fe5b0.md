---
schema_version: 1
id: d5fe5b0
status: closed
closed: 2026-01-07T23:00:17Z
blocked-by: []
created: 2026-01-07T22:48:12Z
type: chore
priority: 2
---
# Improve delete command help output

Make delete command help clearer about interactive vs non-interactive behavior.

Changes to delete.go:

1. Update Long description to explain:
   - What gets deleted (worktree directory and git worktree metadata)
   - Interactive mode (terminal): prompts about branch deletion
   - Non-interactive mode (scripts): keeps branch unless --with-branch
   - Pre-delete hook runs if present and can abort deletion

2. Improve flag description:
   - --with-branch: 'Delete the git branch (skips interactive prompt)'
