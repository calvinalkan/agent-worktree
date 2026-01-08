package main

import (
	"bytes"
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

// prefixWriter wraps an io.Writer and prefixes each line with a string.
type prefixWriter struct {
	w       io.Writer
	prefix  []byte
	atStart bool // true if we're at the start of a line
}

// newPrefixWriter creates a writer that prefixes each line.
func newPrefixWriter(w io.Writer, prefix string) *prefixWriter {
	return &prefixWriter{
		w:       w,
		prefix:  []byte(prefix),
		atStart: true,
	}
}

// errPrefixWrite is returned when the underlying writer fails.
var errPrefixWrite = errors.New("writing hook output")

func (p *prefixWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	totalWritten := len(data) // Report original bytes to caller

	for len(data) > 0 {
		if p.atStart {
			_, writeErr := p.w.Write(p.prefix)
			if writeErr != nil {
				return 0, fmt.Errorf("%w: %w", errPrefixWrite, writeErr)
			}

			p.atStart = false
		}

		idx := bytes.IndexByte(data, '\n')
		if idx == -1 {
			// No newline, write all remaining data
			_, writeErr := p.w.Write(data)
			if writeErr != nil {
				return totalWritten, fmt.Errorf("%w: %w", errPrefixWrite, writeErr)
			}

			return totalWritten, nil
		}

		// Write up to and including newline
		_, writeErr := p.w.Write(data[:idx+1])
		if writeErr != nil {
			return 0, fmt.Errorf("%w: %w", errPrefixWrite, writeErr)
		}

		data = data[idx+1:]
		p.atStart = true
	}

	return totalWritten, nil
}

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
// The hook runs with working directory set to wtPath.
func (h *HookRunner) RunPostCreate(ctx context.Context, info *WorktreeInfo, wtPath string) error {
	wtEnv := hookEnv(info, wtPath, h.repoRoot)

	return runHook(ctx, h.fsys, h.repoRoot, "post-create", h.baseEnv, wtEnv, wtPath, h.stdout, h.stderr)
}

// RunPreDelete executes the pre-delete hook if it exists.
// The hook runs with working directory set to wtPath.
func (h *HookRunner) RunPreDelete(ctx context.Context, info *WorktreeInfo, wtPath string) error {
	wtEnv := hookEnv(info, wtPath, h.repoRoot)

	return runHook(ctx, h.fsys, h.repoRoot, "pre-delete", h.baseEnv, wtEnv, wtPath, h.stdout, h.stderr)
}

// hookEnv creates the WT_* environment variables available to hooks.
// WT_PATH equals the hook's working directory ($PWD).
func hookEnv(info *WorktreeInfo, wtPath, repoRoot string) map[string]string {
	return map[string]string{
		"WT_ID":          strconv.Itoa(info.ID),
		"WT_AGENT_ID":    info.AgentID,
		"WT_NAME":        info.Name,
		"WT_PATH":        wtPath,
		"WT_BASE_BRANCH": info.BaseBranch,
		"WT_REPO_ROOT":   repoRoot,
	}
}

// runHook executes a hook script if it exists.
// hookName is "post-create" or "pre-delete".
// baseEnv is the inherited environment (passed from Run()'s env parameter).
// wtEnv contains the WT_* variables to add.
// wtPath is the worktree path, used as the hook's working directory.
// Returns nil if hook doesn't exist.
// Returns error if hook exists but is not executable, or if execution fails.
func runHook(
	ctx context.Context,
	fsys fs.FS,
	repoRoot string,
	hookName string,
	baseEnv, wtEnv map[string]string,
	wtPath string,
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
	cmd.Dir = wtPath

	// Prefix hook output so it's clear where it comes from
	prefix := fmt.Sprintf("hook(%s): ", hookName)
	cmd.Stdout = newPrefixWriter(stdout, prefix)
	cmd.Stderr = newPrefixWriter(stderr, prefix)

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
