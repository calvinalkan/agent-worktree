---
schema_version: 1
id: d5fe9z0
status: open
blocked-by: []
created: 2026-01-07T22:58:04Z
type: chore
priority: 2
---
# Improve info command help output

Make info command help clearer about field names and usage.

Changes to info.go:

1. Update Long description:
   - Clarify 'wt-managed worktree' means one created by 'wt create'
   - Add examples of --field usage for scripting

2. Update --field flag description to list valid values:
   - 'Output single field: name, agent_id, id, path, base_branch, created'
