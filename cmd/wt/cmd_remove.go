package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/calvinalkan/agent-task/pkg/fs"
	flag "github.com/spf13/pflag"
)

// Errors for remove command.
var (
	errWorktreeNameRequired     = errors.New("worktree name is required (usage: wt remove <name>)")
	errWorktreeNotFound         = errors.New("worktree not found")
	errWorktreeHasChanges       = errors.New("worktree has uncommitted changes (use --force to override)")
	errRemovingWorktreeFailed   = errors.New("removing worktree")
	errCheckingWorktreeStatus   = errors.New("checking worktree status")
	errReadingWorktreeInfo      = errors.New("reading worktree info")
	errPreDeleteHookAbortDelete = errors.New("pre-delete hook aborted deletion (hook exited non-zero)")
)

// RemoveCmd returns the remove command.
func RemoveCmd(cfg Config, fsys fs.FS, git *Git, env map[string]string) *Command {
	flags := flag.NewFlagSet("remove", flag.ContinueOnError)
	flags.BoolP("help", "h", false, "Show help")
	flags.BoolP("force", "f", false, "Remove even if worktree has uncommitted changes")
	flags.BoolP("with-branch", "b", false, "Also delete the git branch (skips interactive prompt)")

	return &Command{
		Flags:   flags,
		Usage:   "remove <name> [flags]",
		Short:   "Remove a worktree",
		Aliases: []string{"rm"},
		Long: `Remove a worktree by name.

Removes the worktree directory and git worktree metadata. If the worktree
has uncommitted changes, use --force to proceed.

In an interactive terminal, you will be prompted about branch deletion.
In non-interactive mode (scripts/pipes), the branch is kept unless
--with-branch is specified.

If .wt/hooks/pre-delete exists and is executable, it runs before deletion
and can abort the operation by exiting non-zero.`,
		Exec: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, args []string) error {
			return execRemove(ctx, stdin, stdout, stderr, cfg, fsys, git, env, flags, args)
		},
	}
}

func execRemove(
	ctx context.Context,
	stdin io.Reader,
	stdout, stderr io.Writer,
	cfg Config,
	fsys fs.FS,
	git *Git,
	env map[string]string,
	flags *flag.FlagSet,
	args []string,
) error {
	if len(args) == 0 {
		return errWorktreeNameRequired
	}

	name := args[0]
	force, _ := flags.GetBool("force")
	withBranch, _ := flags.GetBool("with-branch")

	// 1. Get main repo root (works from inside worktrees too)
	mainRepoRoot, err := git.MainRepoRoot(ctx, cfg.EffectiveCwd)
	if err != nil {
		return ErrNotGitRepository
	}

	// 2. Find worktree by name
	baseDir := resolveWorktreeBaseDir(cfg, mainRepoRoot)
	wtPath := filepath.Join(baseDir, name)

	info, err := readWorktreeInfo(fsys, wtPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", errWorktreeNotFound, name)
		}

		return fmt.Errorf("%w: %w", errReadingWorktreeInfo, err)
	}

	// 3. Check for uncommitted changes
	if !force {
		dirty, err := git.IsDirty(ctx, wtPath)
		if err != nil {
			return fmt.Errorf("%w: %w", errCheckingWorktreeStatus, err)
		}

		if dirty {
			return errWorktreeHasChanges
		}
	}

	// 4. Run pre-delete hook
	hookRunner := NewHookRunner(fsys, mainRepoRoot, env, stdout, stderr)

	err = hookRunner.RunPreDelete(ctx, &info, wtPath, cfg.EffectiveCwd)
	if err != nil {
		return fmt.Errorf("%w: %w", errPreDeleteHookAbortDelete, err)
	}

	// 5. Remove worktree
	err = git.WorktreeRemove(ctx, mainRepoRoot, wtPath, force)
	if err != nil {
		return fmt.Errorf("%w: %w", errRemovingWorktreeFailed, err)
	}

	// 6. Show confirmation that worktree was removed
	fprintln(stdout, "Removed worktree:", wtPath)

	// 7. Determine branch deletion
	deleteBranch := withBranch

	if !withBranch && stdin != nil && IsTerminal() {
		// Interactive prompt - explain that branch is safe and ask about deletion
		fprintln(stdout)
		fprintf(stdout, "Branch '%s' still contains all your commits.\n", name)
		fprintf(stdout, "Also delete the branch? (y/N) ")

		deleteBranch = readYesNo(stdin)
	}
	// Non-interactive without --with-branch: keep branch (deleteBranch stays false)

	// 8. Delete branch if requested
	var branchErr error

	branchDeleted := false

	if deleteBranch {
		branchErr = git.BranchDelete(ctx, mainRepoRoot, name, force)
		if branchErr == nil {
			branchDeleted = true
		}
	}

	// 9. Prune worktree metadata (always run, independent of branch deletion)
	pruneErr := git.WorktreePrune(ctx, mainRepoRoot)

	// 10. Output what actually happened with branch
	if branchDeleted {
		fprintln(stdout, "Deleted branch:", name)
	}

	// 11. Return combined errors if any
	return errors.Join(branchErr, pruneErr)
}

// readYesNo reads a yes/no response from stdin.
// Returns true for 'y' or 'Y', false otherwise.
func readYesNo(stdin io.Reader) bool {
	reader := bufio.NewReader(stdin)

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(response), "y")
}
