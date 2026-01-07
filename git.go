package main

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
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

// Git provides git operations with explicit environment control.
// This allows isolation in tests by passing a controlled environment.
type Git struct {
	env []string
}

// NewGit creates a Git instance with the given environment.
// In production, pass the result of os.Environ().
// In tests, pass nil or empty slice for isolation.
func NewGit(env []string) *Git {
	return &Git{env: env}
}

// RepoRoot returns the repository root directory.
// Returns error if not in a git repository.
func (g *Git) RepoRoot(cwd string) (string, error) {
	cmd := g.newCmd("-C", cwd, "rev-parse", "--show-toplevel")

	out, err := cmd.Output()
	if err != nil {
		return "", ErrNotGitRepository
	}

	return strings.TrimSpace(string(out)), nil
}

// GitCommonDir returns the absolute path to the shared .git directory.
// For a regular repo, this is .git/. For a worktree, this returns
// the main repository's .git directory, ensuring all worktrees
// share the same lock files.
func (g *Git) GitCommonDir(cwd string) (string, error) {
	cmd := g.newCmd("-C", cwd, "rev-parse", "--path-format=absolute", "--git-common-dir")

	out, err := cmd.Output()
	if err != nil {
		return "", ErrNotGitRepository
	}

	return strings.TrimSpace(string(out)), nil
}

// MainRepoRoot returns the root directory of the main repository.
// For a regular repo, this is the same as RepoRoot. For a worktree,
// this returns the main repository's root (not the worktree's root).
// This ensures all worktrees resolve to the same base directory.
func (g *Git) MainRepoRoot(cwd string) (string, error) {
	gitDir, err := g.GitCommonDir(cwd)
	if err != nil {
		return "", err
	}

	// gitDir is /path/to/repo/.git, so parent is the repo root
	return filepath.Dir(gitDir), nil
}

// CurrentBranch returns the current branch name.
func (g *Git) CurrentBranch(cwd string) (string, error) {
	cmd := g.newCmd("-C", cwd, "branch", "--show-current")

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrGitCurrentBranch, err)
	}

	return strings.TrimSpace(string(out)), nil
}

// IsDirty returns true if the worktree has uncommitted changes.
func (g *Git) IsDirty(path string) (bool, error) {
	cmd := g.newCmd("-C", path, "status", "--porcelain")

	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrGitStatusCheck, err)
	}

	return len(out) > 0, nil
}

// WorktreeAdd creates a new worktree with a new branch.
func (g *Git) WorktreeAdd(repoRoot, wtPath, branch, baseBranch string) error {
	cmd := g.newCmd("-C", repoRoot, "worktree", "add", "-b", branch, wtPath, baseBranch)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGitWorktreeAdd, strings.TrimSpace(string(out)))
	}

	return nil
}

// WorktreeRemove removes a worktree.
func (g *Git) WorktreeRemove(repoRoot, wtPath string, force bool) error {
	args := []string{"-C", repoRoot, "worktree", "remove", wtPath}
	if force {
		args = append(args, "--force")
	}

	cmd := g.newCmd(args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGitWorktreeRemove, strings.TrimSpace(string(out)))
	}

	return nil
}

// WorktreePrune prunes stale worktree metadata.
func (g *Git) WorktreePrune(repoRoot string) error {
	cmd := g.newCmd("-C", repoRoot, "worktree", "prune")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGitWorktreePrune, strings.TrimSpace(string(out)))
	}

	return nil
}

// BranchDelete deletes a branch.
func (g *Git) BranchDelete(repoRoot, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}

	cmd := g.newCmd("-C", repoRoot, "branch", flag, branch)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGitBranchDelete, strings.TrimSpace(string(out)))
	}

	return nil
}

// WorktreeList returns paths of all worktrees for the repo.
func (g *Git) WorktreeList(repoRoot string) ([]string, error) {
	cmd := g.newCmd("-C", repoRoot, "worktree", "list", "--porcelain")

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

// ChangedFiles returns all uncommitted files: staged, unstaged, and untracked.
// Untracked files respect .gitignore.
// Returns relative paths from the repository root.
func (g *Git) ChangedFiles(cwd string) ([]string, error) {
	files := make(map[string]struct{})

	// Get staged and unstaged changes compared to HEAD
	cmd := g.newCmd("-C", cwd, "diff", "--name-only", "HEAD")

	out, err := cmd.Output()
	if err != nil {
		// HEAD might not exist (initial commit), try without HEAD
		cmd = g.newCmd("-C", cwd, "diff", "--name-only")

		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("getting diff: %w", err)
		}
	}

	for line := range strings.SplitSeq(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files[line] = struct{}{}
		}
	}

	// Get staged files (in case some are only staged, not yet in HEAD)
	cmd = g.newCmd("-C", cwd, "diff", "--cached", "--name-only")

	out, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("getting staged diff: %w", err)
	}

	for line := range strings.SplitSeq(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files[line] = struct{}{}
		}
	}

	// Get untracked files (respecting .gitignore)
	cmd = g.newCmd("-C", cwd, "ls-files", "--others", "--exclude-standard")

	out, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing untracked files: %w", err)
	}

	for line := range strings.SplitSeq(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files[line] = struct{}{}
		}
	}

	result := make([]string, 0, len(files))
	for f := range files {
		result = append(result, f)
	}

	return result, nil
}

// newCmd creates an exec.Cmd for git with the configured environment.
func (g *Git) newCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Env = g.env

	return cmd
}
