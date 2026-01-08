package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/calvinalkan/agent-task/pkg/fs"
	flag "github.com/spf13/pflag"
)

// Errors for merge command.
var (
	errReadingMergeMetadata   = errors.New("reading worktree metadata")
	errValidatingBranches     = errors.New("validating branches")
	errCheckingMergeWorktree  = errors.New("checking worktree status")
	errCheckingTargetWorktree = errors.New("checking target worktree")
	errRebasingOnto           = errors.New("rebasing onto")
	errMergingInto            = errors.New("merging into")
	errMergeConflict          = errors.New("conflict during rebase")
	errTargetBranchNotExist   = errors.New("branch does not exist")
	errAlreadyOnTarget        = errors.New("already on target branch, nothing to merge")
	errUncommittedChanges     = errors.New("uncommitted changes")
	errTargetHasChanges       = errors.New("has uncommitted changes")
	errNotInMergeWorktree     = errors.New("are you in a wt-managed worktree?")
	errMergeCancelled         = errors.New("merge cancelled")
	errAcquiringMergeLock     = errors.New("acquiring merge lock")
	errMergeLockTimedOut      = errors.New("timed out waiting for merge lock - another merge may be stuck")
)

const (
	maxMergeRetries  = 3
	mergeBaseDelay   = 100 * time.Millisecond
	mergeMaxDelay    = 2 * time.Second
	mergeLockTimeout = 30 * time.Second
)

// mergeLockPath returns the path to the lock file for merge operations.
// Placed in git common directory so all worktrees share the same lock.
func mergeLockPath(gitCommonDir string) string {
	return gitCommonDir + "/wt-merge.lock"
}

// MergeCmd returns the merge command.
func MergeCmd(cfg Config, fsys fs.FS, git *Git, env map[string]string) *Command {
	flags := flag.NewFlagSet("merge", flag.ContinueOnError)
	flags.BoolP("help", "h", false, "Show help")
	flags.String("into", "", "Merge into this branch instead of base_branch")
	flags.Bool("keep", false, "Keep worktree after merge (skip cleanup)")
	flags.Bool("dry-run", false, "Show what would happen without executing")

	return &Command{
		Flags: flags,
		Usage: "merge [flags]",
		Short: "Merge worktree branch into base branch",
		Long: `Merge the current worktree's branch into its base branch (or --into target).

Performs a rebase onto the target branch followed by a fast-forward merge.
After successful merge, the worktree and branch are removed unless --keep is used.

If multiple merges to the same target happen concurrently, the command
automatically retries with exponential backoff.`,
		Exec: func(ctx context.Context, _ io.Reader, stdout, stderr io.Writer, _ []string) error {
			return execMerge(ctx, stdout, stderr, cfg, fsys, git, env, flags)
		},
	}
}

