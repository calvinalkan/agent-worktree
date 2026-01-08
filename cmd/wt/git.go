package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Static errors for git operations.
var (
	ErrNotGitRepository  = errors.New("not a git repository (use -C to specify repo path)")
	ErrGitWorktreeAdd    = errors.New("creating worktree")
	ErrGitWorktreeRemove = errors.New("removing worktree")
	ErrGitWorktreePrune  = errors.New("pruning worktree metadata")
	ErrGitWorktreeList   = errors.New("listing worktrees")
	ErrGitBranchDelete   = errors.New("deleting branch")
	ErrGitCurrentBranch  = errors.New("getting current branch")
	ErrGitStatusCheck    = errors.New("checking git status")
	ErrGitRebase         = errors.New("rebase failed")
	ErrGitRebaseAbort    = errors.New("aborting rebase")
	ErrGitMerge          = errors.New("merge failed")
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
func (g *Git) RepoRoot(ctx context.Context, cwd string) (string, error) {
	cmd := g.newCmdContext(ctx, "-C", cwd, "rev-parse", "--show-toplevel")

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
func (g *Git) GitCommonDir(ctx context.Context, cwd string) (string, error) {
	cmd := g.newCmdContext(ctx, "-C", cwd, "rev-parse", "--path-format=absolute", "--git-common-dir")

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
func (g *Git) MainRepoRoot(ctx context.Context, cwd string) (string, error) {
	gitDir, err := g.GitCommonDir(ctx, cwd)
	if err != nil {
		return "", err
	}

	// gitDir is /path/to/repo/.git, so parent is the repo root
	return filepath.Dir(gitDir), nil
}

// CurrentBranch returns the current branch name.
func (g *Git) CurrentBranch(ctx context.Context, cwd string) (string, error) {
	cmd := g.newCmdContext(ctx, "-C", cwd, "branch", "--show-current")

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrGitCurrentBranch, err)
	}

	return strings.TrimSpace(string(out)), nil
}

// IsDirty returns true if the worktree has any uncommitted changes,
// including modified tracked files and untracked files.
// Use this for checking before deleting a worktree (user might lose work).
func (g *Git) IsDirty(ctx context.Context, path string) (bool, error) {
	cmd := g.newCmdContext(ctx, "-C", path, "status", "--porcelain")

	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrGitStatusCheck, err)
	}

	return len(out) > 0, nil
}

// HasUncommittedTrackedChanges returns true if the worktree has uncommitted
// changes to tracked files (staged or unstaged modifications).
// Untracked files are ignored since they don't affect branch operations like merges.
// Use this for checking if a worktree is safe to merge into.
func (g *Git) HasUncommittedTrackedChanges(ctx context.Context, path string) (bool, error) {
	// Use -uno to exclude untracked files from the status
	cmd := g.newCmdContext(ctx, "-C", path, "status", "--porcelain", "-uno")

	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrGitStatusCheck, err)
	}

	return len(out) > 0, nil
}

// WorktreeAdd creates a new worktree with a new branch.
func (g *Git) WorktreeAdd(ctx context.Context, repoRoot, wtPath, branch, baseBranch string) error {
	cmd := g.newCmdContext(ctx, "-C", repoRoot, "worktree", "add", "-b", branch, wtPath, baseBranch)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGitWorktreeAdd, strings.TrimSpace(string(out)))
	}

	return nil
}

// WorktreeRemove removes a worktree.
func (g *Git) WorktreeRemove(ctx context.Context, repoRoot, wtPath string, force bool) error {
	args := []string{"-C", repoRoot, "worktree", "remove", wtPath}
	if force {
		args = append(args, "--force")
	}

	cmd := g.newCmdContext(ctx, args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGitWorktreeRemove, strings.TrimSpace(string(out)))
	}

	return nil
}

// WorktreePrune prunes stale worktree metadata.
func (g *Git) WorktreePrune(ctx context.Context, repoRoot string) error {
	cmd := g.newCmdContext(ctx, "-C", repoRoot, "worktree", "prune")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGitWorktreePrune, strings.TrimSpace(string(out)))
	}

	return nil
}

// BranchDelete deletes a branch.
func (g *Git) BranchDelete(ctx context.Context, repoRoot, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}

	cmd := g.newCmdContext(ctx, "-C", repoRoot, "branch", flag, branch)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGitBranchDelete, strings.TrimSpace(string(out)))
	}

	return nil
}

