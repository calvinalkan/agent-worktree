---
schema_version: 1
id: d5faxn8
status: open
blocked-by: [d5faw58]
created: 2026-01-07T19:07:01Z
type: task
priority: 3
---
# Handle concurrent worktree creation safely

## Overview
Ensure concurrent `wt create` operations don't produce duplicate IDs.

## Background & Rationale
Per SPEC.md: "No two worktrees for the same repository may have the same id; concurrent wt create operations must be handled safely."

The current ID assignment (scan existing + max+1) has a race condition if two processes run simultaneously.

## Problem Scenario
1. Process A scans worktrees, finds max ID = 5
2. Process B scans worktrees, finds max ID = 5
3. Process A creates worktree with ID 6
4. Process B creates worktree with ID 6 (DUPLICATE!)

## Solution: Use fs.Locker

The `github.com/calvinalkan/agent-task/pkg/fs` package provides a `Locker` type that wraps flock(2) and integrates with the fs.FS abstraction.

```go
// From fs package:
// type Locker struct { ... }
// func NewLocker(fs FS) *Locker
// func (l *Locker) Lock(path string) (*Lock, error)
// func (l *Locker) LockWithTimeout(path string, timeout time.Duration) (*Lock, error)
// 
// type Lock struct { ... }
// func (lk *Lock) Close() error
```

## Implementation

### Create Lock File Path Helper
```go
// worktreeLockPath returns the path to the lock file for worktree operations.
// The lock file is created in the worktree base directory.
func worktreeLockPath(baseDir string) string {
    return filepath.Join(baseDir, ".wt-create.lock")
}
```

### Updated execCreate with Locking
```go
func execCreate(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, cfg Config, fsys fs.FS, env map[string]string, flags *flag.FlagSet) error {
    _ = stdin // not used
    // ... verify git repo, resolve base branch ...
    
    repoRoot, err := gitRepoRoot(cfg.EffectiveCwd)
    if err != nil {
        return fmt.Errorf("not a git repository")
    }
    
    // Resolve base directory and ensure it exists
    baseDir := resolveWorktreeBaseDir(cfg, repoRoot)
    if err := fsys.MkdirAll(baseDir, 0o755); err != nil {
        return fmt.Errorf("creating base directory: %w", err)
    }
    
    // Acquire exclusive lock for ID generation
    // This prevents race conditions when multiple processes create worktrees
    locker := fs.NewLocker(fsys)
    lockPath := worktreeLockPath(baseDir)
    
    lock, err := locker.LockWithTimeout(lockPath, 30*time.Second)
    if err != nil {
        return fmt.Errorf("acquiring create lock: %w", err)
    }
    defer lock.Close()
    
    // Now safe to scan existing worktrees and assign ID
    existing, err := findWorktrees(fsys, baseDir)
    if err != nil {
        return fmt.Errorf("scanning existing worktrees: %w", err)
    }
    
    // Calculate next ID
    nextID := 1
    for _, wt := range existing {
        if wt.ID >= nextID {
            nextID = wt.ID + 1
        }
    }
    
    // ... rest of create logic (generate name, create worktree, etc.) ...
    // Lock is released when function returns (defer lock.Close())
}
```

## Key Points

1. **Lock file location**: `.wt-create.lock` in the worktree base directory
2. **Timeout**: 30 seconds - reasonable for typical create operations
3. **Scope**: Lock is held during scan + ID assignment + worktree creation
4. **Cleanup**: Lock released via defer, even on errors

## Why fs.Locker?
- Integrates with fs.FS abstraction (TECH_SPEC compliant)
- Uses flock(2) internally - works across processes
- Provides timeout support to avoid deadlocks
- Handles edge cases (inode checking, etc.)

## Alternative Considered: Atomic Mkdir
Could use atomic mkdir as a lock (it fails if dir exists). But:
- Only prevents name collisions, not ID collisions
- Doesn't handle cleanup well on crashes
- fs.Locker is cleaner and more robust

## Acceptance Criteria
- Concurrent creates don't produce duplicate IDs
- Uses fs.Locker (not direct syscall/os calls)
- Lock is released even on error (defer)
- Reasonable timeout (30s) to avoid deadlocks
- Lock file is in worktree base directory

## Testing
- Test that lock prevents concurrent ID assignment
- Test timeout behavior
- Test lock release on error
- Use parallel goroutines in test to verify safety
