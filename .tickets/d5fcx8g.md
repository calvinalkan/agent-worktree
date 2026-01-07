---
schema_version: 1
id: d5fcx8g
status: closed
closed: 2026-01-07T22:25:46Z
blocked-by: []
created: 2026-01-07T21:22:42Z
type: task
priority: 2
---
# Use errors.Join for rollback errors in create command

## Overview
When rollback operations fail in `execCreate`, combine errors using `errors.Join` instead of silently ignoring them.

## Current Behavior
In `create.go`, rollback errors are silently discarded:

```go
// After writeWorktreeInfo fails:
_ = git.WorktreeRemove(repoRoot, wtPath, true)
_ = git.BranchDelete(repoRoot, name, true)
return fmt.Errorf("writing worktree metadata: %w", err)

// After copyUncommittedChanges fails:
_ = git.WorktreeRemove(repoRoot, wtPath, true)
_ = git.BranchDelete(repoRoot, name, true)
return fmt.Errorf("copying uncommitted changes: %w", err)

// After post-create hook fails:
_ = git.WorktreeRemove(repoRoot, wtPath, true)
_ = git.BranchDelete(repoRoot, name, true)
return fmt.Errorf("post-create hook failed: %w", err)
```

## Problem
If rollback fails, the user has no idea:
- The worktree may be left in a broken state
- Orphaned branches may remain
- Debugging becomes difficult

## Solution
Use `errors.Join` to combine the original error with rollback errors:

```go
// After writeWorktreeInfo fails:
rmErr := git.WorktreeRemove(repoRoot, wtPath, true)
brErr := git.BranchDelete(repoRoot, name, true)
return errors.Join(
    fmt.Errorf("writing worktree metadata: %w", err),
    rmErr,
    brErr,
)
```

`errors.Join` handles nil errors gracefully - if rollback succeeds, only the original error is returned.

## Affected Locations
1. Line ~117: After `writeWorktreeInfo` fails
2. Line ~128: After `copyUncommittedChanges` fails  
3. Line ~139: After `RunPostCreate` hook fails

## Acceptance Criteria
- All three rollback locations use `errors.Join`
- Original error is always first in the joined error
- Tests verify combined error contains rollback failure info when applicable
