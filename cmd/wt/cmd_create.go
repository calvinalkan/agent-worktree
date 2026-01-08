package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/calvinalkan/agent-task/pkg/fs"
	flag "github.com/spf13/pflag"
)

// ErrNameAlreadyInUse is returned when the requested worktree name is already in use.
var ErrNameAlreadyInUse = errors.New("name already in use (use wt list to see worktrees)")

// CreateCmd returns the create command.
func CreateCmd(cfg Config, fsys fs.FS, git *Git, env map[string]string) *Command {
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	flags.BoolP("help", "h", false, "Show help")
	flags.StringP("name", "n", "", "Worktree and branch name (default: auto-generated)")
	flags.StringP("from-branch", "b", "", "Branch to base off (default: current branch)")
	flags.Bool("with-changes", false, "Copy staged, unstaged, and untracked files to new worktree")
	flags.Bool("json", false, "Output as JSON")

	return &Command{
		Flags: flags,
		Usage: "create [flags]",
		Short: "Create a new worktree",
		Long: `Create a new worktree with auto-generated name and unique ID.

A git branch is created with the same name as the worktree. The worktree
directory is created at <base>/<repo>/<name>, where base is configured
in .wt/config.json or ~/.config/wt/config.json.

Metadata is written to .wt/worktree.json inside the new worktree.
If .wt/hooks/post-create exists and is executable, it runs after creation.`,
		Exec: func(ctx context.Context, _ io.Reader, stdout, stderr io.Writer, _ []string) error {
			customName, _ := flags.GetString("name")
			fromBranch, _ := flags.GetString("from-branch")
			withChanges, _ := flags.GetBool("with-changes")
			jsonOutput, _ := flags.GetBool("json")

			return execCreate(ctx, stdout, stderr, cfg, fsys, git, env, customName, fromBranch, withChanges, jsonOutput)
		},
	}
}

// createLockTimeout is the maximum time to wait for the create lock.
// This is short because we only hold the lock during ID/name generation
// and metadata write, not during slow operations like hooks.
const createLockTimeout = 5 * time.Second

// worktreeLockPath returns the path to the lock file for worktree operations.
// We use a dedicated lock file inside the git common directory to:
// - Avoid orphan files in the workspace (it's inside .git/)
// - Avoid conflicts with git operations (dedicated file, not used by git)
// - Ensure all worktrees share the same lock (using git common dir)
// - Handle cleanup automatically (deleted when repo is deleted).
func worktreeLockPath(gitCommonDir string) string {
	return filepath.Join(gitCommonDir, "wt.lock")
}

// worktreeExcludePattern is the pattern added to .git/info/exclude
// to prevent .wt/worktree.json from being tracked.
const worktreeExcludePattern = ".wt/worktree.json"

// ensureWorktreeExcluded adds .wt/worktree.json to .git/info/exclude if not present.
// Returns a warning message if the operation fails, or empty string on success.
func ensureWorktreeExcluded(fsys fs.FS, gitCommonDir string) string {
	excludePath := filepath.Join(gitCommonDir, "info", "exclude")

	// Read existing content
	content, err := fsys.ReadFile(excludePath)
	if err != nil {
		return fmt.Sprintf("warning: could not read %s: %v\nPlease add '%s' to your .gitignore manually.",
			excludePath, err, worktreeExcludePattern)
	}

	// Check if pattern already exists
	lines := strings.SplitSeq(string(content), "\n")
	for line := range lines {
		if strings.TrimSpace(line) == worktreeExcludePattern {
			return "" // Already present
		}
	}

	// Append pattern
	newContent := string(content)
	if newContent != "" && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}

	newContent += worktreeExcludePattern + "\n"

	// Write back
	err = fsys.WriteFile(excludePath, []byte(newContent), 0o644)
	if err != nil {
		return fmt.Sprintf("warning: could not update %s: %v\nPlease add '%s' to your .gitignore manually.",
			excludePath, err, worktreeExcludePattern)
	}

	return ""
}

