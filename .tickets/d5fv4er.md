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

4. Rebase onto target
   - Run: git rebase <target>
   - On conflict:
     - Get files: git diff --name-only --diff-filter=U
     - Abort: git rebase --abort
     - Print conflicting files and diff commands
     - Exit with error

5. Merge
   - If target checked out elsewhere: git -C <target-wt> merge <feature> --ff-only
   - If target not checked out: git checkout <target> && git merge <feature> --ff-only

6. Cleanup (unless --keep)
   - Use shared cleanupWorktree function (from d5ftwxr)
   - Runs pre-delete hooks, removes worktree, deletes branch, prunes

7. Print success
```

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
- E2E tests cover all scenarios listed above
