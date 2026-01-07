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

// Errors for delete command.
var (
	errWorktreeNameRequired     = errors.New("worktree name is required")
	errWorktreeNotFound         = errors.New("worktree not found")
	errWorktreeHasChanges       = errors.New("worktree has uncommitted changes (use --force to override)")
	errRemovingWorktreeFailed   = errors.New("failed to remove worktree")
	errCheckingWorktreeStatus   = errors.New("failed to check worktree status")
	errReadingWorktreeInfo      = errors.New("failed to read worktree info")
	errPreDeleteHookAbortDelete = errors.New("deletion aborted by pre-delete hook")
)

// DeleteCmd returns the delete command.
func DeleteCmd(cfg Config, fsys fs.FS, git *Git, env map[string]string) *Command {
	flags := flag.NewFlagSet("delete", flag.ContinueOnError)
	flags.BoolP("help", "h", false, "Show help")
	flags.Bool("force", false, "Delete even if worktree has uncommitted changes")
	flags.Bool("with-branch", false, "Also delete the git branch")

	return &Command{
		Flags: flags,
		Usage: "delete <name> [flags]",
		Short: "Delete a worktree",
		Long: `Delete a worktree by name.

If the worktree has uncommitted changes, use --force to proceed.
Use --with-branch to also delete the git branch.`,
		Exec: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, args []string) error {
			return execDelete(ctx, stdin, stdout, stderr, cfg, fsys, git, env, flags, args)
		},
	}
}

func execDelete(
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

	// 1. Verify git repository
	repoRoot, err := git.RepoRoot(cfg.EffectiveCwd)
	if err != nil {
		return ErrNotGitRepository
	}

	// 2. Find worktree by name
	baseDir := resolveWorktreeBaseDir(cfg, repoRoot)
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
		dirty, err := git.IsDirty(wtPath)
		if err != nil {
			return fmt.Errorf("%w: %w", errCheckingWorktreeStatus, err)
		}

		if dirty {
			return errWorktreeHasChanges
		}
	}

	// 4. Run pre-delete hook
	hookRunner := NewHookRunner(fsys, repoRoot, env, stdout, stderr)

	err = hookRunner.RunPreDelete(ctx, &info, wtPath, cfg.EffectiveCwd)
	if err != nil {
		return fmt.Errorf("%w: %w", errPreDeleteHookAbortDelete, err)
	}

	// 5. Remove worktree
	err = git.WorktreeRemove(repoRoot, wtPath, force)
	if err != nil {
		return fmt.Errorf("%w: %w", errRemovingWorktreeFailed, err)
	}

	// 6. Determine branch deletion
	deleteBranch := withBranch

	if !withBranch && stdin != nil && IsTerminal() {
		// Interactive prompt
		deleteBranch = promptYesNo(stdin, stdout, fmt.Sprintf("Delete branch '%s'? (y/N) ", name))
	}
	// Non-interactive without --with-branch: keep branch (deleteBranch stays false)

	// 7. Delete branch if requested
	if deleteBranch {
		err = git.BranchDelete(repoRoot, name, force)
		if err != nil {
			// Log but don't fail - worktree already deleted
			fprintf(stderr, "warning: could not delete branch %s: %v\n", name, err)
		}
	}

	// 8. Prune worktree metadata
	err = git.WorktreePrune(repoRoot)
	if err != nil {
		fprintf(stderr, "warning: git worktree prune failed: %v\n", err)
	}

	// 9. Output success
	if deleteBranch {
		fprintln(stdout, "Deleted worktree and branch:", name)
	} else {
		fprintln(stdout, "Deleted worktree:", name)
	}

	return nil
}

// promptYesNo prompts the user for yes/no confirmation.
// Returns true for 'y' or 'Y', false otherwise.
func promptYesNo(stdin io.Reader, stdout io.Writer, prompt string) bool {
	fprintf(stdout, "%s", prompt)

	reader := bufio.NewReader(stdin)

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(response), "y")
}
