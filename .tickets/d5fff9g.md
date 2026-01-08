---
schema_version: 1
id: d5fff9g
status: closed
closed: 2026-01-08T00:24:49Z
blocked-by: []
created: 2026-01-08T00:17:42Z
type: task
priority: 2
---
# Add .wt/worktree.json exclusion to git exclude before worktree creation

Before creating a worktree, add .wt/worktree.json exclusion to .git/info/exclude (git common dir) if not already present. This ensures the worktree metadata file is not tracked by git.

## Approach

1. Get git common dir (already available in create flow)
2. Read `<git-common-dir>/info/exclude`
3. Append `.wt/worktree.json` if not already present
4. Write it back

## Error handling

If this fails for any reason:
- Print warning to stderr
- Tell user to add the exclusion themselves
- Continue with worktree creation (don't fail)