func execCreate(
	ctx context.Context,
	stdout, stderr io.Writer,
	cfg Config,
	fsys fs.FS,
	git *Git,
	env map[string]string,
	customName, fromBranch string,
	withChanges, jsonOutput bool,
) error {
	// 1. Verify git repository and get main repo root
	// MainRepoRoot returns the main repo's root even when inside a worktree,
	// ensuring all worktrees share the same base directory and lock file.
	mainRepoRoot, err := git.MainRepoRoot(ctx, cfg.EffectiveCwd)
	if err != nil {
		return ErrNotGitRepository
	}

	// 2. Get git common directory (shared across all worktrees) for locking
	gitCommonDir, err := git.GitCommonDir(ctx, cfg.EffectiveCwd)
	if err != nil {
		return fmt.Errorf("cannot determine git directory: %w", err)
	}

	// 2a. Ensure .wt/worktree.json is excluded from git tracking
	if warning := ensureWorktreeExcluded(fsys, gitCommonDir); warning != "" {
		fprintln(stderr, warning)
	}

	// 3. Resolve base branch
	baseBranch := fromBranch
	if baseBranch == "" {
		baseBranch, err = git.CurrentBranch(ctx, cfg.EffectiveCwd)
		if err != nil {
			return fmt.Errorf("getting current branch (use --from-branch if in detached HEAD): %w", err)
		}
	}

	// 4. Create base directory if needed (must exist before locking)
	baseDir := resolveWorktreeBaseDir(cfg, mainRepoRoot)

	err = fsys.MkdirAll(baseDir, 0o750)
	if err != nil {
		return fmt.Errorf("cannot create base directory: %w", err)
	}

	// 5. Acquire exclusive lock for ID generation
	// This prevents race conditions when multiple processes create worktrees
	locker := fs.NewLocker(fsys)
	lockPath := worktreeLockPath(gitCommonDir)

	lockCtx, lockCancel := context.WithTimeout(ctx, createLockTimeout)
	defer lockCancel()

	lock, err := locker.LockWithTimeout(lockCtx, lockPath)
	if err != nil {
		return fmt.Errorf("acquiring create lock (another wt process may be running): %w", err)
	}

	// Safety net - Close is idempotent; we release early after metadata write
	// but this handles cleanup on early returns
	defer func() { _ = lock.Close() }()

	// 6. Find existing worktrees (safe now, we hold the lock)
	existing, err := findWorktrees(fsys, baseDir)
	if err != nil {
		return fmt.Errorf("scanning existing worktrees: %w", err)
	}

	// Calculate next ID
	nextID := 1
	for _, wt := range existing {
		if wt.ID >= nextID {
			nextID = wt.ID + 1
		}
	}

	// 7. Generate agent_id
	existingNames := getExistingNames(existing)

	agentID, err := generateAgentID(existingNames)
	if err != nil {
		return err
	}

	// 8. Set name
	name := customName
	if name == "" {
		name = agentID
	}

	// Check name collision (in case --name was provided)
	if slices.Contains(existingNames, name) {
		return fmt.Errorf("%w: %s", ErrNameAlreadyInUse, name)
	}

	// 9. Resolve worktree path
	wtPath := resolveWorktreePath(cfg, mainRepoRoot, name)

	// 10. git worktree add -b <name> <path> <base-branch>
	err = git.WorktreeAdd(ctx, mainRepoRoot, wtPath, name, baseBranch)
	if err != nil {
		return err
	}

	// 11. Write .wt/worktree.json metadata
	info := &WorktreeInfo{
		Name:       name,
		AgentID:    agentID,
		ID:         nextID,
		BaseBranch: baseBranch,
		Created:    time.Now().UTC(),
	}

	err = writeWorktreeInfo(fsys, wtPath, info)
	if err != nil {
		// Rollback: remove worktree
		rmErr := git.WorktreeRemove(ctx, mainRepoRoot, wtPath, true)
		brErr := git.BranchDelete(ctx, mainRepoRoot, name, true)

		return errors.Join(
			fmt.Errorf("writing worktree metadata: %w", err),
			rmErr,
			brErr,
		)
	}

	// Release lock early - only needed for ID/name generation.
	// Close is idempotent; defer above handles cleanup on early returns.
	_ = lock.Close()

	// 12. If --with-changes: copy uncommitted changes
	if withChanges {
		err = copyUncommittedChanges(ctx, fsys, git, cfg.EffectiveCwd, wtPath)
		if err != nil {
			// Rollback: remove worktree and delete branch
			rmErr := git.WorktreeRemove(ctx, mainRepoRoot, wtPath, true)
			brErr := git.BranchDelete(ctx, mainRepoRoot, name, true)

			return errors.Join(
				fmt.Errorf("copying uncommitted changes: %w", err),
				rmErr,
				brErr,
			)
		}
	}

	// 13. Run post-create hook
	hookRunner := NewHookRunner(fsys, mainRepoRoot, env, stdout, stderr)

	err = hookRunner.RunPostCreate(ctx, info, wtPath, cfg.EffectiveCwd)
	if err != nil {
		// Rollback: remove worktree and delete branch
		rmErr := git.WorktreeRemove(ctx, mainRepoRoot, wtPath, true)
		brErr := git.BranchDelete(ctx, mainRepoRoot, name, true)

		return errors.Join(
			fmt.Errorf("post-create hook failed (check hook output above): %w", err),
			rmErr,
			brErr,
		)
	}

	// 14. Print success output
	if jsonOutput {
		return outputCreateJSON(stdout, name, agentID, nextID, wtPath, baseBranch)
	}

	fprintln(stdout, "Created worktree:")
	fprintf(stdout, "  name:        %s\n", name)
	fprintf(stdout, "  agent_id:    %s\n", agentID)
	fprintf(stdout, "  id:          %d\n", nextID)
	fprintf(stdout, "  path:        %s\n", wtPath)
	fprintf(stdout, "  branch:      %s\n", name)
	fprintf(stdout, "  from:        %s\n", baseBranch)

	return nil
}

