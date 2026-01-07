---
schema_version: 1
id: d5fattr
status: closed
closed: 2026-01-07T19:53:02Z
blocked-by: []
created: 2026-01-07T19:00:59Z
type: task
priority: 1
---
# Implement Git helper functions

## Overview
Implement the git CLI wrapper functions needed by all commands.

## Background & Rationale
All wt commands need to interact with git. Per TECH_SPEC.md, we shell out to the git CLI rather than using a Go git library. This provides:
- Simpler implementation
- Same behavior users expect from git
- No dependency on libgit2 or pure-Go implementations

## Current State
run.go has config loading but no git helpers exist yet.

## Implementation Details
Create these functions (add to run.go or a new git.go file):

### Core Git Queries
```go
// gitRepoRoot returns the repository root directory.
// Returns error if not in a git repository.
func gitRepoRoot(cwd string) (string, error) {
    cmd := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel")
    out, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("not a git repository")
    }
    return strings.TrimSpace(string(out)), nil
}

// gitCurrentBranch returns the current branch name.
func gitCurrentBranch(cwd string) (string, error) {
    cmd := exec.Command("git", "-C", cwd, "branch", "--show-current")
    out, err := cmd.Output()
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(string(out)), nil
}

// gitIsDirty returns true if the worktree has uncommitted changes.
func gitIsDirty(path string) (bool, error) {
    cmd := exec.Command("git", "-C", path, "status", "--porcelain")
    out, err := cmd.Output()
    if err != nil {
        return false, err
    }
    return len(out) > 0, nil
}
```

### Worktree Operations
```go
// gitWorktreeAdd creates a new worktree with a new branch.
func gitWorktreeAdd(repoRoot, wtPath, branch, baseBranch string) error {
    cmd := exec.Command("git", "-C", repoRoot, "worktree", "add", "-b", branch, wtPath, baseBranch)
    return cmd.Run()
}

// gitWorktreeRemove removes a worktree.
func gitWorktreeRemove(repoRoot, wtPath string, force bool) error {
    args := []string{"-C", repoRoot, "worktree", "remove", wtPath}
    if force {
        args = append(args, "--force")
    }
    cmd := exec.Command("git", args...)
    return cmd.Run()
}

// gitWorktreePrune prunes stale worktree metadata.
func gitWorktreePrune(repoRoot string) error {
    cmd := exec.Command("git", "-C", repoRoot, "worktree", "prune")
    return cmd.Run()
}

// gitBranchDelete deletes a branch.
func gitBranchDelete(repoRoot, branch string, force bool) error {
    flag := "-d"
    if force {
        flag = "-D"
    }
    cmd := exec.Command("git", "-C", repoRoot, "branch", flag, branch)
    return cmd.Run()
}

// gitWorktreeList returns paths of all worktrees for the repo.
func gitWorktreeList(repoRoot string) ([]string, error) {
    cmd := exec.Command("git", "-C", repoRoot, "worktree", "list", "--porcelain")
    out, err := cmd.Output()
    if err != nil {
        return nil, err
    }
    // Parse "worktree <path>" lines
    var paths []string
    for _, line := range strings.Split(string(out), "\n") {
        if strings.HasPrefix(line, "worktree ") {
            paths = append(paths, strings.TrimPrefix(line, "worktree "))
        }
    }
    return paths, nil
}
```

## Considerations
- All functions take cwd/repoRoot explicitly (no global state)
- Error messages should be user-friendly
- Consider capturing stderr from git for better error reporting

## Acceptance Criteria
- All git helper functions implemented
- Functions work with -C flag (use cwd parameter)
- Proper error handling for non-git directories

## Testing
- E2E tests will exercise these via commands
- Could add focused tests with real git repos in tmp dirs