func execMerge(
	ctx context.Context,
	stdout, stderr io.Writer,
	cfg Config,
	fsys fs.FS,
	git *Git,
	env map[string]string,
	flags *flag.FlagSet,
) error {
	into, _ := flags.GetString("into")
	keep, _ := flags.GetBool("keep")
	dryRun, _ := flags.GetBool("dry-run")

	// PHASE 1: ALL CHECKS (fail fast, no side effects)

	// 1. Read metadata
	info, err := readWorktreeInfo(fsys, cfg.EffectiveCwd)
	if err != nil {
		return fmt.Errorf("%w: %w (%w)", errReadingMergeMetadata, err, errNotInMergeWorktree)
	}

	// Get current branch (feature)
	featureBranch, err := git.CurrentBranch(ctx, cfg.EffectiveCwd)
	if err != nil {
		return fmt.Errorf("%w: getting current branch: %w", errReadingMergeMetadata, err)
	}

	// Determine target branch
	targetBranch := info.BaseBranch
	if into != "" {
		targetBranch = into
	}

	// 2. Validate branches
	exists, err := git.BranchExists(ctx, cfg.EffectiveCwd, targetBranch)
	if err != nil {
		return fmt.Errorf("%w: %w", errValidatingBranches, err)
	}

	if !exists {
		return fmt.Errorf("%w: '%s' %w (check branch name or use --into)", errValidatingBranches, targetBranch, errTargetBranchNotExist)
	}

	if featureBranch == targetBranch {
		return fmt.Errorf("%w: %w '%s'", errValidatingBranches, errAlreadyOnTarget, targetBranch)
	}

	// 3. Check current worktree clean
	dirty, err := git.IsDirty(ctx, cfg.EffectiveCwd)
	if err != nil {
		return fmt.Errorf("%w: %w", errCheckingMergeWorktree, err)
	}

	if dirty {
		return fmt.Errorf("%w: %w (commit or stash before merging)", errCheckingMergeWorktree, errUncommittedChanges)
	}

	// Get main repo root
	mainRepoRoot, err := git.MainRepoRoot(ctx, cfg.EffectiveCwd)
	if err != nil {
		return fmt.Errorf("%w: %w", errReadingMergeMetadata, err)
	}

	// Get git common directory for lock file
	gitCommonDir, err := git.GitCommonDir(ctx, cfg.EffectiveCwd)
	if err != nil {
		return fmt.Errorf("%w: %w", errReadingMergeMetadata, err)
	}

	// 4. Check target worktree clean (if checked out somewhere)
	targetWtPath, err := git.FindWorktreeForBranch(ctx, cfg.EffectiveCwd, targetBranch)
	if err != nil {
		return fmt.Errorf("%w: %w", errCheckingTargetWorktree, err)
	}

	if targetWtPath != "" {
		// Only check for uncommitted tracked changes, not untracked files
		// Untracked files (like newly created worktree directories) don't affect merges
		targetDirty, dirtyErr := git.HasUncommittedTrackedChanges(ctx, targetWtPath)
		if dirtyErr != nil {
			return fmt.Errorf("%w: %w", errCheckingTargetWorktree, dirtyErr)
		}

		if targetDirty {
			return fmt.Errorf("%w: '%s' %w (commit or stash there first)", errCheckingTargetWorktree, targetWtPath, errTargetHasChanges)
		}
	}

	// Get commit count for dry-run output
	commitCount, err := git.CommitsBetween(ctx, cfg.EffectiveCwd, targetBranch, featureBranch)
	if err != nil {
		// Non-fatal, use 0 for dry-run output
		commitCount = 0
	}

	// Handle dry-run
	if dryRun {
		return printDryRun(stdout, featureBranch, targetBranch, targetWtPath, mainRepoRoot, cfg.EffectiveCwd, info.Name, commitCount, keep)
	}

	// PHASE 2: EXECUTE (with retry loop)

	// 5. Rebase + Merge with lock
	locker := fs.NewLocker(fsys)
	lockPath := mergeLockPath(gitCommonDir)

	err = mergeWithLock(ctx, stderr, git, locker, lockPath, cfg.EffectiveCwd, targetWtPath, featureBranch, targetBranch)
	if err != nil {
		return err
	}

	fprintln(stdout, "Merged", featureBranch, "into", targetBranch)

	// 6. Cleanup (unless --keep)
	if keep {
		fprintln(stdout, "Worktree kept:", cfg.EffectiveCwd)

		return nil
	}

	hookRunner := NewHookRunner(fsys, mainRepoRoot, env, stdout, stderr)

	cleanupErr := CleanupWorktree(ctx, stdout, git, hookRunner, &info, cfg.EffectiveCwd, mainRepoRoot, true, true)
	if cleanupErr != nil {
		// Merge succeeded but cleanup failed - warn but don't fail
		fprintln(stderr, "warning: cleanup failed:", cleanupErr)
		fprintln(stderr, "run 'wt remove", info.Name, "--with-branch' to clean up manually")
	}

	return nil
}

func mergeWithLock(
	ctx context.Context,
	stderr io.Writer,
	git *Git,
	locker *fs.Locker,
	lockPath string,
	wtPath, targetWtPath, featureBranch, targetBranch string,
) error {
	// Acquire merge lock with timeout and retries
	lock, err := acquireMergeLock(ctx, stderr, locker, lockPath)
	if err != nil {
		return err
	}

	defer func() {
		_ = lock.Close()
	}()

	// Rebase onto target (under lock, so target can't move)
	err = git.Rebase(ctx, wtPath, targetBranch)
	if err != nil {
		if isConflict(err) {
			// Get conflicting files for better error message
			files, _ := git.ConflictingFiles(ctx, wtPath)

			// Abort rebase to leave clean state
			_ = git.RebaseAbort(ctx, wtPath)

			return formatConflictError(targetBranch, files)
		}

		// Unknown error - try to abort rebase
		abortErr := git.RebaseAbort(ctx, wtPath)

		return errors.Join(
			fmt.Errorf("%w %s: %w", errRebasingOnto, targetBranch, err),
			abortErr,
		)
	}

	// Perform the merge (under lock, guaranteed to succeed if rebase succeeded)
	if targetWtPath != "" {
		// Target is checked out in another worktree - merge there
		err = git.Merge(ctx, targetWtPath, featureBranch, true)
	} else {
		// Target is not checked out anywhere - use local push to update the branch
		err = git.PushLocal(ctx, wtPath, featureBranch, targetBranch)
	}

	if err != nil {
		return fmt.Errorf("%w %s: %w", errMergingInto, targetBranch, err)
	}

	return nil
}

