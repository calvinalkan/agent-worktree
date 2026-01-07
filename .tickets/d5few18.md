---
schema_version: 1
id: d5few18
status: closed
closed: 2026-01-07T23:48:04Z
blocked-by: []
created: 2026-01-07T23:36:37Z
type: bug
priority: 2
---
# Release lock earlier in create command

Currently the lock is held via defer until the entire create function returns, including:
- git worktree add (slow on large repos)
- copying uncommitted changes
- running post-create hooks

The lock only needs to protect ID/name generation. Release it immediately after writing .wt/worktree.json metadata.

## Changes:

1. Use new context-based LockWithTimeout API (after tk repo update):

```go
// Before
const createLockTimeout = 30 * time.Second
lock, err := locker.LockWithTimeout(lockPath, createLockTimeout)

// After
const createLockTimeout = 5 * time.Second

lockCtx, lockCancel := context.WithTimeout(ctx, createLockTimeout)
defer lockCancel()

lock, err := locker.LockWithTimeout(lockCtx, lockPath)
```

2. Keep defer as safety net, add explicit Close() after metadata write:

```go
defer func() { _ = lock.Close() }() // Safety net - Close is idempotent

// ... generate ID, name, git worktree add, write metadata ...

// Release lock early - only needed for ID/name generation.
// Close is idempotent; defer above handles cleanup on early returns.
_ = lock.Close()

// ... copy changes, run hooks (no longer holding lock) ...
```

3. Reduce createLockTimeout from 30s to 5s since we're only holding it briefly.

## Tests:

Add tests that verify lock is released when context is cancelled:

1. Add `RunWithSignal(sigCh chan os.Signal, args ...string)` method to CLI test helper
2. Test: start create with a slow hook, send signal via sigCh, verify lock file is released
3. Test: verify another wt process can acquire lock after metadata is written (even if hooks still running)

The Run() function already accepts sigCh as last parameter - tests currently pass nil.
