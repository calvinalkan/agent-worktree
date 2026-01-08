---
schema_version: 1
id: d5fs9f0
status: open
blocked-by: []
created: 2026-01-08T11:27:56Z
type: feature
priority: 2
---
# Extend wt info to accept optional worktree identifier

Allow `wt info` to accept an optional name/agent_id/id argument to lookup any worktree, not just the current one.

Current:
  wt info              # current worktree only

Proposed:
  wt info              # current worktree (unchanged)
  wt info foo          # lookup by name or agent_id
  wt info 3            # lookup by id
  wt info foo --field path   # just the path
  wt info foo --json         # full JSON output

This enables shell integration for `wt switch` without adding new commands or jq dependency.

## Acceptance Criteria

- Accept optional positional arg for worktree identifier
- Match by name, agent_id, or id (numeric)
- Error if no match found
- Existing --field and --json flags work with lookup
- No arg = current worktree (existing behavior unchanged)
