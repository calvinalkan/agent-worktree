---
schema_version: 1
id: d5ft4kr
status: open
blocked-by: []
created: 2026-01-08T12:25:51Z
type: bug
priority: 2
---
# wt rm: pressing Enter on branch deletion prompt defaults to Yes instead of No

After removing a worktree with 'wt rm', the prompt asks whether to also delete the branch. Currently pressing Enter (empty input) selects Yes and deletes the branch. This is dangerous - the default should be No (keep the branch) since it's the safer option.

Expected behavior:
- Enter (empty input) → No (keep the branch)
- 'y' or 'Y' → Yes (delete the branch)  
- 'n' or 'N' → No (keep the branch)

Current behavior:
- Enter (empty input) → Yes (deletes the branch)

## Acceptance Criteria

1. 'wt rm <worktree>' followed by Enter at branch prompt should NOT delete the branch
2. Only explicit 'y' or 'Y' should delete the branch
3. Regression tests added for the prompt behavior
