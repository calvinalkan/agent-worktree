---
schema_version: 1
id: d5favh8
status: closed
closed: 2026-01-07T20:30:42Z
blocked-by: []
created: 2026-01-07T19:02:29Z
type: task
priority: 1
---
# Implement worktree base path resolution

## Overview
Implement the logic to resolve the base directory where worktrees are created.

## Background & Rationale
Per SPEC.md, the base path behavior depends on whether it's absolute or relative:

- **Absolute path** (starts with / or ~): worktrees at `<base>/<repo-name>/<worktree-name>/`
- **Relative path**: worktrees at `<base>/<worktree-name>/` (no repo name inserted)

This allows:
- Centralized worktree storage (absolute): `~/code/worktrees/my-repo/swift-fox/`
- Project-local worktrees (relative): `../worktrees/swift-fox/`

## Current State
- Config has Base field (default: ~/code/worktrees)
- ExpandPath() and IsAbsolutePath() helpers exist in run.go
- No worktree path resolution logic

## Implementation Details

### Get Repository Name
```go
// getRepoName extracts the repository name from the root path.
// Returns the last path component (directory name).
func getRepoName(repoRoot string) string {
    return filepath.Base(repoRoot)
}
```

### Resolve Worktree Path
```go
// resolveWorktreePath computes the full path for a new worktree.
// 
// If base is absolute (starts with / or ~):
//   <base>/<repo-name>/<worktree-name>
//
// If base is relative:
//   <effective-cwd>/<base>/<worktree-name>
//
// Examples:
//   base=~/code/worktrees, repo=myapp, name=swift-fox
//     => /home/user/code/worktrees/myapp/swift-fox
//
//   base=../worktrees, cwd=/code/myapp, name=swift-fox
//     => /code/worktrees/swift-fox
func resolveWorktreePath(cfg Config, repoRoot, worktreeName string) string {
    base := ExpandPath(cfg.Base)
    
    if IsAbsolutePath(cfg.Base) {
        // Absolute: include repo name in path
        repoName := getRepoName(repoRoot)
        return filepath.Join(base, repoName, worktreeName)
    }
    
    // Relative: resolve from effective cwd, no repo name
    return filepath.Join(cfg.EffectiveCwd, base, worktreeName)
}
```

### Resolve Worktree Base Directory (for scanning)
```go
// resolveWorktreeBaseDir returns the directory containing worktrees for a repo.
// Used by list/delete to find existing worktrees.
func resolveWorktreeBaseDir(cfg Config, repoRoot string) string {
    base := ExpandPath(cfg.Base)
    
    if IsAbsolutePath(cfg.Base) {
        repoName := getRepoName(repoRoot)
        return filepath.Join(base, repoName)
    }
    
    return filepath.Join(cfg.EffectiveCwd, base)
}
```

## Acceptance Criteria
- Absolute paths expand ~ and include repo name
- Relative paths resolve from effective cwd without repo name
- Paths are properly joined (no double slashes, etc.)
- Works with -C flag (uses EffectiveCwd from config)

## Testing
- Test absolute path with ~ expansion
- Test absolute path with /
- Test relative path resolution
- Test with different repo names
