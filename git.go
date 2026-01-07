package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Static errors for git operations.
var (
	ErrNotGitRepository  = errors.New("not a git repository")
	ErrGitWorktreeAdd    = errors.New("git worktree add failed")
	ErrGitWorktreeRemove = errors.New("git worktree remove failed")
	ErrGitWorktreePrune  = errors.New("git worktree prune failed")
	ErrGitWorktreeList   = errors.New("git worktree list failed")
	ErrGitBranchDelete   = errors.New("git branch delete failed")
	ErrGitCurrentBranch  = errors.New("failed to get current branch")
	ErrGitStatusCheck    = errors.New("failed to check git status")
)

// gitRepoRoot returns the repository root directory.
// Returns error if not in a git repository.
func gitRepoRoot(cwd string) (string, error) {
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel")

	out, err := cmd.Output()
	if err != nil {
		return "", ErrNotGitRepository
	}

	return strings.TrimSpace(string(out)), nil
}

// gitCurrentBranch returns the current branch name.
func gitCurrentBranch(cwd string) (string, error) {
	cmd := exec.Command("git", "-C", cwd, "branch", "--show-current")

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrGitCurrentBranch, err)
	}

	return strings.TrimSpace(string(out)), nil
}

// gitIsDirty returns true if the worktree has uncommitted changes.
func gitIsDirty(path string) (bool, error) {
	cmd := exec.Command("git", "-C", path, "status", "--porcelain")

	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrGitStatusCheck, err)
	}

	return len(out) > 0, nil
}

// gitWorktreeAdd creates a new worktree with a new branch.
func gitWorktreeAdd(repoRoot, wtPath, branch, baseBranch string) error {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "add", "-b", branch, wtPath, baseBranch)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGitWorktreeAdd, strings.TrimSpace(string(out)))
	}

	return nil
}

// gitWorktreeRemove removes a worktree.
func gitWorktreeRemove(repoRoot, wtPath string, force bool) error {
	var cmd *exec.Cmd

	if force {
		cmd = exec.Command("git", "-C", repoRoot, "worktree", "remove", wtPath, "--force")
	} else {
		cmd = exec.Command("git", "-C", repoRoot, "worktree", "remove", wtPath)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGitWorktreeRemove, strings.TrimSpace(string(out)))
	}

	return nil
}

// gitWorktreePrune prunes stale worktree metadata.
func gitWorktreePrune(repoRoot string) error {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "prune")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGitWorktreePrune, strings.TrimSpace(string(out)))
	}

	return nil
}

// gitBranchDelete deletes a branch.
func gitBranchDelete(repoRoot, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}

	cmd := exec.Command("git", "-C", repoRoot, "branch", flag, branch)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGitBranchDelete, strings.TrimSpace(string(out)))
	}

	return nil
}

// gitWorktreeList returns paths of all worktrees for the repo.
func gitWorktreeList(repoRoot string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "list", "--porcelain")

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGitWorktreeList, err)
	}

	// Parse "worktree <path>" lines
	var paths []string

	for line := range strings.SplitSeq(string(out), "\n") {
		if after, ok := strings.CutPrefix(line, "worktree "); ok {
			paths = append(paths, after)
		}
	}

	return paths, nil
}
