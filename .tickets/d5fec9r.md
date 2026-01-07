---
schema_version: 1
id: d5fec9r
status: closed
closed: 2026-01-07T23:06:40Z
blocked-by: []
created: 2026-01-07T23:03:03Z
type: chore
priority: 2
---
# Improve error messages across all commands

Standardize error messages to follow pattern: '<what we tried>: <what went wrong> (actionable hint)'

Also remove 'failed' from error messages since 'error:' prefix already implies failure.

## git.go

- ErrGitWorktreeAdd: 'git worktree add failed' → 'creating worktree'
- ErrGitWorktreeRemove: 'git worktree remove failed' → 'removing worktree'
- ErrGitWorktreePrune: 'git worktree prune failed' → 'pruning worktree metadata'
- ErrGitWorktreeList: 'git worktree list failed' → 'listing worktrees'
- ErrGitBranchDelete: 'git branch delete failed' → 'deleting branch'
- ErrGitCurrentBranch: 'failed to get current branch' → 'getting current branch'
- ErrGitStatusCheck: 'failed to check git status' → 'checking git status'
- ErrNotGitRepository: add hint '(use -C to specify repo path)'

## delete.go

- errWorktreeNameRequired: add '(usage: wt delete <name>)'
- errWorktreeNotFound: include the path we searched in
- errRemovingWorktreeFailed: 'failed to remove worktree' → 'removing worktree'
- errCheckingWorktreeStatus: 'failed to check worktree status' → 'checking worktree status'
- errReadingWorktreeInfo: 'failed to read worktree info' → 'reading worktree info'
- errPreDeleteHookAbortDelete: → 'pre-delete hook aborted deletion (hook exited non-zero)'

## create.go

- ErrNameAlreadyInUse: add '(use wt list to see worktrees)'
- 'cannot determine current branch': → 'getting current branch (use --from-branch if in detached HEAD)'
- 'acquiring create lock': add '(another wt process may be running)'
- 'post-create hook failed': add '(check hook output above)'

## info.go

- errNotInWorktree: add '(use wt list to find worktrees)'
- errInvalidField: add '(valid: name, agent_id, id, path, base_branch, created)'

## hooks.go

- ErrHookNotExecutable: → 'hook not executable: <path> (fix with: chmod +x <path>)'
- ErrHookTimeout: add '(hook may be stuck or waiting for input)'

## names.go

- ErrNameGenerationFailed: → 'generating unique name (too many worktrees? use --name to specify)'
