---
schema_version: 1
id: d5fv4er
status: open
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
- `--keep` - keep worktree after merge (don't cleanup)
- `--dry-run` - show what would happen without doing anything

## Flow

```
1. Read metadata (.wt/worktree.json)
   - Get current branch (feature)
   - Get target: --into flag OR base_branch from metadata

2. Check current worktree clean
   - Error if uncommitted changes

3. Check target worktree clean (if checked out somewhere)
   - Use: git worktree list --porcelain
   - Error if target worktree has uncommitted changes

4. Rebase + Merge (with retry loop)
   - See "Concurrent Merge Handling" below

5. Cleanup (unless --keep)
   - Use shared cleanupWorktree function (from d5ftwxr)
   - Runs pre-delete hooks, removes worktree, deletes branch, prunes

6. Print success
```

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

for attempt := 0; attempt < maxRetries; attempt++ {
    // Rebase onto target
    err := git.Rebase(ctx, wtPath, target)
    if err != nil {
        if isConflict(err) {
            // Conflict - abort and fail (can't auto-resolve)
            return handleConflict(err)
        }
        return err
    }
    
    // Try merge
    err = git.Merge(ctx, targetWt, feature, true) // ff-only
    if err == nil {
        break // Success!
    }
    
    if !isNotFFError(err) {
        return err // Some other error, don't retry
    }
    
    // Target moved, retry
    fprintf(stderr, "Target moved, retrying (%d/%d)...\n", attempt+1, maxRetries)
}

if attempt == maxRetries {
    return fmt.Errorf("merge failed after %d retries (high contention on target branch)", maxRetries)
}
```

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
