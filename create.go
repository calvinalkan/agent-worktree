package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"time"

	"github.com/calvinalkan/agent-task/pkg/fs"
	flag "github.com/spf13/pflag"
)

// ErrNameAlreadyInUse is returned when the requested worktree name is already in use.
var ErrNameAlreadyInUse = errors.New("name already in use")

// CreateCmd returns the create command.
func CreateCmd(cfg Config, fsys fs.FS, git *Git, env map[string]string) *Command {
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	flags.BoolP("help", "h", false, "Show help")
	flags.StringP("name", "n", "", "Custom worktree name")
	flags.StringP("from-branch", "b", "", "Create from branch (default: current branch)")
	flags.Bool("with-changes", false, "Copy uncommitted changes to new worktree")

	return &Command{
		Flags: flags,
		Usage: "create [flags]",
		Short: "Create a new worktree",
		Long: `Create a new worktree with auto-generated name and unique ID.

A random agent_id is generated (e.g., swift-fox) and used as the default
worktree name. Use --name to override.`,
		Exec: func(ctx context.Context, _ io.Reader, stdout, stderr io.Writer, _ []string) error {
			customName, _ := flags.GetString("name")
			fromBranch, _ := flags.GetString("from-branch")
			withChanges, _ := flags.GetBool("with-changes")

			return execCreate(ctx, stdout, stderr, cfg, fsys, git, env, customName, fromBranch, withChanges)
		},
	}
}

func execCreate(
	ctx context.Context,
	stdout, stderr io.Writer,
	cfg Config,
	fsys fs.FS,
	git *Git,
	env map[string]string,
	customName, fromBranch string,
	withChanges bool,
) error {
	// 1. Verify git repository
	repoRoot, err := git.RepoRoot(cfg.EffectiveCwd)
	if err != nil {
		return ErrNotGitRepository
	}

	// 2. Resolve base branch
	baseBranch := fromBranch
	if baseBranch == "" {
		baseBranch, err = git.CurrentBranch(cfg.EffectiveCwd)
		if err != nil {
			return fmt.Errorf("cannot determine current branch: %w", err)
		}
	}

	// 3. Find existing worktrees
	baseDir := resolveWorktreeBaseDir(cfg, repoRoot)

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

	// 4. Generate agent_id
	existingNames := getExistingNames(existing)

	agentID, err := generateAgentID(existingNames)
	if err != nil {
		return err
	}

	// 5. Set name
	name := customName
	if name == "" {
		name = agentID
	}

	// Check name collision (in case --name was provided)
	if slices.Contains(existingNames, name) {
		return fmt.Errorf("%w: %s", ErrNameAlreadyInUse, name)
	}

	// 6. Resolve worktree path
	wtPath := resolveWorktreePath(cfg, repoRoot, name)

	// 7. Create base directory if needed
	wtBaseDir := resolveWorktreeBaseDir(cfg, repoRoot)

	err = fsys.MkdirAll(wtBaseDir, 0o750)
	if err != nil {
		return fmt.Errorf("cannot create base directory: %w", err)
	}

	// 8. git worktree add -b <name> <path> <base-branch>
	err = git.WorktreeAdd(repoRoot, wtPath, name, baseBranch)
	if err != nil {
		return err
	}

	// 9. Write .wt/worktree.json metadata
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
		_ = git.WorktreeRemove(repoRoot, wtPath, true)
		_ = git.BranchDelete(repoRoot, name, true)

		return fmt.Errorf("writing worktree metadata: %w", err)
	}

	// 10. If --with-changes: copy uncommitted changes
	if withChanges {
		err = copyUncommittedChanges(fsys, git, cfg.EffectiveCwd, wtPath)
		if err != nil {
			// Rollback: remove worktree and delete branch
			_ = git.WorktreeRemove(repoRoot, wtPath, true)
			_ = git.BranchDelete(repoRoot, name, true)

			return fmt.Errorf("copying uncommitted changes: %w", err)
		}
	}

	// 11. Run post-create hook
	hookRunner := NewHookRunner(fsys, repoRoot, env, stdout, stderr)

	err = hookRunner.RunPostCreate(ctx, info, wtPath, cfg.EffectiveCwd)
	if err != nil {
		// 12. Rollback: remove worktree and delete branch
		_ = git.WorktreeRemove(repoRoot, wtPath, true)
		_ = git.BranchDelete(repoRoot, name, true)

		return fmt.Errorf("post-create hook failed: %w", err)
	}

	// 13. Print success output
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
func copyUncommittedChanges(fsys fs.FS, git *Git, srcDir, dstDir string) error {
	// Get all uncommitted files (staged, unstaged, and untracked)
	files, err := git.ChangedFiles(srcDir)
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