// copyUncommittedChanges copies staged, unstaged, and untracked files from srcDir to dstDir.
// It respects .gitignore for untracked files.
func copyUncommittedChanges(ctx context.Context, fsys fs.FS, git *Git, srcDir, dstDir string) error {
	// Get all uncommitted files (staged, unstaged, and untracked)
	files, err := git.ChangedFiles(ctx, srcDir)
	if err != nil {
		return fmt.Errorf("getting changed files: %w", err)
	}

	// Copy each file
	for _, relPath := range files {
		srcPath := filepath.Join(srcDir, relPath)
		dstPath := filepath.Join(dstDir, relPath)

		// Read source file
		data, readErr := fsys.ReadFile(srcPath)
		if readErr != nil {
			// File might have been deleted (shown in diff but gone), skip silently
			continue
		}

		// Create parent directories
		mkdirErr := fsys.MkdirAll(filepath.Dir(dstPath), 0o755)
		if mkdirErr != nil {
			return fmt.Errorf("creating directory for %s: %w", relPath, mkdirErr)
		}

		// Write to destination
		writeErr := fsys.WriteFile(dstPath, data, 0o644)
		if writeErr != nil {
			return fmt.Errorf("writing %s: %w", relPath, writeErr)
		}
	}

	return nil
}

// jsonCreateOutput is the JSON output format for the create command.
type jsonCreateOutput struct {
	Name    string `json:"name"`
	AgentID string `json:"agent_id"`
	ID      int    `json:"id"`
	Path    string `json:"path"`
	Branch  string `json:"branch"`
	From    string `json:"from"`
}

func outputCreateJSON(output io.Writer, name, agentID string, id int, path, from string) error {
	result := jsonCreateOutput{
		Name:    name,
		AgentID: agentID,
		ID:      id,
		Path:    path,
		Branch:  name,
		From:    from,
	}

	enc := json.NewEncoder(output)
	enc.SetIndent("", "  ")

	encodeErr := enc.Encode(result)
	if encodeErr != nil {
		return fmt.Errorf("encoding JSON: %w", encodeErr)
	}

	return nil
}