// WorktreeList returns paths of all worktrees for the repo.
func (g *Git) WorktreeList(ctx context.Context, repoRoot string) ([]string, error) {
	cmd := g.newCmdContext(ctx, "-C", repoRoot, "worktree", "list", "--porcelain")

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
func (g *Git) ChangedFiles(ctx context.Context, cwd string) ([]string, error) {
	files := make(map[string]struct{})

	// Get staged and unstaged changes compared to HEAD
	cmd := g.newCmdContext(ctx, "-C", cwd, "diff", "--name-only", "HEAD")

	out, err := cmd.Output()
	if err != nil {
		// HEAD might not exist (initial commit), try without HEAD
		cmd = g.newCmdContext(ctx, "-C", cwd, "diff", "--name-only")

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
	cmd = g.newCmdContext(ctx, "-C", cwd, "diff", "--cached", "--name-only")

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
	cmd = g.newCmdContext(ctx, "-C", cwd, "ls-files", "--others", "--exclude-standard")

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

// BranchExists checks if a branch exists.
func (g *Git) BranchExists(ctx context.Context, dir, branch string) (bool, error) {
	cmd := g.newCmdContext(ctx, "-C", dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)

	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Exit code 1 means branch doesn't exist
			return false, nil
		}

		return false, fmt.Errorf("checking branch existence: %w", err)
	}

	return true, nil
}

// FindWorktreeForBranch returns the worktree path that has the given branch checked out.
// Returns empty string if the branch is not checked out in any worktree.
func (g *Git) FindWorktreeForBranch(ctx context.Context, dir, branch string) (string, error) {
	cmd := g.newCmdContext(ctx, "-C", dir, "worktree", "list", "--porcelain")

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("listing worktrees: %w", err)
	}

	// Parse porcelain output: blocks separated by blank lines
	// Each block has:
	//   worktree <path>
	//   HEAD <sha>
	//   branch refs/heads/<branch>
	var currentPath string

	for line := range strings.SplitSeq(string(out), "\n") {
		if after, ok := strings.CutPrefix(line, "worktree "); ok {
			currentPath = after
		} else if after, ok := strings.CutPrefix(line, "branch refs/heads/"); ok {
			if after == branch {
				return currentPath, nil
			}
		}
	}

	return "", nil
}

// Rebase rebases the current branch onto the target branch.
func (g *Git) Rebase(ctx context.Context, dir, target string) error {
	cmd := g.newCmdContext(ctx, "-C", dir, "rebase", target)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGitRebase, strings.TrimSpace(string(out)))
	}

	return nil
}

// RebaseAbort aborts an in-progress rebase.
func (g *Git) RebaseAbort(ctx context.Context, dir string) error {
	cmd := g.newCmdContext(ctx, "-C", dir, "rebase", "--abort")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGitRebaseAbort, strings.TrimSpace(string(out)))
	}

	return nil
}

// ConflictingFiles returns the list of files with merge conflicts.
func (g *Git) ConflictingFiles(ctx context.Context, dir string) ([]string, error) {
	cmd := g.newCmdContext(ctx, "-C", dir, "diff", "--name-only", "--diff-filter=U")

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing conflicting files: %w", err)
	}

	var files []string

	for line := range strings.SplitSeq(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// Merge merges a branch into the current branch.
// If ffOnly is true, only fast-forward merges are allowed.
func (g *Git) Merge(ctx context.Context, dir, branch string, ffOnly bool) error {
	args := []string{"-C", dir, "merge", branch}
	if ffOnly {
		args = append(args, "--ff-only")
	}

	cmd := g.newCmdContext(ctx, args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGitMerge, strings.TrimSpace(string(out)))
	}

	return nil
}

// PushLocal updates a local branch to match another branch using "git push . src:dst".
// This is a safe, atomic way to fast-forward a branch that isn't checked out.
// Fails if not fast-forward (target moved), which triggers retry logic.
// This is purely local - no network calls, no remote interaction.
func (g *Git) PushLocal(ctx context.Context, dir, sourceBranch, targetBranch string) error {
	refspec := sourceBranch + ":" + targetBranch
	cmd := g.newCmdContext(ctx, "-C", dir, "push", ".", refspec)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGitMerge, strings.TrimSpace(string(out)))
	}

	return nil
}

// CommitsBetween returns the number of commits on branch that are not on target.
func (g *Git) CommitsBetween(ctx context.Context, dir, target, branch string) (int, error) {
	cmd := g.newCmdContext(ctx, "-C", dir, "rev-list", "--count", target+".."+branch)

	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("counting commits: %w", err)
	}

	var count int

	_, err = fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count)
	if err != nil {
		return 0, fmt.Errorf("parsing commit count: %w", err)
	}

	return count, nil
}

// newCmdContext creates an exec.Cmd for git with the configured environment and context.
func (g *Git) newCmdContext(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = g.env

	return cmd
}
