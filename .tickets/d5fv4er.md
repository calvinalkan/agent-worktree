---
schema_version: 1
id: d5fv4er
status: closed
closed: 2026-01-08T14:32:24Z
blocked-by: [d5ftwxr]
created: 2026-01-08T13:33:47Z
type: feature
priority: 2
---
# Implement wt merge command

Implement `wt merge` command to merge a worktree branch back into its base branch (or another branch via --into).

## Command

```
wt merge [--into <branch>] [--keep] [--dry-run]
```

## Flags

- `--into <branch>` - merge into this branch instead of base_branch from metadata
- `--keep` - skip cleanup entirely; worktree and branch remain (run `wt remove` to delete later)
- `--dry-run` - show what would happen without doing anything

## Flow

```
PHASE 1: ALL CHECKS (fail fast, no side effects)
================================================

1. Read metadata
   - Read .wt/worktree.json
   - Get current branch (feature)
   - Get target: --into flag OR base_branch from metadata
   - Error: "reading worktree metadata: <err> (are you in a wt-managed worktree?)"

2. Validate branches
   - Check target branch exists
   - Check feature != target
   - Error: "validating branches: branch '<target>' does not exist"
   - Error: "validating branches: already on '<target>', nothing to merge"

3. Check current worktree clean
   - Run: git status --porcelain
   - Error: "checking worktree status: uncommitted changes (commit or stash before merging)"

4. Check target worktree clean (if checked out somewhere)
   - Find target worktree: git worktree list --porcelain
   - If found, check it's clean
   - Error: "checking target worktree: '<path>' has uncommitted changes (commit or stash there first)"


PHASE 2: EXECUTE (with retry loop)
==================================

5. Rebase + Merge
   - See "Concurrent Merge Handling" below

6. Cleanup (unless --keep)
   - Use shared cleanupWorktree function (from d5ftwxr)
   - Runs pre-delete hooks, removes worktree, deletes branch, prunes

7. Print success
```

## Error Format

All errors follow this structure:
```
<doing what>: <what went wrong> (<actionable hint>)
```

Examples:
- `reading worktree metadata: file not found (are you in a wt-managed worktree?)`
- `checking worktree status: uncommitted changes (commit or stash before merging)`
- `rebasing onto main: conflict in src/auth.go (see diff commands below)`
- `merging into main: not fast-forward (target moved, retrying...)`

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (merge completed, cleanup may have warnings) |
| 1 | Error (merge failed, conflict, validation error, etc.) |

Note: If merge succeeds but cleanup fails, exit 0 with warning on stderr.
The important operation (merge) succeeded.

## Concurrent Merge Handling

When multiple agents merge to the same target branch, a race condition can occur:

```
Agent A                              Agent B
────────                             ────────
rebase onto main (M)              
                                     rebase onto main (M)
merge -> main moves to M-A        
                                     merge -> FAIL! (not FF-able)
```

After A merges, B's branch is based on old main and can't fast-forward.

**Solution: Auto-retry loop**

```go
maxRetries := 3
var mergeErr error

for attempt := 1; attempt <= maxRetries; attempt++ {
    // Rebase onto target
    err := git.Rebase(ctx, wtPath, target)
    if err != nil {
        if isConflict(err) {
            // Conflict - abort and fail (can't auto-resolve)
            return handleConflict(err)
        }
        // Unknown error - try to abort rebase to leave clean state
        abortErr := git.RebaseAbort(ctx, wtPath)
        return errors.Join(
            fmt.Errorf("rebasing onto %s: %w", target, err),
            abortErr, // nil if abort succeeded, error if it failed too
        )
    }
    
    // Try merge
    err = git.Merge(ctx, targetWt, feature, true) // ff-only
    if err == nil {
        mergeErr = nil
        break
    }
    
    if !isNotFFError(err) {
        return err // Some other error, don't retry
    }
    
    mergeErr = err
    
    if attempt == maxRetries {
        break // Don't print retry message on last attempt
    }
    
    // Target moved, retry
    fprintf(stderr, "Target moved, retrying (%d/%d)...\n", attempt, maxRetries)
}

if mergeErr != nil {
    return fmt.Errorf("merge failed after %d retries (high contention on target branch)", maxRetries)
}
```

**Exponential backoff with jitter:**

```go
// Classic exponential backoff with jitter to avoid thundering herd
baseDelay := 100 * time.Millisecond
maxDelay := 2 * time.Second

func backoff(attempt int) time.Duration {
    exp := math.Pow(2, float64(attempt))
    delay := time.Duration(exp) * baseDelay
    if delay > maxDelay {
        delay = maxDelay
    }
    // Jitter: random between 0 and calculated delay
    return time.Duration(rand.Int63n(int64(delay)))
}
```

- Attempt 1: rand(0, 200ms)
- Attempt 2: rand(0, 400ms)
- Attempt 3: rand(0, 800ms)

Jitter prevents synchronized retries when multiple agents fail simultaneously.