// acquireMergeLock attempts to acquire the merge lock with retries and good error messages.
func acquireMergeLock(ctx context.Context, stderr io.Writer, locker *fs.Locker, lockPath string) (*fs.Lock, error) {
	var lastErr error

	for attempt := 1; attempt <= maxMergeRetries; attempt++ {
		lockCtx, cancel := context.WithTimeout(ctx, mergeLockTimeout)

		lock, err := locker.LockWithTimeout(lockCtx, lockPath)

		cancel()

		if err == nil {
			return lock, nil
		}

		lastErr = err

		if attempt == maxMergeRetries {
			break
		}

		// Check if context was cancelled
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%w: %w", errMergeCancelled, ctx.Err())
		}

		fprintf(stderr, "Waiting for merge lock (attempt %d/%d)...\n", attempt, maxMergeRetries)

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%w: %w", errMergeCancelled, ctx.Err())
		case <-time.After(backoff(attempt)):
		}
	}

	fprintf(stderr, "Lock file: %s\n", lockPath)

	return nil, errors.Join(
		errAcquiringMergeLock,
		errMergeLockTimedOut,
		lastErr,
	)
}

func backoff(attempt int) time.Duration {
	exp := math.Pow(2, float64(attempt))
	delay := min(time.Duration(exp)*mergeBaseDelay, mergeMaxDelay)

	if delay <= 0 {
		delay = mergeBaseDelay
	}

	// Jitter: random between 0 and calculated delay using crypto/rand
	var buf [1]byte

	_, _ = rand.Read(buf[:])

	// Use single byte (0-255) to calculate jitter percentage (0-100%)
	// This avoids uint64->int64 conversion concerns
	jitterPercent := int64(buf[0]) % 100

	return delay * time.Duration(jitterPercent) / 100
}

func isConflict(err error) bool {
	msg := err.Error()

	return strings.Contains(msg, "CONFLICT") ||
		strings.Contains(msg, "conflict") ||
		strings.Contains(msg, "could not apply") ||
		strings.Contains(msg, "Merge conflict")
}

// conflictError wraps conflict information with resolution hints.
type conflictError struct {
	target string
	files  []string
}

func (e *conflictError) Error() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s %s: %s", errRebasingOnto, e.target, errMergeConflict))

	if len(e.files) > 0 {
		sb.WriteString(" in ")
		sb.WriteString(strings.Join(e.files, ", "))
	}

	sb.WriteString("\n\nTo resolve:\n")
	sb.WriteString("  1. Fix conflicts manually\n")
	sb.WriteString("  2. git add <fixed-files>\n")
	sb.WriteString("  3. git rebase --continue\n")
	sb.WriteString("  4. Run wt merge again\n")
	sb.WriteString("\nOr abort with: git rebase --abort")

	return sb.String()
}

func (e *conflictError) Unwrap() error {
	return errMergeConflict
}

func formatConflictError(target string, files []string) error {
	return &conflictError{target: target, files: files}
}

func printDryRun(
	stdout io.Writer,
	feature, target, targetWtPath, mainRepoRoot, wtPath, name string,
	commitCount int,
	keep bool,
) error {
	fprintln(stdout, "Dry run: wt merge", feature, "→", target)
	fprintln(stdout)
	fprintln(stdout, "Checks:")
	fprintln(stdout, "  ✓ Current worktree is clean")
	fprintf(stdout, "  ✓ Target branch '%s' exists\n", target)

	if targetWtPath != "" {
		fprintf(stdout, "  ✓ Target worktree %s is clean\n", targetWtPath)
	}

	fprintln(stdout)
	fprintln(stdout, "Would execute:")

	step := 1

	commitDesc := "commits"
	if commitCount == 1 {
		commitDesc = "commit"
	}

	fprintf(stdout, "  %d. Rebase '%s' onto '%s' (%d %s to replay)\n", step, feature, target, commitCount, commitDesc)
	step++

	mergeLocation := mainRepoRoot
	if targetWtPath != "" {
		mergeLocation = targetWtPath
	}

	fprintf(stdout, "  %d. Fast-forward '%s' to '%s' (in %s)\n", step, target, feature, mergeLocation)
	step++

	if !keep {
		fprintf(stdout, "  %d. Run pre-delete hooks\n", step)
		step++

		fprintf(stdout, "  %d. Remove worktree: %s\n", step, wtPath)
		step++

		fprintf(stdout, "  %d. Delete branch: %s\n", step, name)
	}

	fprintln(stdout)
	fprintln(stdout, "No changes made.")

	return nil
}
