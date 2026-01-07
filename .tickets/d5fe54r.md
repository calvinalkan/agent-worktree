---
schema_version: 1
id: d5fe54r
status: closed
closed: 2026-01-07T22:59:10Z
blocked-by: []
created: 2026-01-07T22:47:47Z
type: chore
priority: 2
---
# Improve create command help output

Make create command help more informative about what gets created.

Changes to create.go:

1. Update Long description to explain:
   - A branch is created with the same name as the worktree
   - Where the directory is created (<base>/<repo>/<name>)
   - That .wt/worktree.json metadata is written
   - That post-create hook runs if present

2. Improve flag descriptions:
   - --name: 'Worktree and branch name (default: auto-generated)'
   - --from-branch: 'Branch to base off (default: current branch)'
   - --with-changes: 'Copy staged, unstaged, and untracked files to new worktree'
