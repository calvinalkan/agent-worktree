---
schema_version: 1
id: d5fek2r
status: closed
closed: 2026-01-07T23:22:55Z
blocked-by: []
created: 2026-01-07T23:17:31Z
type: chore
priority: 2
---
# Pass context to Git layer for cancellation support

Currently Git commands don't receive context, so they can't be cancelled and have no timeouts.

Changes needed:

1. Update newCmd to newCmdContext:
   func (g *Git) newCmdContext(ctx context.Context, args ...string) *exec.Cmd {
       cmd := exec.CommandContext(ctx, "git", args...)
       cmd.Env = g.env
       return cmd
   }

2. Add context.Context as first parameter to all Git methods:
   - RepoRoot(ctx, cwd)
   - GitCommonDir(ctx, cwd)
   - MainRepoRoot(ctx, cwd)
   - CurrentBranch(ctx, cwd)
   - IsDirty(ctx, path)
   - WorktreeAdd(ctx, repoRoot, wtPath, branch, baseBranch)
   - WorktreeRemove(ctx, repoRoot, wtPath, force)
   - WorktreePrune(ctx, repoRoot)
   - BranchDelete(ctx, repoRoot, branch, force)
   - WorktreeList(ctx, repoRoot)
   - ChangedFiles(ctx, cwd)

3. Update all callers in cmd_create.go, cmd_delete.go, cmd_list.go, cmd_info.go to pass ctx

4. Update cmd_run.go LoadConfig call - may need context there too for git.RepoRoot

This enables Ctrl+C to actually cancel running git commands.