**Retry only when:**
- FF merge fails because target moved (not a conflict)

**Don't retry when:**
- Rebase conflict (requires manual resolution)
- Other git errors
- Max retries exceeded

## Dry Run Output

```
Dry run: wt merge feature → main

Checks:
  ✓ Current worktree is clean
  ✓ Target branch 'main' exists  
  ✓ Target worktree /path/to/repo is clean

Would execute:
  1. Rebase 'feature' onto 'main' (2 commits to replay)
  2. Fast-forward 'main' to 'feature' (in /path/to/repo)
  3. Run pre-delete hooks
  4. Remove worktree: /path/to/worktrees/feature
  5. Delete branch: feature

No changes made.
```

## Error Handling

Each error includes:
- What failed
- Current state
- Actionable hint to fix

Use errors.Join for partial failures. Key states:
- Rebase conflict: abort rebase, worktree clean, print diff commands
- Merge fails after rebase: branch is rebased, can retry
- Cleanup fails: merge succeeded, warn but don't fail

## Git Layer Additions (git.go)

```go
func (g *Git) Rebase(ctx context.Context, dir, target string) error
func (g *Git) RebaseAbort(ctx context.Context, dir string) error
func (g *Git) ConflictingFiles(ctx context.Context, dir string) ([]string, error)
func (g *Git) IsClean(ctx context.Context, dir string) (bool, error)
func (g *Git) FindWorktreeForBranch(ctx context.Context, dir, branch string) (string, error)
func (g *Git) Merge(ctx context.Context, dir, branch string, ffOnly bool) error
func (g *Git) BranchExists(ctx context.Context, dir, branch string) (bool, error)
// Implementation: git show-ref --verify --quiet refs/heads/<branch>
```

## Test Scenarios

1. Simple merge (no rebase needed)
2. Merge with rebase (target moved ahead)
3. Conflict during rebase
4. --into flag (merge to different branch)
5. --keep flag (keep worktree)
6. --dry-run flag
7. Dirty current worktree (error)
8. Dirty target worktree (error)
9. Nested worktree merge (wt-sub → wt-feature)
10. Hooks (pre-delete runs)
11. Concurrent merge (multiple agents merge simultaneously)
12. Target branch doesn't exist (error with hint)
13. Already up-to-date (no commits - succeed with informative message)
14. Run from non-worktree directory (error with hint)
15. Merge into self - feature == target (error)

## Concurrent Test Example

```go
func TestMerge_Concurrent(t *testing.T) {
    c := NewCLITester(t)
    initRealGitRepo(t, c.Dir)
    c.WriteFile("config.json", `{"base": "worktrees"}`)
    
    n := 5
    wtPaths := make([]string, n)
    
    // Create N worktrees, each with a commit
    for i := 0; i < n; i++ {
        name := fmt.Sprintf("feature-%d", i)
        out := c.MustRun("--config", "config.json", "create", "-n", name)
        wtPaths[i] = extractPath(out)
        c.GitCommit(wtPaths[i], fmt.Sprintf("file-%d.txt", i), "content", fmt.Sprintf("commit %d", i))
    }
    
    // Merge all concurrently
    var wg sync.WaitGroup
    errs := make([]error, n)
    
    for i := 0; i < n; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            wt := NewCLITesterAt(t, wtPaths[idx])
            _, stderr, code := wt.Run("--config", "../config.json", "merge")
            if code != 0 {
                errs[idx] = fmt.Errorf("feature-%d: %s", idx, stderr)
            }
        }(i)
    }
    
    wg.Wait()
    
    // All should succeed
    for _, err := range errs {
        if err != nil {
            t.Error(err)
        }
    }
    
    // All commits on master
    for i := 0; i < n; i++ {
        if !c.BranchContainsCommit("master", fmt.Sprintf("commit %d", i)) {
            t.Errorf("master missing commit %d", i)
        }
    }
    
    // All worktrees removed
    for i := 0; i < n; i++ {
        if c.WorktreeExists(fmt.Sprintf("feature-%d", i)) {
            t.Errorf("feature-%d should be removed", i)
        }
    }
}
```

If retry logic works → all succeed. If broken → some fail. No orchestration needed.

## Test Helpers to Add (testing_test.go)

```go
func (c *CLI) GitCommit(dir, filename, content, message string)
func (c *CLI) BranchExists(branch string) bool
func (c *CLI) WorktreeExists(name string) bool
func (c *CLI) BranchContainsCommit(branch, message string) bool
func (c *CLI) GetCommitCount(branch string) int
```

## Acceptance Criteria

- wt merge rebases onto target and fast-forward merges
- --into flag allows merging to different branch than base_branch
- --keep flag preserves worktree after merge
- --dry-run shows plan without executing
- Conflict aborts cleanly with diff commands printed
- Works with nested worktrees (target checked out elsewhere)
- Uses shared cleanupWorktree function
- All error states have clear messages with hints
- Auto-retries if target moves between rebase and merge (up to 3 times)
- E2E tests cover all scenarios listed above
