---
schema_version: 1
id: d5ftwxr
status: closed
closed: 2026-01-08T13:21:27Z
blocked-by: []
created: 2026-01-08T13:17:43Z
type: task
priority: 2
---
# Extract shared cleanupWorktree function from remove command

Extract the worktree cleanup logic from cmd_remove.go into a reusable function that can be shared between 'wt remove' and 'wt merge' commands.

The function should handle:
1. Running pre-delete hook
2. Removing the worktree (git worktree remove)
3. Deleting the branch (optional)
4. Pruning worktree metadata

Signature:
```go
func cleanupWorktree(
    ctx context.Context,
    stdout io.Writer,
    git *Git,
    hookRunner *HookRunner,
    info *WorktreeInfo,
    wtPath, mainRepoRoot, sourceDir string,
    deleteBranch, force bool,
) error
```

The remove command should be refactored to use this function after its interactive prompts and dirty checks.

## Acceptance Criteria

- New cleanupWorktree function in cmd_remove.go (or separate file)
- Remove command refactored to use cleanupWorktree
- All existing remove tests still pass
- Function handles errors with errors.Join
- Function is exported/accessible for merge command to use
