package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/calvinalkan/agent-task/pkg/fs"
)

// hookTimeout is the maximum time a hook can run before being killed.
const hookTimeout = 5 * time.Minute

// Hook errors.
var (
	ErrHookNotExecutable = errors.New("hook not executable")
	ErrHookTimeout       = errors.New("hook timed out (hook may be stuck or waiting for input)")
	ErrHookFailed        = errors.New("hook failed")
)

// HookRunner executes lifecycle hooks for worktrees.
type HookRunner struct {
	fsys     fs.FS
	repoRoot string
	baseEnv  map[string]string // inherited environment from Run()
	stdout   io.Writer
	stderr   io.Writer
}

// NewHookRunner creates a hook runner.
// baseEnv should be the env map passed to Run() - we don't call os.Environ().
func NewHookRunner(fsys fs.FS, repoRoot string, baseEnv map[string]string, stdout, stderr io.Writer) *HookRunner {
	return &HookRunner{
		fsys:     fsys,
		repoRoot: repoRoot,
		baseEnv:  baseEnv,
		stdout:   stdout,
		stderr:   stderr,
	}
}

// RunPostCreate executes the post-create hook if it exists.
func (h *HookRunner) RunPostCreate(ctx context.Context, info *WorktreeInfo, wtPath, sourceDir string) error {
	wtEnv := hookEnv(info, wtPath, h.repoRoot, sourceDir)

	return runHook(ctx, h.fsys, h.repoRoot, "post-create", h.baseEnv, wtEnv, sourceDir, h.stdout, h.stderr)
}

// RunPreDelete executes the pre-delete hook if it exists.
func (h *HookRunner) RunPreDelete(ctx context.Context, info *WorktreeInfo, wtPath, sourceDir string) error {
	wtEnv := hookEnv(info, wtPath, h.repoRoot, sourceDir)

	return runHook(ctx, h.fsys, h.repoRoot, "pre-delete", h.baseEnv, wtEnv, sourceDir, h.stdout, h.stderr)
}

// hookEnv creates the WT_* environment variables available to hooks.
func hookEnv(info *WorktreeInfo, wtPath, repoRoot, sourceDir string) map[string]string {
	return map[string]string{
		"WT_ID":          strconv.Itoa(info.ID),
		"WT_AGENT_ID":    info.AgentID,
		"WT_NAME":        info.Name,
		"WT_PATH":        wtPath,
		"WT_BASE_BRANCH": info.BaseBranch,
		"WT_REPO_ROOT":   repoRoot,
		"WT_SOURCE":      sourceDir,
	}
}

// runHook executes a hook script if it exists.
// hookName is "post-create" or "pre-delete".
// baseEnv is the inherited environment (passed from Run()'s env parameter).
// wtEnv contains the WT_* variables to add.
// Returns nil if hook doesn't exist.
// Returns error if hook exists but is not executable, or if execution fails.
func runHook(
	ctx context.Context,
	fsys fs.FS,
	repoRoot string,
	hookName string,
	baseEnv, wtEnv map[string]string,
	cwd string,
	stdout, stderr io.Writer,
) error {
	hookPath := filepath.Join(repoRoot, ".wt", "hooks", hookName)

	// Check if hook exists
	info, statErr := fsys.Stat(hookPath)
	if statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return nil // Hook doesn't exist, skip silently
		}

		return fmt.Errorf("checking hook %s: %w", hookName, statErr)
	}

	// Check if executable
	if info.Mode()&0o111 == 0 {
		return fmt.Errorf("%w: %s (fix with: chmod +x %s)", ErrHookNotExecutable, hookPath, hookPath)
	}

	// Build command with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, hookTimeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, hookPath)
	cmd.Dir = cwd
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Send SIGTERM instead of SIGKILL on context cancellation.
	// This gives hooks a chance to clean up gracefully.
	// WaitDelay ensures we SIGKILL after 3s if the hook ignores the signal.
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 7 * time.Second

	// Build environment from baseEnv + wtEnv
	cmd.Env = make([]string, 0, len(baseEnv)+len(wtEnv))

	for k, v := range baseEnv {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	for k, v := range wtEnv {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	runErr := cmd.Run()
	if runErr != nil {
		if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("%w: %s after 5 minutes", ErrHookTimeout, hookName)
		}

		return fmt.Errorf("%w: %s: %w", ErrHookFailed, hookName, runErr)
	}

	return nil
}
